# Go Sync Kit

A generic, event-driven synchronization library for distributed Go applications. Go Sync Kit enables offline-first architectures with conflict resolution and pluggable storage backends.

## 🤖 Project Origins & Collaboration

**Transparency First**: This project was created with the assistance of Large Language Models (LLMs) as a starting point for a collaborative open-source initiative. 

**Why I'm Sharing This**: I'm passionate about learning Go and improving my programming skills. Rather than keeping this as a solo project, I'm open-sourcing it to:

- **Learn from the community** - Your code reviews, suggestions, and contributions will help me grow as a developer
- **Collaborate together** - Let's build something useful while learning from each other
- **Test the concept** - See if there's genuine interest in this approach to synchronization in Go
- **Practice open-source** - Experience the full cycle of maintaining and growing an open-source project

If you're interested in **mentoring**, **contributing**, or **learning alongside me**, I'd love to hear from you! Whether you're a Go expert or fellow learner, all perspectives are valuable.

> 💡 **Note**: While LLMs helped with the initial implementation, all future development will be driven by real-world needs, community feedback, and collaborative human insight.

## Features

### Core Features
- **Event-Driven Architecture**: Built around append-only event streams
- **Offline-First**: Full support for offline operation with automatic sync when reconnected
- **Pluggable Components**: Interfaces for storage, transport, versioning, and conflict resolution
- **Conflict Resolution**: Multiple strategies including last-write-wins, merge, and custom resolvers
- **Concurrent Safe**: Thread-safe operations with proper synchronization

### Transport & Storage
- **Transport Agnostic**: Works with HTTP, gRPC, WebSockets, NATS, or any custom transport
- **Storage Agnostic**: Compatible with SQLite, BadgerDB, PostgreSQL, or any storage backend
- **Advanced Vector Clocks**: Enhanced vector clock implementation with validation and safety limits
- **Context Aware**: Full context support with timeouts and cancellation for all operations

### Performance & Reliability
- **Improved Error Handling**: Enhanced error system with error codes and metadata
- **Metrics Collection**: Built-in metrics tracking for sync operations and performance monitoring
- **Automatic Sync**: Configurable periodic synchronization with exponential backoff
- **Efficient Batching**: Optimized batch processing for large event sets
- **Comprehensive Testing**: Over 90% test coverage with context and race condition testing

### Configuration & Safety
- **Builder Pattern**: Enhanced builder with validation, timeouts, and compression options
- **Filtering**: Sync only specific events based on custom filters
- **Safety Limits**: Configurable limits to prevent resource exhaustion
- **Validation Options**: Built-in validation for sync parameters and configurations

## Installation

```bash
go get github.com/c0deZ3R0/go-sync-kit
```

## Examples

Go Sync Kit provides a comprehensive example suite that demonstrates real-world usage patterns, from basic concepts to advanced techniques.

### 📚 Example Structure

```
examples/
├── quickstart/           # Get started quickly
│   ├── local-only/       # Local synchronization basics
│   └── http-client/      # HTTP client-server sync
└── intermediate/         # Advanced concepts
    ├── 03-events-and-storage/    # Event persistence with SQLite
    ├── 04-conflict-resolution/   # Multiple resolution strategies
    ├── 05-realtime-autosync/     # Background sync patterns
    ├── 06-custom-events-filters/ # Domain-specific filtering
    └── 07-structured-logging/    # Production logging patterns
```

### 🚀 Quickstart Examples

**Local-Only Synchronization**
```bash
cd examples/quickstart/local-only
go run main.go
```
Demonstrates basic event creation, storage, and local sync operations using SQLite.

**HTTP Client-Server Sync**
```bash
cd examples/quickstart/http-client
go run main.go
```
Shows client-server synchronization over HTTP with proper error handling.

### 🎯 Intermediate Examples

**Example 3: Events and Storage**
- Custom event types with proper interface implementation
- SQLite integration and event persistence
- Version handling and aggregate loading
- Comprehensive error handling

