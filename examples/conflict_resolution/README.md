# Conflict Resolution Demo

This example demonstrates the conflict resolution capabilities of go-sync-kit using a distributed counter application.

## Components

- **Server**: Central synchronization server with SQLite storage
- **Client**: Multiple clients that can work offline
- **Dashboard**: Web UI for visualizing sync state
- **Counter Events**: Simple counter operations (create, increment, decrement, reset)
- **Conflict Resolution**: Merge-based strategy for concurrent updates

## Running the Demo

1. Start the server:
```bash
go run main.go -mode server -port 8080
```

2. Start client 1:
```bash
go run main.go -mode client -id client1 -port 8081
```

3. Start client 2:
```bash
go run main.go -mode client -id client2 -port 8082
```

4. Start the dashboard:
```bash
go run main.go -mode dashboard -port 8090
```

## Test Scenarios

### Basic Synchronization
1. Create counter on client 1
2. Increment on client 2
3. Verify sync works

### Offline Operation
1. Disconnect both clients
2. Make changes on each
3. Reconnect and observe sync

### Conflict Resolution
1. Disconnect clients
2. Update same counter differently
3. Reconnect and observe merge

## Directory Structure

```
conflict_resolution/
├── main.go           # Main entry point
├── counter.go        # Counter event types
├── resolver.go       # Conflict resolution
├── server/          
│   └── server.go     # Server implementation
├── client/          
│   └── client.go     # Client implementation (TODO)
├── dashboard/       
│   └── templates/    # Dashboard UI (TODO)
└── scripts/         
    └── test_server.sh # Test scripts
```

## Implementation Details

### Counter Events
- `CounterCreated`: Initialize a counter
- `CounterIncremented`: Add to counter value
- `CounterDecremented`: Subtract from counter value
- `CounterReset`: Set counter to specific value

### Conflict Resolution Strategy
The conflict resolver:
1. Combines events from all sources
2. Orders them by timestamp
3. Applies operations in sequence
4. Creates a final state event if needed

### Storage
Uses SQLite with WAL mode for:
- Durability
- Concurrent access
- Offline capability
