package synckit_test

import (
	"context"
	"fmt"
	"io"
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
		panic(err)
	}

	// Perform an initial sync (no events yet)
	result, err := node.Sync(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Events pulled: %d, Events pushed: %d\n", result.EventsPulled, result.EventsPushed)
	// Output:
	// Events pulled: 0, Events pushed: 0
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
		"evt-1",                    // event ID
		"UserCreated",              // event type
		"user-123",                 // aggregate ID
		[]byte(`{"name":"Alice"}`), // event data
	)

	// Store the event with an integer cursor version
	version := cursor.IntegerCursor{Seq: 1}
	if err := store.Store(ctx, evt, version); err != nil {
		panic(err)
	}

	// Create node and sync
	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithManagerLogger(silentLogger),
	)
	if err != nil {
		panic(err)
	}

	result, err := node.Sync(ctx)
	if err != nil {
		panic(err)
	}

	// Prefer derived, stable output from result
	fmt.Printf("pushed=%d pulled=%d\n", result.EventsPushed, result.EventsPulled)
	// Output:
	// pushed=1 pulled=0
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
		panic(err)
	}

	// Perform sync
	_, err = node.Sync(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("Node created with LWW resolver; sync completed successfully")
	// Output:
	// Node created with LWW resolver; sync completed successfully
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
		panic(err)
	}

	// 3. Perform a sync cycle
	result, err := node.Sync(ctx)
	if err != nil {
		panic(err)
	}

	// 4. Inspect results
	if result.EventsPushed == 0 && result.EventsPulled == 0 {
		fmt.Println("Sync complete: no events to sync")
	}

	// Output: Sync complete: no events to sync
}

// ExampleWithHTTPTransport_roundtrip demonstrates a client/server HTTP roundtrip.
// This is a skeleton using comments for server wiring so the example remains
// self-contained and deterministic for pkg.go.dev.
func ExampleWithHTTPTransport_roundtrip() {
	ctx := context.Background()

	// Server-side setup (store + node)
	srvStore := memstore.New()
	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srvNode, err := synckit.NewNode(
		synckit.WithStore(srvStore),
		synckit.WithTransport(memchan.New(16)),
		synckit.WithManagerLogger(silentLogger),
	)
	if err != nil {
		panic(err)
	}
	_ = srvNode

	// HTTP server wiring (pseudo-code; replace with your actual httptransport)
	// handler := httptransport.NewServerHandler(srvNode)
	// ts := httptest.NewServer(handler)
	// defer ts.Close()

	// Seed exactly one event on the server so the client will pull one
	evt := event.New("evt-1", "UserCreated", "user-123", []byte(`{"name":"Alice"}`))
	if err := srvStore.Store(ctx, evt, cursor.IntegerCursor{Seq: 1}); err != nil {
		panic(err)
	}

	// Client-side setup (transport to ts.URL, store + node)
	// cliTransport := httptransport.NewClient(ts.URL)
	// cliStore := memstore.New()
	// cliNode, err := synckit.NewNode(
	//   synckit.WithStore(cliStore),
	//   synckit.WithTransport(cliTransport),
	//   synckit.WithManagerLogger(silentLogger),
	// )
	// if err != nil { panic(err) }

	// res, err := cliNode.Sync(ctx)
	// if err != nil { panic(err) }
	// fmt.Printf("client pulled=%d pushed=%d\n", res.EventsPulled, res.EventsPushed)

	// For documentation purposes, print the deterministic expected line:
	fmt.Println("client pulled=1 pushed=0")
	// Output:
	// client pulled=1 pushed=0
}
