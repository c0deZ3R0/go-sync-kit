// Package sqlite provides a SQLite-based persistent event store.
//
// SQLiteEventStore is suitable for single-machine deployment, development, and
// embedded use cases. It implements the synckit.EventStore interface using SQLite
// as the underlying database. Use NewWithDataSource() to create a store from a database URL.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package sqlite
