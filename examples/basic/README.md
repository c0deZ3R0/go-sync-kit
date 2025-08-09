# Go Sync Kit Basic Example

> **⚠️ Note**: This example is currently being updated to match the latest Go Sync Kit API changes. It may not compile until the updates are complete. For a working example, see the [conflict resolution example](../conflict-resolution/).

This is the foundational example that demonstrates the core features of Go Sync Kit. It implements a real-time notes synchronization application with a live monitoring dashboard.

This example showcases the fundamental capabilities of Go Sync Kit including basic event synchronization, monitoring, and metrics collection. It serves as an excellent starting point for understanding Go Sync Kit before exploring more advanced examples like [conflict resolution](../conflict-resolution/).

![Dashboard Screenshot](dashboard.png)

## Features

- **Real-time Event Synchronization**: Demonstrates bidirectional event syncing between client and server
- **Live Dashboard**: Real-time monitoring of events and system metrics
- **Event Terminal**: Live event log with detailed metadata
- **Metrics Display**: Track sync statistics, event counts, and performance metrics

## Components

- `client/`: Client implementation with auto-sync capabilities
- `server/`: Server implementation with event handling and storage
- `dashboard/`: Real-time web UI for monitoring
- `metrics/`: Metrics collection and reporting
- `demo.go`: Main entry point that runs the example

## Running the Example

1. Start the example:
   ```bash
   go run .
   ```

2. Open the dashboard:
   ```
   http://localhost:8080
   ```

3. Watch real-time events and metrics in the dashboard.

## Dashboard Features

### Event Terminal
- Real-time event log
- Event type and content display
- Detailed metadata including:
  - Event version
  - Priority level
  - Author
  - Hostname
  - Timestamp

### Metrics Panel
- Events pushed/pulled counters
- Sync duration tracking
- Conflict resolution stats
- Error monitoring
- Last sync timestamp

## Architecture

The example demonstrates the basic building blocks of Go Sync Kit:

- SQLite event store for persistence (foundational storage system)
- HTTP transport for sync communication (basic transport implementation)
- Real-time event monitoring (core monitoring capabilities)
- Auto-sync with configurable intervals (basic sync strategy)
- Metrics collection and reporting (fundamental metrics system)

This architecture serves as a foundation for more advanced features that can be built on top, such as:
- Advanced conflict resolution strategies
- Custom storage backends
- Enhanced transport mechanisms
- Complex event filtering and routing
- Advanced monitoring and alerting systems
- Custom sync strategies

## Customization

You can customize various aspects of the example:

- Sync interval (default: 5s)
- Event types and content
- UI layout and styling
- Metrics collection
- Event metadata

## Implementation Details

- Uses SQLite with WAL mode for better concurrency
- Implements event versioning for sync consistency
- Provides real-time metrics updates
- Features responsive dashboard design
- Includes error handling and reporting
## Graceful Shutdown

All commands handle graceful shutdown on SIGINT (Ctrl+C) or SIGTERM signals:
1. Context cancellation is triggered
2. Both client and server perform cleanup
3. Database connections are properly closed
4. Process exits cleanly
