// Package errors provides custom error types for sync operations.
//
// The core type is SyncError, which wraps underlying errors with structured context
// including the operation, component, error kind, and retryability information.
// Use the E() builder function for flexible error construction, or type-specific
// constructors (NewStorageError, NewNetworkError, etc.) for common cases.
//
// # Error Classification
//
// Errors are classified by Kind (Invalid, NotFound, Permission, Internal, Timeout, etc.)
// and ErrorCode (NetworkFailure, StorageFailure, etc.). This enables downstream handlers
// to apply appropriate retry logic and user-facing messaging.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package errors
