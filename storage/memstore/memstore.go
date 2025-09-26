// Package memstore provides an in-memory implementation of the go-sync-kit EventStore.
// This is perfect for development, testing, and examples where no persistence is needed.
package memstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// MemStore implements the EventStore interface with in-memory storage.
// It's thread-safe and perfect for development, testing, and examples.
type MemStore struct {
	mu       sync.RWMutex
	events   []synckit.EventWithVersion // All events in order
	streams  map[string][]int           // Stream ID -> indices in events slice
	nextSeq  uint64                     // Next sequence number
	closed   bool
}

// Ensure MemStore implements the EventStore interface
var _ synckit.EventStore = (*MemStore)(nil)

// New creates a new in-memory event store.
func New() *MemStore {
	return &MemStore{
		events:  make([]synckit.EventWithVersion, 0),
		streams: make(map[string][]int),
		nextSeq: 1, // Start at 1, like most databases
	}
}

// Store saves an event to the in-memory store.
// The version parameter is ignored as the store auto-generates sequential versions.
func (s *MemStore) Store(ctx context.Context, evt synckit.Event, version synckit.Version) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	// Create a copy of the event to avoid any mutations
	eventCopy := &event.Event{
		EventID:          evt.ID(),
		EventType:        evt.Type(),
		EventAggregateID: evt.AggregateID(),
		Offset:           int64(s.nextSeq),
		EventMetadata:    make(map[string]interface{}),
	}

	// Copy data
	if data := evt.Data(); data != nil {
		if bytes, ok := data.([]byte); ok {
			eventCopy.EventData = make([]byte, len(bytes))
			copy(eventCopy.EventData, bytes)
		} else {
			// Handle other data types by storing as-is
			eventCopy.EventData = []byte(fmt.Sprintf("%v", data))
		}
	}

	// Copy metadata
	if metadata := evt.Metadata(); metadata != nil {
		for k, v := range metadata {
			eventCopy.EventMetadata[k] = v
		}
	}

	// Create version for this event
	eventVersion := cursor.IntegerCursor{Seq: s.nextSeq}

	// Create EventWithVersion
	evWithVersion := synckit.EventWithVersion{
		Event:   eventCopy,
		Version: eventVersion,
	}

	// Add to events slice
	eventIndex := len(s.events)
	s.events = append(s.events, evWithVersion)

	// Add to stream index
	streamID := evt.AggregateID()
	s.streams[streamID] = append(s.streams[streamID], eventIndex)

	// Increment sequence
	s.nextSeq++

	return nil
}

// Load retrieves all events since a given version.
func (s *MemStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok && !since.IsZero() {
		return nil, fmt.Errorf("incompatible version type: expected cursor.IntegerCursor")
	}

	var result []synckit.EventWithVersion

	// Find all events with version > since
	for _, ev := range s.events {
		if evCursor, ok := ev.Version.(cursor.IntegerCursor); ok {
			if evCursor.Seq > sinceCursor.Seq {
				result = append(result, ev)
			}
		}
	}

	return result, nil
}

// LoadByAggregate retrieves events for a specific aggregate since a given version.
func (s *MemStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok && !since.IsZero() {
		return nil, fmt.Errorf("incompatible version type: expected cursor.IntegerCursor")
	}

	var result []synckit.EventWithVersion

	// Get event indices for this stream
	indices, exists := s.streams[aggregateID]
	if !exists {
		return result, nil // Empty result for non-existent stream
	}

	// Filter events by version
	for _, idx := range indices {
		if idx < len(s.events) {
			ev := s.events[idx]
			if evCursor, ok := ev.Version.(cursor.IntegerCursor); ok {
				if evCursor.Seq > sinceCursor.Seq {
					result = append(result, ev)
				}
			}
		}
	}

	return result, nil
}

// LatestVersion returns the highest version number in the store.
func (s *MemStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	if len(s.events) == 0 {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	// Return the next sequence - 1 (since nextSeq is always one ahead)
	return cursor.IntegerCursor{Seq: s.nextSeq - 1}, nil
}

// ParseVersion converts a string representation into a cursor.IntegerCursor.
func (s *MemStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	if versionStr == "" || versionStr == "0" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	// Simple parsing - delegate to cursor package if it has utilities
	// For now, we'll do basic parsing
	var seq uint64
	if _, err := fmt.Sscanf(versionStr, "%d", &seq); err != nil {
		return nil, fmt.Errorf("invalid version string '%s': %w", versionStr, err)
	}

	return cursor.IntegerCursor{Seq: seq}, nil
}

// Close closes the in-memory store and clears all data.
func (s *MemStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.events = nil
	s.streams = nil

	return nil
}

// StoreBatch stores multiple events in the store.
// This is more efficient than calling Store multiple times as it acquires the lock once.
func (s *MemStore) StoreBatch(ctx context.Context, events []synckit.EventWithVersion) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	// Store each event
	for _, evWithVersion := range events {
		evt := evWithVersion.Event

		// Create a copy of the event to avoid any mutations
		eventCopy := &event.Event{
			EventID:          evt.ID(),
			EventType:        evt.Type(),
			EventAggregateID: evt.AggregateID(),
			Offset:           int64(s.nextSeq),
			EventMetadata:    make(map[string]interface{}),
		}

		// Copy data
		if data := evt.Data(); data != nil {
			if bytes, ok := data.([]byte); ok {
				eventCopy.EventData = make([]byte, len(bytes))
				copy(eventCopy.EventData, bytes)
			} else {
				eventCopy.EventData = []byte(fmt.Sprintf("%v", data))
			}
		}

		// Copy metadata
		if metadata := evt.Metadata(); metadata != nil {
			for k, v := range metadata {
				eventCopy.EventMetadata[k] = v
			}
		}

		// Create version for this event
		eventVersion := cursor.IntegerCursor{Seq: s.nextSeq}

		// Create new EventWithVersion with our version
		newEvWithVersion := synckit.EventWithVersion{
			Event:   eventCopy,
			Version: eventVersion,
		}

		// Add to events slice
		eventIndex := len(s.events)
		s.events = append(s.events, newEvWithVersion)

		// Add to stream index
		streamID := evt.AggregateID()
		s.streams[streamID] = append(s.streams[streamID], eventIndex)

		// Increment sequence
		s.nextSeq++
	}

	return nil
}

// Stats returns statistics about the in-memory store.
// This method is not part of the EventStore interface but useful for monitoring.
func (s *MemStore) Stats() MemStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return MemStoreStats{
		TotalEvents:  len(s.events),
		TotalStreams: len(s.streams),
		NextSequence: s.nextSeq,
		Closed:       s.closed,
	}
}

// MemStoreStats contains statistics about the memory store.
type MemStoreStats struct {
	TotalEvents  int    // Total number of events stored
	TotalStreams int    // Total number of unique streams
	NextSequence uint64 // Next sequence number to be assigned
	Closed       bool   // Whether the store is closed
}