package sync

import (
	"context"
	"fmt"
	"sync"
	"time"
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
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		return nil, fmt.Errorf("sync manager is closed")
	}
	sm.mu.RUnlock()

	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)
		sm.notifySubscribers(result)
	}()

	// Pull first to get latest remote changes
	if !sm.options.PushOnly {
		pullResult, err := sm.pull(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("pull failed: %w", err))
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
			result.Errors = append(result.Errors, fmt.Errorf("push failed: %w", err))
		} else {
			result.EventsPushed = pushResult.EventsPushed
		}
	}

	// Get final local version
	localVersion, err := sm.store.LatestVersion(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to get local version: %w", err))
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
		return nil, fmt.Errorf("sync manager is closed")
	}
	sm.mu.RUnlock()

	return sm.push(ctx)
}

// Pull retrieves remote events to local
func (sm *syncManager) Pull(ctx context.Context) (*SyncResult, error) {
	sm.mu.RLock()
	if sm.closed {
		sm.mu.RUnlock()
		return nil, fmt.Errorf("sync manager is closed")
	}
	sm.mu.RUnlock()

	return sm.pull(ctx)
}

func (sm *syncManager) push(ctx context.Context) (*SyncResult, error) {
	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)
	}()

	// Get remote version efficiently
	remoteVersion, err := sm.transport.GetLatestVersion(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to get remote version: %w", err)
	}

	// Load local events since remote version
	localEvents, err := sm.store.Load(ctx, remoteVersion)
	if err != nil {
		return result, fmt.Errorf("failed to load local events: %w", err)
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
		end := i + batchSize
		if end > len(localEvents) {
			end = len(localEvents)
		}

		batch := localEvents[i:end]
		if err := sm.transport.Push(ctx, batch); err != nil {
			return result, fmt.Errorf("failed to push batch: %w", err)
		}

		result.EventsPushed += len(batch)
	}

	return result, nil
}

func (sm *syncManager) pull(ctx context.Context) (*SyncResult, error) {
	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.Duration = time.Since(result.StartTime)
	}()

	// Get local version
	localVersion, err := sm.store.LatestVersion(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to get local version: %w", err)
	}

	// Pull remote events since our local version
	remoteEvents, err := sm.transport.Pull(ctx, localVersion)
	if err != nil {
		return result, fmt.Errorf("failed to pull remote events: %w", err)
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
		// Load local events that might conflict
		localEvents, err := sm.store.Load(ctx, localVersion)
		if err != nil {
			return result, fmt.Errorf("failed to load local events for conflict resolution: %w", err)
		}

		if len(localEvents) > 0 {
			resolvedEvents, err := sm.options.ConflictResolver.Resolve(ctx, localEvents, remoteEvents)
			if err != nil {
				return result, fmt.Errorf("conflict resolution failed: %w", err)
			}
			remoteEvents = resolvedEvents
			result.ConflictsResolved = len(localEvents) + len(remoteEvents) - len(resolvedEvents)
		}
	}

	// Store remote events locally
	for _, ev := range remoteEvents {
		if err := sm.store.Store(ctx, ev.Event, ev.Version); err != nil {
			return result, fmt.Errorf("failed to store remote event: %w", err)
		}
		result.EventsPulled++
	}

	// Get final remote version
	if len(remoteEvents) > 0 {
		result.RemoteVersion = findLatestVersion(remoteEvents)
	}

	return result, nil
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
		return fmt.Errorf("sync manager is closed")
	}

	if sm.options.SyncInterval <= 0 {
		return fmt.Errorf("sync interval must be positive")
	}

	if sm.autoSyncStop != nil {
		return fmt.Errorf("auto sync is already running")
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
				// Create timeout context for each sync
				syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		return fmt.Errorf("auto sync is not running")
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
		return fmt.Errorf("sync manager is closed")
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
		errs = append(errs, fmt.Errorf("failed to close transport: %w", err))
	}
	if err := sm.store.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close store: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
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
					// Log panic from subscriber, but don't crash
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
	for _, ev := range events[1:] {
		if ev.Version.Compare(latest) > 0 {
			latest = ev.Version
		}
	}
	return latest
}
