// Package httptransport provides a client and server implementation for the go-sync-kit Transport over HTTP.
package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// VersionParser converts a version string into synckit.Version.
type VersionParser func(ctx context.Context, s string) (synckit.Version, error)
// --- HTTP Sync Handler (Server) ---

// SyncHandler is an http.Handler that serves sync requests.
type SyncHandler struct {
	store         synckit.EventStore
	logger        *slog.Logger
	versionParser VersionParser
	options       *ServerOptions
}

// NewSyncHandler creates a new handler for serving sync endpoints.
// It requires an EventStore to interact with the database and optionally accepts a VersionParser and ServerOptions.
func NewSyncHandler(store synckit.EventStore, logger *slog.Logger, parser VersionParser, options *ServerOptions) *SyncHandler {
	return NewSyncHandlerWithLogger(store, logger, parser, options)
}

// NewSyncHandlerWithLogger creates a new handler for serving sync endpoints with structured logging.
func NewSyncHandlerWithLogger(store synckit.EventStore, logger *slog.Logger, parser VersionParser, options *ServerOptions) *SyncHandler {
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
			// This could be a unique constraint violation if the event already exists,
			// which is often okay during sync. We log it but don't fail the whole batch.
			// For other errors, we should fail.
			h.logger.Warn("Failed to store event during push",
				slog.String("error", err.Error()),
				slog.String("event_id", ev.Event.ID()),
				slog.String("remote_addr", r.RemoteAddr))
			// In a real app, you might check for specific errors here.
		}
	}

	h.logger.Info("Successfully pushed events",
		slog.Int("event_count", len(jsonEvents)),
		slog.String("remote_addr", r.RemoteAddr))
		h.respond(w, r, http.StatusOK, map[string]string{"status": "ok"})
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
	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		sinceStr = "0"
	}

	h.logger.Debug("Handling pull request",
		slog.String("method", r.Method),
		slog.String("since_version", sinceStr),
		slog.String("remote_addr", r.RemoteAddr))

	if r.Method != http.MethodGet {
		h.logger.Warn("Pull request with invalid method",
			slog.String("method", r.Method),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Use the injected version parser to handle version parsing
	// This decouples the transport from specific version implementations
	version, err := h.versionParser(r.Context(), sinceStr)
	if err != nil {
		h.logger.Warn("Invalid since version in pull request",
			slog.String("since_version", sinceStr),
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusBadRequest, "invalid 'since' version: "+err.Error())
		return
	}

	events, err := h.store.Load(r.Context(), version)
	if err != nil {
		h.logger.Error("Failed to load events from store",
			slog.String("error", err.Error()),
			slog.String("since_version", sinceStr),
			slog.String("remote_addr", r.RemoteAddr))
		h.respondErr(w, r, http.StatusInternalServerError, "could not load events")
		return
	}

	// Convert events to JSON format for response
	jsonEvents := make([]JSONEventWithVersion, len(events))
	for i, ev := range events {
		jsonEvents[i] = toJSONEventWithVersion(ev)
	}

	h.logger.Info("Successfully pulled events",
		slog.Int("event_count", len(events)),
		slog.String("since_version", sinceStr),
		slog.String("remote_addr", r.RemoteAddr))
	h.respond(w, r, http.StatusOK, jsonEvents)
}
