# Cursor Wire Format Specification

This document specifies the wire format for cursors in the go-sync-kit project, including schema definitions, versioning strategy, and backward compatibility guarantees.

## Overview

Cursors in go-sync-kit represent positions in event streams and are used for synchronization between clients and servers. The wire format provides a stable, version-tolerant serialization mechanism that enables interoperability across different versions of the library.

## Wire Format Schema

### WireCursor Structure

The `WireCursor` is the top-level JSON structure used for serializing cursors over the wire:

```go
type WireCursor struct {
    Kind string          `json:"kind"`
    Data json.RawMessage `json:"data"`
}
```

#### Field Definitions

- **`kind`** (string, required): Identifies the cursor type and determines how to interpret the `data` field
- **`data`** (json.RawMessage, required): Type-specific payload containing the cursor's state

### Supported Cursor Kinds

#### 1. Integer Cursor (`"integer"`)

The most common cursor type, representing a simple sequence number.

**Purpose**: High-water mark tracking for simple, linear event streams.

**Data Format**: JSON number (uint64)

**Example**:
```json
{
  "kind": "integer",
  "data": 12345
}
```

**Properties**:
- Sequence numbers are monotonically increasing
- Zero value (`0`) represents the beginning of a stream
- Maximum value is `18446744073709551615` (uint64 max)

#### 2. Vector Cursor (`"vector"`)

Advanced cursor type for distributed systems with multiple nodes.

**Purpose**: Track progress across multiple nodes in a distributed event store using vector clocks.

**Data Format**: JSON object mapping node IDs to their respective sequence numbers

**Example**:
```json
{
  "kind": "vector",
  "data": {
    "node-1": 100,
    "node-2": 75,
    "node-3": 200
  }
}
```

**Properties**:
- Node IDs are strings
- Counters are uint64 values
- Empty map (`{}`) represents the beginning of a stream
- Order of nodes in the map is not significant (JSON object)

## Size Limitations

- **Maximum payload size**: 64 KB (65,536 bytes)
- **Rationale**: Prevents excessive memory usage and network overhead
- **Enforcement**: Validated during unmarshaling via `ValidateWireCursor()`

## Versioning and Compatibility

### Forward Compatibility

The wire format is designed to be forward-compatible:

1. **Kind-based dispatch**: New cursor kinds can be added without breaking existing code
2. **Unknown kinds**: Older versions will reject unknown cursor kinds with descriptive errors
3. **Data field flexibility**: The `json.RawMessage` type allows for evolving data schemas

### Backward Compatibility Guarantees

#### Stable Cursor Kinds

Once released, cursor kinds maintain backward compatibility:

- **`"integer"`**: Data format is stable (JSON number)
- **`"vector"`**: Data format is stable (JSON object with string keys and number values)

#### Breaking Changes

The following changes are considered breaking and require a major version bump:

1. Changing the JSON schema of the `WireCursor` struct
2. Modifying the data format for existing cursor kinds
3. Changing the semantic meaning of cursor kinds
4. Reducing size limitations

#### Non-Breaking Changes

The following changes are backward compatible:

1. Adding new cursor kinds
2. Increasing size limitations
3. Adding optional validation rules
4. Performance improvements to marshaling/unmarshaling

## Implementation Details

### Codec Registry

The cursor system uses a registry-based approach for extensibility:

```go
type Codec interface {
    Kind() string
    Marshal(c Cursor) (json.RawMessage, error)
    Unmarshal(data json.RawMessage) (Cursor, error)
}
```

#### Thread Safety

- Registry operations are protected by `sync.RWMutex`
- Safe for concurrent registration and lookup operations
- Default codecs are registered at initialization

### Validation

#### Wire Format Validation

The `ValidateWireCursor()` function performs the following checks:

1. **Null check**: Rejects `nil` cursors
2. **Size limit**: Enforces 64 KB maximum payload size
3. **Kind validation**: Ensures the cursor kind has a registered codec

#### Data Validation

Individual codecs are responsible for validating their data formats:

- **Integer**: Ensures data is a valid JSON number within uint64 range
- **Vector**: Validates JSON object with string keys and numeric values

## Wire Format Examples

### Complete Examples

