# SQLite EventStore

A production-ready SQLite implementation of the `go-sync-kit` EventStore interface.

## Features

- ✅ **Full EventStore Interface**: Complete implementation of all required methods
- ✅ **Transaction Safety**: All write operations use database transactions  
- ✅ **Connection Pooling**: Configurable connection pool with sensible defaults
- ✅ **Context Support**: All operations respect context cancellation and timeouts
- ✅ **Thread Safety**: Safe for concurrent use across multiple goroutines
- ✅ **Custom Error Types**: Specific error types for better error handling
- ✅ **Performance Optimized**: Proper indexing and efficient queries
- ✅ **Comprehensive Tests**: Full test coverage including benchmarks

## Installation

```bash
go get github.com/mattn/go-sqlite3
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    "github.com/c0deZ3R0/go-sync-kit/sync"
)

func main() {
    // Create a new SQLite store
    store, err := sqlite.New("events.db", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()
    
    // Use with SyncManager
    transport := &MyTransport{} // Your transport implementation
    options := &sync.SyncOptions{
        BatchSize: 100,
    }
    
    syncManager := sync.NewSyncManager(store, transport, options)
    
    // Perform sync operations
    ctx := context.Background()
    result, err := syncManager.Sync(ctx)
    if err != nil {
        log.Printf("Sync failed: %v", err)
        return
    }
    
    log.Printf("Synced: %d events pushed, %d events pulled", 
        result.EventsPushed, result.EventsPulled)
}
```

### Advanced Configuration

```go
config := &sqlite.Config{
    MaxOpenConns:    25,                // Maximum number of open connections
    MaxIdleConns:    5,                 // Maximum number of idle connections
    ConnMaxLifetime: time.Hour,         // Maximum connection lifetime
    ConnMaxIdleTime: 5 * time.Minute,   // Maximum connection idle time
}

store, err := sqlite.New("events.db", config)
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

### Working with Versions

The SQLite implementation uses `IntegerVersion` for simple sequential versioning:

```go
// Create a version
version := sqlite.IntegerVersion(42)

// Compare versions
if version.Compare(sqlite.IntegerVersion(41)) > 0 {
    fmt.Println("Version 42 is greater than 41")
}

// Check if version is zero
if version.IsZero() {
    fmt.Println("This is the initial version")
}

// Convert to string
fmt.Printf("Version: %s\n", version.String())
```

### Monitoring and Observability

```go
// Get database statistics
stats := store.Stats()
fmt.Printf("Open connections: %d\n", stats.OpenConnections)
fmt.Printf("In use: %d\n", stats.InUse)
fmt.Printf("Idle: %d\n", stats.Idle)
```

## Database Schema

The SQLite store creates the following table structure:

```sql
CREATE TABLE events (
    version         INTEGER PRIMARY KEY AUTOINCREMENT,
    id              TEXT NOT NULL UNIQUE,
    aggregate_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    data            TEXT,
    metadata        TEXT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_aggregate_id ON events (aggregate_id);
CREATE INDEX idx_version ON events (version);
CREATE INDEX idx_created_at ON events (created_at);
```

## Error Handling

The SQLite store defines custom error types for better error handling:

```go
import "errors"

// Check for specific errors
_, err := store.Load(ctx, version)
if errors.Is(err, sqlite.ErrStoreClosed) {
    log.Println("Store has been closed")
} else if errors.Is(err, sqlite.ErrIncompatibleVersion) {
    log.Println("Version type is not compatible")
}
```

Available error types:
- `ErrIncompatibleVersion`: Version type is not `IntegerVersion`
- `ErrEventNotFound`: Requested event was not found
- `ErrStoreClosed`: Store has been closed

## Performance Considerations

### Connection Pool Settings

For high-throughput applications, tune the connection pool:

```go
config := &sqlite.Config{
    MaxOpenConns:    50,                // Higher for more concurrent operations
    MaxIdleConns:    10,                // More idle connections for faster reuse
    ConnMaxLifetime: 30 * time.Minute, // Shorter lifetime for busy systems
    ConnMaxIdleTime: 2 * time.Minute,  // Shorter idle time to free resources
}
```

### Batch Operations

When storing many events, consider batching them:

```go
ctx := context.Background()
for _, event := range events {
    if err := store.Store(ctx, event, version); err != nil {
        log.Printf("Failed to store event %s: %v", event.ID(), err)
    }
}
```

### WAL Mode

For better concurrent performance, consider enabling WAL mode:

```go
store, err := sqlite.New("events.db?_journal_mode=WAL", config)
```

## Testing

Run the test suite:

```bash
# Run all tests
go test ./storage/sqlite

# Run tests with race detection
go test -race ./storage/sqlite

# Run benchmarks
go test -bench=. ./storage/sqlite

# Run tests with coverage
go test -cover ./storage/sqlite
```

## Thread Safety

The SQLite EventStore is completely thread-safe and can be safely used across multiple goroutines. All database operations are protected by appropriate synchronization mechanisms.

## Best Practices

1. **Always use defer store.Close()** to ensure proper cleanup
2. **Configure connection pools** based on your application's concurrency needs
3. **Use context.WithTimeout()** for database operations in production
4. **Monitor database statistics** in production environments
5. **Handle errors appropriately** using the provided error types
6. **Consider WAL mode** for applications with high read concurrency

## Limitations

- Uses `IntegerVersion` only - not compatible with vector clocks or other version types
- SQLite limitations apply (single writer, file-based storage)
- Large datasets may require additional optimization (partitioning, archiving)

## Migration from Mock Implementation

Replace your mock EventStore with the SQLite implementation:

```go
// Before
store := &MockEventStore{}

// After  
store, err := sqlite.New("events.db", nil)
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

All interface methods remain the same, so no other code changes are required.
