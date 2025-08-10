package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
)

func main() {
	// Initialize structured logging from environment
	config := logging.GetConfigFromEnv()
	logging.Init(config)

	ctx := context.Background()

	// Demonstrate basic structured logging
	logging.Info("Application starting",
		slog.String("version", "1.0.0"),
		slog.String("environment", config.Environment),
		slog.Time("start_time", time.Now()),
	)

	// Demonstrate child loggers with component context
	dbLogger := logging.WithComponent(logging.Component("database"))
	dbLogger.Info("Database connection established",
		slog.String("host", "localhost"),
		slog.Int("port", 5432),
		slog.String("database", "sync_kit"),
	)

	// Demonstrate operation logging with duration tracking
	err := logging.LogOperation(ctx, 
		logging.Operation("user-authentication"), 
		logging.Component("auth-service"),
		func() error {
			// Simulate some work
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	)
	
	if err != nil {
		logging.Error("Authentication operation failed", 
			slog.String("error", err.Error()))
	}

	// Demonstrate structured error logging with custom SyncError
	syncErr := &errors.SyncError{
		Op:        errors.OpStore,
		Component: "sqlite",
		Code:      errors.ErrCodeStorageFailure,
		Kind:      errors.KindInternal,
		Err:       fmt.Errorf("disk space insufficient"),
		Retryable: true,
		Metadata: map[string]interface{}{
			"disk_usage": "95%",
			"required_space": "100MB",
			"available_space": "10MB",
		},
	}

	logging.LogError(ctx, syncErr, "Storage operation failed",
		slog.String("table", "events"),
		slog.Int("record_count", 1500),
	)

	// Demonstrate grouped attributes
	logging.Info("HTTP request completed",
		slog.Group("request",
			slog.String("method", "POST"),
			slog.String("path", "/api/sync"),
			slog.String("user_id", "user123"),
		),
		slog.Group("response", 
			slog.Int("status", 200),
			slog.Duration("latency", 150*time.Millisecond),
			slog.Int("body_size", 2048),
		),
		slog.Group("metrics",
			slog.Int("db_queries", 3),
			slog.Duration("db_time", 45*time.Millisecond),
		),
	)

	// Demonstrate context-aware logging
	ctx = context.WithValue(ctx, "request_id", "req-abc123")
	ctx = context.WithValue(ctx, "trace_id", "trace-xyz789")
	
	contextLogger := logging.WithContext(ctx, 
		slog.String("user_id", "user456"),
		slog.String("session_id", "sess-def456"),
	)
	
	contextLogger.Info("Processing user request",
		slog.String("action", "sync_data"),
		slog.Int("items_to_sync", 25),
	)

	// Demonstrate different log levels
	logging.Debug("Debug information",
		slog.Any("config", map[string]interface{}{
			"timeout": "30s",
			"retries": 3,
		}),
	)

	logging.Warn("Rate limit approaching",
		slog.Int("current_requests", 95),
		slog.Int("limit", 100),
		slog.Duration("reset_in", 30*time.Second),
	)

	// Demonstrate custom log levels  
	logging.Trace("Detailed trace information",
		slog.String("function", "processRequest"),
		slog.Any("parameters", []string{"param1", "param2"}),
	)

	// Demonstrate dynamic level changes
	if os.Getenv("DEMO_DYNAMIC_LEVEL") == "true" {
		logger, levelVar := logging.NewLoggerWithDynamicLevel(config)
		
		logger.Debug("This won't appear at INFO level")
		logger.Info("This will appear")
		
		// Change to debug level at runtime
		levelVar.SetFromString("debug")
		logger.Debug("This will now appear after level change")
	}

	logging.Info("Application shutdown complete",
		slog.Duration("uptime", 200*time.Millisecond),
		slog.String("reason", "demo_complete"),
	)
}
