package synckit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// StatefulDynamicResolver extends DynamicResolver with state machine capabilities
// providing comprehensive workflow tracking, audit trails, and enhanced observability
// while maintaining full backward compatibility with existing code.
type StatefulDynamicResolver struct {
	// Embed the existing DynamicResolver for backward compatibility
	*DynamicResolver
	
	// State machine and workflow components
	stateMachine    *statemachine.ConflictResolutionStateMachine
	workflowManager *statemachine.WorkflowManager
	options         *statemachine.StatefulResolverOptions
	
	// Performance and metrics tracking
	performanceMetrics *statemachine.ResolverPerformanceMetrics
	metricsLock        sync.RWMutex
	
	// Observability integration
	stateObservers []statemachine.StateObserver[statemachine.ConflictResolutionState]
	observerLock   sync.RWMutex
	
	// Configuration and lifecycle
	enabled bool
	mu      sync.RWMutex
}

// Ensure StatefulDynamicResolver implements the required interfaces
var _ ConflictResolver = (*StatefulDynamicResolver)(nil)
var _ statemachine.StatefulConflictResolver = (*StatefulDynamicResolver)(nil)

// NewStatefulDynamicResolver creates a new stateful dynamic resolver with the given options.
// It wraps an existing DynamicResolver to provide state machine capabilities while maintaining
// complete backward compatibility.
func NewStatefulDynamicResolver(baseResolver *DynamicResolver, options *statemachine.StatefulResolverOptions) (*StatefulDynamicResolver, error) {
	if baseResolver == nil {
		return nil, errors.New("base resolver cannot be nil")
	}
	
	if options == nil {
		options = statemachine.DefaultStatefulResolverOptions()
	}
	
	// Create state machine if enabled
	var stateMachine *statemachine.ConflictResolutionStateMachine
	if options.EnableStateMachine {
		smConfig := &statemachine.StateMachineConfig[statemachine.ConflictResolutionState]{
			InitialState:    statemachine.DCRIdle,
			MaxHistorySize:  options.MaxStateHistorySize,
			EnableMetrics:   true,
			Name:           "StatefulDynamicResolver",
		}
		stateMachine = statemachine.NewConflictResolutionStateMachine(smConfig)
	}
	
	// Create workflow manager if enabled
	var workflowManager *statemachine.WorkflowManager
	if options.EnableWorkflowTracking {
		workflowManager = statemachine.NewWorkflowManager()
	}
	
	// Initialize performance metrics
	var performanceMetrics *statemachine.ResolverPerformanceMetrics
	if options.EnablePerformanceMetrics {
		performanceMetrics = &statemachine.ResolverPerformanceMetrics{
			MetricsCollectionStart: time.Now(),
			LastResetTime:          time.Now(),
		}
	}
	
	resolver := &StatefulDynamicResolver{
		DynamicResolver:    baseResolver,
		stateMachine:       stateMachine,
		workflowManager:    workflowManager,
		options:           options,
		performanceMetrics: performanceMetrics,
		stateObservers:    make([]statemachine.StateObserver[statemachine.ConflictResolutionState], 0),
		enabled:           true,
	}
	
	// Configure state machine observers if we have observability hooks
	if options.ObservabilityHooks != nil && stateMachine != nil {
		observer := &resolverStateObserver{
			resolver: resolver,
			hooks:    options.ObservabilityHooks,
		}
		resolver.SubscribeToStateChanges(observer)
	}
	
	return resolver, nil
}

