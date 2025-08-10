package client

import (
	"context"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/examples/basic/metrics"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

// LastWriteWinsResolver implements a simple conflict resolution strategy
type LastWriteWinsResolver struct{}

func (r *LastWriteWinsResolver) Resolve(ctx context.Context, local, remote []synckit.EventWithVersion) ([]synckit.EventWithVersion, error) {
	if len(local) == 0 {
		return remote, nil
	}
	if len(remote) == 0 {
		return local, nil
	}

	// Return remote version as it's newer
	return remote, nil
}

func RunClient(ctx context.Context) error {
	// Create client store
	clientStore, err := sqlite.New(&sqlite.Config{
		DataSourceName: "file:client.db",
		EnableWAL:      true,
	})
	if err != nil {
		return err
	}
	defer clientStore.Close()

	// Create metrics collector
	metricsCollector := metrics.NewHTTPMetricsCollector()

	// Create HTTP transport
	clientTransport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

	// Configure sync with metrics
	syncOptions := &synckit.SyncOptions{
		BatchSize:        10,
		SyncInterval:     5 * time.Second,
		ConflictResolver: &LastWriteWinsResolver{},
		MetricsCollector: metricsCollector,
	}

	// Create sync manager
	syncManager := synckit.NewSyncManager(clientStore, clientTransport, syncOptions, logging.Default().Logger)

	// Subscribe to sync events
	syncManager.Subscribe(func(result *synckit.SyncResult) {
		log.Printf("✓ Sync completed: pushed=%d, pulled=%d, conflicts=%d",
			result.EventsPushed, result.EventsPulled, result.ConflictsResolved)

		if len(result.Errors) > 0 {
			log.Printf("⚠ Sync errors: %v", result.Errors)
		}
	})

	// Start auto-sync
	if err := syncManager.StartAutoSync(ctx); err != nil {
		return err
	}

	log.Println("Client started with auto-sync enabled")
	log.Println("Press Ctrl+C to stop")

	// Wait for context cancellation
	<-ctx.Done()
	log.Println("Client shutting down...")
	return nil
}
