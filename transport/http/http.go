// Package http provides a client and server implementation for the go-sync-kit Transport over HTTP.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

sync "github.com/c0deZ3R0/go-sync-kit"
syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite" // Used for client-side version parsing
)

// JSONEvent is a JSON-serializable representation of an Event
type JSONEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	AggregateID string                 `json:"aggregate_id"`
	Data        interface{}            `json:"data"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// JSONEventWithVersion is a JSON-serializable representation of EventWithVersion
type JSONEventWithVersion struct {
	Event   JSONEvent `json:"event"`
	Version string    `json:"version"`
}

// toJSONEvent converts a sync.Event to JSONEvent
func toJSONEvent(event sync.Event) JSONEvent {
	return JSONEvent{
		ID:          event.ID(),
		Type:        event.Type(),
		AggregateID: event.AggregateID(),
		Data:        event.Data(),
		Metadata:    event.Metadata(),
	}
}

// toJSONEventWithVersion converts sync.EventWithVersion to JSONEventWithVersion
func toJSONEventWithVersion(ev sync.EventWithVersion) JSONEventWithVersion {
	return JSONEventWithVersion{
		Event:   toJSONEvent(ev.Event),
		Version: ev.Version.String(),
	}
}

// fromJSONEvent converts JSONEvent to a concrete Event implementation
func fromJSONEvent(je JSONEvent) sync.Event {
	return &sqlite.StoredEvent{
		// We need to access the private fields, so let's create a simple event struct instead
	}
}

// SimpleEvent is a simple implementation of sync.Event for HTTP transport
type SimpleEvent struct {
	IDValue          string                 `json:"id"`
	TypeValue        string                 `json:"type"`
	AggregateIDValue string                 `json:"aggregate_id"`
	DataValue        interface{}            `json:"data"`
	MetadataValue    map[string]interface{} `json:"metadata"`
}

func (e *SimpleEvent) ID() string                               { return e.IDValue }
func (e *SimpleEvent) Type() string                             { return e.TypeValue }
func (e *SimpleEvent) AggregateID() string                      { return e.AggregateIDValue }
func (e *SimpleEvent) Data() interface{}                        { return e.DataValue }
func (e *SimpleEvent) Metadata() map[string]interface{}         { return e.MetadataValue }

// fromJSONEventWithVersion converts JSONEventWithVersion back to sync.EventWithVersion
// It uses the provided EventStore to parse the version string, making it version-implementation agnostic
func fromJSONEventWithVersion(ctx context.Context, store sync.EventStore, jev JSONEventWithVersion) (sync.EventWithVersion, error) {
	version, err := store.ParseVersion(ctx, jev.Version)
	if err != nil {
		return sync.EventWithVersion{}, fmt.Errorf("invalid version: %w", err)
	}
	
	event := &SimpleEvent{
		IDValue:          jev.Event.ID,
		TypeValue:        jev.Event.Type,
		AggregateIDValue: jev.Event.AggregateID,
		DataValue:        jev.Event.Data,
		MetadataValue:    jev.Event.Metadata,
	}
	
	return sync.EventWithVersion{
		Event:   event,
		Version: version,
	}, nil
}

// --- HTTP Transport Client ---

// HTTPTransport implements the sync.Transport interface for communicating over HTTP.
type HTTPTransport struct {
	client  *http.Client
	baseURL string // e.g., "http://remote-server.com/sync"
}

// NewTransport creates a new HTTPTransport client.
// If a custom http.Client is not provided, http.DefaultClient will be used.
func NewTransport(baseURL string, client *http.Client) *HTTPTransport {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPTransport{
		client:  client,
		baseURL: baseURL,
	}
}

// Push sends a batch of events to the remote server via an HTTP POST request.
func (t *HTTPTransport) Push(ctx context.Context, events []sync.EventWithVersion) error {
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
func (t *HTTPTransport) Pull(ctx context.Context, since sync.Version) ([]sync.EventWithVersion, error) {
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

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(resp.Body).Decode(&jsonEvents); err != nil {
	return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to decode response: %w", err))
	}

	events := make([]sync.EventWithVersion, len(jsonEvents))
	for i, jev := range jsonEvents {
		// For client-side decoding, we use SQLite's ParseVersion directly
		// since the HTTP transport client doesn't have access to an EventStore
		version, err := sqlite.ParseVersion(jev.Version)
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
		
		events[i] = sync.EventWithVersion{
			Event:   event,
			Version: version,
		}
	}

	return events, nil
}

// Subscribe is not supported by this simple HTTP transport.
// Real-time subscriptions would require WebSockets or gRPC streams.
func (t *HTTPTransport) Subscribe(ctx context.Context, handler func([]sync.EventWithVersion) error) error {
	return syncErrors.New(syncErrors.OpTransport, fmt.Errorf("subscribe is not implemented for HTTP transport"))
}

// Close does nothing for this transport, as the underlying http.Client is managed externally.
func (t *HTTPTransport) Close() error {
	return nil
}

// --- HTTP Sync Handler (Server) ---

// SyncHandler is an http.Handler that serves sync requests.
type SyncHandler struct {
	store  sync.EventStore
	logger *log.Logger
}

// NewSyncHandler creates a new handler for serving sync endpoints.
// It requires an EventStore to interact with the database.
func NewSyncHandler(store sync.EventStore, logger *log.Logger) *SyncHandler {
	return &SyncHandler{
		store:  store,
		logger: logger,
	}
}

// ServeHTTP routes requests to the appropriate handler (/push or /pull).
func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/push":
		h.handlePush(w, r)
	case "/pull":
		h.handlePull(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *SyncHandler) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(r.Body).Decode(&jsonEvents); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	for _, jev := range jsonEvents {
		ev, err := fromJSONEventWithVersion(r.Context(), h.store, jev)
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
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SyncHandler) handlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		sinceStr = "0"
	}

	// Use the EventStore's ParseVersion method to handle version parsing
	// This decouples the transport from specific version implementations
	version, err := h.store.ParseVersion(r.Context(), sinceStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid 'since' version: "+err.Error())
		return
	}

	events, err := h.store.Load(r.Context(), version)
	if err != nil {
		h.logger.Printf("Error loading events from store: %v", err)
		respondWithError(w, http.StatusInternalServerError, "could not load events")
		return
	}

	// Convert events to JSON format for response
	jsonEvents := make([]JSONEventWithVersion, len(events))
	for i, ev := range events {
		jsonEvents[i] = toJSONEventWithVersion(ev)
	}

	h.logger.Printf("Pulled %d events since version %s", len(events), sinceStr)
	respondWithJSON(w, http.StatusOK, jsonEvents)
}

// --- HTTP Helper Functions ---

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		// Fallback if payload marshaling fails
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to marshal response"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

