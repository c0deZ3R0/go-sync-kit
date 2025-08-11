# PostgreSQL EventStore with LISTEN/NOTIFY

This package provides a PostgreSQL implementation of the go-sync-kit EventStore with real-time notification capabilities using PostgreSQL's LISTEN/NOTIFY mechanism.

## Features

- **ACID Transactions**: Full transactional support with PostgreSQL's ACID guarantees
- **Real-time Notifications**: Instant event notifications via PostgreSQL LISTEN/NOTIFY
- **Stream-based Subscriptions**: Subscribe to events for specific aggregates
- **Global Event Subscriptions**: Subscribe to all events across all streams
- **Event Type Filtering**: Subscribe to events of specific types
- **Connection Recovery**: Automatic reconnection with exponential backoff
- **High Performance**: Connection pooling and prepared statements
- **Production Ready**: Comprehensive error handling, logging, and monitoring

## Quick Start

### 1. Start PostgreSQL with Docker

```bash
# Start PostgreSQL using the provided Docker Compose
docker-compose -f docker-compose.test.yml up -d postgres

# Or run PostgreSQL manually
docker run --name postgres-eventstore \
  -e POSTGRES_DB=eventstore \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 -d postgres:15
```

### 2. Basic Usage

```go
package main

import (
    "context"
    "log"
    "os"
    
    "github.com/c0deZ3R0/go-sync-kit/storage/postgres"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func main() {
    // Create configuration
    config := postgres.DefaultConfig("postgres://user:password@localhost/eventstore?sslmode=disable")
    config.Logger = log.New(os.Stdout, "[EventStore] ", log.LstdFlags)
    
    // Create EventStore
    store, err := postgres.New(config)
    if err != nil {
        log.Fatal("Failed to create store:", err)
    }
    defer store.Close()
    
    ctx := context.Background()
    
    // Store an event
    event := &MyEvent{
        id:          "event-1",
        eventType:   "UserCreated",
        aggregateID: "user-123",
        data:        map[string]string{"name": "John Doe"},
    }
    
    err = store.Store(ctx, event, nil)
    if err != nil {
        log.Fatal("Failed to store event:", err)
    }
    
    // Load events
    events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
    if err != nil {
        log.Fatal("Failed to load events:", err)
    }
    
    log.Printf("Loaded %d events", len(events))
}
```

### 3. Real-time Subscriptions

```go
package main

import (
    "context"
    "log"
    "os"
    
    "github.com/c0deZ3R0/go-sync-kit/storage/postgres"
)

func main() {
    config := postgres.DefaultConfig("postgres://user:password@localhost/eventstore?sslmode=disable")
    store, err := postgres.NewRealtimeEventStore(config)
    if err != nil {
        log.Fatal("Failed to create realtime store:", err)
    }
    defer store.Close()
    
    ctx := context.Background()
    
    // Subscribe to events for a specific aggregate
    err = store.SubscribeToStream(ctx, "user-123", func(payload postgres.NotificationPayload) error {
        log.Printf("New event for user-123: %s (%s)", payload.EventType, payload.ID)
        return nil
    })
    if err != nil {
        log.Fatal("Failed to subscribe:", err)
    }
    
    // Subscribe to all events
    err = store.SubscribeToAll(ctx, func(payload postgres.NotificationPayload) error {
        log.Printf("Global event: %s for %s", payload.EventType, payload.AggregateID)
        return nil
    })
    if err != nil {
        log.Fatal("Failed to subscribe to all:", err)
    }
    
    // Keep the program running to receive notifications
    select {}
}
```

## Configuration

The `Config` struct provides comprehensive configuration options:

```go
config := &postgres.Config{
    ConnectionString: "postgres://user:password@localhost/eventstore?sslmode=disable",
    
    // Connection Pool Settings
    MaxOpenConns:    25,              // Maximum number of open connections
    MaxIdleConns:    10,              // Maximum number of idle connections
    ConnMaxLifetime: time.Hour,       // Maximum connection lifetime
    ConnMaxIdleTime: 15 * time.Minute, // Maximum idle time before closing
    
    // LISTEN/NOTIFY Settings
    NotificationTimeout:    30 * time.Second, // Timeout for waiting on notifications
    ReconnectInterval:      5 * time.Second,  // Interval between reconnection attempts
    MaxReconnectAttempts:   10,               // Maximum reconnection attempts
    
    // Performance Settings
    BatchSize:           1000, // Batch size for bulk operations
    EnablePreparedStmts: true, // Enable prepared statements
    
    // Monitoring
    Logger:        logger,     // Custom logger instance
    EnableMetrics: true,       // Enable metrics collection
}
```

## Database Schema

The PostgreSQL EventStore uses the following optimized schema:

```sql
CREATE TABLE events (
    version         BIGSERIAL PRIMARY KEY,           -- Auto-incrementing version
    id              TEXT NOT NULL UNIQUE,            -- Event ID
    aggregate_id    TEXT NOT NULL,                   -- Aggregate identifier
    event_type      TEXT NOT NULL,                   -- Event type
    data            JSONB,                          -- Event payload (JSONB for querying)
    metadata        JSONB,                          -- Event metadata (JSONB for querying)
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    stream_name     TEXT GENERATED ALWAYS AS ('stream_' || aggregate_id) STORED
);

-- Optimized indexes
CREATE INDEX idx_events_aggregate_id ON events (aggregate_id);
CREATE INDEX idx_events_version ON events (version);
CREATE INDEX idx_events_created_at ON events (created_at);
CREATE INDEX idx_events_stream_name ON events (stream_name);
CREATE INDEX idx_events_type ON events (event_type);
CREATE INDEX idx_events_data_gin ON events USING GIN (data);
CREATE INDEX idx_events_metadata_gin ON events USING GIN (metadata);
```

