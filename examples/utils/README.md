# Go Sync Kit Utilities

This directory contains utility functions, mock implementations, and helper code for testing and development with Go Sync Kit.

## Files

### `memory_store.go`
In-memory implementation of the EventStore interface for testing and development:
- Fast, non-persistent storage
- Useful for unit tests and prototyping
- Implements full EventStore interface
- Thread-safe operations

### `mock_transport.go`
Mock transport implementation for testing:
- Simulates network operations without actual network calls
- Configurable delays and failures
- Useful for unit testing sync logic
- Can simulate various network conditions

### `mock_types.go`
Mock event types and utilities:
- Simple event implementations for testing
- Helper functions for creating test events
- Mock data generators

### `cursor_sync.go`
Example of cursor-based synchronization:
- Demonstrates cursor usage patterns
- Shows how to implement custom cursor logic
- Example of incremental sync strategies

### `transport.go`
Basic transport utilities and interfaces:
- Common transport patterns
- Helper functions for transport implementations
- Interface definitions

## Usage

These utilities are designed to be imported and used in your own applications or tests:

```go
import "github.com/c0deZ3R0/go-sync-kit/examples/utils"

// Use memory store for testing
store := utils.NewMemoryStore()

// Use mock transport in tests
transport := utils.NewMockTransport()
```

## Testing

These utilities are particularly useful for:

- **Unit Testing**: Use memory store and mock transport to test sync logic without external dependencies
- **Integration Testing**: Mock different failure scenarios
- **Performance Testing**: Measure sync performance with in-memory storage
- **Development**: Rapid prototyping without setting up databases

## Examples

### Using Memory Store
```go
store := utils.NewMemoryStore()
event := &utils.MockEvent{
    IDValue: "test-1",
    TypeValue: "test.created",
    Data: map[string]interface{}{"value": 42},
}

// Store and retrieve
ctx := context.Background()
err := store.Store(ctx, event, nil)
if err != nil {
    log.Fatal(err)
}

events, err := store.Load(ctx, nil)
if err != nil {
    log.Fatal(err)
}
```

### Using Mock Transport
```go
transport := utils.NewMockTransport()
transport.SetDelay(100 * time.Millisecond) // Simulate network latency

// Use in sync manager
manager := synckit.NewSyncManager(store, transport, opts)
```

## Contributing

When adding new utilities:

1. Follow Go naming conventions
2. Add comprehensive documentation
3. Include usage examples
4. Add tests for utility functions
5. Update this README

## Integration with Examples

These utilities are used by other examples in this directory:
- The basic example might use mock transport for testing
- The conflict resolution example can use memory store for unit tests
- Development scripts can use these for rapid prototyping
