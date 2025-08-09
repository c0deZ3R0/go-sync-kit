package client

import (
	"encoding/json"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/version"
)

// Event types
const (
	EventTypeCounterCreated     = "counter_created"
	EventTypeCounterIncremented = "counter_incremented"
	EventTypeCounterDecremented = "counter_decremented"
	EventTypeCounterReset       = "counter_reset"
)

// CounterEvent represents a counter operation event
type CounterEvent struct {
	id        string
	eventType string
	counterID string
	value     int
	timestamp time.Time
	clientID  string
	version   *version.VectorClock
	metadata  map[string]interface{}
}

// ID returns the event ID
func (e *CounterEvent) ID() string {
	return e.id
}

// Type returns the event type
func (e *CounterEvent) Type() string {
	return e.eventType
}

// AggregateID returns the counter ID
func (e *CounterEvent) AggregateID() string {
	return e.counterID
}

// Metadata returns event metadata
func (e *CounterEvent) Metadata() map[string]interface{} {
	return e.metadata
}

// Data returns the event data
func (e *CounterEvent) Data() interface{} {
	data := map[string]interface{}{
		"value":     e.value,
		"timestamp": e.timestamp.Format(time.RFC3339Nano),
		"clientId":  e.clientID,
	}

	// If vector clock exists, persist the entire clock as a plain map to ensure portability
	if e.version != nil {
		// Copy clocks into a map[string]uint64 for stable JSON encoding/decoding
		clocks := make(map[string]uint64)
		for k, v := range e.version.GetAllClocks() {
			clocks[k] = v
		}
		data["version"] = clocks
	}

	return data
}

// Version returns the event version
func (e *CounterEvent) Version() *version.VectorClock {
	return e.version
}

// SetVersion sets the event version
func (e *CounterEvent) SetVersion(v *version.VectorClock) {
	e.version = v
}

// MarshalJSON implements json.Marshaler
func (e *CounterEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        e.id,
		"type":      e.eventType,
		"counterId": e.counterID,
		"value":     e.value,
		"timestamp": e.timestamp,
		"clientId":  e.clientID,
		"metadata":  e.metadata,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (e *CounterEvent) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	e.id = m["id"].(string)
	e.eventType = m["type"].(string)
	e.counterID = m["counterId"].(string)
	e.value = int(m["value"].(float64))

	if ts, ok := m["timestamp"].(string); ok {
		timestamp, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return err
		}
		e.timestamp = timestamp
	}

	e.clientID = m["clientId"].(string)

	if v, ok := m["version"].(map[string]interface{}); ok {
		// Convert map values to uint64
		clocks := make(map[string]uint64)
		for k, val := range v {
			if fval, ok := val.(float64); ok {
				clocks[k] = uint64(fval)
			}
		}
		e.version = version.NewVectorClockFromMap(clocks)
	}

	if meta, ok := m["metadata"].(map[string]interface{}); ok {
		e.metadata = meta
	}

	return nil
}
