# Storage Layer (Architecture)

What it is:
- The durable event log. Storage persists events and exposes a monotonic Version.
- The boundary for durability and ordering guarantees.

How it fits with SyncNode:
- Push: SyncNode asks Storage for local events since a Version, Transport sends them.
- Pull: Transport brings remote events, SyncNode stores them via Storage.
- LatestVersion: SyncNode queries Storage to know where to resume.
- ParseVersion: Storage translates version strings (e.g., from HTTP) into its Version type.

Contract (simplified):
- Store(e, v) → error
- Load(since) → []EventWithVersion
- LoadByAggregate(id, since) → []EventWithVersion
- LatestVersion() → Version
- ParseVersion(string) → Version
- Close() → error

Swap the backend, keep the contract:
- In-memory (dev), file-backed (SQLite), server DB (Postgres), embedded KV (Badger).
- Your app code talks to the interface, not the concrete store.

Minimal usage:
```go
node, _ := synckit.NewNode(
    synckit.WithStore(store),      // any EventStore impl
    synckit.WithTransport(transport),
    synckit.WithLWW(),
)
```

Notes:
- Append-only mindset: stores return events in order and advance Version monotonically.
- Thread-safe: implementations are safe for concurrent Sync/Push/Pull.
- Choice guide: dev = memstore, single-node = sqlite, multi-node = postgres, high-perf embedded = badger.

---

# Storage - Event Persistence for go-sync-kit

Easy-to-understand guide for choosing and using storage backends.

---

## 🎯 Quick Start - Which Storage Should I Use?

| Scenario | Use This | Why? |
|----------|----------|------|
| 🧪 **Learning / Prototyping** | [memstore](memstore/) | Zero setup, no dependencies |
| 💻 **Single-node app / Desktop** | [sqlite](sqlite/) | Simple, reliable, one file |
| 🌐 **Multi-node / Production** | [postgres](postgres/) | Scalable, LISTEN/NOTIFY support |
| 🚀 **High-performance / Embedded** | [badger](badger/) | Fast, pure Go, no SQL |

---

## 📦 Available Storage Implementations

### 1. MemStore (In-Memory) - **Best for Development** ✨

**Location**: `storage/memstore/`

```go
import "github.com/c0deZ3R0/go-sync-kit/storage/memstore"

store := memstore.New()
```

**Pros**:
- ✅ Zero external dependencies
- ✅ Instant setup - no configuration
- ✅ Perfect for testing and examples
- ✅ Thread-safe
- ✅ 89.3% test coverage

**Cons**:
- ❌ Data lost on restart (no persistence)
- ❌ Memory-only (not for production)

**When to use**: Development, testing, quick demos, CI/CD tests

---

### 2. SQLite - **Best for Single-Node Apps** 🗄️

**Location**: `storage/sqlite/`

```go
import "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"

// Quick start
store, err := sqlite.NewWithDataSource("events.db")

// Production config
store, err := sqlite.New(&sqlite.Config{
    DataSourceName: "events.db",
    EnableWAL:      true,  // Better concurrency
    MaxOpenConns:   25,
})
```

**Pros**:
- ✅ Single file database (easy backup/restore)
- ✅ No server setup required
- ✅ Battle-tested and reliable
- ✅ WAL mode for concurrent reads/writes
- ✅ Production-ready

**Cons**:
- ❌ Single-node only (no distributed support)
- ❌ CGo dependency (cross-compilation complexity)

**When to use**: Desktop apps, single-server deployments, embedded systems

**Read more**: [SQLite README](sqlite/README.md)

---

### 3. PostgreSQL - **Best for Production** 🐘

**Location**: `storage/postgres/`

```go
import "github.com/c0deZ3R0/go-sync-kit/storage/postgres"

store, err := postgres.New("postgres://user:pass@localhost/mydb")
```

**Pros**:
- ✅ Multi-node / distributed support
- ✅ LISTEN/NOTIFY for real-time sync
- ✅ Battle-tested at scale
- ✅ Advanced querying capabilities
- ✅ Built-in replication

**Cons**:
- ❌ Requires PostgreSQL server
- ❌ More complex setup
- ❌ Higher resource usage

**When to use**: Multi-server deployments, high availability requirements, large scale

**Read more**: [PostgreSQL README](postgres/README.md)

---

### 4. BadgerDB - **Best for High Performance** ⚡

**Location**: `storage/badger/`

```go
import "github.com/c0deZ3R0/go-sync-kit/storage/badger"

store, err := badger.New("/path/to/data")
```

**Pros**:
- ✅ Pure Go (easy cross-compilation)
- ✅ High-performance LSM-tree storage
- ✅ Embedded (no server needed)
- ✅ Built-in compression
- ✅ ACID transactions

**Cons**:
- ❌ Larger binary size
- ❌ More memory usage than SQLite

**When to use**: High-throughput apps, embedded systems, pure Go requirement

**Read more**: [BadgerDB README](badger/README.md)

---

## 🚀 Usage Examples

### Development Flow (In-Memory)

Perfect for getting started quickly:

```go
package main

import (
    "context"
    "log"
    
    "github.com/c0deZ3R0/go-sync-kit/storage/memstore"
    "github.com/c0deZ3R0/go-sync-kit/transport/memchan"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func main() {
    // Zero setup - just start coding!
    store := memstore.New()
    transport := memchan.New(16)
    
    node, err := synckit.NewInMemoryNode(store, transport)
    if err != nil {
        log.Fatal(err)
    }
    defer node.Close()
    
    // Start syncing
    result, err := node.Sync(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Synced: %d pushed, %d pulled", 
        result.EventsPushed, result.EventsPulled)
}
```

