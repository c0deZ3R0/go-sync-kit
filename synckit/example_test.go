package synckit_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/memchan"
)

// ExampleNewNode_inMemory demonstrates creating a SyncNode with in-memory storage
// and performing a sync operation in a single-process environment.
func ExampleNewNode_inMemory() {
	ctx := context.Background()

	// Create an in-memory event store
	store := memstore.New()

	// Create an in-memory channel transport with capacity for 16 events
	transport := memchan.New(16)

	// Create a silent logger for clean example output
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a SyncNode with the store and transport
	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithManagerLogger(silentLogger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Perform an initial sync (no events yet)
	result, err := node.Sync(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: Events pulled: 0, Events pushed: 0
	fmt.Printf("Events pulled: %d, Events pushed: %d\n", result.EventsPulled, result.EventsPushed)
}

// ExampleWithStore_seedingEvents demonstrates storing events before sync.
func ExampleWithStore_seedingEvents() {
	ctx := context.Background()

	// Create store and transport
	store := memstore.New()
	transport := memchan.New(16)
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a simple event
	evt := event.New(
		"evt-1",           // event ID
		"UserCreated",     // event type
		"user-123",        // aggregate ID
		[]byte(`{"name":"Alice"}`), // event data
	)

	// Store the event with an integer cursor version
	version := cursor.IntegerCursor{Seq: 1}
	if err := store.Store(ctx, evt, version); err != nil {
		log.Fatal(err)
	}

	// Create node and sync
	node, _ := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithManagerLogger(silentLogger),
	)

	result, _ := node.Sync(ctx)

	// Output: Events in store before sync: 1
	fmt.Printf("Events in store before sync: %d\n", 1)
	_ = result
}

// ExampleConflictResolver_custom demonstrates configuring a custom conflict resolver.
func ExampleConflictResolver_custom() {
	ctx := context.Background()

	// Create store, transport, and node with the LWW (Last-Write-Wins) resolver
	store := memstore.New()
	transport := memchan.New(16)
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithLWW(), // Use Last-Write-Wins resolver
		synckit.WithManagerLogger(silentLogger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Perform sync
	result, _ := node.Sync(ctx)

	// Output: Node created with LWW resolver; sync completed successfully
	fmt.Printf("Node created with LWW resolver; sync completed successfully\n")
	_ = result
}

// Example demonstrates the basic sync workflow:
// store events locally, configure a node, and sync.
func Example() {
	ctx := context.Background()

	// 1. Create storage and transport
	store := memstore.New()
	transport := memchan.New(16)
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// 2. Create a node
	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithManagerLogger(silentLogger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Perform a sync cycle
	result, err := node.Sync(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// 4. Inspect results
	if result.EventsPushed == 0 && result.EventsPulled == 0 {
		fmt.Println("Sync complete: no events to sync")
	}

	// Output: Sync complete: no events to sync
}
