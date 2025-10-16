# Best Practices

[\u2190 Back to Documentation Index](../README.md#-documentation-index)

**TL;DR**
- Pick SQLite for embedded use, Postgres for multi-server replication, Badger for high performance KV
- Batch events in 100–1000 event chunks; use exponential backoff (cap at 60s)
- Idempotent stores allow safe retries—duplicate event IDs are always no-ops
- Partition multi-tenant data at store or transport layer; enforce auth at edges
- Emit state machine signals to logs/metrics for visibility into sync health

---

## Store selection matrix

| Store | Pros | Cons | Recommended for |
|-------|------|------|-----------------|
| **memstore** | Zero deps, instant; no persistence | Only for testing, dev, in-process replay | Development, unit tests |
| **SQLite** | Embedded, single-file, ACID | Single process, no built-in replication | Embedded products, desktop apps, single-server deployments |
| **Postgres** | Multi-server, LISTEN/NOTIFY, ACID | Infrastructure overhead, more tuning | Distributed systems, microservices, hub-and-spoke architectures |
| **Badger** | High performance KV, LSM | Larger footprint, requires tuning | Performance-critical single-server workloads |

**Decision tree:**
- Dev/testing → memstore
- Single-server production → SQLite
- Multi-server hub → Postgres + LISTEN/NOTIFY
- High-throughput single-node → Badger

See [Storage Backends](../README.md#how-tos) examples for setup.

## Batching & intervals

**Event batching:** Group 100–1000 events per request (adjust by payload size and network latency). Reduces round-trips; balances memory vs latency.

**Sync interval:** Start at 5–30 seconds. Lower for real-time (SSE), higher for batch processes. Tune by:
- Event frequency: high → 5–10s
- Network latency: high → 30s
- Disk I/O bottleneck → increase interval, increase batch size

**Backoff strategy:**
```
Attempt 1: immediate
Attempt 2: 1s
Attempt 3: 2s
Attempt 4: 4s
... (exponential, cap at 60s)
```

Use state machine signals to trigger backoff; avoid spinning on sync failures.

See [State machine signals](./overview.md#state-machine-signals).

## Idempotency

An **idempotent store** guarantees: store the same event ID 10 times → stored once. This enables safe retries.

**Why it matters:**
- Network timeouts don't corrupt state
- No need for deduplication logic in your app
- Conflict resolution becomes deterministic

**How to ensure idempotency:**
- Use unique, stable event IDs (UUID or content hash)
- Store enforces unique constraints on ID
- Duplicate ID + same payload → no error, silently ignored
- Duplicate ID + different payload → error or merge by resolver

**On HTTP:** If you expose write endpoints (push events), add idempotency keys:
```go
req.Header.Set("Idempotency-Key", uuid.New().String())
```

See [examples/http_enterprise](../examples/http_enterprise) for implementation.

## Multi-tenant isolation

**Store-level isolation (safest):** Separate database/table per tenant. Hard delete on tenant offboard.

**Transport-level isolation:** Tenant ID in headers/URLs. Filter all responses by tenant. Requires strict enforcement in code.

**Best practice: Hybrid**
1. Enforce tenant ID at HTTP middleware (extract from auth token)
2. Thread tenant ID through all queries
3. Add tenant ID to store schema (`tenant_id` foreign key)
4. Log tenant ID with every operation

Example:
```go
// Middleware extracts tenant from JWT
tenant := extractTenant(req)
ctx = context.WithValue(ctx, "tenant", tenant)

// Store filters by tenant
events := store.ReadSince(ctx, since, tenant)
```

Avoid: Cross-tenant event streams, shared tables without tenant columns.

## Observability hooks

**Monitor state machine signals:**

| Signal | Action |
|--------|--------|
| Pulling started | Record start time; log "Pull started" |
| Pulling done | Calc latency; emit `sync.pull_duration_ms` |
| Resolving conflicts | Emit `sync.conflicts_total`; log conflict count |
| Pushing done | Emit `sync.events_pushed_total` |
| Error | Alert if repeated; log stack + retry attempt |

**What to alert on:**
- Pushing stalled > 5 min (network or resolver timeout)
- Conflict rate > 10% (clock skew or stale clients)
- Pull latency 10x baseline (overloaded server)
- Repeated errors after 3 retries (manual intervention needed)

**Logging pattern:**
```go
ctx.Logger.Info("sync_round_completed",
  "events_pulled", result.EventsPulled,
  "events_pushed", result.EventsPushed,
  "conflicts_resolved", result.ConflictsResolved,
  "duration_ms", elapsedMs,
  "tenant", tenantID,
)
```

See [examples/intermediate/09-advanced-observability](../examples/intermediate/09-advanced-observability) for Prometheus/OpenTelemetry setup.

---

[\u21a9\ufe0e Back to Documentation Index](../README.md#-documentation-index)
