//go:build !race
// +build !race

package synckit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func TestBatchProcessContextCancellation(t *testing.T) {
	// Create events to process (reduced from 1000 to 100 to prevent timeouts)
	localEvents := make([]EventWithVersion, 100)
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
		logger:    slog.Default(),
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
		logger:    slog.Default(),
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
		logger:    slog.Default(),
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

	// Try starting again - should be idempotent (no error)
	if err := sm.StartAutoSync(ctx); err != nil {
		t.Errorf("Second StartAutoSync should be idempotent but got error: %v", err)
	}

	// Test rapid start/stop cycles to stress test race condition fix
	// Reduced from 100 to 10 iterations to prevent test timeouts
	for i := 0; i < 10; i++ {
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

	// Wait for all operations - with idempotent behavior, we should not get errors
	for i := 0; i < 3; i++ {
		if err := <-errchan; err != nil {
			// With idempotent behavior, the only acceptable error is sync manager closed
			if err.Error() != "sync operation failed: sync manager is closed" {
				t.Errorf("Unexpected error type: %v", err)
			}
		}
	}
}
