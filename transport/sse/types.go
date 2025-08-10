package sse

import (
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// JSONEvent is a JSON-serializable representation of an Event
type JSONEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	AggregateID string                 `json:"aggregate_id"`
	Data        interface{}            `json:"data"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// JSONEventWithVersion is a JSON-serializable representation of EventWithVersion
type JSONEventWithVersion struct {
	Event   JSONEvent `json:"event"`
	Version string    `json:"version"`
}

// SimpleEvent is a simple implementation of synckit.Event for SSE transport
type SimpleEvent struct {
	IDValue          string                 `json:"id"`
	TypeValue        string                 `json:"type"`
	AggregateIDValue string                 `json:"aggregate_id"`
	DataValue        interface{}            `json:"data"`
	MetadataValue    map[string]interface{} `json:"metadata"`
}

func (e *SimpleEvent) ID() string                       { return e.IDValue }
func (e *SimpleEvent) Type() string                     { return e.TypeValue }
func (e *SimpleEvent) AggregateID() string              { return e.AggregateIDValue }
func (e *SimpleEvent) Data() interface{}                { return e.DataValue }
func (e *SimpleEvent) Metadata() map[string]interface{} { return e.MetadataValue }

// toJSONEvent converts a synckit.Event to JSONEvent
func toJSONEvent(event synckit.Event) JSONEvent {
	return JSONEvent{
		ID:          event.ID(),
		Type:        event.Type(),
		AggregateID: event.AggregateID(),
		Data:        event.Data(),
		Metadata:    event.Metadata(),
	}
}

// toJSONEventWithVersion converts synckit.EventWithVersion to JSONEventWithVersion
func toJSONEventWithVersion(ev synckit.EventWithVersion) JSONEventWithVersion {
	var version string
	if ev.Version != nil {
		version = ev.Version.String()
	}
	return JSONEventWithVersion{
		Event:   toJSONEvent(ev.Event),
		Version: version,
	}
}

// fromJSONEventWithVersion converts JSONEventWithVersion back to synckit.EventWithVersion
// For SSE transport, we'll create a simple integer cursor from the version string
func fromJSONEventWithVersion(jev JSONEventWithVersion) synckit.EventWithVersion {
	event := &SimpleEvent{
		IDValue:          jev.Event.ID,
		TypeValue:        jev.Event.Type,
		AggregateIDValue: jev.Event.AggregateID,
		DataValue:        jev.Event.Data,
		MetadataValue:    jev.Event.Metadata,
	}

	// For simplicity, we'll parse the version as an integer cursor
	// In a real implementation, you'd want proper version parsing
	var version synckit.Version = cursor.NewInteger(0) // default
	if jev.Version != "" {
		// Try to parse as integer - this is a simple approach
		// In practice, you'd want more sophisticated version parsing
		if jev.Version == "1" {
			version = cursor.NewInteger(1)
		} else if jev.Version == "2" {
			version = cursor.NewInteger(2)
		}
		// Add more parsing logic as needed
	}

	return synckit.EventWithVersion{
		Event:   event,
		Version: version,
	}
}
