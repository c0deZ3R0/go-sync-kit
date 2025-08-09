package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func TestStoreContextCancellation(t *testing.T) {
	store, _ := NewWithDataSource(":memory:")
	defer store.Close()

	event := &MockEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        map[string]string{"key": "value"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel the context

	err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

// MockEvent implements the sync.Event interface for testing
type MockEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (m *MockEvent) ID() string                       { return m.id }
func (m *MockEvent) Type() string                     { return m.eventType }
func (m *MockEvent) AggregateID() string              { return m.aggregateID }
func (m *MockEvent) Data() interface{}                { return m.data }
func (m *MockEvent) Metadata() map[string]interface{} { return m.metadata }

func setupTestDB(t *testing.T) (*SQLiteEventStore, func()) {
	// Create a temporary database file
	tempFile, err := os.CreateTemp("", "test_db_*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()

	store, err := NewWithDataSource(tempFile.Name())
	if err != nil {
		os.Remove(tempFile.Name())
		t.Fatalf("Failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tempFile.Name())
	}

	return store, cleanup
}

func TestIntegerVersion(t *testing.T) {
	v1 := cursor.IntegerCursor{Seq: 1}
	v2 := cursor.IntegerCursor{Seq: 2}
	v3 := cursor.IntegerCursor{Seq: 1}

	// Test Compare
	if v1.Compare(v2) != -1 {
		t.Errorf("Expected v1 < v2, got %d", v1.Compare(v2))
	}
	if v2.Compare(v1) != 1 {
		t.Errorf("Expected v2 > v1, got %d", v2.Compare(v1))
	}
	if v1.Compare(v3) != 0 {
		t.Errorf("Expected v1 == v3, got %d", v1.Compare(v3))
	}

	// Test String
	if v1.String() != "1" {
		t.Errorf("Expected v1.String() == '1', got '%s'", v1.String())
	}

	// Test IsZero
	zero := cursor.IntegerCursor{Seq: 0}
	if !zero.IsZero() {
		t.Error("Expected zero version to be zero")
	}
	if v1.IsZero() {
		t.Error("Expected non-zero version to not be zero")
	}
}

func TestSQLiteEventStore_Store(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	event := &MockEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        map[string]string{"key": "value"},
		metadata:    map[string]interface{}{"source": "test"},
	}

	version := cursor.IntegerCursor{Seq: 1}

	err := store.Store(ctx, event, version)
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Verify we can retrieve the latest version
	latest, err := store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if latest.Compare(cursor.IntegerCursor{Seq: 1}) != 0 {
		t.Errorf("Expected latest version to be 1, got %s", latest.String())
	}
}

func TestSQLiteEventStore_Load(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Store multiple events
	events := []*MockEvent{
		{
			id:          "test-1",
			eventType:   "TestEvent",
			aggregateID: "agg-1",
			data:        "data1",
		},
		{
			id:          "test-2",
			eventType:   "TestEvent",
			aggregateID: "agg-1",
			data:        "data2",
		},
		{
			id:          "test-3",
			eventType:   "TestEvent",
			aggregateID: "agg-2",
			data:        "data3",
		},
	}

	for _, event := range events {
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event %s: %v", event.ID(), err)
		}
	}

	// Load all events
	allEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events: %v", err)
	}

	if len(allEvents) != 3 {
		t.Errorf("Expected 3 events, got %d", len(allEvents))
	}

	// Load events since version 1
	recentEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 1})
	if err != nil {
		t.Fatalf("Failed to load recent events: %v", err)
	}

	if len(recentEvents) != 2 {
		t.Errorf("Expected 2 recent events, got %d", len(recentEvents))
	}
}

