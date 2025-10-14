# Quickstart Example

This example demonstrates the minimal steps to get started with go-sync-kit, following the *pull → resolve → push* flow described in the [Architecture Overview](../../docs/overview.md):

1. **Define a custom event** implementing the Event interface
2. **Store it locally** using an in-memory store
3. **Run a full sync** using SyncNode (the new façade)

## Run it directly:

```bash
go run ./examples/quickstart
```

## Expected output:

```
📝 Stored event: demo event (type: demo, user: user-123)
✅ Sync complete: EventsPushed=1, EventsPulled=0, ConflictsResolved=0
```

**Why EventsPushed=1?** The sync pipeline executes the full *pull → resolve → push* flow. During the push phase, the local event is "pushed" to the transport layer. Since `WithNullTransport()` is a no-op transport (no network), the push succeeds immediately without actually sending data anywhere. EventsPulled=0 because there's no remote to pull from. This demonstrates that the sync machinery works correctly even in a local-only scenario.

## Key concepts shown:

- **Custom Event**: `MyEvent` struct implements the `Event` interface with required methods
- **In-memory Storage**: Uses `memstore.New()` for zero-dependency development
- **SyncNode**: The preferred API façade over the older `SyncManager`
- **Null Transport**: Local-only operation with `WithNullTransport()`
- **Conflict Resolution**: Last-Write-Wins strategy with `WithLWW()`

## Next steps:

- Try the HTTP client/server examples for network sync
- Explore different storage backends (SQLite, Postgres)
- Read the [Architecture Overview](../../docs/overview.md) for deeper understanding