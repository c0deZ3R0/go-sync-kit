package main

import (
	"context"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func main() {
	// Create SQLite store
	store, err := sqlite.NewWithDataSource("local.db")
	if err != nil {
		log.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	mgr, err := synckit.NewManager(
		synckit.WithStore(store),
		synckit.WithNullTransport(), // Use null transport for local-only scenarios
		synckit.WithLWW(),
		synckit.WithBatchSize(200),
		synckit.WithTimeout(20*time.Second),
	)
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	
	result, err := mgr.Sync(context.Background())
	if err != nil {
		log.Fatalf("sync: %v", err)
	}
	
	log.Printf("Local sync completed successfully! Events: %d, Duration: %v\n",
		result.EventsPushed+result.EventsPulled, result.Duration)
}
