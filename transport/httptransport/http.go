// Package httptransport provides a client and server implementation for the go-sync-kit Transport over HTTP.
package httptransport

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
)

// --- HTTP Transport Client ---

// VersionParser converts a version string into synckit.Version.
type VersionParser func(ctx context.Context, s string) (synckit.Version, error)

// HTTPTransport implements the synckit.Transport interface for communicating over HTTP.
type HTTPTransport struct {
	client        *http.Client
	baseURL       string // e.g., "http://remote-server.com/sync"
	versionParser VersionParser
	options       *ClientOptions
}

// NewTransport creates a new HTTPTransport client.
// If a custom http.Client is not provided, http.DefaultClient will be used.
func NewTransport(baseURL string, client *http.Client, parser VersionParser, options *ClientOptions) *HTTPTransport {
	if client == nil {
		client = http.DefaultClient
	}
	if parser == nil {
		// default to integer parser for backward compatibility
		parser = func(ctx context.Context, s string) (synckit.Version, error) {
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, err
			}
		if v == 0 {
			return nil, fmt.Errorf("invalid version: zero is not a valid version")
		}
		return cursor.IntegerCursor{Seq: uint64(v)}, nil
		}
	}
	if options == nil {
		options = DefaultClientOptions()
	}
	return &HTTPTransport{
		client:        client,
		baseURL:       baseURL,
		versionParser: parser,
		options:       options,
	}
}

// Push sends a batch of events to the remote server via an HTTP POST request.
func (t *HTTPTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	if len(events) == 0 {
		return nil // Nothing to push
	}

	jsonData := make([]JSONEventWithVersion, 0, len(events))
	for _, ev := range events {
		jsonData = append(jsonData, toJSONEventWithVersion(ev))
	}

	data, err := json.Marshal(jsonData)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to marshal events: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/push", bytes.NewBuffer(data))
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to create request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Add compression headers if enabled
	if t.options.CompressionEnabled {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return syncErrors.NewRetryable(syncErrors.OpPush, fmt.Errorf("network error: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body)))
	}

	return nil
}

// Pull fetches events from the remote server since a given version via an HTTP GET request.
func (t *HTTPTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	url := fmt.Sprintf("%s/pull?since=%s", t.baseURL, since.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to create request: %w", err))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, syncErrors.NewRetryable(syncErrors.OpPull, fmt.Errorf("network error: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body)))
	}

	// Handle compressed response if enabled
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to create gzip reader: %w", err))
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(reader).Decode(&jsonEvents); err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to decode response: %w", err))
	}

	events := make([]synckit.EventWithVersion, len(jsonEvents))
	for i, jev := range jsonEvents {
	// Use version parser to decode version
	version, err := t.versionParser(ctx, jev.Version)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("invalid version in response: %w", err))
	}

		event := &SimpleEvent{
			IDValue:          jev.Event.ID,
			TypeValue:        jev.Event.Type,
			AggregateIDValue: jev.Event.AggregateID,
			DataValue:        jev.Event.Data,
			MetadataValue:    jev.Event.Metadata,
		}

		events[i] = synckit.EventWithVersion{
			Event:   event,
			Version: version,
		}
	}

	return events, nil
}

// GetLatestVersion fetches the latest version from the remote server.
func (t *HTTPTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	url := fmt.Sprintf("%s/latest-version", t.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("failed to create request for latest version: %w", err))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, syncErrors.NewRetryable(syncErrors.OpTransport, fmt.Errorf("network error while getting latest version: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("server error while fetching latest version (status %d): %s", resp.StatusCode, string(body)))
	}

	var versionStr string
	if err := json.NewDecoder(resp.Body).Decode(&versionStr); err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("failed to decode latest version: %w", err))
	}

	version, err := t.versionParser(ctx, versionStr)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("invalid version format: %w", err))
	}

	return version, nil
}

// Subscribe is not supported by this simple HTTP transport.
// Real-time subscriptions would require WebSockets or gRPC streams.
func (t *HTTPTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return syncErrors.New(syncErrors.OpTransport, fmt.Errorf("subscribe is not implemented for HTTP transport"))
}

// Close does nothing for this transport, as the underlying http.Client is managed externally.
func (t *HTTPTransport) Close() error {
	return nil
}

// --- HTTP Sync Handler (Server) ---

// SyncHandler is an http.Handler that serves sync requests.
type SyncHandler struct {
	store         synckit.EventStore
	logger        *log.Logger
	versionParser VersionParser
	options       *ServerOptions
}

// NewSyncHandler creates a new handler for serving sync endpoints.
// It requires an EventStore to interact with the database and optionally accepts a VersionParser and ServerOptions.
func NewSyncHandler(store synckit.EventStore, logger *log.Logger, parser VersionParser, options *ServerOptions) *SyncHandler {
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
	if r.Method != http.MethodPost {
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check Content-Length if available
	if r.ContentLength > h.options.MaxRequestSize {
		h.respondErr(w, r, http.StatusRequestEntityTooLarge, 
			fmt.Sprintf("request body too large: maximum size is %d bytes", h.options.MaxRequestSize))
		return
	}

	// Wrap body in a LimitReader to prevent memory exhaustion
	limitedReader := io.LimitReader(r.Body, h.options.MaxRequestSize)

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(limitedReader).Decode(&jsonEvents); err != nil {
		if err == io.EOF {
			h.respondErr(w, r, http.StatusBadRequest, "empty request body")
			return
		}
		h.respondErr(w, r, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	for _, jev := range jsonEvents {
		ev, err := fromJSONEventWithVersion(r.Context(), h.versionParser, jev)
		if err != nil {
			h.logger.Printf("Failed to convert JSONEventWithVersion: %v", err)
			continue
		}
		// Note: The server-side store will assign its own version upon insertion.
		// The version from the client is ignored here, which is typical for
		// server-authoritative versioning.
		if err := h.store.Store(r.Context(), ev.Event, ev.Version); err != nil {
			// This could be a unique constraint violation if the event already exists,
			// which is often okay during sync. We log it but don't fail the whole batch.
			// For other errors, we should fail.
			h.logger.Printf("Failed to store event %s: %v", ev.Event.ID(), err)
			// In a real app, you might check for specific errors here.
		}
	}

	h.logger.Printf("Successfully pushed %d events", len(jsonEvents))
	h.respond(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SyncHandler) handleLatestVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	version, err := h.store.LatestVersion(r.Context())
	if err != nil {
		h.logger.Printf("Error getting latest version: %v", err)
		h.respondErr(w, r, http.StatusInternalServerError, "could not get latest version")
		return
	}

	h.respond(w, r, http.StatusOK, version.String())
}

func (h *SyncHandler) handlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		sinceStr = "0"
	}

	// Use the injected version parser to handle version parsing
	// This decouples the transport from specific version implementations
	version, err := h.versionParser(r.Context(), sinceStr)
	if err != nil {
		h.respondErr(w, r, http.StatusBadRequest, "invalid 'since' version: "+err.Error())
		return
	}

	events, err := h.store.Load(r.Context(), version)
	if err != nil {
		h.logger.Printf("Error loading events from store: %v", err)
		h.respondErr(w, r, http.StatusInternalServerError, "could not load events")
		return
	}

	// Convert events to JSON format for response
	jsonEvents := make([]JSONEventWithVersion, len(events))
	for i, ev := range events {
		jsonEvents[i] = toJSONEventWithVersion(ev)
	}

	h.logger.Printf("Pulled %d events since version %s", len(events), sinceStr)
	h.respond(w, r, http.StatusOK, jsonEvents)
}

