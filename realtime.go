package sync

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RealtimeNotifier provides real-time push notifications when data changes.
// This is separate from Transport to allow for different notification mechanisms
// (WebSockets, SSE, NATS, etc.) while keeping the sync logic agnostic.
type RealtimeNotifier interface {
	// Subscribe starts listening for real-time notifications
	// The handler will be called when new data is available for sync
	Subscribe(ctx context.Context, handler NotificationHandler) error
	
	// Unsubscribe stops listening for notifications
	Unsubscribe() error
	
	// Notify sends a notification to connected clients (server-side)
	Notify(ctx context.Context, notification Notification) error
	
	// IsConnected returns true if the real-time connection is active
	IsConnected() bool
	
	// Close closes the notifier connection
	Close() error
}

// NotificationHandler processes incoming real-time notifications
type NotificationHandler func(notification Notification) error

// Notification represents a real-time update notification
type Notification struct {
	// Type of notification (data_changed, sync_requested, etc.)
	Type string
	
	// Source identifies where the change originated
	Source string
	
	// AggregateIDs that were affected (optional, for filtering)
	AggregateIDs []string
	
	// Metadata for additional context
	Metadata map[string]interface{}
	
	// Timestamp when the notification was created
	Timestamp time.Time
}

// NotificationFilter allows filtering which notifications to process
type NotificationFilter func(notification Notification) bool

// RealtimeSyncOptions extends SyncOptions with real-time capabilities
type RealtimeSyncOptions struct {
	SyncOptions
	
	// RealtimeNotifier for push notifications (optional)
	RealtimeNotifier RealtimeNotifier
	
	// NotificationFilter to process only relevant notifications
	NotificationFilter NotificationFilter
	
	// DisablePolling stops periodic sync when real-time is active
	DisablePolling bool
	
	// FallbackInterval for polling when real-time connection is lost
	FallbackInterval time.Duration
	
	// ReconnectBackoff strategy for reconnecting to real-time notifications
	ReconnectBackoff BackoffStrategy
}

// BackoffStrategy defines how to handle reconnection delays
type BackoffStrategy interface {
	// NextDelay returns the delay before the next reconnection attempt
	NextDelay(attempt int) time.Duration
	
	// Reset resets the backoff strategy after a successful connection
	Reset()
}

// ExponentialBackoff implements exponential backoff with jitter
type ExponentialBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func (eb *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	// Ensure attempt is at least 0
	if attempt < 0 {
		attempt = 0
	}

	// Base delay is always InitialDelay
	delay := float64(eb.InitialDelay)

	// Calculate exponential multiplier: Multiplier^attempt
	multiplier := 1.0
	for i := 0; i < attempt; i++ {
		multiplier *= eb.Multiplier
	}

	// Apply multiplier to initial delay
	result := time.Duration(delay * multiplier)

	// Cap at MaxDelay
	if result > eb.MaxDelay {
		result = eb.MaxDelay
	}

	return result
}

func (eb *ExponentialBackoff) Reset() {
	// Nothing to reset for exponential backoff
}

// RealtimeSyncManager extends SyncManager with real-time capabilities
type RealtimeSyncManager interface {
	SyncManager
	
	// EnableRealtime starts real-time notifications
	EnableRealtime(ctx context.Context) error
	
	// DisableRealtime stops real-time notifications and falls back to polling
	DisableRealtime() error
	
	// IsRealtimeActive returns true if real-time notifications are active
	IsRealtimeActive() bool
	
	// GetConnectionStatus returns the current real-time connection status
	GetConnectionStatus() ConnectionStatus
}

// ConnectionStatus represents the state of the real-time connection
type ConnectionStatus struct {
	Connected     bool
	LastConnected time.Time
	ReconnectAttempts int
	Error         error
}

// NewRealtimeSyncManager creates a sync manager with real-time capabilities
func NewRealtimeSyncManager(store EventStore, transport Transport, options *RealtimeSyncOptions) RealtimeSyncManager {
	if options == nil {
		options = &RealtimeSyncOptions{
			SyncOptions: SyncOptions{
				BatchSize: 100,
			},
			FallbackInterval: 30 * time.Second,
			ReconnectBackoff: &ExponentialBackoff{
				InitialDelay: 1 * time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
			},
		}
	}
	
	return &realtimeSyncManager{
		syncManager: syncManager{
			store:     store,
			transport: transport,
			options:   options.SyncOptions,
		},
		realtimeOptions: *options,
		connectionStatus: ConnectionStatus{},
	}
}

