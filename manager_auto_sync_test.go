package sync

import (
	"context"
	"testing"
	"time"
)

func TestAutoSyncRaceCondition(t *testing.T) {
	// Create a sync manager with a short sync interval
	sm := &syncManager{
		store:     &mockEventStore{},
		transport: &mockTransport{},
		options: SyncOptions{
			SyncInterval: 50 * time.Millisecond,
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

