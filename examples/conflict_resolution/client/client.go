package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// Client represents a counter client that can work offline and sync with server
type Client struct {
	id        string
	store     synckit.EventStore
	transport synckit.Transport
	manager   synckit.SyncManager
	logger    *log.Logger
	mu        sync.RWMutex
	counters  map[string]int // Local cache of counter values
}

// Config holds client configuration
type Config struct {
	ID         string
	ServerURL  string
	DBPath     string
	Logger     *log.Logger
	SyncPeriod time.Duration
}

// initializeState loads the initial state from the store
func (c *Client) initializeState(ctx context.Context) error {

// Load all events by passing nil version to get everything
since := cursor.IntegerCursor{Seq: 0}
events, err := c.store.Load(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}

	c.logger.Printf("Loaded %d events from store", len(events))

// Sort events by timestamp
sort.Slice(events, func(i, j int) bool {
	ei, ok := events[i].Event.(*CounterEvent)
	if !ok {
		return false
	}
	ej, ok := events[j].Event.(*CounterEvent)
	if !ok {
		return true
	}
	return ei.timestamp.Before(ej.timestamp)
})

// Apply events to local state
for _, ev := range events {
	if err := c.applyEvent(ev.Event); err != nil {
		c.logger.Printf("Warning: Failed to apply event %s: %v", ev.Event.ID(), err)
	}
}

c.logger.Printf("State reloaded with %d events", len(events))

	return nil
}

// applyEvent updates the local cache based on an event
func (c *Client) applyEvent(event synckit.Event) error {
	storedEvent, ok := event.(*sqlite.StoredEvent)
	if !ok {
		counterEvent, ok := event.(*CounterEvent)
		if !ok {
			return fmt.Errorf("invalid event type: %T", event)
		}
		return c.applyCounterEvent(counterEvent)
	}

	// Parse the event from stored data
	data := storedEvent.Data()
	
	// Handle JSON RawMessage
	if rawJSON, ok := data.(json.RawMessage); ok {
		var eventData struct {
			Value     int       `json:"value"`
			Timestamp time.Time `json:"timestamp"`
			ClientID  string    `json:"clientId"`
		}
		if err := json.Unmarshal(rawJSON, &eventData); err != nil {
			return fmt.Errorf("failed to decode event data: %w", err)
		}
		
		counterEvent := &CounterEvent{
			id:        storedEvent.ID(),
			eventType: storedEvent.Type(),
			counterID: storedEvent.AggregateID(),
			value:     eventData.Value,
			timestamp: eventData.Timestamp,
			clientID:  eventData.ClientID,
			metadata:  storedEvent.Metadata(),
		}
		return c.applyCounterEvent(counterEvent)
	}

	// Handle map[string]interface{}
	if dataMap, ok := data.(map[string]interface{}); ok {
		value, _ := dataMap["value"].(float64)
		timestampStr, _ := dataMap["timestamp"].(string)
		timestamp, _ := time.Parse(time.RFC3339Nano, timestampStr)
		clientID, _ := dataMap["clientId"].(string)

		counterEvent := &CounterEvent{
			id:        storedEvent.ID(),
			eventType: storedEvent.Type(),
			counterID: storedEvent.AggregateID(),
			value:     int(value),
			timestamp: timestamp,
			clientID:  clientID,
			metadata:  storedEvent.Metadata(),
		}
		return c.applyCounterEvent(counterEvent)
	}

	return fmt.Errorf("unsupported event data type: %T", data)
}

func (c *Client) applyCounterEvent(counterEvent *CounterEvent) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	switch counterEvent.Type() {
	case EventTypeCounterCreated:
		if _, exists := c.counters[counterEvent.counterID]; exists {
			return fmt.Errorf("counter %s already exists", counterEvent.counterID)
		}
		c.counters[counterEvent.counterID] = counterEvent.value

	case EventTypeCounterIncremented:
		if _, exists := c.counters[counterEvent.counterID]; !exists {
			return fmt.Errorf("counter %s does not exist", counterEvent.counterID)
		}
		c.counters[counterEvent.counterID] += counterEvent.value

	case EventTypeCounterDecremented:
		if _, exists := c.counters[counterEvent.counterID]; !exists {
			return fmt.Errorf("counter %s does not exist", counterEvent.counterID)
		}
		c.counters[counterEvent.counterID] -= counterEvent.value

	case EventTypeCounterReset:
		c.counters[counterEvent.counterID] = counterEvent.value
	}

	return nil
}

