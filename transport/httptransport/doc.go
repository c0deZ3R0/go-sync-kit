// Package httptransport provides HTTP-based transport for sync operations.
//
// It implements the synckit.Transport interface using JSON over HTTP, supporting
// both client and server roles with structured error responses and request/response
// marshaling.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package httptransport
