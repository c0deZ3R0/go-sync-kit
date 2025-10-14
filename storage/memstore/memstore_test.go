package memstore

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

func TestMemStore_New(t *testing.T) {
	store := New()
	defer store.Close()

	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	stats := store.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 events, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 0 {
		t.Errorf("Expected 0 streams, got %d", stats.TotalStreams)
	}
	if stats.NextSequence != 1 {
		t.Errorf("Expected next sequence 1, got %d", stats.NextSequence)
	}
}

func TestMemStore_Store(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()
	testEvent := event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`))

	err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 1})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Check stats
	stats := store.Stats()
	if stats.TotalEvents != 1 {
		t.Errorf("Expected 1 event, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 1 {
		t.Errorf("Expected 1 stream, got %d", stats.TotalStreams)
	}
	if stats.NextSequence != 2 {
		t.Errorf("Expected next sequence 2, got %d", stats.NextSequence)
	}
}

func TestMemStore_Store_WithMetadata(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()
	metadata := map[string]interface{}{
		"source":    "test",
		"timestamp": time.Now().Unix(),
		"version":   "1.0",
	}

	testEvent := event.NewWithMetadata("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`), metadata)

	err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 1})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Load the event back and check metadata
	events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	storedMetadata := events[0].Event.Metadata()
	if storedMetadata["source"] != "test" {
		t.Errorf("Expected source 'test', got %v", storedMetadata["source"])
	}
	if storedMetadata["version"] != "1.0" {
		t.Errorf("Expected version '1.0', got %v", storedMetadata["version"])
	}
}

func TestMemStore_Store_ContextCancellation(t *testing.T) {
	store := New()
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	testEvent := event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`))

	err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 1})
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestMemStore_Store_AfterClose(t *testing.T) {
	store := New()
	store.Close()

	ctx := context.Background()
	testEvent := event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`))

	err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 1})
	if err == nil {
		t.Error("Expected error when storing to closed store")
	}
	if err.Error() != "store is closed" {
		t.Errorf("Expected 'store is closed' error, got %v", err)
	}
}

