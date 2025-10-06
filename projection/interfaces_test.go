package projection

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// Test implementations for testing purposes

// TestOffsetStore is a simple in-memory offset store for testing
type TestOffsetStore struct {
	offsets map[string]synckit.Version
}

func NewTestOffsetStore() *TestOffsetStore {
	return &TestOffsetStore{
		offsets: make(map[string]synckit.Version),
	}
}

func (t *TestOffsetStore) Get(ctx context.Context, name string) (synckit.Version, error) {
	offset, exists := t.offsets[name]
	if !exists {
		return nil, nil // No offset stored yet
	}
	return offset, nil
}

func (t *TestOffsetStore) Set(ctx context.Context, name string, v synckit.Version) error {
	t.offsets[name] = v
	return nil
}

// TestProjector implements a simple projector for testing
type TestProjector struct {
	name        string
	applied     []synckit.EventWithVersion
	shouldError bool
}

func NewTestProjector(name string) *TestProjector {
	return &TestProjector{
		name:    name,
		applied: make([]synckit.EventWithVersion, 0),
	}
}

func (p *TestProjector) Name() string {
	return p.name
}

func (p *TestProjector) Apply(ctx context.Context, batch []synckit.EventWithVersion) error {
	if p.shouldError {
		return &TestError{msg: "test error in Apply"}
	}

	// Append events to applied slice (idempotent behavior would check for duplicates)
	p.applied = append(p.applied, batch...)
	return nil
}

func (p *TestProjector) SetShouldError(shouldError bool) {
	p.shouldError = shouldError
}

func (p *TestProjector) AppliedEvents() []synckit.EventWithVersion {
	return p.applied
}

// TestError is a custom error for testing
type TestError struct {
	msg string
}

func (e *TestError) Error() string {
	return e.msg
}

// TestEvent implements synckit.Event for testing
type TestEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (e *TestEvent) ID() string                       { return e.id }
func (e *TestEvent) Type() string                     { return e.eventType }
func (e *TestEvent) AggregateID() string              { return e.aggregateID }
func (e *TestEvent) Data() interface{}                { return e.data }
func (e *TestEvent) Metadata() map[string]interface{} { return e.metadata }

// TestEventStore implements a simple in-memory event store for testing
type TestEventStore struct {
	events []synckit.EventWithVersion
}

func NewTestEventStore() *TestEventStore {
	return &TestEventStore{
		events: make([]synckit.EventWithVersion, 0),
	}
}

func (s *TestEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	// Auto-assign version if not provided
	if version == nil {
		version = cursor.IntegerCursor{Seq: uint64(len(s.events) + 1)}
	}

	s.events = append(s.events, synckit.EventWithVersion{
		Event:   event,
		Version: version,
	})
	return nil
}

func (s *TestEventStore) Load(ctx context.Context, since synckit.Version, filters ...types.Filter) ([]types.EventWithVersion, error) {
	if since == nil {
		return s.events, nil
	}

	var result []synckit.EventWithVersion
	for _, ev := range s.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (s *TestEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version, filters ...types.Filter) ([]types.EventWithVersion, error) {
	events, err := s.Load(ctx, since)
	if err != nil {
		return nil, err
	}

	var result []synckit.EventWithVersion
	for _, ev := range events {
		if ev.Event.AggregateID() == aggregateID {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (s *TestEventStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	if len(s.events) == 0 {
		return cursor.IntegerCursor{Seq: 0}, nil
	}
	return s.events[len(s.events)-1].Version, nil
}

func (s *TestEventStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	if versionStr == "" || versionStr == "0" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	val, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer version string '%s': %w", versionStr, err)
	}

	return cursor.IntegerCursor{Seq: uint64(val)}, nil
}

func (s *TestEventStore) Close() error {
	return nil
}

// Test functions

func TestProjectionInterfaces(t *testing.T) {
	// Test that interfaces can be implemented and used
	offsetStore := NewTestOffsetStore()
	projector := NewTestProjector("test-projector")
	eventStore := NewTestEventStore()

	// Test OffsetStore interface
	ctx := context.Background()

	// Test Get when no offset exists
	offset, err := offsetStore.Get(ctx, "test-projector")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if offset != nil {
		t.Fatalf("Expected nil offset, got: %v", offset)
	}

	// Test Set and Get
	testVersion := cursor.IntegerCursor{Seq: 42}
	err = offsetStore.Set(ctx, "test-projector", testVersion)
	if err != nil {
		t.Fatalf("Expected no error setting offset, got: %v", err)
	}

	offset, err = offsetStore.Get(ctx, "test-projector")
	if err != nil {
		t.Fatalf("Expected no error getting offset, got: %v", err)
	}
	if offset == nil {
		t.Fatalf("Expected offset, got nil")
	}
	if offset.Compare(testVersion) != 0 {
		t.Fatalf("Expected offset %v, got %v", testVersion, offset)
	}

	// Test Projector interface
	if projector.Name() != "test-projector" {
		t.Fatalf("Expected name 'test-projector', got: %s", projector.Name())
	}

	// Test Runner creation with various options
	runner := NewRunner(eventStore, offsetStore, projector)
	if runner == nil {
		t.Fatalf("Expected runner, got nil")
	}

	// Test with options
	runnerWithOptions := NewRunner(eventStore, offsetStore, projector,
		WithBatchSize(100),
		WithLogger(slog.Default()),
	)
	if runnerWithOptions == nil {
		t.Fatalf("Expected runner with options, got nil")
	}
}

func TestRunnerOptions(t *testing.T) {
	offsetStore := NewTestOffsetStore()
	projector := NewTestProjector("test-projector")
	eventStore := NewTestEventStore()

	// Test invalid batch size (should be ignored)
	runner := NewRunner(eventStore, offsetStore, projector,
		WithBatchSize(-1),  // Invalid, should be ignored
		WithBatchSize(0),   // Invalid, should be ignored
		WithBatchSize(250), // Valid
	)

	// We can't directly test the internal batchSize field, but we can verify
	// the runner was created successfully
	if runner == nil {
		t.Fatalf("Expected runner, got nil")
	}

	// Test nil logger (should be ignored)
	runnerWithNilLogger := NewRunner(eventStore, offsetStore, projector,
		WithLogger(nil), // Should be ignored
	)
	if runnerWithNilLogger == nil {
		t.Fatalf("Expected runner with nil logger option, got nil")
	}
}
