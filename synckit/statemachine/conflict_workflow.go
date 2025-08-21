package statemachine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// ConflictWorkflow tracks the entire lifecycle of a conflict resolution process
// providing comprehensive audit trails and workflow state management.
type ConflictWorkflow struct {
	// Core conflict information
	conflict      types.Conflict
	conflictID    string
	startTime     time.Time
	endTime       time.Time
	
	// Resolution process tracking
	rulesEvaluated []RuleEvaluation
	rulesMatched   []string
	rulesFailed    []RuleFailure
	decision       string
	reasons        []string
	resolvedEvents []types.EventWithVersion
	
	// State machine integration
	stateMachine *ConflictResolutionStateMachine
	stateHistory []StateTransition[ConflictResolutionState]
	
	// Analysis and metadata
	analysisResults    map[string]interface{}
	performanceMetrics WorkflowPerformanceMetrics
	
	// Thread safety
	mu sync.RWMutex
}

// RuleEvaluation records the evaluation of a single rule
type RuleEvaluation struct {
	RuleName      string                 `json:"rule_name"`
	Matched       bool                   `json:"matched"`
	EvaluatedAt   time.Time              `json:"evaluated_at"`
	Duration      time.Duration          `json:"duration"`
	Error         error                  `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// RuleFailure records when a rule failed during execution
type RuleFailure struct {
	RuleName    string                 `json:"rule_name"`
	Error       error                  `json:"error"`
	FailedAt    time.Time              `json:"failed_at"`
	Context     string                 `json:"context"`
	Retryable   bool                   `json:"retryable"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowPerformanceMetrics tracks performance data for the resolution process
type WorkflowPerformanceMetrics struct {
	TotalDuration     time.Duration `json:"total_duration"`
	AnalysisDuration  time.Duration `json:"analysis_duration"`
	RulesDuration     time.Duration `json:"rules_duration"`
	ResolutionDuration time.Duration `json:"resolution_duration"`
	RulesEvaluated    int           `json:"rules_evaluated"`
	RulesMatched      int           `json:"rules_matched"`
	RulesFailed       int           `json:"rules_failed"`
	MemoryUsage       int64         `json:"memory_usage_bytes"`
}

// WorkflowOptions configures the workflow tracking behavior
type WorkflowOptions struct {
	EnableDetailedTracking bool
	MaxRuleEvaluations    int
	MaxStateHistory       int
	TrackPerformance      bool
	IncludeMetadata       bool
}

// DefaultWorkflowOptions returns sensible defaults for workflow configuration
func DefaultWorkflowOptions() *WorkflowOptions {
	return &WorkflowOptions{
		EnableDetailedTracking: true,
		MaxRuleEvaluations:    100,
		MaxStateHistory:       50,
		TrackPerformance:      true,
		IncludeMetadata:       true,
	}
}

// NewConflictWorkflow creates a new workflow tracker for a conflict
func NewConflictWorkflow(conflict types.Conflict, stateMachine *ConflictResolutionStateMachine, options *WorkflowOptions) *ConflictWorkflow {
	if options == nil {
		options = DefaultWorkflowOptions()
	}

	conflictID := generateConflictID(conflict)
	
	return &ConflictWorkflow{
		conflict:           conflict,
		conflictID:         conflictID,
		startTime:          time.Now(),
		rulesEvaluated:     make([]RuleEvaluation, 0),
		rulesMatched:       make([]string, 0),
		rulesFailed:        make([]RuleFailure, 0),
		reasons:            make([]string, 0),
		resolvedEvents:     make([]types.EventWithVersion, 0),
		stateMachine:       stateMachine,
		stateHistory:       make([]StateTransition[ConflictResolutionState], 0),
		analysisResults:    make(map[string]interface{}),
		performanceMetrics: WorkflowPerformanceMetrics{},
	}
}

// StartAnalysis begins the conflict analysis phase
func (cw *ConflictWorkflow) StartAnalysis() {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	
	analysisStart := time.Now()
	cw.performanceMetrics.AnalysisDuration = analysisStart.Sub(cw.startTime)
	
	// Add basic analysis metadata
	cw.analysisResults["conflict_type"] = cw.conflict.EventType
	cw.analysisResults["aggregate_id"] = cw.conflict.AggregateID
	cw.analysisResults["changed_fields_count"] = len(cw.conflict.ChangedFields)
	cw.analysisResults["has_local_version"] = cw.conflict.Local.Version != nil
	cw.analysisResults["has_remote_version"] = cw.conflict.Remote.Version != nil
	
	// Version comparison analysis
	if cw.conflict.Local.Version != nil && cw.conflict.Remote.Version != nil {
		comparison := cw.conflict.Local.Version.Compare(cw.conflict.Remote.Version)
		cw.analysisResults["version_comparison"] = comparison
		switch comparison {
		case -1:
			cw.analysisResults["version_relationship"] = "local_older"
		case 0:
			cw.analysisResults["version_relationship"] = "equal"
		case 1:
			cw.analysisResults["version_relationship"] = "local_newer"
		}
	}
}

// RecordRuleEvaluation records the evaluation of a rule
func (cw *ConflictWorkflow) RecordRuleEvaluation(ruleName string, matched bool, duration time.Duration, err error, metadata map[string]interface{}) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	
	evaluation := RuleEvaluation{
		RuleName:    ruleName,
		Matched:     matched,
		EvaluatedAt: time.Now(),
		Duration:    duration,
		Error:       err,
		Metadata:    metadata,
	}
	
	cw.rulesEvaluated = append(cw.rulesEvaluated, evaluation)
	cw.performanceMetrics.RulesEvaluated++
	
	if matched {
		cw.rulesMatched = append(cw.rulesMatched, ruleName)
		cw.performanceMetrics.RulesMatched++
	}
	
	if err != nil {
		failure := RuleFailure{
			RuleName:  ruleName,
			Error:     err,
			FailedAt:  time.Now(),
			Context:   "rule_evaluation",
			Retryable: isRetryableError(err),
			Metadata:  metadata,
		}
		cw.rulesFailed = append(cw.rulesFailed, failure)
		cw.performanceMetrics.RulesFailed++
	}
}

