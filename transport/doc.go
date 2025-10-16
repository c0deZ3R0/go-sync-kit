// Package transport provides network transport backends for event synchronization.
//
// The transport package defines interfaces for Push, Pull, and Subscribe operations,
// with implementations for HTTP, WebSockets, Server-Sent Events (SSE), RabbitMQ, and
// in-memory channels. Each backend implements the synckit.Transport interface.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package transport
