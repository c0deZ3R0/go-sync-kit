package statemachine

import (
	"context"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// StatefulConflictResolver extends the ConflictResolver interface with state machine capabilities
// providing enhanced observability, workflow tracking, and audit trails for conflict resolution.
type StatefulConflictResolver interface {
	types.ConflictResolver

	// State machine operations
	GetCurrentState() ConflictResolutionState
	GetStateHistory() []StateTransition[ConflictResolutionState]
	SubscribeToStateChanges(observer StateObserver[ConflictResolutionState])

	// Workflow operations
	GetWorkflowManager() *WorkflowManager
	GetActiveWorkflows() []*WorkflowStatus
	GetWorkflowByID(conflictID string) (*ConflictWorkflow, bool)
	GetAuditTrail(conflictID string) (*ConflictAuditTrail, error)

	// Configuration and lifecycle
	Configure(options *StatefulResolverOptions) error
	IsStateMachineEnabled() bool
	GetPerformanceMetrics() *ResolverPerformanceMetrics
}

// StatefulResolverOptions provides configuration for stateful conflict resolution
type StatefulResolverOptions struct {
	// State machine configuration
	EnableStateMachine     bool
	MaxStateHistorySize    int
	StateTransitionTimeout time.Duration

	// Workflow configuration
	EnableWorkflowTracking bool
	MaxConcurrentWorkflows int
	WorkflowRetentionTime  time.Duration
	WorkflowOptions        *WorkflowOptions

	// Performance and observability
	EnablePerformanceMetrics bool
	MetricsRetentionTime     time.Duration
	EnableAuditTrail         bool
	AuditRetentionTime       time.Duration

	// Integration options
	ObservabilityHooks *ConflictResolutionObservabilityHooks
	Logger             interface{} // Opaque logger type to avoid dependencies
}

// ConflictResolutionObservabilityHooks provides integration with observability systems
type ConflictResolutionObservabilityHooks struct {
	// State transition hooks
	OnStateTransition       func(from, to ConflictResolutionState, metadata map[string]interface{})
	OnStateTransitionFailed func(from, to ConflictResolutionState, err error)

	// Workflow lifecycle hooks
	OnWorkflowStarted   func(conflictID string, conflict types.Conflict)
	OnWorkflowCompleted func(conflictID string, auditTrail *ConflictAuditTrail)
	OnWorkflowFailed    func(conflictID string, err error)

	// Rule evaluation hooks (extends existing hooks)
	OnRuleEvaluationStarted   func(conflictID, ruleName string)
	OnRuleEvaluationCompleted func(conflictID, ruleName string, matched bool, duration time.Duration)
	OnRuleEvaluationFailed    func(conflictID, ruleName string, err error)

	// Performance and metrics hooks
	OnMetricsRecorded      func(metrics *ResolverPerformanceMetrics)
	OnPerformanceThreshold func(metric string, value float64, threshold float64)
}

// ResolverPerformanceMetrics tracks performance data across all conflict resolutions
type ResolverPerformanceMetrics struct {
	// Overall statistics
	TotalConflictsResolved int64         `json:"total_conflicts_resolved"`
	TotalResolutionTime    time.Duration `json:"total_resolution_time"`
	AverageResolutionTime  time.Duration `json:"average_resolution_time"`
	FastestResolutionTime  time.Duration `json:"fastest_resolution_time"`
	SlowestResolutionTime  time.Duration `json:"slowest_resolution_time"`

	// Resolution outcomes
	AutoResolvedCount int64 `json:"auto_resolved_count"`
	ManualReviewCount int64 `json:"manual_review_count"`
	EscalatedCount    int64 `json:"escalated_count"`
	FailedCount       int64 `json:"failed_count"`

	// Rule performance
	RuleEvaluationCount       int64         `json:"rule_evaluation_count"`
	RuleMatchCount            int64         `json:"rule_match_count"`
	RuleFailureCount          int64         `json:"rule_failure_count"`
	AverageRuleEvaluationTime time.Duration `json:"average_rule_evaluation_time"`

	// State machine performance
	StateTransitionCount       int64         `json:"state_transition_count"`
	AverageStateTransitionTime time.Duration `json:"average_state_transition_time"`
	StateTransitionFailures    int64         `json:"state_transition_failures"`

	// Resource usage
	PeakConcurrentWorkflows int   `json:"peak_concurrent_workflows"`
	CurrentActiveWorkflows  int   `json:"current_active_workflows"`
	MemoryUsageBytes        int64 `json:"memory_usage_bytes"`

	// Time-based metrics
	LastResetTime          time.Time `json:"last_reset_time"`
	MetricsCollectionStart time.Time `json:"metrics_collection_start"`
}

// DefaultStatefulResolverOptions returns sensible defaults for stateful resolver configuration
func DefaultStatefulResolverOptions() *StatefulResolverOptions {
	return &StatefulResolverOptions{
		EnableStateMachine:       true,
		MaxStateHistorySize:      100,
		StateTransitionTimeout:   5 * time.Second,
		EnableWorkflowTracking:   true,
		MaxConcurrentWorkflows:   1000,
		WorkflowRetentionTime:    24 * time.Hour,
		WorkflowOptions:          DefaultWorkflowOptions(),
		EnablePerformanceMetrics: true,
		MetricsRetentionTime:     7 * 24 * time.Hour, // 7 days
		EnableAuditTrail:         true,
		AuditRetentionTime:       30 * 24 * time.Hour, // 30 days
	}
}

// ResolutionContext provides enhanced context for stateful conflict resolution
type ResolutionContext struct {
	context.Context

	// State machine context
	StateMachine *ConflictResolutionStateMachine
	CurrentState ConflictResolutionState

	// Workflow context
	Workflow   *ConflictWorkflow
	WorkflowID string

	// Performance tracking
	StartTime          time.Time
	PerformanceTracker *PerformanceTracker

	// Observability context
	TraceID  string
	SpanID   string
	Metadata map[string]interface{}
}

// PerformanceTracker tracks performance metrics during resolution
type PerformanceTracker struct {
	startTime      time.Time
	lastCheckpoint time.Time
	checkpoints    map[string]time.Time
	durations      map[string]time.Duration
	counters       map[string]int64
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker() *PerformanceTracker {
	now := time.Now()
	return &PerformanceTracker{
		startTime:      now,
		lastCheckpoint: now,
		checkpoints:    make(map[string]time.Time),
		durations:      make(map[string]time.Duration),
		counters:       make(map[string]int64),
	}
}

// Checkpoint records a performance checkpoint
func (pt *PerformanceTracker) Checkpoint(name string) time.Duration {
	now := time.Now()
	duration := now.Sub(pt.lastCheckpoint)

	pt.checkpoints[name] = now
	pt.durations[name] = duration
	pt.lastCheckpoint = now

	return duration
}

// Increment increments a counter
func (pt *PerformanceTracker) Increment(name string) {
	pt.counters[name]++
}

// GetDuration returns the duration for a specific checkpoint
func (pt *PerformanceTracker) GetDuration(name string) time.Duration {
	return pt.durations[name]
}

// GetCounter returns the value of a counter
func (pt *PerformanceTracker) GetCounter(name string) int64 {
	return pt.counters[name]
}

// GetTotalDuration returns the total duration since tracking started
func (pt *PerformanceTracker) GetTotalDuration() time.Duration {
	return time.Since(pt.startTime)
}

// GetSummary returns a summary of all tracked metrics
func (pt *PerformanceTracker) GetSummary() *PerformanceTrackerSummary {
	return &PerformanceTrackerSummary{
		StartTime:     pt.startTime,
		TotalDuration: pt.GetTotalDuration(),
		Checkpoints:   copyTimeMap(pt.checkpoints),
		Durations:     copyDurationMap(pt.durations),
		Counters:      copyCounterMap(pt.counters),
	}
}

// PerformanceTrackerSummary provides a snapshot of performance tracking data
type PerformanceTrackerSummary struct {
	StartTime     time.Time                `json:"start_time"`
	TotalDuration time.Duration            `json:"total_duration"`
	Checkpoints   map[string]time.Time     `json:"checkpoints"`
	Durations     map[string]time.Duration `json:"durations"`
	Counters      map[string]int64         `json:"counters"`
}

// ConflictResolutionEvent represents events that can occur during conflict resolution
type ConflictResolutionEvent struct {
	Type       ConflictResolutionEventType `json:"type"`
	Timestamp  time.Time                   `json:"timestamp"`
	ConflictID string                      `json:"conflict_id"`
	WorkflowID string                      `json:"workflow_id,omitempty"`
	StateFrom  ConflictResolutionState     `json:"state_from,omitempty"`
	StateTo    ConflictResolutionState     `json:"state_to,omitempty"`
	RuleName   string                      `json:"rule_name,omitempty"`
	Decision   string                      `json:"decision,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Duration   time.Duration               `json:"duration,omitempty"`
	Metadata   map[string]interface{}      `json:"metadata,omitempty"`
}

// ConflictResolutionEventType represents the different types of events during resolution
type ConflictResolutionEventType string

const (
	EventTypeConflictReceived      ConflictResolutionEventType = "conflict_received"
	EventTypeAnalysisStarted       ConflictResolutionEventType = "analysis_started"
	EventTypeAnalysisCompleted     ConflictResolutionEventType = "analysis_completed"
	EventTypeRuleEvaluationStarted ConflictResolutionEventType = "rule_evaluation_started"
	EventTypeRuleMatched           ConflictResolutionEventType = "rule_matched"
	EventTypeRuleSkipped           ConflictResolutionEventType = "rule_skipped"
	EventTypeRuleEvaluationFailed  ConflictResolutionEventType = "rule_evaluation_failed"
	EventTypeResolutionCompleted   ConflictResolutionEventType = "resolution_completed"
	EventTypeResolutionFailed      ConflictResolutionEventType = "resolution_failed"
	EventTypeManualReviewRequired  ConflictResolutionEventType = "manual_review_required"
	EventTypeConflictEscalated     ConflictResolutionEventType = "conflict_escalated"
	EventTypeStateTransition       ConflictResolutionEventType = "state_transition"
	EventTypeWorkflowStarted       ConflictResolutionEventType = "workflow_started"
	EventTypeWorkflowCompleted     ConflictResolutionEventType = "workflow_completed"
)

// Helper functions for copying maps
func copyTimeMap(src map[string]time.Time) map[string]time.Time {
	dst := make(map[string]time.Time)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyDurationMap(src map[string]time.Duration) map[string]time.Duration {
	dst := make(map[string]time.Duration)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyCounterMap(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
