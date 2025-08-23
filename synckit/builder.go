package synckit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"github.com/c0deZ3R0/go-sync-kit/logging"
)

// ProjectionRunner is an interface that represents a projection runner.
// This is an alias to avoid import cycles with the projection package.
type ProjectionRunner interface {
	// ApplySince applies all events since the last saved offset
	ApplySince(ctx context.Context) (applied int, last Version, err error)
	
	// ApplyBatch applies a specific batch of events directly
	ApplyBatch(ctx context.Context, batch []EventWithVersion) error
}

// ProjectionConfig holds configuration for projection support in SyncManager.
type ProjectionConfig struct {
	// Runners are the projection runners to execute after successful sync
	Runners []ProjectionRunner
	
	// RunOnSync enables automatic projection execution after sync
	RunOnSync bool
	
	// MaxWorkers is the maximum number of concurrent projection workers
	MaxWorkers int
	
	// Timeout is the timeout for projection operations
	Timeout time.Duration
}

// SyncManagerBuilder provides a fluent interface for constructing SyncManager instances.
type SyncManagerBuilder struct {
	store       EventStore
	transport   Transport
	options     *SyncOptions
	logger      *slog.Logger
	pushOnlySet bool // Track if PushOnly was explicitly set
	pullOnlySet bool // Track if PullOnly was explicitly set
	
	// Projection support
	projectionRunners     []ProjectionRunner
	runProjectionsOnSync  bool
	projectionMaxWorkers  int
	projectionTimeout     time.Duration
}

// NewSyncManagerBuilder creates a new builder with default options.
func NewSyncManagerBuilder() *SyncManagerBuilder {
	return &SyncManagerBuilder{
		options: &SyncOptions{
			BatchSize:         100, // Default batch size
			EnableValidation:  false,
			Timeout:           0, // No timeout by default
			EnableCompression: false,
		},
		logger:               logging.Default().Logger, // Use default logger
		projectionMaxWorkers: 3,                         // Default to 3 concurrent projections
		projectionTimeout:    30 * time.Second,          // Default timeout for projections
	}
}

// WithStore sets the EventStore for the SyncManager.
func (b *SyncManagerBuilder) WithStore(store EventStore) *SyncManagerBuilder {
	b.store = store
	return b
}

// WithTransport sets the Transport for the SyncManager.
func (b *SyncManagerBuilder) WithTransport(transport Transport) *SyncManagerBuilder {
	b.transport = transport
	return b
}

// WithBatchSize sets the batch size for sync operations.
func (b *SyncManagerBuilder) WithBatchSize(size int) *SyncManagerBuilder {
	b.options.BatchSize = size
	return b
}

// WithPushOnly configures the SyncManager to only push events.
func (b *SyncManagerBuilder) WithPushOnly() *SyncManagerBuilder {
	b.options.PushOnly = true
	b.options.PullOnly = false
	b.pushOnlySet = true
	return b
}

// WithPullOnly configures the SyncManager to only pull events.
func (b *SyncManagerBuilder) WithPullOnly() *SyncManagerBuilder {
	b.options.PullOnly = true
	b.options.PushOnly = false
	b.pullOnlySet = true
	return b
}

// WithConflictResolver sets the conflict resolution strategy.
func (b *SyncManagerBuilder) WithConflictResolver(resolver ConflictResolver) *SyncManagerBuilder {
	b.options.ConflictResolver = resolver
	return b
}

// WithFilter sets an event filter function.
func (b *SyncManagerBuilder) WithFilter(filter func(Event) bool) *SyncManagerBuilder {
	b.options.Filter = filter
	return b
}

// WithSyncInterval sets the interval for automatic synchronization.
func (b *SyncManagerBuilder) WithSyncInterval(interval time.Duration) *SyncManagerBuilder {
	b.options.SyncInterval = interval
	return b
}

// WithValidation enables additional validation checks during sync operations.
func (b *SyncManagerBuilder) WithValidation() *SyncManagerBuilder {
	b.options.EnableValidation = true
	return b
}

// WithTimeout sets the maximum duration for sync operations.
func (b *SyncManagerBuilder) WithTimeout(timeout time.Duration) *SyncManagerBuilder {
	b.options.Timeout = timeout
	return b
}

// WithCompression enables data compression during transport.
func (b *SyncManagerBuilder) WithCompression(enabled bool) *SyncManagerBuilder {
	b.options.EnableCompression = enabled
	return b
}

