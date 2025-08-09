package version

import (
	"context"
	"fmt"
	syncLib "sync"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// VersionManager defines the interface for managing version state.
// This allows different versioning strategies to be plugged in.
type VersionManager interface {
	// CurrentVersion returns the current version state
	CurrentVersion() synckit.Version

	// NextVersion generates the next version for a new event
	// The nodeID parameter allows node-specific versioning (e.g., for vector clocks)
	NextVersion(nodeID string) synckit.Version

	// UpdateFromVersion updates the internal state based on an observed version
	// This is used when loading events from the store or receiving from peers
	UpdateFromVersion(version synckit.Version) error

	// Clone creates a copy of the version manager
	Clone() VersionManager
}

// VectorClockManager implements VersionManager for vector clock versioning.
type VectorClockManager struct {
	clock *VectorClock
	mu    syncLib.RWMutex
}

// NewVectorClockManager creates a new vector clock version manager.
func NewVectorClockManager() *VectorClockManager {
	return &VectorClockManager{
		clock: NewVectorClock(),
	}
}

// NewVectorClockManagerFromVersion creates a vector clock manager from an existing version.
func NewVectorClockManagerFromVersion(version synckit.Version) (*VectorClockManager, error) {
	if version == nil || version.IsZero() {
		return NewVectorClockManager(), nil
	}

	vc, ok := version.(*VectorClock)
	if !ok {
		return nil, fmt.Errorf("version is not a VectorClock: %T", version)
	}

	return &VectorClockManager{
		clock: vc.Clone(),
	}, nil
}

// CurrentVersion returns the current vector clock state.
func (vm *VectorClockManager) CurrentVersion() synckit.Version {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.clock.Clone()
}

// NextVersion increments the clock for the given node and returns the new version.
func (vm *VectorClockManager) NextVersion(nodeID string) synckit.Version {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	err := vm.clock.Increment(nodeID)
	if err != nil {
		return nil
	}
	return vm.clock.Clone()
}

// UpdateFromVersion merges the observed version into the current state.
func (vm *VectorClockManager) UpdateFromVersion(version synckit.Version) error {
	if version == nil || version.IsZero() {
		return nil
	}

	vc, ok := version.(*VectorClock)
	if !ok {
		return fmt.Errorf("version is not a VectorClock: %T", version)
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.clock.Merge(vc)
}

// Clone creates a copy of the version manager.
func (vm *VectorClockManager) Clone() VersionManager {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return &VectorClockManager{
		clock: vm.clock.Clone(),
	}
}

// Error types
var (
	ErrIncompatibleVersion = fmt.Errorf("incompatible version type")
	ErrStoreClosed = fmt.Errorf("store is closed")
)

// VersionedStore is a decorator for an EventStore that manages versioning automatically.
// It uses a pluggable VersionManager to handle different versioning strategies.
type VersionedStore struct {
	store          synckit.EventStore
	nodeID         string
	versionManager VersionManager
	mu            syncLib.RWMutex
	closed        bool
}

// cursorToVectorClock converts a cursor sequence to a vector clock
func cursorToVectorClock(c cursor.IntegerCursor) *VectorClock {
	vc := NewVectorClock()
	vc.clocks["sequence"] = c.Seq
	return vc
}

// vectorClockToCursor converts a vector clock to a cursor sequence
// Uses the special "sequence" key for tracking the global sequence
func vectorClockToCursor(vc *VectorClock) cursor.IntegerCursor {
	if vc == nil {
		return cursor.IntegerCursor{Seq: 0}
	}
	if seq, ok := vc.clocks["sequence"]; ok {
		return cursor.IntegerCursor{Seq: seq}
	}
	return cursor.IntegerCursor{Seq: 0}
}

// NewVersionedStore creates a new versioned store decorator.
// It automatically initializes the version manager from the store's latest version.
func NewVersionedStore(store synckit.EventStore, nodeID string, versionManager VersionManager) (*VersionedStore, error) {
	if versionManager == nil {
		return nil, fmt.Errorf("version manager cannot be nil")
	}

	// Create versioned store first
	vs := &VersionedStore{
		store:          store,
		nodeID:         nodeID,
		versionManager: versionManager,
	}

	// Now we use our store's LatestVersion method to get the latest version
	latestVersion, err := vs.LatestVersion(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	// Convert cursor version to vector clock and initialize version manager
	if cursorVersion, ok := latestVersion.(cursor.IntegerCursor); ok {
		vcVersion := cursorToVectorClock(cursorVersion)
		if err := versionManager.UpdateFromVersion(vcVersion); err != nil {
			return nil, fmt.Errorf("failed to initialize version manager: %w", err)
		}
	}

	return &VersionedStore{
		store:          store,
		nodeID:         nodeID,
		versionManager: versionManager,
	}, nil
}

// Store generates the next version and stores the event with that version.
func (s *VersionedStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	// Get the next sequence number from store
	latestVersion, err := s.store.LatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	cursorVersion, ok := latestVersion.(cursor.IntegerCursor)
	if !ok {
		return fmt.Errorf("incompatible version type: %T", latestVersion)
	}

	// Increment cursor sequence
	cursorVersion.Seq++

	// Handle vector clock versioning
	if ve, ok := event.(interface{ Version() *VectorClock; SetVersion(*VectorClock) }); ok {
		// Initialize or update vector clock
		vc := ve.Version()
		if vc == nil {
			vc = NewVectorClock()
		}

		// Set sequence number and increment node clock
		vc.clocks["sequence"] = cursorVersion.Seq
		vc.Increment(s.nodeID)
		ve.SetVersion(vc)
	}

	// Store the event with cursor version
	return s.store.Store(ctx, event, cursorVersion)
}

// Load passes through to the underlying store and updates version manager state.
func (s *VersionedStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Never pass a nil version to the underlying store; use zero cursor instead
	if since == nil {
		since = cursor.IntegerCursor{Seq: 0}
	}
	Events, err := s.store.Load(ctx, since)
	if err != nil {
		return nil, err
	}

	// Alias to a local variable named events for the rest of the method
	events := Events

	// Update event versions with both cursor and vector clock
	for _, ev := range events {
		// Get cursor version
		cursorVersion, ok := ev.Version.(cursor.IntegerCursor)
		if !ok {
			fmt.Printf("Warning: event has non-cursor version: %T\n", ev.Version)
			continue
		}

		// Convert cursor to vector clock
		vcVersion := cursorToVectorClock(cursorVersion)

		// Set vector clock on event if it supports it
		if ve, ok := ev.Event.(interface{ SetVersion(*VectorClock) }); ok {
			ve.SetVersion(vcVersion)
		}

		// Update version manager
		if err := s.versionManager.UpdateFromVersion(vcVersion); err != nil {
			fmt.Printf("Warning: failed to update version manager: %v\n", err)
		}
	}

	return events, nil
}

// LoadByAggregate passes through to the underlying store and updates version manager state.
func (s *VersionedStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Never pass a nil version to the underlying store; use zero cursor instead
	if since == nil {
		since = cursor.IntegerCursor{Seq: 0}
	}
	events, err := s.store.LoadByAggregate(ctx, aggregateID, since)
	if err != nil {
		return nil, err
	}

	// Update event versions with both cursor and vector clock
	for _, ev := range events {
		// Get cursor version
		cursorVersion, ok := ev.Version.(cursor.IntegerCursor)
		if !ok {
			fmt.Printf("Warning: event has non-cursor version: %T\n", ev.Version)
			continue
		}

		// Convert cursor to vector clock
		vcVersion := cursorToVectorClock(cursorVersion)

		// Set vector clock on event if it supports it
		if ve, ok := ev.Event.(interface{ SetVersion(*VectorClock) }); ok {
			ve.SetVersion(vcVersion)
		}

		// Update version manager
		if err := s.versionManager.UpdateFromVersion(vcVersion); err != nil {
			fmt.Printf("Warning: failed to update version manager: %v\n", err)
		}
	}

	return events, nil
}

// LatestVersion returns the highest version number in the store.
func (s *VersionedStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	latestVersion, err := s.store.LatestVersion(ctx)
	if err != nil {
		return nil, err
	}

	cursorVersion, ok := latestVersion.(cursor.IntegerCursor)
	if !ok {
		return nil, ErrIncompatibleVersion
	}

	return cursorVersion, nil
}

// ParseVersion delegates to the underlying store.
func (s *VersionedStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	return s.store.ParseVersion(ctx, versionStr)
}

// Close delegates to the underlying store.
func (s *VersionedStore) Close() error {
	return s.store.Close()
}

// GetVersionManager returns the current version manager (useful for testing or advanced use cases).
func (s *VersionedStore) GetVersionManager() VersionManager {
	return s.versionManager
}

// SetNodeID updates the node ID for version generation.
func (s *VersionedStore) SetNodeID(nodeID string) {
	s.nodeID = nodeID
}
