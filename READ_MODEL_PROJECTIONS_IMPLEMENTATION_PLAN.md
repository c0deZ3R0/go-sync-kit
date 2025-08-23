# Read-Model Projections Implementation Plan for go-sync-kit

## Executive Summary

This document outlines a comprehensive plan to add **read-model building/syncing capabilities** with CQRS, event sourcing, and offline-first support to the go-sync-kit library. The implementation maintains the server as the source of truth while enabling deterministic, idempotent projection execution on both clients and servers. The design is non-breaking and follows the existing architectural patterns in the codebase.

## Current Architecture Analysis

### Core Components Understanding

Based on deep analysis of the codebase:

1. **synckit/** - Core interfaces (Event, Version, EventStore, Transport, SyncManager)
   - Uses functional options pattern via `manager_options.go`
   - Supports builder pattern via `builder.go`
   - Integrated state machine support for sync operations
   - Metrics collection and observability hooks already in place

2. **storage/sqlite/** - SQLite implementation with:
   - WAL mode enabled by default for concurrency
   - Connection pooling with sensible defaults
   - Cursor-based versioning using `cursor.IntegerCursor`

3. **transport/httptransport/** - HTTP transport with:
   - Separate client (`client.go`) and server handler (`http.go`)
   - Compression support with gzip
   - Size limit protections
   - Wire format for event serialization

4. **version/** - Version management with:
   - Vector clock support for distributed systems
   - VersionedStore decorator pattern
   - Automatic version management

5. **errors/** - Structured error handling with:
   - Operation and Component tracking
   - Error wrapping and context preservation

## Implementation Phases

### Phase 1: Projection API Interfaces (Non-breaking)

**Goal**: Introduce minimal, general projection API that is idempotent, batch-capable, and resumable.

#### 1.1 Create New Package Structure

```bash
projection/
├── interfaces.go       # Core interfaces
├── runner.go           # Runner implementation
├── metrics.go          # Projection-specific metrics
└── sqlite/
    └── offsets.go      # SQLite offset store
```

#### 1.2 Core Interfaces (`projection/interfaces.go`)

```go
package projection

import (
    "context"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// OffsetStore persists the last applied authoritative version per projection name.
type OffsetStore interface {
    // Get retrieves the last applied version for a projection
    Get(ctx context.Context, name string) (synckit.Version, error)
    
    // Set updates the last applied version for a projection
    Set(ctx context.Context, name string, v synckit.Version) error
}

// Projector applies domain changes from events to a read model.
type Projector interface {
    // Name returns stable identifier used for offset bookkeeping
    Name() string
    
    // Apply applies a batch of events. Must be idempotent.
    Apply(ctx context.Context, batch []synckit.EventWithVersion) error
}

// Runner coordinates loading from EventStore since the last offset and applying via Projector.
type Runner interface {
    // ApplySince applies all events since the last saved offset
    ApplySince(ctx context.Context) (applied int, last synckit.Version, err error)
    
    // ApplyBatch applies a specific batch of events (for server-side hooks)
    ApplyBatch(ctx context.Context, batch []synckit.EventWithVersion) error
}

// RunnerOption configures a Runner
type RunnerOption func(*runner)

// WithBatchSize sets the batch size for processing
func WithBatchSize(n int) RunnerOption {
    return func(r *runner) { r.batchSize = n }
}

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) RunnerOption {
    return func(r *runner) { r.logger = logger }
}

// NewRunner creates a new projection runner
func NewRunner(store synckit.EventStore, offsets OffsetStore, proj Projector, opts ...RunnerOption) Runner {
    r := &runner{
        store:     store,
        offsets:   offsets,
        projector: proj,
        batchSize: 500, // default
        logger:    logging.Default().Logger,
    }
    
    for _, opt := range opts {
        opt(r)
    }
    
    return r
}
```

#### 1.3 Update Error Components (`errors/errors.go`)

Add new operation constants:

```go
const (
    // ... existing operations ...
    OpProjection      Operation = "projection"
    OpProjectionApply Operation = "projection_apply"
    OpOffsetStore     Operation = "offset_store"
)
```

### Phase 2: SQLite Offset Persistence

**Goal**: Production-ready OffsetStore implementation reusing EventStore's version parsing.

#### 2.1 SQLite Offset Store (`projection/sqlite/offsets.go`)

```go
package sqliteproj

import (
    "context"
    "database/sql"
    "fmt"
    
    "github.com/c0deZ3R0/go-sync-kit/synckit"
    "github.com/c0deZ3R0/go-sync-kit/projection"
    "github.com/c0deZ3R0/go-sync-kit/errors"
)

// OffsetStore implements projection.OffsetStore using SQLite
type OffsetStore struct {
    DB           *sql.DB
    ParseVersion func(ctx context.Context, s string) (synckit.Version, error)
    tableName    string
}

// OffsetStoreOption configures an OffsetStore
type OffsetStoreOption func(*OffsetStore)

// WithTableName sets a custom table name
func WithTableName(name string) OffsetStoreOption {
    return func(o *OffsetStore) { o.tableName = name }
}

// NewOffsetStore creates a new SQLite-backed offset store
func NewOffsetStore(db *sql.DB, parseVersion func(ctx context.Context, s string) (synckit.Version, error), opts ...OffsetStoreOption) (*OffsetStore, error) {
    o := &OffsetStore{
        DB:           db,
        ParseVersion: parseVersion,
        tableName:    "projection_offsets",
    }
    
    for _, opt := range opts {
        opt(o)
    }
    
    if err := o.ensure(context.Background()); err != nil {
        return nil, errors.E(
            errors.Op("NewOffsetStore"),
            errors.Component("projection/sqlite"),
            err,
        )
    }
    
    return o, nil
}

func (o *OffsetStore) ensure(ctx context.Context) error {
    query := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            name TEXT PRIMARY KEY,
            version TEXT NOT NULL,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`, o.tableName)
    
    _, err := o.DB.ExecContext(ctx, query)
    return err
}

func (o *OffsetStore) Get(ctx context.Context, name string) (synckit.Version, error) {
    query := fmt.Sprintf(`SELECT version FROM %s WHERE name = ?`, o.tableName)
    
    var versionStr string
    err := o.DB.QueryRowContext(ctx, query, name).Scan(&versionStr)
    if err == sql.ErrNoRows {
        return nil, nil // No offset yet, start from beginning
    }
    if err != nil {
        return nil, errors.E(
            errors.Op("OffsetStore.Get"),
            errors.Component("projection/sqlite"),
            err,
        )
    }
    
    return o.ParseVersion(ctx, versionStr)
}

func (o *OffsetStore) Set(ctx context.Context, name string, v synckit.Version) error {
    query := fmt.Sprintf(`
        INSERT INTO %s (name, version, updated_at)
        VALUES (?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(name) DO UPDATE SET 
            version = excluded.version,
            updated_at = CURRENT_TIMESTAMP
    `, o.tableName)
    
    _, err := o.DB.ExecContext(ctx, query, name, v.String())
    if err != nil {
        return errors.E(
            errors.Op("OffsetStore.Set"),
            errors.Component("projection/sqlite"),
            err,
        )
    }
    
    return nil
}
```

### Phase 3: Projection Runner Implementation

**Goal**: Robust, idempotent runner with batching, context handling, and progress tracking.

#### 3.1 Runner Implementation (`projection/runner.go`)

```go
package projection

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"
    
    "github.com/c0deZ3R0/go-sync-kit/synckit"
    "github.com/c0deZ3R0/go-sync-kit/errors"
    "github.com/c0deZ3R0/go-sync-kit/logging"
)

type runner struct {
    store     synckit.EventStore
    offsets   OffsetStore
    projector Projector
    batchSize int
    logger    *slog.Logger
    mu        sync.Mutex
}

func (r *runner) ApplySince(ctx context.Context) (int, synckit.Version, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    start := time.Now()
    
    // Get last processed offset
    since, err := r.offsets.Get(ctx, r.projector.Name())
    if err != nil {
        return 0, nil, errors.E(
            errors.OpProjection,
            errors.Component("projection"),
            fmt.Errorf("failed to get offset: %w", err),
        )
    }
    
    r.logger.Debug("Starting projection catch-up",
        slog.String("projector", r.projector.Name()),
        slog.String("since_version", fmt.Sprintf("%v", since)),
    )
    
    total := 0
    var lastVersion synckit.Version
    
    for {
        // Check context cancellation
        select {
        case <-ctx.Done():
            r.logger.Warn("Projection cancelled",
                slog.String("projector", r.projector.Name()),
                slog.Int("events_processed", total),
                slog.Duration("duration", time.Since(start)),
            )
            return total, lastVersion, ctx.Err()
        default:
        }
        
        // Load next batch
        events, err := r.store.Load(ctx, since)
        if err != nil {
            return total, lastVersion, errors.E(
                errors.OpProjection,
                errors.Component("projection"),
                fmt.Errorf("failed to load events: %w", err),
            )
        }
        
        if len(events) == 0 {
            r.logger.Debug("Projection caught up",
                slog.String("projector", r.projector.Name()),
                slog.Int("total_events", total),
                slog.Duration("duration", time.Since(start)),
            )
            return total, lastVersion, nil
        }
        
        // Apply batch size limit
        batch := events
        if r.batchSize > 0 && len(events) > r.batchSize {
            batch = events[:r.batchSize]
        }
        
        // Apply events to projection
        if err := r.projector.Apply(ctx, batch); err != nil {
            r.logger.Error("Failed to apply events to projection",
                slog.String("projector", r.projector.Name()),
                slog.Int("batch_size", len(batch)),
                slog.String("error", err.Error()),
            )
            return total, lastVersion, errors.E(
                errors.OpProjectionApply,
                errors.Component("projection"),
                fmt.Errorf("projector failed: %w", err),
            )
        }
        
        // Update offset to last event in batch
        lastVersion = batch[len(batch)-1].Version
        if err := r.offsets.Set(ctx, r.projector.Name(), lastVersion); err != nil {
            return total, lastVersion, errors.E(
                errors.OpOffsetStore,
                errors.Component("projection"),
                fmt.Errorf("failed to update offset: %w", err),
            )
        }
        
        total += len(batch)
        since = lastVersion
        
        r.logger.Debug("Applied projection batch",
            slog.String("projector", r.projector.Name()),
            slog.Int("batch_size", len(batch)),
            slog.Int("total_processed", total),
            slog.String("last_version", lastVersion.String()),
        )
        
        // Small sleep to prevent tight loop
        if len(batch) < r.batchSize {
            break // Caught up
        }
        time.Sleep(10 * time.Millisecond)
    }
    
    return total, lastVersion, nil
}

func (r *runner) ApplyBatch(ctx context.Context, batch []synckit.EventWithVersion) error {
    if len(batch) == 0 {
        return nil
    }
    
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Apply events
    if err := r.projector.Apply(ctx, batch); err != nil {
        return errors.E(
            errors.OpProjectionApply,
            errors.Component("projection"),
            fmt.Errorf("failed to apply batch: %w", err),
        )
    }
    
    // Update offset to last event
    lastVersion := batch[len(batch)-1].Version
    if err := r.offsets.Set(ctx, r.projector.Name(), lastVersion); err != nil {
        return errors.E(
            errors.OpOffsetStore,
            errors.Component("projection"),
            fmt.Errorf("failed to update offset: %w", err),
        )
    }
    
    r.logger.Debug("Applied direct batch to projection",
        slog.String("projector", r.projector.Name()),
        slog.Int("batch_size", len(batch)),
        slog.String("last_version", lastVersion.String()),
    )
    
    return nil
}
```

### Phase 4: SyncManager Integration

**Goal**: First-class library support to run projections automatically after sync.

#### 4.1 Add Manager Options (`synckit/manager_options.go`)

```go
// Add to existing manager_options.go

// WithProjections adds projection runners to execute after successful sync
func WithProjections(runners ...projection.Runner) ManagerOption {
    return func(b *SyncManagerBuilder) error {
        // Store runners in builder for later use
        // This will be added to the SyncManagerBuilder struct
        b.projectionRunners = append(b.projectionRunners, runners...)
        return nil
    }
}

// WithProjectionsOnSync enables automatic projection execution after sync
func WithProjectionsOnSync(enabled bool) ManagerOption {
    return func(b *SyncManagerBuilder) error {
        b.runProjectionsOnSync = enabled
        return nil
    }
}

// WithProjectionBatchSize sets the batch size for projection processing
func WithProjectionBatchSize(size int) ManagerOption {
    return func(b *SyncManagerBuilder) error {
        b.projectionBatchSize = size
        return nil
    }
}
```

#### 4.2 Update SyncManager (`synckit/manager.go`)

Add to syncManager struct:

```go
type syncManager struct {
    // ... existing fields ...
    
    // Projection support
    projectionRunners    []projection.Runner
    runProjectionsOnSync bool
    projectionBatchSize  int
    projectionPool       chan struct{} // Worker pool for concurrent projections
}
```

Add projection execution after successful sync:

```go
// In Sync() method, after successful sync (line ~195)
if sm.runProjectionsOnSync && len(sm.projectionRunners) > 0 && len(result.Errors) == 0 {
    sm.runProjections(ctx, result)
}

// New method to run projections
func (sm *syncManager) runProjections(ctx context.Context, syncResult *SyncResult) {
    if len(sm.projectionRunners) == 0 {
        return
    }
    
    sm.logger.Debug("Running projections after sync",
        slog.Int("runner_count", len(sm.projectionRunners)),
    )
    
    // Use worker pool to limit concurrent projections
    poolSize := 3 // Default pool size
    if sm.projectionPool == nil {
        sm.projectionPool = make(chan struct{}, poolSize)
    }
    
    var wg sync.WaitGroup
    for _, runner := range sm.projectionRunners {
        wg.Add(1)
        go func(r projection.Runner) {
            defer wg.Done()
            
            // Acquire worker slot
            sm.projectionPool <- struct{}{}
            defer func() { <-sm.projectionPool }()
            
            // Create timeout context
            projCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            defer cancel()
            
            applied, lastVersion, err := r.ApplySince(projCtx)
            if err != nil {
                sm.logger.Error("Projection failed",
                    slog.String("error", err.Error()),
                )
                // Record metrics
                if sm.options.MetricsCollector != nil {
                    sm.options.MetricsCollector.RecordSyncErrors("projection", "projection_failure")
                }
            } else {
                sm.logger.Info("Projection completed",
                    slog.Int("events_applied", applied),
                    slog.String("last_version", fmt.Sprintf("%v", lastVersion)),
                )
            }
        }(runner)
    }
    
    // Wait with timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        sm.logger.Debug("All projections completed")
    case <-time.After(60 * time.Second):
        sm.logger.Warn("Projections timed out")
    }
}
```

### Phase 5: Server-Side Projection Hooks

**Goal**: Enable server read models built only from server-committed events.

#### 5.1 Add Hooks to HTTP Handler (`transport/httptransport/http.go`)

```go
// Add to SyncHandler struct
type SyncHandler struct {
    // ... existing fields ...
    hooks *SyncHooks
}

// SyncHooks provides extensibility points for the sync handler
type SyncHooks struct {
    // AfterCommit is called after events are successfully committed to storage
    AfterCommit func(ctx context.Context, committed []synckit.EventWithVersion)
    
    // BeforePull is called before pulling events (for metrics, etc.)
    BeforePull func(ctx context.Context, since synckit.Version)
}

// NewSyncHandlerWithHooks creates a handler with hooks
func NewSyncHandlerWithHooks(store synckit.EventStore, logger *slog.Logger, parser VersionParser, options *ServerOptions, hooks *SyncHooks) *SyncHandler {
    h := NewSyncHandler(store, logger, parser, options)
    h.hooks = hooks
    return h
}

// In handlePush method, after successful storage (line ~177)
if h.hooks != nil && h.hooks.AfterCommit != nil && len(storedEvents) > 0 {
    // Run hook in goroutine to avoid blocking response
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        h.hooks.AfterCommit(ctx, storedEvents)
    }()
}
```

#### 5.2 Server Integration Example

```go
// Example server setup with projections
func setupServerWithProjections() {
    // Create store
    store, _ := sqlite.NewWithDataSource("server.db")
    
    // Create offset store (can share DB)
    db, _ := sql.Open("sqlite3", "server.db")
    offsetStore, _ := sqliteproj.NewOffsetStore(db, store.ParseVersion)
    
    // Create projector (user implements this)
    projector := &MyReadModelProjector{db: db}
    
    // Create runner
    runner := projection.NewRunner(store, offsetStore, projector,
        projection.WithBatchSize(100),
    )
    
    // Create hooks
    hooks := &httptransport.SyncHooks{
        AfterCommit: func(ctx context.Context, committed []synckit.EventWithVersion) {
            // Apply directly to projection
            if err := runner.ApplyBatch(ctx, committed); err != nil {
                log.Printf("Projection failed: %v", err)
            }
        },
    }
    
    // Create handler with hooks
    handler := httptransport.NewSyncHandlerWithHooks(store, logger, nil, nil, hooks)
    
    // Start server
    http.ListenAndServe(":8080", handler)
}
```

### Phase 6: Observability and Metrics

**Goal**: Production-grade metrics and health checks for projections.

#### 6.1 Projection Metrics (`projection/metrics.go`)

```go
package projection

import (
    "github.com/prometheus/client_golang/prometheus"
    "time"
)

var (
    projectionsApplied = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "synckit_projections_applied_total",
            Help: "Total number of events applied to projections",
        },
        []string{"projection"},
    )
    
    projectionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "synckit_projection_duration_seconds",
            Help: "Duration of projection operations",
        },
        []string{"projection"},
    )
    
    projectionErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "synckit_projection_errors_total",
            Help: "Total number of projection errors",
        },
        []string{"projection", "error_type"},
    )
    
    projectionLag = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "synckit_projection_lag_seconds",
            Help: "Lag between event creation and projection processing",
        },
        []string{"projection"},
    )
)

func init() {
    prometheus.MustRegister(
        projectionsApplied,
        projectionDuration,
        projectionErrors,
        projectionLag,
    )
}

// RecordProjectionApplied records successful projection application
func RecordProjectionApplied(name string, count int, duration time.Duration) {
    projectionsApplied.WithLabelValues(name).Add(float64(count))
    projectionDuration.WithLabelValues(name).Observe(duration.Seconds())
}

// RecordProjectionError records a projection error
func RecordProjectionError(name string, errorType string) {
    projectionErrors.WithLabelValues(name, errorType).Inc()
}

// UpdateProjectionLag updates the lag metric
func UpdateProjectionLag(name string, lag time.Duration) {
    projectionLag.WithLabelValues(name).Set(lag.Seconds())
}
```

### Phase 7: Testing Strategy

#### 7.1 Unit Tests

1. **Offset Store Tests** (`projection/sqlite/offsets_test.go`)
   - Test create table
   - Test get/set operations
   - Test concurrent access
   - Test version parsing

2. **Runner Tests** (`projection/runner_test.go`)
   - Test batch processing
   - Test context cancellation
   - Test error handling
   - Test idempotency

#### 7.2 Integration Tests

```go
// internal/integration-tests/projection_test.go
func TestProjectionIntegration(t *testing.T) {
    // Setup
    store, _ := sqlite.NewWithDataSource(":memory:")
    db, _ := sql.Open("sqlite3", ":memory:")
    offsetStore, _ := sqliteproj.NewOffsetStore(db, store.ParseVersion)
    
    // Create test projector
    projector := &testProjector{
        applied: make([]synckit.EventWithVersion, 0),
    }
    
    // Create runner
    runner := projection.NewRunner(store, offsetStore, projector,
        projection.WithBatchSize(10),
    )
    
    // Store test events
    for i := 0; i < 25; i++ {
        event := &TestEvent{id: fmt.Sprintf("test-%d", i)}
        store.Store(ctx, event, cursor.IntegerCursor{Seq: uint64(i + 1)})
    }
    
    // Run projection
    applied, last, err := runner.ApplySince(context.Background())
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, 25, applied)
    assert.Equal(t, uint64(25), last.(cursor.IntegerCursor).Seq)
    assert.Equal(t, 25, len(projector.applied))
}
```

#### 7.3 Race Condition Tests

```go
func TestProjectionConcurrency(t *testing.T) {
    // Run with: go test -race
    
    store, _ := sqlite.NewWithDataSource(":memory:?cache=shared")
    
    var wg sync.WaitGroup
    
    // Concurrent writers
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 100; j++ {
                event := &TestEvent{id: fmt.Sprintf("w%d-e%d", id, j)}
                store.Store(context.Background(), event, nil)
            }
        }(i)
    }
    
    // Concurrent projection runners
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            runner := createTestRunner(store, fmt.Sprintf("proj-%d", id))
            for j := 0; j < 5; j++ {
                runner.ApplySince(context.Background())
                time.Sleep(100 * time.Millisecond)
            }
        }(i)
    }
    
    wg.Wait()
}
```

## Implementation Timeline

### Week 1: Core Infrastructure
- Day 1-2: Implement Phase 1 (Projection API interfaces)
- Day 3-4: Implement Phase 2 (SQLite offset store)
- Day 5: Write unit tests for Phase 1-2

### Week 2: Runner and Integration
- Day 1-2: Implement Phase 3 (Projection runner)
- Day 3-4: Implement Phase 4 (SyncManager integration)
- Day 5: Integration testing

### Week 3: Server-Side and Production Features
- Day 1-2: Implement Phase 5 (Server hooks)
- Day 3: Implement Phase 6 (Observability)
- Day 4-5: Comprehensive testing and documentation

## Testing Checklist

- [ ] Unit tests for all new components
- [ ] Integration tests for client-server scenarios
- [ ] Race condition tests with `-race` flag
- [ ] Benchmark tests for performance validation
- [ ] Manual testing with example application
- [ ] Load testing with high event volumes
- [ ] Failure scenario testing (network, storage)
- [ ] Backward compatibility verification

## Key Design Decisions

1. **Non-Breaking Changes**: All additions are backward compatible
2. **Functional Options**: Consistent with existing patterns
3. **Idempotent Operations**: All projections are idempotent
4. **Server Authority**: Server commits are source of truth
5. **Offset Management**: Per-projection offset tracking
6. **Worker Pool**: Limited concurrent projections
7. **Timeout Protection**: All operations have timeouts
8. **Metrics First**: Built-in observability

## Migration Guide for Users

### Basic Usage

```go
// 1. Create projector (user implements)
type UserCountProjector struct {
    db *sql.DB
}

func (p *UserCountProjector) Name() string { return "user_count" }

func (p *UserCountProjector) Apply(ctx context.Context, events []synckit.EventWithVersion) error {
    for _, ev := range events {
        if ev.Event.Type() == "UserCreated" {
            // Update read model
            _, err := p.db.Exec("UPDATE stats SET user_count = user_count + 1")
            if err != nil {
                return err
            }
        }
    }
    return nil
}

// 2. Setup projection
offsetStore, _ := sqliteproj.NewOffsetStore(db, store.ParseVersion)
projector := &UserCountProjector{db: db}
runner := projection.NewRunner(store, offsetStore, projector)

// 3. Add to sync manager
syncManager, _ := synckit.NewManager(
    synckit.WithStore(store),
    synckit.WithTransport(transport),
    synckit.WithProjections(runner),
    synckit.WithProjectionsOnSync(true),
)
```

## Performance Considerations

1. **Batch Size**: Default 500 events, configurable
2. **Worker Pool**: 3 concurrent projections max
3. **Timeouts**: 30s per projection, 60s total
4. **Memory**: Batch processing limits memory usage
5. **SQLite**: WAL mode for concurrent access
6. **Metrics**: Lightweight Prometheus collectors

## Security Considerations

1. **SQL Injection**: Use parameterized queries
2. **Resource Limits**: Timeouts and batch sizes
3. **Error Handling**: No sensitive data in logs
4. **Concurrent Access**: Mutex protection
5. **Context Cancellation**: Graceful shutdown

## Future Enhancements

1. **Snapshot Support**: Periodic projection snapshots
2. **Parallel Projections**: Multiple projectors per runner
3. **Projection Registry**: Central registration system
4. **SSE Integration**: Real-time projection updates
5. **RabbitMQ Fan-out**: Distributed projections
6. **Projection Versioning**: Schema evolution support
7. **Optimistic Local Apply**: Immediate local projection

## Conclusion

This implementation provides a robust, production-ready projection system that integrates seamlessly with go-sync-kit's existing architecture. The design maintains backward compatibility while enabling powerful CQRS and event sourcing patterns for both client and server applications.
