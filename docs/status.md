# Feature Status Matrix

This document provides a comprehensive status overview of all go-sync-kit features, organized by category. Use this to understand what's production-ready, experimental, or planned.

---

## Status Meanings

- **Stable**: API unlikely to change; used in examples/CI; docs complete; known limits documented. Safe for production use.
- **Experimental**: API may change; limited docs/ops guidance; not yet recommended for production without careful evaluation.
- **Planned**: Not available yet; tracked in roadmap or design docs.

---

## Store Backends

Event persistence implementations for durable event logs.

| Feature | Status | Docs | Notes |
|---------|--------|------|-------|
| **MemStore** (In-Memory) | **Stable** | [storage/README.md](../storage/README.md#1-memstore-in-memory---best-for-development-), [storage/memstore/](../storage/memstore/) | Zero dependencies; perfect for dev/test |
| **SQLite** | **Stable** | [storage/README.md](../storage/README.md#2-sqlite---best-for-single-node-apps-️), [storage/sqlite/README.md](../storage/sqlite/README.md) | Single-node; WAL mode; production-ready for desktop/embedded |
| **PostgreSQL** | **Stable** | [storage/README.md](../storage/README.md#3-postgresql---best-for-production-), [storage/postgres/README.md](../storage/postgres/README.md), [design/POSTGRES_EVENTSTORE_DESIGN.md](design/POSTGRES_EVENTSTORE_DESIGN.md) | Multi-node; LISTEN/NOTIFY; battle-tested at scale |
| **BadgerDB** | **Experimental** | [storage/README.md](../storage/README.md#4-badgerdb---best-for-high-performance-), [storage/badger/README.md](../storage/badger/README.md) | High-performance LSM; needs ops guidance for GC/compaction |

---

## Transports

Network transport implementations for moving events between nodes.

| Feature | Status | Docs | Notes |
|---------|--------|------|-------|
| **HTTP** (Push/Pull) | **Stable** | [transport/README.md](../transport/README.md), [design/HTTP_D1.md](design/HTTP_D1.md), [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md) | Request/response sync; enterprise features stable |
| **SSE** (Server-Sent Events) | **Experimental** | [transport/sse/README.md](../transport/sse/README.md), [examples/HTTP_EXAMPLES.md](../examples/HTTP_EXAMPLES.md#-real-time-events-with-sse) | Subscribe-only MVP; lacks auth/reconnection/pooling |
| **RabbitMQ** | **Experimental** | [transport/rabbitmq/README.md](../transport/rabbitmq/README.md), [design/RABBITMQ_ROADMAP.md](design/RABBITMQ_ROADMAP.md) | Phase 1 complete; needs DLQ/retry/priority for Stable |
| **memchan** (In-Memory) | **Stable** | [transport/README.md](../transport/README.md) | Local testing; zero network overhead |

---

## Real-time Options

Real-time event streaming and notification mechanisms.

| Feature | Status | Docs | Notes |
|---------|--------|------|-------|
| **SSE streaming** | **Experimental** | [transport/sse/README.md](../transport/sse/README.md), [examples/HTTP_EXAMPLES.md](../examples/HTTP_EXAMPLES.md#-real-time-events-with-sse) | Client/server streaming; needs auth and reconnect logic |
| **PostgreSQL LISTEN/NOTIFY** | **Experimental** | [storage/postgres/README.md](../storage/postgres/README.md), [design/POSTGRES_EVENTSTORE_DESIGN.md](design/POSTGRES_EVENTSTORE_DESIGN.md) | Real-time DB notifications; production-tested pattern |
| **RealtimeSyncManager** | **Experimental** | [examples/intermediate/05-realtime-autosync](../examples/intermediate/05-realtime-autosync), [synckit/realtime.go](../synckit/realtime.go) | Core real-time sync orchestration; API stabilizing |
| **RabbitMQ subscribe** | **Experimental** | [transport/rabbitmq/README.md](../transport/rabbitmq/README.md#basic-consumer), [design/RABBITMQ_ROADMAP.md](design/RABBITMQ_ROADMAP.md) | Message queue subscriptions; Phase 2 features needed |

---

## Resolvers

Conflict resolution strategies for handling concurrent updates.

| Feature | Status | Docs | Notes |
|---------|--------|------|-------|
| **Last-Write-Wins (LWW)** | **Stable** | [overview.md](overview.md#conflict-resolution-strategies), [examples/intermediate/04-conflict-resolution](../examples/intermediate/04-conflict-resolution) | Default resolver; well-tested |
| **First-Write-Wins** | **Experimental** | [examples/intermediate/04-conflict-resolution](../examples/intermediate/04-conflict-resolution) | Alternative timestamp-based strategy |
| **Additive Merge** | **Experimental** | [examples/intermediate/04-conflict-resolution](../examples/intermediate/04-conflict-resolution) | CRDT-style additive merges |
| **Manual/Stateful** | **Experimental** | [examples/intermediate/08-stateful-resolvers](../examples/intermediate/08-stateful-resolvers), [examples/intermediate/08-stateful-resolvers/README.md](../examples/intermediate/08-stateful-resolvers/README.md) | Custom business logic; API stabilizing |

---

## Cross-cutting Capabilities

Enterprise features and operational capabilities that span multiple components.

| Capability | Status | Docs | Notes |
|------------|--------|------|-------|
| **HTTP Authentication** (Bearer/HMAC) | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md#-authentication-middleware), [transport/httptransport/middleware](../transport/httptransport/middleware) | Production auth middleware |
| **HTTP Authorization** | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md), [transport/httptransport/middleware](../transport/httptransport/middleware) | Role-based access control |
| **Idempotency Keys** | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md#-idempotency-support), [transport/httptransport/idempotency.go](../transport/httptransport/idempotency.go) | Prevents duplicate processing |
| **Rate Limiting** | **Stable** | [best-practices.md](best-practices.md), HTTP transport docs | Basic rate limiting patterns documented |
| **Backpressure** | **Experimental** | [best-practices.md](best-practices.md), transport docs | Pattern guidance; needs more tooling |
| **Multitenancy** | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md#-multitenancy-support), [transport/httptransport/middleware/tenant.go](../transport/httptransport/middleware/tenant.go) | Tenant isolation via headers |
| **Structured Errors** | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md#-structured-error-responses), [transport/httptransport/errors.go](../transport/httptransport/errors.go) | Standardized JSON error format |
| **HTTP Query Filtering** | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md#-advanced-filtering), [transport/httptransport/query.go](../transport/httptransport/query.go) | Filter by type/tenant/aggregate |
| **Observability: Health Checks** | **Stable** | [observability/health/README.md](../observability/health/README.md), [observability/README.md](../observability/README.md) | Liveness/readiness probes |
| **Observability: Tracing** | **Stable** | [observability/README.md](../observability/README.md), [observability/tracing](../observability/tracing) | OpenTelemetry integration |
| **Observability: Metrics** | **Stable** | [observability/README.md](../observability/README.md), [observability/metrics](../observability/metrics) | Prometheus-compatible metrics |
| **Observability: Logging** | **Stable** | [logging/README.md](../logging/README.md), [examples/intermediate/07-structured-logging](../examples/intermediate/07-structured-logging) | Structured logging support |
| **Projections** | **Stable** | [design/PROJECTION_IMPLEMENTATION_COMPLETE.md](design/PROJECTION_IMPLEMENTATION_COMPLETE.md), [design/READ_MODEL_PROJECTIONS.md](design/READ_MODEL_PROJECTIONS.md), [projection/](../projection) | CQRS/event sourcing projections |
| **Codec Registry** | **Stable** | [synckit/codec/README.md](../synckit/codec/README.md) | Pluggable event data encoding |
| **State Machine** | **Experimental** | [design/STATE_MACHINE_ROADMAP.md](design/STATE_MACHINE_ROADMAP.md), [synckit/statemachine](../synckit/statemachine) | Sync operation state tracking |
| **Compression** (HTTP) | **Stable** | [design/HTTP_ENHANCEMENTS.md](design/HTTP_ENHANCEMENTS.md), [transport/httptransport/compression.go](../transport/httptransport/compression.go) | gzip compression for HTTP transport |

---

## Notes

### Promotion Criteria

**Experimental → Stable**:
- Complete operational documentation (deployment, scaling, failure modes)
- Integration tests and examples demonstrating production patterns
- Known limitations clearly documented
- Migration guide if API changes occurred
- Performance characteristics documented

**Planned → Experimental**:
- Implementation merged to main branch
- Basic documentation and examples
- Unit tests passing

### Updating This Matrix

When adding new features or changing status:
1. Update the appropriate table above
2. Ensure documentation links are current
3. Add acceptance criteria for promotion if status is Experimental
4. For Planned features, link to the tracking issue or roadmap doc

### Related Documents

- [Architecture Overview](overview.md) - Mental model of core components
- [Best Practices](best-practices.md) - Production deployment guidance
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
- [Examples Index](../examples/README.md) - Runnable code examples

---

**Last Updated**: 2025-10-26  
**Maintenance**: This matrix should be updated whenever features are added, promoted, or deprecated.
