// Package storage provides persistent backends for event storage.
//
// The storage package defines interfaces and provides concrete implementations
// for persisting events: memstore (in-memory), sqlite (file-based), postgres (server-based),
// and badger (embedded key-value store). Each backend implements the synckit.EventStore interface.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package storage
