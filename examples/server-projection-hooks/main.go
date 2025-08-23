// Package main demonstrates Phase 5: Server-side projection hooks
// This example shows how to set up server-side projection hooks with the SyncHandler
// to enable server read models built from only server-committed events.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/projection"
	"github.com/c0deZ3R0/go-sync-kit/projection/badger"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"

	_ "github.com/mattn/go-sqlite3"
)

// UserCountProjector is an example projector that maintains a count of users
type UserCountProjector struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewUserCountProjector(db *sql.DB, logger *slog.Logger) *UserCountProjector {
	return &UserCountProjector{
		db:     db,
		logger: logger,
	}
}

func (p *UserCountProjector) Name() string {
	return "user_count"
}

func (p *UserCountProjector) Apply(ctx context.Context, events []synckit.EventWithVersion) error {
	p.logger.Info("Applying events to user count projection", slog.Int("event_count", len(events)))
	
	// Create read model table if it doesn't exist
	_, err := p.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_stats (
			id INTEGER PRIMARY KEY,
			user_count INTEGER DEFAULT 0,
			last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_stats table: %w", err)
	}
	
	// Initialize stats if not exists
	_, err = p.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO user_stats (id, user_count) VALUES (1, 0)
	`)
	if err != nil {
		return fmt.Errorf("failed to initialize user_stats: %w", err)
	}
	
	for _, ev := range events {
		// Example: increment counter for UserCreated events
		eventType := ev.Event.Type()
		p.logger.Debug("Processing event", slog.String("type", eventType), slog.String("id", ev.Event.ID()))
		
		if eventType == "UserCreated" {
			_, err := p.db.ExecContext(ctx, `
				UPDATE user_stats 
				SET user_count = user_count + 1, 
					last_updated = CURRENT_TIMESTAMP 
				WHERE id = 1
			`)
			if err != nil {
				return fmt.Errorf("failed to update user count: %w", err)
			}
			p.logger.Info("Incremented user count")
		} else if eventType == "UserDeleted" {
			_, err := p.db.ExecContext(ctx, `
				UPDATE user_stats 
				SET user_count = GREATEST(0, user_count - 1),
					last_updated = CURRENT_TIMESTAMP 
				WHERE id = 1
			`)
			if err != nil {
				return fmt.Errorf("failed to decrement user count: %w", err)
			}
			p.logger.Info("Decremented user count")
		}
	}
	
	return nil
}

// getUserCount returns the current user count from the read model
func (p *UserCountProjector) getUserCount(ctx context.Context) (int, error) {
	var count int
	err := p.db.QueryRowContext(ctx, "SELECT user_count FROM user_stats WHERE id = 1").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user count: %w", err)
	}
	return count, nil
}

func main() {
	logger := logging.Default().Logger
	
	logger.Info("Starting server with projection hooks example")
	
	// Create event store (SQLite)
	store, err := sqlite.NewWithDataSource("server.db")
	if err != nil {
		log.Fatalf("Failed to create event store: %v", err)
	}
	
	// Create separate SQLite database for read models  
	readModelDB, err := sql.Open("sqlite3", "server_read_models.db")
	if err != nil {
		log.Fatalf("Failed to create read model database: %v", err)
	}
	defer readModelDB.Close()
	
	// Create BadgerDB-based offset store for projection state
	offsetConfig := badger.DefaultConfig("projection_offsets")
	offsetStore, err := badger.NewOffsetStore(offsetConfig, store.ParseVersion)
	if err != nil {
		log.Fatalf("Failed to create offset store: %v", err)
	}
	defer offsetStore.Close()
	
	// Create projector
	projector := NewUserCountProjector(readModelDB, logger)
	
	// Create projection runner
	runner := projection.NewRunner(store, offsetStore, projector,
		projection.WithBatchSize(50),
		projection.WithLogger(logger),
	)
	
	// Create sync hooks
	hooks := &httptransport.SyncHooks{
		AfterCommit: func(ctx context.Context, committed []synckit.EventWithVersion) {
			// Apply server-committed events directly to projections
			logger.Info("AfterCommit hook called", slog.Int("committed_events", len(committed)))
			if err := runner.ApplyBatch(ctx, committed); err != nil {
				logger.Error("Projection failed in AfterCommit hook", slog.String("error", err.Error()))
			} else {
				// Log current user count after projection update
				if count, err := projector.getUserCount(ctx); err == nil {
					logger.Info("Projection updated - current user count", slog.Int("count", count))
				}
			}
		},
		BeforePull: func(ctx context.Context, since synckit.Version) {
			// Optional: record metrics, log activity, etc.
			logger.Debug("BeforePull hook called", slog.String("since_version", since.String()))
		},
	}
	
	// Create HTTP handler with hooks
	handler := httptransport.NewSyncHandlerWithHooks(store, logger, nil, nil, hooks)
	
	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/sync/", handler)
	
	// Add a simple endpoint to query the read model
	mux.HandleFunc("/stats/users", func(w http.ResponseWriter, r *http.Request) {
		count, err := projector.getUserCount(r.Context())
		if err != nil {
			logger.Error("Failed to get user count", slog.String("error", err.Error()))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"user_count": %d}`, count)
	})
	
	// Add a simple health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "healthy", "hooks_enabled": true}`)
	})
	
	logger.Info("Server starting on :8080")
	logger.Info("Endpoints:")
	logger.Info("  POST /sync/push - Push events (triggers AfterCommit hook)")
	logger.Info("  GET  /sync/pull?since=<version> - Pull events (triggers BeforePull hook)")
	logger.Info("  GET  /stats/users - Get current user count from read model")
	logger.Info("  GET  /health - Health check")
	
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
