// Package projection provides event projection and materialized view support.
//
// Projections allow deriving read-optimized data models from event streams, supporting
// eventual consistency and multiple backend implementations (in-memory, BadgerDB).
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package projection
