# Go Sync Kit

A generic, event-driven synchronization library for distributed Go applications. Go Sync Kit enables offline-first architectures with conflict resolution and pluggable storage backends.

## Features

- **Event-Driven Architecture**: Built around append-only event streams
- **Offline-First**: Full support for offline operation with automatic sync when reconnected
- **Pluggable Components**: Interfaces for storage, transport, versioning, and conflict resolution
- **Conflict Resolution**: Multiple strategies including last-write-wins, merge, and custom resolvers
- **Concurrent Safe**: Thread-safe operations with proper synchronization
- **Transport Agnostic**: Works with HTTP, gRPC, WebSockets, NATS, or any custom transport
- **Storage Agnostic**: Compatible with SQLite, BadgerDB, PostgreSQL, or any storage backend
- **Automatic Sync**: Configurable periodic synchronization
- **Filtering**: Sync only specific events based on custom filters
- **Batching**: Efficient batch processing for large event sets

## Installation

```bash
go get github.com/c0deZ3R0/go-sync-kit
```

## Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/c0deZ3R0/go-sync-kit"
)

// Implement your event type
type MyEvent struct {
    id          string
    eventType   string
    aggregateID string
    data        interface{}
}

func (e *MyEvent) ID() string { return e.id }
func (e *MyEvent) Type() string { return e.eventType }
func (e *MyEvent) AggregateID() string { return e.aggregateID }
func (e *MyEvent) Data() interface{} { return e.data }
func (e *MyEvent) Metadata() map[string]interface{} { return nil }

func main() {
    // Create your storage and transport implementations
    store := &MyEventStore{} // Your EventStore implementation
    transport := &MyTransport{} // Your Transport implementation
    
    // Configure sync options
    options := &sync.SyncOptions{
        BatchSize:    100,
        SyncInterval: 30 * time.Second,
        ConflictResolver: &LastWriteWinsResolver{},
    }
    
    // Create sync manager
    syncManager := sync.NewSyncManager(store, transport, options)
    
    // Perform sync
    ctx := context.Background()
    result, err := syncManager.Sync(ctx)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Synced: %d pushed, %d pulled\n", 
        result.EventsPushed, result.EventsPulled)
}
```

## Architecture

Go Sync Kit follows clean architecture principles with clear separation of concerns:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   SyncManager   │    │    Transport    │
│                 │───▶│                 │───▶│                 │
│  (Your Code)    │    │  (Coordination) │    │   (Network)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                               │
                               ▼
                    ┌─────────────────┐    ┌─────────────────┐
                    │   EventStore    │    │ ConflictResolver │
                    │                 │    │                 │
                    │   (Storage)     │    │  (Resolution)   │
                    └─────────────────┘    └─────────────────┘
```

## Core Interfaces

### Event
Represents a syncable event in your system:

```go
type Event interface {
    ID() string
    Type() string
    AggregateID() string
    Data() interface{}
    Metadata() map[string]interface{}
}
```

### Version
Handles versioning for sync operations:

```go
type Version interface {
    Compare(other Version) int
    String() string
    IsZero() bool
}
```

### EventStore
Provides persistence for events:

```go
type EventStore interface {
    Store(ctx context.Context, event Event, version Version) error
    Load(ctx context.Context, since Version) ([]EventWithVersion, error)
    LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error)
    LatestVersion(ctx context.Context) (Version, error)
    Close() error
}
```

### Transport
Handles network communication:

```go
type Transport interface {
    Push(ctx context.Context, events []EventWithVersion) error
    Pull(ctx context.Context, since Version) ([]EventWithVersion, error)
    Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error
    Close() error
}
```

### ConflictResolver
Resolves conflicts when the same data is modified concurrently:

```go
type ConflictResolver interface {
    Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error)
}
```

## Conflict Resolution Strategies

Go Sync Kit supports multiple conflict resolution strategies:

### Last-Write-Wins
```go
type LastWriteWinsResolver struct{}

func (r *LastWriteWinsResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
    // Keep the events with the latest timestamp
    var resolved []EventWithVersion
    // Implementation logic here...
    return resolved, nil
}
```

### Custom Merge Strategy
```go
type CustomMergeResolver struct{}

func (r *CustomMergeResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
    // Implement your custom merge logic
    // Could merge data fields, prompt user, etc.
    return mergedEvents, nil
}
```

## Configuration Options

```go
type SyncOptions struct {
    // Sync direction control
    PushOnly bool
    PullOnly bool
    
    // Conflict handling
    ConflictResolver ConflictResolver
    
    // Event filtering
    Filter func(Event) bool
    
    // Performance tuning
    BatchSize int
    SyncInterval time.Duration
}
```

