// Package postgres provides a PostgreSQL-based persistent event store.
//
// PostgresEventStore is suitable for production deployments requiring durability,
// concurrency, and advanced querying. It implements the synckit.EventStore interface
// using PostgreSQL as the underlying database. Use NewWithDataSource() to create a store
// from a connection string.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package postgres