#### Integer Cursor Examples

```json
// Zero cursor (beginning of stream)
{
  "kind": "integer", 
  "data": 0
}

// Active cursor
{
  "kind": "integer",
  "data": 42
}

// Maximum value
{
  "kind": "integer",
  "data": 18446744073709551615
}
```

#### Vector Cursor Examples

```json
// Empty vector (beginning of stream)
{
  "kind": "vector",
  "data": {}
}

// Single node
{
  "kind": "vector",
  "data": {
    "primary": 1000
  }
}

// Multi-node distributed system
{
  "kind": "vector",
  "data": {
    "web-1": 150,
    "web-2": 143,
    "worker-1": 89,
    "worker-2": 91
  }
}
```

### Round-trip Consistency

All cursor types must maintain round-trip consistency:

```go
// Marshal -> Unmarshal should preserve equality
original := IntegerCursor{Seq: 123}
wire, err := MarshalWire(original)
restored, err := UnmarshalWire(wire)
assert.Equal(t, original, restored)
```

## Error Handling

### Common Errors

1. **Unknown cursor kind**: `unknown cursor kind: <kind>`
2. **Payload too large**: `cursor payload too large: <size> bytes`
3. **Invalid JSON**: JSON parsing errors from standard library
4. **Type mismatch**: Codec-specific validation errors

### Error Recovery

- Clients should handle unknown cursor kinds gracefully
- Servers may provide cursor kind negotiation in future versions
- Applications should validate cursors before persistence

## Migration Guidelines

### Adding New Cursor Kinds

1. Define the cursor type implementing the `Cursor` interface
2. Create a codec implementing the `Codec` interface
3. Register the codec during initialization
4. Add comprehensive tests including wire format tests
5. Update documentation with examples

### Deprecating Cursor Kinds

1. Mark as deprecated in documentation
2. Maintain codec implementation for backward compatibility
3. Provide migration path to new cursor kind
4. Remove only after major version bump with sufficient notice

## Security Considerations

### Input Validation

- Always validate cursors received over the network
- Enforce size limitations to prevent DoS attacks
- Validate JSON structure before processing

### Resource Limits

- 64 KB size limit prevents memory exhaustion
- Cursor validation is bounded-time operation
- Codec registry has reasonable size limits

## Performance Characteristics

### Marshaling Performance

- **Integer cursors**: O(1) - simple JSON number encoding
- **Vector cursors**: O(n) where n is the number of nodes
- **Memory allocation**: Minimal for integer cursors, proportional to node count for vector cursors

### Network Efficiency

- JSON format provides good human readability vs. size trade-off
- Compression-friendly due to structured format
- Average cursor size: 50-200 bytes for typical use cases

## Testing Requirements

### Compatibility Tests

All cursor implementations must pass:

1. **Round-trip tests**: Marshal/unmarshal consistency
2. **Wire format tests**: Specific JSON format validation  
3. **Size limit tests**: Payload size enforcement
4. **Concurrent access tests**: Registry thread safety
5. **Fuzz tests**: Robustness against malformed input

### Regression Protection

- Deterministic tests with known wire format examples
- Version compatibility matrix testing
- Performance benchmark regression detection

## Future Considerations

### Potential Enhancements

1. **Binary wire format**: For high-performance scenarios
2. **Compression**: Built-in payload compression for large cursors
3. **Checksum validation**: Data integrity verification
4. **Schema evolution**: Formal schema versioning system

### Extensibility Points

- Codec registry allows new cursor types
- `json.RawMessage` provides flexible data encoding
- Size limits can be increased in future versions
- Additional validation hooks can be added

---

## Appendix: JSON Schema

### WireCursor JSON Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "kind": {
      "type": "string",
      "enum": ["integer", "vector"]
    },
    "data": {}
  },
  "required": ["kind", "data"],
  "additionalProperties": false
}
```

### Integer Data Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "number",
  "minimum": 0,
  "maximum": 18446744073709551615,
  "multipleOf": 1
}
```

### Vector Data Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "patternProperties": {
    "^.*$": {
      "type": "number",
      "minimum": 0,
      "maximum": 18446744073709551615,
      "multipleOf": 1
    }
  },
  "additionalProperties": false
}
```
