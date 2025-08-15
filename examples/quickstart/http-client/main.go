package main

import (
	"context"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
	// Create SQLite store
	store, err := sqlite.NewWithDataSource("app.db")
	if err != nil {
		log.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Create HTTP transport
	transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

	// Create sync manager with functional options
	mgr, err := synckit.NewManager(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithLWW(),
		synckit.WithBatchSize(100),
		synckit.WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	result, err := mgr.Sync(context.Background())
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	log.Printf("Sync completed successfully! Events pushed: %d, Events pulled: %d, Duration: %v\n",
		result.EventsPushed, result.EventsPulled, result.Duration)
}
