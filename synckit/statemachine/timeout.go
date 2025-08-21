package statemachine

import (
	"context"
	"sync"
	"time"
)

// TimeoutHandler manages state timeouts to prevent states from getting stuck.
// This is crucial for enterprise scenarios where states might hang due to
// network issues, external service failures, or other unforeseen circumstances.
type TimeoutHandler[T comparable] struct {
	stateMachine StateMachine[T]
	timeouts     map[T]time.Duration
	
	// Current timeout tracking
	currentTimeout context.CancelFunc
	mu             sync.RWMutex
	
	// Configuration
	config TimeoutConfig
	
	// Callbacks
	onTimeout func(from T, duration time.Duration)
}

// TimeoutConfig configures timeout behavior
type TimeoutConfig struct {
	// DefaultTimeout is used when no specific timeout is configured for a state
	DefaultTimeout time.Duration
	
	// EnableTimeouts globally enables/disables timeout handling
	EnableTimeouts bool
	
	// TimeoutAction specifies what to do when a timeout occurs
	TimeoutAction TimeoutAction
	
	// TargetState is the state to transition to on timeout (if TimeoutAction is TransitionTo)
	TargetState interface{}
	
	// MaxRetries for timeout recovery attempts
	MaxRetries int
	
	// RetryDelay between timeout recovery attempts
	RetryDelay time.Duration
}

// TimeoutAction defines what should happen when a state timeout occurs
type TimeoutAction int

const (
	// TimeoutActionTransition automatically transitions to a target state
	TimeoutActionTransition TimeoutAction = iota
	
	// TimeoutActionFail marks the state machine as failed
	TimeoutActionFail
	
	// TimeoutActionCallback calls a custom callback function
	TimeoutActionCallback
	
	// TimeoutActionReset resets the state machine to initial state
	TimeoutActionReset
	
	// TimeoutActionIgnore logs the timeout but takes no action
	TimeoutActionIgnore
)

// NewTimeoutHandler creates a new timeout handler for a state machine.
func NewTimeoutHandler[T comparable](sm StateMachine[T], config TimeoutConfig) *TimeoutHandler[T] {
	return &TimeoutHandler[T]{
		stateMachine: sm,
		timeouts:     make(map[T]time.Duration),
		config:       config,
	}
}

// SetTimeout configures a timeout for a specific state.
// If the state machine remains in this state longer than the specified duration,
// the configured timeout action will be triggered.
func (th *TimeoutHandler[T]) SetTimeout(state T, timeout time.Duration) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.timeouts[state] = timeout
}

// SetTimeouts configures multiple state timeouts at once.
func (th *TimeoutHandler[T]) SetTimeouts(timeouts map[T]time.Duration) {
	th.mu.Lock()
	defer th.mu.Unlock()
	for state, timeout := range timeouts {
		th.timeouts[state] = timeout
	}
}

// RemoveTimeout removes a timeout for a specific state.
func (th *TimeoutHandler[T]) RemoveTimeout(state T) {
	th.mu.Lock()
	defer th.mu.Unlock()
	delete(th.timeouts, state)
}

// OnTimeout sets a callback function to be called when a timeout occurs.
func (th *TimeoutHandler[T]) OnTimeout(callback func(from T, duration time.Duration)) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.onTimeout = callback
}

// StartTimeout begins timeout monitoring for the current state.
// This should be called after each state transition.
func (th *TimeoutHandler[T]) StartTimeout() {
	if !th.config.EnableTimeouts {
		return
	}
	
	th.mu.Lock()
	defer th.mu.Unlock()
	
	// Cancel any existing timeout
	if th.currentTimeout != nil {
		th.currentTimeout()
		th.currentTimeout = nil
	}
	
	currentState := th.stateMachine.Current()
	timeout := th.getTimeoutForState(currentState)
	
	if timeout <= 0 {
		// No timeout configured for this state
		return
	}
	
	// Create a cancellable context for this timeout
	ctx, cancel := context.WithCancel(context.Background())
	th.currentTimeout = cancel
	
	// Start timeout monitoring in a goroutine
	go th.monitorTimeout(ctx, currentState, timeout)
}

// StopTimeout cancels any active timeout monitoring.
func (th *TimeoutHandler[T]) StopTimeout() {
	th.mu.Lock()
	defer th.mu.Unlock()
	
	if th.currentTimeout != nil {
		th.currentTimeout()
		th.currentTimeout = nil
	}
}

// getTimeoutForState returns the configured timeout for a state,
// or the default timeout if none is specifically configured.
func (th *TimeoutHandler[T]) getTimeoutForState(state T) time.Duration {
	if timeout, exists := th.timeouts[state]; exists {
		return timeout
	}
	return th.config.DefaultTimeout
}

// monitorTimeout runs in a goroutine to monitor for state timeouts.
func (th *TimeoutHandler[T]) monitorTimeout(ctx context.Context, state T, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	select {
	case <-ctx.Done():
		// Timeout was cancelled (state changed or timeout handler stopped)
		return
		
	case <-timer.C:
		// Timeout occurred
		th.handleTimeout(state, timeout)
	}
}

// handleTimeout processes a state timeout according to the configured action.
func (th *TimeoutHandler[T]) handleTimeout(state T, duration time.Duration) {
	// Call the timeout callback if configured
	if th.onTimeout != nil {
		th.onTimeout(state, duration)
	}
	
	// Take the configured action
	switch th.config.TimeoutAction {
	case TimeoutActionTransition:
		th.handleTimeoutTransition(state)
		
	case TimeoutActionFail:
		th.handleTimeoutFail(state)
		
	case TimeoutActionReset:
		th.handleTimeoutReset(state)
		
	case TimeoutActionCallback:
		// Callback was already called above
		
	case TimeoutActionIgnore:
		// Do nothing except log (callback handles logging)
	}
}

