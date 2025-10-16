# Documentation Index

[← Back to main README](../README.md#-documentation-index)

Comprehensive documentation for go-sync-kit. All key docs are one or two clicks away.

---

## 📚 Core Documentation

### Getting Started
- **[Architecture Overview](overview.md)** – Mental model: SyncNode, Store, Transport, Resolver; pull→resolve→push flow

### Concepts & Design
- **[State Machine](design/STATE_MACHINE_ROADMAP.md)** – State transitions and observability during sync operations
- **[Read Model Projections](design/READ_MODEL_PROJECTIONS.md)** – CQRS/event sourcing projection patterns
- **[Cursor Wire Format](design/CURSOR_WIRE_D1.md)** – Version/cursor encoding for transport protocols

### How-To Guides

**Transport Layer:**
- **[HTTP Transport](design/HTTP_D1.md)** – HTTP client/server sync protocol specification
- **[HTTP Enhancements](design/HTTP_ENHANCEMENTS.md)** – Advanced features: filtering, multitenancy, auth
- **[HTTP Migration Guide](design/MIGRATION_GUIDE_HTTP.md)** – Upgrading to enhanced HTTP transport
- **[RabbitMQ Transport](design/RABBITMQ_ROADMAP.md)** – Durable messaging with RabbitMQ

**Storage Backends:**
- **[PostgreSQL EventStore](design/POSTGRES_EVENTSTORE_DESIGN.md)** – PostgreSQL with LISTEN/NOTIFY for real-time events
- In-memory (memstore) – See [quickstart example](../examples/quickstart)
- SQLite – See [HTTP examples](../examples/HTTP_EXAMPLES.md)
- Badger – See package docs

**Observability:**
- **[Benchmarks & Fuzzing](testing/BENCHMARKS_AND_FUZZING.md)** – Performance testing and quality assurance
- Metrics/Tracing – See [examples/intermediate/09-advanced-observability](../examples/intermediate/09-advanced-observability)
- Structured Logging – See [examples/intermediate/07-structured-logging](../examples/intermediate/07-structured-logging)

### Implementation Status
- **[Projections Complete](design/PROJECTION_IMPLEMENTATION_COMPLETE.md)** – Current status of projection implementation
- **[Implementation Plan](design/IMPLEMENTATION_PLAN.md)** – Active development roadmap

---

## 📂 Additional Resources

### Project Root
- **[CHANGELOG.md](../CHANGELOG.md)** – Version history and release notes
- **[CONTRIBUTING.md](../CONTRIBUTING.md)** – Contribution guidelines and code standards
- **[WARP.md](../WARP.md)** – Development tool configuration

### Examples Directory
See **[examples/README.md](../examples/README.md)** for full index of runnable examples.

### API Reference
- **[pkg.go.dev](https://pkg.go.dev/github.com/c0deZ3R0/go-sync-kit)** – Complete Go package documentation

---

## 📋 Documentation Organization

**`/docs/`** (this directory)  
Architecture, design specs, testing strategies, migration guides

**`/docs/design/`**  
Design documents and technical specifications

**`/docs/testing/`**  
Performance benchmarks and testing documentation

**`/examples/`**  
Runnable code examples with READMEs (preferred for usage docs)

**Package docs**  
Go doc comments throughout the codebase
