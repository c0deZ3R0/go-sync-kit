package synckit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// CursorMode specifies the versioning strategy for sync operations.
type CursorMode int

const (
	// CursorInteger uses simple integer-based versioning (default).
	CursorInteger CursorMode = iota

	// CursorVector uses vector clock-based versioning for distributed scenarios.
	CursorVector
)

// RetryPolicy defines the backoff and retry behavior for sync operations.
type RetryPolicy struct {
	// Max is the maximum number of retry attempts (0 = no retries, <0 = unlimited).
	Max int

	// Base is the initial delay between retries (must be > 0 if retries enabled).
	Base time.Duration

	// Cap is the maximum delay between retries (must be >= Base).
	Cap time.Duration

	// Jitter adds randomness to retry delays to avoid thundering herd (recommended: true).
	Jitter bool
}

// Config provides a canonical configuration structure for creating a SyncManager.
// It offers a declarative alternative to functional options, with built-in validation.
//
// Example usage:
//
//	cfg := synckit.Config{
//	    Store:     myStore,
//	    Transport: myTransport,
//	    Logger:    slog.Default(),
//	    Cursor:    synckit.CursorInteger,
//	    Retry: synckit.RetryPolicy{
//	        Max:    3,
//	        Base:   100 * time.Millisecond,
//	        Cap:    5 * time.Second,
//	        Jitter: true,
//	    },
//	    Timeout: 30 * time.Second,
//	}
//
//	mgr, err := synckit.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
type Config struct {
	// Store provides local event persistence (required).
	Store EventStore

	// Transport handles network communication (optional for local-only scenarios).
	Transport Transport

	// Logger for structured logging (optional; defaults to slog.Default()).
	Logger *slog.Logger

	// Cursor specifies the versioning strategy (Integer or Vector).
	// Default: CursorInteger.
	Cursor CursorMode

	// Retry defines backoff/retry policy for transient failures.
	// Default: no retries.
	Retry RetryPolicy

	// Resolvers is the conflict resolution registry (optional).
	// If nil, uses Last-Write-Wins as default resolver.
	// Note: ResolverRegistry type is forward-compatible placeholder; use ConflictResolver for now.
	Resolvers ConflictResolver

	// Timeout is the maximum duration for sync operations (0 = no timeout).
	// Default: 0 (no timeout).
	Timeout time.Duration

	// BatchSize limits the number of events to sync at once.
	// Default: 100.
	BatchSize int

	// SyncInterval for automatic periodic sync (0 = disabled).
	// Default: 0 (manual sync only).
	SyncInterval time.Duration

	// PushOnly restricts sync to only pushing local events (no pull).
	// Default: false.
	PushOnly bool

	// PullOnly restricts sync to only pulling remote events (no push).
	// Default: false.
	PullOnly bool

	// EnableValidation enables additional validation checks during sync.
	// Default: false.
	EnableValidation bool

	// EnableCompression enables data compression during transport.
	// Default: false.
	EnableCompression bool

	// Filter is an optional event filter function.
	// Events are synced only if Filter returns true.
	Filter func(Event) bool

	// MetricsCollector for observability hooks (optional).
	MetricsCollector MetricsCollector

	// Tracer for distributed tracing (optional).
	Tracer interface {
		StartSyncOperation(ctx context.Context, operation string) (context.Context, trace.Span)
		StartTransportOperation(ctx context.Context, operation, transport string) (context.Context, trace.Span)
		StartStorageOperation(ctx context.Context, operation, storageType string) (context.Context, trace.Span)
		StartConflictResolution(ctx context.Context, strategy string) (context.Context, trace.Span)
		RecordError(span trace.Span, err error, description string)
		SetSyncResult(span trace.Span, eventsPushed, eventsPulled, conflictsResolved int)
	}
}

