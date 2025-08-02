package sync

import (
	"context"
)
// Mock types for testing

// Mock version implementation for testing
type mockVersion struct{}

func (v *mockVersion) Compare(other Version) int { return 0 }
func (v *mockVersion) String() string           { return "0" }
func (v *mockVersion) IsZero() bool             { return true }

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

func (r *mockConflictResolver) Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error) {
	return nil, nil
}
