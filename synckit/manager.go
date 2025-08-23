package synckit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// syncManager implements the SyncManager interface
type syncManager struct {
	store     EventStore
	transport Transport
	options   SyncOptions
	logger    *slog.Logger

	// State machine for operation tracking
	stateMachine statemachine.StateMachine[SyncState]

	// Internal state
	mu               sync.RWMutex
	autoSyncCancel   context.CancelFunc // Cancel function for auto-sync context
	autoSyncInterval time.Duration      // Current auto-sync interval
	subscribers      []func(*SyncResult)
	closed           bool

	// Projection support
	projectionConfig *ProjectionConfig
	projectionPool   chan struct{} // Worker pool for concurrent projections
}

// Sync performs a bidirectional sync operation with state machine tracking
func (sm *syncManager) Sync(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	sm.logger.Info("Starting bidirectional sync operation")
	
	// Check if manager is closed
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync manager is closed"))
		sm.logger.Error("Sync operation failed: manager is closed", "error", err)
		return nil, err
	}
	sm.mu.RUnlock()

	// Check if sync is already in progress (state machine validation)
	if sm.stateMachine != nil {
		currentState := sm.stateMachine.Current()
		if !currentState.CanAutoSync() {
			err := syncErrors.New(syncErrors.OpSync, fmt.Errorf("sync operation already in progress: %s", currentState))
			sm.logger.Warn("Sync operation rejected: already in progress", 
				"current_state", currentState.String(),
				"error", err)
			return nil, err
		}
		
		// Transition to initializing state
		if err := sm.stateMachine.TransitionWithContext(SyncInitializing, map[string]interface{}{
			"operation": "full_sync",
			"started_at": start.Format(time.RFC3339),
		});
		 err != nil {
			sm.logger.Error("Failed to transition to initializing state", "error", err)
			return nil, syncErrors.NewWithComponent(syncErrors.OpSync, "state_machine", err)
		}
	}

	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)
		
		// Transition to final state based on result
		if sm.stateMachine != nil {
			finalState := SyncCompleted
			finalMetadata := map[string]interface{}{
				"operation": "full_sync",
				"duration_ms": result.Duration.Milliseconds(),
				"events_pushed": result.EventsPushed,
				"events_pulled": result.EventsPulled,
				"conflicts_resolved": result.ConflictsResolved,
			}
			
			if len(result.Errors) > 0 {
				finalState = SyncFailed
				finalMetadata["error_count"] = len(result.Errors)
				finalMetadata["errors"] = result.Errors
			}
			
			if err := sm.stateMachine.TransitionWithContext(finalState, finalMetadata); err != nil {
				sm.logger.Error("Failed to transition to final state", 
					"target_state", finalState.String(),
					"error", err)
			}
			
			// Always transition back to idle after completion or failure
			if err := sm.stateMachine.TransitionWithContext(SyncIdle, map[string]interface{}{
				"operation_completed": true,
				"final_state": finalState.String(),
			}); err != nil {
				sm.logger.Error("Failed to transition back to idle state", "error", err)
			}
		}
		
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
		// Transition to pulling state
		if sm.stateMachine != nil {
			if err := sm.stateMachine.TransitionWithContext(SyncPulling, map[string]interface{}{
				"operation": "pull",
				"push_only": false,
			}); err != nil {
				sm.logger.Error("Failed to transition to pulling state", "error", err)
				result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpSync, "state_machine", err))
				return result, nil // Defer will handle final state transition
			}
		}
		
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
		// Transition to pushing state
		if sm.stateMachine != nil {
			if err := sm.stateMachine.TransitionWithContext(SyncPushing, map[string]interface{}{
				"operation": "push",
				"pull_only": false,
			}); err != nil {
				sm.logger.Error("Failed to transition to pushing state", "error", err)
				result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpSync, "state_machine", err))
				return result, nil // Defer will handle final state transition
			}
		}
		
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

	// Run projections if configured and sync was successful
	if len(result.Errors) == 0 {
		if err := sm.runProjectionsAfterSync(ctx, result); err != nil {
			sm.logger.Error("Failed to run projections after sync", "error", err)
			result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpSync, "projections", err))
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

	// Process conflicts with the new dynamic resolver
	if sm.options.ConflictResolver != nil {
		sm.logger.Debug("Starting conflict detection and resolution", "remote_events", len(remoteEvents))
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
			sm.logger.Debug("Detecting conflicts between local and remote events",
				"local_events", len(localEvents),
				"remote_events", len(remoteEvents))
			
			// Detect actual conflicts by comparing events
			conflicts := sm.detectConflicts(localEvents, remoteEvents)
			if len(conflicts) > 0 {
				sm.logger.Info("Found conflicts, starting resolution", "conflict_count", len(conflicts))
				
				// Transition to conflict resolution state
				if sm.stateMachine != nil {
					if err := sm.stateMachine.TransitionWithContext(SyncResolvingConflicts, map[string]interface{}{
						"operation": "conflict_resolution",
						"conflict_count": len(conflicts),
					}); err != nil {
						sm.logger.Error("Failed to transition to conflict resolution state", "error", err)
						result.Errors = append(result.Errors, syncErrors.NewWithComponent(syncErrors.OpSync, "state_machine", err))
						return result, nil // Defer will handle final state transition
					}
				}
				
				// Check context before starting conflict resolution
				select {
				case <-ctx.Done():
					sm.logger.Warn("Conflict resolution canceled by context", "error", ctx.Err())
					sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "context_canceled")
					return result, syncErrors.NewWithComponent(syncErrors.OpConflictResolve, "resolver", ctx.Err())
				default:
				}
				
				// Resolve each conflict using the new interface
				var resolvedEvents []EventWithVersion
				for _, conflict := range conflicts {
					resolvedConflict, err := sm.options.ConflictResolver.Resolve(ctx, conflict)
					if err != nil {
						if errors.Is(err, context.Canceled) {
							sm.logger.Warn("Conflict resolution canceled", "error", err)
							sm.options.MetricsCollector.RecordSyncErrors("conflict_resolution", "context_canceled")
						} else {
							sm.logger.Error("Conflict resolution failed", "conflict", conflict, "error", err)
						}
						return result, syncErrors.NewWithComponent(syncErrors.OpConflictResolve, "resolver", err)
					}
					
					// Apply resolved events
					resolvedEvents = append(resolvedEvents, resolvedConflict.ResolvedEvents...)
					
					// Log resolution details for audit
					if len(resolvedConflict.Reasons) > 0 {
						sm.logger.Info("Conflict resolved with reasons", 
							"aggregate_id", conflict.AggregateID,
							"event_type", conflict.EventType,
							"decision", resolvedConflict.Decision,
							"reasons", resolvedConflict.Reasons)
					}
				}
				
				// Replace remote events with resolved events
				remoteEvents = resolvedEvents
				result.ConflictsResolved = len(conflicts)
				sm.logger.Info("Conflict resolution completed",
					"conflicts_resolved", result.ConflictsResolved,
					"final_events", len(resolvedEvents))
			} else {
				sm.logger.Debug("No conflicts detected between local and remote events")
			}
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