**Example 4: Conflict Resolution**
- Last-Write-Wins (LWW) strategy
- First-Write-Wins (FWW) strategy  
- Additive Merge strategy
- Custom conflict resolver implementation
- Multi-client conflict simulation

**Example 5: Real-time Auto Sync**
- Background synchronization with timers
- Graceful shutdown handling
- Sync statistics and monitoring
- Multi-client scenarios with different intervals

**Example 6: Custom Events and Filters**
- Domain-specific event types (User, Product, Order)
- Metadata-based filtering
- Priority and tenant-based sync rules
- Performance optimization through selective sync

### 🏃‍♂️ Running the Examples

```bash
# Clone the repository
git clone https://github.com/c0deZ3R0/go-sync-kit
cd go-sync-kit

# Run any example
cd examples/quickstart/local-only
go run main.go

# Or try an intermediate example
cd examples/intermediate/04-conflict-resolution
go run main.go
```

Each example is self-contained with its own `go.mod` and includes detailed comments explaining the concepts and implementation patterns.

## Quick Start

### ⚡ Functional Options API (New & Recommended)

Go Sync Kit now provides a simplified functional options API that makes configuration cleaner and more discoverable:

```go
// Create a manager with functional options
manager, err := synckit.NewManager(
    synckit.WithStore(store),               // SQLite, BadgerDB, etc.
    synckit.WithNullTransport(),           // Local-only (no network)
    synckit.WithLWW(),                     // Last-Write-Wins conflict resolution
    synckit.WithBatchSize(100),            // Batch size for sync operations
    synckit.WithTimeout(30*time.Second),   // Operation timeout
    synckit.WithFilter(myEventFilter),     // Custom event filter
)
```

**Available Options:**
- `WithStore(store)` - Set the event store (SQLite, BadgerDB, etc.)
- `WithTransport(transport)` - Set network transport (HTTP, gRPC, etc.)
- `WithNullTransport()` - Use null transport for local-only scenarios
- `WithLWW()` - Last-Write-Wins conflict resolution
- `WithFWW()` - First-Write-Wins conflict resolution  
- `WithAdditiveMerge()` - Additive merge conflict resolution
- `WithConflictResolver(resolver)` - Custom conflict resolver
- `WithFilter(filter)` - Event filtering function
- `WithBatchSize(size)` - Sync batch size
- `WithTimeout(duration)` - Operation timeout
- `WithValidation()` - Enable additional validation
- `WithCompression(enabled)` - Enable/disable compression
- `WithPushOnly()` - Push-only synchronization
- `WithPullOnly()` - Pull-only synchronization

