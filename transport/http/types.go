package http

import (
    "encoding/json"

    sync "github.com/c0deZ3R0/go-sync-kit"
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
)

// JSONEventWithVersion represents an event with version in JSON format
type JSONEvent struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    AggregateID string                 `json:"aggregate_id"`
    Data        map[string]interface{} `json:"data,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type JSONEventWithVersion struct {
    Event   JSONEvent `json:"event"`
    Version string    `json:"version"`
}

func toJSONEventWithVersion(ev sync.EventWithVersion) JSONEventWithVersion {
    jsonEvent := JSONEvent{
        ID:          ev.Event.ID(),
        Type:        ev.Event.Type(),
        AggregateID: ev.Event.AggregateID(),
        Data:        ev.Event.Data(),
        Metadata:    ev.Event.Metadata(),
    }

    // Convert version to string
    version := "0"
    if iv, ok := ev.Version.(sqlite.IntegerVersion); ok {
        version = strconv.FormatInt(int64(iv), 10)
    }

    return JSONEventWithVersion{
        Event:   jsonEvent,
        Version: version,
    }
}

func fromJSONEventWithVersion(je JSONEventWithVersion) sync.EventWithVersion {
    ev := &SimpleEvent{
        IDValue:          je.Event.ID,
        TypeValue:        je.Event.Type,
        AggregateIDValue: je.Event.AggregateID,
        DataValue:        je.Event.Data,
        MetadataValue:    je.Event.Metadata,
    }

    // Parse version as integer
    var version sqlite.IntegerVersion
    if v, err := strconv.ParseInt(je.Version, 10, 64); err == nil {
        version = sqlite.IntegerVersion(v)
    }

    return sync.EventWithVersion{
        Event:   ev,
        Version: version,
    }
}