// Validate checks the configuration for correctness and returns an error if invalid.
// It enforces required fields, sane timeouts, and retry bounds.
func (c *Config) Validate() error {
	// Required: Store
	if c.Store == nil {
		return errors.New("Store is required")
	}

	// Timeout must be non-negative
	if c.Timeout < 0 {
		return errors.New("Timeout must be non-negative")
	}

	// BatchSize must be positive if set
	if c.BatchSize < 0 {
		return errors.New("BatchSize must be non-negative")
	}

	// SyncInterval must be non-negative
	if c.SyncInterval < 0 {
		return errors.New("SyncInterval must be non-negative")
	}

	// PushOnly and PullOnly are mutually exclusive
	if c.PushOnly && c.PullOnly {
		return errors.New("PushOnly and PullOnly are mutually exclusive")
	}

	// Validate RetryPolicy if retries are enabled
	if c.Retry.Max != 0 {
		if c.Retry.Max < -1 {
			return errors.New("Retry.Max must be >= -1 (-1 for unlimited, 0 for disabled, >0 for limited)")
		}
		if c.Retry.Base <= 0 {
			return errors.New("Retry.Base must be > 0 when retries are enabled")
		}
		if c.Retry.Cap < c.Retry.Base {
			return fmt.Errorf("Retry.Cap (%v) must be >= Retry.Base (%v)", c.Retry.Cap, c.Retry.Base)
		}
	}

	// Cursor mode validation (currently only Integer and Vector are defined)
	if c.Cursor != CursorInteger && c.Cursor != CursorVector {
		return fmt.Errorf("invalid CursorMode: %d (must be CursorInteger or CursorVector)", c.Cursor)
	}

	return nil
}

// New is the canonical constructor for creating a SyncManager from a Config.
// It validates the configuration and wires up the manager using the builder internally.
//
// This is the recommended entrypoint for applications. Functional options (WithX)
// remain supported for advanced use cases and backward compatibility.
//
// Example:
//
//	cfg := synckit.Config{
//	    Store:     store,
//	    Transport: transport,
//	    Timeout:   30 * time.Second,
//	}
//	mgr, err := synckit.New(cfg)
func New(cfg Config) (SyncManager, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return newManagerFromConfig(cfg)
}

// newManagerFromConfig is the internal constructor that wires up a SyncManager from Config.
// It uses the existing SyncManagerBuilder to avoid duplication.
func newManagerFromConfig(cfg Config) (SyncManager, error) {
	builder := NewSyncManagerBuilder()

	// Required fields
	builder.WithStore(cfg.Store)

	// Optional Transport
	if cfg.Transport != nil {
		builder.WithTransport(cfg.Transport)
	}

	// Logger
	if cfg.Logger != nil {
		builder.WithLogger(cfg.Logger)
	}

	// Conflict resolver
	if cfg.Resolvers != nil {
		builder.WithConflictResolver(cfg.Resolvers)
	}

	// Sync options
	if cfg.BatchSize > 0 {
		builder.WithBatchSize(cfg.BatchSize)
	}

	if cfg.Timeout > 0 {
		builder.WithTimeout(cfg.Timeout)
	}

	if cfg.SyncInterval > 0 {
		builder.WithSyncInterval(cfg.SyncInterval)
	}

	if cfg.PushOnly {
		builder.WithPushOnly()
	}

	if cfg.PullOnly {
		builder.WithPullOnly()
	}

	if cfg.EnableValidation {
		builder.WithValidation()
	}

	if cfg.EnableCompression {
		builder.WithCompression(true)
	}

	if cfg.Filter != nil {
		builder.WithFilter(cfg.Filter)
	}

	// Observability
	if cfg.MetricsCollector != nil {
		builder.WithMetricsCollector(cfg.MetricsCollector)
	}

	if cfg.Tracer != nil {
		builder.WithTracer(cfg.Tracer)
	}

	// Note: CursorMode and RetryPolicy are currently not wired into the builder.
	// These are placeholders for future enhancements. For now, they are validated
	// but not actively used in manager construction.
	// TODO: Wire CursorMode and RetryPolicy into the builder when underlying
	// state machine and transport layers support these features.

	// Build the manager
	return builder.Build()
}
