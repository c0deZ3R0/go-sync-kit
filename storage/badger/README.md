# BadgerDB Event Store

This package provides a BadgerDB implementation of the event store interface for Go Sync Kit. BadgerDB is a fast key-value store designed for high performance and efficient storage on SSDs.

## Features

- **Atomic Operations**: All event store operations are atomic and ACID compliant
- **Context Support**: Full context support with timeouts and cancellation
- **Concurrent Access**: Thread-safe operations with proper synchronization
- **WAL**: Write-Ahead Logging support for durability
- **Efficient Storage**: Optimized for SSDs with LSM tree structure
- **Configurable**: Options for memory usage, compression, and more

## Installation

```bash
go get github.com/c0deZ3R0/go-sync-kit
```

## Usage

### Basic Setup

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/c0deZ3R0/go-sync-kit/storage/badger"
)

func main() {
    // Create a new BadgerDB store with default configuration
    store, err := badger.New(&badger.Config{
        Path: "./data",  // Directory where BadgerDB files will be stored
        Logger: log.New(os.Stdout, "[BadgerStore] ", log.LstdFlags),
    })
    if err != nil {
        log.Fatalf("Failed to create BadgerDB store: %v", err)
    }
    defer store.Close()

    // Use the store...
}
```

### Advanced Configuration

```go
config := &badger.Config{
    Path: "./data",
    Logger: logger,
    Options: &badger.Options{
        // Memory budget for table building
        MemTableSize: 64 << 20, // 64MB

        // Memory budget for block cache
        BlockCacheSize: 64 << 20, // 64MB

        // Number of memtables to keep in memory
        NumMemtables: 5,

        // Number of Level 0 tables before compaction
        NumLevelZeroTables: 5,

        // Maximum size of value log entry
        ValueLogFileSize: 1 << 30, // 1GB
    },
}

store, err := badger.New(config)
if err != nil {
    log.Fatal(err)
}
```

### Storing Events

```go
event := &MyEvent{
    id: "evt-123",
    eventType: "UserCreated",
    aggregateID: "user-456",
    data: map[string]interface{}{
        "name": "John Doe",
        "email": "john@example.com",
    },
}

version := sync.NewVersion("1.0")
err := store.Store(context.Background(), event, version)
if err != nil {
    log.Printf("Failed to store event: %v", err)
}
```

### Loading Events

```go
// Load all events since a specific version
events, err := store.Load(context.Background(), lastVersion)
if err != nil {
    log.Printf("Failed to load events: %v", err)
}

// Load events for a specific aggregate
aggregateEvents, err := store.LoadByAggregate(context.Background(), "user-456", lastVersion)
if err != nil {
    log.Printf("Failed to load aggregate events: %v", err)
}
```

### Getting Latest Version

```go
version, err := store.LatestVersion(context.Background())
if err != nil {
    log.Printf("Failed to get latest version: %v", err)
}
```

## Performance Tuning

### Memory Usage

BadgerDB's memory usage can be configured through several options:

```go
config := &badger.Config{
    Options: &badger.Options{
        // Increase memory table size for better write performance
        MemTableSize: 128 << 20, // 128MB

        // Increase block cache size for better read performance
        BlockCacheSize: 256 << 20, // 256MB

        // Control number of memory tables
        NumMemtables: 2,
    },
}
```

### Compaction

Compaction settings can be tuned for optimal performance:

```go
config := &badger.Config{
    Options: &badger.Options{
        // Level 0 compaction threshold
        NumLevelZeroTables: 5,
        NumLevelZeroTablesStall: 10,

        // Base table size for each level
        BaseTableSize: 2 << 20, // 2MB

        // Level multiplier
        LevelSizeMultiplier: 10,
    },
}
```

### Value Log

Configure value log settings for larger entries:

```go
config := &badger.Config{
    Options: &badger.Options{
        // Increase value log file size
        ValueLogFileSize: 1 << 30, // 1GB

        // Configure value log GC
        ValueLogMaxEntries: 1000000,
    },
}
```

## Garbage Collection

BadgerDB requires periodic garbage collection to reclaim space. You can run GC manually:

```go
func runGC(store *badger.BadgerEventStore) {
    ctx := context.Background()
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            err := store.RunGC(ctx)
            if err != nil {
                log.Printf("GC error: %v", err)
            }
        }
    }
}
```

## Testing

The BadgerDB implementation includes comprehensive tests:

```bash
go test ./storage/badger/... -v
```

Test coverage includes:
- Basic CRUD operations
- Concurrent access
- Context cancellation
- Edge cases
- Version management
- Load stress testing

## Best Practices

1. **Directory Location**: Place BadgerDB files on SSD for best performance
2. **Backup Strategy**: Use BadgerDB's built-in backup functionality
3. **GC Timing**: Run garbage collection during off-peak hours
4. **Memory Configuration**: Adjust memory settings based on available RAM
5. **Monitoring**: Monitor disk usage and GC metrics

## Limitations

1. **Disk Space**: BadgerDB requires about 2x the data size due to LSM tree design
2. **Memory Usage**: Memory usage increases with the number of keys
3. **GC Impact**: Garbage collection can impact performance temporarily

## Error Handling

The implementation provides detailed error information:

```go
if err := store.Store(ctx, event, version); err != nil {
    switch {
    case errors.Is(err, badger.ErrVersionConflict):
        // Handle version conflict
    case errors.Is(err, badger.ErrEventNotFound):
        // Handle not found error
    case errors.Is(err, context.DeadlineExceeded):
        // Handle timeout
    default:
        // Handle other errors
    }
}
```
