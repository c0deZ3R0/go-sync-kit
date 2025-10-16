// Package memchan provides an in-memory channel-based transport.
//
// MemChanTransport is useful for testing, local development, and single-process
// deployments. It implements the synckit.Transport interface using Go channels
// for push, pull, and subscription operations.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package memchan