// RecordStateTransition records a state machine transition
func (cw *ConflictWorkflow) RecordStateTransition(from, to ConflictResolutionState, duration time.Duration, metadata map[string]interface{}) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	
	transition := StateTransition[ConflictResolutionState]{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
		Duration:  duration,
		Metadata:  metadata,
	}
	
	cw.stateHistory = append(cw.stateHistory, transition)
}

// CompleteResolution finalizes the workflow with the resolution result
func (cw *ConflictWorkflow) CompleteResolution(decision string, reasons []string, resolvedEvents []types.EventWithVersion) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	
	cw.endTime = time.Now()
	cw.decision = decision
	cw.reasons = append(cw.reasons, reasons...)
	cw.resolvedEvents = append(cw.resolvedEvents, resolvedEvents...)
	
	// Update performance metrics
	cw.performanceMetrics.TotalDuration = cw.endTime.Sub(cw.startTime)
	cw.performanceMetrics.ResolutionDuration = time.Since(cw.endTime.Add(-cw.performanceMetrics.RulesDuration))
}

// GenerateAuditTrail creates a comprehensive audit trail for the resolution process
func (cw *ConflictWorkflow) GenerateAuditTrail() *ConflictAuditTrail {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	
	return &ConflictAuditTrail{
		ConflictID:         cw.conflictID,
		Timestamp:          cw.startTime,
		CompletedAt:        cw.endTime,
		EventType:          cw.conflict.EventType,
		AggregateID:        cw.conflict.AggregateID,
		ChangedFields:      append([]string(nil), cw.conflict.ChangedFields...),
		Decision:           cw.decision,
		Reasons:            append([]string(nil), cw.reasons...),
		RulesEvaluated:     append([]RuleEvaluation(nil), cw.rulesEvaluated...),
		RulesMatched:       append([]string(nil), cw.rulesMatched...),
		RulesFailed:        append([]RuleFailure(nil), cw.rulesFailed...),
		StateHistory:       append([]StateTransition[ConflictResolutionState](nil), cw.stateHistory...),
		AnalysisResults:    copyMetadata(cw.analysisResults),
		PerformanceMetrics: cw.performanceMetrics,
		ResolvedEvents:     append([]types.EventWithVersion(nil), cw.resolvedEvents...),
	}
}

// GetCurrentStatus returns the current status of the workflow
func (cw *ConflictWorkflow) GetCurrentStatus() *WorkflowStatus {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	
	currentState := DCRIdle
	if cw.stateMachine != nil {
		currentState = cw.stateMachine.Current()
	}
	
	return &WorkflowStatus{
		ConflictID:         cw.conflictID,
		CurrentState:       currentState,
		Duration:           time.Since(cw.startTime),
		RulesEvaluated:     len(cw.rulesEvaluated),
		RulesMatched:       len(cw.rulesMatched),
		RulesFailed:        len(cw.rulesFailed),
		Decision:           cw.decision,
		IsComplete:         !cw.endTime.IsZero(),
		LastActivity:       cw.getLastActivityTime(),
	}
}

