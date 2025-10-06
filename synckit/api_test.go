package synckit_test

import (
	"context"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// TestImportSurface is a compile-only test verifying that applications can import
// synckit and access all core types via type aliases without needing to import
// synckit/types or other subpackages explicitly.
func TestImportSurface(t *testing.T) {
	// Verify Event type is accessible
	var _ synckit.Event

	// Verify Version type is accessible
	var _ synckit.Version

	// Verify EventWithVersion type is accessible
	var _ synckit.EventWithVersion

	// Verify Conflict types are accessible
	var _ synckit.Conflict
	var _ synckit.ResolvedConflict

	// Verify ConflictResolver interface is accessible
	var _ synckit.ConflictResolver

	// Verify EventStore interface is accessible
	var _ synckit.EventStore

	// Verify Transport interface is accessible
	var _ synckit.Transport

	// Verify CursorTransport interface is accessible
	var _ synckit.CursorTransport
}

// mockEvent is a minimal Event implementation for testing
type mockEvent struct {
	id          string
	eventType   string
	aggregateID string
}

func (m *mockEvent) ID() string                       { return m.id }
func (m *mockEvent) Type() string                     { return m.eventType }
func (m *mockEvent) AggregateID() string              { return m.aggregateID }
func (m *mockEvent) Data() interface{}                { return nil }
func (m *mockEvent) Metadata() map[string]interface{} { return nil }

// mockVersion is a minimal Version implementation for testing
type mockVersion struct{ v int }

func (m *mockVersion) Compare(other synckit.Version) int { return 0 }
func (m *mockVersion) String() string                    { return "v1" }
func (m *mockVersion) IsZero() bool                      { return m.v == 0 }

// mockResolver is a minimal ConflictResolver for testing
type mockResolver struct{}

func (r *mockResolver) Resolve(ctx context.Context, c synckit.Conflict) (synckit.ResolvedConflict, error) {
	return synckit.ResolvedConflict{
		ResolvedEvents: []synckit.EventWithVersion{c.Local},
		Decision:       "local-wins",
	}, nil
}

// TestTypeCompatibility verifies that aliased types are compatible with
// implementations and can be used in function signatures.
func TestTypeCompatibility(t *testing.T) {
	evt := &mockEvent{id: "evt1", eventType: "test", aggregateID: "agg1"}
	ver := &mockVersion{v: 1}
	resolver := &mockResolver{}

	// These should compile without issue
	var _ synckit.Event = evt
	var _ synckit.Version = ver
	var _ synckit.ConflictResolver = resolver

	evtWithVer := synckit.EventWithVersion{
		Event:   evt,
		Version: ver,
	}

	conflict := synckit.Conflict{
		EventType:   "test",
		AggregateID: "agg1",
		Local:       evtWithVer,
		Remote:      evtWithVer,
	}

	// Should compile: resolver.Resolve accepts synckit.Conflict
	_, _ = resolver.Resolve(context.Background(), conflict)
}
