package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	stdSync "sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
)

// NotificationPayload represents the structure of notification payloads
type NotificationPayload struct {
	Version     int64     `json:"version"`
	ID          string    `json:"id"`
	AggregateID string    `json:"aggregate_id"`
	EventType   string    `json:"event_type"`
	StreamName  string    `json:"stream_name,omitempty"` // Only present in global notifications
	CreatedAt   time.Time `json:"created_at"`
}

// EventHandler is a function type for handling incoming event notifications
type EventHandler func(payload NotificationPayload) error

// SubscriptionManager manages subscriptions to PostgreSQL LISTEN/NOTIFY channels
type SubscriptionManager struct {
	subscriptions map[string][]EventHandler
	mu            stdSync.RWMutex
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string][]EventHandler),
	}
}

// Subscribe adds a handler for a specific channel
func (sm *SubscriptionManager) Subscribe(channel string, handler EventHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.subscriptions[channel] = append(sm.subscriptions[channel], handler)
}

// Unsubscribe removes handlers for a specific channel
func (sm *SubscriptionManager) Unsubscribe(channel string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.subscriptions, channel)
}

// GetChannels returns all subscribed channels
func (sm *SubscriptionManager) GetChannels() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	channels := make([]string, 0, len(sm.subscriptions))
	for channel := range sm.subscriptions {
		channels = append(channels, channel)
	}
	return channels
}

// HandleNotification processes an incoming notification
func (sm *SubscriptionManager) HandleNotification(channel string, payload string) error {
	sm.mu.RLock()
	handlers, exists := sm.subscriptions[channel]
	sm.mu.RUnlock()
	
	if !exists {
		return nil // No handlers for this channel
	}
	
	// Parse the JSON payload
	var notificationPayload NotificationPayload
	if err := json.Unmarshal([]byte(payload), &notificationPayload); err != nil {
		return fmt.Errorf("failed to parse notification payload: %w", err)
	}
	
	// Call all handlers for this channel
	for _, handler := range handlers {
		if err := handler(notificationPayload); err != nil {
			// Log error but continue processing other handlers
			// In a production system, you might want to implement retry logic
			return fmt.Errorf("handler error for channel %s: %w", channel, err)
		}
	}
	
	return nil
}

// NotificationListener manages PostgreSQL LISTEN/NOTIFY connections for real-time event streaming
type NotificationListener struct {
	connectionString string
	logger           *log.Logger
	
	// Connection management
	listener *pq.Listener
	mu       stdSync.RWMutex
	closed   int32 // atomic
	
	// Subscription management
	subscriptions *SubscriptionManager
	
	// Configuration
	reconnectInterval    time.Duration
	maxReconnectAttempts int
	notificationTimeout  time.Duration
	
	// Channels for coordination
	done chan struct{}
}

// NewNotificationListener creates a new PostgreSQL notification listener
func NewNotificationListener(connectionString string, logger *log.Logger) (*NotificationListener, error) {
	if connectionString == "" {
		return nil, fmt.Errorf("connection string cannot be empty")
	}
	
	if logger == nil {
		logger = log.New(log.Writer(), "[NotificationListener] ", log.LstdFlags)
	}
	
	nl := &NotificationListener{
		connectionString:     connectionString,
		logger:              logger,
		subscriptions:       NewSubscriptionManager(),
		reconnectInterval:   5 * time.Second,
		maxReconnectAttempts: 10,
		notificationTimeout: 30 * time.Second,
		done:                make(chan struct{}),
	}
	
	// Initialize the pq.Listener
	nl.listener = pq.NewListener(
		connectionString,
		nl.reconnectInterval,
		nl.notificationTimeout,
		nl.eventCallback,
	)
	
	return nl, nil
}

// eventCallback handles pq.Listener events
func (nl *NotificationListener) eventCallback(event pq.ListenerEventType, err error) {
	switch event {
	case pq.ListenerEventConnected:
		nl.logger.Printf("Connected to PostgreSQL for LISTEN/NOTIFY")
	case pq.ListenerEventDisconnected:
		nl.logger.Printf("Disconnected from PostgreSQL: %v", err)
	case pq.ListenerEventReconnected:
		nl.logger.Printf("Reconnected to PostgreSQL")
		// Re-subscribe to all channels after reconnection
		nl.resubscribeAllChannels()
	case pq.ListenerEventConnectionAttemptFailed:
		nl.logger.Printf("Connection attempt failed: %v", err)
	}
}

