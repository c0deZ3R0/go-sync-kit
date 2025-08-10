package logging

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/errors"
)

func TestLogger(t *testing.T) {
	// Test different environments
	configs := []Config{
		{Level: "debug", Format: "text", Environment: EnvDevelopment, AddSource: true},
		{Level: "info", Format: "json", Environment: EnvProduction, AddSource: false},
	}

	for _, config := range configs {
		t.Run("Environment_"+config.Environment, func(t *testing.T) {
			logger := NewLogger(config)

			// Test basic logging
			logger.Debug("Debug message", slog.String("key", "value"))
			logger.Info("Info message", slog.Int("count", 42))
			logger.Warn("Warning message", slog.Bool("enabled", true))

			// Test error logging
			testErr := errors.New(errors.OpStore, fmt.Errorf("storage error"))
			logger.LogError(context.Background(), testErr, "Operation failed")

			// Test child loggers
			childLogger := logger.WithComponent(Component("test"))
			childLogger.Info("Child logger message")

			// Test operation logging
			err := logger.LogOperation(
				context.Background(),
				Operation("test_op"),
				Component("test_component"),
				func() error {
					time.Sleep(10 * time.Millisecond)
					return nil
				},
			)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDynamicLevel(t *testing.T) {
	config := Config{
		Level:       "info",
		Format:      "text",
		Environment: EnvTest,
		AddSource:   false,
	}

	logger, levelVar := NewLoggerWithDynamicLevel(config)

	// Initially at info level - debug should not appear
	logger.Debug("This should not appear")
	logger.Info("This should appear")

	// Change to debug level
	levelVar.SetFromString("debug")
	logger.Debug("This should now appear")

	// Test custom levels
	logger.Log(context.Background(), slog.Level(LevelTrace), "Trace message")
}

func TestSyncErrorValuer(t *testing.T) {
	syncErr := &errors.SyncError{
		Op:        errors.OpSync,
		Component: "test",
		Code:      errors.ErrCodeStorageFailure,
		Kind:      errors.KindInternal,
		Err:       fmt.Errorf("underlying error"),
		Retryable: true,
		Metadata: map[string]interface{}{
			"retry_count": 3,
			"timeout":     "30s",
		},
	}

	valuer := SyncErrorValuer{SyncError: syncErr}
	logValue := valuer.LogValue()

	// Verify the log value is properly structured
	if logValue.Kind() != slog.KindGroup {
		t.Errorf("Expected group value, got %v", logValue.Kind())
	}
}

func TestContextExtraction(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	ctx = context.WithValue(ctx, "trace_id", "trace-456")

	logger := NewLogger(Config{Level: "debug", Format: "text", Environment: EnvTest})
	contextLogger := logger.WithContext(ctx)

	contextLogger.Info("Message with context")
}

func BenchmarkLogger(b *testing.B) {
	config := Config{
		Level:       "info",
		Format:      "json",
		Environment: EnvProduction,
		AddSource:   false,
	}
	logger := NewLogger(config)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "Benchmark message",
			slog.String("operation", "benchmark"),
			slog.Int("iteration", i),
			slog.Duration("elapsed", time.Microsecond*100),
		)
	}
}
