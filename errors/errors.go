// Package errors provides custom error types for the sync package
package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents the type of error that occurred
type ErrorCode string

const (
	ErrCodeNetworkFailure   ErrorCode = "NETWORK_FAILURE"
	ErrCodeStorageFailure   ErrorCode = "STORAGE_FAILURE"
	ErrCodeConflictFailure  ErrorCode = "CONFLICT_FAILURE"
	ErrCodeValidationFailure ErrorCode = "VALIDATION_FAILURE"
)

// Operation represents the type of sync operation
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

	// Error code for the error type
	Code ErrorCode

	// Metadata for additional context
	Metadata map[string]interface{}
}

func (e *SyncError) Error() string {
	var msg string
	if e.Component != "" {
		msg = fmt.Sprintf("%s operation failed in %s component", e.Op, e.Component)
	} else {
		msg = fmt.Sprintf("%s operation failed", e.Op)
	}
	
	if e.Code != "" {
		msg += fmt.Sprintf(" [%s]", e.Code)
	}
	
	return msg + fmt.Sprintf(": %v", e.Err)
}

func (e *SyncError) Unwrap() error {
	return e.Err
}

// NewStorageError creates a new storage-related SyncError
func NewStorageError(op Operation, cause error) *SyncError {
	return &SyncError{
		Code:      ErrCodeStorageFailure,
		Op:        op,
		Component: "store",
		Err:       cause,
		Retryable: true,
	}
}

// NewConflictError creates a new conflict-related SyncError
func NewConflictError(op Operation, cause error) *SyncError {
	return &SyncError{
		Code:      ErrCodeConflictFailure,
		Op:        op,
		Component: "sync",
		Err:       cause,
		Retryable: false,
	}
}

// NewValidationError creates a new validation-related SyncError
func NewValidationError(op Operation, cause error) *SyncError {
	return &SyncError{
		Code:      ErrCodeValidationFailure,
		Op:        op,
		Err:       cause,
		Retryable: false,
	}
}

// NewNetworkError creates a new network-related SyncError
func NewNetworkError(op Operation, cause error) *SyncError {
	return &SyncError{
		Code:      ErrCodeNetworkFailure,
		Op:        op,
		Component: "transport",
		Err:       cause,
		Retryable: true,
	}
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
