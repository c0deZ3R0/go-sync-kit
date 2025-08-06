package sync

import (
	"context"
	"sync"
	"time"

	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
)

// TestEvent implements Event interface for testing
type TestEvent struct{}

func (m *TestEvent) ID() string                       { return "test-id" }
func (m *TestEvent) Type() string                     { return "test-type" }
func (m *TestEvent) AggregateID() string              { return "test-aggregate" }
func (m *TestEvent) Data() interface{}                { return nil }
func (m *TestEvent) Metadata() map[string]interface{} { return nil }

// TestVersion implements Version interface for testing
type TestVersion struct{}

func (m *TestVersion) Compare(_ Version) int { return 0 }
func (m *TestVersion) String() string        { return "test" }
func (m *TestVersion) IsZero() bool          { return false }

// TestEventStore implements a simple in-memory event store for testing
type TestEventStore struct {
	events []EventWithVersion
}

func (m *TestEventStore) Store(_ context.Context, event Event, version Version) error {
	m.events = append(m.events, EventWithVersion{Event: event, Version: version})
	return nil
}

func (m *TestEventStore) Load(_ context.Context, _ Version) ([]EventWithVersion, error) {
	return m.events, nil
}

func (m *TestEventStore) LoadByAggregate(_ context.Context, _ string, _ Version) ([]EventWithVersion, error) {
	return nil, nil
}

func (m *TestEventStore) LatestVersion(_ context.Context) (Version, error) {
	return &TestVersion{}, nil
}

func (m *TestEventStore) ParseVersion(_ context.Context, _ string) (Version, error) {
	return &TestVersion{}, nil
}

func (m *TestEventStore) Close() error {
	return nil
}

// TestTransport implements a simple transport for testing
type TestTransport struct {
	shouldError bool
	events      []EventWithVersion
	mu          sync.Mutex
}

func (m *TestTransport) Push(ctx context.Context, events []EventWithVersion) error {
	if m.shouldError {
		return syncErrors.New(syncErrors.OpPush, nil)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func (m *TestTransport) Pull(ctx context.Context, since Version) ([]EventWithVersion, error) {
	if m.shouldError {
		return nil, syncErrors.New(syncErrors.OpPull, nil)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Return all events if since is nil or zero
	if since == nil || since.IsZero() {
		return m.events, nil
	}

	// Filter events that are newer than the since version
	var result []EventWithVersion
	for _, ev := range m.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *TestTransport) GetLatestVersion(ctx context.Context) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.events) == 0 {
		return &TestVersion{}, nil
	}

	// Find the latest version
	latest := m.events[0].Version
	for _, ev := range m.events[1:] {
		if ev.Version.Compare(latest) > 0 {
			latest = ev.Version
		}
	}
	return latest, nil
}

func (m *TestTransport) Subscribe(_ context.Context, _ func([]EventWithVersion) error) error {
	return nil
}

func (m *TestTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
	return nil
}

// MockMetricsCollector implements MetricsCollector interface for testing
type MockMetricsCollector struct {
	DurationCalls []struct {
		Operation string
		Duration  time.Duration
	}
	EventCalls []struct {
		Pushed, Pulled int
	}
	ErrorCalls []struct {
		Operation, ErrorType string
	}
	ConflictCalls []struct {
		Resolved int
	}
}

func (m *MockMetricsCollector) RecordSyncDuration(operation string, duration time.Duration) {
	m.DurationCalls = append(m.DurationCalls, struct {
		Operation string
		Duration  time.Duration
	}{operation, duration})
}

func (m *MockMetricsCollector) RecordSyncEvents(pushed, pulled int) {
	m.EventCalls = append(m.EventCalls, struct {
		Pushed, Pulled int
	}{pushed, pulled})
}

func (m *MockMetricsCollector) RecordSyncErrors(operation string, errorType string) {
	m.ErrorCalls = append(m.ErrorCalls, struct {
		Operation, ErrorType string
	}{operation, errorType})
}

func (m *MockMetricsCollector) RecordConflicts(resolved int) {
	m.ConflictCalls = append(m.ConflictCalls, struct {
		Resolved int
	}{resolved})
}
