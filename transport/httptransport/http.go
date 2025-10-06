// Package httptransport provides a client and server implementation for the go-sync-kit Transport over HTTP.
package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Operation constants for consistent error reporting
const (
	opPush          = "httptransport.Push"
	opPull          = "httptransport.Pull"
	opLatestVersion = "httptransport.GetLatestVersion"
	opSubscribe     = "httptransport.Subscribe"
	opPushHandler   = "httptransport.handlePush"
	opPullHandler   = "httptransport.handlePull"
	opLatestHandler = "httptransport.handleLatestVersion"
)

// VersionParser converts a version string into synckit.Version.
type VersionParser func(ctx context.Context, s string) (synckit.Version, error)

// --- HTTP Sync Handler (Server) ---

// SyncHooks provides extensibility points for the sync handler
type SyncHooks struct {
	// AfterCommit is called after events are successfully committed to storage
	AfterCommit func(ctx context.Context, committed []synckit.EventWithVersion)

	// BeforePull is called before pulling events (for metrics, etc.)
	BeforePull func(ctx context.Context, since synckit.Version)
}

// SyncHandler is an http.Handler that serves sync requests.
type SyncHandler struct {
	store         synckit.EventStore
	logger        *slog.Logger
	versionParser VersionParser
	options       *ServerOptions
	hooks         *SyncHooks
}

// NewSyncHandler creates a new handler for serving sync endpoints.
// It requires an EventStore to interact with the database and optionally accepts a VersionParser and ServerOptions.
func NewSyncHandler(store synckit.EventStore, logger *slog.Logger, parser VersionParser, options *ServerOptions) *SyncHandler {
	return NewSyncHandlerWithLogger(store, logger, parser, options)
}

// NewSyncHandlerWithLogger creates a new handler for serving sync endpoints with structured logging.
func NewSyncHandlerWithLogger(store synckit.EventStore, logger *slog.Logger, parser VersionParser, options *ServerOptions) *SyncHandler {
	return NewSyncHandlerWithHooks(store, logger, parser, options, nil)
}

// NewSyncHandlerWithHooks creates a new handler for serving sync endpoints with hooks support.
func NewSyncHandlerWithHooks(store synckit.EventStore, logger *slog.Logger, parser VersionParser, options *ServerOptions, hooks *SyncHooks) *SyncHandler {
	if logger == nil {
		logger = logging.Default().Logger
	}
	if parser == nil {
		// Default to using store's ParseVersion method if no parser provided
		parser = store.ParseVersion
	}
	if options == nil {
		options = DefaultServerOptions()
	}
	return &SyncHandler{
		store:         store,
		logger:        logger,
		versionParser: parser,
		options:       options,
		hooks:         hooks,
	}
}

// Helper function for common response handling
func (h *SyncHandler) respond(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	respondWithJSON(w, r, code, payload, h.options)
}

func (h *SyncHandler) respondErr(w http.ResponseWriter, r *http.Request, code int, message string) {
	respondWithError(w, r, code, message, h.options)
}

// ServeHTTP routes requests to the appropriate handler (/push or /pull).
func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip the /sync prefix if present
	path := r.URL.Path
	if p := "/sync"; len(path) >= len(p) && path[:len(p)] == p {
		path = path[len(p):]
	}

	switch path {
	case "/push":
		h.handlePush(w, r)
	case "/pull":
		h.handlePull(w, r)
	case "/latest-version":
		h.handleLatestVersion(w, r)
	case "/pull-cursor":
		h.handlePullCursor(w, r, NewCursorOptions())
	default:
		respondWithError(w, r, http.StatusNotFound, "not found", h.options)
	}
}

