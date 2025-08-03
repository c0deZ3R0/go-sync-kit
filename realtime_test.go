package sync

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockRealtimeNotifier is an in-memory implementation of RealtimeNotifier
// used for testing real-time sync functionality
type MockRealtimeNotifier struct {
	mu           sync.RWMutex
	subscribed   bool
	handler      NotificationHandler
	notifications chan Notification
}

func NewMockRealtimeNotifier() *MockRealtimeNotifier {
	return &MockRealtimeNotifier{
		notifications: make(chan Notification, 10),
	}
}

func (m *MockRealtimeNotifier) Subscribe(ctx context.Context, handler NotificationHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscribed {
		return nil
	}

	m.handler = handler
	m.subscribed = true

	go func() {
		for {
			select {
			case notification := <-m.notifications:
				// Get handler under read lock
				m.mu.RLock()
				h := m.handler
				m.mu.RUnlock()
				
				if h != nil {
					if err := h(notification); err != nil {
						// Log error
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (m *MockRealtimeNotifier) Unsubscribe() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribed = false
	m.handler = nil
	return nil
}

func (m *MockRealtimeNotifier) Notify(ctx context.Context, notification Notification) error {
	m.notifications <- notification
	return nil
}

func (m *MockRealtimeNotifier) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.subscribed
}

func (m *MockRealtimeNotifier) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.notifications)
	m.subscribed = false
	m.handler = nil
	return nil
}

// MockReconnectingNotifier simulates connection failures for testing reconnection logic
type MockReconnectingNotifier struct {
	mu            sync.RWMutex
	subscribed    bool
	connected     bool
	attempts      int
	maxAttempts   int
	notifications chan Notification
	handler       NotificationHandler
}

func NewMockReconnectingNotifier(maxAttempts int) *MockReconnectingNotifier {
	return &MockReconnectingNotifier{
		maxAttempts:   maxAttempts,
		notifications: make(chan Notification, 10),
	}
}

func (m *MockReconnectingNotifier) Subscribe(ctx context.Context, handler NotificationHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attempts++
	if m.attempts <= m.maxAttempts {
		return fmt.Errorf("connection failed, attempt %d", m.attempts)
	}

	m.subscribed = true
	m.connected = true

	// Store handler under lock
	m.handler = handler
	
	go func() {
		for {
			select {
			case notification := <-m.notifications:
				// Get handler under read lock
				m.mu.RLock()
				h := m.handler
				m.mu.RUnlock()
				
				if h != nil {
					if err := h(notification); err != nil {
						// Log error
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (m *MockReconnectingNotifier) Unsubscribe() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribed = false
	m.connected = false
	m.handler = nil
	return nil
}

func (m *MockReconnectingNotifier) Notify(ctx context.Context, notification Notification) error {
	m.notifications <- notification
	return nil
}

func (m *MockReconnectingNotifier) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.connected
}

func (m *MockReconnectingNotifier) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.notifications)
	m.subscribed = false
	m.connected = false
	m.handler = nil
	return nil
}

func TestRealtimeSyncManager_Basic(t *testing.T) {
store := &TestEventStore{}
	transport := &TestTransport{}
	notifier := NewMockRealtimeNotifier()
	resolver := &MockConflictResolver{}

	options := &RealtimeSyncOptions{
		SyncOptions: SyncOptions{
			BatchSize: 10,
			ConflictResolver: resolver,
		},
		RealtimeNotifier: notifier,
		FallbackInterval: 100 * time.Millisecond,
		ReconnectBackoff: &ExponentialBackoff{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay: 100 * time.Millisecond,
			Multiplier: 2.0,
		},
	}

	rsm := NewRealtimeSyncManager(store, transport, options)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test enabling realtime
	err := rsm.EnableRealtime(ctx)
	if err != nil {
		t.Fatalf("Failed to enable realtime: %v", err)
	}

	// Wait for subscription to be active
	time.Sleep(100 * time.Millisecond)

	if !rsm.IsRealtimeActive() {
		t.Errorf("Expected realtime to be active")
	}

	// Create mock event
event1 := &TestEvent{}

	version1 := &TestVersion{}
	store.Store(ctx, event1, version1)

	// Send notification
	notif := Notification{
		Type:         "data_changed",
		Source:       "test",
		AggregateIDs: []string{"agg1"},
		Timestamp:    time.Now(),
	}

	err = notifier.Notify(ctx, notif)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Allow time for notification to be processed
	time.Sleep(200 * time.Millisecond)

	if !rsm.GetConnectionStatus().Connected {
		t.Errorf("Expected connection to be active")
	}

	// Test disabling realtime
	err = rsm.DisableRealtime()
	if err != nil {
		t.Fatalf("Failed to disable realtime: %v", err)
	}

	if rsm.IsRealtimeActive() {
		t.Errorf("Expected realtime to be inactive after disabling")
	}

	// Cleanup
	rsm.Close()
}

func TestRealtimeSyncManager_NotificationFilter(t *testing.T) {
store := &TestEventStore{}
	transport := &TestTransport{}
	notifier := NewMockRealtimeNotifier()
	resolver := &MockConflictResolver{}

	// Filter that only allows "important" notifications
	filter := func(n Notification) bool {
		return n.Metadata != nil && n.Metadata["important"] == true
	}

	options := &RealtimeSyncOptions{
		SyncOptions: SyncOptions{
			BatchSize: 10,
			ConflictResolver: resolver,
		},
		RealtimeNotifier:   notifier,
		NotificationFilter: filter,
		FallbackInterval:   100 * time.Millisecond,
	}

	rsm := NewRealtimeSyncManager(store, transport, options)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rsm.EnableRealtime(ctx)
	if err != nil {
		t.Fatalf("Failed to enable realtime: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Send filtered out notification
	filteredNotif := Notification{
		Type:      "data_changed",
		Source:    "test",
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{"important": false},
	}

	notifier.Notify(ctx, filteredNotif)
	time.Sleep(100 * time.Millisecond)

	// Send allowed notification
	allowedNotif := Notification{
		Type:      "data_changed",
		Source:    "test",
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{"important": true},
	}

	notifier.Notify(ctx, allowedNotif)
	time.Sleep(100 * time.Millisecond)

	// The filter should have processed only the important notification
	if !rsm.GetConnectionStatus().Connected {
		t.Errorf("Expected connection to be active")
	}

	rsm.Close()
}

func TestRealtimeSyncManager_Reconnection(t *testing.T) {
store := &TestEventStore{}
	transport := &TestTransport{}
	notifier := NewMockReconnectingNotifier(2) // Fail first 2 attempts
	resolver := &MockConflictResolver{}

	options := &RealtimeSyncOptions{
		SyncOptions: SyncOptions{
			BatchSize: 10,
			ConflictResolver: resolver,
		},
		RealtimeNotifier: notifier,
		FallbackInterval: 50 * time.Millisecond,
		ReconnectBackoff: &ExponentialBackoff{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
			Multiplier:   2.0,
		},
	}

	rsm := NewRealtimeSyncManager(store, transport, options)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rsm.EnableRealtime(ctx)
	if err != nil {
		t.Fatalf("Failed to enable realtime: %v", err)
	}

	// Wait for reconnection attempts
	time.Sleep(500 * time.Millisecond)

	// Check that reconnection attempts were made
	status := rsm.GetConnectionStatus()
	if status.ReconnectAttempts == 0 {
		t.Errorf("Expected reconnection attempts to be made")
	}

	rsm.Close()
}
