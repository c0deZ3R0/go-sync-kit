# Sync Kit Example

This example demonstrates how to use the Sync Kit library to create a simple client-server application with real-time synchronization.

## Structure

```
example/
├── cmd/
│   ├── client/     # Client binary
│   ├── demo/       # Combined client-server demo
│   └── server/     # Server binary
├── client/         # Client implementation
├── server/         # Server implementation
└── metrics_impl.go # Common metrics implementation
```

## Running the Example

You have three options to run the example:

1. Run both client and server together (recommended for testing):
```bash
go run cmd/demo/main.go
```

2. Run server and client separately (in different terminals):
```bash
# Terminal 1 - Start the server
go run cmd/server/main.go

# Terminal 2 - Start the client
go run cmd/client/main.go
```

## Implementation Details

- The server maintains a SQLite database (`notes.db`) for persistent storage
- The client maintains its own SQLite database (`notes_client.db`) for local state
- Both use WAL (Write-Ahead Logging) mode for better concurrent access
- Real-time synchronization is implemented using websockets
- Changes are versioned using vector clocks

## Graceful Shutdown

All commands handle graceful shutdown on SIGINT (Ctrl+C) or SIGTERM signals:
1. Context cancellation is triggered
2. Both client and server perform cleanup
3. Database connections are properly closed
4. Process exits cleanly
