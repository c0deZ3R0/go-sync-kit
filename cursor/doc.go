// Package cursor provides version and ordering abstractions for tracking event positions.
//
// Cursor implements the synckit/types.Version interface to support multiple versioning
// strategies: simple integer sequence numbers, vector clocks, and pluggable custom implementations.
// Cursors enable efficient event pagination, conflict detection, and causality tracking in
// distributed systems.
//
// # Wire Format
//
// Cursors can be marshaled to a JSON wire format (WireCursor) for transport over HTTP or
// other protocols. Use MarshalWire and UnmarshalWire for serialization.
//
// # Pluggable Codecs
//
// Custom cursor implementations can be registered via Register so they are available
// for wire round-trips. The built-in implementations are IntegerCursor and VectorCursor.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package cursor
