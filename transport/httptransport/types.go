package httptransport

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/c0deZ3R0/go-sync-kit/synckit"
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

// SimpleEvent is a simple implementation of synckit.Event for HTTP transport
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
    return JSONEventWithVersion{
        Event:   toJSONEvent(ev.Event),
        Version: ev.Version.String(),
    }
}

// fromJSONEvent converts JSONEvent to a concrete Event implementation
func fromJSONEvent(je JSONEvent) synckit.Event {
    return &SimpleEvent{
        IDValue:          je.ID,
        TypeValue:        je.Type,
        AggregateIDValue: je.AggregateID,
        DataValue:        je.Data,
        MetadataValue:    je.Metadata,
    }
}

// fromJSONEventWithVersion converts JSONEventWithVersion back to synckit.EventWithVersion
// It uses the SyncHandler's version parser to parse the version string
func fromJSONEventWithVersion(ctx context.Context, parser VersionParser, jev JSONEventWithVersion) (synckit.EventWithVersion, error) {
    version, err := parser(ctx, jev.Version)
    if err != nil {
        return synckit.EventWithVersion{}, fmt.Errorf("invalid version: %w", err)
    }

    event := &SimpleEvent{
        IDValue:          jev.Event.ID,
        TypeValue:        jev.Event.Type,
        AggregateIDValue: jev.Event.AggregateID,
        DataValue:        jev.Event.Data,
        MetadataValue:    jev.Event.Metadata,
    }

    return synckit.EventWithVersion{
        Event:   event,
        Version: version,
    }, nil
}

// Helper functions for HTTP responses
func respondWithError(w http.ResponseWriter, code int, message string) {
    respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
    response, err := json.Marshal(payload)
    if err != nil {
        // Fallback if payload marshaling fails
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte(`{"error": "failed to marshal response"}`)) 
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    w.Write(response)
}
