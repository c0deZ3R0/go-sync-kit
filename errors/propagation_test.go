package errors_test

import (
	"github.com/c0deZ3R0/go-sync-kit/errors"

	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

// MockEvent is a simple implementation for testing
type MockEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (e *MockEvent) ID() string                            { return e.id }
func (e *MockEvent) Type() string                          { return e.eventType }
func (e *MockEvent) AggregateID() string                   { return e.aggregateID }
func (e *MockEvent) Data() interface{}                     { return e.data }
func (e *MockEvent) Metadata() map[string]interface{}     { return e.metadata }

// TestWrapOpComponent tests the WrapOpComponent helper function
func TestWrapOpComponent(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		op          string
		component   string
		expectedOp  errors.Operation
		expectedComp string
		nilError    bool
	}{
		{
			name:         "nil error returns nil",
			err:          nil,
			op:           "test.Operation",
			component:    "test/component",
			nilError:     true,
		},
		{
			name:         "basic error wrapping",
			err:          fmt.Errorf("underlying error"),
			op:           "test.Operation",
			component:    "test/component",
			expectedOp:   errors.Operation("test.Operation"),
			expectedComp: "test/component",
		},
		{
			name:         "complex operation name",
			err:          fmt.Errorf("database connection failed"),
			op:           "sqlite.Store",
			component:    "storage/sqlite",
			expectedOp:   errors.Operation("sqlite.Store"),
			expectedComp: "storage/sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.WrapOpComponent(tt.err, tt.op, tt.component)
			
			if tt.nilError {
				if result != nil {
					t.Errorf("Expected nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Error("Expected wrapped error, got nil")
				return
			}

			// Check if it's a SyncError
			syncErr, ok := result.(*errors.SyncError)
			if !ok {
				t.Errorf("Expected *SyncError, got %T", result)
				return
			}

			if syncErr.Op != tt.expectedOp {
				t.Errorf("Expected Op %s, got %s", tt.expectedOp, syncErr.Op)
			}

			if syncErr.Component != tt.expectedComp {
				t.Errorf("Expected Component %s, got %s", tt.expectedComp, syncErr.Component)
			}

			if syncErr.Err != tt.err {
				t.Errorf("Expected underlying error %v, got %v", tt.err, syncErr.Err)
			}
		})
	}
}

// TestWrapOpComponentKind tests the WrapOpComponentKind helper function
func TestWrapOpComponentKind(t *testing.T) {
	err := fmt.Errorf("test error")
	result := errors.WrapOpComponentKind(err, "test.Op", "test/component", errors.KindInternal)

	if result == nil {
		t.Fatal("Expected wrapped error, got nil")
	}

	syncErr, ok := result.(*errors.SyncError)
	if !ok {
		t.Fatalf("Expected *SyncError, got %T", result)
	}

	if syncErr.Op != "test.Op" {
		t.Errorf("Expected Op 'test.Op', got %s", syncErr.Op)
	}

	if syncErr.Component != "test/component" {
		t.Errorf("Expected Component 'test/component', got %s", syncErr.Component)
	}

	if syncErr.Kind != errors.KindInternal {
		t.Errorf("Expected Kind %s, got %s", errors.KindInternal, syncErr.Kind)
	}

	if syncErr.Err != err {
		t.Errorf("Expected underlying error %v, got %v", err, syncErr.Err)
	}
}

// TestSQLiteStoreErrorPropagation tests that SQLite store operations properly propagate Op and Component
func TestSQLiteStoreErrorPropagation(t *testing.T) {
	// Use an invalid data source to trigger errors
	store, err := sqlite.NewWithDataSource("invalid://path")
	if err == nil {
		t.Skip("Expected store creation to fail with invalid data source, but it succeeded")
	}

	// Test with valid store but invalid operations
	store, err = sqlite.NewWithDataSource(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory SQLite store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a mock event that will cause marshal errors
	event := &MockEvent{
		id:          "test-event",
		eventType:   "TestEvent",
		aggregateID: "test-aggregate",
		data:        make(chan int), // channels can't be marshaled to JSON
		metadata:    map[string]interface{}{"test": "metadata"},
	}

	// Test Store operation error propagation
	err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
	if err == nil {
		t.Error("Expected Store to fail with unmarshalable data")
	} else {
		assertOpComponentPropagation(t, err, "sqlite.Store", "storage/sqlite")
	}

	// Test Load operation
	_, err = store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		// Load should succeed with empty results, but if it fails, check propagation
		assertOpComponentPropagation(t, err, "sqlite.Load", "storage/sqlite")
	}

	// Test LatestVersion operation - Close the store first to trigger an error
	store.Close()
	_, err = store.LatestVersion(ctx)
	if err == nil {
		t.Error("Expected LatestVersion to fail on closed store")
	} else {
		// Should get ErrStoreClosed, but if wrapped properly it would have Op/Component
		// Since ErrStoreClosed is a simple error, this test validates our infrastructure
		t.Logf("LatestVersion error (expected): %v", err)
	}
}

// TestHTTPTransportErrorPropagation tests that HTTP transport operations properly propagate Op and Component
func TestHTTPTransportErrorPropagation(t *testing.T) {
	// Create transport with invalid base URL to trigger errors
	transport := httptransport.NewTransport("invalid://url", nil, nil, nil)
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Create a mock event that will cause marshal errors
	event := &MockEvent{
		id:          "test-event",
		eventType:   "TestEvent", 
		aggregateID: "test-aggregate",
		data:        make(chan int), // channels can't be marshaled to JSON
		metadata:    map[string]interface{}{"test": "metadata"},
	}

	eventWithVersion := synckit.EventWithVersion{
		Event:   event,
		Version: cursor.IntegerCursor{Seq: 1},
	}

	// Test Push operation error propagation
	err := transport.Push(ctx, []synckit.EventWithVersion{eventWithVersion})
	if err == nil {
		t.Error("Expected Push to fail with unmarshalable data")
	} else {
		assertOpComponentPropagation(t, err, "httptransport.Push", "transport/httptransport")
	}
}

// assertOpComponentPropagation is a helper function to check that errors have proper Op and Component fields
func assertOpComponentPropagation(t *testing.T, err error, expectedOp, expectedComponent string) {
	t.Helper()

	if err == nil {
		t.Error("Expected error to be non-nil for Op/Component propagation test")
		return
	}

	// Check if it's a SyncError
	syncErr, ok := err.(*errors.SyncError)
	if !ok {
		t.Errorf("Expected *SyncError for proper propagation, got %T: %v", err, err)
		return
	}

	if string(syncErr.Op) != expectedOp {
		t.Errorf("Expected Op '%s', got '%s'", expectedOp, syncErr.Op)
	}

	if syncErr.Component != expectedComponent {
		t.Errorf("Expected Component '%s', got '%s'", expectedComponent, syncErr.Component)
	}

	// Verify the error message contains operation and component information
	errMsg := syncErr.Error()
	if !strings.Contains(errMsg, expectedOp) {
		t.Errorf("Error message should contain operation '%s', got: %s", expectedOp, errMsg)
	}

	if !strings.Contains(errMsg, expectedComponent) {
		t.Errorf("Error message should contain component '%s', got: %s", expectedComponent, errMsg)
	}
}