func TestSQLiteEventStore_LoadByAggregate(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Store events for different aggregates
	events := []*MockEvent{
		{
			id:          "test-1",
			eventType:   "TestEvent",
			aggregateID: "agg-1",
			data:        "data1",
		},
		{
			id:          "test-2",
			eventType:   "TestEvent",
			aggregateID: "agg-1",
			data:        "data2",
		},
		{
			id:          "test-3",
			eventType:   "TestEvent",
			aggregateID: "agg-2",
			data:        "data3",
		},
	}

	for _, event := range events {
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event %s: %v", event.ID(), err)
		}
	}

	// Load events for agg-1
	agg1Events, err := store.LoadByAggregate(ctx, "agg-1", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events for agg-1: %v", err)
	}

	if len(agg1Events) != 2 {
		t.Errorf("Expected 2 events for agg-1, got %d", len(agg1Events))
	}

	// Load events for agg-2
	agg2Events, err := store.LoadByAggregate(ctx, "agg-2", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events for agg-2: %v", err)
	}

	if len(agg2Events) != 1 {
		t.Errorf("Expected 1 event for agg-2, got %d", len(agg2Events))
	}

	// Verify aggregate ID is correct
	for _, evt := range agg1Events {
		if evt.Event.AggregateID() != "agg-1" {
			t.Errorf("Expected aggregate ID 'agg-1', got '%s'", evt.Event.AggregateID())
		}
	}
}

func TestSQLiteEventStore_LatestVersion(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Initially, latest version should be 0
	latest, err := store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if !latest.IsZero() {
		t.Errorf("Expected initial version to be zero, got %s", latest.String())
	}

	// Store an event
	event := &MockEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "data",
	}

	err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Latest version should now be 1
	latest, err = store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if latest.Compare(cursor.IntegerCursor{Seq: 1}) != 0 {
		t.Errorf("Expected latest version to be 1, got %s", latest.String())
	}
}

func TestSQLiteEventStore_Close(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer func() {
		// Don't call cleanup() as we're testing Close() explicitly
		// We'll manually clean up if needed
		_ = cleanup // Avoid unused variable warning
	}()

	ctx := context.Background()

	// Close the store
	err := store.Close()
	if err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Subsequent operations should fail
	_, err = store.LatestVersion(ctx)
	if err != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed, got %v", err)
	}

	// Closing again should be safe
	err = store.Close()
	if err != nil {
		t.Errorf("Expected no error on second close, got %v", err)
	}
}

func TestSQLiteEventStore_Config(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_config_*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	config := &Config{
		DataSourceName:  tempFile.Name(),
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}

	store, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create store with config: %v", err)
	}
	defer store.Close()

	// Test that we can get stats (which indicates connection pool is working)
	stats := store.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("Expected MaxOpenConnections to be 10, got %d", stats.MaxOpenConnections)
	}
}

func TestSQLiteEventStore_WithLogging(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_logging_*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Create a buffer to capture log output
	var logBuffer bytes.Buffer
	logger := log.New(&logBuffer, "TEST: ", log.LstdFlags)

	config := &Config{
		DataSourceName: tempFile.Name(),
		Logger:         logger,
		TableName:      "custom_events",
	}

	store, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create store with logging: %v", err)
	}
	defer store.Close()

	// Check that log messages were written
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "Opening database") {
		t.Error("Expected 'Opening database' in log output")
	}
	if !strings.Contains(logOutput, "Connection pool configured") {
		t.Error("Expected 'Connection pool configured' in log output")
	}
	if !strings.Contains(logOutput, "Successfully initialized with table: custom_events") {
		t.Error("Expected 'Successfully initialized' in log output")
	}
}

// Removed this test as it doesn't work with non-interface types

// This won't work as written because incompatibleVersion doesn't implement sync.Version
// Let's create a proper test:

type IncompatibleVersion struct {
	value int
}

func (v IncompatibleVersion) Compare(other synckit.Version) int { return 0 }
func (v IncompatibleVersion) String() string                    { return "incompatible" }
func (v IncompatibleVersion) IsZero() bool                      { return v.value == 0 }