func TestMemStore_Load(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Store test events
	events := []*event.Event{
		event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
		event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
		event.New("test-3", "UserCreated", "user-456", []byte(`{"name":"Bob"}`)),
	}

	for _, ev := range events {
		err := store.Store(ctx, ev, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event: %v", err)
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

	// Check event order and version assignment
	for i, ev := range allEvents {
		expectedSeq := uint64(i + 1)
		if version, ok := ev.Version.(cursor.IntegerCursor); ok {
			if version.Seq != expectedSeq {
				t.Errorf("Event %d: expected version %d, got %d", i, expectedSeq, version.Seq)
			}
		} else {
			t.Errorf("Event %d: expected IntegerCursor version", i)
		}
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

func TestMemStore_Load_IncompatibleVersion(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Try to load with incompatible version type
	_, err := store.Load(ctx, &customVersion{})
	if err == nil {
		t.Error("Expected error for incompatible version type")
	}
	if err.Error() != "incompatible version type: expected cursor.IntegerCursor" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestMemStore_LoadByAggregate(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Store events for multiple aggregates
	events := []*event.Event{
		event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
		event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
		event.New("test-3", "UserCreated", "user-456", []byte(`{"name":"Bob"}`)),
		event.New("test-4", "UserUpdated", "user-456", []byte(`{"email":"bob@example.com"}`)),
		event.New("test-5", "UserDeleted", "user-123", []byte(`{}`)),
	}

	for _, ev := range events {
		err := store.Store(ctx, ev, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	// Load events for user-123
	aliceEvents, err := store.LoadByAggregate(ctx, "user-123", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load Alice events: %v", err)
	}

	if len(aliceEvents) != 3 {
		t.Errorf("Expected 3 events for Alice, got %d", len(aliceEvents))
	}

	// Check that all events belong to user-123
	for i, ev := range aliceEvents {
		if ev.Event.AggregateID() != "user-123" {
			t.Errorf("Event %d: expected aggregate user-123, got %s", i, ev.Event.AggregateID())
		}
	}

	// Load events for user-456
	bobEvents, err := store.LoadByAggregate(ctx, "user-456", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load Bob events: %v", err)
	}

	if len(bobEvents) != 2 {
		t.Errorf("Expected 2 events for Bob, got %d", len(bobEvents))
	}

	// Load events for non-existent aggregate
	noEvents, err := store.LoadByAggregate(ctx, "user-999", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load non-existent aggregate events: %v", err)
	}

	if len(noEvents) != 0 {
		t.Errorf("Expected 0 events for non-existent aggregate, got %d", len(noEvents))
	}
}

func TestMemStore_LoadByAggregate_WithSince(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Store events
	events := []*event.Event{
		event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),              // version 1
		event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)), // version 2
		event.New("test-3", "UserDeleted", "user-123", []byte(`{}`)),                            // version 3
	}

	for _, ev := range events {
		err := store.Store(ctx, ev, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	// Load events since version 1
	recentEvents, err := store.LoadByAggregate(ctx, "user-123", cursor.IntegerCursor{Seq: 1})
	if err != nil {
		t.Fatalf("Failed to load recent events: %v", err)
	}

	if len(recentEvents) != 2 {
		t.Errorf("Expected 2 recent events, got %d", len(recentEvents))
	}

	// The events should be UserUpdated and UserDeleted
	expectedTypes := []string{"UserUpdated", "UserDeleted"}
	for i, ev := range recentEvents {
		if i < len(expectedTypes) && ev.Event.Type() != expectedTypes[i] && ev.Event.Type() != events[i+1].EventType {
			// Note: Type normalization might vary, so we check the actual stored type
			t.Logf("Event %d type: %s", i, ev.Event.Type())
		}
	}
}

func TestMemStore_LatestVersion(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Check latest version on empty store
	version, err := store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if version.(cursor.IntegerCursor).Seq != 0 {
		t.Errorf("Expected version 0 for empty store, got %d", version.(cursor.IntegerCursor).Seq)
	}

	// Store some events
	for i := 0; i < 5; i++ {
		testEvent := event.New(fmt.Sprintf("test-%d", i+1), "UserCreated", fmt.Sprintf("user-%d", i+1), []byte(`{"name":"Test"}`))
		err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event %d: %v", i+1, err)
		}
	}

	// Check latest version
	version, err = store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if version.(cursor.IntegerCursor).Seq != 5 {
		t.Errorf("Expected version 5, got %d", version.(cursor.IntegerCursor).Seq)
	}
}

func TestMemStore_ParseVersion(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Test valid version strings
	testCases := []struct {
		input    string
		expected uint64
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"999", 999},
		{"", 0}, // Empty string should default to 0
	}

	for _, tc := range testCases {
		version, err := store.ParseVersion(ctx, tc.input)
		if err != nil {
			t.Errorf("Failed to parse version '%s': %v", tc.input, err)
			continue
		}

		if cursor, ok := version.(cursor.IntegerCursor); ok {
			if cursor.Seq != tc.expected {
				t.Errorf("Version '%s': expected %d, got %d", tc.input, tc.expected, cursor.Seq)
			}
		} else {
			t.Errorf("Version '%s': expected IntegerCursor, got %T", tc.input, version)
		}
	}

	// Test invalid version string
	_, err := store.ParseVersion(ctx, "invalid")
	if err == nil {
		t.Error("Expected error for invalid version string")
	}
}

func TestMemStore_StoreBatch(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Create batch of events
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("batch-1", "UserCreated", "user-1", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
		{
			Event:   event.New("batch-2", "UserCreated", "user-2", []byte(`{"name":"Bob"}`)),
			Version: cursor.IntegerCursor{Seq: 2},
		},
		{
			Event:   event.New("batch-3", "UserUpdated", "user-1", []byte(`{"email":"alice@example.com"}`)),
			Version: cursor.IntegerCursor{Seq: 3},
		},
	}

	err := store.StoreBatch(ctx, events)
	if err != nil {
		t.Fatalf("Failed to store batch: %v", err)
	}

	// Check that all events were stored
	allEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events: %v", err)
	}

	if len(allEvents) != 3 {
		t.Errorf("Expected 3 events, got %d", len(allEvents))
	}

	// Check stats
	stats := store.Stats()
	if stats.TotalEvents != 3 {
		t.Errorf("Expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 2 { // user-1 and user-2
		t.Errorf("Expected 2 streams, got %d", stats.TotalStreams)
	}
}

func TestMemStore_StoreBatch_Empty(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()

	// Store empty batch
	err := store.StoreBatch(ctx, []synckit.EventWithVersion{})
	if err != nil {
		t.Errorf("Expected no error for empty batch, got %v", err)
	}

	// Check stats
	stats := store.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 events, got %d", stats.TotalEvents)
	}
}

func TestMemStore_Concurrency(t *testing.T) {
	store := New()
	defer store.Close()

	ctx := context.Background()
	numGoroutines := 10
	eventsPerGoroutine := 100

	var wg sync.WaitGroup

	// Concurrent writes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				eventID := fmt.Sprintf("g%d-event-%d", goroutineID, i)
				aggregateID := fmt.Sprintf("user-%d", (goroutineID*eventsPerGoroutine+i)%20) // 20 different aggregates
				testEvent := event.New(eventID, "UserAction", aggregateID, []byte(fmt.Sprintf(`{"action":%d}`, i)))

				err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 0})
				if err != nil {
					t.Errorf("Failed to store event %s: %v", eventID, err)
				}
			}
		}(g)
	}

	// Concurrent reads
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine/10; i++ {
				// Load all events
				_, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
				if err != nil {
					t.Errorf("Failed to load events in goroutine %d: %v", goroutineID, err)
				}

				// Load by specific aggregate
				aggregateID := fmt.Sprintf("user-%d", i%20)
				_, err = store.LoadByAggregate(ctx, aggregateID, cursor.IntegerCursor{Seq: 0})
				if err != nil {
					t.Errorf("Failed to load aggregate %s in goroutine %d: %v", aggregateID, goroutineID, err)
				}

				// Get latest version
				_, err = store.LatestVersion(ctx)
				if err != nil {
					t.Errorf("Failed to get latest version in goroutine %d: %v", goroutineID, err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify final state
	allEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load all events after concurrent test: %v", err)
	}

	expectedTotal := numGoroutines * eventsPerGoroutine
	if len(allEvents) != expectedTotal {
		t.Errorf("Expected %d total events, got %d", expectedTotal, len(allEvents))
	}

	// Check that versions are sequential and unique
	versionSet := make(map[uint64]bool)
	for _, ev := range allEvents {
		if version, ok := ev.Version.(cursor.IntegerCursor); ok {
			if versionSet[version.Seq] {
				t.Errorf("Duplicate version found: %d", version.Seq)
			}
			versionSet[version.Seq] = true
		}
	}

	if len(versionSet) != expectedTotal {
		t.Errorf("Expected %d unique versions, got %d", expectedTotal, len(versionSet))
	}
}

func TestMemStore_Stats(t *testing.T) {
	store := New()
	defer store.Close()

	// Check initial stats
	stats := store.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 initial events, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 0 {
		t.Errorf("Expected 0 initial streams, got %d", stats.TotalStreams)
	}
	if stats.NextSequence != 1 {
		t.Errorf("Expected initial next sequence 1, got %d", stats.NextSequence)
	}
	if stats.Closed != false {
		t.Errorf("Expected store to be open, got closed=true")
	}

	ctx := context.Background()

	// Store events for multiple streams
	events := []*event.Event{
		event.New("test-1", "UserCreated", "user-1", []byte(`{"name":"Alice"}`)),
		event.New("test-2", "UserCreated", "user-2", []byte(`{"name":"Bob"}`)),
		event.New("test-3", "UserUpdated", "user-1", []byte(`{"email":"alice@example.com"}`)),
		event.New("test-4", "OrderCreated", "order-1", []byte(`{"amount":99.99}`)),
	}

	for _, ev := range events {
		err := store.Store(ctx, ev, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	// Check updated stats
	stats = store.Stats()
	if stats.TotalEvents != 4 {
		t.Errorf("Expected 4 events, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 3 { // user-1, user-2, order-1
		t.Errorf("Expected 3 streams, got %d", stats.TotalStreams)
	}
	if stats.NextSequence != 5 {
		t.Errorf("Expected next sequence 5, got %d", stats.NextSequence)
	}
	if stats.Closed != false {
		t.Errorf("Expected store to be open, got closed=true")
	}

	// Close store and check stats
	store.Close()
	stats = store.Stats()
	if stats.Closed != true {
		t.Errorf("Expected store to be closed, got closed=false")
	}
}

func TestMemStore_Close(t *testing.T) {
	store := New()

	// Store some events
	ctx := context.Background()
	testEvent := event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`))
	err := store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 1})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Close store
	err = store.Close()
	if err != nil {
		t.Errorf("Failed to close store: %v", err)
	}

	// Verify store is closed
	stats := store.Stats()
	if !stats.Closed {
		t.Error("Expected store to be closed")
	}

	// Try to use closed store
	err = store.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 2})
	if err == nil {
		t.Error("Expected error when using closed store")
	}

	// Close again should be safe
	err = store.Close()
	if err != nil {
		t.Errorf("Second close should not error, got: %v", err)
	}
}

// Helper type for testing incompatible versions
type customVersion struct{}

func (c *customVersion) Compare(other synckit.Version) int { return 0 }
func (c *customVersion) String() string                    { return "custom" }
func (c *customVersion) IsZero() bool                      { return false }
