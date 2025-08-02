package version

import (
	"context"
	"fmt"
	syncLib "sync"

	sync "github.com/c0deZ3R0/go-sync-kit"
)

// VersionManager defines the interface for managing version state.
// This allows different versioning strategies to be plugged in.
type VersionManager interface {
	// CurrentVersion returns the current version state
	CurrentVersion() sync.Version
	
	// NextVersion generates the next version for a new event
	// The nodeID parameter allows node-specific versioning (e.g., for vector clocks)
	NextVersion(nodeID string) sync.Version
	
	// UpdateFromVersion updates the internal state based on an observed version
	// This is used when loading events from the store or receiving from peers
	UpdateFromVersion(version sync.Version) error
	
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
func NewVectorClockManagerFromVersion(version sync.Version) (*VectorClockManager, error) {
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
func (vm *VectorClockManager) CurrentVersion() sync.Version {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.clock.Clone()
}

// NextVersion increments the clock for the given node and returns the new version.
func (vm *VectorClockManager) NextVersion(nodeID string) sync.Version {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	err := vm.clock.Increment(nodeID)
	if err != nil {
		return nil
	}
	return vm.clock.Clone()
}

// UpdateFromVersion merges the observed version into the current state.
func (vm *VectorClockManager) UpdateFromVersion(version sync.Version) error {
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

// VersionedStore is a decorator for an EventStore that manages versioning automatically.
// It uses a pluggable VersionManager to handle different versioning strategies.
type VersionedStore struct {
	store           sync.EventStore
	nodeID          string
	versionManager  VersionManager
}

// NewVersionedStore creates a new versioned store decorator.
// It automatically initializes the version manager from the store's latest version.
func NewVersionedStore(store sync.EventStore, nodeID string, versionManager VersionManager) (*VersionedStore, error) {
	if versionManager == nil {
		return nil, fmt.Errorf("version manager cannot be nil")
	}
	
	// On startup, get the latest version from the store and update the version manager
	latestVersion, err := store.LatestVersion(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version from store: %w", err)
	}
	
	if err := versionManager.UpdateFromVersion(latestVersion); err != nil {
		return nil, fmt.Errorf("failed to initialize version manager: %w", err)
	}
	
	return &VersionedStore{
		store:          store,
		nodeID:         nodeID,
		versionManager: versionManager,
	}, nil
}

// Store generates the next version and stores the event with that version.
func (s *VersionedStore) Store(ctx context.Context, event sync.Event, version sync.Version) error {
	// If no version is provided, generate the next version
	if version == nil {
		version = s.versionManager.NextVersion(s.nodeID)
	} else {
		// If a version is provided, update our version manager state
		if err := s.versionManager.UpdateFromVersion(version); err != nil {
			return fmt.Errorf("failed to update version manager: %w", err)
		}
	}
	
	return s.store.Store(ctx, event, version)
}

// Load passes through to the underlying store and updates version manager state.
func (s *VersionedStore) Load(ctx context.Context, since sync.Version) ([]sync.EventWithVersion, error) {
	events, err := s.store.Load(ctx, since)
	if err != nil {
		return nil, err
	}
	
	// Update version manager state from loaded events
	for _, event := range events {
		if updateErr := s.versionManager.UpdateFromVersion(event.Version); updateErr != nil {
			// Log the error but don't fail the entire operation
			// In production, you might want to use a proper logger here
			fmt.Printf("Warning: failed to update version manager from loaded event: %v\n", updateErr)
		}
	}
	
	return events, nil
}

// LoadByAggregate passes through to the underlying store and updates version manager state.
func (s *VersionedStore) LoadByAggregate(ctx context.Context, aggregateID string, since sync.Version) ([]sync.EventWithVersion, error) {
	events, err := s.store.LoadByAggregate(ctx, aggregateID, since)
	if err != nil {
		return nil, err
	}
	
	// Update version manager state from loaded events
	for _, event := range events {
		if updateErr := s.versionManager.UpdateFromVersion(event.Version); updateErr != nil {
			fmt.Printf("Warning: failed to update version manager from loaded event: %v\n", updateErr)
		}
	}
	
	return events, nil
}

// LatestVersion returns the current version from the version manager.
func (s *VersionedStore) LatestVersion(ctx context.Context) (sync.Version, error) {
	return s.versionManager.CurrentVersion(), nil
}

// ParseVersion delegates to the underlying store.
func (s *VersionedStore) ParseVersion(ctx context.Context, versionStr string) (sync.Version, error) {
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
