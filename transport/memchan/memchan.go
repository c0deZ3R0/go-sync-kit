// Package memchan provides an in-memory channel-based transport implementation
// for the go-sync-kit. This is perfect for development, testing, and examples
// where real network communication is not needed.
package memchan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// MemChan implements the Transport interface using in-memory channels.
// It simulates real-time event streaming without any network overhead.
type MemChan struct {
	mu       sync.RWMutex
	events   []synckit.EventWithVersion // Stored events (acts as remote store)
	subs     []chan synckit.Event       // Active subscribers
	capacity int                        // Channel capacity for subscribers
	closed   bool
}

// Ensure MemChan implements the Transport interface
var _ synckit.Transport = (*MemChan)(nil)

// New creates a new in-memory channel transport with the specified channel capacity.
func New(capacity int) *MemChan {
	if capacity <= 0 {
		capacity = 16 // Default capacity
	}

	return &MemChan{
		events:   make([]synckit.EventWithVersion, 0),
		subs:     make([]chan synckit.Event, 0),
		capacity: capacity,
	}
}

// Push sends events to all subscribers and stores them for future Pull operations.
func (c *MemChan) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("transport is closed")
	}

	// Store events for Pull operations
	c.events = append(c.events, events...)

	// Notify all subscribers
	for _, ev := range events {
		for i, sub := range c.subs {
			select {
			case sub <- ev.Event:
				// Event sent successfully
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Subscriber channel is full, drop the event
				// This simulates real-world scenarios where slow subscribers
				// might miss events if their buffer is full
				// In production, you might want to log this or have retry logic
				_ = i // Just to avoid unused variable warning
			}
		}
	}

	return nil
}

// Pull retrieves events from the in-memory store since the given version.
func (c *MemChan) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok && !since.IsZero() {
		return nil, fmt.Errorf("incompatible version type: expected cursor.IntegerCursor")
	}

	var result []synckit.EventWithVersion

	// Find all events with version > since
	for _, ev := range c.events {
		if evCursor, ok := ev.Version.(cursor.IntegerCursor); ok {
			if evCursor.Seq > sinceCursor.Seq {
				result = append(result, ev)
			}
		}
	}

	return result, nil
}

// GetLatestVersion returns the highest version number available.
func (c *MemChan) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	if len(c.events) == 0 {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	// Find the highest version
	var maxVersion uint64 = 0
	for _, ev := range c.events {
		if evCursor, ok := ev.Version.(cursor.IntegerCursor); ok {
			if evCursor.Seq > maxVersion {
				maxVersion = evCursor.Seq
			}
		}
	}

	return cursor.IntegerCursor{Seq: maxVersion}, nil
}

// Subscribe sets up real-time event streaming to a handler function.
// The handler will be called for each new event that gets pushed.
func (c *MemChan) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Create subscriber channel
	ch := make(chan synckit.Event, c.capacity)

	c.mu.Lock()
	c.subs = append(c.subs, ch)
	c.mu.Unlock()

	// Start subscriber goroutine
	go func() {
		defer func() {
			// Clean up: remove channel from subscribers list
			c.mu.Lock()
			for i, sub := range c.subs {
				if sub == ch {
					c.subs = append(c.subs[:i], c.subs[i+1:]...)
					close(ch)
					break
				}
			}
			c.mu.Unlock()
		}()

		// Batch events to simulate real-world batching behavior
		ticker := time.NewTicker(50 * time.Millisecond) // Batch every 50ms
		defer ticker.Stop()

		var batch []synckit.EventWithVersion

		for {
			select {
			case <-ctx.Done():
				// Process final batch before exiting
				if len(batch) > 0 {
					handler(batch)
				}
				return

			case ev, ok := <-ch:
				if !ok {
					// Channel closed, process final batch and exit
					if len(batch) > 0 {
						handler(batch)
					}
					return
				}

				// Add to batch
				// We need to create a version for this event since it comes from the channel
				// In real scenarios, the version would be preserved through the transport
				eventWithVersion := synckit.EventWithVersion{
					Event:   ev,
					Version: cursor.IntegerCursor{Seq: uint64(time.Now().UnixNano())}, // Simple version assignment
				}
				batch = append(batch, eventWithVersion)

			case <-ticker.C:
				// Send batch if we have events
				if len(batch) > 0 {
					if err := handler(batch); err != nil {
						// In a real transport, you might want to handle errors differently
						// For now, we just log and continue
						fmt.Printf("Handler error: %v\n", err)
					}
					batch = nil // Clear batch
				}
			}
		}
	}()

	return nil
}

// Close shuts down the transport and cleans up resources.
func (c *MemChan) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Close all subscriber channels
	for _, ch := range c.subs {
		close(ch)
	}

	// Clear data
	c.events = nil
	c.subs = nil

	return nil
}

// Stats returns statistics about the in-memory transport.
// This method is not part of the Transport interface but useful for monitoring.
func (c *MemChan) Stats() MemChanStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return MemChanStats{
		TotalEvents:        len(c.events),
		ActiveSubscribers:  len(c.subs),
		ChannelCapacity:    c.capacity,
		Closed:             c.closed,
	}
}

// MemChanStats contains statistics about the memory transport.
type MemChanStats struct {
	TotalEvents       int  // Total number of events stored
	ActiveSubscribers int  // Number of active subscribers
	ChannelCapacity   int  // Channel capacity for subscribers
	Closed            bool // Whether the transport is closed
}

// GetEvents returns all stored events (for testing/debugging purposes).
func (c *MemChan) GetEvents() []synckit.EventWithVersion {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	events := make([]synckit.EventWithVersion, len(c.events))
	copy(events, c.events)
	return events
}

// ClearEvents removes all stored events (for testing purposes).
func (c *MemChan) ClearEvents() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = nil
}

// CreatePair creates a pair of connected MemChan transports for bidirectional testing.
// This simulates two nodes communicating with each other.
func CreatePair(capacity int) (*MemChan, *MemChan) {
	transport1 := New(capacity)
	transport2 := New(capacity)

	// In a real scenario, you might want to create some form of 
	// bidirectional connection between them, but for simplicity,
	// we'll just return two independent transports
	return transport1, transport2
}

// NewHub creates a central hub that can coordinate multiple transports.
// This is useful for testing scenarios with multiple nodes.
type Hub struct {
	mu         sync.RWMutex
	transports map[string]*MemChan
	events     []synckit.EventWithVersion
}

// NewHub creates a new transport hub.
func NewHub() *Hub {
	return &Hub{
		transports: make(map[string]*MemChan),
		events:     make([]synckit.EventWithVersion, 0),
	}
}

// AddTransport adds a transport to the hub with the given name.
func (h *Hub) AddTransport(name string, transport *MemChan) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.transports[name] = transport
}

// RemoveTransport removes a transport from the hub.
func (h *Hub) RemoveTransport(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.transports, name)
}

// Broadcast sends an event to all transports in the hub.
func (h *Hub) Broadcast(ctx context.Context, events []synckit.EventWithVersion) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.events = append(h.events, events...)

	for name, transport := range h.transports {
		if err := transport.Push(ctx, events); err != nil {
			return fmt.Errorf("failed to broadcast to transport %s: %w", name, err)
		}
	}

	return nil
}

// GetHubEvents returns all events that have passed through the hub.
func (h *Hub) GetHubEvents() []synckit.EventWithVersion {
	h.mu.RLock()
	defer h.mu.RUnlock()

	events := make([]synckit.EventWithVersion, len(h.events))
	copy(events, h.events)
	return events
}