// Resolve implements the ConflictResolver interface with enhanced state machine capabilities.
// This method provides full backward compatibility while adding workflow tracking and audit trails
// when state machine features are enabled.
func (sdr *StatefulDynamicResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	// Start performance tracking if enabled
	var performanceTracker *statemachine.PerformanceTracker
	if sdr.options.EnablePerformanceMetrics {
		performanceTracker = statemachine.NewPerformanceTracker()
		performanceTracker.Checkpoint("resolve_started")
	}
	
	// Create workflow if tracking is enabled
	var workflow *statemachine.ConflictWorkflow
	if sdr.options.EnableWorkflowTracking && sdr.workflowManager != nil {
		workflow = sdr.workflowManager.StartWorkflow(c, sdr.stateMachine, sdr.options.WorkflowOptions)
		
		// Trigger observability hook
		if sdr.options.ObservabilityHooks != nil && sdr.options.ObservabilityHooks.OnWorkflowStarted != nil {
			sdr.options.ObservabilityHooks.OnWorkflowStarted(workflow.GetCurrentStatus().ConflictID, c)
		}
	}
	
	// Initialize state machine for this resolution if enabled
	if sdr.stateMachine != nil {
		conflictID := generateConflictID(c)
		if err := sdr.stateMachine.StartResolution(conflictID); err != nil {
			return ResolvedConflict{}, err
		}
		
		// Start analysis phase
		if workflow != nil {
			workflow.StartAnalysis()
		}
		
		if performanceTracker != nil {
			performanceTracker.Checkpoint("analysis_started")
		}
	}
	
	// Perform the actual resolution with enhanced tracking
	result, err := sdr.resolveWithTracking(ctx, c, workflow, performanceTracker)
	
	// Complete workflow and state machine
	if sdr.stateMachine != nil {
		if err != nil {
			sdr.stateMachine.FailResolution()
		} else {
			switch result.Decision {
			case "manual_review":
				sdr.stateMachine.RequireManualReview()
			case "escalated":
				sdr.stateMachine.EscalateConflict()
			default:
				sdr.stateMachine.CompleteResolution()
			}
		}
		
		// Reset state machine for next conflict
		defer func() {
			if resetErr := sdr.stateMachine.Reset(); resetErr != nil && sdr.options.Logger != nil {
				// Log reset error if logger is available
				// Note: Logger is opaque interface, so we can't log directly here
			}
		}()
	}
	
	// Complete workflow tracking
	if workflow != nil {
		if err == nil {
			workflow.CompleteResolution(result.Decision, result.Reasons, result.ResolvedEvents)
		}
		
		// Generate audit trail and trigger completion hook
		if auditTrail := sdr.workflowManager.CompleteWorkflow(workflow.GetCurrentStatus().ConflictID); auditTrail != nil {
			if sdr.options.ObservabilityHooks != nil && sdr.options.ObservabilityHooks.OnWorkflowCompleted != nil {
				sdr.options.ObservabilityHooks.OnWorkflowCompleted(auditTrail.ConflictID, auditTrail)
			}
		}
	}
	
	// Update performance metrics
	if performanceTracker != nil && sdr.performanceMetrics != nil {
		sdr.updatePerformanceMetrics(performanceTracker, result, err)
	}
	
	return result, err
}

// resolveWithTracking performs the actual conflict resolution with enhanced tracking
func (sdr *StatefulDynamicResolver) resolveWithTracking(ctx context.Context, c Conflict, workflow *statemachine.ConflictWorkflow, tracker *statemachine.PerformanceTracker) (ResolvedConflict, error) {
	// Begin rule application phase
	if sdr.stateMachine != nil {
		if err := sdr.stateMachine.BeginRuleApplication(); err != nil {
			return ResolvedConflict{}, err
		}
	}
	
	if tracker != nil {
		tracker.Checkpoint("rules_started")
	}
	
	// Evaluate rules with enhanced tracking
	for _, r := range sdr.rules {
		ruleStart := time.Now()
		
		// Trigger rule evaluation hook
		if sdr.options.ObservabilityHooks != nil && sdr.options.ObservabilityHooks.OnRuleEvaluationStarted != nil {
			conflictID := ""
			if workflow != nil {
				conflictID = workflow.GetCurrentStatus().ConflictID
			}
			sdr.options.ObservabilityHooks.OnRuleEvaluationStarted(conflictID, r.Name)
		}
		
		// Track rule evaluation in state machine
		if sdr.stateMachine != nil {
			sdr.stateMachine.RecordRuleEvaluation(r.Name)
		}
		
		// Evaluate the rule
		matched := r.Matcher != nil && r.Matcher(c)
		ruleDuration := time.Since(ruleStart)
		
		// Record rule evaluation in workflow
		if workflow != nil {
			workflow.RecordRuleEvaluation(r.Name, matched, ruleDuration, nil, nil)
		}
		
		// Trigger rule evaluation completed hook
		if sdr.options.ObservabilityHooks != nil && sdr.options.ObservabilityHooks.OnRuleEvaluationCompleted != nil {
			conflictID := ""
			if workflow != nil {
				conflictID = workflow.GetCurrentStatus().ConflictID
			}
			sdr.options.ObservabilityHooks.OnRuleEvaluationCompleted(conflictID, r.Name, matched, ruleDuration)
		}
		
		if matched {
			// Record rule match in state machine
			if sdr.stateMachine != nil {
				sdr.stateMachine.RecordRuleMatched(r.Name)
			}
			
			// Call original hook for backward compatibility
			if sdr.hooks.OnRuleMatched != nil {
				sdr.hooks.OnRuleMatched(c, r)
			}
			
			// Resolve using the matched rule
			res, err := r.Resolver.Resolve(ctx, c)
			if err != nil {
				// Record rule failure
				if sdr.stateMachine != nil {
					sdr.stateMachine.RecordRuleFailed(r.Name)
				}
				
				if workflow != nil {
					workflow.RecordRuleEvaluation(r.Name, true, ruleDuration, err, nil)
				}
				
				// Trigger rule evaluation failed hook
				if sdr.options.ObservabilityHooks != nil && sdr.options.ObservabilityHooks.OnRuleEvaluationFailed != nil {
					conflictID := ""
					if workflow != nil {
						conflictID = workflow.GetCurrentStatus().ConflictID
					}
					sdr.options.ObservabilityHooks.OnRuleEvaluationFailed(conflictID, r.Name, err)
				}
				
				// Call original error hook for backward compatibility
				if sdr.hooks.OnError != nil {
					sdr.hooks.OnError(c, err)
				}
				
				return ResolvedConflict{}, err
			}
			
			// Call original resolved hook for backward compatibility
			if sdr.hooks.OnResolved != nil {
				sdr.hooks.OnResolved(c, res)
			}
			
			if tracker != nil {
				tracker.Checkpoint("resolution_completed")
				tracker.Increment("rules_matched")
			}
			
			return res, nil
		}
		
		if tracker != nil {
			tracker.Increment("rules_evaluated")
		}
	}
	
	// No rule matched, try fallback
	if sdr.fallback == nil {
		err := errors.New("no rule matched and no fallback configured")
		
		// Call original error hook for backward compatibility
		if sdr.hooks.OnError != nil {
			sdr.hooks.OnError(c, err)
		}
		
		return ResolvedConflict{}, err
	}
	
	// Call original fallback hook for backward compatibility
	if sdr.hooks.OnFallback != nil {
		sdr.hooks.OnFallback(c)
	}
	
	// Execute fallback
	res, err := sdr.fallback.Resolve(ctx, c)
	if err != nil {
		// Call original error hook for backward compatibility
		if sdr.hooks.OnError != nil {
			sdr.hooks.OnError(c, err)
		}
		
		return ResolvedConflict{}, err
	}
	
	// Call original resolved hook for backward compatibility
	if sdr.hooks.OnResolved != nil {
		sdr.hooks.OnResolved(c, res)
	}
	
	if tracker != nil {
		tracker.Checkpoint("fallback_completed")
		tracker.Increment("fallback_used")
	}
	
	return res, nil
}

