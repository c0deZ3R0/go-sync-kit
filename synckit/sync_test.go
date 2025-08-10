package synckit

import (
	"context"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/logging"
)

type mockEventWithDetails struct {
	TestEvent
	id          string
	typeName    string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (m *mockEventWithDetails) ID() string                       { return m.id }
func (m *mockEventWithDetails) Type() string                     { return m.typeName }
func (m *mockEventWithDetails) AggregateID() string              { return m.aggregateID }
func (m *mockEventWithDetails) Data() interface{}                { return m.data }
func (m *mockEventWithDetails) Metadata() map[string]interface{} { return m.metadata }

type mockVersionWithTimestamp struct {
	TestVersion
	timestamp time.Time
}

func (m *mockVersionWithTimestamp) String() string { return m.timestamp.String() }
func (m *mockVersionWithTimestamp) Compare(other Version) int {
	if other == nil {
		return 1 // This version is after nil
	}
	otherMock, ok := other.(*mockVersionWithTimestamp)
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

func TestSyncManager_Push_GetLatestVersion(t *testing.T) {
	// Create test dependencies
	store := &TestEventStore{}
	transport := &TestTransport{}

	// Create sync manager
	sm := NewSyncManager(store, transport, &SyncOptions{}, logging.Default().Logger)

	ctx := context.Background()

	// Add some events to transport with different versions
	transportEvents := []EventWithVersion{
		{
			Event:   &mockEventWithDetails{id: "1", typeName: "TestEvent"},
			Version: &mockVersionWithTimestamp{timestamp: time.Now().Add(-2 * time.Hour)},
		},
		{
			Event:   &mockEventWithDetails{id: "2", typeName: "TestEvent"},
			Version: &mockVersionWithTimestamp{timestamp: time.Now().Add(-1 * time.Hour)},
		},
	}
	transport.events = transportEvents

	// Add a local event that's newer than transport events
	localEvent := &mockEventWithDetails{id: "3", typeName: "TestEvent"}
	localVersion := &mockVersionWithTimestamp{timestamp: time.Now()}
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
	store := &TestEventStore{}
	transport := &TestTransport{}
	resolver := &MockConflictResolver{}

	sm := NewSyncManager(store, transport, &SyncOptions{
		ConflictResolver: resolver,
	}, logging.Default().Logger)

	ctx := context.Background()

	// Create mock events
	event1 := &mockEventWithDetails{
		id:          "1",
		typeName:    "TestEvent",
		aggregateID: "agg1",
		data:        "event data",
		metadata:    map[string]interface{}{},
	}

	version1 := &mockVersionWithTimestamp{timestamp: time.Now()}

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
	transport := &TestTransport{}
	ctx := context.Background()

	// Create events with different timestamps
	time1 := time.Now().Add(-2 * time.Hour)
	time2 := time.Now().Add(-1 * time.Hour)
	time3 := time.Now()

	// Add events to transport
	transport.events = []EventWithVersion{
		{
			Event:   &mockEventWithDetails{id: "1", typeName: "TestEvent"},
			Version: &mockVersionWithTimestamp{timestamp: time1},
		},
		{
			Event:   &mockEventWithDetails{id: "2", typeName: "TestEvent"},
			Version: &mockVersionWithTimestamp{timestamp: time2},
		},
		{
			Event:   &mockEventWithDetails{id: "3", typeName: "TestEvent"},
			Version: &mockVersionWithTimestamp{timestamp: time3},
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
			since:         &mockVersionWithTimestamp{timestamp: time.Time{}},
			expectedCount: 3,
		},
		{
			name:          "since first event returns two events",
			since:         &mockVersionWithTimestamp{timestamp: time1},
			expectedCount: 2,
		},
		{
			name:          "since second event returns one event",
			since:         &mockVersionWithTimestamp{timestamp: time2},
			expectedCount: 1,
		},
		{
			name:          "since last event returns no events",
			since:         &mockVersionWithTimestamp{timestamp: time3},
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
