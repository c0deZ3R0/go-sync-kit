# Event Data Codec Registry

The codec registry provides a pluggable system for encoding and decoding event data across different transport mechanisms. This system allows domain-specific types to be encoded without double-encoding through generic JSON marshaling.

## Overview

The codec registry enables:
- **Stable wire format**: Consistent event data encoding across all transports
- **Domain-specific codecs**: Custom encoding/decoding for complex types
- **Pluggable architecture**: Easy to add new codecs for different data types
- **Fallback compatibility**: Graceful fallback to JSON for unknown types
- **Transport agnostic**: Works with HTTP, SSE, and future transport implementations

## Key Components

### Codec Interface

```go
type Codec interface {
    Kind() string                        // Unique identifier for this codec
    Encode(any) (json.RawMessage, error) // Encode value to JSON raw message
    Decode(json.RawMessage) (any, error) // Decode JSON raw message to value
}
```

### Registry

```go
type Registry struct { /* ... */ }

// Create a new registry
registry := codec.NewRegistry()

// Register a codec
registry.Register(myCodec)

// Get a codec by kind
codec, found := registry.Get("my_kind")
```

### Default Registry

A global registry is available for convenience:

```go
// Register with the default registry
codec.Register(myCodec)

// Get from the default registry
codec, found := codec.Get("my_kind")
```

## Usage

### 1. Define a Codec

```go
type UserProfileCodec struct{}

func (c *UserProfileCodec) Kind() string {
    return "user_profile"
}

func (c *UserProfileCodec) Encode(v any) (json.RawMessage, error) {
    profile, ok := v.(*UserProfile)
    if !ok {
        return nil, fmt.Errorf("expected *UserProfile, got %T", v)
    }
    
    // Custom encoding logic (validation, transformation, etc.)
    return json.Marshal(profile)
}

func (c *UserProfileCodec) Decode(raw json.RawMessage) (any, error) {
    var profile UserProfile
    if err := json.Unmarshal(raw, &profile); err != nil {
        return nil, err
    }
    
    // Custom decoding logic (validation, transformation, etc.)
    return &profile, nil
}
```

### 2. Register the Codec

```go
registry := codec.NewRegistry()
registry.Register(&UserProfileCodec{})

// Or use the default registry
codec.Register(&UserProfileCodec{})
```

### 3. Use with Events

To use a codec with events, add the codec kind to the event metadata:

```go
event := &MyEvent{
    // ... other fields ...
    metadata: map[string]interface{}{
        "kind": "user_profile", // This tells the system which codec to use
    },
}
```

The system checks these metadata keys in order:
1. `kind`
2. `codec_kind` 
3. `data_kind`

### 4. Integration with HTTP Transport

```go
// Create codec-aware encoder
registry := codec.NewRegistry()
registry.Register(&UserProfileCodec{})
encoder := httptransport.NewCodecAwareEncoder(registry, true) // true = enable fallback

// Configure server options
serverOpts := httptransport.DefaultServerOptions()
serverOpts.CodecAwareEncoder = encoder

// Configure client options
clientOpts := httptransport.DefaultClientOptions()
clientOpts.CodecAwareEncoder = encoder

// Create transport with codec support
transport := httptransport.NewTransport(baseURL, httpClient, versionParser, clientOpts)
handler := httptransport.NewSyncHandler(store, logger, versionParser, serverOpts)
```

## Wire Format

Events with codec support use the `WireEvent` format:

```go
type WireEvent struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    AggregateID string                 `json:"aggregate_id"`
    Data        json.RawMessage        `json:"data"`         // Encoded data
    DataKind    string                 `json:"data_kind,omitempty"` // Codec identifier
    Metadata    map[string]interface{} `json:"metadata"`
}
```

When a codec is used:
- `Data` contains the codec-encoded data
- `DataKind` contains the codec identifier
- During decoding, the system looks up the codec by `DataKind`

When no codec is available:
- `Data` contains JSON-encoded data
- `DataKind` is empty
- Standard JSON unmarshaling is used

## Fallback Behavior

The `CodecAwareEncoder` supports fallback mode:

```go
encoder := httptransport.NewCodecAwareEncoder(registry, true) // fallback enabled
```

With fallback enabled:
- If no codec is found for a kind, falls back to JSON encoding
- If codec encoding fails, falls back to JSON encoding
- If codec decoding fails, falls back to JSON decoding

With fallback disabled:
- Returns errors when codecs are not found
- Returns errors when codec operations fail
- Useful for strict validation scenarios

## Thread Safety

The registry is thread-safe and supports concurrent access:
- Multiple goroutines can safely register codecs
- Multiple goroutines can safely retrieve codecs
- All operations use proper locking

## Best Practices

1. **Unique Kind Names**: Use unique, descriptive names for codec kinds
2. **Version Compatibility**: Design codecs to handle version changes gracefully
3. **Error Handling**: Provide clear error messages in codec implementations
4. **Testing**: Test codec round-trips thoroughly
5. **Documentation**: Document the expected data types for each codec

## Examples

See `examples/codec-aware-transport/main.go` for a complete working example demonstrating:
- Custom codec implementations
- Registry usage
- Event encoding/decoding
- HTTP transport integration
- Round-trip testing

## Compatibility

The codec system is designed for compatibility with:
- **Existing Code**: Works alongside existing JSON-based events
- **Future Transports**: SSE, WebSocket, gRPC transports can use the same codecs
- **Different Languages**: Wire format is language-agnostic JSON
- **Legacy Systems**: Falls back gracefully for unknown types