func TestSQLiteEventStore_IncompatibleVersionProper(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to load with an incompatible version type
	incompatibleVersion := IncompatibleVersion{value: 1}

	_, err := store.Load(ctx, incompatibleVersion)
	if err != ErrIncompatibleVersion {
		t.Errorf("Expected ErrIncompatibleVersion, got %v", err)
	}

	_, err = store.LoadByAggregate(ctx, "test", incompatibleVersion)
	if err != ErrIncompatibleVersion {
		t.Errorf("Expected ErrIncompatibleVersion, got %v", err)
	}
}

func TestParseVersion(t *testing.T) {
	// Test valid version strings
	version, err := ParseVersion("42")
	if err != nil {
		t.Errorf("Expected no error for valid version string, got: %v", err)
	}
	if version != (cursor.IntegerCursor{Seq: 42}) {
		t.Errorf("Expected version 42, got %v", version)
	}

	// Test zero version
	version, err = ParseVersion("0")
	if err != nil {
		t.Errorf("Expected no error for zero version, got: %v", err)
	}
	if version != (cursor.IntegerCursor{Seq: 0}) {
		t.Errorf("Expected version 0, got %v", version)
	}

	// Test empty string
	version, err = ParseVersion("")
	if err != nil {
		t.Errorf("Expected no error for empty string, got: %v", err)
	}
	if version != (cursor.IntegerCursor{Seq: 0}) {
		t.Errorf("Expected version 0 for empty string, got %v", version)
	}

	// Test invalid string
	_, err = ParseVersion("invalid")
	if err == nil {
		t.Error("Expected error for invalid version string")
	}
	if !strings.Contains(err.Error(), "invalid version string") {
		t.Errorf("Expected error message to contain 'invalid version string', got: %v", err)
	}

	// Test negative version
	version, err = ParseVersion("-1")
	if err != nil {
		t.Errorf("Expected no error for negative version, got: %v", err)
	}
	if version != (cursor.IntegerCursor{Seq: 0xFFFFFFFFFFFFFFFF}) {
		t.Errorf("Expected version -1, got %v", version)
	}
}

func TestSQLiteEventStore_WALMode(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-events-wal-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Test with EnableWAL=true
	config := &Config{
		DataSourceName: tempFile.Name(),
		EnableWAL:      true,
	}

	store, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create store with WAL enabled: %v", err)
	}
	defer store.Close()

	// Verify that WAL mode was applied to the connection string
	if !strings.Contains(config.DataSourceName, "_journal_mode=WAL") {
		t.Errorf("Expected DataSourceName to contain '_journal_mode=WAL', got: %s", config.DataSourceName)
	}

	// Test basic functionality with WAL mode
	ctx := context.Background()
	event := &MockEvent{
		id:          "wal-test-1",
		eventType:   "TestEvent",
		aggregateID: "aggregate-1",
		data:        "wal-test-data",
	}

	if err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 1}); err != nil {
		t.Fatalf("Failed to store event in WAL mode: %v", err)
	}

	events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events in WAL mode: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}

func BenchmarkSQLiteEventStore_Store(b *testing.B) {
	store, cleanup := setupTestDB(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	event := &MockEvent{
		id:          "bench-event",
		eventType:   "BenchEvent",
		aggregateID: "bench-agg",
		data:        "benchmark data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.id = fmt.Sprintf("bench-event-%d", i)
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Failed to store event: %v", err)
		}
	}
}

func BenchmarkSQLiteEventStore_Load(b *testing.B) {
	store, cleanup := setupTestDB(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with events
	for i := 0; i < 1000; i++ {
		event := &MockEvent{
			id:          fmt.Sprintf("event-%d", i),
			eventType:   "BenchEvent",
			aggregateID: "bench-agg",
			data:        fmt.Sprintf("data-%d", i),
		}
		store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Failed to load events: %v", err)
		}
	}
}