// updatePerformanceMetrics updates the resolver's performance metrics
func (sdr *StatefulDynamicResolver) updatePerformanceMetrics(tracker *statemachine.PerformanceTracker, result ResolvedConflict, err error) {
	sdr.metricsLock.Lock()
	defer sdr.metricsLock.Unlock()
	
	if sdr.performanceMetrics == nil {
		return
	}
	
	totalDuration := tracker.GetTotalDuration()
	
	// Update overall statistics
	sdr.performanceMetrics.TotalConflictsResolved++
	sdr.performanceMetrics.TotalResolutionTime += totalDuration
	
	// Calculate average resolution time correctly
	if sdr.performanceMetrics.TotalConflictsResolved > 0 {
		sdr.performanceMetrics.AverageResolutionTime = time.Duration(int64(sdr.performanceMetrics.TotalResolutionTime) / sdr.performanceMetrics.TotalConflictsResolved)
	}
	
	// Update min/max resolution times
	if sdr.performanceMetrics.FastestResolutionTime == 0 || totalDuration < sdr.performanceMetrics.FastestResolutionTime {
		sdr.performanceMetrics.FastestResolutionTime = totalDuration
	}
	if totalDuration > sdr.performanceMetrics.SlowestResolutionTime {
		sdr.performanceMetrics.SlowestResolutionTime = totalDuration
	}
	
	// Update resolution outcome counters
	if err != nil {
		sdr.performanceMetrics.FailedCount++
	} else {
		switch result.Decision {
		case "manual_review":
			sdr.performanceMetrics.ManualReviewCount++
		case "escalated":
			sdr.performanceMetrics.EscalatedCount++
		default:
			sdr.performanceMetrics.AutoResolvedCount++
		}
	}
	
	// Update rule performance metrics
	sdr.performanceMetrics.RuleEvaluationCount += tracker.GetCounter("rules_evaluated")
	sdr.performanceMetrics.RuleMatchCount += tracker.GetCounter("rules_matched")
	
	// Trigger metrics recorded hook
	if sdr.options.ObservabilityHooks != nil && sdr.options.ObservabilityHooks.OnMetricsRecorded != nil {
		sdr.options.ObservabilityHooks.OnMetricsRecorded(sdr.performanceMetrics)
	}
}

// StatefulConflictResolver interface implementations

func (sdr *StatefulDynamicResolver) GetCurrentState() statemachine.ConflictResolutionState {
	if sdr.stateMachine == nil {
		return statemachine.DCRIdle
	}
	return sdr.stateMachine.Current()
}

func (sdr *StatefulDynamicResolver) GetStateHistory() []statemachine.StateTransition[statemachine.ConflictResolutionState] {
	if sdr.stateMachine == nil {
		return nil
	}
	return sdr.stateMachine.History()
}

