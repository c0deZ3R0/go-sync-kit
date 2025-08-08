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

// Required Event interface methods
func (e *CounterEvent) ID() string { return e.id }
func (e *CounterEvent) Type() string { return e.eventType }
func (e *CounterEvent) Time() time.Time { return e.timestamp }
func (e *CounterEvent) AggregateID() string { return e.counterID }
func (e *CounterEvent) Metadata() map[string]interface{} { return e.metadata }
func (e *CounterEvent) Data() interface{} {
	return map[string]interface{}{
		"value":     e.value,
		"clientId":  e.clientID,
		"timestamp": e.timestamp,
	}
}

