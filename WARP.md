# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

go-sync-kit provides tiny, composable building blocks for event sync in Go. The project emphasizes simplicity and a clear mental model (Node → Store + Transport + Resolver), with an in-memory dev experience that requires no external dependencies. It offers HTTP presets for client/server setups, pluggable conflict resolution, and production-ready stores/transports. The README has been transformed into a welcoming front door that focuses on quick starts and practical examples rather than exhaustive API documentation.

## Recent Major Features (v0.22.0 and beyond)

### Latest Changes:
- **Concise README** (PR #61): Transformed README into welcoming front door (1,500+ lines → 173 lines)
- **SyncNode API** (PR #59): New preferred API with preset functions and migration guide
- **In-Memory Components** (PR #58): memstore + memchan for instant dev/testing (2,555+ lines added)
  - storage/memstore: Thread-safe in-memory EventStore
  - transport/memchan: Real-time channel-based transport
  - examples/inmem: Complete working example without external dependencies
  - event/ package: Concrete Event struct implementation

### Key Features:
- Zero-dependency development experience (memstore + memchan)
- SyncNode façade with three presets (InMemory, HTTPClient, HTTPServer)
- Production HTTP examples with graceful shutdown
- Comprehensive HTTP_EXAMPLES.md documentation
- Enhanced logging consistency across examples

## Repository Workflow & Guardrails

- Default branch: main
- Always do work on a feature branch (e.g., feature/<short-desc>). For transport work, feature/transport-<name> is recommended.
- Do not commit or push changes unless explicitly instructed. Do not rebase, pull, or merge unless directed.
- Use non-interactive Git commands and --no-pager to avoid paginated output.
- Prefer absolute paths with -C when running git from scripts.

## Developer Tooling

### Task Automation (Taskfile.yml)

The project includes a Taskfile for common development tasks:

```bash
# View available tasks
task --list

# Common tasks (if Taskfile is configured)
task build      # Build the project
task test       # Run tests
task lint       # Run linters
task format     # Format code
```

### Git Hooks (hooks/)

Pre-configured git hooks for code quality:
- **pre-commit**: Runs before commits (formatting, quick tests)
- **pre-push**: Runs before push (full test suite)

Install hooks: Copy from `hooks/` to `.git/hooks/`

### Editor Configuration (.editorconfig)

Consistent code style across editors:
- Indentation: Tabs
- Line endings: LF (Unix-style)
- Trim trailing whitespace
- Insert final newline

## Development Commands

### Build and Test Commands

```bash
# Build the entire project
go build ./...

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run only unit tests (faster, no external dependencies)
go test -short -v ./...

# Run tests with coverage
go test -cover ./...

# Run specific test packages
go test ./synckit/...
go test ./storage/sqlite/...
go test ./storage/memstore/...
go test ./transport/httptransport/...
go test ./transport/memchan/...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run fuzz tests
go test -fuzz=. ./cursor/
go test -fuzz=. ./synckit/
go test -fuzz=. ./transport/httptransport/

# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run ./...

# Tidy dependencies
go mod tidy
```

### PostgreSQL Development (storage/postgres)

```bash
# Navigate to postgres storage directory
cd storage/postgres

# Start PostgreSQL with Docker Compose
make docker-up

# Run PostgreSQL-specific tests
make test

# Run only integration tests
make test-integration

# Run benchmarks
make benchmark

# Connect to test database
make db-connect

# View logs
make docker-logs

# Clean up
make docker-down
```

### Example Applications

The examples directory now provides focused, practical demonstrations:

```bash
# In-memory patterns (hub, subscriptions)
cd examples/inmem
go run main.go

# HTTP client
cd examples/http_client
go run main.go

# HTTP server
cd examples/http_server
go run main.go

# Production server with graceful shutdown
cd examples/http_server
go run main_production.go

# Observability examples
cd examples/observability_basic
go run main.go
```

**Documentation**: For detailed HTTP examples and patterns, see `examples/HTTP_EXAMPLES.md`.

## Documentation Philosophy

The README now serves as a welcoming front door focusing on:
- Quick starts (60-second in-memory, HTTP client/server)
- Core concepts (simple mental model)
- Migration guidance (SyncManager → SyncNode)
- Links to examples and deeper documentation

Detailed API documentation, advanced patterns, and architectural details are in:
- `/examples` directory (practical demonstrations)
- `/docs` directory (design, testing, implementation guides)
- Code comments (Go doc standards)

### Additional Context (Obsidian Knowledge)
If Obsidian MCP is available, these notes provide deeper conceptual explanations. Use the note titles to find them in the vault:
- Transport: "Transport Layer.md", "HTTP Transport.md", "Realtime Transport (SSE or Rabbit MQ).md"
- Storage: "Stores.md", "The Sqlite EventStore.md", "The Postgres EventStore.md"
- State machine: "State Machine.md", "Do i need the state machine.md"
- Resolvers: "Resolvers.md", "Conflict Resolvers.md", "Server Side Authoritative Resolver.md"
- Versioning: "vector clocks.md", "How State Machines, Vector Clocks & Resolvers work together.md"
- Projections & offsets: "Projections.md", "Offset stores.md"
- Roles & topology: "Client.md", "Client App.md", "Server.md", "Client Server.md"
- Legacy/compat: "syncManager.md"

## Architecture Overview

go-sync-kit follows a layered, plugin-based architecture with these core components:

### Core Interfaces (synckit/sync.go)

- **Event**: Represents syncable events with ID, Type, AggregateID, Data, and Metadata
- **EventStore**: Provides persistence with Store, Load, LoadByAggregate, LatestVersion, ParseVersion methods
- **Transport**: Handles network communication with Push, Pull, Subscribe, GetLatestVersion methods  
- **ConflictResolver**: Resolves conflicts with Resolve method
- **SyncManager**: Orchestrates sync operations with Sync, Push, Pull, auto-sync capabilities
- **SyncNode**: New preferred API (type alias to SyncManager) with convenience presets

### Storage Layer (storage/)

- **SQLite** (storage/sqlite/): Production-ready SQLite implementation with WAL mode, connection pooling, and comprehensive testing
- **PostgreSQL** (storage/postgres/): Feature-rich PostgreSQL implementation with LISTEN/NOTIFY support and real-time capabilities
- **BadgerDB** (storage/badger/): Embedded key-value store implementation
- **MemStore** (storage/memstore/): Thread-safe in-memory EventStore for development/testing (no external dependencies)
  - Perfect for quick prototyping and examples
  - 100% test coverage with comprehensive test suite
  - No SQLite or database setup required

### Transport Layer (transport/)

- **HTTP Transport** (transport/httptransport/): RESTful HTTP client/server with compression, validation, security hardening
- **SSE Transport** (transport/sse/): Server-Sent Events for real-time streaming with cursor-based pagination
- **MemChan** (transport/memchan/): Real-time channel-based transport for in-memory communication
  - Thread-safe channel-based transport
  - Hub-based pub/sub with multiple subscribers
  - Perfect for development and testing
  - No HTTP server or network setup required
  - 100% test coverage with comprehensive test suite
- **RabbitMQ Transport** (transport/rabbitmq/): Durable messaging with publish/subscribe, routing (direct, topic, fanout), publisher confirms, dead-letter queues, retries, priorities, TTL, and consumer prefetch. See RABBITMQ_ROADMAP.md.
- Custom transports can be implemented for gRPC, WebSockets, NATS, etc.

### Versioning Strategies (version/)

- **Vector Clocks**: Distributed versioning with causal ordering and conflict detection
- **Simple Versioning**: Timestamp or sequential ID based versioning for centralized scenarios
- **Custom Versioning**: Extensible VersionManager interface for custom strategies

### Conflict Resolution (synckit/conflict.go, synckit/resolver.go)

- **Last Write Wins**: Simple timestamp-based resolution
- **Dynamic Resolver**: Configurable resolver with multiple strategies
- **Custom Resolvers**: Implement ConflictResolver interface for domain-specific logic

### Read-Model Projections (projection/)

- **Projection API**: Core interfaces for Projector, OffsetStore, and Runner
- **BadgerDB Offset Store**: High-performance embedded offset persistence with 91.5% test coverage
- **Projection Runner**: Batch processing with resumable execution and context cancellation
- **Unified Observability**: Integrated metrics and health checks for projection monitoring
- **Auto-execution**: Automatic projection running after sync operations
- **CQRS/Event Sourcing**: Full support for read-model building patterns

### SyncNode API (New Preferred Interface)

- **SyncNode** (synckit/node.go): Cleaner, more intuitive API façade over SyncManager
  - Currently implemented as type alias for zero overhead
  - Three convenience presets: `NewInMemoryNode()`, `NewHTTPClientNode()`, `NewHTTPServerNode()`
  - 100% compatible with SyncManager methods
  - Future-proof design for potential wrapper struct migration
  - See `synckit/SYNCNODE_MIGRATION.md` for complete migration guide

### Event Package (event/)

- **Concrete Event struct**: Ready-to-use Event implementation
- No need to implement Event interface for basic use cases
- Used extensively in memstore/memchan examples

### Key Design Patterns

1. **Clean Architecture**: Clear separation between transport, storage, business logic
2. **Interface-Based Design**: All components are interfaces for maximum flexibility
3. **Context-Aware**: Full context support with timeouts and cancellation
4. **Error Handling**: Structured error system with error codes and metadata (errors/)
5. **Metrics & Observability**: Built-in metrics collection and structured logging (logging/)
6. **Builder Pattern**: Configuration through builders with validation
7. **Decorator Pattern**: VersionedStore wraps base stores with versioning logic
8. **Façade Pattern**: SyncNode provides simplified interface over SyncManager
9. **Preset Functions**: Quick-start functions for common configurations

## SyncNode Presets (Convenience Functions)

Three quick-start functions for common configurations (synckit/node_presets.go):

### NewInMemoryNode(store, transport)
- For development and testing
- Works with memstore.New() and memchan.New()
- Zero external dependencies
- Includes LWW conflict resolution by default

### NewHTTPClientNode(store, transport)
- For HTTP client applications
- Works with sqlite.New() or postgres.New()
- Uses httptransport.NewTransport() pointing to server
- Includes LWW conflict resolution by default

### NewHTTPServerNode(store, transport)
- For HTTP server applications
- Works with sqlite.New() or postgres.New()
- Uses httptransport configured for server mode
- Includes LWW conflict resolution by default

## Key Implementation Details

### Event Flow
1. Events implement the Event interface with ID, Type, AggregateID, Data, Metadata (or use concrete event.Event struct)
2. Events are stored in EventStore with associated Version
3. SyncNode/SyncManager orchestrates Push (local→remote) and Pull (remote→local) operations
4. Transport layer handles network serialization/communication
5. ConflictResolver handles concurrent modifications during sync

### Testing Architecture
The project has extensive testing with:
- Unit tests with mocks (synckit/testing_mocks.go, synckit/test_helpers.go)
- Integration tests for storage implementations
- Fuzz tests for critical components (cursor/, transport/httptransport/)
- Benchmark tests for performance validation
- Integration testing for conflict resolution and multiuser scenarios

### Configuration Patterns
- SyncOptions struct for sync behavior configuration
- Builder pattern for complex configuration (synckit/builder.go)
- Environment-specific configurations for storage and transport
- Sensible defaults with override capabilities

### Error Handling Strategy
- Custom error types in errors/ package with error codes and metadata
- Context-aware error propagation
- Retryable vs non-retryable error classification
- Structured logging integration

### Dependencies and Module Structure
- Main module: `github.com/c0deZ3R0/go-sync-kit`
- Go 1.24.4+ required
- Key dependencies: SQLite (mattn/go-sqlite3), PostgreSQL (lib/pq), testing (stretchr/testify)
- Organized into focused packages to prevent circular dependencies

This architecture enables building distributed systems with offline capabilities, automatic conflict resolution, and pluggable persistence/transport layers while maintaining type safety and performance.

## Quick Reference: Common Workflows

### Starting a New Project
```go
// Development/Testing (no external deps)
store := memstore.New()
transport := memchan.New(16)
node, _ := synckit.NewInMemoryNode(store, transport)

// Production HTTP Client
store, _ := sqlite.New(&sqlite.Config{DataSourceName: "client.db"})
transport := httptransport.NewTransport("http://server:8080/sync", nil, nil, nil)
node, _ := synckit.NewHTTPClientNode(store, transport)

// Production HTTP Server
store, _ := postgres.New("connection-string")
transport := httptransport.NewTransport("", nil, nil, nil)
node, _ := synckit.NewHTTPServerNode(store, transport)
```

### Running Examples
```bash
# In-memory quick start (no setup)
cd examples/inmem && go run main.go

# HTTP server + client
cd examples/http_server && go run main.go         # Terminal 1
cd examples/http_client && go run main.go          # Terminal 2

# Production server with graceful shutdown
cd examples/http_server && go run main_production.go
```

### Testing Strategy
```bash
# Quick tests (no PostgreSQL required)
go test -short ./...

# All tests including integration
go test ./...

# PostgreSQL tests only (requires POSTGRES_TEST=1)
POSTGRES_TEST=1 go test ./storage/postgres/...

# Test specific components
go test ./synckit -run TestSyncNode
go test ./storage/memstore -v
go test ./transport/memchan -v
```

### Key Files for Common Tasks

| Task | Files to Check |
|------|----------------|
| Quick start example | `examples/inmem/main.go` |
| HTTP setup | `examples/HTTP_EXAMPLES.md` |
| API migration | `synckit/SYNCNODE_MIGRATION.md` |
| Adding storage | `storage/storage.go` (interface) |
| Adding transport | `transport/transport.go` (interface) |
| Conflict resolution | `synckit/resolver.go`, `synckit/conflict.go` |
| Recent changes | `CHANGELOG.md` |
| Architecture docs | `docs/design/` |