## LISTEN/NOTIFY Channels

The implementation automatically creates and manages the following notification channels:

- **Stream Channels**: `stream_{aggregate_id}` - Notifications for specific aggregates
- **Global Channel**: `events_global` - Notifications for all events

### Notification Payload

Each notification contains a JSON payload with event information:

```json
{
    "version": 12345,
    "id": "event-abc-123",
    "aggregate_id": "user-456",
    "event_type": "UserUpdated",
    "stream_name": "stream_user-456",
    "created_at": "2024-01-15T10:30:00Z"
}
```

## Testing

### Prerequisites

Start the test PostgreSQL instance:

```bash
docker-compose -f docker-compose.test.yml up -d postgres
```

### Running Tests

```bash
# Run all tests
go test ./storage/postgres/...

# Run only unit tests (skip integration tests)
go test -short ./storage/postgres/...

# Run with verbose output
go test -v ./storage/postgres/...

# Run benchmarks
go test -bench=. ./storage/postgres/...

# Set custom connection string
POSTGRES_TEST_CONNECTION="postgres://user:pass@host/db" go test ./storage/postgres/...
```

### Integration Tests

The package includes comprehensive integration tests that:

- Test basic EventStore operations (Store, Load, LoadByAggregate)
- Verify real-time notifications work correctly
- Test connection recovery scenarios
- Benchmark performance under load

## Performance Considerations

### Connection Pooling

The EventStore uses connection pooling for optimal performance:

```go
config := postgres.DefaultConfig(connectionString)
config.MaxOpenConns = 25    // Adjust based on your workload
config.MaxIdleConns = 10    // Keep some connections ready
config.ConnMaxLifetime = time.Hour
config.ConnMaxIdleTime = 15 * time.Minute
```

### Prepared Statements

Prepared statements are enabled by default for better performance:

```go
config.EnablePreparedStmts = true
```

### Batch Operations

For high-throughput scenarios, use batch operations:

```go
events := []synckit.EventWithVersion{...}
err := store.StoreBatch(ctx, events)
```

### Indexing

The schema includes optimized indexes for common query patterns:

- B-tree indexes for exact lookups and range queries
- GIN indexes for JSONB data and metadata querying

## Monitoring and Observability

### Database Statistics

Monitor connection pool statistics:

```go
stats := store.Stats()
fmt.Printf("Open connections: %d\n", stats.OpenConnections)
fmt.Printf("In use: %d\n", stats.InUse)
fmt.Printf("Idle: %d\n", stats.Idle)
```

### Logging

Configure comprehensive logging:

```go
logger := log.New(os.Stdout, "[PostgresEventStore] ", log.LstdFlags|log.Lshortfile)
config.Logger = logger
```

### Health Checks

Check listener connectivity:

```go
if store.IsListenerConnected() {
    log.Println("LISTEN/NOTIFY is connected")
} else {
    log.Println("LISTEN/NOTIFY is disconnected")
}
```

## Error Handling

The implementation provides detailed error types:

- `ErrStoreClosed`: Store has been closed
- `ErrIncompatibleVersion`: Version type mismatch
- `ErrEventNotFound`: Event not found
- `ErrInvalidConnection`: Database connection issues

All errors are wrapped with context using the `syncErrors` package.

## Production Deployment

### Connection String

Use a production-grade connection string with appropriate settings:

```
postgres://user:password@host:5432/database?sslmode=require&connect_timeout=10&statement_timeout=30000&idle_in_transaction_session_timeout=60000
```

### Security

- Enable SSL/TLS encryption (`sslmode=require`)
- Use connection pooling appropriately
- Set up proper database permissions
- Consider using connection string from environment variables

### High Availability

- Use PostgreSQL with replication for high availability
- Configure proper backup strategies
- Monitor database health and connection pool metrics
- Set up alerts for notification listener disconnections

### Scaling

- Partition the events table for very large datasets
- Use read replicas for read-heavy workloads
- Consider horizontal scaling with multiple EventStore instances
- Monitor and tune PostgreSQL configuration for your workload

## Migration from SQLite

To migrate from the SQLite EventStore:

1. Export data from SQLite
2. Transform to PostgreSQL format
3. Import into PostgreSQL
4. Update application configuration
5. Test thoroughly with both stores

A migration utility will be provided in future releases.

## Troubleshooting

### Common Issues

1. **Connection refused**: Ensure PostgreSQL is running and accessible
2. **Permission denied**: Check database user permissions
3. **Notifications not received**: Verify LISTEN/NOTIFY setup and firewall settings
4. **High connection count**: Tune connection pool settings
5. **Slow queries**: Check indexes and query plans

### Debug Logging

Enable detailed logging to troubleshoot issues:

```go
logger := log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lshortfile)
config.Logger = logger
```

### Connection Testing

Test database connectivity:

```bash
psql -h localhost -U user -d eventstore -c "SELECT version();"
```

## Contributing

Contributions are welcome! Please:

1. Add tests for new features
2. Update documentation
3. Follow existing code style
4. Test with real PostgreSQL instances

## License

This implementation is part of the go-sync-kit project and follows the same license terms.
