package synckit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit-agent2/cursor"
	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
)

// syncManager implements the SyncManager interface
type syncManager struct {
	store     EventStore
	transport Transport
	options   SyncOptions

	// Internal state
	mu           sync.RWMutex
	autoSyncStop chan struct{}
	subscribers  []func(*SyncResult)
	closed       bool
}

// Sync performs a bidirectional sync operation
func (sm *syncManager) Sync(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		return nil, syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
	}
	sm.mu.RUnlock()

	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)
		sm.notifySubscribers(result)

		// Record metrics
		sm.options.MetricsCollector.RecordSyncDuration("full_sync", time.Since(start))
		if len(result.Errors) == 0 {
			sm.options.MetricsCollector.RecordSyncEvents(result.EventsPushed, result.EventsPulled)
			if result.ConflictsResolved > 0 {
				sm.options.MetricsCollector.RecordConflicts(result.ConflictsResolved)
			}
		} else {
			sm.options.MetricsCollector.RecordSyncErrors("full_sync", "sync_failure")
		}
	}()

	// Pull first to get latest remote changes
	if !sm.options.PushOnly {
		pullResult, err := sm.pull(ctx)
		if err != nil {
			result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", err))
		} else {
			result.EventsPulled = pullResult.EventsPulled
			result.ConflictsResolved = pullResult.ConflictsResolved
			result.RemoteVersion = pullResult.RemoteVersion
		}
	}

	// Then push local changes
	if !sm.options.PullOnly {
		pushResult, err := sm.push(ctx)
		if err != nil {
			result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpPush, "transport", err))
		} else {
			result.EventsPushed = pushResult.EventsPushed
		}
	}

	// Get final local version
	localVersion, err := sm.store.LatestVersion(ctx)
	if err != nil {
		result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err))
	} else {
		result.LocalVersion = localVersion
	}

	return result, nil
}

// Push sends local events to remote
func (sm *syncManager) Push(ctx context.Context) (*SyncResult, error) {
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		return nil, syncErrors.New(syncErrors.OpPush, fmt.Errorf("sync manager is closed"))
	}
	sm.mu.RUnlock()

	return sm.push(ctx)
}

// Pull retrieves remote events to local
func (sm *syncManager) Pull(ctx context.Context) (*SyncResult, error) {
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		return nil, syncErrors.New(syncErrors.OpPull, fmt.Errorf("sync manager is closed"))
	}
	sm.mu.RUnlock()

	return sm.pull(ctx)
}

func (sm *syncManager) push(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)

		// Record push metrics
		sm.options.MetricsCollector.RecordSyncDuration("push", time.Since(start))
		if result.EventsPushed > 0 {
			sm.options.MetricsCollector.RecordSyncEvents(result.EventsPushed, 0)
		}
	}()

	// Create a timeout context for database and transport operations
	opCtx, cancel := sm.withTimeout(ctx)
	defer cancel()

	// Get remote version efficiently
	remoteVersion, err := sm.transport.GetLatestVersion(opCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			sm.options.MetricsCollector.RecordSyncErrors("push", "context_canceled")
		} else if errors.Is(err, context.DeadlineExceeded) {
			sm.options.MetricsCollector.RecordSyncErrors("push", "timeout")
		} else {
			sm.options.MetricsCollector.RecordSyncErrors("push", "push_failure")
		}
		return result, syncErrors.NewWithComponent(syncErrors.OpPush, "transport", err)
	}

	// Load local events since remote version
	localEvents, err := sm.store.Load(opCtx, remoteVersion)
	if err != nil {
		return result, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err)
	}

	if len(localEvents) == 0 {
		return result, nil // Nothing to push
	}

	// Apply filter if configured
	if sm.options.Filter != nil {
		filtered := make([]EventWithVersion, 0, len(localEvents))
		for _, ev := range localEvents {
			if sm.options.Filter(ev.Event) {
				filtered = append(filtered, ev)
			}
		}
		localEvents = filtered
	}

	// Push in batches
	batchSize := sm.options.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(localEvents); i += batchSize {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(localEvents) {
			end = len(localEvents)
		}

		batch := localEvents[i:end]
		if err := sm.transport.Push(ctx, batch); err != nil {
			return result, syncErrors.NewWithComponent(syncErrors.OpPush, "transport", err)
		}

		result.EventsPushed += len(batch)
	}

	return result, nil
}

type exponentialBackoff struct {
	initialDelay time.Duration
	maxDelay     time.Duration
	multiplier   float64
}

func (eb *exponentialBackoff) nextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential delay: initialDelay * multiplier^attempt
	delay := float64(eb.initialDelay)
	if attempt > 0 {
		for i := 0; i < attempt; i++ {
			delay *= eb.multiplier
		}
	}

	// Convert back to time.Duration and cap at maxDelay
	result := time.Duration(delay)
	if result > eb.maxDelay {
		result = eb.maxDelay
	}

	return result
}

