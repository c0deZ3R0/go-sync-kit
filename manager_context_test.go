package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// mockEventStore is a mock implementation of EventStore
type mockEventStore struct {
	events []EventWithVersion
}

func (s *mockEventStore) Store(ctx context.Context, event Event, version Version) error {
	return nil
}

func (s *mockEventStore) Load(ctx context.Context, version Version) ([]EventWithVersion, error) {
	return s.events, nil
}

func (s *mockEventStore) LatestVersion(ctx context.Context) (Version, error) {
	if len(s.events) == 0 {
		return mockIntegerVersion(0), nil
	}
	return s.events[len(s.events)-1].Version, nil
}

func (s *mockEventStore) Close() error {
	return nil
}

// mockSlowStore simulates a slow database store
type mockSlowStore struct {
	*mockEventStore
	delay time.Duration
}

func (s *mockSlowStore) Load(ctx context.Context, version Version) ([]EventWithVersion, error) {
	select {
	case <-time.After(s.delay):
		return []EventWithVersion{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// mockTransport is a mock implementation of Transport
type mockTransport struct {
	events []EventWithVersion
}

func (t *mockTransport) Push(ctx context.Context, events []EventWithVersion) error {
	return nil
}

func (t *mockTransport) Pull(ctx context.Context, version Version) ([]EventWithVersion, error) {
	return t.events, nil
}

func (t *mockTransport) GetLatestVersion(ctx context.Context) (Version, error) {
	if len(t.events) == 0 {
		return mockIntegerVersion(0), nil
	}
	return t.events[len(t.events)-1].Version, nil
}

func (t *mockTransport) Close() error {
	return nil
}

// mockSlowTransport simulates a slow transport
type mockSlowTransport struct {
	*mockTransport
	delay time.Duration
}

func (t *mockSlowTransport) Pull(ctx context.Context, version Version) ([]EventWithVersion, error) {
	select {
	case <-time.After(t.delay):
		return []EventWithVersion{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// mockMetricsRecorder captures metrics for testing
type mockMetricsRecorder struct {
	contextCanceledCount atomic.Int64
	timeoutCount        atomic.Int64
	lastOperation       string
	lastReason         string
}

func (m *mockMetricsRecorder) RecordSyncDuration(operation string, duration time.Duration) {}
func (m *mockMetricsRecorder) RecordSyncEvents(pushed, pulled int) {}
func (m *mockMetricsRecorder) RecordConflicts(resolved int) {}
func (m *mockMetricsRecorder) RecordSyncErrors(operation, reason string) {
	m.lastOperation = operation
	m.lastReason = reason
	if reason == "context_canceled" {
		m.contextCanceledCount.Add(1)
	}
	if reason == "timeout" {
		m.timeoutCount.Add(1)
	}
}

func TestPullContextTimeout(t *testing.T) {
	// Create mock store and transport that are slow
	store := &mockSlowStore{
		mockEventStore: &mockEventStore{},
		delay:         2 * time.Second,
	}
	transport := &mockSlowTransport{
		mockTransport: &mockTransport{},
		delay:         2 * time.Second,
	}
	metrics := &mockMetricsRecorder{}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			MetricsCollector: metrics,
		},
	}

	// Use a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Attempt to pull - should timeout
	_, err := sm.Pull(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded error, got: %v", err)
	}

	if metrics.timeoutCount.Load() == 0 {
		t.Error("expected timeout metric to be recorded")
	}
	if metrics.lastOperation != "pull" {
		t.Errorf("expected operation 'pull', got: %s", metrics.lastOperation)
	}
	if metrics.lastReason != "timeout" {
		t.Errorf("expected reason 'timeout', got: %s", metrics.lastReason)
	}
}

func TestPullContextCancellation(t *testing.T) {
	// Create mock store and transport
	store := &mockSlowStore{
		mockEventStore: &mockEventStore{},
		delay:         2 * time.Second,
	}
	transport := &mockSlowTransport{
		mockTransport: &mockTransport{},
		delay:         2 * time.Second,
	}
	metrics := &mockMetricsRecorder{}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			MetricsCollector: metrics,
		},
	}

	// Create a context and cancel it shortly after starting
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Attempt to pull - should be cancelled
	_, err := sm.Pull(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled error, got: %v", err)
	}

	if metrics.contextCanceledCount.Load() == 0 {
		t.Error("expected context cancelled metric to be recorded")
	}
	if metrics.lastOperation != "pull" {
		t.Errorf("expected operation 'pull', got: %s", metrics.lastOperation)
	}
	if metrics.lastReason != "context_canceled" {
		t.Errorf("expected reason 'context_canceled', got: %s", metrics.lastReason)
	}
}

// mockSlowConflictResolver simulates a slow conflict resolution process
type mockSlowConflictResolver struct {
	delay time.Duration
}

func (r *mockSlowConflictResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
	select {
	case <-time.After(r.delay):
		return remote, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestConflictResolutionContextCancellation(t *testing.T) {
	// Create mock components
	store := &mockEventStore{
		events: []EventWithVersion{
			{Event: &mockEvent{id: "1"}, Version: mockIntegerVersion(1)},
		},
	}
	transport := &mockTransport{
		events: []EventWithVersion{
			{Event: &mockEvent{id: "2"}, Version: mockIntegerVersion(2)},
		},
	}
	resolver := &mockSlowConflictResolver{
		delay: 2 * time.Second,
	}
	metrics := &mockMetricsRecorder{}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			ConflictResolver:  resolver,
			MetricsCollector: metrics,
		},
	}

	// Create a context and cancel it shortly after starting
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Attempt to pull - should be cancelled during conflict resolution
	_, err := sm.Pull(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled error, got: %v", err)
	}

	// Verify metrics for conflict resolution cancellation
	if metrics.contextCanceledCount.Load() == 0 {
		t.Error("expected context cancelled metric to be recorded")
	}
	if metrics.lastOperation != "conflict_resolution" {
		t.Errorf("expected operation 'conflict_resolution', got: %s", metrics.lastOperation)
	}
	if metrics.lastReason != "context_canceled" {
		t.Errorf("expected reason 'context_canceled', got: %s", metrics.lastReason)
	}
}
