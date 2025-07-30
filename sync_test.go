package sync

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockEvent is a simple implementation of the Event interface
type MockEvent struct {
	id          string
	typeName    string
	aggregateID string
	data       interface{}
	metadata   map[string]interface{}
}

func (m *MockEvent) ID() string                               { return m.id }
func (m *MockEvent) Type() string                             { return m.typeName }
func (m *MockEvent) AggregateID() string                      { return m.aggregateID }
func (m *MockEvent) Data() interface{}                        { return m.data }
func (m *MockEvent) Metadata() map[string]interface{}         { return m.metadata }

// MockVersion is a simple implementation of the Version interface
type MockVersion struct {
	timestamp time.Time
}

func (m *MockVersion) Compare(other Version) int {
	if other == nil {
		return 1 // This version is after nil
	}
	otherMock, ok := other.(*MockVersion)
	if !ok {
		return 0 // Unknown type, consider equal
	}
	switch {
	case m.timestamp.Before(otherMock.timestamp):
		return -1
	case m.timestamp.After(otherMock.timestamp):
		return 1
	default:
		return 0
	}
}

func (m *MockVersion) String() string { return m.timestamp.String() }
func (m *MockVersion) IsZero() bool   { return m.timestamp.IsZero() }

// MockEventStore is a simple implementation of the EventStore interface
// using an in-memory slice and synchronization for concurrency.
type MockEventStore struct {
	mu      sync.RWMutex
	events  []EventWithVersion
	latest  *MockVersion
}

func (m *MockEventStore) Store(ctx context.Context, event Event, version Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, EventWithVersion{Event: event, Version: version})
	mv := version.(*MockVersion)
	m.latest = mv
	return nil
}

func (m *MockEventStore) Load(ctx context.Context, since Version) ([]EventWithVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if since == nil || since.IsZero() {
		return m.events, nil
	}

	var result []EventWithVersion
	for _, ev := range m.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}

	return result, nil
}

func (m *MockEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []EventWithVersion
	for _, ev := range m.events {
		if ev.Event.AggregateID() == aggregateID && ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}

	return result, nil
}

func (m *MockEventStore) LatestVersion(ctx context.Context) (Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.latest == nil {
		return &MockVersion{timestamp: time.Time{}}, nil
	}
	return m.latest, nil
}

func (m *MockEventStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = nil
	m.latest = nil
	return nil
}

// MockTransport is a simple implementation of the Transport interface
// for testing purposes

type MockTransport struct {
	events []EventWithVersion
	mu     sync.Mutex
}

func (m *MockTransport) Push(ctx context.Context, events []EventWithVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, events...)
	return nil
}

func (m *MockTransport) Pull(ctx context.Context, since Version) ([]EventWithVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.events, nil
}

func (m *MockTransport) Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error {
	return nil
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = nil
	return nil
}

func TestSyncManager_Sync(t *testing.T) {
	store := &MockEventStore{}
	transport := &MockTransport{}
	resolver := &MockConflictResolver{}

	sm := NewSyncManager(store, transport, &SyncOptions{
		ConflictResolver: resolver,
	})

	ctx := context.Background()

	// Create mock events
	event1 := &MockEvent{
		id:          "1",
		typeName:    "TestEvent",
		aggregateID: "agg1",
		data:       "event data",
		metadata:   map[string]interface{}{},
	}

  version1 := &MockVersion{timestamp: time.Now()}

  store.Store(ctx, event1, version1)

    // Perform sync
	result, err := sm.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Assert expectations
	if result.EventsPushed != 1 {
		t.Errorf("Expected 1 event pushed, got %d", result.EventsPushed)
	}

	if result.EventsPulled != 0 {
		t.Errorf("Expected 0 events pulled, got %d", result.EventsPulled)
	}
}

// MockConflictResolver is a simple implementation of the ConflictResolver interface

type MockConflictResolver struct{}

func (m *MockConflictResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
	return remote, nil // Use remote for simplicity
}