### Using Functional Options (Recommended)

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
    "github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
    // Create SQLite store
    store, err := sqlite.NewWithDataSource("app.db")
    if err != nil {
        log.Fatalf("Failed to create SQLite store: %v", err)
    }
    defer store.Close()

    // Create HTTP transport
    transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

    mgr, err := synckit.NewManager(
        synckit.WithStore(store),
        synckit.WithTransport(transport),
        synckit.WithLWW(),
        synckit.WithBatchSize(100),
        synckit.WithTimeout(30*time.Second),
    )
    if err != nil {
        log.Fatalf("Failed to create manager: %v", err)
    }

    if err := mgr.Sync(context.Background()); err != nil {
        log.Fatalf("Sync failed: %v", err)
    }
    
    log.Println("Sync completed successfully!")
}
```

### Using Builder Pattern

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
    "github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

type MyEvent struct {
    id          string
    eventType   string
    aggregateID string
    data        interface{}
    metadata    map[string]interface{}
}

func (e *MyEvent) ID() string { return e.id }
func (e *MyEvent) Type() string { return e.eventType }
func (e *MyEvent) AggregateID() string { return e.aggregateID }
func (e *MyEvent) Data() interface{} { return e.data }
func (e *MyEvent) Metadata() map[string]interface{} { return e.metadata }

func main() {
    // Create SQLite Event Store
    storeConfig := &sqlite.Config{DataSourceName: "file:events.db", EnableWAL: true}
    store, err := sqlite.New(storeConfig)
    if err != nil {
        log.Fatalf("Failed to create SQLite store: %v", err)
    }
    defer store.Close()

    // Set up HTTP server with SyncHandler
    logger := log.New(os.Stdout, "[SyncHandler] ", log.LstdFlags)
    // Use default version parser (store.ParseVersion)
    handler := httptransport.NewSyncHandler(store, logger, nil, nil)
    server := &http.Server{Addr: ":8080", Handler: handler}
	
    go func() {
        if err := server.ListenAndServe(); err != nil {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()

    // Set up HTTP Client with HTTPTransport
    clientTransport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

    // Configure Sync Options
    syncOptions := &synckit.SyncOptions{
        BatchSize: 10,
        SyncInterval: 10 * time.Second,
    }

    // Create and start SyncManager
    syncManager, err := synckit.NewManager(
        synckit.WithStore(store),
        synckit.WithTransport(clientTransport),
        synckit.WithLWW(),
        synckit.WithBatchSize(syncOptions.BatchSize),
        synckit.WithSyncInterval(syncOptions.SyncInterval),
    )
    if err != nil {
        log.Fatalf("Failed to create sync manager: %v", err)
    }
    ctx := context.Background()

    // Run synchronization
    result, err := syncManager.Sync(ctx)
    if err != nil {
        log.Fatalf("Sync error: %v", err)
    }
    log.Printf("Sync completed: %+v", result)
}
```

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/c0deZ3R0/go-sync-kit"
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
)

// Implement your event type
type MyEvent struct {
    id          string
    eventType   string
    aggregateID string
    data        interface{}
    metadata    map[string]interface{}
}

func (e *MyEvent) ID() string { return e.id }
func (e *MyEvent) Type() string { return e.eventType }
func (e *MyEvent) AggregateID() string { return e.aggregateID }
func (e *MyEvent) Data() interface{} { return e.data }
func (e *MyEvent) Metadata() map[string]interface{} { return e.metadata }

func main() {
    // Create an SQLite event store
    logger := log.New(os.Stdout, "[SQLite EventStore] ", log.LstdFlags)
    config := &sqlite.Config{
        DataSourceName: "file:events.db",
        Logger:         logger,
        EnableWAL:      true,  // Enable WAL for better concurrency
    }

    store, err := sqlite.New(config)
    if err != nil {
        log.Fatalf("Failed to create SQLite store: %v", err)
    }
    defer store.Close()

    // Create a transport
    transport := &MyTransport{} // Your Transport implementation

    // Configure sync options
    options := &synckit.SyncOptions{
        BatchSize:        100,
        SyncInterval:     30 * time.Second,
        ConflictResolver: &LastWriteWinsResolver{},
    }

    // Create sync manager  
    syncManager, err := synckit.NewManager(
        synckit.WithStore(store),
        synckit.WithTransport(transport),
        synckit.WithConflictResolver(options.ConflictResolver),
        synckit.WithBatchSize(options.BatchSize),
        synckit.WithSyncInterval(options.SyncInterval),
    )
    if err != nil {
        log.Fatalf("Failed to create sync manager: %v", err)
    }

    // Perform sync
    ctx := context.Background()
    result, err := syncManager.Sync(ctx)
    if err != nil {
        log.Fatalf("Sync failed: %v", err)
    }

    fmt.Printf("Synced: %d pushed, %d pulled\n", 
        result.EventsPushed, result.EventsPulled)
}
```


## Release Information

See [CHANGELOG.md](CHANGELOG.md) for detailed release notes and version history.

Latest version: **v0.10.0** - [What's new in v0.10.0](CHANGELOG.md#v0100) - Real-time SSE transport for live event streaming with cursor-based pagination and hybrid transport support.

## Architecture
Go Sync Kit follows clean architecture principles with clear separation of concerns:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   SyncManager   │    │    Transport    │
│                 │───▶│                 │───▶│                 │
│  (Your Code)    │    │  (Coordination) │    │   (Network)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                               │
                               ▼
                    ┌─────────────────┐    ┌─────────────────┐
                    │   EventStore    │    │ ConflictResolver │
                    │                 │    │                 │
                    │   (Storage)     │    │  (Resolution)   │
                    └─────────────────┘    └─────────────────┘
```

