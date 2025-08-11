package synckit

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/synckit/dynres"
	"github.com/c0deZ3R0/go-sync-kit/interfaces"
)

// TestPhase3Integration tests the Phase 3 integration of DynamicResolver
// with the sync engine, validating conflict detection and resolution.
func TestPhase3Integration(t *testing.T) {
	// Create a test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock store and transport
	store := &MockEventStore{}
	transport := &MockTransport{}

	// Test 1: Default DynamicResolver with LWW fallback
	t.Run("DefaultDynamicResolver", func(t *testing.T) {
		opts := &SyncOptions{
			BatchSize: 10,
		}

		// Create sync manager - should auto-create DynamicResolver
		sm := NewSyncManager(store, transport, opts, logger)

		// Verify that a ConflictResolver was set
		syncMgr := sm.(*syncManager)
		if syncMgr.options.ConflictResolver == nil {
			t.Fatal("Expected default ConflictResolver to be set, but it was nil")
		}

		// Verify it's a DynamicResolver
		if _, ok := syncMgr.options.ConflictResolver.(*DynamicResolver); !ok {
			t.Errorf("Expected default resolver to be *DynamicResolver, got %T", syncMgr.options.ConflictResolver)
		}
	})

	// Test 2: Custom DynamicResolver with rules
	t.Run("CustomDynamicResolverWithRules", func(t *testing.T) {
		// Create a custom DynamicResolver with rules
		resolver, err := NewDynamicResolver(
			WithEventTypeRule("user_rule", "UserUpdated", &dynres.LastWriteWinsResolver{}),
			WithEventTypeRule("order_rule", "OrderCreated", &dynres.AdditiveMergeResolver{}),
			WithFallback(&dynres.ManualReviewResolver{}),
			WithLogger(logger),
		)
		if err != nil {
			t.Fatalf("Failed to create DynamicResolver: %v", err)
		}

		opts := &SyncOptions{
			BatchSize:        10,
			ConflictResolver: resolver,
		}

		// Create sync manager with custom resolver
		sm := NewSyncManager(store, transport, opts, logger)
		syncMgr := sm.(*syncManager)

		// Verify the resolver is set correctly
		if syncMgr.options.ConflictResolver != resolver {
			t.Errorf("Expected custom resolver to be preserved")
		}
	})

	// Test 3: Conflict Detection
	t.Run("ConflictDetection", func(t *testing.T) {
		sm := &syncManager{
			logger: logger,
		}

		// Create test events for conflict detection
		localEvents := []EventWithVersion{
			{Event: &Phase3TestEvent{id: "1", eventType: "UserUpdated", aggregateID: "user-123", data: "local-data"}, Version: &Phase3TestVersion{version: "v1"}},
		}
		remoteEvents := []EventWithVersion{
			{Event: &Phase3TestEvent{id: "2", eventType: "UserUpdated", aggregateID: "user-123", data: "remote-data"}, Version: &Phase3TestVersion{version: "v2"}},
		}

		// Detect conflicts
		conflicts := sm.detectConflicts(localEvents, remoteEvents)

		// Verify conflict was detected
		if len(conflicts) != 1 {
			t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
		}

		conflict := conflicts[0]
		if conflict.EventType != "UserUpdated" {
			t.Errorf("Expected EventType 'UserUpdated', got %s", conflict.EventType)
		}
		if conflict.AggregateID != "user-123" {
			t.Errorf("Expected AggregateID 'user-123', got %s", conflict.AggregateID)
		}

		// Verify metadata was populated
		if conflict.Metadata["local_version"] != "v1" {
			t.Errorf("Expected local_version 'v1', got %v", conflict.Metadata["local_version"])
		}
		if conflict.Metadata["remote_version"] != "v2" {
			t.Errorf("Expected remote_version 'v2', got %v", conflict.Metadata["remote_version"])
		}
	})

	// Test 4: No Conflicts When Events Don't Match
	t.Run("NoConflictsForDifferentAggregates", func(t *testing.T) {
		sm := &syncManager{
			logger: logger,
		}

		// Create test events for different aggregates (should not conflict)
		localEvents := []EventWithVersion{
			{Event: &Phase3TestEvent{id: "1", eventType: "UserUpdated", aggregateID: "user-123", data: "local-data"}, Version: &Phase3TestVersion{version: "v1"}},
		}
		remoteEvents := []EventWithVersion{
			{Event: &Phase3TestEvent{id: "2", eventType: "UserUpdated", aggregateID: "user-456", data: "remote-data"}, Version: &Phase3TestVersion{version: "v2"}},
		}

		// Detect conflicts
		conflicts := sm.detectConflicts(localEvents, remoteEvents)

		// Verify no conflicts
		if len(conflicts) != 0 {
			t.Errorf("Expected 0 conflicts for different aggregates, got %d", len(conflicts))
		}
	})
}

// Phase3TestEvent implements the Event interface for testing
type Phase3TestEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (e *Phase3TestEvent) ID() string                         { return e.id }
func (e *Phase3TestEvent) Type() string                       { return e.eventType }
func (e *Phase3TestEvent) AggregateID() string                { return e.aggregateID }
func (e *Phase3TestEvent) Data() interface{}                  { return e.data }
func (e *Phase3TestEvent) Metadata() map[string]interface{}   { return e.metadata }

// Phase3TestVersion implements the Version interface for testing
type Phase3TestVersion struct {
	version string
}

func (v *Phase3TestVersion) String() string                    { return v.version }
func (v *Phase3TestVersion) Compare(other interfaces.Version) int {
	if other == nil {
		return 1
	}
	otherTest, ok := other.(*Phase3TestVersion)
	if !ok {
		return 0
	}
	if v.version == otherTest.version {
		return 0
	}
	if v.version < otherTest.version {
		return -1
	}
	return 1
}

func (v *Phase3TestVersion) IsZero() bool {
	return v.version == "" || v.version == "0"
}

// MockEventStore for testing
type MockEventStore struct{}

func (m *MockEventStore) Store(ctx context.Context, event Event, version Version) error {
	return nil
}

func (m *MockEventStore) Load(ctx context.Context, since Version) ([]EventWithVersion, error) {
	return []EventWithVersion{}, nil
}

func (m *MockEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error) {
	return []EventWithVersion{}, nil
}

func (m *MockEventStore) LatestVersion(ctx context.Context) (Version, error) {
	return &Phase3TestVersion{version: "v1"}, nil
}

func (m *MockEventStore) ParseVersion(ctx context.Context, versionStr string) (Version, error) {
	return &Phase3TestVersion{version: versionStr}, nil
}

func (m *MockEventStore) Close() error {
	return nil
}

// MockTransport for testing
type MockTransport struct{}

func (m *MockTransport) Push(ctx context.Context, events []EventWithVersion) error {
	return nil
}

func (m *MockTransport) Pull(ctx context.Context, since Version) ([]EventWithVersion, error) {
	return []EventWithVersion{}, nil
}

func (m *MockTransport) GetLatestVersion(ctx context.Context) (Version, error) {
	return &Phase3TestVersion{version: "v1"}, nil
}

func (m *MockTransport) Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error {
	return nil
}

func (m *MockTransport) Close() error {
	return nil
}
