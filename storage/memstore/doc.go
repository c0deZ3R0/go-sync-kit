// Package memstore provides an in-memory implementation of the go-sync-kit EventStore.
//
// MemStore is perfect for development, testing, demos, and single-process apps that don't
// require persistence or cross-process synchronization. It implements the synckit.EventStore
// interface and maintains all events in memory with thread-safe access.
//
// Use New() to create a new store. Events are assigned sequential IntegerCursor versions
// automatically. Close the store when done to release resources.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package memstore