## Core Interfaces

### Event
Represents a syncable event in your system:

```go
type Event interface {
    ID() string
    Type() string
    AggregateID() string
    Data() interface{}
    Metadata() map[string]interface{}
}
```

### Version
Handles versioning for sync operations:

```go
type Version interface {
    Compare(other Version) int
    String() string
    IsZero() bool
}
```

### EventStore
Provides persistence for events:

```go
type EventStore interface {
    Store(ctx context.Context, event Event, version Version) error
    Load(ctx context.Context, since Version) ([]EventWithVersion, error)
    LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error)
    LatestVersion(ctx context.Context) (Version, error)
    Close() error
}
```

### Transport
Handles network communication:

```go
type Transport interface {
    Push(ctx context.Context, events []EventWithVersion) error
    Pull(ctx context.Context, since Version) ([]EventWithVersion, error)
    Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error
    Close() error
}
```

### ConflictResolver
Resolves conflicts when the same data is modified concurrently:

```go
type ConflictResolver interface {
    Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error)
}
```

## Conflict Resolution Strategies

Go Sync Kit supports multiple conflict resolution strategies:

### Last-Write-Wins
```go
type LastWriteWinsResolver struct{}

func (r *LastWriteWinsResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
    // Keep the events with the latest timestamp
    var resolved []EventWithVersion
    // Implementation logic here...
    return resolved, nil
}
```

### Custom Merge Strategy
```go
type CustomMergeResolver struct{}

func (r *CustomMergeResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
    // Implement your custom merge logic
    // Could merge data fields, prompt user, etc.
    return mergedEvents, nil
}
```

## Configuration Options

```go
type SyncOptions struct {
    // Sync direction control
    PushOnly bool
    PullOnly bool
    
    // Conflict handling
    ConflictResolver ConflictResolver
    
    // Event filtering
    Filter func(Event) bool
    
    // Performance tuning
    BatchSize int
    SyncInterval time.Duration
}
```

### Example with filtering:
```go
options := &synckit.SyncOptions{
    BatchSize: 50,
    Filter: func(e synckit.Event) bool {
        // Only sync specific event types
        return e.Type() == "UserCreated" || e.Type() == "OrderPlaced"
    },
}
```

## Versioning Strategies

Go Sync Kit supports multiple versioning strategies suitable for different architectures.

### Vector Clocks (Recommended for Distributed Systems)

For multi-master, peer-to-peer, or offline-first scenarios where writes can happen on multiple nodes concurrently, using a vector clock is the recommended approach. The library provides a `VersionedStore` decorator that manages versioning logic automatically.

**Key Benefits:**
- **Causal ordering**: Determines if events happened-before, happened-after, or are concurrent
- **Conflict detection**: Automatically identifies conflicting concurrent writes
- **Distributed-friendly**: No central coordination required
- **Offline-first**: Works perfectly for disconnected clients

**Usage:**

```go
import (
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    "github.com/c0deZ3R0/go-sync-kit/version"
    sync "github.com/c0deZ3R0/go-sync-kit"
)

// 1. Create a base store (e.g., SQLite)
storeConfig := &sqlite.Config{DataSourceName: "file:events.db", EnableWAL: true}
baseStore, err := sqlite.New(storeConfig)
if err != nil {
    // handle error
}

// 2. Define a unique ID for the current node
nodeID := "client-A"

// 3. Create a vector clock version manager
versionManager := version.NewVectorClockManager()

// 4. Wrap the base store with the VersionedStore decorator
versionedStore, err := version.NewVersionedStore(baseStore, nodeID, versionManager)
if err != nil {
    // handle error
}

// 5. Use the decorated store. It now handles vector clock versioning automatically.
syncManager := sync.NewSyncManager(versionedStore, transport, options)

// When you store an event, the version is managed automatically
err = versionedStore.Store(ctx, myNewEvent, nil) // nil means auto-generate version
```

