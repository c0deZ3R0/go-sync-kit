// Package event provides concrete event types for the sync kit.
package event

import (
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// Event represents a concrete implementation of the Event interface.
// It provides a simple, ready-to-use event structure for most use cases.
type Event struct {
	// EventID is the unique identifier for this event
	EventID string `json:"id"`
	
	// EventType represents the event type (e.g., "UserCreated", "OrderUpdated")
	EventType string `json:"type"`
	
	// EventAggregateID is the ID of the aggregate this event belongs to
	EventAggregateID string `json:"aggregate_id"`
	
	// EventData contains the event payload as raw bytes
	EventData []byte `json:"data"`
	
	// EventMetadata contains additional event metadata
	EventMetadata map[string]interface{} `json:"metadata,omitempty"`
	
	// Timestamp records when the event was created
	Timestamp time.Time `json:"timestamp"`
	
	// Offset is set by the event store to track event ordering
	Offset int64 `json:"offset,omitempty"`
}

// Ensure Event implements the types.Event interface
var _ types.Event = (*Event)(nil)

// ID returns the unique identifier for this event.
func (e *Event) ID() string {
	return e.EventID
}

// Type returns the event type.
func (e *Event) Type() string {
	return e.EventType
}

// AggregateID returns the ID of the aggregate this event belongs to.
func (e *Event) AggregateID() string {
	return e.EventAggregateID
}

// Data returns the event payload.
func (e *Event) Data() interface{} {
	return e.EventData
}

// Metadata returns additional event metadata.
func (e *Event) Metadata() map[string]interface{} {
	if e.EventMetadata == nil {
		return make(map[string]interface{})
	}
	return e.EventMetadata
}

// New creates a new Event with the given parameters.
func New(id, eventType, aggregateID string, data []byte) *Event {
	return &Event{
		EventID:          id,
		EventType:        eventType,
		EventAggregateID: aggregateID,
		EventData:        data,
		Timestamp:        time.Now(),
		EventMetadata:    make(map[string]interface{}),
	}
}

// NewWithMetadata creates a new Event with the given parameters and metadata.
func NewWithMetadata(id, eventType, aggregateID string, data []byte, metadata map[string]interface{}) *Event {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	return &Event{
		EventID:          id,
		EventType:        eventType,
		EventAggregateID: aggregateID,
		EventData:        data,
		Timestamp:        time.Now(),
		EventMetadata:    metadata,
	}
}
