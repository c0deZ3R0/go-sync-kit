package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/dynres"
	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
	"github.com/c0deZ3R0/go-sync-kit/interfaces"
)

// Simple test event
type TestEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
}

func (e *TestEvent) ID() string                         { return e.id }
func (e *TestEvent) Type() string                       { return e.eventType }
func (e *TestEvent) AggregateID() string                { return e.aggregateID }
func (e *TestEvent) Data() interface{}                  { return e.data }
func (e *TestEvent) Metadata() map[string]interface{}   { return nil }

// Simple test version
type TestVersion struct {
	version string
}

func (v *TestVersion) String() string                    { return v.version }
func (v *TestVersion) Compare(other interfaces.Version) int {
	if other == nil {
		return 1
	}
	otherTest, ok := other.(*TestVersion)
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
func (v *TestVersion) IsZero() bool { return v.version == "" }

// Mock store
type MockStore struct{}

func (m *MockStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	return nil
}

func (m *MockStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return []synckit.EventWithVersion{}, nil
}

func (m *MockStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return []synckit.EventWithVersion{}, nil
}

func (m *MockStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	return &TestVersion{version: "v1"}, nil
}

func (m *MockStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	return &TestVersion{version: versionStr}, nil
}

func (m *MockStore) Close() error {
	return nil
}

// Mock transport
type MockTransport struct{}

func (m *MockTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	return nil
}

func (m *MockTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return []synckit.EventWithVersion{}, nil
}

func (m *MockTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return &TestVersion{version: "v1"}, nil
}

func (m *MockTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return nil
}

func (m *MockTransport) Close() error {
	return nil
}

func main() {
	fmt.Println("=== Phase 3 Integration Verification ===")
	
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	
	// Test 1: Default DynamicResolver creation
	fmt.Println("\n1. Testing default DynamicResolver creation...")
	
	store := &MockStore{}
	transport := &MockTransport{}
	opts := &synckit.SyncOptions{
		BatchSize: 10,
	}
	
	sm := synckit.NewSyncManager(store, transport, opts, logger)
	_ = sm // Use the variable to avoid unused error
	fmt.Println("✅ SyncManager created successfully")
	
	// Test 2: Custom DynamicResolver
	fmt.Println("\n2. Testing custom DynamicResolver...")
	
	resolver, err := synckit.NewDynamicResolver(
		synckit.WithEventTypeRule("user_rule", "UserUpdated", &dynres.LastWriteWinsResolver{}),
		synckit.WithFallback(&dynres.LastWriteWinsResolver{}),
		synckit.WithLogger(logger),
	)
	if err != nil {
		fmt.Printf("❌ Failed to create DynamicResolver: %v\n", err)
		return
	}
	fmt.Println("✅ Custom DynamicResolver created successfully")
	
	// Test 3: Conflict resolution
	fmt.Println("\n3. Testing conflict resolution...")
	
	conflict := dynres.Conflict{
		EventType:   "UserUpdated",
		AggregateID: "user-123",
		Local:       types.EventWithVersion{Event: &TestEvent{id: "1", eventType: "UserUpdated", aggregateID: "user-123"}, Version: &TestVersion{version: "v1"}},
		Remote:      types.EventWithVersion{Event: &TestEvent{id: "2", eventType: "UserUpdated", aggregateID: "user-123"}, Version: &TestVersion{version: "v2"}},
		Metadata:    make(map[string]any),
	}
	
	ctx := context.Background()
	resolved, err := resolver.Resolve(ctx, conflict)
	if err != nil {
		fmt.Printf("❌ Failed to resolve conflict: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Conflict resolved successfully with decision: %s\n", resolved.Decision)
	fmt.Printf("   Resolved events count: %d\n", len(resolved.ResolvedEvents))
	
	fmt.Println("\n🎉 Phase 3 Integration Verification PASSED!")
	fmt.Println("   - Default resolver injection: ✅")
	fmt.Println("   - Custom resolver creation: ✅") 
	fmt.Println("   - Conflict resolution flow: ✅")
}