### Production Flow (SQLite or PostgreSQL)

When you're ready for persistence:

```go
package main

import (
    "context"
    "log"
    
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    "github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func main() {
    // SQLite for single-node
    store, err := sqlite.NewWithDataSource("events.db")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()
    
    // HTTP transport for client/server
    transport := httptransport.NewTransport(
        "http://server:8080/sync", 
        nil, nil, nil,
    )
    
    node, err := synckit.NewHTTPClientNode(store, transport)
    if err != nil {
        log.Fatal(err)
    }
    defer node.Close()
    
    // Sync with server
    result, err := node.Sync(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Synced: %d pushed, %d pulled", 
        result.EventsPushed, result.EventsPulled)
}
```

---

## 🔌 The EventStore Interface

All storage implementations satisfy this interface:

```go
type EventStore interface {
    // Store saves an event with a version
    Store(ctx context.Context, event Event, version Version) error
    
    // Load retrieves all events since a version
    Load(ctx context.Context, since Version) ([]EventWithVersion, error)
    
    // LoadByAggregate retrieves events for a specific aggregate
    LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error)
    
    // LatestVersion returns the highest version in the store
    LatestVersion(ctx context.Context) (Version, error)
    
    // ParseVersion converts a string to a Version (for HTTP, etc.)
    ParseVersion(ctx context.Context, s string) (Version, error)
    
    // Close closes the store and releases resources
    Close() error
}
```

**What this means**: Switch storage backends by just changing the constructor - the rest of your code stays the same!

---

## 🔄 Switching Storage Backends

Switching is as easy as changing one line:

```go
// Development (in-memory)
store := memstore.New()

// Single-node production (SQLite)
store, _ := sqlite.NewWithDataSource("events.db")

// Multi-node production (PostgreSQL)
store, _ := postgres.New("postgres://localhost/db")

// High-performance (BadgerDB)
store, _ := badger.New("/data/path")

// Everything else stays the same!
node, _ := synckit.NewNode(
    synckit.WithStore(store),
    synckit.WithTransport(transport),
    synckit.WithLWW(),
)
```

---

## 📊 Feature Comparison

| Feature | MemStore | SQLite | PostgreSQL | BadgerDB |
|---------|----------|--------|------------|----------|
| **Setup Complexity** | None | Low | Medium | Low |
| **External Deps** | None | CGo | Server | None |
| **Persistence** | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Multi-node** | ❌ No | ❌ No | ✅ Yes | ❌ No |
| **Concurrent Writes** | ✅ Fast | ✅ Good | ✅ Excellent | ✅ Excellent |
| **Real-time Events** | ✅ Built-in | ❌ No | ✅ LISTEN/NOTIFY | ❌ No |
| **Transactions** | N/A | ✅ Yes | ✅ Yes | ✅ Yes |
| **Cross-compile** | ✅ Easy | ⚠️ Harder | ✅ Easy | ✅ Easy |
| **Binary Size** | Tiny | Small | Small | Large |
| **Memory Usage** | Low | Low | Medium | Higher |
| **Test Coverage** | 89.3% | 45.9% | - | - |

---

## 💡 Common Patterns

### Pattern 1: Development → Production Migration

```go
// Start development with in-memory
func newDevStore() synckit.EventStore {
    return memstore.New()
}

// Switch to production with environment variable
func newStore() synckit.EventStore {
    if os.Getenv("ENV") == "development" {
        return memstore.New()
    }
    
    store, err := sqlite.NewWithDataSource(
        os.Getenv("DB_PATH"),
    )
    if err != nil {
        log.Fatal(err)
    }
    return store
}
```

### Pattern 2: Multi-tenant with Separate Databases

```go
func getTenantStore(tenantID string) (synckit.EventStore, error) {
    dbPath := fmt.Sprintf("data/%s.db", tenantID)
    return sqlite.NewWithDataSource(dbPath)
}
```

### Pattern 3: Testing with In-Memory

```go
func TestMyFeature(t *testing.T) {
    // Always use memstore for tests - fast and clean
    store := memstore.New()
    transport := memchan.New(16)
    
    node, _ := synckit.NewInMemoryNode(store, transport)
    defer node.Close()
    
    // Your test code here
}
```

---

## 🎓 Learning Path

1. **Start Here**: Use `memstore` to understand concepts
2. **Next Step**: Add persistence with `sqlite` 
3. **Scale Up**: Move to `postgres` when you need multiple nodes
4. **Optimize**: Consider `badger` for high-performance needs

Each README in the subdirectories has detailed examples and configuration options.

---

## 🔍 Need Help Choosing?

### Choose **MemStore** if:
- 🧪 You're learning or prototyping
- 🧪 Writing tests or examples
- 🧪 Don't need persistence

### Choose **SQLite** if:
- 💻 Building a desktop application
- 💻 Single server deployment
- 💻 Want simple backup (just copy the .db file)

### Choose **PostgreSQL** if:
- 🌐 Multiple servers syncing together
- 🌐 Need real-time LISTEN/NOTIFY
- 🌐 High availability requirements

### Choose **BadgerDB** if:
- ⚡ Need maximum performance
- ⚡ Pure Go requirement
- ⚡ Embedded high-throughput app

---

## 📚 Additional Resources

- **Main README**: [../README.md](../README.md)
- **WARP.md**: [../WARP.md](../WARP.md) - Complete architecture guide
- **Examples**: [../examples/](../examples/) - Working code samples

---

**Quick Links**:
- [MemStore Documentation](memstore/)
- [SQLite Documentation](sqlite/README.md)
- [PostgreSQL Documentation](postgres/README.md)
- [BadgerDB Documentation](badger/README.md)

---

**Happy Syncing! 🚀**