func (sm *syncManager) syncWithRetry(ctx context.Context, operation func() error) error {
	if sm.options.RetryConfig == nil {
		return operation()
	}

	config := sm.options.RetryConfig
	eb := &exponentialBackoff{
		initialDelay: config.InitialDelay,
		maxDelay:     config.MaxDelay,
		multiplier:   config.Multiplier,
	}

	// Initial attempt, no delay
	err := operation()
	if err == nil {
		return nil
	}

	if !syncErrors.IsRetryable(err) {
		return err
	}

	// Starting from 1 since we already did attempt 0
	for attempt := 1; attempt < config.MaxAttempts; attempt++ {
		// Calculate and apply delay before retry
		delay := eb.nextDelay(attempt - 1) // attempt-1 to start with initial delay

		// Use timer instead of time.After for more precise timing
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		// Try operation again
		err = operation()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !syncErrors.IsRetryable(err) {
			return err
		}
	}

	// All retries failed
	return err
}

func (sm *syncManager) pull(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)

		// Record pull metrics
		sm.options.MetricsCollector.RecordSyncDuration("pull", time.Since(start))
		if result.EventsPulled > 0 {
			sm.options.MetricsCollector.RecordSyncEvents(0, result.EventsPulled)
		}
		if result.ConflictsResolved > 0 {
			sm.options.MetricsCollector.RecordConflicts(result.ConflictsResolved)
		}
	}()

	// Create a timeout context for transport operations
	opCtx, cancel := sm.withTimeout(ctx)
	defer cancel()

	// Prefer cursor path if supported
	if ct, ok := sm.transport.(CursorTransport); ok {
		var since cursor.Cursor
		if sm.options.LastCursorLoader != nil {
			since = sm.options.LastCursorLoader()
		}
		limit := sm.options.BatchSize
		if limit <= 0 {
			limit = 200
		}

		events, next, err := ct.PullWithCursor(opCtx, since, limit)
		if err != nil {
			// Record pull metrics
			if errors.Is(err, context.Canceled) {
				sm.options.MetricsCollector.RecordSyncErrors("pull", "context_canceled")
			} else if errors.Is(err, context.DeadlineExceeded) {
				sm.options.MetricsCollector.RecordSyncErrors("pull", "timeout")
			} else {
				sm.options.MetricsCollector.RecordSyncErrors("pull", "pull_failure")
			}
			return result, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", err)
		}

		// Apply filter
		if sm.options.Filter != nil {
			filtered := make([]EventWithVersion, 0, len(events))
			for _, ev := range events {
				if sm.options.Filter(ev.Event) {
					filtered = append(filtered, ev)
				}
			}
			events = filtered
		}

		// Store remote events locally
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}
			if err := sm.store.Store(ctx, ev.Event, ev.Version); err != nil {
				return result, syncErrors.NewWithComponent(syncErrors.OpStore, "store", err)
			}
			result.EventsPulled++
		}

		// Save cursor
		if sm.options.CursorSaver != nil && next != nil {
			_ = sm.options.CursorSaver(next)
		}

		if len(events) > 0 {
			result.RemoteVersion = findLatestVersion(events)
		}
		return result, nil
	}

	// Fallback: legacy version-based path
	localVersion, err := sm.store.LatestVersion(ctx)
	if err != nil {
		return result, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err)
	}

	// Pull remote events since our local version
	remoteEvents, err := sm.transport.Pull(opCtx, localVersion)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			sm.options.MetricsCollector.RecordSyncErrors("pull", "context_canceled")
		} else if errors.Is(err, context.DeadlineExceeded) {
			sm.options.MetricsCollector.RecordSyncErrors("pull", "timeout")
		} else {
			sm.options.MetricsCollector.RecordSyncErrors("pull", "pull_failure")
		}
		return result, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", err)
	}

	if len(remoteEvents) == 0 {
		return result, nil // Nothing to pull
	}

	// Apply filter if configured
	if sm.options.Filter != nil {
		filtered := make([]EventWithVersion, 0, len(remoteEvents))
		for _, ev := range remoteEvents {
			if sm.options.Filter(ev.Event) {
				filtered = append(filtered, ev)
			}
		}
		remoteEvents = filtered
	}

	// Check for conflicts if we have a conflict resolver
	if sm.options.ConflictResolver != nil {
	// Create a timeout context for database operations
	dbCtx, cancel := sm.withTimeout(ctx)
	defer cancel()

		// Load local events that might conflict
		localEvents, err := sm.store.Load(dbCtx, localVersion)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "db_timeout")
			}
			return result, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err)
		}

		if len(localEvents) > 0 {
			// Check context before starting conflict resolution
			select {
			case <-ctx.Done():
				sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "context_canceled")
				return result, syncErrors.NewWithComponent(syncErrors.OpConflictResolve, "resolver", ctx.Err())
			default:
			}

			resolvedEvents, err := sm.options.ConflictResolver.Resolve(ctx, localEvents, remoteEvents)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "context_canceled")
				}
				return result, syncErrors.NewWithComponent(syncErrors.OpConflictResolve, "resolver", err)
			}
			remoteEvents = resolvedEvents
			result.ConflictsResolved = len(localEvents) + len(remoteEvents) - len(resolvedEvents)
		}
	}

	// Store remote events locally
	for _, ev := range remoteEvents {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		if err := sm.store.Store(ctx, ev.Event, ev.Version); err != nil {
			return result, syncErrors.NewWithComponent(syncErrors.OpStore, "store", err)
		}
		result.EventsPulled++
	}

	// Get final remote version
	if len(remoteEvents) > 0 {
		result.RemoteVersion = findLatestVersion(remoteEvents)
	}

	return result, nil
}

