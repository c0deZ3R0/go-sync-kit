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
			if err.Error() != "auto sync is not running" &&
				err.Error() != "auto sync is already running" &&
				err.Error() != "sync manager is closed" {
				t.Errorf("Unexpected error type: %v", err)
			}
		}
	}
}

// Mock implementations for testing

// Mock version implementation for testing
type mockVersion struct{}

func (v *mockVersion) Compare(other Version) int { return 0 }
func (v *mockVersion) String() string           { return "0" }
func (v *mockVersion) IsZero() bool             { return true }

type mockEventStore struct{}

func (m *mockEventStore) Store(ctx context.Context, event Event, version Version) error { 
	return nil 
}

func (m *mockEventStore) Load(ctx context.Context, since Version) ([]EventWithVersion, error) {
	return nil, nil
}

func (m *mockEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error) {
	return nil, nil
}

func (m *mockEventStore) LatestVersion(ctx context.Context) (Version, error) { 
	return &mockVersion{}, nil 
}

func (m *mockEventStore) ParseVersion(ctx context.Context, versionStr string) (Version, error) {
	return &mockVersion{}, nil
}

func (m *mockEventStore) Close() error { return nil }

type mockTransport struct{}

func (m *mockTransport) Push(ctx context.Context, events []EventWithVersion) error { 
	return nil 
}

func (m *mockTransport) Pull(ctx context.Context, since Version) ([]EventWithVersion, error) {
	return nil, nil
}

func (m *mockTransport) GetLatestVersion(ctx context.Context) (Version, error) { 
	return &mockVersion{}, nil 
}

func (m *mockTransport) Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error {
	return nil
}

func (m *mockTransport) Close() error { return nil }
