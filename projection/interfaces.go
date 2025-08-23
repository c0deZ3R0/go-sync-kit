// Package projection provides read-model building capabilities for go-sync-kit.
// It enables CQRS, event sourcing, and offline-first architectures by allowing
// applications to build deterministic, idempotent projections from event streams.
package projection

import (
	"context"
	"log/slog"

	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/observability/metrics"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// OffsetStore persists the last applied authoritative version per projection name.
// This enables resumable projection processing and idempotent operations.
type OffsetStore interface {
	// Get retrieves the last applied version for a projection.
	// Returns nil if no offset has been stored yet (start from beginning).
	Get(ctx context.Context, name string) (synckit.Version, error)

	// Set updates the last applied version for a projection.
	// This should be called atomically after successfully applying events.
	Set(ctx context.Context, name string, v synckit.Version) error
}

// Projector applies domain changes from events to a read model.
// Implementations must be idempotent - applying the same events multiple times
// should produce the same result.
type Projector interface {
	// Name returns a stable identifier used for offset bookkeeping.
	// This name should be unique across all projectors in an application.
	Name() string

	// Apply applies a batch of events to the read model.
	// Must be idempotent - applying the same events multiple times should be safe.
	// Events are provided in order and should be processed sequentially.
	Apply(ctx context.Context, batch []synckit.EventWithVersion) error
}

// Runner coordinates loading events from EventStore since the last offset
// and applying them via a Projector. It handles batching, error recovery,
// and progress tracking.
type Runner interface {
	// ApplySince applies all events since the last saved offset.
	// Returns the number of events applied, the last processed version, and any error.
	// This method is idempotent and can be called multiple times safely.
	ApplySince(ctx context.Context) (applied int, last synckit.Version, err error)

	// ApplyBatch applies a specific batch of events directly.
	// This is useful for server-side hooks that want to apply events immediately
	// after they are committed to storage.
	ApplyBatch(ctx context.Context, batch []synckit.EventWithVersion) error
}

// RunnerOption configures a Runner using the functional options pattern.
type RunnerOption func(*runner)

// WithBatchSize sets the batch size for processing events.
// Default is 500 events per batch.
func WithBatchSize(n int) RunnerOption {
	return func(r *runner) { 
		if n > 0 {
			r.batchSize = n 
		}
	}
}

// WithLogger sets a custom structured logger for the runner.
// If not provided, uses the default logger from the logging package.
func WithLogger(logger *slog.Logger) RunnerOption {
	return func(r *runner) { 
		if logger != nil {
			r.logger = logger 
		}
	}
}

// WithMetrics enables metrics collection using the provided SyncKitMetrics instance.
// This integrates the runner with the unified observability system.
func WithMetrics(metricsCollector *metrics.SyncKitMetrics) RunnerOption {
	return func(r *runner) { 
		r.metricsEnabled = true
		r.metrics = metricsCollector
	}
}

// WithMetricsEnabled enables basic metrics collection using the legacy system.
// For new code, prefer WithMetrics() with SyncKitMetrics for better integration.
func WithMetricsEnabled(enabled bool) RunnerOption {
	return func(r *runner) { 
		r.metricsEnabled = enabled
		// Try to use default projection metrics if available
		if enabled && r.metrics == nil {
			// Fallback to legacy projection metrics system
			// Note: This maintains backward compatibility
		}
	}
}

// NewRunner creates a new projection runner with the given components and options.
// The runner will coordinate loading events from the EventStore since the last
// offset stored in OffsetStore and applying them via the Projector.
func NewRunner(store synckit.EventStore, offsets OffsetStore, proj Projector, opts ...RunnerOption) Runner {
	r := &runner{
		store:     store,
		offsets:   offsets,
		projector: proj,
		batchSize: 500, // default batch size
		logger:    logging.Default().Logger,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

