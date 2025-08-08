package main

import (
	"encoding/json"
	"time"
)

// EventType constants for counter events
const (
	EventTypeCounterCreated    = "CounterCreated"
	EventTypeCounterIncremented = "CounterIncremented"
	EventTypeCounterDecremented = "CounterDecremented"
	EventTypeCounterReset      = "CounterReset"
)

// CounterEvent implements the synckit.Event interface
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

// MarshalJSON implements json.Marshaler
func (e *CounterEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID          string                 `json:"id"`
		Type        string                 `json:"type"`
		CounterID   string                 `json:"counterId"`
		Value       int                    `json:"value"`
		Timestamp   time.Time              `json:"timestamp"`
		ClientID    string                 `json:"clientId"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}{
		ID:        e.id,
		Type:      e.eventType,
		CounterID: e.counterID,
		Value:     e.value,
		Timestamp: e.timestamp,
		ClientID:  e.clientID,
		Metadata:  e.metadata,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (e *CounterEvent) UnmarshalJSON(data []byte) error {
	var v struct {
		ID          string                 `json:"id"`
		Type        string                 `json:"type"`
		CounterID   string                 `json:"counterId"`
		Value       int                    `json:"value"`
		Timestamp   time.Time              `json:"timestamp"`
		ClientID    string                 `json:"clientId"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	e.id = v.ID
	e.eventType = v.Type
	e.counterID = v.CounterID
	e.value = v.Value
	e.timestamp = v.Timestamp
	e.clientID = v.ClientID
	e.metadata = v.Metadata

	return nil
}
