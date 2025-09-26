// HTTP Server Example for Go Sync Kit
// Demonstrates creating a sync server using NewHTTPServerNode preset
package main

import (
	"log"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
	log.Printf("=== Go Sync Kit HTTP Server Example ===")

	// Event store (SQLite here, could be Postgres in prod)
	// Note: server.db persists between runs; delete it if you want a clean slate
	store, err := sqlite.New(&sqlite.Config{DataSourceName: "server.db"})
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// HTTP transport in server mode
	transport := httptransport.NewTransport("", nil, nil, nil)

	// SyncNode configured as server
	node, err := synckit.NewHTTPServerNode(store, transport)
	if err != nil {
		log.Fatalf("failed to create server node: %v", err)
	}
	defer node.Close()

	// Expose sync handler
	handler := httptransport.NewSyncHandler(store, nil, nil, nil)
	http.Handle("/sync", handler)

	log.Printf("✅ Sync server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}