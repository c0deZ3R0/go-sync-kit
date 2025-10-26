# Examples Index

Progressive examples from beginner to advanced. Start with [quickstart](quickstart) and work your way up.

---

## 🎯 Quick Start

### [quickstart/local-only/](quickstart/local-only)
Simplest possible setup with in-memory store and null transport. No network needed.  
**Topics:** Event interface, memstore, null transport, basic sync flow

### [quickstart/http-client/](quickstart/http-client)
Minimal HTTP client that syncs with a server. Perfect first step after local-only.  
**Topics:** HTTP transport client, basic network sync, client setup

---

## 🌐 HTTP Examples

### [http_server/](http_server)
Production-ready HTTP server with SQLite storage, graceful shutdown, and observability.  
**Topics:** HTTP transport, SQLite, server setup, production patterns

### [http_client/](http_client)
Standalone HTTP client example connecting to sync server.  
**Topics:** HTTP transport client, remote sync, connection management

See [http_client/README.md](http_client/README.md) for run commands.

### [http_enterprise/](http_enterprise)
Enterprise features: multitenancy, authentication, filtering, idempotency keys.  
**Topics:** Bearer auth, tenant isolation, advanced filtering, middleware

- [http_enterprise/server/](http_enterprise/server) - Production-style server with auth
- [http_enterprise/client/](http_enterprise/client) - Client with token auth and signing

### [HTTP_EXAMPLES.md](HTTP_EXAMPLES.md)
Comprehensive HTTP transport guide with SSE (Server-Sent Events) for real-time push.

---

## 📦 In-Memory Examples

### [inmem/](inmem)
In-memory sync patterns: hub/spoke, subscriptions, local-only architectures.  
**Topics:** memchan transport, hub pattern, event subscriptions

---

## 🏛️ Intermediate Topics

### [intermediate/03-events-and-storage/](intermediate/03-events-and-storage)
Custom event creation, storage with versioning, event retrieval patterns.  
**Topics:** Event interface, SQLite storage, versioning, Load operations

### [intermediate/04-conflict-resolution/](intermediate/04-conflict-resolution)
Conflict detection and resolution strategies (LWW, FWW, custom resolvers).  
**Topics:** ConflictResolver interface, deterministic merge, conflict patterns

### [intermediate/05-realtime-autosync/](intermediate/05-realtime-autosync)
Timers, background sync, and graceful shutdown patterns.  
**Topics:** StartAutoSync, StopAutoSync, signal handling, context cancellation

### [intermediate/06-custom-events-filters/](intermediate/06-custom-events-filters)
Selective sync by event type and metadata filtering.  
**Topics:** Event filtering, selective sync, custom predicates, bandwidth optimization

### [intermediate/07-structured-logging/](intermediate/07-structured-logging)
Integrating structured logging (slog) for production observability.  
**Topics:** slog integration, log levels, contextual logging

### [intermediate/08-stateful-resolvers/](intermediate/08-stateful-resolvers)
Advanced conflict resolution with stateful resolvers and business logic.  
**Topics:** Stateful resolvers, domain-specific merge, compensating events

### [intermediate/09-advanced-observability/](intermediate/09-advanced-observability)
Metrics (Prometheus) and distributed tracing (OpenTelemetry).  
**Topics:** Metrics collection, tracing, performance monitoring

### [intermediate/10-state-machine-enhancements/](intermediate/10-state-machine-enhancements)
Sync state machine: transitions, hooks, error handling.  
**Topics:** State machine, lifecycle hooks, error recovery

---

## 🔧 Advanced Patterns

### [server-projection-hooks/](server-projection-hooks)
Read-model projections and CQRS patterns with event hooks.  
**Topics:** Projections, CQRS, read models, materialized views

---

## 📝 Example Guidelines

**Running Examples:**
```bash
cd examples/<example-directory>
go run .
```

**Creating New Examples:**
1. Create directory under `examples/`
2. Add `README.md` with clear description and topics covered
3. Implement in `main.go` with inline comments
4. Test thoroughly and document expected output
5. Add to this index

**Requirements:**  
Go 1.21+ recommended
