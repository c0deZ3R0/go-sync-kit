# FAQ

[\u2190 Back to Documentation Index](../README.md#-documentation-index)

**TL;DR**
- SyncNode orchestrates pull → resolve → push; you don't need to understand state machine signals to use the library
- Server-authoritative is simplest; peer-to-peer requires custom conflict resolution
- Custom resolvers solve domain-specific conflicts beyond last-write-wins
- Idempotent stores make retries safe—duplicate event IDs are no-ops
- Start with SQLite + HTTP for production; scale to Postgres when you need multi-server replication

---

## Do I need the state machine?

**Short answer:** No, not for basic usage.

The [state machine signals](./overview.md#state-machine-signals) are useful for observability, retry logic, and integration. Calling `node.Sync(ctx)` or `node.StartAutoSync(ctx)` runs invisibly. See [State machine signals](./overview.md#state-machine-signals) for debugging.

## Server-authoritative vs peer-to-peer?

**Server-authoritative** (simplest): One node is the source of truth. Clients pull, local edits are staged, then pushed back. Use the built-in LWW (last-write-wins) resolver.

**Peer-to-peer** (complex): All nodes are equal. Conflicts inevitable. Requires custom domain logic to resolve. Example: a conflict resolver that merges CRDTs or applies operational transforms.

Start with server-authoritative; peer-to-peer needs custom [conflict resolution](./overview.md#conflict-resolution-strategies).

## When should I implement a custom resolver?

When LWW doesn't match your domain:
- **CRDTs** (e.g., counter increments, set unions)
- **Operational transforms** (edit-conflict merging)
- **Domain rules** (e.g., "inventory never goes negative")

See [Conflict resolution strategies](./overview.md#conflict-resolution-strategies) and [examples/intermediate/04-conflict-resolution](../examples/intermediate/04-conflict-resolution).

## How do I handle large backfills efficiently?

Use cursor-based pagination (batch 100–1000 events). Disable conflict resolution during backfill. Consider manual `Pull(ctx)` with custom backoff for massive backfills.

See [examples/http_server](../examples/http_server) for patterns.

## What does "idempotent store" mean here?

An **idempotent store** means duplicate event IDs are no-ops. Push the same event 10 times, it's stored once. This makes retries safe: if a network request fails halfway, resend it—the store handles dedup.

Why it matters:
- Network timeouts don't cause duplicates
- No need to track "which events were already stored"
- Simplifies state machine retry loops

Your store must enforce unique event IDs; SyncNode relies on this. See [Store](./overview.md#store) for details.

## Versioning and vector clocks: how do I reason about conflicts?

**Integer versions** (simple): Sequential counters. Conflicts = "remote > local" → use remote.

**Vector clocks** (complex): Track per-node versions. Conflict = neither dominates (e.g., `[local: {A:2, B:1}, remote: {A:1, B:2}]`). Requires custom resolution logic.

Start with integer versions and LWW. Only use vector clocks if you need multi-writer conflict detection.

## What's the simplest production-ready combo?

- **Store:** SQLite (embedded database, single process)
- **Transport:** HTTP (client/server)
- **Resolver:** LWW (last-write-wins, default)

See [examples/http_server/main_production.go](../examples/http_server) and [Best Practices](./best-practices.md#store-selection-matrix).

## How do I do multi-tenant isolation?

Partition at the **store level** (separate DB per tenant) or **transport level** (tenant ID in headers/URLs, filter responses).

**Best practice:** Use tenant headers (e.g., `X-SyncKit-Tenant: acme-corp`) + authentication. See [examples/http_enterprise](../examples/http_enterprise) for multitenancy patterns.

## How should I structure event payloads and metadata?

**Payloads:** Immutable, versioned, timestamped. Avoid mutable nested objects.

**Metadata:** Use `Event.Metadata()` for tags (tenant, user, source). Keep lightweight.

Example:
```go
type MyEvent struct {
  ID string // unique
  AggregateID string // grouping key
  Type string // "OrderCreated"
  Timestamp time.Time
  Data interface{}
}
func (e MyEvent) Metadata() map[string]interface{} {
  return map[string]interface{}{"tenant": "acme", "user": "alice"}
}
```

## When should I use SSE vs polling?

**SSE (Server-Sent Events):** Real-time push. Low-latency, persistent connection. Use when you want instant updates.

**Polling:** Request/response. More compatible, easier to debug, higher latency. Use for mobile or unstable networks.

See [examples/HTTP_EXAMPLES.md](../examples/HTTP_EXAMPLES.md) for SSE setup.

## How do I observe sync health?

Monitor [state machine signals](./overview.md#state-machine-signals) and emit logs/metrics:
- **Pushing stalled** → alert (network or resolver timeout)
- **High conflict rate** → check client clocks or resolver logic
- **Pull latency** → baseline, alert if 10x higher

See [examples/intermediate/09-advanced-observability](../examples/intermediate/09-advanced-observability) for Prometheus/OpenTelemetry integration.

## What's the path to migrate transports or stores?

1. Run old and new in parallel; compare outputs
2. Backfill new store from old
3. Cut traffic over gradually
4. Monitor for conflicts; fall back if needed

See [Best Practices](./best-practices.md) for safe defaults and [Troubleshooting](./troubleshooting.md) for common issues.

## Tips for mobile and spotty networks?

- Use exponential backoff (cap at 60s)
- Batch events locally; sync when connected
- Use SSE with reconnection logic
- Reduce `SyncInterval` during unstable periods

See [examples/http_server](../examples/http_server) for production backoff patterns.

## How do I test conflict scenarios deterministically?

1. Use an in-memory store + null transport for unit tests
2. Manually create conflicting events with different vector clocks
3. Assert resolver output matches expected merge logic
4. Use state machine signals to verify retry behavior

See [examples/intermediate/04-conflict-resolution](../examples/intermediate/04-conflict-resolution) and project tests.

## What are safe defaults?

- **Batch size:** 100–1000 events per request
- **Sync interval:** 5–30 seconds
- **Backoff cap:** 60 seconds
- **Request timeout:** 30 seconds

Start here, measure, iterate.

---

[\u21a9\ufe0e Back to Documentation Index](../README.md#-documentation-index)
