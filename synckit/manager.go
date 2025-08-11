package synckit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
)

// syncManager implements the SyncManager interface
type syncManager struct {
	store     EventStore
	transport Transport
	options   SyncOptions
	logger    *slog.Logger

	// Internal state
	mu           sync.RWMutex
	autoSyncStop chan struct{}
	subscribers  []func(*SyncResult)
	closed       bool
}

// Sync performs a bidirectional sync operation
func (sm *syncManager) Sync(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	sm.logger.Info("Starting bidirectional sync operation")
	
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
		sm.logger.Error("Sync operation failed: manager is closed", "error", err)
		return nil, err
	}
	sm.mu.RUnlock()

	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)
		sm.notifySubscribers(result)

		// Record metrics and log completion
		sm.options.MetricsCollector.RecordSyncDuration("full_sync", time.Since(start))
		if len(result.Errors) == 0 {
			sm.options.MetricsCollector.RecordSyncEvents(result.EventsPushed, result.EventsPulled)
			if result.ConflictsResolved > 0 {
				sm.options.MetricsCollector.RecordConflicts(result.ConflictsResolved)
			}
			sm.logger.Info("Sync operation completed successfully",
				"duration", result.Duration,
				"events_pushed", result.EventsPushed,
				"events_pulled", result.EventsPulled,
				"conflicts_resolved", result.ConflictsResolved)
		} else {
			sm.options.MetricsCollector.RecordSyncErrors("full_sync", "sync_failure")
			sm.logger.Error("Sync operation completed with errors",
				"duration", result.Duration,
				"error_count", len(result.Errors),
				"errors", result.Errors)
		}
	}()

	// Pull first to get latest remote changes
	if !sm.options.PushOnly {
		sm.logger.Debug("Starting pull phase of sync operation")
		pullResult, err := sm.pull(ctx)
		if err != nil {
			sm.logger.Error("Pull phase failed", "error", err)
			result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", err))
		} else {
			sm.logger.Debug("Pull phase completed successfully",
				"events_pulled", pullResult.EventsPulled,
				"conflicts_resolved", pullResult.ConflictsResolved)
			result.EventsPulled = pullResult.EventsPulled
			result.ConflictsResolved = pullResult.ConflictsResolved
			result.RemoteVersion = pullResult.RemoteVersion
		}
	}

	// Then push local changes
	if !sm.options.PullOnly {
		sm.logger.Debug("Starting push phase of sync operation")
		pushResult, err := sm.push(ctx)
		if err != nil {
			sm.logger.Error("Push phase failed", "error", err)
			result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpPush, "transport", err))
		} else {
			sm.logger.Debug("Push phase completed successfully",
				"events_pushed", pushResult.EventsPushed)
			result.EventsPushed = pushResult.EventsPushed
		}
	}

	// Get final local version
	localVersion, err := sm.store.LatestVersion(ctx)
	if err != nil {
		sm.logger.Error("Failed to get final local version", "error", err)
		result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err))
	} else {
		result.LocalVersion = localVersion
	}

	return result, nil
}

// Push sends local events to remote
func (sm *syncManager) Push(ctx context.Context) (*SyncResult, error) {
	sm.logger.Info("Starting push operation")
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		err := syncErrors.New(syncErrors.OpPush, fmt.Errorf("sync manager is closed"))
		sm.logger.Error("Push operation failed: manager is closed", "error", err)
		return nil, err
	}
	sm.mu.RUnlock()

	return sm.push(ctx)
}

