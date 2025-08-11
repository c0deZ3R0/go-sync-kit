package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/dashboard"
	"github.com/c0deZ3R0/go-sync-kit/http/httptransport"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/metrics"
	"github.com/c0deZ3R0/go-sync-kit/store/sqlite"
)

// ServerEvent represents a server-generated event
type ServerEvent struct {
	eventID       string
	eventType     string
	noteID        string
	content       string
	timestamp     time.Time
	eventMetadata map[string]interface{}
}

// Required methods for Event interface
func (e *ServerEvent) ID() string                       { return e.eventID }
func (e *ServerEvent) Type() string                     { return e.eventType }
func (e *ServerEvent) Time() time.Time                  { return e.timestamp }
func (e *ServerEvent) Data() interface{}                { return e }
func (e *ServerEvent) AggregateID() string              { return e.noteID }
func (e *ServerEvent) Metadata() map[string]interface{} { return e.eventMetadata }

// example/server.go
func RunServer(ctx context.Context) error {
	// Initialize structured logging
	logConfig := logging.GetConfigFromEnv()
	logging.Init(logConfig)
	logger := logging.WithComponent(logging.Component("server"))

	logger.InfoContext(ctx, "Starting sync server",
		slog.String("version", "1.0.0"),
		slog.String("environment", logConfig.Environment),
	)

	// Initialize dashboard metrics
	dashboard.InitializeMetrics()

	// Create metrics collector
	metricsCollector := metrics.NewHTTPMetricsCollector()

	// Create SQLite event store
	storeConfig := &sqlite.Config{
		DataSourceName: "file:notes.db",
		EnableWAL:      true,
	}
	store, err := sqlite.New(storeConfig)
	if err != nil {
		return fmt.Errorf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Set up HTTP mux with multiple endpoints
	mux := http.NewServeMux()

	// Sync handler (using standard log for compatibility with transport)
	stdLogger := log.New(os.Stdout, "[SyncHandler] ", log.LstdFlags)
	syncHandler := transport.NewSyncHandler(store, stdLogger)
	mux.Handle("/sync/", syncHandler) // Note the trailing slash

	// Metrics endpoint
	mux.Handle("/metrics", metricsCollector)
	mux.HandleFunc("/dashboard-metrics", dashboard.MetricsHandler)

	// Dashboard and events endpoints
	mux.Handle("/", dashboard.Handler())
	mux.HandleFunc("/events", dashboard.EventsHandler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check if store is accessible
		hcCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if _, err := store.LatestVersion(hcCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Create server with timeouts
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start event generator with metrics
	go generateServerEvents(ctx, store, metricsCollector)

	// Start server
	logger.InfoContext(ctx, "HTTP server starting",
		slog.String("address", ":8080"),
		slog.Group("endpoints",
			slog.String("/", "Dashboard UI"),
			slog.String("/sync", "Sync endpoint"),
			slog.String("/metrics", "Metrics endpoint"),
			slog.String("/health", "Health check"),
		),
	)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.LogError(ctx, err, "HTTP server error")
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	logger.InfoContext(ctx, "Shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %v", err)
	}

	return nil
}

// Update event generator to track metrics
func generateServerEvents(ctx context.Context, store *sqlite.SQLiteEventStore, metrics *metrics.HTTPMetricsCollector) {
	logger := logging.WithComponent(logging.Component("event-generator"))
	logger.InfoContext(ctx, "Event generator started",
		slog.Duration("interval", 10*time.Second),
	)

	// Keep track of cumulative events pushed
	cumulativePushed := 0
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Event templates with more variety
	eventTemplates := []struct {
		Type     string
		Contents []string
	}{
		{
			Type: "server_notification",
			Contents: []string{
				"System maintenance scheduled",
				"New feature deployed",
				"Performance optimization complete",
			},
		},
		{
			Type: "system_update",
			Contents: []string{
				"Database backup completed",
				"Cache cleared successfully",
				"Configuration updated",
			},
		},
		{
			Type: "user_alert",
			Contents: []string{
				"New user registered",
				"Subscription renewed",
				"Payment processed",
			},
		},
	}

	eventCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.InfoContext(ctx, "Event generator stopped")
			return
		case <-ticker.C:
			// Get current version
			version, err := store.LatestVersion(ctx)
			if err != nil {
				logging.LogError(ctx, err, "Failed to get latest version")
				continue
			}

			// Select random template
			template := eventTemplates[rand.Intn(len(eventTemplates))]
			content := template.Contents[rand.Intn(len(template.Contents))]

			event := &ServerEvent{
				eventID:   fmt.Sprintf("srv_%d_%d", time.Now().UnixNano(), eventCount),
				eventType: template.Type,
				noteID:    fmt.Sprintf("note_%d", eventCount),
				content:   content,
				timestamp: time.Now(),
				eventMetadata: map[string]interface{}{
					"author":   "server",
					"priority": rand.Intn(5) + 1,
					"version":  version.String(),
					"hostname": getHostname(),
				},
			}

			start := time.Now()
			if err := store.Store(ctx, event, nil); err != nil {
				logging.LogError(ctx, err, "Failed to store server event",
					slog.String("event_id", event.eventID),
					slog.String("event_type", event.eventType),
				)
				metrics.RecordSyncErrors("store", "storage")
				continue
			}

			elapsed := time.Since(start)
			metrics.RecordSyncDuration("store", elapsed)
			metrics.RecordSyncEvents(1, 0) // 1 event generated/pushed
			cumulativePushed++

			// Update dashboard metrics
			dashboard.UpdateMetrics(dashboard.Metrics{
				Events: struct {
					Pushed int `json:"pushed"`
					Pulled int `json:"pulled"`
				}{
					Pushed: cumulativePushed,
					Pulled: 0,
				},
				ConflictsResolved: 0,
				DurationsMs: struct {
					PushTotal int `json:"push_total"`
					PullTotal int `json:"pull_total"`
					SyncTotal int `json:"sync_total"`
				}{
					PushTotal: int(elapsed.Milliseconds()),
					PullTotal: 0,
					SyncTotal: int(elapsed.Milliseconds()),
				},
				LastSync: time.Now().Format(time.RFC3339),
				Errors:   make(map[string]int),
			})

			// Add event to dashboard
			dashboard.AddEvent(dashboard.EventLog{
				ID:        event.eventID,
				Type:      event.eventType,
				Content:   event.content,
				Timestamp: event.timestamp.Format(time.RFC3339),
				Metadata:  event.eventMetadata,
			})

			logger.DebugContext(ctx, "Event generated and stored",
				slog.Int("event_count", eventCount),
				slog.String("event_id", event.eventID),
				slog.String("event_type", event.eventType),
				slog.String("content", event.content),
				slog.Duration("store_duration", elapsed),
				slog.Int("cumulative_pushed", cumulativePushed),
			)
			eventCount++
		}
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