**Real-world Example:**

```go
// Node A creates an event
nodeAStore.Store(ctx, userCreatedEvent, nil)
// Result: {"A": 1}

// Node B creates an event independently  
nodeBStore.Store(ctx, orderPlacedEvent, nil)
// Result: {"B": 1}

// When nodes sync, vector clocks detect concurrent operations
// and enable proper conflict resolution
```

### Simple Versioning (Default)

For single-master or centralized scenarios, you can use simpler versioning strategies like timestamps or sequential IDs. The underlying storage implementations (like SQLite) handle this automatically.

```go
// SQLite store uses timestamp-based versioning by default
store, err := sqlite.New(config)
// No decorator needed - works out of the box
```

### Custom Versioning Strategies

You can implement your own versioning strategy by implementing the `VersionManager` interface:

```go
type CustomVersionManager struct {
    // Your custom state
}

func (vm *CustomVersionManager) CurrentVersion() synckit.Version
    // Return current version
}

func (vm *CustomVersionManager) NextVersion(nodeID string) synckit.Version
    // Generate next version
}

func (vm *CustomVersionManager) UpdateFromVersion(version sync.Version) error {
    // Update internal state from observed version
}

func (vm *CustomVersionManager) Clone() VersionManager {
    // Create a copy
}
```

## Storage Implementations

### SQLite Implementation (Production-Ready)

The built-in SQLite implementation comes with production-ready defaults optimized for performance and reliability:

```go
import "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"

// Basic usage with production defaults
store, err := sqlite.New(&sqlite.Config{
    DataSourceName: "file:events.db",
    // WAL mode is enabled by default for better concurrency
    // Connection pool defaults: max open 25, max idle 5
    // Connection lifetimes: 1 hour max, 5 minutes max idle
})
```

**Production Features:**
- **WAL Mode**: Enabled by default for better read/write concurrency
- **Connection Pool**: Sensible defaults (max open: 25, max idle: 5)
- **Connection Management**: Automatic connection lifetime management
- **Table Schema**: Optimized event storage with proper indexing
- **Thread Safety**: Full concurrent access support

**Custom Configuration:**
```go
config := &sqlite.Config{
    DataSourceName:      "file:production.db",
    EnableWAL:           true,  // Default: true
    MaxOpenConns:        50,    // Default: 25
    MaxIdleConns:        10,    // Default: 5
    ConnMaxLifetime:     2 * time.Hour,     // Default: 1 hour
    ConnMaxIdleTime:     10 * time.Minute,  // Default: 5 minutes
    TableName:           "my_events",       // Default: "events"
    Logger:              myLogger,          // Default: discard
}

store, err := sqlite.New(config)
```

**Integration Tests:**
The SQLite implementation includes comprehensive integration tests covering WAL behavior, concurrent writes, and production scenarios.

### BadgerDB Example
```go
type BadgerEventStore struct {
    db *badger.DB
}

func (b *BadgerEventStore) Store(ctx context.Context, event Event, version Version) error {
    return b.db.Update(func(txn *badger.Txn) error {
        key := []byte(fmt.Sprintf("event:%s", event.ID()))
        eventData := EventWithVersion{Event: event, Version: version}
        data, err := json.Marshal(eventData)
        if err != nil {
            return err
        }
        return txn.Set(key, data)
    })
}
```

## Transport Implementations

### Built-in HTTP Transport

Go Sync Kit includes a production-ready HTTP transport implementation that provides both client and server components.

### Built-in SSE Transport (Real-time)

For real-time event streaming, Go Sync Kit provides a Server-Sent Events (SSE) transport that enables live synchronization without polling.

