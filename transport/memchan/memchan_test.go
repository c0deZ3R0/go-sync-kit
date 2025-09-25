package memchan

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

func TestMemChan_New(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	if transport == nil {
		t.Fatal("Expected non-nil transport")
	}

	stats := transport.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 events, got %d", stats.TotalEvents)
	}
	if stats.ActiveSubscribers != 0 {
		t.Errorf("Expected 0 subscribers, got %d", stats.ActiveSubscribers)
	}
	if stats.ChannelCapacity != 16 {
		t.Errorf("Expected capacity 16, got %d", stats.ChannelCapacity)
	}
	if stats.Closed {
		t.Error("Expected transport to be open")
	}
}

func TestMemChan_New_WithDefaultCapacity(t *testing.T) {
	transport := New(0) // Should use default capacity
	defer transport.Close()

	stats := transport.Stats()
	if stats.ChannelCapacity != 16 { // Default capacity
		t.Errorf("Expected default capacity 16, got %d", stats.ChannelCapacity)
	}

	transport2 := New(-5) // Should use default capacity
	defer transport2.Close()

	stats2 := transport2.Stats()
	if stats2.ChannelCapacity != 16 {
		t.Errorf("Expected default capacity 16, got %d", stats2.ChannelCapacity)
	}
}

func TestMemChan_Push(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx := context.Background()
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
		{
			Event:   event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
			Version: cursor.IntegerCursor{Seq: 2},
		},
	}

	err := transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Check stats
	stats := transport.Stats()
	if stats.TotalEvents != 2 {
		t.Errorf("Expected 2 events, got %d", stats.TotalEvents)
	}

	// Check that events can be retrieved
	storedEvents := transport.GetEvents()
	if len(storedEvents) != 2 {
		t.Errorf("Expected 2 stored events, got %d", len(storedEvents))
	}

	// Check event content
	if storedEvents[0].Event.ID() != "test-1" {
		t.Errorf("Expected event ID 'test-1', got '%s'", storedEvents[0].Event.ID())
	}
	if storedEvents[1].Event.ID() != "test-2" {
		t.Errorf("Expected event ID 'test-2', got '%s'", storedEvents[1].Event.ID())
	}
}

func TestMemChan_Push_ContextCancellation(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err := transport.Push(ctx, events)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestMemChan_Push_AfterClose(t *testing.T) {
	transport := New(16)
	transport.Close()

	ctx := context.Background()
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err := transport.Push(ctx, events)
	if err == nil {
		t.Error("Expected error when pushing to closed transport")
	}
	if err.Error() != "transport is closed" {
		t.Errorf("Expected 'transport is closed' error, got %v", err)
	}
}

func TestMemChan_Pull(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx := context.Background()

	// Push some events first
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
		{
			Event:   event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
			Version: cursor.IntegerCursor{Seq: 2},
		},
		{
			Event:   event.New("test-3", "UserDeleted", "user-456", []byte(`{}`)),
			Version: cursor.IntegerCursor{Seq: 3},
		},
	}

	err := transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Pull all events
	pulledEvents, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to pull events: %v", err)
	}

	if len(pulledEvents) != 3 {
		t.Errorf("Expected 3 events, got %d", len(pulledEvents))
	}

	// Pull events since version 1
	recentEvents, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 1})
	if err != nil {
		t.Fatalf("Failed to pull recent events: %v", err)
	}

	if len(recentEvents) != 2 {
		t.Errorf("Expected 2 recent events, got %d", len(recentEvents))
	}

	// Check event order
	if recentEvents[0].Event.ID() != "test-2" {
		t.Errorf("Expected first recent event ID 'test-2', got '%s'", recentEvents[0].Event.ID())
	}
	if recentEvents[1].Event.ID() != "test-3" {
		t.Errorf("Expected second recent event ID 'test-3', got '%s'", recentEvents[1].Event.ID())
	}
}

func TestMemChan_Pull_IncompatibleVersion(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx := context.Background()

	// Try to pull with incompatible version type
	_, err := transport.Pull(ctx, &customVersion{})
	if err == nil {
		t.Error("Expected error for incompatible version type")
	}
	if err.Error() != "incompatible version type: expected cursor.IntegerCursor" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestMemChan_GetLatestVersion(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx := context.Background()

	// Check latest version on empty transport
	version, err := transport.GetLatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if version.(cursor.IntegerCursor).Seq != 0 {
		t.Errorf("Expected version 0 for empty transport, got %d", version.(cursor.IntegerCursor).Seq)
	}

	// Push some events
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 5},
		},
		{
			Event:   event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
			Version: cursor.IntegerCursor{Seq: 3},
		},
		{
			Event:   event.New("test-3", "UserDeleted", "user-456", []byte(`{}`)),
			Version: cursor.IntegerCursor{Seq: 10},
		},
	}

	err = transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Check latest version - should be the highest
	version, err = transport.GetLatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if version.(cursor.IntegerCursor).Seq != 10 {
		t.Errorf("Expected version 10, got %d", version.(cursor.IntegerCursor).Seq)
	}
}

