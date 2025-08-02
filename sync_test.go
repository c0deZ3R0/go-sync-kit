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

func (m *MockEventStore) ParseVersion(ctx context.Context, versionStr string) (Version, error) {
	// For testing, we parse the string as a time.Time
	t, err := time.Parse(time.RFC3339, versionStr)
	if err != nil {
		// If it fails, use the current time as a fallback
		t = time.Now()
	}
	return &MockVersion{timestamp: t}, nil
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

func (m *MockTransport) Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error {
	return nil
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = nil
	return nil
}

func (m *MockTransport) GetLatestVersion(ctx context.Context) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.events) == 0 {
		return &MockVersion{timestamp: time.Time{}}, nil
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

func TestSyncManager_Push_GetLatestVersion(t *testing.T) {
	// Create test dependencies
	store := &MockEventStore{}
	transport := &MockTransport{}

	// Create sync manager
	sm := NewSyncManager(store, transport, &SyncOptions{})

	ctx := context.Background()

	// Add some events to transport with different versions
	transportEvents := []EventWithVersion{
		{
			Event: &MockEvent{id: "1", typeName: "TestEvent"},
			Version: &MockVersion{timestamp: time.Now().Add(-2 * time.Hour)},
		},
		{
			Event: &MockEvent{id: "2", typeName: "TestEvent"},
			Version: &MockVersion{timestamp: time.Now().Add(-1 * time.Hour)},
		},
	}
	transport.events = transportEvents

	// Add a local event that's newer than transport events
	localEvent := &MockEvent{id: "3", typeName: "TestEvent"}
	localVersion := &MockVersion{timestamp: time.Now()}
	store.Store(ctx, localEvent, localVersion)

	// Perform push
	result, err := sm.Push(ctx)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify that the event was pushed
	if result.EventsPushed != 1 {
		t.Errorf("Expected 1 event pushed, got %d", result.EventsPushed)
	}

	// Verify that the new event is in transport
	if len(transport.events) != 3 { // 2 original + 1 pushed
		t.Errorf("Expected 3 events in transport, got %d", len(transport.events))
	}
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

func TestMockTransport_Pull_SinceParameter(t *testing.T) {
	// Create a new mock transport
	transport := &MockTransport{}
	ctx := context.Background()

	// Create events with different timestamps
	time1 := time.Now().Add(-2 * time.Hour)
	time2 := time.Now().Add(-1 * time.Hour)
	time3 := time.Now()

	// Add events to transport
	transport.events = []EventWithVersion{
		{
			Event:   &MockEvent{id: "1", typeName: "TestEvent"},
			Version: &MockVersion{timestamp: time1},
		},
		{
			Event:   &MockEvent{id: "2", typeName: "TestEvent"},
			Version: &MockVersion{timestamp: time2},
		},
		{
			Event:   &MockEvent{id: "3", typeName: "TestEvent"},
			Version: &MockVersion{timestamp: time3},
		},
	}

	// Test cases
	tests := []struct {
		name          string
		since         Version
		expectedCount int
	}{
		{
			name:          "nil version returns all events",
			since:         nil,
			expectedCount: 3,
		},
		{
			name:          "zero version returns all events",
			since:         &MockVersion{timestamp: time.Time{}},
			expectedCount: 3,
		},
		{
			name:          "since first event returns two events",
			since:         &MockVersion{timestamp: time1},
			expectedCount: 2,
		},
		{
			name:          "since second event returns one event",
			since:         &MockVersion{timestamp: time2},
			expectedCount: 1,
		},
		{
			name:          "since last event returns no events",
			since:         &MockVersion{timestamp: time3},
			expectedCount: 0,
		},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events, err := transport.Pull(ctx, tc.since)
			if err != nil {
				t.Fatalf("Pull failed: %v", err)
			}
			if len(events) != tc.expectedCount {
				t.Errorf("Expected %d events, got %d", tc.expectedCount, len(events))
			}
		})
	}
}
