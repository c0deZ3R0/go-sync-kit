package httptransport

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// MockEvent implements the synckit.Event interface for testing
type MockEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (m *MockEvent) ID() string                               { return m.id }
func (m *MockEvent) Type() string                             { return m.eventType }
func (m *MockEvent) AggregateID() string                      { return m.aggregateID }
func (m *MockEvent) Data() interface{}                        { return m.data }
func (m *MockEvent) Metadata() map[string]interface{}         { return m.metadata }

// MockEventStore implements a simple in-memory event store for testing
type MockEventStore struct {
	events []synckit.EventWithVersion
	mux    sync.RWMutex
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{}
}

func (m *MockEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	// Mock server-side version assignment
	var seq uint64 = 1
	if len(m.events) > 0 {
		if lastVersion, ok := m.events[len(m.events)-1].Version.(cursor.IntegerCursor); ok {
			seq = lastVersion.Seq + 1
		}
	}

	m.events = append(m.events, synckit.EventWithVersion{
		Event:   event,
		Version: cursor.IntegerCursor{Seq: seq},
	})
	return nil
}

func (m *MockEventStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	var result []synckit.EventWithVersion
	for _, ev := range m.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *MockEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	var result []synckit.EventWithVersion
	for _, ev := range m.events {
		if ev.Event.AggregateID() == aggregateID && ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *MockEventStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	if len(m.events) == 0 {
		return cursor.IntegerCursor{Seq: 0}, nil
	}
	return m.events[len(m.events)-1].Version, nil
}

func (m *MockEventStore) ParseVersion(ctx context.Context, s string) (synckit.Version, error) {
	seq, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return cursor.IntegerCursor{Seq: seq}, nil
}

func (m *MockEventStore) Close() error {
	return nil
}

func setupTestStore(t *testing.T) (*MockEventStore, func()) {
	store := NewMockEventStore()
	cleanup := func() {}
	return store, cleanup
}

func TestHTTPTransport_NewTransport(t *testing.T) {
// Test with default client
transport := NewTransport("http://example.com", http.DefaultClient, nil, DefaultClientOptions())
	if transport.client != http.DefaultClient {
		t.Error("Expected default client when nil is provided")
	}
	if transport.baseURL != "http://example.com" {
		t.Errorf("Expected baseURL 'http://example.com', got '%s'", transport.baseURL)
	}

	// Test with custom client
customClient := &http.Client{Timeout: 5 * time.Second}
transport = NewTransport("http://custom.com", customClient, nil, DefaultClientOptions())
	if transport.client != customClient {
		t.Error("Expected custom client to be used")
	}
}

func TestHTTPTransport_Push_EmptyEvents(t *testing.T) {
transport := NewTransport("http://example.com", http.DefaultClient, nil, DefaultClientOptions())
	
	ctx := context.Background()
	err := transport.Push(ctx, []synckit.EventWithVersion{})
	
	if err != nil {
		t.Errorf("Expected no error for empty events, got: %v", err)
	}
}

func TestHTTPTransport_RequestSizeLimit(t *testing.T) {
	// Create a large event payload that exceeds the size limit
	longString := strings.Repeat("x", 10*1024*1024+1) // 10MB + 1 byte
	events := []synckit.EventWithVersion{
		{
			Event: &SimpleEvent{
				IDValue:          "1",
				TypeValue:        "test",
				AggregateIDValue: "agg1",
				DataValue:        longString,
			},
		},
	}

	// Create server with default options (10MB limit)
	handler := NewSyncHandler(
		NewMockEventStore(),
		log.New(os.Stderr, "[test] ", log.LstdFlags),
		nil,
		DefaultServerOptions(),
	)

	// Create test server
	s := httptest.NewServer(handler)
	defer s.Close()

	// Create client
	transport := NewTransport(s.URL, nil, nil, DefaultClientOptions())

	// Try to push the large event
	err := transport.Push(context.Background(), events)

	// Should get a 413 Request Entity Too Large error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "413")
}

func TestHTTPTransport_Compression(t *testing.T) {
	// Create a payload that should trigger compression
	longString := strings.Repeat("test data ", 1000) // ~9KB
	events := []synckit.EventWithVersion{
		{
			Event: &SimpleEvent{
				IDValue:          "1",
				TypeValue:        "test",
				AggregateIDValue: "agg1",
				DataValue:        longString,
			},
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	// Create handler with compression enabled
	handler := NewSyncHandler(
		NewMockEventStore(),
		log.New(os.Stderr, "[test] ", log.LstdFlags),
		nil,
		&ServerOptions{
			MaxRequestSize:       10 * 1024 * 1024,
			CompressionEnabled:   true,
			CompressionThreshold: 1024, // 1KB
		},
	)

	// Create test server
	s := httptest.NewServer(handler)
	defer s.Close()

	// Create client with compression enabled and appropriate size limits
	transport := NewTransport(s.URL, nil, nil, &ClientOptions{
		CompressionEnabled: true,
		MaxResponseSize: 10 * 1024 * 1024, // 10MB
		MaxDecompressedResponseSize: 20 * 1024 * 1024, // 20MB
	})

// Store events first
	err := transport.Push(context.Background(), events)
	require.NoError(t, err)

	// Pull events
	fetched, err := transport.Pull(context.Background(), cursor.IntegerCursor{Seq: 0})

	// Should succeed and get the same data back
	require.NoError(t, err)
	assert.Equal(t, len(events), len(fetched))
	// Compare only the event data, not version since server assigns its own
	fetchedEvent, ok := fetched[0].Event.(*SimpleEvent)
	require.True(t, ok, "Expected fetched event to be *SimpleEvent")
	originalEvent := events[0].Event.(*SimpleEvent)
	assert.Equal(t, originalEvent.DataValue, fetchedEvent.DataValue)
	assert.Equal(t, originalEvent.IDValue, fetchedEvent.IDValue)
	assert.Equal(t, originalEvent.TypeValue, fetchedEvent.TypeValue)
	assert.Equal(t, originalEvent.AggregateIDValue, fetchedEvent.AggregateIDValue)
}

func testHelperRespondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to marshal response"}`)) 
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func TestHTTPTransport_Push(t *testing.T) {
	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/push" {
			t.Errorf("Expected /push path, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json content type, got %s", r.Header.Get("Content-Type"))
		}
		
		// Verify request body
		var events []JSONEventWithVersion
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

transport := NewTransport(server.URL, http.DefaultClient, nil, DefaultClientOptions())
	
	events := []synckit.EventWithVersion{
		{
			Event: &MockEvent{
				id:          "test-1",
				eventType:   "TestEvent",
				aggregateID: "agg-1",
				data:        "test data",
			},
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	ctx := context.Background()
	err := transport.Push(ctx, events)
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestHTTPTransport_Push_ServerError(t *testing.T) {
	// Create a test server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

transport := NewTransport(server.URL, http.DefaultClient, nil, DefaultClientOptions())
	
	events := []synckit.EventWithVersion{
		{
			Event: &MockEvent{id: "test-1"},
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	ctx := context.Background()
	err := transport.Push(ctx, events)
	
	if err == nil {
		t.Error("Expected error for server error response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected error to contain status code 500, got: %v", err)
	}
}

func TestHTTPTransport_Pull_Success(t *testing.T) {
	expectedEvents := []synckit.EventWithVersion{
		{
			Event: &MockEvent{
				id:          "test-1",
				eventType:   "TestEvent",
				aggregateID: "agg-1",
				data:        "test data",
			},
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	// Create a test server that returns events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/pull" {
			t.Errorf("Expected /pull path, got %s", r.URL.Path)
		}
		
		since := r.URL.Query().Get("since")
		if since != "0" {
			t.Errorf("Expected since=0, got since=%s", since)
		}
		
		// Convert to JSON format for response
		jsonEvents := make([]JSONEventWithVersion, len(expectedEvents))
		for i, ev := range expectedEvents {
			jsonEvents[i] = JSONEventWithVersion{
				Event: JSONEvent{
					ID:          ev.Event.ID(),
					Type:        ev.Event.Type(),
					AggregateID: ev.Event.AggregateID(),
					Data:        ev.Event.Data(),
					Metadata:    ev.Event.Metadata(),
				},
				Version: ev.Version.String(),
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(jsonEvents)
	}))
	defer server.Close()

transport := NewTransport(server.URL, http.DefaultClient, nil, DefaultClientOptions())
	
	ctx := context.Background()
	events, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}

func TestHTTPTransport_Subscribe_NotImplemented(t *testing.T) {
transport := NewTransport("http://example.com", http.DefaultClient, nil, DefaultClientOptions())
	
	ctx := context.Background()
	err := transport.Subscribe(ctx, func([]synckit.EventWithVersion) error { return nil })
	
	if err == nil {
		t.Error("Expected error for unimplemented Subscribe method")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not implemented' error, got: %v", err)
	}
}

func TestHTTPTransport_Close(t *testing.T) {
transport := NewTransport("http://example.com", http.DefaultClient, nil, DefaultClientOptions())
	
	err := transport.Close()
	
	if err != nil {
		t.Errorf("Expected no error from Close, got: %v", err)
	}
}

func TestSyncHandler_NewSyncHandler_WithDefaultParser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	
	// Test with default parser
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())

	if handler.store != store {
		t.Error("Expected store to be set correctly")
	}
	if handler.logger != logger {
		t.Error("Expected logger to be set correctly")
	}

	// Verify default parser uses store's ParseVersion
	ctx := context.Background()
	version, err := handler.versionParser(ctx, "1")
	if err != nil {
		t.Errorf("Expected no error from default parser, got: %v", err)
	}

	// Verify parsed version matches store's ParseVersion result
	expectedVersion, err := store.ParseVersion(ctx, "1")
	if err != nil {
		t.Errorf("Expected no error from store ParseVersion, got: %v", err)
	}

	if version.String() != expectedVersion.String() {
		t.Errorf("Expected version %v, got %v", expectedVersion, version)
	}
}

func TestSyncHandler_NewSyncHandler_WithCustomParser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Create a custom parser that always returns version 42
	customParser := func(ctx context.Context, s string) (synckit.Version, error) {
		return cursor.IntegerCursor{Seq: 42}, nil
	}

	// Test with custom parser
handler := NewSyncHandler(store, logger, customParser, DefaultServerOptions())

	if handler.store != store {
		t.Error("Expected store to be set correctly")
	}
	if handler.logger != logger {
		t.Error("Expected logger to be set correctly")
	}
	if handler.versionParser == nil {
		t.Error("Expected custom version parser to be set")
	}

	// Test that custom parser is used and ignores input
	ctx := context.Background()
	version, err := handler.versionParser(ctx, "any")
	if err != nil {
		t.Errorf("Expected no error from custom parser, got: %v", err)
	}
	if v, ok := version.(cursor.IntegerCursor); !ok || v.Seq != 42 {
		t.Errorf("Expected version to be IntegerCursor{42}, got: %v", version)
	}
}

func TestSyncHandler_HandlePush_MethodNotAllowed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	
	req := httptest.NewRequest(http.MethodGet, "/push", nil)
	w := httptest.NewRecorder()
	
	handler.handlePush(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestSyncHandler_HandlePush_InvalidJSON(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler.handlePush(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSyncHandler_HandlePull_Success(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	// First, store some events
	ctx := context.Background()
	event := &MockEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "test data",
	}
	store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
	
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	
	req := httptest.NewRequest(http.MethodGet, "/pull?since=0", nil)
	w := httptest.NewRecorder()
	
	handler.handlePull(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(w.Body).Decode(&jsonEvents); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
	
	if len(jsonEvents) != 1 {
		t.Errorf("Expected 1 event, got %d", len(jsonEvents))
	}
}

func TestSyncHandler_HandlePull_MethodNotAllowed(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	
	req := httptest.NewRequest(http.MethodPost, "/pull", nil)
	w := httptest.NewRecorder()
	
	handler.handlePull(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestSyncHandler_HandlePull_WithCustomParser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Create a custom parser that validates version format
	customParser := func(ctx context.Context, s string) (synckit.Version, error) {
		if !strings.HasPrefix(s, "v") {
			return nil, fmt.Errorf("version must start with 'v'")
		}
		// Strip 'v' prefix and parse as integer
		seq, err := strconv.ParseUint(s[1:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid version number: %w", err)
		}
		return cursor.IntegerCursor{Seq: seq}, nil
	}

	// Create handler with custom parser
handler := NewSyncHandler(store, logger, customParser, DefaultServerOptions())

	// Test valid version format
	req := httptest.NewRequest(http.MethodGet, "/pull?since=v1", nil)
	w := httptest.NewRecorder()
	handler.handlePull(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for valid version, got %d", w.Code)
	}

	// Test invalid version format
	req = httptest.NewRequest(http.MethodGet, "/pull?since=1", nil)
	w = httptest.NewRecorder()
	handler.handlePull(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid version format, got %d", w.Code)
	}

	// Verify error message
	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
	if !strings.Contains(response["error"], "version must start with 'v'") {
		t.Errorf("Expected error message about version format, got: %s", response["error"])
	}
}

func TestSyncHandler_HandlePull_InvalidVersion(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	
	req := httptest.NewRequest(http.MethodGet, "/pull?since=invalid", nil)
	w := httptest.NewRecorder()
	
	handler.handlePull(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSyncHandler_ServeHTTP_Routing(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	
	// Test /push route
	req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader("[]"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected /push to be handled, got status %d", w.Code)
	}
	
	// Test /pull route
	req = httptest.NewRequest(http.MethodGet, "/pull", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected /pull to be handled, got status %d", w.Code)
	}
	
	// Test unknown route
	req = httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown route, got status %d", w.Code)
	}
}

func TestEndToEnd_HTTPTransportWithSyncHandler(t *testing.T) {
	// Set up the server
	store, cleanup := setupTestStore(t)
	defer cleanup()
	
	logger := log.New(os.Stdout, "[E2E] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := NewSyncHandler(store, logger, nil, DefaultServerOptions())
	server := httptest.NewServer(handler)
	defer server.Close()
	
	// Set up the client
transport := NewTransport(server.URL, http.DefaultClient, nil, DefaultClientOptions())
	
	// Create test events
	events := []synckit.EventWithVersion{
		{
			Event: &MockEvent{
				id:          "e2e-test-1",
				eventType:   "E2EEvent",
				aggregateID: "e2e-agg-1",
				data:        map[string]string{"key": "value"},
				metadata:    map[string]interface{}{"source": "test"},
			},
			Version: cursor.IntegerCursor{Seq: 1},
		},
		{
			Event: &MockEvent{
				id:          "e2e-test-2",
				eventType:   "E2EEvent",
				aggregateID: "e2e-agg-1",
				data:        map[string]string{"key2": "value2"},
			},
			Version: cursor.IntegerCursor{Seq: 2},
		},
	}
	
	ctx := context.Background()
	
	// Test Push
	err := transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}
	
	// Test Pull
	pulledEvents, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to pull events: %v", err)
	}
	
	if len(pulledEvents) != 2 {
		t.Errorf("Expected 2 events, got %d", len(pulledEvents))
	}
	
	// Verify event data
	if pulledEvents[0].Event.ID() != "e2e-test-1" {
		t.Errorf("Expected first event ID 'e2e-test-1', got '%s'", pulledEvents[0].Event.ID())
	}
	if pulledEvents[1].Event.ID() != "e2e-test-2" {
		t.Errorf("Expected second event ID 'e2e-test-2', got '%s'", pulledEvents[1].Event.ID())
	}
}

func BenchmarkHTTPTransport_Push(b *testing.B) {
	// Set up a simple test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
transport := NewTransport(server.URL, http.DefaultClient, nil, DefaultClientOptions())
	
	events := []synckit.EventWithVersion{
		{
			Event: &MockEvent{
				id:          "bench-test-1",
				eventType:   "BenchEvent",
				aggregateID: "bench-agg-1",
				data:        "benchmark data",
			},
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}
	
	ctx := context.Background()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		events[0].Event.(*MockEvent).id = fmt.Sprintf("bench-test-%d", i)
		err := transport.Push(ctx, events)
		if err != nil {
			b.Fatalf("Push failed: %v", err)
		}
	}
}

func BenchmarkHTTPTransport_Pull(b *testing.B) {
	// Set up a test server that returns events
events := []synckit.EventWithVersion{
		{
			Event: &MockEvent{
				id:          "bench-test-1",
				eventType:   "BenchEvent",
				aggregateID: "bench-agg-1",
				data:        "benchmark data",
			},
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(events)
	}))
	defer server.Close()
	
transport := NewTransport(server.URL, http.DefaultClient, nil, DefaultClientOptions())
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Pull failed: %v", err)
		}
	}
}

// Test compression size limits
func TestSyncHandler_CompressionSizeLimits(t *testing.T) {
	store := NewMockEventStore()
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)

	// Create server with small limits for testing
	opts := &ServerOptions{
		MaxRequestSize:      1024,  // 1KB compressed limit
		MaxDecompressedSize: 2048,  // 2KB decompressed limit
		CompressionEnabled:  true,
		CompressionThreshold: 100,
	}
	handler := NewSyncHandler(store, logger, nil, opts)

	t.Run("CompressedSizeExceedsLimit", func(t *testing.T) {
		// Let's create multiple large events that together exceed the compressed size limit
		events := make([]JSONEventWithVersion, 50)
		for i := 0; i < 50; i++ {
			// Create somewhat random data for each event
			eventData := fmt.Sprintf("Event_%d_with_random_data_%d", i, i*12345)
			events[i] = JSONEventWithVersion{
				Event: JSONEvent{
					ID:          fmt.Sprintf("event-%d", i),
					Type:        fmt.Sprintf("EventType%d", i%5),
					AggregateID: fmt.Sprintf("aggregate-%d", i%10),
					Data:        eventData + strings.Repeat(fmt.Sprintf("_%d", i), 50),
					Metadata: map[string]interface{}{
						"timestamp": fmt.Sprintf("2023-01-01T%02d:%02d:00Z", i%24, i%60),
						"source":    fmt.Sprintf("service-%d", i%3),
					},
				},
				Version: fmt.Sprintf("%d", i+1),
			}
		}

		data, err := json.Marshal(events)
		require.NoError(t, err)

		// Compress the data
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(data)
		require.NoError(t, err)
		gz.Close()

		// Verify compressed size exceeds our limit
		compressedSize := buf.Len()
		t.Logf("Compressed size: %d bytes (should exceed %d)", compressedSize, opts.MaxRequestSize)
		require.Greater(t, int64(compressedSize), opts.MaxRequestSize, "Compressed size should exceed limit")

		// Make request with compressed data that exceeds limit
		req := httptest.NewRequest(http.MethodPost, "/push", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.ContentLength = int64(buf.Len()) // Set the content length
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Should get 413 Request Entity Too Large
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("DecompressedSizeExceedsLimit", func(t *testing.T) {
		// Create a payload that compresses well but exceeds decompressed limit
		longString := strings.Repeat("a", 3000) // 3KB when decompressed, but compresses very well
		events := []JSONEventWithVersion{
			{
				Event: JSONEvent{
					ID:          "1",
					Type:        "test",
					AggregateID: "agg1",
					Data:        longString,
				},
				Version: "1",
			},
		}

		data, err := json.Marshal(events)
		require.NoError(t, err)

		// Compress the data (should compress very well)
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(data)
		require.NoError(t, err)
		gz.Close()

		// Verify compressed size is under MaxRequestSize but decompressed exceeds MaxDecompressedSize
		compressedSize := buf.Len()
		t.Logf("Compressed size: %d bytes, Decompressed size: %d bytes", compressedSize, len(data))
		require.Less(t, int64(compressedSize), opts.MaxRequestSize, "Compressed size should be under limit")
		require.Greater(t, int64(len(data)), opts.MaxDecompressedSize, "Decompressed size should exceed limit")

		// Make request
		req := httptest.NewRequest(http.MethodPost, "/push", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Should get 413 Request Entity Too Large for decompressed size limit exceeded
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("ValidCompressedRequest", func(t *testing.T) {
		// Create a small payload that fits within both limits
		events := []JSONEventWithVersion{
			{
				Event: JSONEvent{
					ID:          "1",
					Type:        "test",
					AggregateID: "agg1",
					Data:        "small data",
				},
				Version: "1",
			},
		}

		data, err := json.Marshal(events)
		require.NoError(t, err)

		// Compress the data
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(data)
		require.NoError(t, err)
		gz.Close()

		// Make request
		req := httptest.NewRequest(http.MethodPost, "/push", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Should succeed
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidGzipPayload", func(t *testing.T) {
		// Send invalid gzip data
		req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader("invalid gzip data"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Should get 400 Bad Request
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid request body")
	})
}

// Test cursor API size limits
func TestSyncHandler_CursorAPICompressionSizeLimits(t *testing.T) {
	store := NewMockEventStore()
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)

	// Create server with small limits for testing
	opts := &ServerOptions{
		MaxRequestSize:      1024,  // 1KB compressed limit
		MaxDecompressedSize: 2048,  // 2KB decompressed limit
		CompressionEnabled:  true,
		CompressionThreshold: 100,
	}
	handler := NewSyncHandler(store, logger, nil, opts)

	t.Run("CursorAPI_DecompressedSizeExceedsLimit", func(t *testing.T) {
		// Create a payload that compresses well but exceeds decompressed limit
		longString := strings.Repeat("a", 3000) // 3KB when decompressed, but compresses very well
		// Use a simple request structure that will generate large JSON when marshaled
		cursorRequest := map[string]interface{}{
			"since": nil,
			"limit": 100,
			"large_data": longString,  // Add the large data field
		}

		data, err := json.Marshal(cursorRequest)
		require.NoError(t, err)

		// Compress the data (should compress very well)
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(data)
		require.NoError(t, err)
		gz.Close()

		// Verify compressed size is under MaxRequestSize but decompressed exceeds MaxDecompressedSize
		compressedSize := buf.Len()
		t.Logf("Cursor API - Compressed size: %d bytes, Decompressed size: %d bytes", compressedSize, len(data))
		require.Less(t, int64(compressedSize), opts.MaxRequestSize, "Compressed size should be under limit")
		require.Greater(t, int64(len(data)), opts.MaxDecompressedSize, "Decompressed size should exceed limit")

		// Make request to cursor API endpoint
		req := httptest.NewRequest(http.MethodPost, "/cursor/pull", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()

		handler.handlePullCursor(w, req)

		// Should get 413 Request Entity Too Large for decompressed size limit exceeded
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("CursorAPI_ValidCompressedRequest", func(t *testing.T) {
		// Create a small payload that fits within both limits
		cursorRequest := map[string]interface{}{
			"since": nil,
			"limit": 100,
			"small_data": "small data",
		}

		data, err := json.Marshal(cursorRequest)
		require.NoError(t, err)

		// Compress the data
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(data)
		require.NoError(t, err)
		gz.Close()

		// Make request to cursor API endpoint
		req := httptest.NewRequest(http.MethodPost, "/cursor/pull", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()

		handler.handlePullCursor(w, req)

		// Should succeed
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