// Pull retrieves remote events to local
func (sm *syncManager) Pull(ctx context.Context) (*SyncResult, error) {
	sm.logger.Info("Starting pull operation")
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		err := syncErrors.New(syncErrors.OpPull, fmt.Errorf("sync manager is closed"))
		sm.logger.Error("Pull operation failed: manager is closed", "error", err)
		return nil, err
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
	sm.logger.Debug("Getting remote version for push operation")
	remoteVersion, err := sm.transport.GetLatestVersion(opCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			sm.logger.Warn("Push operation canceled by context", "error", err)
			sm.options.MetricsCollector.RecordSyncErrors("push", "context_canceled")
		} else if errors.Is(err, context.DeadlineExceeded) {
			sm.logger.Warn("Push operation timed out getting remote version", "error", err)
			sm.options.MetricsCollector.RecordSyncErrors("push", "timeout")
		} else {
			sm.logger.Error("Push operation failed to get remote version", "error", err)
			sm.options.MetricsCollector.RecordSyncErrors("push", "push_failure")
		}
		return result, syncErrors.NewWithComponent(syncErrors.OpPush, "transport", err)
	}

	// Load local events since remote version
	sm.logger.Debug("Loading local events for push operation", "since_version", remoteVersion)
	localEvents, err := sm.store.Load(opCtx, remoteVersion)
	if err != nil {
		sm.logger.Error("Failed to load local events for push", "error", err)
		return result, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err)
	}

	if len(localEvents) == 0 {
		sm.logger.Debug("No local events to push")
		return result, nil // Nothing to push
	}

	// Apply filter if configured
	if sm.options.Filter != nil {
		originalCount := len(localEvents)
		filtered := make([]EventWithVersion, 0, len(localEvents))
		for _, ev := range localEvents {
			if sm.options.Filter(ev.Event) {
				filtered = append(filtered, ev)
			}
		}
		localEvents = filtered
		sm.logger.Debug("Applied event filter for push",
			"original_count", originalCount,
			"filtered_count", len(localEvents))
	}

	// Push in batches
	batchSize := sm.options.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	sm.logger.Debug("Starting batch push operation",
		"total_events", len(localEvents),
		"batch_size", batchSize)

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
		sm.logger.Debug("Pushing event batch",
			"batch_number", (i/batchSize)+1,
			"batch_size", len(batch),
			"total_batches", (len(localEvents)+batchSize-1)/batchSize)
		
		if err := sm.transport.Push(ctx, batch); err != nil {
			sm.logger.Error("Failed to push event batch",
				"batch_number", (i/batchSize)+1,
				"batch_size", len(batch),
				"error", err)
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
		sm.logger.Debug("No retry configuration, executing operation once")
		return operation()
	}

	config := sm.options.RetryConfig
	sm.logger.Debug("Starting operation with retry",
		"max_attempts", config.MaxAttempts,
		"initial_delay", config.InitialDelay,
		"max_delay", config.MaxDelay,
		"multiplier", config.Multiplier)
		
	eb := &exponentialBackoff{
		initialDelay: config.InitialDelay,
		maxDelay:     config.MaxDelay,
		multiplier:   config.Multiplier,
	}

	// Initial attempt, no delay
	sm.logger.Debug("Executing operation (attempt 1)")
	err := operation()
	if err == nil {
		sm.logger.Debug("Operation succeeded on first attempt")
		return nil
	}

	if !syncErrors.IsRetryable(err) {
		sm.logger.Debug("Operation failed with non-retryable error", "error", err)
		return err
	}

	// Starting from 1 since we already did attempt 0
	sm.logger.Warn("Operation failed with retryable error, starting retry sequence",
		"error", err,
		"max_attempts", config.MaxAttempts)
		
	for attempt := 1; attempt < config.MaxAttempts; attempt++ {
		// Calculate and apply delay before retry
		delay := eb.nextDelay(attempt - 1) // attempt-1 to start with initial delay
		sm.logger.Debug("Waiting before retry",
			"attempt", attempt+1,
			"delay", delay)

		// Use timer instead of time.After for more precise timing
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			sm.logger.Warn("Retry sequence canceled by context", "error", ctx.Err())
			return ctx.Err()
		case <-timer.C:
		}

		// Try operation again
		sm.logger.Debug("Retrying operation", "attempt", attempt+1)
		err = operation()
		if err == nil {
			sm.logger.Info("Operation succeeded after retry", "attempt", attempt+1)
			return nil
		}

		// Check if error is retryable
		if !syncErrors.IsRetryable(err) {
			sm.logger.Warn("Retry failed with non-retryable error",
				"attempt", attempt+1,
				"error", err)
			return err
		}
		
		sm.logger.Debug("Retry attempt failed, will continue retrying",
			"attempt", attempt+1,
			"error", err)
	}

	// All retries failed
	sm.logger.Error("All retry attempts exhausted",
		"total_attempts", config.MaxAttempts,
		"final_error", err)
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

	// Get local version
	sm.logger.Debug("Getting local version for pull operation")
	localVersion, err := sm.store.LatestVersion(ctx)
	if err != nil {
		sm.logger.Error("Failed to get local version for pull", "error", err)
		return result, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err)
	}

	// Create a timeout context for transport operations
	opCtx, cancel := sm.withTimeout(ctx)
	defer cancel()

	// Pull remote events since our local version
	sm.logger.Debug("Pulling remote events", "since_version", localVersion)
	remoteEvents, err := sm.transport.Pull(opCtx, localVersion)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			sm.logger.Warn("Pull operation canceled by context", "error", err)
			sm.options.MetricsCollector.RecordSyncErrors("pull", "context_canceled")
		} else if errors.Is(err, context.DeadlineExceeded) {
			sm.logger.Warn("Pull operation timed out", "error", err)
			sm.options.MetricsCollector.RecordSyncErrors("pull", "timeout")
		} else {
			sm.logger.Error("Pull operation failed", "error", err)
			sm.options.MetricsCollector.RecordSyncErrors("pull", "pull_failure")
		}
		return result, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", err)
	}

	if len(remoteEvents) == 0 {
		sm.logger.Debug("No remote events to pull")
		return result, nil // Nothing to pull
	}

	// Apply filter if configured
	if sm.options.Filter != nil {
		originalCount := len(remoteEvents)
		filtered := make([]EventWithVersion, 0, len(remoteEvents))
		for _, ev := range remoteEvents {
			if sm.options.Filter(ev.Event) {
				filtered = append(filtered, ev)
			}
		}
		remoteEvents = filtered
		sm.logger.Debug("Applied event filter for pull",
			"original_count", originalCount,
			"filtered_count", len(remoteEvents))
	}

	// Check for conflicts if we have a conflict resolver
	if sm.options.ConflictResolver != nil {
		sm.logger.Debug("Starting conflict resolution", "remote_events", len(remoteEvents))
		// Create a timeout context for database operations
		dbCtx, cancel := sm.withTimeout(ctx)
		defer cancel()

		// Load local events that might conflict
		localEvents, err := sm.store.Load(dbCtx, localVersion)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				sm.logger.Error("Conflict resolution timed out loading local events", "error", err)
				sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "db_timeout")
			} else {
				sm.logger.Error("Failed to load local events for conflict resolution", "error", err)
			}
			return result, syncErrors.NewWithComponent(syncErrors.OpLoad, "store", err)
		}

		if len(localEvents) > 0 {
			sm.logger.Debug("Found potential conflicts",
				"local_events", len(localEvents),
				"remote_events", len(remoteEvents))
			// Check context before starting conflict resolution
			select {
			case <-ctx.Done():
				sm.logger.Warn("Conflict resolution canceled by context", "error", ctx.Err())
				sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "context_canceled")
				return result, syncErrors.NewWithComponent(syncErrors.OpConflictResolve, "resolver", ctx.Err())
			default:
			}

			resolvedEvents, err := sm.options.ConflictResolver.Resolve(ctx, localEvents, remoteEvents)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					sm.logger.Warn("Conflict resolution canceled", "error", err)
					sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "context_canceled")
				} else {
					sm.logger.Error("Conflict resolution failed", "error", err)
				}
				return result, syncErrors.NewWithComponent(syncErrors.OpConflictResolve, "resolver", err)
			}
			remoteEvents = resolvedEvents
			result.ConflictsResolved = len(localEvents) + len(remoteEvents) - len(resolvedEvents)
			sm.logger.Info("Conflict resolution completed",
				"conflicts_resolved", result.ConflictsResolved,
				"final_events", len(resolvedEvents))
		} else {
			sm.logger.Debug("No local events found, no conflicts to resolve")
		}
	}

	// Store remote events locally
	sm.logger.Debug("Storing remote events locally", "event_count", len(remoteEvents))
	for i, ev := range remoteEvents {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			sm.logger.Warn("Pull operation canceled while storing events",
				"events_stored", i,
				"total_events", len(remoteEvents))
			return result, ctx.Err()
		default:
		}

		if err := sm.store.Store(ctx, ev.Event, ev.Version); err != nil {
			sm.logger.Error("Failed to store remote event",
				"event_index", i,
				"version", ev.Version,
				"error", err)
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
	sm.logger.Info("Starting automatic sync", "interval", sm.options.SyncInterval)
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check context cancellation first
	select {
	case <-ctx.Done():
		sm.logger.Warn("Auto sync start canceled by context", "error", ctx.Err())
		return ctx.Err()
	default:
	}

	if sm.closed {
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
		sm.logger.Error("Cannot start auto sync: manager is closed", "error", err)
		return err
	}

	if sm.options.SyncInterval <= 0 {
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync interval must be positive"))
		sm.logger.Error("Cannot start auto sync: invalid interval",
			"interval", sm.options.SyncInterval,
			"error", err)
		return err
	}

	if sm.autoSyncStop != nil {
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("auto sync is already running"))
		sm.logger.Warn("Auto sync is already running", "error", err)
		return err
	}

	// FIXED: Create channel while holding lock
	stopChan := make(chan struct{})
	sm.autoSyncStop = stopChan

	go func() {
		sm.logger.Info("Auto sync goroutine started", "interval", sm.options.SyncInterval)
		ticker := time.NewTicker(sm.options.SyncInterval)
		defer func() {
			ticker.Stop()
			sm.logger.Info("Auto sync goroutine stopped")
		}()

		for {
			select {
			case <-ctx.Done():
				sm.logger.Info("Auto sync stopping due to context cancellation")
				return
			case <-stopChan: // Use local copy to avoid race
				sm.logger.Info("Auto sync stopping due to explicit stop")
				return
			case <-ticker.C:
				sm.logger.Debug("Auto sync tick - starting sync operation")
				// Create timeout context derived from parent context
				syncCtx, cancel := sm.withTimeout(ctx)
				_, err := sm.Sync(syncCtx)
				cancel() // Always cancel to free resources

				if err != nil {
					sm.logger.Error("Auto sync operation failed", "error", err)
					result := &SyncResult{
						StartTime: time.Now(),
						Duration:  0,
						Errors:    []error{err},
					}
					sm.notifySubscribers(result)
				} else {
					sm.logger.Debug("Auto sync operation completed successfully")
				}
			}
		}
	}()

	return nil
}