// WithLogger sets a custom logger for the SyncManager.
func (b *SyncManagerBuilder) WithLogger(logger *slog.Logger) *SyncManagerBuilder {
	b.logger = logger
	return b
}

// WithTracer sets a tracer for distributed tracing.
func (b *SyncManagerBuilder) WithTracer(tracer interface {
	StartSyncOperation(ctx context.Context, operation string) (context.Context, trace.Span)
	StartTransportOperation(ctx context.Context, operation, transport string) (context.Context, trace.Span)
	StartStorageOperation(ctx context.Context, operation, storageType string) (context.Context, trace.Span)
	StartConflictResolution(ctx context.Context, strategy string) (context.Context, trace.Span)
	RecordError(span trace.Span, err error, description string)
	SetSyncResult(span trace.Span, eventsPushed, eventsPulled, conflictsResolved int)
}) *SyncManagerBuilder {
	b.options.Tracer = tracer
	return b
}

// WithMetricsCollector sets a metrics collector for observability.
func (b *SyncManagerBuilder) WithMetricsCollector(collector MetricsCollector) *SyncManagerBuilder {
	b.options.MetricsCollector = collector
	return b
}

// WithHealthChecker sets a health checker for monitoring sync-kit component health.
func (b *SyncManagerBuilder) WithHealthChecker(checker interface{}) *SyncManagerBuilder {
	// In a real implementation, you would store this in SyncOptions or
	// configure the SyncManager with health checking capabilities
	// For now, we'll store it as a generic interface
	// TODO: Implement health checker integration
	return b
}

// WithProjections adds projection runners to execute after successful sync.
func (b *SyncManagerBuilder) WithProjections(runners ...ProjectionRunner) *SyncManagerBuilder {
	b.projectionRunners = append(b.projectionRunners, runners...)
	return b
}

// WithProjectionsOnSync enables automatic projection execution after sync.
func (b *SyncManagerBuilder) WithProjectionsOnSync(enabled bool) *SyncManagerBuilder {
	b.runProjectionsOnSync = enabled
	return b
}

// WithProjectionMaxWorkers sets the maximum number of concurrent projection workers.
func (b *SyncManagerBuilder) WithProjectionMaxWorkers(workers int) *SyncManagerBuilder {
	if workers > 0 {
		b.projectionMaxWorkers = workers
	}
	return b
}

// WithProjectionTimeout sets the timeout for projection operations.
func (b *SyncManagerBuilder) WithProjectionTimeout(timeout time.Duration) *SyncManagerBuilder {
	b.projectionTimeout = timeout
	return b
}

// Build creates a new SyncManager instance with the configured options.
func (b *SyncManagerBuilder) Build() (SyncManager, error) {
	// Validate required components
	if b.store == nil {
		return nil, fmt.Errorf("EventStore is required")
	}
	if b.transport == nil {
		return nil, fmt.Errorf("Transport is required")
	}

	// Validate push/pull settings - check if both were explicitly set
	if b.pushOnlySet && b.pullOnlySet {
		return nil, fmt.Errorf("cannot set both PushOnly and PullOnly to true")
	}

	// Validate batch size
	if b.options.BatchSize <= 0 {
		return nil, fmt.Errorf("BatchSize must be positive, got %d", b.options.BatchSize)
	}

	// Create projection configuration
	projectionConfig := &ProjectionConfig{
		Runners:     b.projectionRunners,
		RunOnSync:   b.runProjectionsOnSync,
		MaxWorkers:  b.projectionMaxWorkers,
		Timeout:     b.projectionTimeout,
	}
	
	// Create a new SyncManager instance with projection support
	return NewSyncManager(b.store, b.transport, b.options, b.logger, projectionConfig), nil
}

// Reset clears the builder, allowing reuse.
func (b *SyncManagerBuilder) Reset() *SyncManagerBuilder {
	b.store = nil
	b.transport = nil
	b.options = &SyncOptions{
		BatchSize:         100,
		EnableValidation:  false,
		Timeout:           0,
		EnableCompression: false,
	}
	b.logger = logging.Default().Logger
	b.pushOnlySet = false
	b.pullOnlySet = false
	
	// Reset projection fields
	b.projectionRunners = nil
	b.runProjectionsOnSync = false
	b.projectionMaxWorkers = 3
	b.projectionTimeout = 30 * time.Second
	
	return b
}