**Key Features:**
- **Real-time streaming** using standard SSE protocol
- **Cursor-based pagination** for efficient, resumable streaming  
- **Subscribe-only MVP** - designed for real-time event consumption
- **Hybrid usage** - combine with HTTP transport (HTTP for Push/Pull, SSE for Subscribe)
- **JSON wire format** for cross-platform compatibility

#### SSE Server Setup
```go
import "github.com/c0deZ3R0/go-sync-kit/transport/sse"

// Create SSE server
server := sse.NewServer(eventStore, logger)

// Mount SSE endpoint
http.Handle("/sse", server.Handler())
http.ListenAndServe(":8080", nil)
```

#### SSE Client Setup
```go
import "github.com/c0deZ3R0/go-sync-kit/transport/sse"

// Create SSE client
client := sse.NewClient("http://localhost:8080", nil)

// Subscribe to real-time events
err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
    // Process real-time events as they arrive
    for _, event := range events {
        log.Printf("Received: %s - %s", event.Event.Type(), event.Event.ID())
    }
    return nil
})
```

#### Hybrid Transport Usage
```go
// Use HTTP transport for Push/Pull operations
httpTransport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

// Use SSE transport for real-time Subscribe operations  
sseTransport := sse.NewClient("http://localhost:8080", nil)

// Create SyncManager with HTTP transport (SSE used separately for real-time)
syncManager, err := synckit.NewManager(
    synckit.WithStore(store),
    synckit.WithTransport(httpTransport),
    synckit.WithLWW(),  // or your preferred conflict resolution
)
if err != nil {
    log.Fatalf("Failed to create sync manager: %v", err)
}

// Run periodic sync via HTTP
result, err := syncManager.Sync(ctx)

// Run real-time subscription via SSE
go sseTransport.Subscribe(ctx, eventHandler)
```

#### Client Setup
```go
import "github.com/c0deZ3R0/go-sync-kit/transport/httptransport"

// Create HTTP transport client
clientTransport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

// Use with SyncManager
syncManager, err := synckit.NewManager(
    synckit.WithStore(store),
    synckit.WithTransport(clientTransport),
    synckit.WithLWW(),  // or your preferred options
)
if err != nil {
    log.Fatalf("Failed to create sync manager: %v", err)
}
```

#### Server Setup
```go
import "github.com/c0deZ3R0/go-sync-kit/transport/httptransport"

// Create HTTP sync handler
logger := log.New(os.Stdout, "[SyncHandler] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := httptransport.NewSyncHandler(store, logger, nil, nil)

// Start HTTP server
server := &http.Server{Addr: ":8080", Handler: handler}
go server.ListenAndServe()
```

#### API Endpoints

The HTTP transport provides two RESTful endpoints:

- **POST /push** - Accepts events to be stored on the server
- **GET /pull?since=<version>** - Returns events since the specified version

#### Features

- **JSON serialization** with proper interface handling
- **Context cancellation** support
- **Comprehensive error handling** with HTTP status codes
- **Storage-agnostic** server implementation
- **Configurable HTTP client** for custom timeouts, TLS, etc.
- **Batch processing** for efficient sync operations
- **Flexible version parsing** with custom version parser support
- **Compression support** - Automatic gzip compression for large payloads
- **Security hardening** - Protection against zip bombs and size-based attacks
- **Content validation** - Strict Content-Type validation and error mapping
- **Configurable limits** - Request/response size limits with separate compression controls

#### Version Parsing

Both client and server support custom version parsing through an injectable `VersionParser` function:

```go
// Define a custom parser that requires a 'v' prefix
customParser := func(ctx context.Context, s string) (synckit.Version, error) {
    if !strings.HasPrefix(s, "v") {
        return nil, fmt.Errorf("version must start with 'v'")
    }
    // Strip 'v' prefix and parse as integer
    seq, err := strconv.ParseUint(s[1:], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid version number: %w", err)
    }
    return cursor.IntegerCursor{Seq: seq}, nil
}

// Use custom parser in client transport
clientTransport := httptransport.NewTransport("http://localhost:8080", nil, customParser, nil)

// Use same parser in server handler for consistent version parsing
handler := httptransport.NewSyncHandler(store, logger, customParser, nil)
```

