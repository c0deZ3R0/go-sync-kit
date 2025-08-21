package statemachine

import (
	"sync"
	"time"
)

// ConflictResolutionState represents the current state of conflict resolution processing
type ConflictResolutionState int

const (
	// DCRIdle indicates the conflict resolver is idle and ready for new conflicts
	DCRIdle ConflictResolutionState = iota
	
	// DCRAnalyzing indicates the conflict is being analyzed and metadata extracted
	DCRAnalyzing
	
	// DCRApplyingRules indicates rules are being evaluated and applied to the conflict
	DCRApplyingRules
	
	// DCRResolved indicates the conflict was successfully resolved automatically
	DCRResolved
	
	// DCRManualReview indicates the conflict requires manual review
	DCRManualReview
	
	// DCREscalated indicates the conflict was escalated to a higher authority
	DCREscalated
	
	// DCRFailed indicates the conflict resolution process failed
	DCRFailed
)

// String returns the string representation of the conflict resolution state
func (s ConflictResolutionState) String() string {
	switch s {
	case DCRIdle:
		return "idle"
	case DCRAnalyzing:
		return "analyzing"
	case DCRApplyingRules:
		return "applying_rules"
	case DCRResolved:
		return "resolved"
	case DCRManualReview:
		return "manual_review"
	case DCREscalated:
		return "escalated"
	case DCRFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ConflictResolutionStateMachine manages the state of conflict resolution processes
type ConflictResolutionStateMachine struct {
	stateMachine StateMachine[ConflictResolutionState]
	
	// Additional conflict resolution specific state
	currentConflictID string
	resolutionPath    []string              // Track which rules were applied
	matchedRules      []string              // Track which rules matched
	failedRules       []string              // Track which rules failed
	startTime         time.Time
	analysisMetadata  map[string]interface{}
	
	// Thread safety
	mu sync.RWMutex
}

// NewConflictResolutionStateMachine creates a new conflict resolution state machine
func NewConflictResolutionStateMachine(config *StateMachineConfig[ConflictResolutionState]) *ConflictResolutionStateMachine {
	if config == nil {
		// Define valid state transitions for conflict resolution
		transitionRules := TransitionRules[ConflictResolutionState]{
			DCRIdle: {DCRAnalyzing},
			DCRAnalyzing: {DCRApplyingRules, DCRFailed},
			DCRApplyingRules: {DCRResolved, DCRManualReview, DCREscalated, DCRFailed},
			DCRResolved: {DCRIdle},
			DCRManualReview: {DCRResolved, DCREscalated, DCRFailed, DCRIdle},
			DCREscalated: {DCRResolved, DCRFailed, DCRIdle},
			DCRFailed: {DCRIdle},
		}
		config = &StateMachineConfig[ConflictResolutionState]{
			InitialState:    DCRIdle,
			TransitionRules: transitionRules,
			MaxHistorySize:  100,
			EnableMetrics:   true,
			Name:           "ConflictResolutionStateMachine",
		}
	}

	// Define valid state transitions for conflict resolution if not provided
	if config.TransitionRules == nil {
		config.TransitionRules = TransitionRules[ConflictResolutionState]{
			DCRIdle: {DCRAnalyzing},
			DCRAnalyzing: {DCRApplyingRules, DCRFailed},
			DCRApplyingRules: {DCRResolved, DCRManualReview, DCREscalated, DCRFailed},
			DCRResolved: {DCRIdle},
			DCRManualReview: {DCRResolved, DCREscalated, DCRFailed, DCRIdle},
			DCREscalated: {DCRResolved, DCRFailed, DCRIdle},
			DCRFailed: {DCRIdle},
		}
	}

	sm, err := New(*config)
	if err != nil {
		// Fallback to basic configuration
		fallbackConfig := DefaultConfig(DCRIdle, config.TransitionRules)
		fallbackConfig.Name = "ConflictResolutionStateMachine"
		sm, _ = New(fallbackConfig)
	}
	
	return &ConflictResolutionStateMachine{
		stateMachine:     sm,
		resolutionPath:   make([]string, 0),
		matchedRules:     make([]string, 0),
		failedRules:      make([]string, 0),
		analysisMetadata: make(map[string]interface{}),
	}
}

// StartResolution begins a new conflict resolution process
func (sm *ConflictResolutionStateMachine) StartResolution(conflictID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.stateMachine.Transition(DCRAnalyzing); err != nil {
		return err
	}

	sm.currentConflictID = conflictID
	sm.startTime = time.Now()
	sm.resolutionPath = make([]string, 0)
	sm.matchedRules = make([]string, 0)
	sm.failedRules = make([]string, 0)
	sm.analysisMetadata = make(map[string]interface{})

	return nil
}

// AddAnalysisMetadata adds metadata discovered during conflict analysis
func (sm *ConflictResolutionStateMachine) AddAnalysisMetadata(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.analysisMetadata[key] = value
}

// RecordRuleEvaluation records that a rule was evaluated
func (sm *ConflictResolutionStateMachine) RecordRuleEvaluation(ruleName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.resolutionPath = append(sm.resolutionPath, ruleName)
}

// RecordRuleMatched records that a rule matched the conflict
func (sm *ConflictResolutionStateMachine) RecordRuleMatched(ruleName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.matchedRules = append(sm.matchedRules, ruleName)
}

// RecordRuleFailed records that a rule failed during execution
func (sm *ConflictResolutionStateMachine) RecordRuleFailed(ruleName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.failedRules = append(sm.failedRules, ruleName)
}

// BeginRuleApplication transitions to applying rules state
func (sm *ConflictResolutionStateMachine) BeginRuleApplication() error {
	return sm.stateMachine.Transition(DCRApplyingRules)
}

// CompleteResolution marks the conflict as successfully resolved
func (sm *ConflictResolutionStateMachine) CompleteResolution() error {
	return sm.stateMachine.Transition(DCRResolved)
}

// RequireManualReview marks the conflict for manual review
func (sm *ConflictResolutionStateMachine) RequireManualReview() error {
	return sm.stateMachine.Transition(DCRManualReview)
}

// EscalateConflict escalates the conflict to a higher authority
func (sm *ConflictResolutionStateMachine) EscalateConflict() error {
	return sm.stateMachine.Transition(DCREscalated)
}

// FailResolution marks the conflict resolution as failed
func (sm *ConflictResolutionStateMachine) FailResolution() error {
	return sm.stateMachine.Transition(DCRFailed)
}

// Reset returns the state machine to idle state
func (sm *ConflictResolutionStateMachine) Reset() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.stateMachine.Transition(DCRIdle); err != nil {
		return err
	}

	sm.currentConflictID = ""
	sm.resolutionPath = make([]string, 0)
	sm.matchedRules = make([]string, 0)
	sm.failedRules = make([]string, 0)
	sm.analysisMetadata = make(map[string]interface{})
	sm.startTime = time.Time{}

	return nil
}

// Current returns the current state of the conflict resolution state machine
func (sm *ConflictResolutionStateMachine) Current() ConflictResolutionState {
	return sm.stateMachine.Current()
}

// GetResolutionSummary returns a summary of the current resolution process
func (sm *ConflictResolutionStateMachine) GetResolutionSummary() *ConflictResolutionSummary {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var duration time.Duration
	if !sm.startTime.IsZero() {
		duration = time.Since(sm.startTime)
	}

	return &ConflictResolutionSummary{
		ConflictID:       sm.currentConflictID,
		CurrentState:     sm.stateMachine.Current(),
		ResolutionPath:   append([]string(nil), sm.resolutionPath...), // Copy slice
		MatchedRules:     append([]string(nil), sm.matchedRules...),
		FailedRules:      append([]string(nil), sm.failedRules...),
		AnalysisMetadata: copyMetadata(sm.analysisMetadata),
		Duration:         duration,
		StartTime:        sm.startTime,
	}
}

// ConflictResolutionSummary provides a summary of a conflict resolution process
type ConflictResolutionSummary struct {
	ConflictID       string                    `json:"conflict_id"`
	CurrentState     ConflictResolutionState   `json:"current_state"`
	ResolutionPath   []string                  `json:"resolution_path"`
	MatchedRules     []string                  `json:"matched_rules"`
	FailedRules      []string                  `json:"failed_rules"`
	AnalysisMetadata map[string]interface{}    `json:"analysis_metadata"`
	Duration         time.Duration             `json:"duration"`
	StartTime        time.Time                 `json:"start_time"`
}

// IsTerminalState returns true if the current state is a terminal state
func (s ConflictResolutionState) IsTerminalState() bool {
	switch s {
	case DCRResolved, DCREscalated, DCRFailed:
		return true
	default:
		return false
	}
}

// IsActiveState returns true if the state represents active processing
func (s ConflictResolutionState) IsActiveState() bool {
	switch s {
	case DCRAnalyzing, DCRApplyingRules:
		return true
	default:
		return false
	}
}

// RequiresIntervention returns true if the state requires human intervention
func (s ConflictResolutionState) RequiresIntervention() bool {
	switch s {
	case DCRManualReview, DCREscalated:
		return true
	default:
		return false
	}
}

// History returns the state transition history from the underlying state machine
func (sm *ConflictResolutionStateMachine) History() []StateTransition[ConflictResolutionState] {
	return sm.stateMachine.History()
}

// Subscribe adds a state observer to the underlying state machine
func (sm *ConflictResolutionStateMachine) Subscribe(observer StateObserver[ConflictResolutionState]) {
	sm.stateMachine.Subscribe(observer)
}

// Unsubscribe removes a state observer from the underlying state machine
func (sm *ConflictResolutionStateMachine) Unsubscribe(observer StateObserver[ConflictResolutionState]) {
	sm.stateMachine.Unsubscribe(observer)
}

// copyMetadata creates a deep copy of metadata map
func copyMetadata(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
