# Server-Side Projection Hooks Example

This example demonstrates **Phase 5** of the Read-Model Projections implementation: Server-side projection hooks in the HTTP transport.

## Overview

Server-side projection hooks enable server read models that are built from only server-committed events. This ensures consistency and prevents clients from polluting server projections with invalid or unauthorized events.

## Features

- **AfterCommit Hook**: Called asynchronously after events are successfully committed to storage
- **BeforePull Hook**: Called before pulling events (useful for metrics, logging, etc.)
- **Real-time Projections**: Server read models are updated immediately as events are committed
- **Non-blocking**: Hooks run asynchronously to avoid blocking HTTP responses
- **Error Isolation**: Hook failures don't affect the sync operation

## Architecture

```
Client Push Request → Event Storage → AfterCommit Hook → Projection Update → Read Model
                                   ↘️ HTTP Response (non-blocking)
```

## Running the Example

1. **Start the server:**
   ```bash
   go run main.go
   ```

2. **The server will start on :8080 with these endpoints:**
   - `POST /sync/push` - Push events (triggers AfterCommit hook)
   - `GET  /sync/pull?since=<version>` - Pull events (triggers BeforePull hook)  
   - `GET  /stats/users` - Get current user count from read model
   - `GET  /health` - Health check

## Testing the Hooks

### 1. Check Initial State
```bash
curl http://localhost:8080/health
curl http://localhost:8080/stats/users
```

### 2. Push Some Events
You need to create mock events with the correct wire format. Here's an example using a hypothetical client:

```json
POST /sync/push
Content-Type: application/json

[
  {
    "event": {
      "id": "user-1",
      "type": "UserCreated", 
      "data": "{\"name\":\"John Doe\"}",
      "timestamp": "2024-01-01T00:00:00Z"
    },
    "version": "1"
  },
  {
    "event": {
      "id": "user-2", 
      "type": "UserCreated",
      "data": "{\"name\":\"Jane Smith\"}",
      "timestamp": "2024-01-01T00:01:00Z"
    },
    "version": "2"
  }
]
```

### 3. Check Updated Read Model
```bash
curl http://localhost:8080/stats/users
# Should show: {"user_count": 2}
```

### 4. Pull Events (triggers BeforePull hook)
```bash
curl "http://localhost:8080/sync/pull?since=0"
```

## Key Components

### SyncHooks
```go
type SyncHooks struct {
    // AfterCommit is called after events are successfully committed to storage
    AfterCommit func(ctx context.Context, committed []synckit.EventWithVersion)
    
    // BeforePull is called before pulling events (for metrics, etc.)
    BeforePull func(ctx context.Context, since synckit.Version)
}
```

### UserCountProjector
A simple projector that maintains a user count in a SQLite read model:
- Creates/maintains `user_stats` table
- Increments count on `UserCreated` events
- Decrements count on `UserDeleted` events
- Exposes `getUserCount()` method for queries

### Projection Runner
Handles the mechanics of applying events to projections:
- Batch processing with configurable size
- Offset management via BadgerDB
- Error handling and logging
- Idempotent operations

## Hook Workflow

1. **Client sends events** via `POST /sync/push`
2. **Server validates and stores** events in event store
3. **HTTP response sent immediately** (non-blocking)
4. **AfterCommit hook triggered** asynchronously with successfully committed events
5. **Projection runner applies** events to read model
6. **Read model updated** and available for queries

## Benefits

- **Consistency**: Only server-committed events affect server read models
- **Performance**: Hooks are asynchronous and don't block responses  
- **Reliability**: Hook failures are isolated from sync operations
- **Real-time**: Read models are updated as soon as events are committed
- **Flexibility**: Both AfterCommit and BeforePull hooks for different use cases

## Error Handling

- Hook failures are logged but don't affect the sync operation
- Projections use timeouts to prevent hanging
- Failed projections can be retried (implementation dependent)
- Offset management ensures exactly-once processing

## Next Steps

This example can be extended with:
- Multiple projectors running concurrently
- More sophisticated read models (denormalized views, aggregates)
- External projections (Elasticsearch, Redis, etc.)
- Projection snapshots for large datasets
- Metrics and monitoring integration
