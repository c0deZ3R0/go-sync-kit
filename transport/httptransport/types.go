package httptransport

import (
	"context"
	"fmt"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// ServerOptions configures the HTTP transport server behavior
type ServerOptions struct {
	// MaxRequestSize is the maximum allowed size of incoming request bodies in bytes (compressed)
	// If 0, defaults to 10MB
	MaxRequestSize int64

	// MaxDecompressedSize is the maximum allowed size of decompressed request bodies in bytes
	// This prevents zip-bomb attacks when handling gzip-compressed requests
	// If 0, defaults to 20MB
	MaxDecompressedSize int64

	// CompressionEnabled enables gzip/deflate compression for responses
	// Responses larger than CompressionThreshold will be compressed
	CompressionEnabled bool

	// CompressionThreshold is the minimum size in bytes before responses are compressed
	// If CompressionEnabled is true and 0, defaults to 1KB
	CompressionThreshold int64

	// RequestTimeout is the maximum duration for processing a single request
	// If 0, defaults to 30 seconds
	RequestTimeout time.Duration

	// ShutdownTimeout is the maximum duration to wait for in-flight requests during shutdown
	// If 0, defaults to 10 seconds
	ShutdownTimeout time.Duration
}

// DefaultServerOptions returns the default server options
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		MaxRequestSize:       10 * 1024 * 1024, // 10MB
		MaxDecompressedSize:  20 * 1024 * 1024, // 20MB
		CompressionEnabled:   true,
		CompressionThreshold: 1024,            // 1KB
		RequestTimeout:       30 * time.Second, // 30s
		ShutdownTimeout:      10 * time.Second, // 10s
	}
}

// ClientOptions configures the HTTP transport client behavior
type ClientOptions struct {
	// CompressionEnabled enables sending Accept-Encoding header for gzip/deflate
	CompressionEnabled bool

	// DisableAutoDecompression disables Go's automatic response decompression
	// When true, the client can enforce both compressed and decompressed size limits
	// When false (default), Go auto-decompresses responses transparently
	DisableAutoDecompression bool

	// MaxResponseSize is the maximum allowed size of response bodies in bytes (compressed)
	// If 0, defaults to 10MB
	MaxResponseSize int64

	// MaxDecompressedResponseSize is the maximum allowed size of decompressed response bodies in bytes
	// This prevents zip-bomb attacks when handling gzip-compressed responses
	// If 0, defaults to 20MB
	MaxDecompressedResponseSize int64

	// RequestTimeout is the maximum duration for a single request including retries
	// If 0, defaults to 30 seconds
	RequestTimeout time.Duration

	// RetryMax is the maximum number of retries for failed requests
	// If 0, defaults to 3
	RetryMax int

	// RetryWaitMin is the minimum time to wait between retries
	// If 0, defaults to 1 second
	RetryWaitMin time.Duration

	// RetryWaitMax is the maximum time to wait between retries
	// If 0, defaults to 30 seconds
	RetryWaitMax time.Duration
}

// DefaultClientOptions returns the default client options
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		CompressionEnabled:          true,
		MaxResponseSize:             10 * 1024 * 1024, // 10MB
		MaxDecompressedResponseSize: 20 * 1024 * 1024, // 20MB
		RequestTimeout:              30 * time.Second,  // 30s
		RetryMax:                    3,                 // 3 retries
		RetryWaitMin:                1 * time.Second,   // 1s
		RetryWaitMax:                30 * time.Second,  // 30s
	}
}

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
    var version string
    if ev.Version != nil {
        version = ev.Version.String()
    }
	return JSONEventWithVersion{
		Event:   toJSONEvent(ev.Event),
		Version: version,
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

