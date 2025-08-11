// Package httptransport provides a client and server implementation for the go-sync-kit Transport over HTTP.
package httptransport

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// --- HTTP Transport Client ---

// VersionParser converts a version string into synckit.Version.
type VersionParser func(ctx context.Context, s string) (synckit.Version, error)

// HTTPTransport implements the synckit.Transport interface for communicating over HTTP.
type HTTPTransport struct {
	client        *http.Client
	baseURL       string // e.g., "http://remote-server.com/sync"
	versionParser VersionParser
	logger        *slog.Logger
	options       *ClientOptions
}

// newHTTPClient creates a custom HTTP client based on ClientOptions.
func newHTTPClient(opts *ClientOptions) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()

	// If DisableAutoDecompression is true, disable Go's automatic decompression
	// so we can enforce both compressed and decompressed size limits
	tr.DisableCompression = opts != nil && opts.DisableAutoDecompression

	return &http.Client{Transport: tr}
}

// NewTransport creates a new HTTPTransport client.
// If a custom http.Client is not provided, one will be created based on the options.
func NewTransport(baseURL string, client *http.Client, parser VersionParser, options *ClientOptions) *HTTPTransport {
	return NewTransportWithLogger(baseURL, client, parser, options, logging.Default().Logger)
}

// NewTransportWithLogger creates a new HTTPTransport client with a custom logger.
func NewTransportWithLogger(baseURL string, client *http.Client, parser VersionParser, options *ClientOptions, logger *slog.Logger) *HTTPTransport {
	if options == nil {
		options = DefaultClientOptions()
	}

	// Validate client options
	if err := ValidateClientOptions(options); err != nil {
		// For backward compatibility, we'll just use default options if validation fails
		// In a future version, we might want to panic or return an error
		options = DefaultClientOptions()
	}

	if client == nil {
		client = newHTTPClient(options)
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

	return &HTTPTransport{
		client:        client,
		baseURL:       baseURL,
		versionParser: parser,
		logger:        logger,
		options:       options,
	}
}

// Push sends a batch of events to the remote server via an HTTP POST request.
//
// Note: If CompressionEnabled is true in ClientOptions, the client will compress JSON request bodies over 1KB
// using gzip and set the "Content-Encoding: gzip" header. It also sets "Accept-Encoding" header to
// indicate support for gzip and deflate compressed responses.
//
// The server is responsible for decompressing gzip-compressed requests and compressing responses if requested.
//
// This explicit behavior is needed because Go's default http.Client automatically decompresses gzip responses.
// Our client disables that implicit decompression by managing compression explicitly for more control and security.
func (t *HTTPTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	if len(events) == 0 {
		t.logger.Debug("Push called with no events, skipping")
		return nil // Nothing to push
	}

	t.logger.Debug("Starting push operation",
		slog.Int("event_count", len(events)),
		slog.String("base_url", t.baseURL))

	jsonData := make([]JSONEventWithVersion, 0, len(events))
	for _, ev := range events {
		jsonData = append(jsonData, toJSONEventWithVersion(ev))
	}

	data, err := json.Marshal(jsonData)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to marshal events: %w", err))
	}

	// Prepare the request body (with optional compression)
	var requestBody io.Reader = bytes.NewBuffer(data)
	contentEncoding := ""

	// Compress request body if enabled and data is large enough
	if t.options.CompressionEnabled && len(data) >= 1024 { // 1KB threshold for request compression
		var compressed bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressed)
		if _, err := gzipWriter.Write(data); err != nil {
			t.logger.Error("Failed to compress push request",
				slog.String("error", err.Error()),
				slog.Int("original_size", len(data)))
			return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to compress request: %w", err))
		}
		if err := gzipWriter.Close(); err != nil {
			t.logger.Error("Failed to close gzip writer for push request",
				slog.String("error", err.Error()))
			return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to close gzip writer: %w", err))
		}
		requestBody = &compressed
		contentEncoding = "gzip"
		t.logger.Debug("Compressed push request",
			slog.Int("original_size", len(data)),
			slog.Int("compressed_size", compressed.Len()),
			slog.Float64("compression_ratio", float64(compressed.Len())/float64(len(data))))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/push", requestBody)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to create request: %w", err))
	}

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")

	// Set compression headers
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	// If DisableAutoDecompression is true and compression is enabled,
	// explicitly set Accept-Encoding since auto-decompression is disabled
	if t.options.CompressionEnabled {
		if t.options.DisableAutoDecompression {
			// Explicitly request gzip when auto-decompression is disabled
			req.Header.Set("Accept-Encoding", "gzip")
		} else {
			// Normal case: let Go handle Accept-Encoding automatically
			req.Header.Set("Accept-Encoding", "gzip, deflate")
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		t.logger.Error("Push request failed",
			slog.String("error", err.Error()),
			slog.String("url", t.baseURL+"/push"))
		return syncErrors.NewRetryable(syncErrors.OpPush, fmt.Errorf("network error: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.logger.Error("Push request returned error status",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
			slog.String("url", t.baseURL+"/push"))
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body)))
	}

	t.logger.Debug("Push operation completed successfully",
		slog.Int("event_count", len(events)))
	return nil
}

// Pull fetches events from the remote server since a given version via an HTTP GET request.
func (t *HTTPTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	t.logger.Debug("Starting pull operation",
		slog.String("since_version", since.String()),
		slog.String("base_url", t.baseURL))

	url := fmt.Sprintf("%s/pull?since=%s", t.baseURL, since.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to create request: %w", err))
	}

	// Set Accept-Encoding header if compression is enabled
	if t.options.CompressionEnabled {
		if t.options.DisableAutoDecompression {
			// Explicitly request gzip when auto-decompression is disabled
			req.Header.Set("Accept-Encoding", "gzip")
		} else {
			// Normal case: let Go handle Accept-Encoding automatically
			req.Header.Set("Accept-Encoding", "gzip, deflate")
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		t.logger.Error("Pull request failed",
			slog.String("error", err.Error()),
			slog.String("url", url))
		return nil, syncErrors.NewRetryable(syncErrors.OpPull, fmt.Errorf("network error: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.logger.Error("Pull request returned error status",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
			slog.String("url", url))
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body)))
	}

	// Handle compressed response and enforce size limits
	reader, cleanup, err := createSafeResponseReader(resp, t.options)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to create safe response reader: %w", err))
	}
	defer cleanup()

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(reader).Decode(&jsonEvents); err != nil {
		// Check if this is a size limit violation
		if errors.Is(err, errResponseDecompressedTooLarge) {
			return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("response decompressed size exceeds limit: %w", err))
		}
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

	t.logger.Debug("Pull operation completed successfully",
		slog.Int("event_count", len(events)),
		slog.String("since_version", since.String()))
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

	// Handle compressed response and enforce size limits
	reader, cleanup, err := createSafeResponseReader(resp, t.options)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("failed to create safe response reader: %w", err))
	}
	defer cleanup()

	var versionStr string
	if err := json.NewDecoder(reader).Decode(&versionStr); err != nil {
		// Check if this is a size limit violation
		if errors.Is(err, errResponseDecompressedTooLarge) {
			return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("response decompressed size exceeds limit: %w", err))
		}
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
