package http

import (
    "encoding/json"

    sync "github.com/c0deZ3R0/go-sync-kit"
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
)

// JSONEventWithVersion represents an event with version in JSON format
type JSONEventWithVersion struct {
    ID          string          `json:"id"`
    Type        string          `json:"type"`
    AggregateID string          `json:"aggregate_id"`
    Data        json.RawMessage `json:"data,omitempty"`
    Metadata    json.RawMessage `json:"metadata,omitempty"`
    Version     uint64          `json:"version"`
}

func toJSONEventWithVersion(ev sync.EventWithVersion) JSONEventWithVersion {
    je := JSONEventWithVersion{
        ID:          ev.Event.ID,
        Type:        ev.Event.Type,
        AggregateID: ev.Event.AggregateID,
    }

    // Convert version
    if iv, ok := ev.Version.(sqlite.IntegerVersion); ok {
        je.Version = uint64(iv)
    }

    // Marshal data and metadata if present
    if ev.Event.Data != nil {
        data, _ := json.Marshal(ev.Event.Data)
        je.Data = data
    }
    if ev.Event.Metadata != nil {
        meta, _ := json.Marshal(ev.Event.Metadata)
        je.Metadata = meta
    }

    return je
}

func fromJSONEventWithVersion(je JSONEventWithVersion) sync.EventWithVersion {
    ev := sync.EventWithVersion{
        Event: sync.Event{
            ID:          je.ID,
            Type:        je.Type,
            AggregateID: je.AggregateID,
        },
        Version: sqlite.IntegerVersion(je.Version),
    }

    // Unmarshal data and metadata if present
    if len(je.Data) > 0 {
        var data map[string]interface{}
        if err := json.Unmarshal(je.Data, &data); err == nil {
            ev.Event.Data = data
        }
    }
    if len(je.Metadata) > 0 {
        var meta map[string]interface{}
        if err := json.Unmarshal(je.Metadata, &meta); err == nil {
            ev.Event.Metadata = meta
        }
    }

    return ev
}