If no parser is provided, the transport falls back to using the store's `ParseVersion` method:

#### Complete Client/Server Example
```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
synckit "github.com/c0deZ3R0/go-sync-kit"
"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
    // 1. Create SQLite store
    store, err := sqlite.NewWithDataSource("file:events.db")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // 2. Start HTTP server
    logger := log.New(os.Stdout, "[SyncHandler] ", log.LstdFlags)
// Use default version parser (store.ParseVersion)
handler := httptransport.NewSyncHandler(store, logger, nil, nil)
    server := &http.Server{Addr: ":8080", Handler: handler}
    
    go func() {
        log.Println("Starting sync server on :8080")
        if err := server.ListenAndServe(); err != nil {
            log.Printf("Server error: %v", err)
        }
    }()

    // Give server time to start
    time.Sleep(100 * time.Millisecond)

    // 3. Create HTTP client transport
    clientTransport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

    // 4. Create sync manager using functional options
    syncManager, err := synckit.NewManager(
        synckit.WithStore(store),
        synckit.WithTransport(clientTransport),
        synckit.WithLWW(),
        synckit.WithBatchSize(10),
        synckit.WithSyncInterval(10*time.Second),
    )
    if err != nil {
        log.Fatalf("Failed to create sync manager: %v", err)
    }

    // 6. Perform synchronization
    ctx := context.Background()
    result, err := syncManager.Sync(ctx)
    if err != nil {
        log.Fatalf("Sync failed: %v", err)
    }

    log.Printf("Sync completed: %d pushed, %d pulled", 
        result.EventsPushed, result.EventsPulled)
}
```

### Custom HTTP Transport Example
```go
type CustomHTTPTransport struct {
    client  *http.Client
    baseURL string
}

func (h *CustomHTTPTransport) Push(ctx context.Context, events []EventWithVersion) error {
    data, err := json.Marshal(events)
    if err != nil {
        return err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", 
        h.baseURL+"/custom/push", bytes.NewBuffer(data))
    if err != nil {
        return err
    }
    
    resp, err := h.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### NATS Transport Example (with Watermill)
```go
type NATSTransport struct {
    publisher  message.Publisher
    subscriber message.Subscriber
}

