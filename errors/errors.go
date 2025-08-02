// Package errors provides custom error types for the sync package
package errors

import (
	"errors"
	"fmt"
)

// Operation represents a sync operation type
type Operation string

const (
	OpSync            Operation = "sync"
	OpPush            Operation = "push"
	OpPull            Operation = "pull"
	OpStore           Operation = "store"
	OpLoad            Operation = "load"
	OpConflictResolve Operation = "conflict_resolve"
	OpTransport       Operation = "transport"
	OpClose           Operation = "close"
)

// SyncError represents an error that occurred during synchronization
type SyncError struct {
	// Operation during which the error occurred
	Op Operation

	// Component that generated the error (e.g., "store", "transport")
	Component string

	// Underlying error
	Err error

	// Whether the operation can be retried
	Retryable bool
}

func (e *SyncError) Error() string {
	if e.Component != "" {
		return fmt.Sprintf("%s operation failed in %s component: %v", e.Op, e.Component, e.Err)
	}
	return fmt.Sprintf("%s operation failed: %v", e.Op, e.Err)
}

func (e *SyncError) Unwrap() error {
	return e.Err
}

// New creates a new SyncError
func New(op Operation, err error) *SyncError {
	return &SyncError{
		Op:  op,
		Err: err,
	}
}

// NewWithComponent creates a new SyncError with component information
func NewWithComponent(op Operation, component string, err error) *SyncError {
	return &SyncError{
		Op:        op,
		Component: component,
		Err:       err,
	}
}

// NewRetryable creates a new retryable SyncError
func NewRetryable(op Operation, err error) *SyncError {
	return &SyncError{
		Op:        op,
		Err:       err,
		Retryable: true,
	}
}

// IsRetryable checks if an error is a retryable SyncError
func IsRetryable(err error) bool {
	var syncErr *SyncError
	if errors.As(err, &syncErr) {
		return syncErr.Retryable
	}
	return false
}