// handleTimeoutTransition transitions to the configured target state on timeout.
func (th *TimeoutHandler[T]) handleTimeoutTransition(state T) {
	if th.config.TargetState == nil {
		return
	}
	
	if targetState, ok := th.config.TargetState.(T); ok {
		th.stateMachine.TransitionWithContext(targetState, map[string]interface{}{
			"timeout":      true,
			"timeout_from": state,
			"reason":       "state_timeout",
		})
	}
}

// handleTimeoutFail transitions to a failed state on timeout.
func (th *TimeoutHandler[T]) handleTimeoutFail(state T) {
	// For this to work, the state machine must have a "failed" state defined
	// This is a generic approach - specific implementations might need different logic
	
	// Try common failure state names
	failureStates := []interface{}{"failed", "error", "timeout", "stuck"}
	
	for _, failState := range failureStates {
		if typedState, ok := failState.(T); ok {
			if th.stateMachine.CanTransition(typedState) {
				th.stateMachine.TransitionWithContext(typedState, map[string]interface{}{
					"timeout":      true,
					"timeout_from": state,
					"reason":       "state_timeout_failure",
				})
				return
			}
		}
	}
}

// handleTimeoutReset resets the state machine to its initial state on timeout.
func (th *TimeoutHandler[T]) handleTimeoutReset(state T) {
	// Reset to initial state - we need to access the initial state from config
	// This is a simplified implementation - in practice you might want to store the initial state
	// or provide it through configuration
	if resetter, ok := th.stateMachine.(interface{ Reset() error }); ok {
		resetter.Reset()
	}
}

// TimeoutObserver is a state observer that automatically starts/stops timeouts
// based on state transitions. This provides automatic timeout management.
type TimeoutObserver[T comparable] struct {
	timeoutHandler *TimeoutHandler[T]
}

// NewTimeoutObserver creates a new timeout observer that automatically manages
// timeouts based on state transitions.
func NewTimeoutObserver[T comparable](timeoutHandler *TimeoutHandler[T]) *TimeoutObserver[T] {
	return &TimeoutObserver[T]{
		timeoutHandler: timeoutHandler,
	}
}

// OnTransition is called when a state transition occurs.
func (to *TimeoutObserver[T]) OnTransition(transition StateTransition[T]) {
	// Start timeout for the new state
	to.timeoutHandler.StartTimeout()
}

// OnTransitionFailed is called when a state transition fails.
func (to *TimeoutObserver[T]) OnTransitionFailed(from, toState T, err error, metadata map[string]interface{}) {
	// Keep existing timeout on transition failure
}

// TimeoutMetrics tracks timeout-related metrics for monitoring and alerting.
type TimeoutMetrics struct {
	TotalTimeouts      int64         `json:"total_timeouts"`
	TimeoutsByState    map[string]int64 `json:"timeouts_by_state"`
	AverageTimeoutTime time.Duration `json:"average_timeout_time"`
	MaxTimeoutTime     time.Duration `json:"max_timeout_time"`
	LastTimeout        time.Time     `json:"last_timeout"`
	
	mu sync.RWMutex
}

// NewTimeoutMetrics creates a new timeout metrics tracker.
func NewTimeoutMetrics() *TimeoutMetrics {
	return &TimeoutMetrics{
		TimeoutsByState: make(map[string]int64),
	}
}

// RecordTimeout records a timeout occurrence for metrics.
func (tm *TimeoutMetrics) RecordTimeout(state string, duration time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	tm.TotalTimeouts++
	tm.TimeoutsByState[state]++
	tm.LastTimeout = time.Now()
	
	// Update duration metrics
	if duration > tm.MaxTimeoutTime {
		tm.MaxTimeoutTime = duration
	}
	
	// Calculate running average (simplified)
	if tm.TotalTimeouts == 1 {
		tm.AverageTimeoutTime = duration
	} else {
		// Running average: new_avg = old_avg + (new_value - old_avg) / count
		tm.AverageTimeoutTime = tm.AverageTimeoutTime + 
			(duration-tm.AverageTimeoutTime)/time.Duration(tm.TotalTimeouts)
	}
}

// GetMetrics returns a copy of the current metrics.
func (tm *TimeoutMetrics) GetMetrics() TimeoutMetrics {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	copyMetrics := TimeoutMetrics{
		TotalTimeouts:      tm.TotalTimeouts,
		TimeoutsByState:    make(map[string]int64),
		AverageTimeoutTime: tm.AverageTimeoutTime,
		MaxTimeoutTime:     tm.MaxTimeoutTime,
		LastTimeout:        tm.LastTimeout,
	}
	
	for state, count := range tm.TimeoutsByState {
		copyMetrics.TimeoutsByState[state] = count
	}
	
	return copyMetrics
}

// DefaultTimeoutConfig returns sensible defaults for timeout configuration.
func DefaultTimeoutConfig[T comparable](failureState T) TimeoutConfig {
	return TimeoutConfig{
		DefaultTimeout: 5 * time.Minute,
		EnableTimeouts: true,
		TimeoutAction:  TimeoutActionTransition,
		TargetState:    failureState,
		MaxRetries:     3,
		RetryDelay:     30 * time.Second,
	}
}
