package client

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
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

	return &Client{
		id:        config.ID,
		store:     store,
		transport: transport,
		manager:   manager,
		logger:    config.Logger,
		counters:  make(map[string]int),
	}, nil
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