// handleSyncResults processes events received during sync
func (c *Client) handleSyncResults(result *synckit.SyncResult) {
	if result == nil {
		return
	}

	// Log sync result details
	c.logger.Printf("Sync completed: pushed %d, pulled %d, conflicts %d",
		result.EventsPushed, result.EventsPulled, result.ConflictsResolved)

	// If we pushed or pulled events, or resolved conflicts, reload state
	if result.EventsPushed > 0 || result.EventsPulled > 0 || result.ConflictsResolved > 0 {
		c.logger.Printf("Reloading state due to %d pushed, %d pulled events and %d resolved conflicts",
			result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
		
		// Reset counters map and reload all events
		c.mu.Lock()
		c.counters = make(map[string]int)
		c.mu.Unlock()

		// Reload complete state
		ctx := context.Background()
		since := cursor.IntegerCursor{Seq: 0}
		events, err := c.store.Load(ctx, since)
		if err != nil {
			c.logger.Printf("Failed to load events: %v", err)
			return
		}

		c.logger.Printf("Reloading state with %d events from store", len(events))

		sort.Slice(events, func(i, j int) bool {
			ei, oki := events[i].Event.(*CounterEvent)
			ej, okj := events[j].Event.(*CounterEvent)
			// If either event is not a CounterEvent, maintain stable order
			if !oki || !okj {
				return i < j
			}
			// First compare by client ID
			if ei.clientID != ej.clientID {
				return ei.clientID < ej.clientID
			}
			// If same client, compare by timestamp
			return ei.timestamp.Before(ej.timestamp)
		})

// Apply events in sorted order
		for _, ev := range events {
			if counterEvent, ok := ev.Event.(*CounterEvent); ok {
				c.logger.Printf("Applying event: id=%s type=%s value=%d clientId=%s",
					counterEvent.id, counterEvent.eventType, counterEvent.value, counterEvent.clientID)
			}
			if err := c.applyEvent(ev.Event); err != nil {
				c.logger.Printf("Warning: Failed to apply event %s: %v", ev.Event.ID(), err)
			}
		}

		// Log final state
		c.logger.Printf("Final state after applying %d events:", len(events))
		for id, value := range c.counters {
			c.logger.Printf("  Counter %s = %d", id, value)
		}
	}
}

// New creates a new counter client
func New(config Config) (*Client, error) {
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, fmt.Sprintf("[Client %s] ", config.ID), log.LstdFlags)
	}

	// Ensure database directory exists
	if err := os.MkdirAll(filepath.Dir(config.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create SQLite store
	storeConfig := &sqlite.Config{
		DataSourceName: fmt.Sprintf("file:%s", config.DBPath),
		EnableWAL:     true,
		Logger:        config.Logger,
	}

	store, err := sqlite.New(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite store: %w", err)
	}

	// Create HTTP transport
	transport := httptransport.NewTransport(
		config.ServerURL+"/sync",
		&http.Client{Timeout: 10 * time.Second},
		nil,
	)

	// Create sync options
	opts := &synckit.SyncOptions{
		ConflictResolver: NewCounterConflictResolver(),
		SyncInterval:    config.SyncPeriod,
		RetryConfig: &synckit.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
		},
	}

	// Create sync manager
	manager := synckit.NewSyncManager(store, transport, opts)

	// Create client
	client := &Client{
		id:        config.ID,
		store:     store,
		transport: transport,
		manager:   manager,
		logger:    config.Logger,
		counters:  make(map[string]int),
	}

	// Initialize state from store
	if err := client.initializeState(context.Background()); err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	// Subscribe to sync results
	manager.Subscribe(client.handleSyncResults)

	return client, nil
}

// CreateCounter creates a new counter
func (c *Client) CreateCounter(ctx context.Context, counterID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if counter already exists
	if _, exists := c.counters[counterID]; exists {
		return fmt.Errorf("counter %s already exists", counterID)
	}

	// Create counter event
	event := &CounterEvent{
		id:        fmt.Sprintf("%s-%s-%d", c.id, counterID, time.Now().UnixNano()),
		eventType: EventTypeCounterCreated,
		counterID: counterID,
		value:     0,
		timestamp: time.Now(),
		clientID:  c.id,
	}

	// Store event
	if err := c.store.Store(ctx, event, nil); err != nil {
		return fmt.Errorf("failed to store counter creation event: %w", err)
	}

	// Update local cache
	c.counters[counterID] = 0
	return nil
}

// IncrementCounter increments a counter by a given value
func (c *Client) IncrementCounter(ctx context.Context, counterID string, value int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if counter exists
	if _, exists := c.counters[counterID]; !exists {
		return fmt.Errorf("counter %s does not exist", counterID)
	}

	// Create increment event
	event := &CounterEvent{
		id:        fmt.Sprintf("%s-%s-%d", c.id, counterID, time.Now().UnixNano()),
		eventType: EventTypeCounterIncremented,
		counterID: counterID,
		value:     value,
		timestamp: time.Now(),
		clientID:  c.id,
	}

	// Store event
	if err := c.store.Store(ctx, event, nil); err != nil {
		return fmt.Errorf("failed to store increment event: %w", err)
	}

	// Update local cache
	c.counters[counterID] += value
	return nil
}

// GetCounter returns the current value of a counter
func (c *Client) GetCounter(counterID string) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, exists := c.counters[counterID]
	if !exists {
		return 0, fmt.Errorf("counter %s does not exist", counterID)
	}

	return value, nil
}

// ListCounters returns all counter IDs and their values
func (c *Client) ListCounters() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid external modifications
	counters := make(map[string]int, len(c.counters))
	for id, value := range c.counters {
		counters[id] = value
	}

	return counters
}

// StartSync starts automatic synchronization
func (c *Client) StartSync(ctx context.Context) error {
	c.logger.Printf("Starting automatic synchronization")
	return c.manager.StartAutoSync(ctx)
}

// StopSync stops automatic synchronization
func (c *Client) StopSync() error {
	c.logger.Printf("Stopping automatic synchronization")
	return c.manager.StopAutoSync()
}

// Sync performs a manual synchronization
func (c *Client) Sync(ctx context.Context) error {
	c.logger.Printf("Performing manual synchronization")
	result, err := c.manager.Sync(ctx)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	c.logger.Printf("Sync completed: pushed %d, pulled %d, resolved %d conflicts",
		result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
	return nil
}

// Close closes the client and releases resources
func (c *Client) Close() error {
	c.logger.Printf("Closing client")
	if err := c.manager.Close(); err != nil {
		return fmt.Errorf("failed to close sync manager: %w", err)
	}
	if err := c.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %w", err)
	}
	return nil
}
