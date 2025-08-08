package client

import (
	"time"
)

// Event type constants
const (
	EventTypeCounterCreated    = "CounterCreated"
	EventTypeCounterIncremented = "CounterIncremented"
	EventTypeCounterDecremented = "CounterDecremented"
	EventTypeCounterReset      = "CounterReset"
)

// CounterEvent represents a counter-related event
type CounterEvent struct {
	id          string
	eventType   string
	counterID   string
	value       int
	timestamp   time.Time
	clientID    string
	metadata    map[string]interface{}
}

// ID returns the event's unique identifier
func (e *CounterEvent) ID() string {
	return e.id
}

// Type returns the event type
func (e *CounterEvent) Type() string {
	return e.eventType
}

// AggregateID returns the counter ID this event belongs to
func (e *CounterEvent) AggregateID() string {
	return e.counterID
}

// Data returns the event payload
func (e *CounterEvent) Data() interface{} {
	return map[string]interface{}{
		"value":     e.value,
		"timestamp": e.timestamp,
		"clientID":  e.clientID,
	}
}

// Metadata returns additional event metadata
func (e *CounterEvent) Metadata() map[string]interface{} {
	return e.metadata
}
