package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/postgres"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// UserEvent represents a domain event for user operations
type UserEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        map[string]interface{}
	metadata    map[string]interface{}
	timestamp   time.Time
}

func (e *UserEvent) ID() string          { return e.id }
func (e *UserEvent) Type() string        { return e.eventType }
func (e *UserEvent) AggregateID() string { return e.aggregateID }
func (e *UserEvent) Data() interface{}   { return e.data }
func (e *UserEvent) Metadata() map[string]interface{} { return e.metadata }

// NewUserEvent creates a new user event
func NewUserEvent(id, eventType, aggregateID string, data map[string]interface{}) *UserEvent {
	return &UserEvent{
		id:          id,
		eventType:   eventType,
		aggregateID: aggregateID,
		data:        data,
		metadata: map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"source":    "postgres-example",
		},
		timestamp: time.Now(),
	}
}

func main() {
	logger := log.New(os.Stdout, "[PostgreSQL EventStore Example] ", log.LstdFlags)

	// Get connection string from environment or use default
	connectionString := os.Getenv("POSTGRES_CONNECTION")
	if connectionString == "" {
		connectionString = "postgres://testuser:testpass123@localhost:5432/eventstore_test?sslmode=disable"
	}

	logger.Printf("Connecting to PostgreSQL: %s", maskPassword(connectionString))

	// Create configuration
	config := postgres.DefaultConfig(connectionString)
	config.Logger = logger
	config.MaxOpenConns = 10
	config.MaxIdleConns = 5

	// Create basic EventStore
	logger.Println("Creating PostgreSQL EventStore...")
	store, err := postgres.New(config)
	if err != nil {
		log.Fatal("Failed to create EventStore:", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Demonstrate basic EventStore operations
	demonstrateBasicOperations(ctx, store, logger)

	// Create realtime EventStore for LISTEN/NOTIFY functionality
	logger.Println("\nCreating Realtime EventStore for LISTEN/NOTIFY...")
	realtimeStore, err := postgres.NewRealtimeEventStore(config)
	if err != nil {
		log.Fatal("Failed to create Realtime EventStore:", err)
	}
	defer realtimeStore.Close()

	// Demonstrate real-time notifications
	demonstrateRealtimeNotifications(ctx, realtimeStore, logger)
}

func demonstrateBasicOperations(ctx context.Context, store *postgres.PostgresEventStore, logger *log.Logger) {
	logger.Println("\n=== Basic EventStore Operations ===")

	// Store some events
	events := []*UserEvent{
		NewUserEvent("evt-1", "UserCreated", "user-alice", map[string]interface{}{
			"name":  "Alice Johnson",
			"email": "alice@example.com",
			"role":  "admin",
		}),
		NewUserEvent("evt-2", "UserUpdated", "user-alice", map[string]interface{}{
			"email": "alice.johnson@example.com",
		}),
		NewUserEvent("evt-3", "UserCreated", "user-bob", map[string]interface{}{
			"name":  "Bob Smith",
			"email": "bob@example.com",
			"role":  "user",
		}),
		NewUserEvent("evt-4", "UserActivated", "user-alice", map[string]interface{}{
			"activated_at": time.Now().Unix(),
		}),
	}

	logger.Printf("Storing %d events...", len(events))
	for _, event := range events {
		err := store.Store(ctx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			log.Printf("Failed to store event %s: %v", event.ID(), err)
			continue
		}
		logger.Printf("✓ Stored event: %s (%s) for aggregate %s", 
			event.ID(), event.Type(), event.AggregateID())
	}

	// Get latest version
	version, err := store.LatestVersion(ctx)
	if err != nil {
		log.Printf("Failed to get latest version: %v", err)
	} else {
		logger.Printf("Latest version: %d", version.(cursor.IntegerCursor).Seq)
	}

	// Load all events
	logger.Println("\nLoading all events...")
	allEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		log.Printf("Failed to load events: %v", err)
	} else {
		logger.Printf("Loaded %d events", len(allEvents))
		for _, ev := range allEvents {
			logger.Printf("  - %d: %s (%s)", 
				ev.Version.(cursor.IntegerCursor).Seq, ev.Event.Type(), ev.Event.ID())
		}
	}

	// Load events by aggregate
	logger.Println("\nLoading events for user-alice...")
	aliceEvents, err := store.LoadByAggregate(ctx, "user-alice", cursor.IntegerCursor{Seq: 0})
	if err != nil {
		log.Printf("Failed to load events for user-alice: %v", err)
	} else {
		logger.Printf("Loaded %d events for user-alice", len(aliceEvents))
		for _, ev := range aliceEvents {
			logger.Printf("  - %s: %s", ev.Event.Type(), ev.Event.ID())
		}
	}

	// Demonstrate batch operations
	logger.Println("\nDemonstrating batch operations...")
	batchEvents := []synckit.EventWithVersion{
		{
			Event: NewUserEvent("batch-1", "UserLoggedIn", "user-alice", map[string]interface{}{
				"login_time": time.Now().Unix(),
				"ip_address": "192.168.1.100",
			}),
			Version: cursor.IntegerCursor{Seq: 0},
		},
		{
			Event: NewUserEvent("batch-2", "UserLoggedIn", "user-bob", map[string]interface{}{
				"login_time": time.Now().Unix(),
				"ip_address": "192.168.1.101",
			}),
			Version: cursor.IntegerCursor{Seq: 0},
		},
	}

	err = store.StoreBatch(ctx, batchEvents)
	if err != nil {
		log.Printf("Failed to store batch: %v", err)
	} else {
		logger.Printf("✓ Stored batch of %d events", len(batchEvents))
	}

	// Show database statistics
	stats := store.Stats()
	logger.Printf("\nDatabase connection statistics:")
	logger.Printf("  Open connections: %d", stats.OpenConnections)
	logger.Printf("  In use: %d", stats.InUse)
	logger.Printf("  Idle: %d", stats.Idle)
	logger.Printf("  Wait count: %d", stats.WaitCount)
	logger.Printf("  Wait duration: %v", stats.WaitDuration)
}

func demonstrateRealtimeNotifications(ctx context.Context, store *postgres.RealtimeEventStore, logger *log.Logger) {
	logger.Println("\n=== Real-time LISTEN/NOTIFY Demonstrations ===")

	// Create a context with timeout for the demonstration
	demoCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Channels to track notifications
	streamNotifications := make(chan postgres.NotificationPayload, 10)
	globalNotifications := make(chan postgres.NotificationPayload, 10)
	typeNotifications := make(chan postgres.NotificationPayload, 10)

	// Subscribe to events for a specific user stream
	logger.Println("Subscribing to events for user-charlie...")
	err := store.SubscribeToStream(demoCtx, "user-charlie", func(payload postgres.NotificationPayload) error {
		logger.Printf("📢 Stream notification: %s (%s) for %s at version %d", 
			payload.EventType, payload.ID, payload.AggregateID, payload.Version)
		streamNotifications <- payload
		return nil
	})
	if err != nil {
		log.Printf("Failed to subscribe to stream: %v", err)
		return
	}

	// Subscribe to all events
	logger.Println("Subscribing to all events...")
	err = store.SubscribeToAll(demoCtx, func(payload postgres.NotificationPayload) error {
		logger.Printf("🌍 Global notification: %s (%s) for %s on stream %s", 
			payload.EventType, payload.ID, payload.AggregateID, payload.StreamName)
		globalNotifications <- payload
		return nil
	})
	if err != nil {
		log.Printf("Failed to subscribe to all events: %v", err)
		return
	}

	// Subscribe to events of a specific type
	logger.Println("Subscribing to UserLoggedOut events...")
	err = store.SubscribeToEventType(demoCtx, "UserLoggedOut", func(payload postgres.NotificationPayload) error {
		logger.Printf("🔓 Logout notification: %s (%s) for %s", 
			payload.EventType, payload.ID, payload.AggregateID)
		typeNotifications <- payload
		return nil
	})
	if err != nil {
		log.Printf("Failed to subscribe to event type: %v", err)
		return
	}

	// Give subscriptions time to establish
	logger.Println("Waiting for subscriptions to establish...")
	time.Sleep(2 * time.Second)

	// Check if listener is connected
	if store.IsListenerConnected() {
		logger.Println("✓ LISTEN/NOTIFY connection is active")
	} else {
		logger.Println("✗ LISTEN/NOTIFY connection is not active")
		return
	}

	// Show active subscriptions
	activeSubscriptions := store.GetActiveSubscriptions()
	logger.Printf("Active subscriptions: %v", activeSubscriptions)

	// Store some events to trigger notifications
	logger.Println("\nStoring events to demonstrate real-time notifications...")

	testEvents := []*UserEvent{
		NewUserEvent("rt-1", "UserCreated", "user-charlie", map[string]interface{}{
			"name":  "Charlie Brown",
			"email": "charlie@example.com",
		}),
		NewUserEvent("rt-2", "UserUpdated", "user-charlie", map[string]interface{}{
			"email": "charlie.brown@example.com",
		}),
		NewUserEvent("rt-3", "UserCreated", "user-diana", map[string]interface{}{
			"name":  "Diana Prince",
			"email": "diana@example.com",
		}),
		NewUserEvent("rt-4", "UserLoggedOut", "user-charlie", map[string]interface{}{
			"logout_time": time.Now().Unix(),
		}),
		NewUserEvent("rt-5", "UserLoggedOut", "user-diana", map[string]interface{}{
			"logout_time": time.Now().Unix(),
		}),
	}

	for i, event := range testEvents {
		logger.Printf("Storing event %d: %s (%s) for %s", 
			i+1, event.Type(), event.ID(), event.AggregateID())
		
		err := store.Store(demoCtx, event, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			log.Printf("Failed to store event %s: %v", event.ID(), err)
			continue
		}

		// Small delay between events to see notifications arrive
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for notifications and summarize
	logger.Println("\nWaiting for notifications to arrive...")
	time.Sleep(3 * time.Second)

	// Count received notifications
	streamCount := len(streamNotifications)
	globalCount := len(globalNotifications)
	typeCount := len(typeNotifications)

	logger.Printf("\nNotification Summary:")
	logger.Printf("  Stream notifications (user-charlie): %d", streamCount)
	logger.Printf("  Global notifications: %d", globalCount)
	logger.Printf("  Type notifications (UserLoggedOut): %d", typeCount)

	// Drain and display some notifications
	logger.Println("\nSample notifications received:")

	for i := 0; i < 3 && len(streamNotifications) > 0; i++ {
		notification := <-streamNotifications
		logger.Printf("  Stream: %s v%d at %v", 
			notification.EventType, notification.Version, notification.CreatedAt)
	}

	for i := 0; i < 3 && len(globalNotifications) > 0; i++ {
		notification := <-globalNotifications
		logger.Printf("  Global: %s v%d for %s", 
			notification.EventType, notification.Version, notification.AggregateID)
	}

	for len(typeNotifications) > 0 {
		notification := <-typeNotifications
		logger.Printf("  Type: %s v%d for %s", 
			notification.EventType, notification.Version, notification.AggregateID)
	}

	logger.Println("\n✓ Real-time notification demonstration completed!")
}

// maskPassword masks the password in a connection string for safe logging
func maskPassword(connStr string) string {
	// Simple password masking for demo purposes
	if len(connStr) > 50 {
		return connStr[:30] + "***masked***" + connStr[len(connStr)-10:]
	}
	return "***masked***"
}
