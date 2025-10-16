# Architecture Overview

[← Back to main README](../README.md#-documentation-index)

This guide gives you a practical mental model for **go-sync-kit**. It connects the core pieces—**SyncNode**, **Store**, **Transport**, and **Resolver**—and walks through a typical "pull → resolve → push" round, including how versions and the state machine fit in.

---

## Core pieces at a glance

<a id="syncnode"></a>
### SyncNode
The façade you embed in your app or service. It orchestrates a sync round (pull → resolve → push), talks to your Store via the chosen Transport, emits **state machine** signals, and returns a `SyncResult`. Typical entry points include `Sync(ctx)`, `Pull(ctx)`, `Push(ctx)`, `StartAutoSync(ctx)`, and `Close()`.

<a id="store"></a>
### Store
Your durable event log (e.g., in-memory for dev; SQLite/Postgres/Badger for prod). It must:
- Append and read events by version
- Be **idempotent** (duplicate event IDs are no-ops)
- Support efficient range reads ("since version/vector clock")

<a id="transport"></a>
### Transport
How nodes talk (HTTP request/response, SSE for realtime fan-out, in-memory `memchan`, etc.). Transports **carry** events and version metadata; they **do not** decide conflicts.

<a id="resolver"></a>
### Resolver
Deterministic merge logic when both sides changed the same thing. Use a default (e.g., LWW or server-authoritative) or supply domain-specific rules. Resolvers should be **deterministic** and **idempotent**.

---

<a id="how-a-sync-runs-pull--resolve--push"></a>
## How a sync runs (pull → resolve → push)

1. **Start** – `node.Sync(ctx)` captures the local version (vector clock) and enters *Pulling*.
2. **Pull** – Ask remote for events **since** local version. Receive `events[]` + remote version.
3. **Apply + Resolve**  
   - Persist incoming events in order (idempotently).  
   - For conflicts, run the **Resolver** to produce a deterministic merged outcome (may emit compensating events).
4. **Push** – Compute the **delta** the remote is missing (by comparing versions) and send it.
5. **Commit** – On success, both sides advance versions. Node exits through *Pushing → Done* and returns a `SyncResult` (counts, timings, conflicts).
6. **Realtime (optional)** – With SSE or similar, subscribe to remote changes to shorten pull intervals.

---

<a id="typical-client--server-round-ascii"></a>
## Typical client ↔ server round (ASCII)

```

Client App (SyncNode)                      Server Hub (SyncNode)
|                                            |
| Sync()                                     |
|------------------------------------------->|
| 1) PULL: GET /events?since=<clientVC>      |
|<-------------------------------------------|  events[], serverVC
| 2) APPLY+RESOLVE (local)                   |
|    - write events idempotently             |
|    - resolver merges conflicts             |
| 3) PUSH: POST /events (delta since VC)     |
|------------------------------------------->|
|<-------------------------------------------|  ack + new serverVC
| 4) COMMIT: advance clientVC, emit signals  |
|                                            |

```

---

## Events & versions (vector clocks)

- **Event envelope**: `{ id, stream/aggregate id, type, payload, timestamp, version/clock }`.
- **Vector clock (VC)**: per-peer counters (or equivalent monotone version) to answer "do you have what I have?" without global ordering.
- **Idempotency**: Stores must treat duplicate IDs as no-ops; Pull/Push must be retry-safe.
- **Delta selection**: The **Push** set is computed by comparing local VC vs remote VC; send only missing events.
- **Causality**: VC preserves "happens-before". If neither side's VC dominates, a **conflict** exists and the Resolver must decide.

*Tip:* Keep event payloads focused; project into read models/materialized views for queries.

---

## Conflict resolution strategies

- **Server-authoritative** – Server wins for contested fields/streams. Simple and predictable for centralized truth.
- **Last-Write-Wins (LWW)** – Prefer the newest by causal version (only use timestamps as a last-ditch tie-breaker).
- **Domain-specific merge** – Deterministic rules (e.g., additive counters, CRDT-like sets, "approved beats draft", etc.). Emit explicit **merge events** for auditability.

**Guidelines**
- Deterministic + idempotent
- Prefer causal data (versions) over wall-clock time
- Log conflicts with enough context to reproduce

---

## State machine signals

Nodes emit signals so you can observe and trace sync:

- **States**: `Idle` → `Pulling` → `Resolving` → `Pushing` → `Done` (or `Error`)
- **Key signals**:
  - `SyncStarted{ runID }`
  - `PullStarted/PullFinished{ nEvents, fromVC }`
  - `ResolveStarted/ResolveFinished{ nConflicts }`
  - `PushStarted/PushFinished{ nEvents }`
  - `SyncFinished{ result }` or `SyncFailed{ err }`

Export durations per state and counts per outcome to logs/metrics/tracing. Hook these to your UI spinners and health checks.

---

## Where to go next

- Quick start (in-memory & HTTP) in the README  
- Store/transport specific docs for configuration and performance tips  
- Deeper dives on conflict strategies and state machine behavior in this folder