// resubscribeAllChannels re-subscribes to all channels after reconnection
func (nl *NotificationListener) resubscribeAllChannels() {
	channels := nl.subscriptions.GetChannels()
	for _, channel := range channels {
		if err := nl.listener.Listen(channel); err != nil {
			nl.logger.Printf("Failed to re-subscribe to channel %s: %v", channel, err)
		} else {
			nl.logger.Printf("Re-subscribed to channel: %s", channel)
		}
	}
}

// Start begins listening for notifications
func (nl *NotificationListener) Start(ctx context.Context) error {
	if atomic.LoadInt32(&nl.closed) == 1 {
		return fmt.Errorf("listener is closed")
	}
	
	nl.logger.Printf("Starting notification listener...")
	
	go nl.listenLoop(ctx)
	return nil
}

// listenLoop is the main loop that processes notifications
func (nl *NotificationListener) listenLoop(ctx context.Context) {
	defer nl.logger.Printf("Notification listener stopped")
	
	for {
		select {
		case <-ctx.Done():
			nl.logger.Printf("Context cancelled, stopping listener")
			return
		case <-nl.done:
			nl.logger.Printf("Listener closed, stopping listen loop")
			return
		case notification := <-nl.listener.Notify:
			if notification != nil {
				nl.handleNotification(notification)
			}
		case <-time.After(90 * time.Second):
			// Send a ping to keep the connection alive
			go func() {
				if err := nl.listener.Ping(); err != nil {
					nl.logger.Printf("Ping failed: %v", err)
				}
			}()
		}
	}
}

// handleNotification processes a single notification
func (nl *NotificationListener) handleNotification(notification *pq.Notification) {
	if notification == nil {
		return
	}
	
	nl.logger.Printf("Received notification on channel %s: %s", notification.Channel, notification.Extra)
	
	if err := nl.subscriptions.HandleNotification(notification.Channel, notification.Extra); err != nil {
		nl.logger.Printf("Error handling notification: %v", err)
	}
}

// SubscribeToStream subscribes to events for a specific aggregate stream
func (nl *NotificationListener) SubscribeToStream(aggregateID string, handler EventHandler) error {
	if atomic.LoadInt32(&nl.closed) == 1 {
		return fmt.Errorf("listener is closed")
	}
	
	channel := fmt.Sprintf("stream_%s", aggregateID)
	
	// Add to subscription manager
	nl.subscriptions.Subscribe(channel, handler)
	
	// Subscribe to the channel in PostgreSQL
	if err := nl.listener.Listen(channel); err != nil {
		// Remove from subscription manager if PostgreSQL subscription failed
		nl.subscriptions.Unsubscribe(channel)
		return fmt.Errorf("failed to listen to channel %s: %w", channel, err)
	}
	
	nl.logger.Printf("Subscribed to stream: %s", aggregateID)
	return nil
}

// SubscribeToAll subscribes to all events via the global channel
func (nl *NotificationListener) SubscribeToAll(handler EventHandler) error {
	if atomic.LoadInt32(&nl.closed) == 1 {
		return fmt.Errorf("listener is closed")
	}
	
	channel := "events_global"
	
	// Add to subscription manager
	nl.subscriptions.Subscribe(channel, handler)
	
	// Subscribe to the channel in PostgreSQL
	if err := nl.listener.Listen(channel); err != nil {
		// Remove from subscription manager if PostgreSQL subscription failed
		nl.subscriptions.Unsubscribe(channel)
		return fmt.Errorf("failed to listen to global channel: %w", err)
	}
	
	nl.logger.Printf("Subscribed to global events channel")
	return nil
}

// SubscribeToEventType subscribes to events of a specific type across all streams
// This is implemented by subscribing to the global channel and filtering by event type
func (nl *NotificationListener) SubscribeToEventType(eventType string, handler EventHandler) error {
	if atomic.LoadInt32(&nl.closed) == 1 {
		return fmt.Errorf("listener is closed")
	}
	
	// Create a filtering handler that only processes events of the specified type
	filteringHandler := func(payload NotificationPayload) error {
		if payload.EventType == eventType {
			return handler(payload)
		}
		return nil // Skip events of other types
	}
	
	return nl.SubscribeToAll(filteringHandler)
}