// ConflictAuditTrail provides a complete audit record of the conflict resolution process
type ConflictAuditTrail struct {
	ConflictID         string                                      `json:"conflict_id"`
	Timestamp          time.Time                                   `json:"timestamp"`
	CompletedAt        time.Time                                   `json:"completed_at"`
	EventType          string                                      `json:"event_type"`
	AggregateID        string                                      `json:"aggregate_id"`
	ChangedFields      []string                                    `json:"changed_fields"`
	Decision           string                                      `json:"decision"`
	Reasons            []string                                    `json:"reasons"`
	RulesEvaluated     []RuleEvaluation                           `json:"rules_evaluated"`
	RulesMatched       []string                                    `json:"rules_matched"`
	RulesFailed        []RuleFailure                              `json:"rules_failed"`
	StateHistory       []StateTransition[ConflictResolutionState] `json:"state_history"`
	AnalysisResults    map[string]interface{}                     `json:"analysis_results"`
	PerformanceMetrics WorkflowPerformanceMetrics                 `json:"performance_metrics"`
	ResolvedEvents     []types.EventWithVersion                 `json:"resolved_events"`
}

// WorkflowStatus provides real-time status information about a workflow
type WorkflowStatus struct {
	ConflictID     string                    `json:"conflict_id"`
	CurrentState   ConflictResolutionState   `json:"current_state"`
	Duration       time.Duration             `json:"duration"`
	RulesEvaluated int                       `json:"rules_evaluated"`
	RulesMatched   int                       `json:"rules_matched"`
	RulesFailed    int                       `json:"rules_failed"`
	Decision       string                    `json:"decision"`
	IsComplete     bool                      `json:"is_complete"`
	LastActivity   time.Time                 `json:"last_activity"`
}

// WorkflowManager manages multiple concurrent conflict workflows
type WorkflowManager struct {
	workflows map[string]*ConflictWorkflow
	mu        sync.RWMutex
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		workflows: make(map[string]*ConflictWorkflow),
	}
}

// StartWorkflow begins tracking a new conflict resolution workflow
func (wm *WorkflowManager) StartWorkflow(conflict types.Conflict, stateMachine *ConflictResolutionStateMachine, options *WorkflowOptions) *ConflictWorkflow {
	workflow := NewConflictWorkflow(conflict, stateMachine, options)
	
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	wm.workflows[workflow.conflictID] = workflow
	return workflow
}

// GetWorkflow retrieves a workflow by ID
func (wm *WorkflowManager) GetWorkflow(conflictID string) (*ConflictWorkflow, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	workflow, exists := wm.workflows[conflictID]
	return workflow, exists
}

// CompleteWorkflow finalizes and archives a workflow
func (wm *WorkflowManager) CompleteWorkflow(conflictID string) *ConflictAuditTrail {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	
	workflow, exists := wm.workflows[conflictID]
	if !exists {
		return nil
	}
	
	auditTrail := workflow.GenerateAuditTrail()
	delete(wm.workflows, conflictID)
	
	return auditTrail
}

// ListActiveWorkflows returns information about all active workflows
func (wm *WorkflowManager) ListActiveWorkflows() []*WorkflowStatus {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	
	statuses := make([]*WorkflowStatus, 0, len(wm.workflows))
	for _, workflow := range wm.workflows {
		statuses = append(statuses, workflow.GetCurrentStatus())
	}
	
	return statuses
}

// Helper functions

func generateConflictID(conflict types.Conflict) string {
	return fmt.Sprintf("%s:%s:%d", conflict.EventType, conflict.AggregateID, time.Now().UnixNano())
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for context cancellation or timeout (not retryable)
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	// Add more sophisticated error classification as needed
	return true
}

func (cw *ConflictWorkflow) getLastActivityTime() time.Time {
	if len(cw.stateHistory) > 0 {
		return cw.stateHistory[len(cw.stateHistory)-1].Timestamp
	}
	if len(cw.rulesEvaluated) > 0 {
		return cw.rulesEvaluated[len(cw.rulesEvaluated)-1].EvaluatedAt
	}
	return cw.startTime
}

