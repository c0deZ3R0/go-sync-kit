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
	"net/http/httptest"
	"os"
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

	m.events = append(m.events, synckit.EventWithVersion{Event: event, Version: version})
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
	transport := NewTransport("http://example.com", nil, nil)
	if transport.client != http.DefaultClient {
		t.Error("Expected default client when nil is provided")
	}
	if transport.baseURL != "http://example.com" {
		t.Errorf("Expected baseURL 'http://example.com', got '%s'", transport.baseURL)
	}

	// Test with custom client
customClient := &http.Client{Timeout: 5 * time.Second}
	transport = NewTransport("http://custom.com", customClient, nil)
	if transport.client != customClient {
		t.Error("Expected custom client to be used")
	}
}

func TestHTTPTransport_Push_EmptyEvents(t *testing.T) {
	transport := NewTransport("http://example.com", nil, nil)
	
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
	server := httptest.NewServer(NewSyncHandler(
		NewMockEventStore(),
		log.New(os.Stderr, "[test] ", log.LstdFlags),
		nil,
		DefaultServerOptions(),
	))
	defer server.Close()

	// Create client
	transport := NewTransport(server.URL, nil, nil, DefaultClientOptions())

	// Try to push the large event
	err := transport.Push(context.Background(), events)

	// Should get a 413 Request Entity Too Large error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "413")
}

func TestHTTPTransport_Compression(t *testing.T) {
	// Create a payload that's larger than the compression threshold
	longString := strings.Repeat("test data ", 1000) // ~9KB
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

	// Track if compression was used
	var compressionUsed bool

	// Create test server that checks for compression
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send compressed response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		
		gz := gzip.NewWriter(w)
		defer gz.Close()
		
		response, _ := json.Marshal(events)
		gz.Write(response)
		compressionUsed = true
	}))
	defer server.Close()

	// Create client with compression enabled
	transport := NewTransport(server.URL, nil, nil, &ClientOptions{CompressionEnabled: true})

	// Pull events
	fetched, err := transport.Pull(context.Background(), nil)

	// Verify compression was used and data was correctly decompressed
	require.NoError(t, err)
	assert.True(t, compressionUsed, "Compression should have been used")
	assert.Equal(t, len(events), len(fetched))
	assert.Equal(t, events[0].Event.(*SimpleEvent).DataValue, fetched[0].Event.(*SimpleEvent).DataValue)
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

transport := NewTransport(server.URL, nil, nil)
	
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

transport := NewTransport(server.URL, nil, nil)
	
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

transport := NewTransport(server.URL, nil, nil)
	
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
	transport := NewTransport("http://example.com", nil, nil)
	
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
	transport := NewTransport("http://example.com", nil, nil)
	
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
	handler := NewSyncHandler(store, logger, nil)

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
	handler := NewSyncHandler(store, logger, customParser)

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
handler := NewSyncHandler(store, logger, nil)
	
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
handler := NewSyncHandler(store, logger, nil)
	
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
handler := NewSyncHandler(store, logger, nil)
	
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
handler := NewSyncHandler(store, logger, nil)
	
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
	handler := NewSyncHandler(store, logger, customParser)

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
handler := NewSyncHandler(store, logger, nil)
	
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
handler := NewSyncHandler(store, logger, nil)
	
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
handler := NewSyncHandler(store, logger, nil)
	server := httptest.NewServer(handler)
	defer server.Close()
	
	// Set up the client
transport := NewTransport(server.URL, nil, nil)
	
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
	
transport := NewTransport(server.URL, nil, nil)
	
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
	
transport := NewTransport(server.URL, nil, nil)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Pull failed: %v", err)
		}
	}
}