func (sdr *StatefulDynamicResolver) SubscribeToStateChanges(observer statemachine.StateObserver[statemachine.ConflictResolutionState]) {
	sdr.observerLock.Lock()
	defer sdr.observerLock.Unlock()
	
	sdr.stateObservers = append(sdr.stateObservers, observer)
	
	if sdr.stateMachine != nil {
		sdr.stateMachine.Subscribe(observer)
	}
}

func (sdr *StatefulDynamicResolver) GetWorkflowManager() *statemachine.WorkflowManager {
	return sdr.workflowManager
}

func (sdr *StatefulDynamicResolver) GetActiveWorkflows() []*statemachine.WorkflowStatus {
	if sdr.workflowManager == nil {
		return nil
	}
	return sdr.workflowManager.ListActiveWorkflows()
}

func (sdr *StatefulDynamicResolver) GetWorkflowByID(conflictID string) (*statemachine.ConflictWorkflow, bool) {
	if sdr.workflowManager == nil {
		return nil, false
	}
	return sdr.workflowManager.GetWorkflow(conflictID)
}

func (sdr *StatefulDynamicResolver) GetAuditTrail(conflictID string) (*statemachine.ConflictAuditTrail, error) {
	if sdr.workflowManager == nil {
		return nil, errors.New("workflow tracking is not enabled")
	}
	
	// Try to get active workflow first
	if workflow, exists := sdr.workflowManager.GetWorkflow(conflictID); exists {
		return workflow.GenerateAuditTrail(), nil
	}
	
	// If not found in active workflows, it might be completed
	// In a real implementation, you might want to store completed audit trails
	return nil, errors.New("audit trail not found for conflict ID: " + conflictID)
}

func (sdr *StatefulDynamicResolver) Configure(options *statemachine.StatefulResolverOptions) error {
	sdr.mu.Lock()
	defer sdr.mu.Unlock()
	
	if options == nil {
		return errors.New("options cannot be nil")
	}
	
	sdr.options = options
	return nil
}

func (sdr *StatefulDynamicResolver) IsStateMachineEnabled() bool {
	return sdr.stateMachine != nil && sdr.options.EnableStateMachine
}

func (sdr *StatefulDynamicResolver) GetPerformanceMetrics() *statemachine.ResolverPerformanceMetrics {
	sdr.metricsLock.RLock()
	defer sdr.metricsLock.RUnlock()
	
	if sdr.performanceMetrics == nil {
		return nil
	}
	
	// Return a copy to prevent external modification
	metricsCopy := *sdr.performanceMetrics
	if sdr.workflowManager != nil {
		metricsCopy.CurrentActiveWorkflows = len(sdr.workflowManager.ListActiveWorkflows())
	}
	
	return &metricsCopy
}

// resolverStateObserver integrates state machine transitions with observability hooks
type resolverStateObserver struct {
	resolver *StatefulDynamicResolver
	hooks    *statemachine.ConflictResolutionObservabilityHooks
}

func (rso *resolverStateObserver) OnTransition(transition statemachine.StateTransition[statemachine.ConflictResolutionState]) {
	if rso.hooks.OnStateTransition != nil {
		rso.hooks.OnStateTransition(transition.From, transition.To, transition.Metadata)
	}
}

func (rso *resolverStateObserver) OnTransitionFailed(from, to statemachine.ConflictResolutionState, err error, metadata map[string]interface{}) {
	if rso.hooks.OnStateTransitionFailed != nil {
		rso.hooks.OnStateTransitionFailed(from, to, err)
	}
}

// NewStatefulDynamicResolverFromOptions creates a StatefulDynamicResolver directly from DynamicResolver options.
// This is a convenience function that creates the base DynamicResolver and wraps it with state machine capabilities.
func NewStatefulDynamicResolverFromOptions(dynamicOpts []Option, statefulOpts *statemachine.StatefulResolverOptions) (*StatefulDynamicResolver, error) {
	baseResolver, err := NewDynamicResolver(dynamicOpts...)
	if err != nil {
		return nil, err
	}
	
	return NewStatefulDynamicResolver(baseResolver, statefulOpts)
}

// WithStateMachine is a convenience option for adding state machine capabilities to an existing DynamicResolver.
func WithStateMachine(options *statemachine.StatefulResolverOptions) Option {
	return optionFn(func(o *resolverOptions) {
		// This will be handled by the sync manager during construction
		// We store the options in a special metadata field
		if o.logger == nil {
			o.logger = map[string]interface{}{
				"stateful_options": options,
			}
		} else if loggerMap, ok := o.logger.(map[string]interface{}); ok {
			loggerMap["stateful_options"] = options
		}
	})
}

// Helper function to generate a conflict ID
func generateConflictID(c Conflict) string {
	return c.EventType + ":" + c.AggregateID + ":" + time.Now().Format(time.RFC3339Nano)
}