// UnsubscribeFromStream unsubscribes from a specific aggregate stream
func (nl *NotificationListener) UnsubscribeFromStream(aggregateID string) error {
	channel := fmt.Sprintf("stream_%s", aggregateID)
	
	// Remove from subscription manager
	nl.subscriptions.Unsubscribe(channel)
	
	// Unsubscribe from the channel in PostgreSQL
	if err := nl.listener.Unlisten(channel); err != nil {
		return fmt.Errorf("failed to unlisten from channel %s: %w", channel, err)
	}
	
	nl.logger.Printf("Unsubscribed from stream: %s", aggregateID)
	return nil
}

// UnsubscribeFromAll unsubscribes from the global events channel
func (nl *NotificationListener) UnsubscribeFromAll() error {
	channel := "events_global"
	
	// Remove from subscription manager
	nl.subscriptions.Unsubscribe(channel)
	
	// Unsubscribe from the channel in PostgreSQL
	if err := nl.listener.Unlisten(channel); err != nil {
		return fmt.Errorf("failed to unlisten from global channel: %w", err)
	}
	
	nl.logger.Printf("Unsubscribed from global events channel")
	return nil
}

// GetActiveChannels returns a list of currently subscribed channels
func (nl *NotificationListener) GetActiveChannels() []string {
	return nl.subscriptions.GetChannels()
}

// IsConnected returns true if the listener is connected to PostgreSQL
func (nl *NotificationListener) IsConnected() bool {
	if atomic.LoadInt32(&nl.closed) == 1 {
		return false
	}
	
	// Use ping to check connectivity
	return nl.listener.Ping() == nil
}

// Close shuts down the notification listener
func (nl *NotificationListener) Close() error {
	if !atomic.CompareAndSwapInt32(&nl.closed, 0, 1) {
		return nil // Already closed
	}
	
	nl.logger.Printf("Closing notification listener...")
	
	// Signal the listen loop to stop
	close(nl.done)
	
	// Close the pq.Listener
	if nl.listener != nil {
		if err := nl.listener.Close(); err != nil {
			nl.logger.Printf("Error closing pq.Listener: %v", err)
			return err
		}
	}
	
	nl.logger.Printf("Notification listener closed")
	return nil
}

// WithReconnectSettings allows customizing reconnection behavior
func (nl *NotificationListener) WithReconnectSettings(interval time.Duration, maxAttempts int) *NotificationListener {
	nl.reconnectInterval = interval
	nl.maxReconnectAttempts = maxAttempts
	return nl
}

// WithNotificationTimeout allows customizing the notification timeout
func (nl *NotificationListener) WithNotificationTimeout(timeout time.Duration) *NotificationListener {
	nl.notificationTimeout = timeout
	return nl
}

// RealtimeEventStore extends the PostgresEventStore with real-time subscription capabilities
type RealtimeEventStore struct {
	*PostgresEventStore
}

// NewRealtimeEventStore creates a new PostgresEventStore with real-time capabilities
func NewRealtimeEventStore(config *Config) (*RealtimeEventStore, error) {
	store, err := New(config)
	if err != nil {
		return nil, err
	}
	
	return &RealtimeEventStore{
		PostgresEventStore: store,
	}, nil
}

// SubscribeToStream subscribes to real-time events for a specific aggregate
func (rs *RealtimeEventStore) SubscribeToStream(ctx context.Context, aggregateID string, handler func(NotificationPayload) error) error {
	if err := rs.listener.Start(ctx); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	
	return rs.listener.SubscribeToStream(aggregateID, handler)
}

// SubscribeToAll subscribes to all real-time events
func (rs *RealtimeEventStore) SubscribeToAll(ctx context.Context, handler func(NotificationPayload) error) error {
	if err := rs.listener.Start(ctx); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	
	return rs.listener.SubscribeToAll(handler)
}

// SubscribeToEventType subscribes to events of a specific type
func (rs *RealtimeEventStore) SubscribeToEventType(ctx context.Context, eventType string, handler func(NotificationPayload) error) error {
	if err := rs.listener.Start(ctx); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	
	return rs.listener.SubscribeToEventType(eventType, handler)
}

// GetActiveSubscriptions returns information about active subscriptions
func (rs *RealtimeEventStore) GetActiveSubscriptions() []string {
	return rs.listener.GetActiveChannels()
}

// IsListenerConnected returns true if the notification listener is connected
func (rs *RealtimeEventStore) IsListenerConnected() bool {
	return rs.listener.IsConnected()
}
