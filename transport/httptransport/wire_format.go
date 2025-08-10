package httptransport

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/codec"
)

// WireEvent represents an event optimized for wire transmission with codec support.
// It extends JSONEvent with optional codec-aware encoding for the Data field.
type WireEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	AggregateID string                 `json:"aggregate_id"`
	Data        json.RawMessage        `json:"data"`         // Raw JSON for codec-aware or fallback encoding
	DataKind    string                 `json:"data_kind,omitempty"` // Optional: codec identifier
	Metadata    map[string]interface{} `json:"metadata"`
}

// WireEventWithVersion pairs a WireEvent with version information.
type WireEventWithVersion struct {
	Event   WireEvent `json:"event"`
	Version string    `json:"version"`
}

// CodecAwareEncoder handles encoding/decoding of events using registered codecs.
type CodecAwareEncoder struct {
	registry *codec.Registry
	fallback bool // Whether to fall back to JSON encoding for unknown types
}

// NewCodecAwareEncoder creates a new codec-aware encoder.
// If registry is nil, it uses the default registry.
func NewCodecAwareEncoder(registry *codec.Registry, fallback bool) *CodecAwareEncoder {
	if registry == nil {
		registry = codec.DefaultRegistry
	}
	return &CodecAwareEncoder{
		registry: registry,
		fallback: fallback,
	}
}

// EncodeEvent converts a synckit.Event to WireEvent using codec if available.
func (e *CodecAwareEncoder) EncodeEvent(event synckit.Event) (WireEvent, error) {
	wireEvent := WireEvent{
		ID:          event.ID(),
		Type:        event.Type(),
		AggregateID: event.AggregateID(),
		Metadata:    event.Metadata(),
	}

	data := event.Data()
	
	// Try to determine codec kind from metadata
	var codecKind string
	if metadata := event.Metadata(); metadata != nil {
		if kind, ok := metadata["kind"].(string); ok {
			codecKind = kind
		} else if kind, ok := metadata["codec_kind"].(string); ok {
			codecKind = kind
		} else if kind, ok := metadata["data_kind"].(string); ok {
			codecKind = kind
		}
	}

	// If we have a codec kind, try to use the codec
	if codecKind != "" {
		if c, found := e.registry.Get(codecKind); found {
			encodedData, err := c.Encode(data)
			if err != nil {
				if !e.fallback {
					return wireEvent, fmt.Errorf("failed to encode data with codec %s: %w", codecKind, err)
				}
				// Fall back to JSON encoding
				encodedData, err = json.Marshal(data)
				if err != nil {
					return wireEvent, fmt.Errorf("failed to encode data with JSON fallback: %w", err)
				}
			} else {
				wireEvent.DataKind = codecKind
			}
			wireEvent.Data = encodedData
			return wireEvent, nil
		}
	}

	// Fallback to JSON encoding
	if e.fallback {
		encodedData, err := json.Marshal(data)
		if err != nil {
			return wireEvent, fmt.Errorf("failed to encode data with JSON fallback: %w", err)
		}
		wireEvent.Data = encodedData
		return wireEvent, nil
	}

	return wireEvent, fmt.Errorf("no codec found for kind %s and fallback disabled", codecKind)
}

// DecodeEvent converts a WireEvent back to a concrete Event implementation.
func (e *CodecAwareEncoder) DecodeEvent(wireEvent WireEvent) (synckit.Event, error) {
	var data interface{}
	
	// If DataKind is specified, try to use the codec
	if wireEvent.DataKind != "" {
		if c, found := e.registry.Get(wireEvent.DataKind); found {
			decodedData, err := c.Decode(wireEvent.Data)
			if err != nil {
				if !e.fallback {
					return nil, fmt.Errorf("failed to decode data with codec %s: %w", wireEvent.DataKind, err)
				}
				// Fall back to JSON decoding
				if err := json.Unmarshal(wireEvent.Data, &data); err != nil {
					return nil, fmt.Errorf("failed to decode data with JSON fallback: %w", err)
				}
			} else {
				data = decodedData
			}
		} else if !e.fallback {
			return nil, fmt.Errorf("codec %s not found and fallback disabled", wireEvent.DataKind)
		} else {
			// Codec not found, fall back to JSON
			if err := json.Unmarshal(wireEvent.Data, &data); err != nil {
				return nil, fmt.Errorf("failed to decode data with JSON fallback: %w", err)
			}
		}
	} else {
		// No codec specified, use JSON decoding
		if err := json.Unmarshal(wireEvent.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to decode data with JSON: %w", err)
		}
	}

	return &SimpleEvent{
		IDValue:          wireEvent.ID,
		TypeValue:        wireEvent.Type,
		AggregateIDValue: wireEvent.AggregateID,
		DataValue:        data,
		MetadataValue:    wireEvent.Metadata,
	}, nil
}

// EncodeEventWithVersion converts synckit.EventWithVersion to WireEventWithVersion.
func (e *CodecAwareEncoder) EncodeEventWithVersion(ev synckit.EventWithVersion) (WireEventWithVersion, error) {
	wireEvent, err := e.EncodeEvent(ev.Event)
	if err != nil {
		return WireEventWithVersion{}, err
	}

	var version string
	if ev.Version != nil {
		version = ev.Version.String()
	}

	return WireEventWithVersion{
		Event:   wireEvent,
		Version: version,
	}, nil
}

// DecodeEventWithVersion converts WireEventWithVersion back to synckit.EventWithVersion.
func (e *CodecAwareEncoder) DecodeEventWithVersion(ctx context.Context, parser VersionParser, wireEv WireEventWithVersion) (synckit.EventWithVersion, error) {
	event, err := e.DecodeEvent(wireEv.Event)
	if err != nil {
		return synckit.EventWithVersion{}, fmt.Errorf("failed to decode event: %w", err)
	}

	version, err := parser(ctx, wireEv.Version)
	if err != nil {
		return synckit.EventWithVersion{}, fmt.Errorf("failed to parse version: %w", err)
	}

	return synckit.EventWithVersion{
		Event:   event,
		Version: version,
	}, nil
}

// Legacy conversion functions for backward compatibility

// toWireEvent converts a JSONEvent to WireEvent (backward compatibility).
func toWireEvent(jsonEvent JSONEvent) (WireEvent, error) {
	data, err := json.Marshal(jsonEvent.Data)
	if err != nil {
		return WireEvent{}, fmt.Errorf("failed to marshal data: %w", err)
	}

	return WireEvent{
		ID:          jsonEvent.ID,
		Type:        jsonEvent.Type,
		AggregateID: jsonEvent.AggregateID,
		Data:        data,
		Metadata:    jsonEvent.Metadata,
	}, nil
}

// fromWireEvent converts a WireEvent to JSONEvent (backward compatibility).
func fromWireEvent(wireEvent WireEvent) (JSONEvent, error) {
	var data interface{}
	if err := json.Unmarshal(wireEvent.Data, &data); err != nil {
		return JSONEvent{}, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return JSONEvent{
		ID:          wireEvent.ID,
		Type:        wireEvent.Type,
		AggregateID: wireEvent.AggregateID,
		Data:        data,
		Metadata:    wireEvent.Metadata,
	}, nil
}
