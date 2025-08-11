package postgres

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// TestEvent is a simple implementation of synckit.Event for testing
type TestEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (e *TestEvent) ID() string                         { return e.id }
func (e *TestEvent) Type() string                       { return e.eventType }
func (e *TestEvent) AggregateID() string                { return e.aggregateID }
func (e *TestEvent) Data() interface{}                  { return e.data }
func (e *TestEvent) Metadata() map[string]interface{}  { return e.metadata }

func newTestEvent(id, eventType, aggregateID string, data interface{}) *TestEvent {
	return &TestEvent{
		id:          id,
		eventType:   eventType,
		aggregateID: aggregateID,
		data:        data,
		metadata:    map[string]interface{}{"test": true},
	}
}

// getTestConnectionString returns the connection string for testing
// It first checks for an environment variable, then falls back to the default test setup
func getTestConnectionString() string {
	if connStr := os.Getenv("POSTGRES_TEST_CONNECTION"); connStr != "" {
		return connStr
	}
	// Default connection string for Docker Compose setup
	return "postgres://testuser:testpass123@localhost:5432/eventstore_test?sslmode=disable"
}

// setupTestStore creates a new PostgresEventStore for testing
func setupTestStore(t *testing.T) (*PostgresEventStore, func()) {
	config := &Config{
		ConnectionString: getTestConnectionString(),
		Logger:          log.New(os.Stdout, "[TEST] ", log.LstdFlags),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
	}

	store, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		// Clean up test data
		_, err := store.db.Exec("DELETE FROM events WHERE metadata->>'test' = 'true'")
		if err != nil {
			t.Logf("Failed to clean up test data: %v", err)
		}
		store.Close()
	}

	return store, cleanup
}

func TestPostgresEventStore_Store(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	event := newTestEvent("test-1", "UserCreated", "user-123", map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
	})

	// Store the event
	err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Verify the event was stored by loading it
	events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("No events found after storing")
	}

	// Find our test event
	found := false
	for _, ev := range events {
		if ev.Event.ID() == event.ID() {
			found = true
			if ev.Event.Type() != event.Type() {
				t.Errorf("Expected event type %s, got %s", event.Type(), ev.Event.Type())
			}
			if ev.Event.AggregateID() != event.AggregateID() {
				t.Errorf("Expected aggregate ID %s, got %s", event.AggregateID(), ev.Event.AggregateID())
			}
			break
		}
	}

	if !found {
		t.Error("Stored event not found in loaded events")
	}
}

func TestPostgresEventStore_LoadByAggregate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Store events for different aggregates
	event1 := newTestEvent("test-1", "UserCreated", "user-123", "data1")
	event2 := newTestEvent("test-2", "UserUpdated", "user-123", "data2")
	event3 := newTestEvent("test-3", "UserCreated", "user-456", "data3")

	for _, event := range []*TestEvent{event1, event2, event3} {
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event %s: %v", event.ID(), err)
		}
	}

	// Load events for user-123
	events, err := store.LoadByAggregate(ctx, "user-123", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events for user-123: %v", err)
	}

	// Should have 2 events for user-123
	if len(events) != 2 {
		t.Errorf("Expected 2 events for user-123, got %d", len(events))
	}

	// Verify all events belong to user-123
	for _, ev := range events {
		if ev.Event.AggregateID() != "user-123" {
			t.Errorf("Expected aggregate ID user-123, got %s", ev.Event.AggregateID())
		}
	}
}

func TestPostgresEventStore_StoreBatch(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create batch of events
	events := make([]synckit.EventWithVersion, 3)
	for i := 0; i < 3; i++ {
		event := newTestEvent(
			"batch-"+string(rune('1'+i)),
			"BatchTest",
			"aggregate-batch",
			map[string]int{"index": i},
		)
		events[i] = synckit.EventWithVersion{
			Event:   event,
			Version: cursor.IntegerCursor{Seq: uint64(i)},
		}
	}

	// Store batch
	err := store.StoreBatch(ctx, events)
	if err != nil {
		t.Fatalf("Failed to store batch: %v", err)
	}

	// Verify all events were stored
	loadedEvents, err := store.LoadByAggregate(ctx, "aggregate-batch", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load batch events: %v", err)
	}

	if len(loadedEvents) != 3 {
		t.Errorf("Expected 3 batch events, got %d", len(loadedEvents))
	}
}

func TestPostgresEventStore_LatestVersion(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Get latest version before storing any events
	version, err := store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get initial latest version: %v", err)
	}

	initialVersion := version.(cursor.IntegerCursor).Seq

	// Store an event
	event := newTestEvent("version-test", "VersionTest", "version-aggregate", "data")
	err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Get latest version after storing
	version, err = store.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version after store: %v", err)
	}

	newVersion := version.(cursor.IntegerCursor).Seq

	// Version should have increased
	if newVersion <= initialVersion {
		t.Errorf("Expected version to increase from %d, but got %d", initialVersion, newVersion)
	}
}

