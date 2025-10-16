// Package errors provides custom error types for the sync package
package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents the type of error that occurred.
type ErrorCode string

const (
	// ErrCodeNetworkFailure indicates a network-related error.
	ErrCodeNetworkFailure ErrorCode = "NETWORK_FAILURE"
	// ErrCodeStorageFailure indicates a storage-related error.
	ErrCodeStorageFailure ErrorCode = "STORAGE_FAILURE"
	// ErrCodeConflictFailure indicates a conflict resolution error.
	ErrCodeConflictFailure ErrorCode = "CONFLICT_FAILURE"
	// ErrCodeValidationFailure indicates a validation error.
	ErrCodeValidationFailure ErrorCode = "VALIDATION_FAILURE"
)

// Kind represents the category of error for structured error handling.
type Kind string

const (
	// KindInvalid indicates invalid input or request.
	KindInvalid Kind = "INVALID"
	// KindNotFound indicates a resource not found.
	KindNotFound Kind = "NOT_FOUND"
	// KindPermission indicates permission denied.
	KindPermission Kind = "PERMISSION"
	// KindInternal indicates an internal server error.
	KindInternal Kind = "INTERNAL"
	// KindTimeout indicates an operation timeout.
	KindTimeout Kind = "TIMEOUT"
	// KindUnavailable indicates service unavailable.
	KindUnavailable Kind = "UNAVAILABLE"
	// KindConflict indicates a resource conflict.
	KindConflict Kind = "CONFLICT"
	// KindTooLarge indicates request too large.
	KindTooLarge Kind = "TOO_LARGE"
	// KindMethodNotAllowed indicates HTTP method not allowed.
	KindMethodNotAllowed Kind = "METHOD_NOT_ALLOWED"
)

// Operation represents the type of sync operation.
type Operation string

const (
	// OpSync is a general sync operation.
	OpSync Operation = "sync"
	// OpPush is a push operation.
	OpPush Operation = "push"
	// OpPull is a pull operation.
	OpPull Operation = "pull"
	// OpStore is a store operation.
	OpStore Operation = "store"
	// OpLoad is a load operation.
	OpLoad Operation = "load"
	// OpConflictResolve is a conflict resolution operation.
	OpConflictResolve Operation = "conflict_resolve"
	// OpTransport is a transport operation.
	OpTransport Operation = "transport"
	// OpClose is a close operation.
	OpClose Operation = "close"
	// OpProjection is a projection operation.
	OpProjection Operation = "projection"
	// OpProjectionApply is a projection apply operation.
	OpProjectionApply Operation = "projection_apply"
	// OpOffsetStore is an offset store operation.
	OpOffsetStore Operation = "offset_store"
)

// SyncError represents an error that occurred during synchronization.
type SyncError struct {
	// Operation during which the error occurred.
	Op Operation

	// Component that generated the error (e.g., "store", "transport").
	Component string

	// Err is the underlying error.
	Err error

	// Retryable indicates whether the operation can be retried.
	Retryable bool

	// Code is the error code for the error type.
	Code ErrorCode

	// Kind represents the category of error.
	Kind Kind

	// Metadata holds additional context.
	Metadata map[string]interface{}
}

// Error implements the error interface.
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

// Unwrap returns the underlying error.
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

// IsRetryable checks if an error is a retryable SyncError.
func IsRetryable(err error) bool {
	var syncErr *SyncError
	if errors.As(err, &syncErr) {
		return syncErr.Retryable
	}
	return false
}

// E creates a structured error using a builder pattern.
// This function provides a flexible way to create errors with multiple attributes.
// Arguments can be provided in any order and include:
//   - Operation: specifies the operation during which the error occurred
//   - Component: specifies the component that generated the error (string)
//   - Kind: specifies the category of error
//   - error: the underlying error
//   - string: additional context message
//
// Example:
//
//	err := E(Op("httptransport.handlePush"), Component("httptransport"), Kind(KindInvalid), underlyingErr, "decode request")
func E(args ...interface{}) *SyncError {
	e := &SyncError{}
	var contextMsgs []string

	for _, arg := range args {
		switch a := arg.(type) {
		case Operation:
			e.Op = a
		case Kind:
			e.Kind = a
		case ErrorCode:
			e.Code = a
		case componentArg:
			e.Component = string(a)
		case error:
			e.Err = a
		case string:
			contextMsgs = append(contextMsgs, a)
		}
	}

	// If we have context messages but no underlying error, create one from the context
	if e.Err == nil && len(contextMsgs) > 0 {
		e.Err = fmt.Errorf("%s", contextMsgs[0])
	} else if len(contextMsgs) > 0 {
		// Wrap the underlying error with context
		e.Err = fmt.Errorf("%s: %w", contextMsgs[0], e.Err)
	}

	// Set default retryable based on Kind
	switch e.Kind {
	case KindInternal, KindTimeout, KindUnavailable:
		e.Retryable = true
	case KindInvalid, KindNotFound, KindPermission, KindConflict, KindTooLarge, KindMethodNotAllowed:
		e.Retryable = false
	default:
		// Default to non-retryable for safety
		e.Retryable = false
	}

	return e
}

// Helper functions for creating typed arguments to E()

// Op creates an Operation argument for E()
func Op(op string) Operation {
	return Operation(op)
}

// Component creates a component string argument for E() that's clearly distinguished from context messages
func Component(component string) componentArg {
	return componentArg(component)
}

// componentArg is a distinct type to differentiate component names from context messages.
type componentArg string
