// HTTP Client Example for Go Sync Kit
// Demonstrates creating a sync client using NewHTTPClientNode preset
package main

import (
	"context"
	"log"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
	log.Printf("=== Go Sync Kit HTTP Client Example ===")

	// Local store (each client maintains its own DB)
	// Note: client.db persists between runs; delete it if you want a clean slate
	store, err := sqlite.New(&sqlite.Config{DataSourceName: "client.db"})
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// HTTP transport pointed at server
	transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

	// SyncNode configured as client
	node, err := synckit.NewHTTPClientNode(store, transport)
	if err != nil {
		log.Fatalf("failed to create client node: %v", err)
	}
	defer node.Close()

	// Perform a sync
	ctx := context.Background()
	result, err := node.Sync(ctx)
	if err != nil {
		log.Fatalf("sync failed: %v", err)
	}

	log.Printf("✅ Sync complete: pushed %d, pulled %d, conflicts %d",
		result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
}