// getStopChannel safely gets the notification stop channel
func (rsm *realtimeSyncManager) getStopChannel() <-chan struct{} {
	rsm.realtimeMu.RLock()
	defer rsm.realtimeMu.RUnlock()
	return rsm.notificationStop
}

// Helper methods for thread-safe connectionStatus updates
func (rsm *realtimeSyncManager) updateConnectionStatus(connected bool, lastConnected time.Time, err error) {
	rsm.realtimeMu.Lock()
	defer rsm.realtimeMu.Unlock()
	
	rsm.connectionStatus.Connected = connected
	if !lastConnected.IsZero() {
		rsm.connectionStatus.LastConnected = lastConnected
	}
	rsm.connectionStatus.Error = err
}

func (rsm *realtimeSyncManager) updateReconnectAttempts(attempts int) {
	rsm.realtimeMu.Lock()
	defer rsm.realtimeMu.Unlock()
	
	rsm.connectionStatus.ReconnectAttempts = attempts
}

// realtimeSyncManager implements RealtimeSyncManager
type realtimeSyncManager struct {
	syncManager
	realtimeOptions  RealtimeSyncOptions
	connectionStatus ConnectionStatus
	realtimeMu       sync.RWMutex
	notificationStop chan struct{}
	fallbackTicker   *time.Ticker
}

// EnableRealtime starts real-time notifications
func (rsm *realtimeSyncManager) EnableRealtime(ctx context.Context) error {
	rsm.realtimeMu.Lock()
	defer rsm.realtimeMu.Unlock()
	
	if rsm.realtimeOptions.RealtimeNotifier == nil {
		return fmt.Errorf("no realtime notifier configured")
	}
	
	if rsm.notificationStop != nil {
		return fmt.Errorf("realtime notifications already active")
	}
	
	rsm.notificationStop = make(chan struct{})
	
	// Start notification handler
	go rsm.handleNotifications(ctx)
	
	// Start connection monitor
	go rsm.monitorConnection(ctx)
	
	return nil
}

// DisableRealtime stops real-time notifications
func (rsm *realtimeSyncManager) DisableRealtime() error {
	rsm.realtimeMu.Lock()
	defer rsm.realtimeMu.Unlock()
	
	if rsm.notificationStop == nil {
		return fmt.Errorf("realtime notifications not active")
	}
	
	close(rsm.notificationStop)
	rsm.notificationStop = nil
	
	if rsm.fallbackTicker != nil {
		rsm.fallbackTicker.Stop()
		rsm.fallbackTicker = nil
	}
	
	if rsm.realtimeOptions.RealtimeNotifier != nil {
		rsm.realtimeOptions.RealtimeNotifier.Unsubscribe()
	}
	
	return nil
}

// IsRealtimeActive returns true if real-time notifications are active
func (rsm *realtimeSyncManager) IsRealtimeActive() bool {
	rsm.realtimeMu.RLock()
	defer rsm.realtimeMu.RUnlock()
	return rsm.notificationStop != nil
}

// GetConnectionStatus returns the current connection status
func (rsm *realtimeSyncManager) GetConnectionStatus() ConnectionStatus {
	rsm.realtimeMu.RLock()
	defer rsm.realtimeMu.RUnlock()
	return rsm.connectionStatus
}

func (rsm *realtimeSyncManager) handleNotifications(ctx context.Context) {
	notifier := rsm.realtimeOptions.RealtimeNotifier
	backoff := rsm.realtimeOptions.ReconnectBackoff
	if backoff == nil {
		backoff = &ExponentialBackoff{
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		}
	}

	// Add maximum timeout for connection attempts
	maxTimeout := 5 * time.Minute
	attempt := 0
	
	for {
		// FIXED: Check stop channel under lock to avoid race
		rsm.realtimeMu.RLock()
		stopChan := rsm.notificationStop
		rsm.realtimeMu.RUnlock()
		
		if stopChan == nil {
			return // Already stopped
		}
		
		select {
		case <-ctx.Done():
			return
		case <-stopChan: // Use local copy to avoid race
			return
		default:
		}
		
		// Try to subscribe to notifications
		err := notifier.Subscribe(ctx, func(notification Notification) error {
			// Reset backoff on successful notification
			backoff.Reset()
			attempt = 0
			
			// FIXED: Use thread-safe helper method
			rsm.updateConnectionStatus(true, time.Now(), nil)
			
			// Apply filter if configured
			if rsm.realtimeOptions.NotificationFilter != nil {
				if !rsm.realtimeOptions.NotificationFilter(notification) {
					return nil // Skip filtered notifications
				}
			}
			
			// Handle different notification types
			switch notification.Type {
			case "data_changed", "sync_requested":
				// Trigger sync in background
				go func() {
					syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					
					result, err := rsm.Sync(syncCtx)
					if err == nil {
						rsm.notifySubscribers(result)
					}
				}()
			}
			
			return nil
		})
		
		if err != nil {
			// FIXED: Use thread-safe helper methods
			rsm.updateConnectionStatus(false, time.Time{}, err)
			rsm.updateReconnectAttempts(attempt)
			
			// Start fallback polling if not disabled
			if !rsm.realtimeOptions.DisablePolling && rsm.fallbackTicker == nil {
				rsm.startFallbackPolling(ctx)
			}
			
			// Wait before reconnecting
			delay := backoff.NextDelay(attempt)
			if delay > maxTimeout {
				delay = maxTimeout
			}
			attempt++

		// FIXED: Check stop channel under lock to avoid race
		rsm.realtimeMu.RLock()
		stopChan := rsm.notificationStop
		rsm.realtimeMu.RUnlock()
		
		if stopChan == nil {
			return // Already stopped
		}
		
		select {
		case <-ctx.Done():
			return
		case <-stopChan: // Use local copy to avoid race
			return
		case <-time.After(delay):
			continue
		}
		} else {
			// Connection successful, stop fallback polling
			rsm.stopFallbackPolling()
		}
	}
}

