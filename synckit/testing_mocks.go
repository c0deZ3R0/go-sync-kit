package synckit

import (
	"context"
	"fmt"
	"time"
)

// Mock types for testing

// Mock version implementation for testing
type mockVersion struct{}

func (v *mockVersion) Compare(other Version) int { return 0 }
func (v *mockVersion) String() string            { return "0" }
func (v *mockVersion) IsZero() bool              { return true }

// Mock event store implementation for testing
type mockEventStore struct {
	EventStore
}

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

// Mock transport implementation for testing
type mockTransport struct {
	Transport
}

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

// Mock conflict resolver implementation for testing
type mockConflictResolver struct {
	ConflictResolver
}

func (r *mockConflictResolver) Resolve(ctx context.Context, conflict Conflict) (ResolvedConflict, error) {
	return ResolvedConflict{}, nil
}

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

// mockMetricsCollector implements MetricsCollector interface for testing
type mockMetricsCollector struct{}

func (m *mockMetricsCollector) RecordSyncDuration(operation string, duration time.Duration) {}
func (m *mockMetricsCollector) RecordSyncEvents(pushed, pulled int)                         {}
func (m *mockMetricsCollector) RecordConflicts(resolved int)                                {}
func (m *mockMetricsCollector) RecordSyncErrors(operation, reason string)                   {}

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
