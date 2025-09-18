package synckit

import (
	"fmt"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// SyncState represents the state of a sync operation.
type SyncState int

const (
	// SyncIdle indicates the sync manager is ready for new operations
	SyncIdle SyncState = iota

	// SyncInitializing indicates a sync operation is being set up
	SyncInitializing

	// SyncPushing indicates local events are being sent to remote
	SyncPushing

	// SyncPulling indicates remote events are being retrieved
	SyncPulling

	// SyncResolvingConflicts indicates conflicts are being processed
	SyncResolvingConflicts

	// SyncCompleted indicates the sync operation completed successfully
	SyncCompleted

	// SyncFailed indicates the sync operation failed
	SyncFailed

	// SyncCancelled indicates the sync operation was cancelled
	SyncCancelled
)

// String returns the string representation of the sync state.
func (s SyncState) String() string {
	switch s {
	case SyncIdle:
		return "idle"
	case SyncInitializing:
		return "initializing"
	case SyncPushing:
		return "pushing"
	case SyncPulling:
		return "pulling"
	case SyncResolvingConflicts:
		return "resolving_conflicts"
	case SyncCompleted:
		return "completed"
	case SyncFailed:
		return "failed"
	case SyncCancelled:
		return "cancelled"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// IsTerminal returns true if this state represents the end of a sync operation.
func (s SyncState) IsTerminal() bool {
	return s == SyncCompleted || s == SyncFailed || s == SyncCancelled
}

// IsActive returns true if this state represents an active sync operation.
func (s SyncState) IsActive() bool {
	return s == SyncInitializing || s == SyncPushing || s == SyncPulling || s == SyncResolvingConflicts
}

// CanAutoSync returns true if auto-sync operations are allowed in this state.
func (s SyncState) CanAutoSync() bool {
	return s == SyncIdle
}

// SyncStateTransitionRules defines the valid transitions for sync operations.
func SyncStateTransitionRules() statemachine.TransitionRules[SyncState] {
	return statemachine.TransitionRules[SyncState]{
		// From Idle, can start a new sync operation
		SyncIdle: {SyncInitializing},

		// From Initializing, can go to push/pull phases or fail/cancel
		SyncInitializing: {SyncPushing, SyncPulling, SyncFailed, SyncCancelled},

		// From Pushing, can proceed to pull, complete, or fail/cancel
		SyncPushing: {SyncPulling, SyncCompleted, SyncFailed, SyncCancelled},

		// From Pulling, can proceed to push, resolve conflicts, complete, or fail/cancel
		SyncPulling: {SyncPushing, SyncResolvingConflicts, SyncCompleted, SyncFailed, SyncCancelled},

		// From Resolving Conflicts, can complete or fail/cancel
		SyncResolvingConflicts: {SyncCompleted, SyncFailed, SyncCancelled},

		// Terminal states can only return to Idle
		SyncCompleted: {SyncIdle},
		SyncFailed:    {SyncIdle},
		SyncCancelled: {SyncIdle},
	}
}

// NewSyncStateMachine creates a new state machine for sync operations.
func NewSyncStateMachine() (statemachine.StateMachine[SyncState], error) {
	return statemachine.NewBuilder(SyncIdle).
		Allow(SyncIdle, SyncInitializing).
		Allow(SyncInitializing, SyncPushing, SyncPulling, SyncFailed, SyncCancelled).
		Allow(SyncPushing, SyncPulling, SyncCompleted, SyncFailed, SyncCancelled).
		Allow(SyncPulling, SyncPushing, SyncResolvingConflicts, SyncCompleted, SyncFailed, SyncCancelled).
		Allow(SyncResolvingConflicts, SyncCompleted, SyncFailed, SyncCancelled).
		Allow(SyncCompleted, SyncIdle).
		Allow(SyncFailed, SyncIdle).
		Allow(SyncCancelled, SyncIdle).
		WithName("sync_operations").
		Build()
}

// SyncStateObserver provides a convenient way to observe sync state changes.
type SyncStateObserver struct {
	// OnStateChange is called when a sync state transition succeeds
	OnStateChange func(from, to SyncState, duration time.Duration, metadata map[string]interface{})

	// OnStateChangeError is called when a sync state transition fails
	OnStateChangeError func(from, to SyncState, err error, metadata map[string]interface{})
}

// OnTransition implements the StateObserver interface.
func (o *SyncStateObserver) OnTransition(transition statemachine.StateTransition[SyncState]) {
	if o.OnStateChange != nil {
		o.OnStateChange(transition.From, transition.To, transition.Duration, transition.Metadata)
	}
}

// OnTransitionFailed implements the StateObserver interface.
func (o *SyncStateObserver) OnTransitionFailed(from, to SyncState, err error, metadata map[string]interface{}) {
	if o.OnStateChangeError != nil {
		o.OnStateChangeError(from, to, err, metadata)
	}
}

// StateAwareSyncResult extends SyncResult with state machine information.
type StateAwareSyncResult struct {
	*SyncResult

	// StateTransitions contains the state transitions that occurred during sync
	StateTransitions []statemachine.StateTransition[SyncState]

	// FinalState is the final state of the sync operation
	FinalState SyncState

	// StateChanges is the number of state transitions that occurred
	StateChanges int
}

// NewStateAwareSyncResult creates a StateAwareSyncResult from a regular SyncResult and state machine history.
func NewStateAwareSyncResult(result *SyncResult, stateMachine statemachine.StateMachine[SyncState]) *StateAwareSyncResult {
	history := stateMachine.History()

	// Find transitions that occurred during this sync operation
	var syncTransitions []statemachine.StateTransition[SyncState]

	// Look for transitions from the start time of the sync operation
	if result != nil {
		for _, transition := range history {
			if transition.Timestamp.After(result.StartTime) || transition.Timestamp.Equal(result.StartTime) {
				syncTransitions = append(syncTransitions, transition)
			}
		}
	}

	return &StateAwareSyncResult{
		SyncResult:       result,
		StateTransitions: syncTransitions,
		FinalState:       stateMachine.Current(),
		StateChanges:     len(syncTransitions),
	}
}
