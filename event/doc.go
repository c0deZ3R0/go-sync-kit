// Package event provides concrete event types for the sync kit.
//
// The Event struct is a ready-to-use implementation of the synckit/types.Event interface,
// with support for standard fields: ID, Type, AggregateID, Data, Metadata, and Timestamp.
// Use New() or NewWithMetadata() to create events.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package event