func (h *SyncHandler) handlePush(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling push request",
		slog.String("method", r.Method),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()))

	if r.Method != http.MethodPost {
		h.logger.Warn("Push request with invalid method",
			slog.String("method", r.Method),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Validate Content-Type for JSON endpoints
	if !validateContentType(w, r, h.options) {
		return // validateContentType already sent the response
	}

	// Check Content-Length if available
	if r.ContentLength > h.options.MaxRequestSize {
		h.logger.Warn("Push request body too large",
			slog.Int64("content_length", r.ContentLength),
			slog.Int64("max_size", h.options.MaxRequestSize),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("request body too large: maximum size is %d bytes", h.options.MaxRequestSize))
		return
	}

	// Create safe reader that handles both compressed and decompressed size limits
	safeReader, cleanup, err := createSafeRequestReader(w, r, h.options)
	if err != nil {
		h.logger.Error("Failed to create safe request reader",
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		// Use mapped error handling for consistent HTTP status codes
		respondWithMappedError(w, r, err, "invalid request body", h.options)
		return
	}
	defer cleanup()

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(safeReader).Decode(&jsonEvents); err != nil {
		if err == io.EOF {
			h.logger.Error("Empty request body in push request",
				slog.String("remote_addr", r.RemoteAddr))
			h.respondErr(w, r, http.StatusBadRequest, "empty request body")
			return
		}
		// Log the error with structured logging
		h.logger.Error("Failed to decode push request body",
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		// Use mapped error handling for consistent HTTP status codes
		respondWithMappedError(w, r, err, "invalid request body", h.options)
		return
	}

	// Track successfully committed events for hooks
	var committedEvents []synckit.EventWithVersion

	for _, jev := range jsonEvents {
		ev, err := fromJSONEventWithVersion(r.Context(), h.versionParser, jev)
		if err != nil {
			h.logger.Warn("Failed to convert JSON event in push request",
				slog.String("error", err.Error()),
				slog.String("event_id", jev.Event.ID),
				slog.String("remote_addr", r.RemoteAddr))
			continue
		}
		// Note: The server-side store will assign its own version upon insertion.
		// The version from the client is ignored here, which is typical for
		// server-authoritative versioning.
		if err := h.store.Store(r.Context(), ev.Event, ev.Version); err != nil {
			// Log the error for diagnosis, but continue processing other events
			h.logger.Warn("Failed to store event during push",
				slog.String("error", err.Error()),
				slog.String("event_id", ev.Event.ID()),
				slog.String("remote_addr", r.RemoteAddr))
			// Note: Could check for specific store errors here and fail the batch if needed
			// For now, we continue to be resilient to duplicate events during sync
		} else {
			// Event was successfully stored, add to committed events
			committedEvents = append(committedEvents, ev)
		}
	}

	h.logger.Info("Successfully pushed events",
		slog.Int("event_count", len(jsonEvents)),
		slog.Int("committed_count", len(committedEvents)),
		slog.String("remote_addr", r.RemoteAddr))

	// Send response first
	h.respond(w, r, http.StatusOK, map[string]string{"status": "ok"})

	// Call AfterCommit hook asynchronously if there are committed events
	if h.hooks != nil && h.hooks.AfterCommit != nil && len(committedEvents) > 0 {
		go func() {
			// Create timeout context for hook execution
			hookCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			h.logger.Debug("Calling AfterCommit hook",
				slog.Int("committed_events", len(committedEvents)),
				slog.String("remote_addr", r.RemoteAddr))

			h.hooks.AfterCommit(hookCtx, committedEvents)
		}()
	}
}

func (h *SyncHandler) handleLatestVersion(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling latest version request",
		slog.String("method", r.Method),
		slog.String("remote_addr", r.RemoteAddr))

	if r.Method != http.MethodGet {
		h.logger.Warn("Latest version request with invalid method",
			slog.String("method", r.Method),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	version, err := h.store.LatestVersion(r.Context())
	if err != nil {
		h.logger.Error("Failed to get latest version",
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusInternalServerError, "could not get latest version")
		return
	}

	h.logger.Debug("Returning latest version",
		slog.String("version", version.String()),
		slog.String("remote_addr", r.RemoteAddr))
	h.respond(w, r, http.StatusOK, version.String())
}

func (h *SyncHandler) handlePull(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling pull request",
		slog.String("method", r.Method),
		slog.String("remote_addr", r.RemoteAddr))

	if r.Method != http.MethodGet {
		h.logger.Warn("Pull request with invalid method",
			slog.String("method", r.Method),
			slog.String("remote_addr", r.RemoteAddr))
		respondWithStructuredError(w, r, opPull, ErrCodeInvalidRequest, "method not allowed", h.options)
		return
	}

	// Parse query parameters (since, limit, filters)
	query, err := ParsePullQuery(r.Context(), r, h.versionParser)
	if err != nil {
		h.logger.Warn("Invalid pull query parameters",
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		respondWithStructuredError(w, r, opPull, ErrCodeInvalidCursor, err.Error(), h.options)
		return
	}

	h.logger.Debug("Parsed pull query",
		slog.String("since", query.Since.String()),
		slog.Int("limit", query.Limit),
		slog.Int("filter_count", len(query.Filters)),
		slog.String("remote_addr", r.RemoteAddr))

	// Call BeforePull hook if configured
	if h.hooks != nil && h.hooks.BeforePull != nil {
		h.logger.Debug("Calling BeforePull hook",
			slog.String("since_version", query.Since.String()),
			slog.String("remote_addr", r.RemoteAddr))
		h.hooks.BeforePull(r.Context(), query.Since)
	}

	// Load events with filters
	events, err := h.store.Load(r.Context(), query.Since, query.Filters...)
	if err != nil {
		h.logger.Error("Failed to load events from store",
			slog.String("error", err.Error()),
			slog.String("since_version", query.Since.String()),
			slog.String("remote_addr", r.RemoteAddr))
		respondWithStructuredError(w, r, opPull, ErrCodeInternal, "failed to load events", h.options)
		return
	}

	// Apply limit to result set
	if len(events) > query.Limit {
		h.logger.Debug("Applying limit to events",
			slog.Int("original_count", len(events)),
			slog.Int("limit", query.Limit))
		events = events[:query.Limit]
	}

	// Convert events to JSON format for response
	jsonEvents := make([]JSONEventWithVersion, len(events))
	for i, ev := range events {
		jsonEvents[i] = toJSONEventWithVersion(ev)
	}

	h.logger.Info("Successfully pulled events",
		slog.Int("event_count", len(events)),
		slog.String("since_version", query.Since.String()),
		slog.Int("filter_count", len(query.Filters)),
		slog.String("remote_addr", r.RemoteAddr))
	h.respond(w, r, http.StatusOK, jsonEvents)
}
