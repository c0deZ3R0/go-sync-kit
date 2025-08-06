package sync

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// contextAwareEventStore wraps mockEventStore to make it context-aware
type contextAwareEventStore struct {
	*mockEventStore
	events []EventWithVersion
}

func (s *contextAwareEventStore) Load(ctx context.Context, version Version) ([]EventWithVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.events, nil
}

// contextAwareTransport wraps mockTransport to make it context-aware
type contextAwareTransport struct {
	*mockTransport
}

func (t *contextAwareTransport) Push(ctx context.Context, events []EventWithVersion) error {
	time.Sleep(5 * time.Millisecond) // Simulate work
	return ctx.Err()                 // Return context error if cancelled
}

// mockMetricsCollector implements MetricsCollector interface for testing
type mockMetricsCollector struct{}

func (m *mockMetricsCollector) RecordSyncDuration(operation string, duration time.Duration) {}
func (m *mockMetricsCollector) RecordSyncEvents(pushed, pulled int)                         {}
func (m *mockMetricsCollector) RecordConflicts(resolved int)                                {}
func (m *mockMetricsCollector) RecordSyncErrors(operation, reason string)                   {}

// mockEvent implements Event interface for testing
type mockEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (m *mockEvent) ID() string                       { return m.id }
func (m *mockEvent) Type() string                     { return m.eventType }
func (m *mockEvent) AggregateID() string              { return m.aggregateID }
func (m *mockEvent) Data() interface{}                { return m.data }
func (m *mockEvent) Metadata() map[string]interface{} { return m.metadata }

// mockIntegerVersion implements Version interface for testing
type mockIntegerVersion int64

func (v mockIntegerVersion) Compare(other Version) int {
	ov, ok := other.(mockIntegerVersion)
	if !ok {
		return -1
	}
	if v < ov {
		return -1
	}
	if v > ov {
		return 1
	}
	return 0
}

func (v mockIntegerVersion) String() string { return fmt.Sprintf("%d", v) }
func (v mockIntegerVersion) IsZero() bool   { return v == 0 }

func TestBatchProcessContextCancellation(t *testing.T) {
	// Create many events to process
	localEvents := make([]EventWithVersion, 1000)
	for i := range localEvents {
		localEvents[i] = EventWithVersion{
			Event:   &mockEvent{id: fmt.Sprintf("event-%d", i)},
			Version: mockIntegerVersion(i),
		}
	}

	store := &contextAwareEventStore{
		mockEventStore: &mockEventStore{},
		events:         localEvents,
	}

	transport := &contextAwareTransport{
		mockTransport: &mockTransport{},
	}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			BatchSize:        1, // Small batch size to trigger multiple iterations
			MetricsCollector: &mockMetricsCollector{},
		},
	}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Try to push events
	_, err := sm.push(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	// Use errors.Is to check wrapped error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected wrapped context.Canceled error, got: %v", err)
	}
}

func TestStartAutoSyncContextCancellation(t *testing.T) {
	sm := &syncManager{
		store:     &mockEventStore{},
		transport: &mockTransport{},
		options: SyncOptions{
			SyncInterval:     50 * time.Millisecond,
			MetricsCollector: &mockMetricsCollector{},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel the context

	err := sm.StartAutoSync(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

func TestAutoSyncRaceCondition(t *testing.T) {
	// Create a sync manager with a short sync interval
	sm := &syncManager{
		store:     &mockEventStore{},
		transport: &mockTransport{},
		options: SyncOptions{
			SyncInterval:     50 * time.Millisecond,
			MetricsCollector: &mockMetricsCollector{},
		},
	}

	// Test starting auto-sync multiple times (should fail after first)
	ctx := context.Background()
	if err := sm.StartAutoSync(ctx); err != nil {
		t.Fatalf("First StartAutoSync failed: %v", err)
	}

	// Try starting again - should fail
	if err := sm.StartAutoSync(ctx); err == nil {
		t.Error("Second StartAutoSync should have failed but didn't")
	}

	// Test rapid start/stop cycles to stress test race condition fix
	for i := 0; i < 100; i++ {
		// Stop auto-sync
		if err := sm.StopAutoSync(); err != nil {
			t.Fatalf("StopAutoSync failed on iteration %d: %v", i, err)
		}

		// Immediately start again
		if err := sm.StartAutoSync(ctx); err != nil {
			t.Fatalf("StartAutoSync failed on iteration %d: %v", i, err)
		}
	}

	// Test parallel operations
	errchan := make(chan error, 3)

	// Goroutine trying to stop
	go func() {
		if err := sm.StopAutoSync(); err != nil {
			errchan <- err
			return
		}
		errchan <- nil
	}()

	// Goroutine trying to start
	go func() {
		if err := sm.StartAutoSync(ctx); err != nil {
			errchan <- err
			return
		}
		errchan <- nil
	}()

	// Goroutine trying to close
	go func() {
		if err := sm.Close(); err != nil {
			errchan <- err
			return
		}
		errchan <- nil
	}()

	// Wait for all operations
	for i := 0; i < 3; i++ {
		if err := <-errchan; err != nil {
			// We expect some errors here since operations are racing,
			// but they should be our expected error types
			if err.Error() != "sync operation failed: auto sync is not running" &&
				err.Error() != "sync operation failed: auto sync is already running" &&
				err.Error() != "sync operation failed: sync manager is closed" {
				t.Errorf("Unexpected error type: %v", err)
			}
		}
	}
}