func TestMemChan_Subscribe(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Set up subscription
	eventsReceived := make(chan []synckit.EventWithVersion, 1)
	
	err := transport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
		eventsReceived <- events
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to start
	time.Sleep(100 * time.Millisecond)

	// Check that we have an active subscriber
	stats := transport.Stats()
	if stats.ActiveSubscribers != 1 {
		t.Errorf("Expected 1 active subscriber, got %d", stats.ActiveSubscribers)
	}

	// Push some events
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("sub-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
		{
			Event:   event.New("sub-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
			Version: cursor.IntegerCursor{Seq: 2},
		},
	}

	err = transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Wait for events to be received
	select {
	case receivedEvents := <-eventsReceived:
		if len(receivedEvents) == 0 {
			t.Error("Expected to receive events, but got empty batch")
		}
		t.Logf("Received %d events via subscription", len(receivedEvents))
	case <-ctx.Done():
		t.Error("Timeout waiting for subscribed events")
	}
}

func TestMemChan_Subscribe_NilHandler(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx := context.Background()

	err := transport.Subscribe(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil handler")
	}
	if err.Error() != "handler cannot be nil" {
		t.Errorf("Expected 'handler cannot be nil' error, got %v", err)
	}
}

func TestMemChan_Subscribe_ContextCancellation(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Set up subscription
	err := transport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to end subscription
	cancel()

	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Check that subscriber was cleaned up
	stats := transport.Stats()
	if stats.ActiveSubscribers != 0 {
		t.Errorf("Expected 0 active subscribers after cancellation, got %d", stats.ActiveSubscribers)
	}
}

func TestMemChan_Subscribe_HandlerError(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	handlerErrors := make(chan error, 1)

	// Set up subscription with error-prone handler
	err := transport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
		testErr := fmt.Errorf("handler error")
		handlerErrors <- testErr
		return testErr
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to start
	time.Sleep(50 * time.Millisecond)

	// Push an event to trigger handler
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("error-test", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err = transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// The handler should still be called despite returning an error
	select {
	case handlerErr := <-handlerErrors:
		if handlerErr.Error() != "handler error" {
			t.Errorf("Expected 'handler error', got %v", handlerErr)
		}
	case <-ctx.Done():
		t.Error("Timeout waiting for handler error")
	}
}

func TestMemChan_Close(t *testing.T) {
	transport := New(16)

	// Push some events
	ctx := context.Background()
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err := transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Add a subscriber
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()

	err = transport.Subscribe(subCtx, func(events []synckit.EventWithVersion) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to start
	time.Sleep(50 * time.Millisecond)

	// Verify we have a subscriber
	stats := transport.Stats()
	if stats.ActiveSubscribers != 1 {
		t.Errorf("Expected 1 subscriber, got %d", stats.ActiveSubscribers)
	}

	// Close transport
	err = transport.Close()
	if err != nil {
		t.Errorf("Failed to close transport: %v", err)
	}

	// Verify transport is closed
	stats = transport.Stats()
	if !stats.Closed {
		t.Error("Expected transport to be closed")
	}
	if stats.ActiveSubscribers != 0 {
		t.Errorf("Expected 0 subscribers after close, got %d", stats.ActiveSubscribers)
	}

	// Try to use closed transport
	err = transport.Push(ctx, events)
	if err == nil {
		t.Error("Expected error when using closed transport")
	}

	// Close again should be safe
	err = transport.Close()
	if err != nil {
		t.Errorf("Second close should not error, got: %v", err)
	}
}

func TestMemChan_ClearEvents(t *testing.T) {
	transport := New(16)
	defer transport.Close()

	ctx := context.Background()

	// Push some events
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("test-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
		{
			Event:   event.New("test-2", "UserUpdated", "user-123", []byte(`{"email":"alice@example.com"}`)),
			Version: cursor.IntegerCursor{Seq: 2},
		},
	}

	err := transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Verify events are stored
	storedEvents := transport.GetEvents()
	if len(storedEvents) != 2 {
		t.Errorf("Expected 2 stored events, got %d", len(storedEvents))
	}

	// Clear events
	transport.ClearEvents()

	// Verify events are cleared
	storedEvents = transport.GetEvents()
	if len(storedEvents) != 0 {
		t.Errorf("Expected 0 stored events after clear, got %d", len(storedEvents))
	}

	// Stats should reflect cleared events
	stats := transport.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 events in stats after clear, got %d", stats.TotalEvents)
	}
}

func TestMemChan_CreatePair(t *testing.T) {
	transport1, transport2 := CreatePair(8)
	defer transport1.Close()
	defer transport2.Close()

	if transport1 == nil || transport2 == nil {
		t.Fatal("Expected non-nil transports from CreatePair")
	}

	// Check both transports are independent
	stats1 := transport1.Stats()
	stats2 := transport2.Stats()

	if stats1.ChannelCapacity != 8 {
		t.Errorf("Transport1: expected capacity 8, got %d", stats1.ChannelCapacity)
	}
	if stats2.ChannelCapacity != 8 {
		t.Errorf("Transport2: expected capacity 8, got %d", stats2.ChannelCapacity)
	}

	// Test that they are independent by pushing to one
	ctx := context.Background()
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("pair-test", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err := transport1.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push to transport1: %v", err)
	}

	// transport1 should have the event, transport2 should not
	events1 := transport1.GetEvents()
	events2 := transport2.GetEvents()

	if len(events1) != 1 {
		t.Errorf("Transport1: expected 1 event, got %d", len(events1))
	}
	if len(events2) != 0 {
		t.Errorf("Transport2: expected 0 events, got %d", len(events2))
	}
}

func TestMemChan_Hub(t *testing.T) {
	hub := NewHub()
	if hub == nil {
		t.Fatal("Expected non-nil hub")
	}

	// Create transports
	node1 := New(8)
	node2 := New(8)
	node3 := New(8)

	defer node1.Close()
	defer node2.Close()
	defer node3.Close()

	// Add to hub
	hub.AddTransport("node1", node1)
	hub.AddTransport("node2", node2)
	hub.AddTransport("node3", node3)

	// Broadcast events
	ctx := context.Background()
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("hub-1", "SystemBroadcast", "all-nodes", []byte(`{"message":"maintenance"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err := hub.Broadcast(ctx, events)
	if err != nil {
		t.Fatalf("Failed to broadcast: %v", err)
	}

	// Check that all nodes received the event
	for i, node := range []*MemChan{node1, node2, node3} {
		nodeEvents := node.GetEvents()
		if len(nodeEvents) != 1 {
			t.Errorf("Node %d: expected 1 event, got %d", i+1, len(nodeEvents))
		}
		if len(nodeEvents) > 0 && nodeEvents[0].Event.ID() != "hub-1" {
			t.Errorf("Node %d: expected event ID 'hub-1', got '%s'", i+1, nodeEvents[0].Event.ID())
		}
	}

	// Check hub events
	hubEvents := hub.GetHubEvents()
	if len(hubEvents) != 1 {
		t.Errorf("Hub: expected 1 event, got %d", len(hubEvents))
	}

	// Remove a transport
	hub.RemoveTransport("node2")

	// Broadcast again
	events2 := []synckit.EventWithVersion{
		{
			Event:   event.New("hub-2", "SystemBroadcast", "all-nodes", []byte(`{"message":"update"}`)),
			Version: cursor.IntegerCursor{Seq: 2},
		},
	}

	err = hub.Broadcast(ctx, events2)
	if err != nil {
		t.Fatalf("Failed to broadcast after removal: %v", err)
	}

	// Check that only remaining nodes received the second event
	node1Events := node1.GetEvents()
	node2Events := node2.GetEvents()
	node3Events := node3.GetEvents()

	if len(node1Events) != 2 {
		t.Errorf("Node1: expected 2 events, got %d", len(node1Events))
	}
	if len(node2Events) != 1 { // Should only have the first event
		t.Errorf("Node2: expected 1 event (not updated after removal), got %d", len(node2Events))
	}
	if len(node3Events) != 2 {
		t.Errorf("Node3: expected 2 events, got %d", len(node3Events))
	}
}

func TestMemChan_Concurrency(t *testing.T) {
	transport := New(32)
	defer transport.Close()

	ctx := context.Background()
	numGoroutines := 10
	eventsPerGoroutine := 50

	var wg sync.WaitGroup

	// Concurrent pushers
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				events := []synckit.EventWithVersion{
					{
						Event:   event.New(fmt.Sprintf("g%d-e%d", goroutineID, i), "ConcurrentTest", fmt.Sprintf("user-%d", i%10), []byte(`{"test":true}`)),
						Version: cursor.IntegerCursor{Seq: uint64(goroutineID*eventsPerGoroutine + i + 1)},
					},
				}

				err := transport.Push(ctx, events)
				if err != nil {
					t.Errorf("Goroutine %d: failed to push event %d: %v", goroutineID, i, err)
				}
			}
		}(g)
	}

	// Concurrent pullers
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine/10; i++ {
				// Pull all events
				_, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
				if err != nil {
					t.Errorf("Goroutine %d: failed to pull events: %v", goroutineID, err)
				}

				// Get latest version
				_, err = transport.GetLatestVersion(ctx)
				if err != nil {
					t.Errorf("Goroutine %d: failed to get latest version: %v", goroutineID, err)
				}
			}
		}(g)
	}

	// Concurrent stats readers
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine/5; i++ {
				stats := transport.Stats()
				_ = stats // Just read the stats
			}
		}(g)
	}

	wg.Wait()

	// Verify final state
	allEvents := transport.GetEvents()
	expectedTotal := numGoroutines * eventsPerGoroutine
	if len(allEvents) != expectedTotal {
		t.Errorf("Expected %d total events, got %d", expectedTotal, len(allEvents))
	}

	// Check that all event IDs are unique
	eventIDs := make(map[string]bool)
	for _, ev := range allEvents {
		id := ev.Event.ID()
		if eventIDs[id] {
			t.Errorf("Duplicate event ID found: %s", id)
		}
		eventIDs[id] = true
	}
}

func TestMemChan_Subscribe_MultipleSubscribers(t *testing.T) {
	transport := New(32)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	numSubscribers := 3
	eventsReceived := make([]chan []synckit.EventWithVersion, numSubscribers)

	// Set up multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		eventsReceived[i] = make(chan []synckit.EventWithVersion, 5)
		subscriberID := i

		err := transport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			eventsReceived[subscriberID] <- events
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to subscribe subscriber %d: %v", i, err)
		}
	}

	// Give subscriptions time to start
	time.Sleep(100 * time.Millisecond)

	// Check that we have the expected number of subscribers
	stats := transport.Stats()
	if stats.ActiveSubscribers != numSubscribers {
		t.Errorf("Expected %d active subscribers, got %d", numSubscribers, stats.ActiveSubscribers)
	}

	// Push some events
	events := []synckit.EventWithVersion{
		{
			Event:   event.New("multi-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`)),
			Version: cursor.IntegerCursor{Seq: 1},
		},
	}

	err := transport.Push(ctx, events)
	if err != nil {
		t.Fatalf("Failed to push events: %v", err)
	}

	// Verify all subscribers received events
	for i := 0; i < numSubscribers; i++ {
		select {
		case receivedEvents := <-eventsReceived[i]:
			if len(receivedEvents) == 0 {
				t.Errorf("Subscriber %d: expected to receive events, but got empty batch", i)
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("Subscriber %d: timeout waiting for events", i)
		}
	}
}

func TestMemChan_Subscribe_SlowSubscriber(t *testing.T) {
	transport := New(2) // Small buffer to test overflow
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	eventsReceived := make(chan []synckit.EventWithVersion, 1)

	// Set up a slow subscriber (doesn't read from channel immediately)
	err := transport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
		// Simulate slow processing
		time.Sleep(200 * time.Millisecond)
		eventsReceived <- events
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to start
	time.Sleep(50 * time.Millisecond)

	// Push many events quickly to test buffer overflow
	for i := 0; i < 10; i++ {
		events := []synckit.EventWithVersion{
			{
				Event:   event.New(fmt.Sprintf("overflow-%d", i), "UserCreated", fmt.Sprintf("user-%d", i), []byte(`{"name":"Test"}`)),
				Version: cursor.IntegerCursor{Seq: uint64(i + 1)},
			},
		}

		err := transport.Push(ctx, events)
		if err != nil {
			t.Fatalf("Failed to push events %d: %v", i, err)
		}
	}

	// The slow subscriber may not receive all events due to buffer overflow
	// This test just verifies that the system doesn't crash or hang
	select {
	case receivedEvents := <-eventsReceived:
		t.Logf("Slow subscriber received %d events (some may have been dropped)", len(receivedEvents))
	case <-ctx.Done():
		t.Log("Slow subscriber test completed (timeout expected due to slow processing)")
	}

	// Verify that the events were still stored for Pull operations
	allEvents := transport.GetEvents()
	if len(allEvents) != 10 {
		t.Errorf("Expected 10 events to be stored, got %d", len(allEvents))
	}
}

// Helper type for testing incompatible versions
type customVersion struct{}

func (c *customVersion) Compare(other synckit.Version) int { return 0 }
func (c *customVersion) String() string                   { return "custom" }
func (c *customVersion) IsZero() bool                     { return false }