// StartAutoSync begins automatic synchronization at the configured interval.
// This method is idempotent - calling it multiple times will not create duplicate goroutines.
func (sm *syncManager) StartAutoSync(ctx context.Context) error {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		sm.logger.Warn("Auto sync start canceled by context", "error", ctx.Err())
		return ctx.Err()
	default:
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Idempotent check: if auto sync is already running, return nil
	if sm.autoSyncCancel != nil {
		sm.logger.Debug("Auto sync is already running, ignoring duplicate start request")
		return nil
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

	// Create cancellable context for auto-sync goroutine
	autoSyncCtx, cancel := context.WithCancel(ctx)
	sm.autoSyncCancel = cancel
	sm.autoSyncInterval = sm.options.SyncInterval

	sm.logger.Info("Starting automatic sync", "interval", sm.autoSyncInterval)

	// Start auto-sync goroutine
	go func() {
		sm.logger.Info("Auto sync goroutine started", "interval", sm.autoSyncInterval)
		// Use the configured interval from options, not the stored interval
		ticker := time.NewTicker(sm.options.SyncInterval)
		defer func() {
			ticker.Stop()
			sm.logger.Info("Auto sync goroutine stopped")
		}()

		for {
			select {
			case <-autoSyncCtx.Done():
				sm.logger.Info("Auto sync stopping due to context cancellation")
				return
			case <-ticker.C:
				sm.logger.Debug("Auto sync tick - checking if sync is possible")
				
				// Check state machine to prevent overlapping sync operations
				if sm.stateMachine != nil {
					currentState := sm.stateMachine.Current()
					if !currentState.CanAutoSync() {
						sm.logger.Debug("Auto sync skipped - sync operation already in progress",
							"current_state", currentState.String())
						continue // Skip this tick and wait for next one
					}
				}
				
				// Check transport health if transport supports state awareness
				if !sm.isTransportHealthyForSync() {
					sm.logger.Debug("Auto sync skipped - transport not healthy for sync operations")
					continue // Skip this tick and wait for next one
				}
				
				sm.logger.Debug("Auto sync tick - starting sync operation")
				// Create timeout context derived from auto-sync context
				syncCtx, cancel := sm.withTimeout(autoSyncCtx)
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

// StopAutoSync stops automatic synchronization.
// This method is idempotent - calling it multiple times will not cause errors.
func (sm *syncManager) StopAutoSync() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Idempotent check: if auto sync is not running, return nil (no-op)
	if sm.autoSyncCancel == nil {
		sm.logger.Debug("Auto sync is not running, ignoring stop request")
		return nil
	}

	sm.logger.Info("Stopping automatic sync")
	
	// Cancel the auto-sync context to stop the goroutine
	sm.autoSyncCancel()
	sm.autoSyncCancel = nil
	sm.autoSyncInterval = 0
	
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
	if sm.autoSyncCancel != nil {
		sm.logger.Debug("Stopping auto sync as part of close")
		sm.autoSyncCancel()
		sm.autoSyncCancel = nil
		sm.autoSyncInterval = 0
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

// detectConflicts analyzes local and remote events to identify conflicts.
// It builds rich Conflict instances with metadata as required by Phase 3.
// This method implements simple conflict detection based on:
// 1. Same AggregateID and EventType
// 2. Different versions/timestamps indicating concurrent modifications
func (sm *syncManager) detectConflicts(localEvents, remoteEvents []EventWithVersion) []Conflict {
	var conflicts []Conflict
	
	// Create a map of local events by aggregate+type for quick lookup
	localByKey := make(map[string][]EventWithVersion)
	for _, local := range localEvents {
		key := local.Event.AggregateID() + ":" + local.Event.Type()
		localByKey[key] = append(localByKey[key], local)
	}
	
	// Check each remote event for conflicts with local events
	for _, remote := range remoteEvents {
		key := remote.Event.AggregateID() + ":" + remote.Event.Type()
		localMatches, exists := localByKey[key]
		
		if !exists {
			continue // No local event with same aggregate+type, no conflict
		}
		
		// For each matching local event, check if there's a version conflict
		for _, local := range localMatches {
			// Simple conflict detection: if we have both local and remote events
			// for the same aggregate+type, and they have different versions,
			// consider it a conflict
			if local.Version.Compare(remote.Version) != 0 {
				conflict := Conflict{
					EventType:   remote.Event.Type(),
					AggregateID: remote.Event.AggregateID(),
					Local:       local,
					Remote:      remote,
					Metadata:    make(map[string]any),
				}
				
				// Add metadata for enhanced conflict resolution
				conflict.Metadata["local_version"] = local.Version.String()
				conflict.Metadata["remote_version"] = remote.Version.String()
				conflict.Metadata["version_comparison"] = local.Version.Compare(remote.Version)
				
				// Try to detect changed fields by comparing event data
				if changedFields := sm.detectChangedFields(local.Event, remote.Event); len(changedFields) > 0 {
					conflict.ChangedFields = changedFields
				}
				
				// Add origin metadata if available from event metadata
				if localMeta := local.Event.Metadata(); localMeta != nil {
					if origin, ok := localMeta["origin"]; ok {
						conflict.Metadata["local_origin"] = origin
					}
					if timestamp, ok := localMeta["timestamp"]; ok {
						conflict.Metadata["local_timestamp"] = timestamp
					}
				}
				if remoteMeta := remote.Event.Metadata(); remoteMeta != nil {
					if origin, ok := remoteMeta["origin"]; ok {
						conflict.Metadata["remote_origin"] = origin
					}
					if timestamp, ok := remoteMeta["timestamp"]; ok {
						conflict.Metadata["remote_timestamp"] = timestamp
					}
				}
				
				conflicts = append(conflicts, conflict)
				
				sm.logger.Debug("Detected conflict",
					"aggregate_id", conflict.AggregateID,
					"event_type", conflict.EventType,
					"local_version", local.Version.String(),
					"remote_version", remote.Version.String(),
					"changed_fields", conflict.ChangedFields)
			}
		}
	}
	
	return conflicts
}


// isTransportHealthyForSync checks if the transport is healthy enough for sync operations
// This method supports state-aware transports and falls back to always allowing sync
// for non-stateful transports to maintain backward compatibility
func (sm *syncManager) isTransportHealthyForSync() bool {
	// Check if transport implements state awareness
	if statefulTransport, ok := sm.transport.(interface {
		IsHealthy() bool
	}); ok {
		isHealthy := statefulTransport.IsHealthy()
		if !isHealthy {
			sm.logger.Debug("Transport is not healthy for sync operations")
			return false
		}
		sm.logger.Debug("Transport is healthy for sync operations")
		return true
	}
	
	// Check if it's a StatefulTransportWrapper from statemachine package
	if statefulWrapper, ok := sm.transport.(interface {
		GetConnectionState() interface{}
		CanSendData() bool
		CanReceiveData() bool
	}); ok {
		// For sync operations, we need both send and receive capabilities
		canSend := statefulWrapper.CanSendData()
		canReceive := statefulWrapper.CanReceiveData()
		
		if !canSend || !canReceive {
			sm.logger.Debug("Transport state does not allow sync operations",
				"can_send", canSend,
				"can_receive", canReceive)
			return false
		}
		
		sm.logger.Debug("Transport state allows sync operations",
			"can_send", canSend,
			"can_receive", canReceive)
		return true
	}
	
	// For non-stateful transports, always allow sync (backward compatibility)
	sm.logger.Debug("Transport does not support state awareness, allowing sync")
	return true
}

// runProjectionsAfterSync executes all configured projection runners after a successful sync.
// It uses a worker pool to limit concurrency and applies timeouts to prevent hanging.
func (sm *syncManager) runProjectionsAfterSync(ctx context.Context, syncResult *SyncResult) error {
	// Early return if no projections are configured
	if sm.projectionConfig == nil || !sm.projectionConfig.RunOnSync || len(sm.projectionConfig.Runners) == 0 {
		sm.logger.Debug("No projections configured to run after sync")
		return nil
	}

	sm.logger.Info("Starting projection execution after successful sync",
		"runner_count", len(sm.projectionConfig.Runners),
		"max_workers", sm.projectionConfig.MaxWorkers,
		"timeout", sm.projectionConfig.Timeout)

	// Create context with timeout for projection execution
	projectionCtx, cancel := context.WithTimeout(ctx, sm.projectionConfig.Timeout)
	defer cancel()

	// Create error group for coordinating projection execution
	errorChan := make(chan error, len(sm.projectionConfig.Runners))
	completedChan := make(chan struct{}, len(sm.projectionConfig.Runners))

	// Execute each projection runner concurrently using the worker pool
	for i, runner := range sm.projectionConfig.Runners {
		go func(runnerIndex int, projRunner ProjectionRunner) {
			// Acquire worker slot from pool
			select {
			case sm.projectionPool <- struct{}{}:
				defer func() { <-sm.projectionPool }() // Release worker slot
			case <-projectionCtx.Done():
				errorChan <- fmt.Errorf("projection runner %d timed out waiting for worker slot", runnerIndex)
				return
			}

			sm.logger.Debug("Starting projection runner", "runner_index", runnerIndex)
			
			// Execute projection with timeout context - use ApplySince to catch up projections
			applied, lastVersion, err := projRunner.ApplySince(projectionCtx)
			if err != nil {
				sm.logger.Error("Projection runner failed", 
					"runner_index", runnerIndex,
					"error", err)
				errorChan <- fmt.Errorf("projection runner %d failed: %w", runnerIndex, err)
			} else {
				sm.logger.Debug("Projection runner completed successfully", 
					"runner_index", runnerIndex,
					"applied_events", applied,
					"last_version", lastVersion)
				completedChan <- struct{}{}
			}
		}(i, runner)
	}

	// Wait for all projections to complete or timeout/error
	var projectionErrors []error
	completedCount := 0
	totalRunners := len(sm.projectionConfig.Runners)

	for completedCount < totalRunners {
		select {
		case <-completedChan:
			completedCount++
		case err := <-errorChan:
			completedCount++
			projectionErrors = append(projectionErrors, err)
		case <-projectionCtx.Done():
			// Timeout occurred - collect any remaining runners as errors
			remainingRunners := totalRunners - completedCount
			if remainingRunners > 0 {
				timeoutErr := fmt.Errorf("projection execution timed out with %d runners still pending after %v", 
					remainingRunners, sm.projectionConfig.Timeout)
				projectionErrors = append(projectionErrors, timeoutErr)
			}
			break // Exit the waiting loop on timeout
		}
	}

	// Log projection execution summary
	if len(projectionErrors) == 0 {
		sm.logger.Info("All projection runners completed successfully", 
			"completed_count", completedCount)
		return nil
	} else {
		sm.logger.Error("Projection execution completed with errors",
			"completed_count", completedCount,
			"error_count", len(projectionErrors),
			"total_runners", totalRunners)
		
		// Combine all projection errors into a single error
		errorMessages := make([]string, len(projectionErrors))
		for i, err := range projectionErrors {
			errorMessages[i] = err.Error()
		}
		return fmt.Errorf("projection execution failed: %s", strings.Join(errorMessages, "; "))
	}
}

// detectChangedFields attempts to detect which fields changed between two events.
// This is a basic implementation that could be enhanced with more sophisticated
// comparison logic based on event structure.
func (sm *syncManager) detectChangedFields(local, remote Event) []string {
	// For now, we'll do a simple check based on event data comparison
	// In a real implementation, this would be more sophisticated and potentially
	// use reflection or schema-based comparison
	
	localData := local.Data()
	remoteData := remote.Data()
	
	if localData == nil && remoteData == nil {
		return nil
	}
	
	if localData == nil || remoteData == nil {
		return []string{"data"}
	}
	
	// Simple comparison - if data differs, mark "data" as changed
	// A more sophisticated implementation would inspect struct fields
	if fmt.Sprintf("%v", localData) != fmt.Sprintf("%v", remoteData) {
		return []string{"data"}
	}
	
	return nil
}
