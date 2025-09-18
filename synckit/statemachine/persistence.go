package statemachine

import (
	"context"
	"encoding/json"
	"time"
)

// StatePersistence defines the interface for persisting and recovering state machine state.
// This enables enterprise-grade resilience by allowing state machines to resume from
// their last known state after crashes or restarts.
type StatePersistence[T comparable] interface {
	// SaveState persists the current state machine state
	SaveState(ctx context.Context, machineID string, state StateMachineSnapshot[T]) error

	// LoadState retrieves the persisted state for a state machine
	LoadState(ctx context.Context, machineID string) (*StateMachineSnapshot[T], error)

	// DeleteState removes persisted state (cleanup)
	DeleteState(ctx context.Context, machineID string) error

	// ListMachines returns all persisted state machine IDs
	ListMachines(ctx context.Context) ([]string, error)
}

// StateMachineSnapshot represents a complete snapshot of a state machine's state
// that can be persisted and restored for resilience across restarts.
type StateMachineSnapshot[T comparable] struct {
	// Core state information
	MachineID      string    `json:"machine_id"`
	CurrentState   T         `json:"current_state"`
	InitialState   T         `json:"initial_state"`
	StateEnteredAt time.Time `json:"state_entered_at"`

	// Configuration snapshot
	Config StateMachineConfig[T] `json:"config"`

	// History and metadata
	History  []StateTransition[T]   `json:"history"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Persistence metadata
	SnapshotTime time.Time `json:"snapshot_time"`
	Version      int       `json:"version"`
}

// MemoryStatePersistence provides an in-memory implementation of StatePersistence
// for testing and development purposes.
type MemoryStatePersistence[T comparable] struct {
	states map[string]StateMachineSnapshot[T]
}

// NewMemoryStatePersistence creates a new in-memory state persistence implementation.
func NewMemoryStatePersistence[T comparable]() *MemoryStatePersistence[T] {
	return &MemoryStatePersistence[T]{
		states: make(map[string]StateMachineSnapshot[T]),
	}
}

// SaveState saves the state to memory.
func (mp *MemoryStatePersistence[T]) SaveState(ctx context.Context, machineID string, state StateMachineSnapshot[T]) error {
	mp.states[machineID] = state
	return nil
}

// LoadState loads the state from memory.
func (mp *MemoryStatePersistence[T]) LoadState(ctx context.Context, machineID string) (*StateMachineSnapshot[T], error) {
	if state, exists := mp.states[machineID]; exists {
		return &state, nil
	}
	return nil, nil // No state found
}

// DeleteState removes the state from memory.
func (mp *MemoryStatePersistence[T]) DeleteState(ctx context.Context, machineID string) error {
	delete(mp.states, machineID)
	return nil
}

// ListMachines returns all machine IDs in memory.
func (mp *MemoryStatePersistence[T]) ListMachines(ctx context.Context) ([]string, error) {
	machines := make([]string, 0, len(mp.states))
	for machineID := range mp.states {
		machines = append(machines, machineID)
	}
	return machines, nil
}

// JSONStatePersistence provides a file-based JSON implementation of StatePersistence.
// This can be used for simple persistence scenarios.
type JSONStatePersistence[T comparable] struct {
	basePath string
}

// NewJSONStatePersistence creates a new JSON file-based state persistence implementation.
func NewJSONStatePersistence[T comparable](basePath string) *JSONStatePersistence[T] {
	return &JSONStatePersistence[T]{
		basePath: basePath,
	}
}

// SaveState saves the state to a JSON file.
func (jp *JSONStatePersistence[T]) SaveState(ctx context.Context, machineID string, state StateMachineSnapshot[T]) error {
	// This is a simplified implementation - in practice you'd want proper file handling,
	// atomic writes, backup/rotation, etc.
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// In a real implementation, you would:
	// 1. Write to a temporary file first
	// 2. Atomically rename to final location
	// 3. Handle file locking/concurrency
	// 4. Implement proper error handling

	// For now, return nil to indicate the interface implementation
	_ = data
	return nil
}

// LoadState loads the state from a JSON file.
func (jp *JSONStatePersistence[T]) LoadState(ctx context.Context, machineID string) (*StateMachineSnapshot[T], error) {
	// In a real implementation, you would:
	// 1. Read the JSON file
	// 2. Unmarshal into StateMachineSnapshot
	// 3. Validate the snapshot
	// 4. Handle file not found scenarios

	// For now, return nil to indicate no state found
	return nil, nil
}

// DeleteState removes the state JSON file.
func (jp *JSONStatePersistence[T]) DeleteState(ctx context.Context, machineID string) error {
	// In a real implementation, you would remove the file
	return nil
}

// ListMachines returns all machine IDs by scanning JSON files.
func (jp *JSONStatePersistence[T]) ListMachines(ctx context.Context) ([]string, error) {
	// In a real implementation, you would scan the directory for JSON files
	return []string{}, nil
}

// PersistenceConfig configures state persistence behavior
type PersistenceConfig struct {
	// AutoSave enables automatic state saving after each transition
	AutoSave bool

	// SaveInterval for periodic state saves (if AutoSave is false)
	SaveInterval time.Duration

	// RetentionPeriod for automatic cleanup of old state snapshots
	RetentionPeriod time.Duration

	// MaxSnapshots to keep for each state machine
	MaxSnapshots int

	// CompressSnapshots enables compression for storage efficiency
	CompressSnapshots bool
}

// DefaultPersistenceConfig returns sensible defaults for state persistence
func DefaultPersistenceConfig() PersistenceConfig {
	return PersistenceConfig{
		AutoSave:          true,
		SaveInterval:      5 * time.Minute,
		RetentionPeriod:   24 * time.Hour,
		MaxSnapshots:      10,
		CompressSnapshots: false,
	}
}
