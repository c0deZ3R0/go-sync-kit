# BadgerDB Projection Offset Store

This package provides a BadgerDB-backed implementation of the projection offset store interface. BadgerDB offers excellent performance for key-value storage operations and is well-suited for high-concurrency scenarios.

## Features

- **High Performance**: BadgerDB is optimized for fast reads and writes
- **Concurrent Access**: Handles multiple goroutines accessing the same store safely
- **Persistent Storage**: Offsets are stored durably on disk
- **Atomic Operations**: All operations are atomic to ensure consistency
- **Garbage Collection**: Built-in support for BadgerDB garbage collection
- **Production Ready**: Uses BadgerDB's proven default configurations

## Usage

### Basic Setup

```go
package main

import (
    "context"
    "log"
    
    "github.com/c0deZ3R0/go-sync-kit/projection/badger"
    "github.com/c0deZ3R0/go-sync-kit/cursor"
)

func main() {
    // Create a config pointing to your data directory
    config := badger.DefaultConfig("/path/to/badger/data")
    
    // Create the offset store
    store, err := badger.NewOffsetStore(config, parseVersionFunc)
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()
    
    // Use the store
    ctx := context.Background()
    
    // Set an offset
    version := cursor.IntegerCursor{Seq: 42}
    err = store.Set(ctx, "my-projection", version)
    if err != nil {
        log.Fatal(err)
    }
    
    // Get an offset
    retrievedVersion, err := store.Get(ctx, "my-projection")
    if err != nil {
        log.Fatal(err)
    }
    
    if retrievedVersion != nil {
        log.Printf("Current offset: %s", retrievedVersion.String())
    }
}

// You need to provide a version parser function
func parseVersionFunc(ctx context.Context, versionStr string) (synckit.Version, error) {
    // Implementation depends on your version format
    // This is just an example for integer cursors
    if versionStr == "" || versionStr == "0" {
        return cursor.IntegerCursor{Seq: 0}, nil
    }
    
    val, err := strconv.ParseInt(versionStr, 10, 64)
    if err != nil {
        return nil, err
    }
    
    return cursor.IntegerCursor{Seq: uint64(val)}, nil
}
```

### Advanced Configuration

```go
import "github.com/dgraph-io/badger/v4"

// Create custom BadgerDB options if needed
opts := badger.DefaultOptions("/path/to/data")
opts.SyncWrites = false // For higher performance, lower durability

config := &badger.Config{
    Path: "/path/to/data",
    BadgerOptions: &opts,
}

store, err := badger.NewOffsetStore(config, parseVersionFunc)
```

### With Custom Logger

```go
import "log/slog"

logger := slog.Default()
store, err := badger.NewOffsetStore(
    config, 
    parseVersionFunc, 
    badger.WithLogger(logger),
)
```

## API Reference

### Core Operations

- `Get(ctx, projectionName)` - Retrieve the last applied version for a projection
- `Set(ctx, projectionName, version)` - Update the last applied version for a projection  
- `Reset(ctx, projectionName)` - Clear the offset for a projection (restarts from beginning)
- `ListProjections(ctx)` - Get all projection names that have stored offsets

### Administrative Operations

- `Close()` - Close the store and release resources
- `RunGC(ctx)` - Run garbage collection to reclaim disk space

## Performance Characteristics

- **Reads**: Very fast, especially for recently written keys
- **Writes**: Fast atomic writes with configurable sync behavior
- **Concurrent Access**: Excellent concurrent read performance, writes are serialized per key
- **Memory Usage**: Efficient with BadgerDB's LSM tree design
- **Disk Usage**: Compact storage with garbage collection support

## Error Handling

The store returns structured errors using the go-sync-kit error package:

```go
offset, err := store.Get(ctx, "projection-name")
if err != nil {
    // Handle error - could be network, disk, or application error
    // Error includes component info and operation context
    log.Printf("Failed to get offset: %v", err)
}
```

## Maintenance

### Garbage Collection

Run garbage collection periodically to reclaim disk space:

```go
// Run GC in a background goroutine
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        if err := store.RunGC(ctx); err != nil {
            log.Printf("GC failed: %v", err)
        }
    }
}()
```

### Monitoring

List all projections to see what's being tracked:

```go
projections, err := store.ListProjections(ctx)
if err == nil {
    log.Printf("Tracking %d projections: %v", len(projections), projections)
}
```

## Thread Safety

The BadgerDB offset store is fully thread-safe and can be used concurrently from multiple goroutines. All operations are atomic and the store handles concurrent access internally.

## Comparison with SQLite Store

| Feature | BadgerDB Store | SQLite Store |
|---------|---------------|--------------|
| Concurrent Reads | Excellent | Good |
| Concurrent Writes | Very Good | Limited |
| Setup Complexity | Low | Medium |
| Memory Usage | Low | Medium |
| File Format | BadgerDB LSM | SQLite DB |
| In-Memory Testing | Disk-based | Supports :memory: |

## Dependencies

- `github.com/dgraph-io/badger/v4` - BadgerDB key-value store
- Standard library packages for context, sync, logging

## Testing

Run the full test suite including race detection:

```bash
go test -race
```

Run benchmarks:

```bash
go test -bench=.
```