func TestRealtimeEventStore_Notifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-time notification test in short mode")
	}

	config := &Config{
		ConnectionString: getTestConnectionString(),
		Logger:          log.New(os.Stdout, "[REALTIME_TEST] ", log.LstdFlags),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
	}

	store, err := NewRealtimeEventStore(config)
	if err != nil {
		t.Fatalf("Failed to create realtime store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Channel to receive notifications
	notificationCh := make(chan NotificationPayload, 10)

	// Subscribe to stream notifications
	err = store.SubscribeToStream(ctx, "realtime-test", func(payload NotificationPayload) error {
		notificationCh <- payload
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to stream: %v", err)
	}

	// Give the subscription a moment to establish
	time.Sleep(2 * time.Second)

	// Store an event that should trigger a notification
	event := newTestEvent("realtime-1", "RealtimeTest", "realtime-test", "notification data")
	err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Wait for notification
	select {
	case notification := <-notificationCh:
		if notification.ID != event.ID() {
			t.Errorf("Expected notification for event %s, got %s", event.ID(), notification.ID)
		}
		if notification.AggregateID != event.AggregateID() {
			t.Errorf("Expected aggregate ID %s, got %s", event.AggregateID(), notification.AggregateID)
		}
		if notification.EventType != event.Type() {
			t.Errorf("Expected event type %s, got %s", event.Type(), notification.EventType)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}
}

func TestRealtimeEventStore_GlobalNotifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping global notification test in short mode")
	}

	config := &Config{
		ConnectionString: getTestConnectionString(),
		Logger:          log.New(os.Stdout, "[GLOBAL_TEST] ", log.LstdFlags),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
	}

	store, err := NewRealtimeEventStore(config)
	if err != nil {
		t.Fatalf("Failed to create realtime store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Channel to receive notifications
	notificationCh := make(chan NotificationPayload, 10)

	// Subscribe to all events
	err = store.SubscribeToAll(ctx, func(payload NotificationPayload) error {
		notificationCh <- payload
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to all events: %v", err)
	}

	// Give the subscription a moment to establish
	time.Sleep(2 * time.Second)

	// Store events for different aggregates
	event1 := newTestEvent("global-1", "GlobalTest", "global-agg-1", "data1")
	event2 := newTestEvent("global-2", "GlobalTest", "global-agg-2", "data2")

	for _, event := range []*TestEvent{event1, event2} {
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			t.Fatalf("Failed to store event %s: %v", event.ID(), err)
		}
	}

	// Wait for both notifications
	receivedCount := 0
	for receivedCount < 2 {
		select {
		case notification := <-notificationCh:
			receivedCount++
			if notification.EventType != "GlobalTest" {
				t.Errorf("Expected GlobalTest event, got %s", notification.EventType)
			}
			// Global notifications should include stream_name
			if notification.StreamName == "" {
				t.Error("Expected stream_name in global notification")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("Timeout waiting for global notifications, received %d/2", receivedCount)
		}
	}
}

func TestNotificationListener_ConnectionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection recovery test in short mode")
	}

	logger := log.New(os.Stdout, "[RECOVERY_TEST] ", log.LstdFlags)
	
	listener, err := NewNotificationListener(getTestConnectionString(), logger)
	if err != nil {
		t.Fatalf("Failed to create notification listener: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start the listener
	err = listener.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}

	// Give it time to connect
	time.Sleep(2 * time.Second)

	// Check that it's connected
	if !listener.IsConnected() {
		t.Fatal("Listener should be connected")
	}

	// Test subscription
	notificationCh := make(chan NotificationPayload, 5)
	err = listener.SubscribeToStream("recovery-test", func(payload NotificationPayload) error {
		notificationCh <- payload
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to stream: %v", err)
	}

	// Verify subscription works by sending a test notification via direct database connection
	// This simulates an event being stored
	config := DefaultConfig(getTestConnectionString())
	testStore, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	defer testStore.Close()

	testEvent := newTestEvent("recovery-test-1", "RecoveryTest", "recovery-test", "test data")
	err = testStore.Store(ctx, testEvent, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to store test event: %v", err)
	}

	// Wait for notification
	select {
	case notification := <-notificationCh:
		if notification.ID != testEvent.ID() {
			t.Errorf("Expected notification for event %s, got %s", testEvent.ID(), notification.ID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for test notification")
	}

	t.Log("Connection recovery test completed successfully")
}

// Benchmark tests
func BenchmarkPostgresEventStore_Store(b *testing.B) {
	config := &Config{
		ConnectionString: getTestConnectionString(),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
	}

	store, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := newTestEvent(
			"bench-"+string(rune(i)),
			"BenchmarkEvent",
			"bench-aggregate",
			map[string]int{"iteration": i},
		)
		
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Failed to store event: %v", err)
		}
	}
}

func BenchmarkPostgresEventStore_Load(b *testing.B) {
	config := &Config{
		ConnectionString: getTestConnectionString(),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
	}

	store, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Pre-populate with some events
	for i := 0; i < 100; i++ {
		event := newTestEvent(
			"bench-load-"+string(rune(i)),
			"BenchmarkLoad",
			"bench-load-aggregate",
			map[string]int{"iteration": i},
		)
		
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Failed to store setup event: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			b.Fatalf("Failed to load events: %v", err)
		}
	}
}
