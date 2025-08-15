# PostgreSQL EventStore with LISTEN/NOTIFY Design

## Overview

This document outlines the design and implementation of a PostgreSQL-backed EventStore for the go-sync-kit project that leverages PostgreSQL's LISTEN/NOTIFY mechanism to provide real-time event notifications to subscribers.

## Goals

1. **Transactional Event Persistence**: Provide ACID guarantees for event storage
2. **Real-time Notifications**: Use PostgreSQL LISTEN/NOTIFY for instant event propagation
3. **Stream-based Notifications**: Each stream (aggregate) has its own notification channel
4. **Scalable Architecture**: Support multiple concurrent subscribers
5. **Backward Compatibility**: Maintain compatibility with existing EventStore interface
6. **Production Ready**: Include proper error handling, connection pooling, and monitoring

## Architecture

### Core Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  EventStore     │───▶│  PostgreSQL      │───▶│  LISTEN/NOTIFY  │
│  Interface      │    │  Database        │    │  Channels       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │  Events Table    │    │  Subscribers    │
                       │  - Transactional │    │  - Real-time    │
                       │  - ACID          │    │  - Per-stream   │
                       └──────────────────┘    └─────────────────┘
```

### Database Schema

```sql
-- Events table with enhanced indexing for performance
CREATE TABLE events (
    version         BIGSERIAL PRIMARY KEY,
    id              TEXT NOT NULL UNIQUE,
    aggregate_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    data            JSONB,
    metadata        JSONB,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    stream_name     TEXT GENERATED ALWAYS AS ('stream_' || aggregate_id) STORED
);

-- Indexes for optimal query performance
CREATE INDEX idx_events_aggregate_id ON events (aggregate_id);
CREATE INDEX idx_events_version ON events (version);
CREATE INDEX idx_events_created_at ON events (created_at);
CREATE INDEX idx_events_stream_name ON events (stream_name);
CREATE INDEX idx_events_type ON events (event_type);

-- Trigger function to send notifications
CREATE OR REPLACE FUNCTION notify_event_inserted()
RETURNS TRIGGER AS $$
BEGIN
    -- Notify on the specific stream channel
    PERFORM pg_notify(NEW.stream_name, json_build_object(
        'version', NEW.version,
        'aggregate_id', NEW.aggregate_id,
        'event_type', NEW.event_type,
        'id', NEW.id
    )::text);
    
    -- Notify on the global events channel
    PERFORM pg_notify('events_global', json_build_object(
        'version', NEW.version,
        'aggregate_id', NEW.aggregate_id,
        'event_type', NEW.event_type,
        'id', NEW.id
    )::text);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to fire notifications
CREATE TRIGGER events_notify_trigger
    AFTER INSERT ON events
    FOR EACH ROW
    EXECUTE FUNCTION notify_event_inserted();
```

## Implementation Plan

### Phase 1: Core PostgreSQL EventStore
- [x] Create PostgreSQL package structure
- [ ] Implement basic EventStore interface
- [ ] Add connection pooling and configuration
- [ ] Create database migrations
- [ ] Add comprehensive error handling

### Phase 2: LISTEN/NOTIFY Integration
- [ ] Implement notification listener
- [ ] Add stream-specific subscription management
- [ ] Create subscriber registration/deregistration
- [ ] Handle connection recovery and reconnection

### Phase 3: Real-time Capabilities
- [ ] Extend EventStore with real-time interfaces
- [ ] Add subscription lifecycle management
- [ ] Implement notification filtering
- [ ] Add metrics and monitoring

### Phase 4: Integration & Testing
- [ ] Docker Compose setup for testing
- [ ] Integration tests with real PostgreSQL
- [ ] Performance benchmarks
- [ ] Documentation and examples

## Key Features

### 1. Transactional Append Operations
```go
// Store with transactional guarantees
func (ps *PostgresEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
    tx, err := ps.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Insert event
    _, err = tx.ExecContext(ctx, insertEventQuery, ...)
    if err != nil {
        return err
    }
    
    return tx.Commit() // Triggers notification automatically
}
```

### 2. Stream-based Notifications
```go
// Subscribe to specific aggregate events
func (ps *PostgresEventStore) SubscribeToStream(ctx context.Context, aggregateID string, handler func(Event) error) error {
    channelName := "stream_" + aggregateID
    return ps.listener.Subscribe(channelName, handler)
}
```

### 3. Global Event Subscription
```go
// Subscribe to all events
func (ps *PostgresEventStore) SubscribeToAll(ctx context.Context, handler func(Event) error) error {
    return ps.listener.Subscribe("events_global", handler)
}
```

## Configuration

```go
type PostgresConfig struct {
    // Database connection
    ConnectionString    string
    MaxOpenConns       int
    MaxIdleConns       int
    ConnMaxLifetime    time.Duration
    ConnMaxIdleTime    time.Duration
    
    // LISTEN/NOTIFY settings
    NotificationTimeout time.Duration
    ReconnectInterval   time.Duration
    MaxReconnectAttempts int
    
    // Performance tuning
    BatchSize          int
    EnablePreparedStmts bool
    
    // Monitoring
    EnableMetrics      bool
    Logger            *log.Logger
}
```

## Error Handling Strategy

1. **Connection Failures**: Automatic reconnection with exponential backoff
2. **Transaction Failures**: Proper rollback and error propagation
3. **Notification Failures**: Buffering and retry mechanisms
4. **Subscription Failures**: Graceful degradation and recovery

## Testing Strategy

### Unit Tests
- Mock database interactions
- Test error conditions
- Verify interface compliance

### Integration Tests
- Real PostgreSQL instance (Docker)
- Concurrent operations
- Network partition simulation
- Long-running stability tests

### Docker Compose Setup
```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: eventstore_test
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
    ports:
      - "5432:5432"
    volumes:
      - ./migrations:/docker-entrypoint-initdb.d/
```

## Performance Considerations

1. **Connection Pooling**: Optimize for concurrent operations
2. **Prepared Statements**: Reduce query parsing overhead
3. **Batch Operations**: Support bulk inserts for better throughput
4. **Indexing Strategy**: Optimize for common query patterns
5. **JSONB Usage**: Leverage PostgreSQL's native JSON capabilities

## Security Considerations

1. **SQL Injection Prevention**: Use parameterized queries exclusively
2. **Connection Security**: Support TLS connections
3. **Access Control**: Database-level permissions and roles
4. **Audit Logging**: Track all modifications

## Monitoring and Observability

1. **Metrics Collection**: Connection pool stats, query performance, notification latency
2. **Health Checks**: Database connectivity and listener status
3. **Structured Logging**: Comprehensive operational visibility
4. **Distributed Tracing**: Integration with OpenTelemetry

## Future Enhancements

1. **Partitioning**: Table partitioning for large datasets
2. **Read Replicas**: Scale read operations
3. **Event Sourcing**: Complete event sourcing framework
4. **Streaming**: Apache Kafka integration
5. **Multi-tenant**: Schema-per-tenant support

## Migration Path

For existing SQLite users:
1. Data export/import utilities
2. Schema migration scripts
3. Configuration migration guides
4. Gradual rollout strategies

## Conclusion

This PostgreSQL EventStore implementation will provide a robust, scalable, and real-time event storage solution that maintains compatibility with the existing go-sync-kit architecture while adding powerful new capabilities through PostgreSQL's advanced features.
