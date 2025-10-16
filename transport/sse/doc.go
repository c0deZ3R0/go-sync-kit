// Package sse provides a Server-Sent Events transport.
//
// SSETransport implements the synckit.Transport interface using HTTP Server-Sent Events
// for efficient push-based event streaming. It's suitable for real-time synchronization
// where clients maintain long-lived connections to receive updates.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package sse
