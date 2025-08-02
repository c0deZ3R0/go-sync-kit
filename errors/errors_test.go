package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestSyncError_Error(t *testing.T) {
	tests := []struct {
		name      string
		op        Operation
		component string
		code      ErrorCode
		err       error
		want      string
	}{
		{
			name:      "with component and code",
			op:        OpSync,
			component: "store",
			code:      ErrCodeStorageFailure,
			err:       fmt.Errorf("failed to connect"),
			want:      "sync operation failed in store component [STORAGE_FAILURE]: failed to connect",
		},
		{
			name:      "with component no code",
			op:        OpSync,
			component: "store",
			err:       fmt.Errorf("failed to connect"),
			want:      "sync operation failed in store component: failed to connect",
		},
		{
			name: "without component with code",
			op:   OpPush,
			code: ErrCodeNetworkFailure,
			err:  fmt.Errorf("network error"),
			want: "push operation failed [NETWORK_FAILURE]: network error",
		},
		{
			name: "without component or code",
			op:   OpPush,
			err:  fmt.Errorf("network error"),
			want: "push operation failed: network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &SyncError{
				Op:        tt.op,
				Component: tt.component,
				Err:       tt.err,
				Code:      tt.code,
			}

			if got := e.Error(); got != tt.want {
				t.Errorf("SyncError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewNetworkError(t *testing.T) {
	cause := fmt.Errorf("network failure")
	syncErr := NewNetworkError(OpTransport, cause)

	if syncErr.Code != ErrCodeNetworkFailure {
		t.Errorf("NewNetworkError() Code = %v, want %v", syncErr.Code, ErrCodeNetworkFailure)
	}
	if syncErr.Component != "transport" {
		t.Errorf("NewNetworkError() Component = %v, want %v", syncErr.Component, "transport")
	}
	if syncErr.Err != cause {
		t.Errorf("NewNetworkError() Err = %v, want %v", syncErr.Err, cause)
	}
	if !syncErr.Retryable {
		t.Error("NewNetworkError() created non-retryable error")
	}
}

func TestNewStorageError(t *testing.T) {
	cause := fmt.Errorf("storage failure")
	syncErr := NewStorageError(OpStore, cause)

	if syncErr.Code != ErrCodeStorageFailure {
		t.Errorf("NewStorageError() Code = %v, want %v", syncErr.Code, ErrCodeStorageFailure)
	}
	if syncErr.Component != "store" {
		t.Errorf("NewStorageError() Component = %v, want %v", syncErr.Component, "store")
	}
	if syncErr.Err != cause {
		t.Errorf("NewStorageError() Err = %v, want %v", syncErr.Err, cause)
	}
	if !syncErr.Retryable {
		t.Error("NewStorageError() created non-retryable error")
	}
}

func TestNewConflictError(t *testing.T) {
	cause := fmt.Errorf("conflict detected")
	syncErr := NewConflictError(OpSync, cause)

	if syncErr.Code != ErrCodeConflictFailure {
		t.Errorf("NewConflictError() Code = %v, want %v", syncErr.Code, ErrCodeConflictFailure)
	}
	if syncErr.Component != "sync" {
		t.Errorf("NewConflictError() Component = %v, want %v", syncErr.Component, "sync")
	}
	if syncErr.Err != cause {
		t.Errorf("NewConflictError() Err = %v, want %v", syncErr.Err, cause)
	}
	if syncErr.Retryable {
		t.Error("NewConflictError() created retryable error when it shouldn't")
	}
}

func TestNewValidationError(t *testing.T) {
	cause := fmt.Errorf("validation failed")
	syncErr := NewValidationError(OpSync, cause)

	if syncErr.Code != ErrCodeValidationFailure {
		t.Errorf("NewValidationError() Code = %v, want %v", syncErr.Code, ErrCodeValidationFailure)
	}
	if syncErr.Err != cause {
		t.Errorf("NewValidationError() Err = %v, want %v", syncErr.Err, cause)
	}
	if syncErr.Retryable {
		t.Error("NewValidationError() created retryable error when it shouldn't")
	}
}

func TestSyncError_Unwrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	e := &SyncError{
		Op:  OpSync,
		Err: originalErr,
	}

	if unwrapped := e.Unwrap(); unwrapped != originalErr {
		t.Errorf("SyncError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable sync error",
			err:  NewRetryable(OpSync, fmt.Errorf("temporary error")),
			want: true,
		},
		{
			name: "non-retryable sync error",
			err:  New(OpSync, fmt.Errorf("permanent error")),
			want: false,
		},
		{
			name: "non-sync error",
			err:  fmt.Errorf("regular error"),
			want: false,
		},
		{
			name: "wrapped retryable error",
			err:  fmt.Errorf("wrapped: %w", NewRetryable(OpSync, fmt.Errorf("temporary"))),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorConstructors(t *testing.T) {
	originalErr := fmt.Errorf("test error")

	t.Run("New", func(t *testing.T) {
		e := New(OpSync, originalErr)
		if e.Op != OpSync {
			t.Errorf("New() Op = %v, want %v", e.Op, OpSync)
		}
		if e.Err != originalErr {
			t.Errorf("New() Err = %v, want %v", e.Err, originalErr)
		}
		if e.Retryable {
			t.Error("New() created retryable error when it shouldn't")
		}
	})

	t.Run("NewWithComponent", func(t *testing.T) {
		e := NewWithComponent(OpSync, "store", originalErr)
		if e.Op != OpSync {
			t.Errorf("NewWithComponent() Op = %v, want %v", e.Op, OpSync)
		}
		if e.Component != "store" {
			t.Errorf("NewWithComponent() Component = %v, want %v", e.Component, "store")
		}
		if e.Err != originalErr {
			t.Errorf("NewWithComponent() Err = %v, want %v", e.Err, originalErr)
		}
	})

	t.Run("NewRetryable", func(t *testing.T) {
		e := NewRetryable(OpSync, originalErr)
		if !e.Retryable {
			t.Error("NewRetryable() created non-retryable error")
		}
		if e.Op != OpSync {
			t.Errorf("NewRetryable() Op = %v, want %v", e.Op, OpSync)
		}
		if e.Err != originalErr {
			t.Errorf("NewRetryable() Err = %v, want %v", e.Err, originalErr)
		}
	})
}

func TestErrorsAs(t *testing.T) {
	var syncErr *SyncError
	err := fmt.Errorf("wrapped: %w", New(OpSync, fmt.Errorf("inner")))
	
	if !errors.As(err, &syncErr) {
		t.Error("errors.As() failed to detect SyncError")
	}
	
	if syncErr.Op != OpSync {
		t.Errorf("errors.As() Op = %v, want %v", syncErr.Op, OpSync)
	}
}