func (n *NATSTransport) Push(ctx context.Context, events []EventWithVersion) error {
    for _, event := range events {
        data, err := json.Marshal(event)
        if err != nil {
            return err
        }
        
        msg := message.NewMessage(watermill.NewUUID(), data)
        err = n.publisher.Publish("sync.events", msg)
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Advanced Usage

### Automatic Sync
```go
// Start automatic sync every 30 seconds
ctx := context.Background()
err := syncManager.StartAutoSync(ctx)
if err != nil {
    log.Fatal(err)
}

// Stop automatic sync
err = syncManager.StopAutoSync()
```

### Event Subscriptions
```go
err := syncManager.Subscribe(func(result *sync.SyncResult) {
    log.Printf("Sync completed: %d events pushed, %d pulled, %d conflicts resolved",
        result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
    
    if len(result.Errors) > 0 {
        log.Printf("Sync errors: %v", result.Errors)
    }
})
```

### Manual Push/Pull
```go
// Push only local changes
result, err := syncManager.Push(ctx)

// Pull only remote changes  
result, err := syncManager.Pull(ctx)
```

## Testing

Go Sync Kit is designed for testability with mock implementations included:

```go
func TestMySync(t *testing.T) {
    store := &MockEventStore{}
    transport := &MockTransport{}
    resolver := &MockConflictResolver{}
    
    sm := sync.NewSyncManager(store, transport, &sync.SyncOptions{
        ConflictResolver: resolver,
    })
    
    // Test your sync logic
    result, err := sm.Sync(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, 1, result.EventsPushed)
}
```

Run tests:
```bash
go test ./...
```

## Performance Considerations

- **Batching**: Use appropriate batch sizes for your network conditions
- **Filtering**: Apply filters to reduce sync overhead
- **Storage**: Choose storage backends appropriate for your scale
- **Conflict Resolution**: Simple strategies (like last-write-wins) are faster than complex merging

## Roadmap

### Completed ✅
- [x] **Enhanced Context Support** - Comprehensive context handling with timeouts and cancellation
- [x] **Advanced Vector Clocks** - Complete implementation with validation and safety limits
- [x] **SQLite EventStore** - Production-ready SQLite implementation with WAL support
- [x] **Vector Clock Versioning** - Complete implementation with VersionedStore decorator
- [x] **HTTP Transport** - Production-ready HTTP transport with context support
- [x] **SSE Transport** - Real-time Server-Sent Events transport for live event streaming
- [x] **Metrics Collection** - Built-in metrics tracking for sync operations
- [x] **Error System** - Enhanced error handling with codes and metadata
- [x] **Builder Pattern** - Improved configuration with validation
- [x] **BadgerDB Store** - Production-ready BadgerDB implementation with atomic operations

### Next Up 🚀
- [ ] **Storage Implementations**
  - [ ] PostgreSQL store with LISTEN/NOTIFY
  - [ ] Redis store with pub/sub support
- [ ] **Transport Layer**
  - [ ] gRPC transport with streaming
  - [ ] WebSocket transport for real-time sync
  - [ ] NATS transport for event streaming
- [ ] **Performance**
  - [ ] Compression support for large payloads
  - [ ] Connection pooling for databases
  - [ ] Batch operation optimizations

### Future Plans 🔮
- [ ] **Schema Evolution** - Support for data model changes
- [ ] **GraphQL Transport** - Support for GraphQL subscriptions
- [ ] **Observability** - OpenTelemetry integration
- [ ] **Security** - Built-in encryption and access control
- [ ] **Clustering** - Support for node discovery and gossip protocols

## Documentation

Go Sync Kit maintains organized documentation to help users and contributors:

### 📖 User Documentation
- **README.md** (this file) - Complete API documentation with examples
- **CHANGELOG.md** - Version history and release notes
- **examples/** - Progressive examples from basic to advanced usage

### 📁 Technical Documentation
- **docs/** - Organized technical documentation
  - **docs/design/** - Architecture and design specifications
  - **docs/testing/** - Testing strategies, benchmarks, and quality assurance
  - **docs/implementation/** - Implementation guides and technical details

### 🔍 Finding Information
- **Getting Started**: Start with the examples in `examples/quickstart/`
- **Advanced Usage**: See `examples/intermediate/` for production patterns
- **API Reference**: Go doc comments throughout the codebase
- **Architecture Details**: See `docs/design/` for system design documents
- **Performance**: See `docs/testing/` for benchmarking information

For the most current information, always refer to the working examples in the `examples/` directory, as they represent the current API and best practices.

## Contributing

**We're actively seeking feedback and contributions!** As a project in active development (v0.6.0), your input is especially valuable.

### Ways to Contribute:
- **Try it out** and report your experience
- **Open issues** for bugs, feature requests, or API suggestions
- **Share feedback** on the API design and usability
- **Contribute code** improvements and new features
- **Write examples** showing real-world usage
- **Mentor & teach** - Help me learn Go best practices and patterns
- **Learn together** - If you're also learning Go, let's collaborate and grow together
- **Code reviews** - Point out improvements, suggest better approaches, or explain Go idioms

### Code Contributions:
1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Feedback & Discussion:
- Open an issue to discuss API changes or improvements
- Share your use case and how Go Sync Kit fits (or doesn't fit)
- Suggest better naming, patterns, or architectural improvements

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Inspiration

This project was inspired by:
- [Go Kit](https://github.com/go-kit/kit) for clean interface design
- [PouchDB](https://pouchdb.com/) for sync protocol concepts
- [CouchDB](https://couchdb.apache.org/) for replication patterns
- [Watermill](https://github.com/ThreeDotsLabs/watermill) for event streaming architecture
