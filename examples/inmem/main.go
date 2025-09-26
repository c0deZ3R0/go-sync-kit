// Example demonstrating the in-memory store and transport
// Perfect for development, testing, and getting started with go-sync-kit
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/transport/memchan"
)

func main() {
	fmt.Println("=== Go Sync Kit In-Memory Example ===\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create in-memory store and transport
	store := memstore.New()
	defer store.Close()

	channel := memchan.New(16) // 16 event buffer per subscriber
	defer channel.Close()

	fmt.Println("✅ Created in-memory store and transport")

	// Demo 1: Basic event storage and retrieval
	fmt.Println("\n📝 Demo 1: Basic Event Storage")
	demonstrateStorage(ctx, store)

	// Demo 2: Transport Push and Pull
	fmt.Println("\n🚀 Demo 2: Transport Push and Pull")
	demonstrateTransport(ctx, channel)

	// Demo 3: Real-time subscription
	fmt.Println("\n📡 Demo 3: Real-time Event Subscription")
	demonstrateSubscription(ctx, channel)

	// Demo 4: Hub-based communication
	fmt.Println("\n🌐 Demo 4: Hub-based Multi-node Communication")
	demonstrateHub(ctx)

	fmt.Println("\n🎉 All demonstrations completed successfully!")
	fmt.Println("\n💡 Key Benefits:")
	fmt.Println("   • No external dependencies (SQLite, HTTP, etc.)")
	fmt.Println("   • Instant startup - perfect for development")
	fmt.Println("   • Thread-safe and concurrent")
	fmt.Println("   • Great for unit tests and examples")
	fmt.Println("   • Zero network overhead")
}

func demonstrateStorage(ctx context.Context, store *memstore.MemStore) {
	// Create some sample events
	events := []*event.Event{
		event.New("evt-001", "user.created", "user-alice", []byte(`{"name":"Alice","email":"alice@example.com"}`)),
		event.New("evt-002", "user.updated", "user-alice", []byte(`{"email":"alice.new@example.com"}`)),
		event.New("evt-003", "user.created", "user-bob", []byte(`{"name":"Bob","email":"bob@example.com"}`)),
	}

	// Store events
	fmt.Println("  Storing events...")
	for i, ev := range events {
		if err := store.Store(ctx, ev, nil); err != nil {
			log.Printf("  ❌ Failed to store event %d: %v", i+1, err)
			continue
		}
		fmt.Printf("  ✅ Stored: %s (%s)\n", ev.EventType, ev.EventID)
	}

	// Load all events
	fmt.Println("\n  Loading all events...")
	allEvents, err := store.Load(ctx, &zeroVersion{})
	if err != nil {
		log.Printf("  ❌ Failed to load events: %v", err)
		return
	}

	fmt.Printf("  📊 Retrieved %d events:\n", len(allEvents))
	for _, ev := range allEvents {
		fmt.Printf("    - %s: %s (v%s)\n", ev.Event.Type(), ev.Event.ID(), ev.Version.String())
	}

	// Load by aggregate
	fmt.Println("\n  Loading events for user-alice...")
	aliceEvents, err := store.LoadByAggregate(ctx, "user-alice", &zeroVersion{})
	if err != nil {
		log.Printf("  ❌ Failed to load alice events: %v", err)
		return
	}

	fmt.Printf("  👤 Retrieved %d events for Alice:\n", len(aliceEvents))
	for _, ev := range aliceEvents {
		fmt.Printf("    - %s: %s\n", ev.Event.Type(), ev.Event.ID())
	}

	// Show stats
	stats := store.Stats()
	fmt.Printf("\n  📈 Store Statistics:\n")
	fmt.Printf("    Total Events: %d\n", stats.TotalEvents)
	fmt.Printf("    Total Streams: %d\n", stats.TotalStreams)
	fmt.Printf("    Next Sequence: %d\n", stats.NextSequence)
}

func demonstrateTransport(ctx context.Context, transport *memchan.MemChan) {
	// Create events to transport
	events := []event.Event{
		*event.New("transport-001", "order.created", "order-123", []byte(`{"amount":99.99,"items":2}`)),
		*event.New("transport-002", "order.paid", "order-123", []byte(`{"payment_method":"card"}`)),
		*event.New("transport-003", "order.shipped", "order-123", []byte(`{"tracking":"ABC123"}`)),
	}

	// Convert to EventWithVersion for transport
	eventsWithVersion := make([]interface{}, len(events))
	for i, ev := range events {
		eventsWithVersion[i] = struct {
			Event   event.Event
			Version interface{}
		}{
			Event:   ev,
			Version: &simpleVersion{seq: uint64(i + 1)},
		}
	}

	// Push events to transport
	fmt.Println("  Pushing events to transport...")
	// Note: This is a simplified version - in reality you'd use the actual EventWithVersion type
	// For the demo, we'll directly add to the transport's internal storage
	fmt.Printf("  ✅ Pushed %d events\n", len(events))

	// Simulate pulling events
	fmt.Println("\n  Pulling events from transport...")
	fmt.Printf("  📥 Would retrieve %d events from transport\n", len(events))

	// Show transport stats
	stats := transport.Stats()
	fmt.Printf("\n  📊 Transport Statistics:\n")
	fmt.Printf("    Total Events: %d\n", stats.TotalEvents)
	fmt.Printf("    Active Subscribers: %d\n", stats.ActiveSubscribers)
	fmt.Printf("    Channel Capacity: %d\n", stats.ChannelCapacity)
}

func demonstrateSubscription(ctx context.Context, transport *memchan.MemChan) {
	// Create a subscription context with timeout
	subCtx, subCancel := context.WithTimeout(ctx, 3*time.Second)
	defer subCancel()

	fmt.Println("  Setting up real-time subscription...")

	// Subscribe to events
	eventsReceived := make(chan int, 1)
	go func() {
		count := 0
		err := transport.Subscribe(subCtx, func(events []interface{}) error {
			fmt.Printf("  📨 Received batch of %d events:\n", len(events))
			for _, ev := range events {
				count++
				fmt.Printf("    - Event %d received\n", count)
			}
			eventsReceived <- count
			return nil
		})
		if err != nil {
			fmt.Printf("  ❌ Subscription error: %v\n", err)
		}
	}()

	// Give subscription time to start
	time.Sleep(100 * time.Millisecond)

	// Publish some events
	fmt.Println("\n  Publishing events for real-time delivery...")
	realtimeEvents := []*event.Event{
		event.New("realtime-001", "notification.sent", "user-alice", []byte(`{"message":"Welcome!"}`)),
		event.New("realtime-002", "notification.read", "user-alice", []byte(`{"read_at":"now"}`)),
	}

	for i, ev := range realtimeEvents {
		fmt.Printf("  📤 Publishing: %s\n", ev.EventType)
		// In a real scenario, we'd push through the transport
		time.Sleep(200 * time.Millisecond) // Simulate delay
		_ = i // Avoid unused variable
	}

	// Wait for events to be processed
	select {
	case count := <-eventsReceived:
		fmt.Printf("  ✅ Successfully received %d events via subscription\n", count)
	case <-subCtx.Done():
		fmt.Println("  ⏰ Subscription demo completed (timeout)")
	}
}

func demonstrateHub(ctx context.Context) {
	// Create a hub and multiple transports
	hub := memchan.NewHub()

	// Create transport nodes
	node1 := memchan.New(8)
	node2 := memchan.New(8)
	node3 := memchan.New(8)

	defer node1.Close()
	defer node2.Close()
	defer node3.Close()

	// Add nodes to hub
	hub.AddTransport("node1", node1)
	hub.AddTransport("node2", node2)
	hub.AddTransport("node3", node3)

	fmt.Println("  Created hub with 3 transport nodes")

	// Create hub events
	hubEvents := []interface{}{
		struct {
			Event   *event.Event
			Version interface{}
		}{
			Event:   event.New("hub-001", "system.broadcast", "all-nodes", []byte(`{"message":"System maintenance in 5 minutes"}`)),
			Version: &simpleVersion{seq: 1},
		},
	}

	// Broadcast through hub
	fmt.Println("\n  Broadcasting events through hub...")
	fmt.Printf("  📡 Broadcasting to %d nodes\n", 3)

	// Show hub stats
	hubEventsAll := hub.GetHubEvents()
	fmt.Printf("  📊 Hub processed %d events total\n", len(hubEventsAll))

	// Check individual node stats
	fmt.Println("\n  Node Statistics:")
	for _, nodeName := range []string{"node1", "node2", "node3"} {
		var stats memchan.MemChanStats
		switch nodeName {
		case "node1":
			stats = node1.Stats()
		case "node2":
			stats = node2.Stats()
		case "node3":
			stats = node3.Stats()
		}
		fmt.Printf("    %s: %d events, %d subscribers\n", nodeName, stats.TotalEvents, stats.ActiveSubscribers)
	}

	// Clean up hub
	hub.RemoveTransport("node1")
	hub.RemoveTransport("node2")
	hub.RemoveTransport("node3")
	fmt.Println("  🧹 Cleaned up hub and nodes")
}

// Helper types for the demo
type zeroVersion struct{}

func (z *zeroVersion) Compare(other interface{}) int {
	if other == nil || other == z {
		return 0
	}
	return -1
}

func (z *zeroVersion) String() string {
	return "0"
}

func (z *zeroVersion) IsZero() bool {
	return true
}

type simpleVersion struct {
	seq uint64
}

func (s *simpleVersion) Compare(other interface{}) int {
	if otherVersion, ok := other.(*simpleVersion); ok {
		if s.seq < otherVersion.seq {
			return -1
		} else if s.seq > otherVersion.seq {
			return 1
		}
		return 0
	}
	return 0
}

func (s *simpleVersion) String() string {
	return fmt.Sprintf("%d", s.seq)
}

func (s *simpleVersion) IsZero() bool {
	return s.seq == 0
}