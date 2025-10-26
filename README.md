# go-sync-kit

[![Go Reference](https://pkg.go.dev/badge/github.com/c0deZ3R0/go-sync-kit.svg)](https://pkg.go.dev/github.com/c0deZ3R0/go-sync-kit)
[![Go Report Card](https://goreportcard.com/badge/github.com/c0deZ3R0/go-sync-kit)](https://goreportcard.com/report/github.com/c0deZ3R0/go-sync-kit)

> **⚠️ DISCLAIMER:** This library is under active development and has not been thoroughly tested in production environments. **Breaking changes may occur between versions** as we iterate on the API design. Use with caution in production systems. We recommend:
> - Pinning to specific versions in your `go.mod`
> - Thoroughly testing in staging environments before deploying to production
> - Reviewing release notes and migration guides when upgrading
> 
> Contributions, bug reports, and production feedback are welcome!

Tiny, composable building blocks for **event sync** in Go.

- ✅ Simple mental model: **[Node](docs/overview.md#syncnode) → [Store](docs/overview.md#store) + [Transport](docs/overview.md#transport) (+ [Resolver](docs/overview.md#resolver))**
- ⚡ In-memory dev experience (no external deps)
- 🌐 HTTP presets for client/server
- 🔧 Pluggable conflict resolution
- 📦 Production-ready stores/transports

> **New to go-sync-kit?** Start with the [Architecture overview →](docs/overview.md)

---

## Table of Contents
- [Architecture overview](docs/overview.md)
- [Feature Status Matrix](docs/status.md) – Stable / Experimental / Planned
- [Why go-sync-kit?](#why-go-sync-kit)
- [Install](#install)
- [60-Second Quick Start (In-Memory)](#60-second-quick-start-in-memory)
- [HTTP Client/Server Quick Start](#http-clientserver-quick-start)
- [Core Concepts](#core-concepts)
- [Examples & Docs](#examples--docs)
- [Migration from SyncManager](#migration-from-syncmanager)
- [Stability & Versioning](#stability--versioning)
- [Contributing](#contributing)

---

## Why go-sync-kit?

You have data changing **locally** and **remotely**. You want to:
- push local events, pull remote events,
- resolve conflicts sanely,
- and not wire an entire distributed system framework.

**go-sync-kit** gives you *just enough*:
- A **Node** you start,
- a **Store** where events live,
- a **Transport** that moves them,
- an optional **Resolver** for conflicts.

---

## Install

```bash
go get github.com/c0deZ3R0/go-sync-kit@latest
```

---

## 60-Second Quick Start (In-Memory)

```go
package main

import (
	"context"
	"log"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// MyEvent implements the Event interface
type MyEvent struct {
	EventID   string
	EventType string
	UserID    string
	Name      string
}

func (e MyEvent) ID() string                      { return e.EventID }
func (e MyEvent) Type() string                    { return e.EventType }
func (e MyEvent) AggregateID() string             { return e.UserID }
func (e MyEvent) Data() interface{}               { return e }
func (e MyEvent) Metadata() map[string]interface{} { return nil }

func main() {
	ctx := context.Background()
	store := memstore.New()

	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithNullTransport(), // Local-only
		synckit.WithLWW(),
	)
	if err != nil { log.Fatal(err) }
	defer node.Close()

	// Store an event (memstore auto-generates sequential versions)
	event := MyEvent{EventID: "1", EventType: "demo", UserID: "user-123", Name: "demo event"}
	store.Store(ctx, event, cursor.IntegerCursor{})

	// Sync
	res, err := node.Sync(ctx)
	if err != nil { log.Fatal(err) }

	log.Printf("✅ Sync complete: EventsPushed=%d, EventsPulled=%d, ConflictsResolved=%d",
		res.EventsPushed, res.EventsPulled, res.ConflictsResolved)
}
```

See [examples/quickstart](examples/quickstart/README.md) to run this snippet yourself.

Want to see more in-memory patterns (hub, subscriptions)? See [`examples/inmem`](examples/inmem).

---

## HTTP Client/Server Quick Start

**Server**
```go
// examples/http_server/main.go
store, _ := sqlite.New(&sqlite.Config{DataSourceName: "server.db"})
transport := httptransport.NewTransport("", nil, nil, nil) // server mode
node, _ := synckit.NewHTTPServerNode(store, transport)
handler := httptransport.NewSyncHandler(store, nil, nil, nil)

http.Handle("/sync", handler)
log.Fatal(http.ListenAndServe(":8080", nil))
```

**Client**
```go
// examples/http_client/main.go
store, _ := sqlite.New(&sqlite.Config{DataSourceName: "client.db"})
transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)
node, _ := synckit.NewHTTPClientNode(store, transport)

res, _ := node.Sync(context.Background())
// pushed/pulled metrics in res
```

- **Full walk-throughs**: [`examples/HTTP_EXAMPLES.md`](examples/HTTP_EXAMPLES.md)
- **Production server with graceful shutdown**: [`examples/http_server/main_production.go`](examples/http_server/main_production.go)

### HTTP Enterprise Features (v0.24+)

The HTTP transport now includes production-ready enterprise features:

**✨ Structured Error Responses** – Standardized JSON error format with codes
```json
{"error": {"code": "INVALID_CURSOR", "message": "...", "op": "pull"}}
```

**🔍 Advanced Filtering** – Query by type, tenant, aggregate_id
```bash
curl "http://localhost:8080/sync/pull?since=42&type=OrderCreated&tenant=acme&limit=100"
```

**🏢 Multitenancy Support** – Tenant isolation via headers
```go
req.Header.Set("X-SyncKit-Tenant", "acme-corp")
```

**🔒 Idempotency Keys** – Prevent duplicate event processing
```go
req.Header.Set("Idempotency-Key", uuid.New().String())
```

**🛡️ Authentication Middleware** – Bearer token, HMAC, custom validators
```go
import "github.com/c0deZ3R0/go-sync-kit/transport/httptransport/middleware"

// Bearer token authentication
authMiddleware := middleware.BearerAuth(func(token string) (userID, tenantID string, err error) {
	return validateToken(token) // your validation logic
})

// Apply to handler
handler := middleware.Chain(
	httptransport.NewSyncHandler(store, nil, nil, nil),
	authMiddleware,
	middleware.TenantExtractor("X-SyncKit-Tenant"),
)
```

**📖 Full API Reference**: [`docs/http-spec.md`](docs/http-spec.md)  
**📖 Migration Guide**: [`docs/MIGRATION_GUIDE_HTTP.md`](docs/MIGRATION_GUIDE_HTTP.md)

---

## Core Concepts

**[SyncNode](docs/overview.md#syncnode)** – the participant you run. It exposes:
- `Sync(ctx)`, `Push(ctx)`, `Pull(ctx)`
- `StartAutoSync(ctx)`, `StopAutoSync()`, `Subscribe(...)`, `Close()`

**[Store](docs/overview.md#store)** – event persistence (e.g. `memstore`, `sqlite`, `postgres`).

**[Transport](docs/overview.md#transport)** – how events move (e.g. HTTP request/response, in-memory `memchan`, SSE for real-time subscriptions).

**[Resolver](docs/overview.md#resolver)** – strategy for conflicts (e.g. LWW). Pluggable.

---

## 📚 Documentation Index

**Getting Started**
- [Quick Start](examples/quickstart/README.md)
- [Installation](#install)

**Concepts**
- [Architecture Overview](docs/overview.md)
- [Conflict Resolution](docs/overview.md#conflict-resolution-strategies)
- [State Machine](docs/overview.md#state-machine-signals)
- [FAQ](docs/faq.md)
- [Best Practices](docs/best-practices.md)
- [Troubleshooting](docs/troubleshooting.md)

**How-Tos**
- [HTTP Transport](examples/http_server/README.md)
- [SSE (Realtime)](examples/HTTP_EXAMPLES.md)
- [Storage Backends](storage/README.md)
- [Observability & Metrics](examples/intermediate/09-advanced-observability/README.md)

**Examples**
- [Examples Directory Index](examples/README.md)

**Reference**
- [Go Reference (pkg.go.dev)](https://pkg.go.dev/github.com/c0deZ3R0/go-sync-kit)
- [CHANGELOG](CHANGELOG.md)
- [CONTRIBUTING](CONTRIBUTING.md)
- [Full Documentation Index](docs/README.md)

---

## Migration from SyncManager

SyncNode is the preferred API. It's a drop-in façade over SyncManager:

```go
// old (still works)
m, _ := synckit.NewManager(/* options */)

// new (preferred)
n, _ := synckit.NewNode(/* same options */)
```

SyncManager remains for compatibility but is deprecated in docs.

---

## Stability & Versioning

- **Semantic versioning**
- **Deprecated symbols** kept for at least one minor release before removal  
- **Go ≥ 1.21** recommended

---

## Contributing

PRs welcome! Please:
- keep examples minimal,
- prefer docs in `/examples` or `/docs`,
- add tests for new store/transport/resolver integrations.

---

License: MIT — see [LICENSE](LICENSE).

