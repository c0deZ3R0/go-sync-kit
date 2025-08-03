package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// slowOperationStore extends mockEventStore with delayed operations
type slowOperationStore struct {
	*mockEventStore
	delay time.Duration
}

func (s *slowOperationStore) Load(ctx context.Context, version Version) ([]EventWithVersion, error) {
	select {
	case <-time.After(s.delay):
		events := []EventWithVersion{
			{Event: &mockEvent{id: "local-1"}, Version: mockIntegerVersion(1)},
		}
		return events, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// slowOperationTransport extends mockTransport with delayed operations
type slowOperationTransport struct {
	*mockTransport
	delay time.Duration
}

func (t *slowOperationTransport) Pull(ctx context.Context, version Version) ([]EventWithVersion, error) {
	select {
	case <-time.After(t.delay):
		events := []EventWithVersion{
			{Event: &mockEvent{id: "remote-1"}, Version: mockIntegerVersion(2)},
		}
		return events, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type metricsRecorder struct {
	contextCanceledCount atomic.Int64
	timeoutCount        atomic.Int64
	lastOperation       string
	lastReason         string
}

func (m *metricsRecorder) RecordSyncDuration(operation string, duration time.Duration) {}
func (m *metricsRecorder) RecordSyncEvents(pushed, pulled int) {}
func (m *metricsRecorder) RecordConflicts(resolved int) {}
func (m *metricsRecorder) RecordSyncErrors(operation, reason string) {
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
	store := &slowOperationStore{
		mockEventStore: &mockEventStore{},
		delay:         2 * time.Second,
	}
	transport := &slowOperationTransport{
		mockTransport: &mockTransport{},
		delay:         2 * time.Second,
	}
	metrics := &metricsRecorder{}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			MetricsCollector: metrics,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

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
	store := &slowOperationStore{
		mockEventStore: &mockEventStore{},
		delay:         2 * time.Second,
	}
	transport := &slowOperationTransport{
		mockTransport: &mockTransport{},
		delay:         2 * time.Second,
	}
	metrics := &metricsRecorder{}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			MetricsCollector: metrics,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

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

type delayedResolver struct {
	delay time.Duration
}

func (r *delayedResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
	select {
	case <-time.After(r.delay):
		return remote, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestConflictResolutionContextCancellation(t *testing.T) {
	store := &slowOperationStore{
		mockEventStore: &mockEventStore{},
		delay:         50 * time.Millisecond,
	}
	transport := &slowOperationTransport{
		mockTransport: &mockTransport{},
		delay:         50 * time.Millisecond,
	}
	resolver := &delayedResolver{
		delay: 2 * time.Second,
	}
	metrics := &metricsRecorder{}

	sm := &syncManager{
		store:     store,
		transport: transport,
		options: SyncOptions{
			ConflictResolver:  resolver,
			MetricsCollector: metrics,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond) // Wait for pull and store to complete
		cancel()
	}()

	_, err := sm.Pull(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled error, got: %v", err)
	}

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