// StopAutoSync stops automatic synchronization
func (sm *syncManager) StopAutoSync() error {
	sm.logger.Info("Stopping automatic sync")
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.autoSyncStop == nil {
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("auto sync is not running"))
		sm.logger.Warn("Cannot stop auto sync: not running", "error", err)
		return err
	}

	// FIXED: Close channel safely
	close(sm.autoSyncStop)
	sm.autoSyncStop = nil
	sm.logger.Info("Auto sync stopped successfully")
	return nil
}

// Subscribe to sync events
func (sm *syncManager) Subscribe(handler func(*SyncResult)) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
		sm.logger.Error("Cannot subscribe: manager is closed", "error", err)
		return err
	}

	sm.subscribers = append(sm.subscribers, handler)
	sm.logger.Debug("New subscriber added", "total_subscribers", len(sm.subscribers))
	return nil
}

// Close shuts down the sync manager
func (sm *syncManager) Close() error {
	sm.logger.Info("Closing sync manager")
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		sm.logger.Debug("Sync manager already closed")
		return nil
	}

	sm.closed = true

	// Stop auto sync if running
	if sm.autoSyncStop != nil {
		sm.logger.Debug("Stopping auto sync as part of close")
		close(sm.autoSyncStop)
		sm.autoSyncStop = nil
	}

	// Close transport and store
	var errs []error
	if err := sm.transport.Close(); err != nil {
		sm.logger.Error("Error closing transport", "error", err)
		errs = append(errs, syncErrors.NewWithComponent(syncErrors.OpClose, "transport", err))
	}
	if err := sm.store.Close(); err != nil {
		sm.logger.Error("Error closing store", "error", err)
		errs = append(errs, syncErrors.NewWithComponent(syncErrors.OpClose, "store", err))
	}

	if len(errs) > 0 {
		sm.logger.Error("Sync manager closed with errors", "error_count", len(errs))
		return syncErrors.New(syncErrors.OpClose, fmt.Errorf("multiple close errors: %v", errs))
	}

	sm.logger.Info("Sync manager closed successfully")
	return nil
}

func (sm *syncManager) notifySubscribers(result *SyncResult) {
	sm.mu.RLock()
	subscribers := make([]func(*SyncResult), len(sm.subscribers))
	copy(subscribers, sm.subscribers)
	sm.mu.RUnlock()

	if len(subscribers) > 0 {
		sm.logger.Debug("Notifying sync result subscribers", "subscriber_count", len(subscribers))
	}

	for _, handler := range subscribers {
		go func(h func(*SyncResult)) {
			defer func() {
				if r := recover(); r != nil {
					sm.logger.Error("Subscriber panic recovered",
						"panic", r,
						"sync_errors", len(result.Errors),
						"events_pushed", result.EventsPushed,
						"events_pulled", result.EventsPulled)
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
