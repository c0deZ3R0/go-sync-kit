# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

Go Sync Kit is a generic, event-driven synchronization library for distributed Go applications. It enables offline-first architectures with conflict resolution and pluggable storage backends. The project follows clean architecture principles with clear separation of concerns across transport, storage, versioning, and conflict resolution layers.

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
go test ./transport/httptransport/...

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

```bash
# Basic example with dashboard
cd examples/basic
go run .

# Conflict resolution showcase
cd examples/conflict-resolution
go run .

# Real-time sync example
cd examples/real-time-sync
go run .

# Dynamic resolver showcase
cd examples/dynamic-resolver-showcase
go run .
```

## Architecture Overview

Go Sync Kit follows a layered, plugin-based architecture with these core components:

### Core Interfaces (synckit/sync.go)

- **Event**: Represents syncable events with ID, Type, AggregateID, Data, and Metadata
- **EventStore**: Provides persistence with Store, Load, LoadByAggregate, LatestVersion, ParseVersion methods
- **Transport**: Handles network communication with Push, Pull, Subscribe, GetLatestVersion methods  
- **ConflictResolver**: Resolves conflicts with Resolve method
- **SyncManager**: Orchestrates sync operations with Sync, Push, Pull, auto-sync capabilities

### Storage Layer (storage/)

- **SQLite** (storage/sqlite/): Production-ready SQLite implementation with WAL mode, connection pooling, and comprehensive testing
- **PostgreSQL** (storage/postgres/): Feature-rich PostgreSQL implementation with LISTEN/NOTIFY support and real-time capabilities
- **BadgerDB**: Mentioned in roadmap, embedded key-value store option

### Transport Layer (transport/)

- **HTTP Transport** (transport/httptransport/): RESTful HTTP client/server with compression, validation, security hardening
- **SSE Transport** (transport/sse/): Server-Sent Events for real-time streaming with cursor-based pagination
- Custom transports can be implemented for gRPC, WebSockets, NATS, etc.

### Versioning Strategies (version/)

- **Vector Clocks**: Distributed versioning with causal ordering and conflict detection
- **Simple Versioning**: Timestamp or sequential ID based versioning for centralized scenarios
- **Custom Versioning**: Extensible VersionManager interface for custom strategies

### Conflict Resolution (synckit/conflict.go, synckit/resolver.go)

- **Last Write Wins**: Simple timestamp-based resolution
- **Dynamic Resolver**: Configurable resolver with multiple strategies
- **Custom Resolvers**: Implement ConflictResolver interface for domain-specific logic

### Key Design Patterns

1. **Clean Architecture**: Clear separation between transport, storage, business logic
2. **Interface-Based Design**: All components are interfaces for maximum flexibility
3. **Context-Aware**: Full context support with timeouts and cancellation
4. **Error Handling**: Structured error system with error codes and metadata (errors/)
5. **Metrics & Observability**: Built-in metrics collection and structured logging (logging/)
6. **Builder Pattern**: Configuration through builders with validation
7. **Decorator Pattern**: VersionedStore wraps base stores with versioning logic

## Key Implementation Details

### Event Flow
1. Events implement the Event interface with ID, Type, AggregateID, Data, Metadata
2. Events are stored in EventStore with associated Version
3. SyncManager orchestrates Push (local→remote) and Pull (remote→local) operations
4. Transport layer handles network serialization/communication
5. ConflictResolver handles concurrent modifications during sync

### Testing Architecture
The project has extensive testing with:
- Unit tests with mocks (synckit/testing_mocks.go, synckit/test_helpers.go)
- Integration tests for storage implementations
- Fuzz tests for critical components (cursor/, transport/httptransport/)
- Benchmark tests for performance validation
- Phase-based integration testing (phase3_integration_test.go, phase4_integration_test.go)

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
