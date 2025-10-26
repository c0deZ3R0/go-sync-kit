# Realtime Auto-Sync

Demonstrates timers, background sync, and graceful shutdown patterns.

## Run

```bash
cd examples/intermediate/05-realtime-autosync
go run .
```

## What It Does

- Automatic periodic sync with configurable intervals
- Background goroutine management
- Graceful shutdown on SIGINT/SIGTERM
- Uses SQLite for persistence

**Artifact**: Creates `autosync.db` in the current directory (safe to delete between runs).

## Key Concepts

- `StartAutoSync()` / `StopAutoSync()`
- Signal handling
- Context cancellation
- Clean resource cleanup

See also: [State Machine Enhancements](../10-state-machine-enhancements/README.md)
