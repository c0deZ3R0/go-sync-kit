# Example 3: Event Creation and Storage

This example builds upon the basic examples by demonstrating how to create, store, and retrieve custom events using Go Sync Kit.

## What You'll Learn

- **Custom Event Types**: How to create events that implement the `Event` interface
- **Event Storage**: Storing events with proper versioning and metadata
- **Event Retrieval**: Loading events from storage with filtering capabilities
- **Aggregate IDs**: Grouping related events by aggregate identifier
- **Event Versioning**: Understanding how events are versioned for synchronization

## Key Concepts

### Event Interface
All events must implement:
```go
type Event interface {
    Type() string        // Event type identifier
    AggregateID() string // Groups related events
    Data() []byte        // JSON serialized event data
    OccurredAt() time.Time // When the event happened
}
```

### Event Storage
- Events are stored with sequential version numbers
- Each event maintains its timestamp and metadata
- Events can be retrieved by version range or aggregate ID

## Running the Example

```bash
cd 03-events-and-storage
go run main.go
```

## What Happens

1. **Event Creation**: Creates user and product events with rich metadata
2. **Storage**: Stores events with sequential versioning
3. **Retrieval**: Demonstrates different ways to load events
4. **Filtering**: Shows aggregate-based event filtering
5. **Sync**: Performs a local sync operation

## Output Structure

The example shows:
- 📝 Event creation and storage process
- 👤 User-specific events with metadata
- 🛍️ Product events with pricing information
- 📚 Event retrieval and inspection
- 🔍 Aggregate-based filtering
- 📊 Storage statistics and versioning

## Next Steps

After running this example, you'll understand:
- How to structure custom events
- Event storage and retrieval patterns
- Version management for synchronization
- Aggregate-based event organization

**Continue to Example 4** to learn about conflict resolution strategies.