func (rsm *realtimeSyncManager) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		// FIXED: Check stop channel under lock to avoid race
		rsm.realtimeMu.RLock()
		stopChan := rsm.notificationStop
		rsm.realtimeMu.RUnlock()
		
		if stopChan == nil {
			return // Already stopped
		}
		
		select {
		case <-ctx.Done():
			return
		case <-stopChan: // Use local copy to avoid race
			return
		case <-ticker.C:
			if rsm.realtimeOptions.RealtimeNotifier != nil {
				connected := rsm.realtimeOptions.RealtimeNotifier.IsConnected()
				
				// FIXED: Use thread-safe helper method and check status while holding lock
				rsm.realtimeMu.Lock()
				prevConnected := rsm.connectionStatus.Connected
				rsm.realtimeMu.Unlock()
				
				if prevConnected != connected {
					if connected {
						rsm.updateConnectionStatus(true, time.Now(), nil)
						rsm.stopFallbackPolling()
					} else {
						rsm.updateConnectionStatus(false, time.Time{}, nil)
						rsm.startFallbackPolling(ctx)
					}
				}
			}
		}
	}
}

func (rsm *realtimeSyncManager) startFallbackPolling(ctx context.Context) {
	rsm.realtimeMu.Lock()
	if rsm.fallbackTicker != nil || rsm.realtimeOptions.DisablePolling {
		rsm.realtimeMu.Unlock()
		return
	}
	
	rsm.fallbackTicker = time.NewTicker(rsm.realtimeOptions.FallbackInterval)
	ticker := rsm.fallbackTicker // Keep local reference
	rsm.realtimeMu.Unlock()
	
	go func() {
		defer ticker.Stop()
		
		for {
			// FIXED: Check stop channel under lock to avoid race
			rsm.realtimeMu.RLock()
			stopChan := rsm.notificationStop
			tickChan := rsm.fallbackTicker.C
			rsm.realtimeMu.RUnlock()
			
			if stopChan == nil {
				return // Already stopped
			}
			
			select {
			case <-ctx.Done():
				return
			case <-stopChan: // Use local copy to avoid race
				return
			case <-tickChan:
				// Only poll if real-time is not connected
				if !rsm.realtimeOptions.RealtimeNotifier.IsConnected() {
					go func() {
						syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						
						result, err := rsm.Sync(syncCtx)
						if err == nil {
							rsm.notifySubscribers(result)
						}
					}()
				}
			}
		}
	}()
}

func (rsm *realtimeSyncManager) stopFallbackPolling() {
	rsm.realtimeMu.Lock()
	defer rsm.realtimeMu.Unlock()
	
	if rsm.fallbackTicker != nil {
		rsm.fallbackTicker.Stop()
		rsm.fallbackTicker = nil
	}
}

// Close extends the base close method to handle real-time resources
func (rsm *realtimeSyncManager) Close() error {
	// Disable real-time first
	rsm.DisableRealtime()
	
	// Close the notifier
	if rsm.realtimeOptions.RealtimeNotifier != nil {
		if err := rsm.realtimeOptions.RealtimeNotifier.Close(); err != nil {
			// Log error but continue with cleanup
		}
	}
	
	// Call base close
	return rsm.syncManager.Close()
}
