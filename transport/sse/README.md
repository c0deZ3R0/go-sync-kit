# SSE Transport

The SSE (Server-Sent Events) transport package provides a minimal realtime channel to drive RealtimeSyncManager without complicating the HTTP transport.

## Overview

This package implements a cursor-based Server-Sent Events transport for the go-sync-kit. It enables real-time streaming of events from server to client using the standard SSE protocol.

## Features

- **Cursor-based pagination**: Uses the cursor package for efficient event streaming
- **Real-time streaming**: Server pushes events to clients as they become available
- **Minimal overhead**: Lightweight implementation focused on the Subscribe operation
- **Compatible with synckit.Transport**: Implements the Transport interface for Subscribe operation
- **Error handling**: Comprehensive error handling using the kit's error system

## Architecture

The SSE transport consists of two main components:

### Server
- `Server`: HTTP handler that streams events via SSE
- `NewServer()`: Constructor with configurable store and logger
- `Handler()`: Returns HTTP handler for SSE endpoint

### Client  
- `Client`: HTTP client that consumes SSE streams
- `NewClient()`: Constructor with configurable HTTP client
- `Subscribe()`: Implements realtime event subscription

## Usage

### Server Setup

```go
import (
    "log"
    "net/http"
    
    "github.com/c0deZ3R0/go-sync-kit/transport/sse"
    "github.com/c0deZ3R0/go-sync-kit/cursor"
)

// Initialize cursor codecs
cursor.InitDefaultCodecs()

// Create SSE server with your event store
server := sse.NewServer(eventStore, log.Default())

// Set up HTTP endpoint
http.Handle("/sse", server.Handler())
http.ListenAndServe(":8080", nil)
```

### Client Usage

```go
import (
    "context"
    "log"
    
    "github.com/c0deZ3R0/go-sync-kit/transport/sse"
)

// Create SSE client
client := sse.NewClient("http://localhost:8080", nil)

// Subscribe to real-time events
ctx := context.Background()
err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
    for _, event := range events {
        log.Printf("Received event: %s", event.Event.Type())
        // Process event...
    }
    return nil
})
```

### With SyncManager

```go
// Use SSE client as transport for Subscribe operations
// while keeping HTTP transport for Push/Pull operations

httpTransport := httptransport.NewClient("http://localhost:8080", nil)
sseTransport := sse.NewClient("http://localhost:8080", nil)

// Create a composite transport or use them separately
// depending on your RealtimeSyncManager implementation
```

## Protocol Details

### SSE Message Format

Events are streamed as JSON payloads in SSE format:

```
data: {"events": [...], "next_cursor": {"kind": "integer", "data": 123}}

```

### Cursor Parameter

Clients can specify a starting cursor via query parameter:

```
GET /sse?cursor={"kind":"integer","data":123}
```

### Batch Processing

- Server batches events (default: 100 per message)
- Client receives batches and processes them via the handler function
- Next cursor is included in each response for resumption

## Error Handling

The package uses the kit's structured error system:

- `KindInvalid`: Bad cursor format or payload
- `KindInternal`: Server-side errors (store failures)
- `KindMethodNotAllowed`: Unsupported Transport operations (Push/Pull)

## Limitations

This is an MVP implementation with the following constraints:

- **Subscribe-only**: Only implements `Subscribe()` method of Transport interface
- **Single cursor type**: Works best with IntegerCursor and VectorCursor
- **No authentication**: Basic implementation without authentication/authorization
- **No connection pooling**: Each subscription creates a new HTTP connection
- **Simple load strategy**: Uses basic `store.Load()` - adapt for cursor-aware stores

## Integration Notes

- Keep HTTP transport for Push/Pull operations
- Use SSE transport for Subscribe operations in RealtimeSyncManager
- Ensure cursor codecs are initialized before use
- Consider connection limits and resource cleanup for production use

## Future Enhancements

- Authentication/authorization middleware
- Connection pooling and management  
- More sophisticated cursor strategies
- Compression for large payloads
- Reconnection and resume logic
- Metrics and monitoring hooks
