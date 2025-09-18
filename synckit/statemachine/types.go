// Package statemachine provides state machine functionality for go-sync-kit operations.
// It enables enhanced observability, reliability, and debuggability through explicit state management.
package statemachine

import (
	"sync/atomic"
	"time"
)

// StateMachine defines the interface for state machines in go-sync-kit.
// It provides thread-safe state management with transition validation and history tracking.
type StateMachine[T comparable] interface {
	// Current returns the current state
	Current() T

	// Transition attempts to transition to the new state
	// Returns error if transition is invalid
	Transition(to T) error

	// TransitionWithContext transitions with additional metadata
	TransitionWithContext(to T, metadata map[string]interface{}) error

	// CanTransition checks if a transition from current state to target state is valid
	CanTransition(to T) bool

	// Subscribe adds a state change observer
	Subscribe(observer StateObserver[T])

	// Unsubscribe removes a state change observer
	Unsubscribe(observer StateObserver[T])

	// History returns recent state transition history.
	History() []StateTransition[T]

	// ExportDOT generates a DOT format representation of the state machine for visualization.
	ExportDOT() string

	// Persistence operations
	EnablePersistence(persistence StatePersistence[T], machineID string, config PersistenceConfig) error
	DisablePersistence()
	CreateSnapshot() *StateMachineSnapshot[T]
}

// StateObserver defines an interface for components that want to observe state changes.
// StateObserver receives notifications about state transitions.
// Implementations should be thread-safe and non-blocking.
type StateObserver[T comparable] interface {
	// OnTransition is called when a state transition succeeds
	OnTransition(transition StateTransition[T])

	// OnTransitionFailed is called when a state transition fails
	OnTransitionFailed(from, to T, err error, metadata map[string]interface{})
}

// StateTransition records information about a state change.
type StateTransition[T comparable] struct {
	// From is the previous state
	From T

	// To is the new state
	To T

	// Timestamp when the transition occurred
	Timestamp time.Time

	// Duration spent in the previous state (if available)
	Duration time.Duration

	// Metadata associated with the transition
	Metadata map[string]interface{}

	// TransitionID for tracking and correlation
	TransitionID string
}

// StateValidator validates state transitions.
type StateValidator[T comparable] interface {
	// ValidateTransition returns error if transition is invalid
	ValidateTransition(from, to T) error
}

// TransitionRules defines valid state transitions for a state machine.
type TransitionRules[T comparable] map[T][]T

// Contains checks if a target state is allowed from the current state.
func (tr TransitionRules[T]) Contains(from, to T) bool {
	allowedStates, exists := tr[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedStates {
		if allowed == to {
			return true
		}
	}
	return false
}

// StateMetrics provides metrics collection for state machine operations.
type StateMetrics[T comparable] interface {
	// RecordTransition records a successful state transition
	RecordTransition(from, to T, duration time.Duration, metadata map[string]interface{})

	// RecordTransitionError records a failed state transition
	RecordTransitionError(from, to T, err error, metadata map[string]interface{})

	// RecordCurrentState updates the current state metric
	RecordCurrentState(state T)

	// RecordStateDuration records time spent in a state
	RecordStateDuration(state T, duration time.Duration)
}

// StateMachineConfig provides configuration for state machine instances.
type StateMachineConfig[T comparable] struct {
	// InitialState is the starting state
	InitialState T

	// TransitionRules define valid state transitions
	TransitionRules TransitionRules[T]

	// Validator for custom transition validation (optional)
	Validator StateValidator[T]

	// Metrics collector for state machine operations (optional)
	Metrics StateMetrics[T]

	// MaxHistorySize limits the number of transitions to keep in history
	MaxHistorySize int

	// EnableMetrics controls whether to collect and emit metrics
	EnableMetrics bool

	// Name is a human-readable identifier for this state machine instance
	Name string
}

// DefaultConfig returns a default configuration for a state machine.
func DefaultConfig[T comparable](initialState T, rules TransitionRules[T]) StateMachineConfig[T] {
	return StateMachineConfig[T]{
		InitialState:    initialState,
		TransitionRules: rules,
		MaxHistorySize:  50,
		EnableMetrics:   true,
	}
}

// AtomicState provides thread-safe access to a state value using atomic operations.
type AtomicState[T comparable] struct {
	value atomic.Value
}

// NewAtomicState creates a new atomic state container with the given initial value.
func NewAtomicState[T comparable](initial T) *AtomicState[T] {
	as := &AtomicState[T]{}
	as.value.Store(initial)
	return as
}

// Load returns the current state value.
func (as *AtomicState[T]) Load() T {
	return as.value.Load().(T)
}

// Store sets the state value.
func (as *AtomicState[T]) Store(state T) {
	as.value.Store(state)
}

// CompareAndSwap atomically compares the current value with old and sets it to new if they match.
func (as *AtomicState[T]) CompareAndSwap(old, new T) bool {
	return as.value.CompareAndSwap(old, new)
}