### Example with filtering:
```go
options := &sync.SyncOptions{
    BatchSize: 50,
    Filter: func(e sync.Event) bool {
        // Only sync specific event types
        return e.Type() == "UserCreated" || e.Type() == "OrderPlaced"
    },
}
```

## Storage Implementations

### SQLite Example
```go
type SQLiteEventStore struct {
    db *sql.DB
}

func (s *SQLiteEventStore) Store(ctx context.Context, event Event, version Version) error {
    query := `INSERT INTO events (id, type, aggregate_id, data, version) VALUES (?, ?, ?, ?, ?)`
    _, err := s.db.ExecContext(ctx, query, event.ID(), event.Type(), 
        event.AggregateID(), event.Data(), version.String())
    return err
}

// Implement other EventStore methods...
```

### BadgerDB Example
```go
type BadgerEventStore struct {
    db *badger.DB
}

func (b *BadgerEventStore) Store(ctx context.Context, event Event, version Version) error {
    return b.db.Update(func(txn *badger.Txn) error {
        key := []byte(fmt.Sprintf("event:%s", event.ID()))
        eventData := EventWithVersion{Event: event, Version: version}
        data, err := json.Marshal(eventData)
        if err != nil {
            return err
        }
        return txn.Set(key, data)
    })
}
```

## Transport Implementations

### HTTP Transport Example
```go
type HTTPTransport struct {
    client  *http.Client
    baseURL string
}

func (h *HTTPTransport) Push(ctx context.Context, events []EventWithVersion) error {
    data, err := json.Marshal(events)
    if err != nil {
        return err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", 
        h.baseURL+"/sync/push", bytes.NewBuffer(data))
    if err != nil {
        return err
    }
    
    resp, err := h.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### NATS Transport Example (with Watermill)
```go
type NATSTransport struct {
    publisher  message.Publisher
    subscriber message.Subscriber
}

func (n *NATSTransport) Push(ctx context.Context, events []EventWithVersion) error {
    for _, event := range events {
        data, err := json.Marshal(event)
        if err != nil {
            return err
        }
        
        msg := message.NewMessage(watermill.NewUUID(), data)
        err = n.publisher.Publish("sync.events", msg)
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Advanced Usage

### Automatic Sync
```go
// Start automatic sync every 30 seconds
ctx := context.Background()
err := syncManager.StartAutoSync(ctx)
if err != nil {
    log.Fatal(err)
}

// Stop automatic sync
err = syncManager.StopAutoSync()
```

### Event Subscriptions
```go
err := syncManager.Subscribe(func(result *sync.SyncResult) {
    log.Printf("Sync completed: %d events pushed, %d pulled, %d conflicts resolved",
        result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
    
    if len(result.Errors) > 0 {
        log.Printf("Sync errors: %v", result.Errors)
    }
})
```

### Manual Push/Pull
```go
// Push only local changes
result, err := syncManager.Push(ctx)

// Pull only remote changes  
result, err := syncManager.Pull(ctx)
```

## Testing

Go Sync Kit is designed for testability with mock implementations included:

```go
func TestMySync(t *testing.T) {
    store := &MockEventStore{}
    transport := &MockTransport{}
    resolver := &MockConflictResolver{}
    
    sm := sync.NewSyncManager(store, transport, &sync.SyncOptions{
        ConflictResolver: resolver,
    })
    
    // Test your sync logic
    result, err := sm.Sync(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, 1, result.EventsPushed)
}
```

Run tests:
```bash
go test ./...
```

## Performance Considerations

- **Batching**: Use appropriate batch sizes for your network conditions
- **Filtering**: Apply filters to reduce sync overhead
- **Storage**: Choose storage backends appropriate for your scale
- **Conflict Resolution**: Simple strategies (like last-write-wins) are faster than complex merging

## Roadmap

- [ ] Built-in storage implementations (SQLite, BadgerDB)
- [ ] Built-in transport implementations (HTTP, gRPC, WebSocket)
- [ ] Vector clock versioning implementation
- [ ] Compression support for large event payloads
- [ ] Metrics and observability hooks
- [ ] Schema evolution support

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Inspiration

This project was inspired by:
- [Go Kit](https://github.com/go-kit/kit) for clean interface design
- [PouchDB](https://pouchdb.com/) for sync protocol concepts
- [CouchDB](https://couchdb.apache.org/) for replication patterns
- [Watermill](https://github.com/ThreeDotsLabs/watermill) for event streaming architecture