func (sm *syncManager) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if sm.options.Timeout > 0 {
		return context.WithTimeout(ctx, sm.options.Timeout)
	}
	// If no timeout is specified, use 30 seconds as a reasonable default
	return context.WithTimeout(ctx, 30*time.Second)
}

// StartAutoSync begins automatic synchronization at the configured interval
func (sm *syncManager) StartAutoSync(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check context cancellation first
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if sm.closed {
		return syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
	}

	if sm.options.SyncInterval <= 0 {
		return syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync interval must be positive"))
	}

	if sm.autoSyncStop != nil {
		return syncErrors.New(syncErrors.OpSync, fmt.Errorf("auto sync is already running"))
	}

	// FIXED: Create channel while holding lock
	stopChan := make(chan struct{})
	sm.autoSyncStop = stopChan

	go func() {
		ticker := time.NewTicker(sm.options.SyncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopChan: // Use local copy to avoid race
				return
			case <-ticker.C:
				// Create timeout context derived from parent context
				syncCtx, cancel := sm.withTimeout(ctx)
				_, err := sm.Sync(syncCtx)
				cancel() // Always cancel to free resources

				if err != nil {
					result := &SyncResult{
						StartTime: time.Now(),
						Duration:  0,
						Errors:    []error{err},
					}
					sm.notifySubscribers(result)
				}
			}
		}
	}()

	return nil
}

// StopAutoSync stops automatic synchronization
func (sm *syncManager) StopAutoSync() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.autoSyncStop == nil {
		return syncErrors.New(syncErrors.OpSync, fmt.Errorf("auto sync is not running"))
	}

	// FIXED: Close channel safely
	close(sm.autoSyncStop)
	sm.autoSyncStop = nil
	return nil
}

// Subscribe to sync events
func (sm *syncManager) Subscribe(handler func(*SyncResult)) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
	}

	sm.subscribers = append(sm.subscribers, handler)
	return nil
}

// Close shuts down the sync manager
func (sm *syncManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return nil
	}

	sm.closed = true

	// Stop auto sync if running
	if sm.autoSyncStop != nil {
		close(sm.autoSyncStop)
		sm.autoSyncStop = nil
	}

	// Close transport and store
	var errs []error
	if err := sm.transport.Close(); err != nil {
		errs = append(errs, syncErrors.NewWithComponent(syncErrors.OpClose, "transport", err))
	}
	if err := sm.store.Close(); err != nil {
		errs = append(errs, syncErrors.NewWithComponent(syncErrors.OpClose, "store", err))
	}

	if len(errs) > 0 {
		return syncErrors.New(syncErrors.OpClose, fmt.Errorf("multiple close errors: %v", errs))
	}

	return nil
}

func (sm *syncManager) notifySubscribers(result *SyncResult) {
	sm.mu.RLock()
	subscribers := make([]func(*SyncResult), len(sm.subscribers))
	copy(subscribers, sm.subscribers)
	sm.mu.RUnlock()

	for _, handler := range subscribers {
		go func(h func(*SyncResult)) {
			defer func() {
				if r := recover(); r != nil {
					//TODO: Log panic from subscriber, but don't crash
				}
			}()
			h(result)
		}(handler)
	}
}

// findLatestVersion returns the latest version from a slice of events.
// If events is empty, returns nil.
func findLatestVersion(events []EventWithVersion) Version {
	if len(events) == 0 {
		return nil
	}
	latest := events[0].Version
	for i := 1; i < len(events); i++ {
		if events[i].Version.Compare(latest) > 0 {
			latest = events[i].Version
		}
	}
	return latest
}
