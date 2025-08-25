package synckit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// Test helper types and utilities

// mockStatefulResolver is a test double for testing StatefulDynamicResolver
type mockStatefulResolver struct {
	name string
	res  ResolvedConflict
	err  error
	calls int
}

func (m *mockStatefulResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	m.calls++
	if m.err != nil {
		return ResolvedConflict{}, m.err
	}
	return m.res, nil
}

// slowResolver is a mock resolver that takes time to resolve
type slowResolver struct {
	name  string
	res   ResolvedConflict
	err   error
	calls int
	delay time.Duration
}

func (s *slowResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	s.calls++
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if s.err != nil {
		return ResolvedConflict{}, s.err
	}
	return s.res, nil
}

// testObservabilityHooks captures hook calls for testing
type testObservabilityHooks struct {
	mu sync.Mutex

	stateTransitions          []stateTransitionCall
	stateTransitionFailures   []stateTransitionFailureCall
	workflowsStarted         []workflowStartedCall
	workflowsCompleted       []workflowCompletedCall
	workflowsFailed          []workflowFailedCall
	ruleEvaluationsStarted   []ruleEvaluationCall
	ruleEvaluationsCompleted []ruleEvaluationCompletedCall
	ruleEvaluationsFailed    []ruleEvaluationFailedCall
	metricsRecorded          []metricsRecordedCall
}

type stateTransitionCall struct {
	from     statemachine.ConflictResolutionState
	to       statemachine.ConflictResolutionState
	metadata map[string]interface{}
}

type stateTransitionFailureCall struct {
	from statemachine.ConflictResolutionState
	to   statemachine.ConflictResolutionState
	err  error
}

type workflowStartedCall struct {
	conflictID string
	conflict   Conflict
}

type workflowCompletedCall struct {
	conflictID  string
	auditTrail  *statemachine.ConflictAuditTrail
}

type workflowFailedCall struct {
	conflictID string
	err        error
}

type ruleEvaluationCall struct {
	conflictID string
	ruleName   string
}

type ruleEvaluationCompletedCall struct {
	conflictID string
	ruleName   string
	matched    bool
	duration   time.Duration
}

type ruleEvaluationFailedCall struct {
	conflictID string
	ruleName   string
	err        error
}

type metricsRecordedCall struct {
	metrics *statemachine.ResolverPerformanceMetrics
}

func newTestObservabilityHooks() *testObservabilityHooks {
	hooks := &testObservabilityHooks{
		stateTransitions:          make([]stateTransitionCall, 0),
		stateTransitionFailures:   make([]stateTransitionFailureCall, 0),
		workflowsStarted:         make([]workflowStartedCall, 0),
		workflowsCompleted:       make([]workflowCompletedCall, 0),
		workflowsFailed:          make([]workflowFailedCall, 0),
		ruleEvaluationsStarted:   make([]ruleEvaluationCall, 0),
		ruleEvaluationsCompleted: make([]ruleEvaluationCompletedCall, 0),
		ruleEvaluationsFailed:    make([]ruleEvaluationFailedCall, 0),
		metricsRecorded:          make([]metricsRecordedCall, 0),
	}

	return hooks
}

func (th *testObservabilityHooks) toConflictResolutionObservabilityHooks() *statemachine.ConflictResolutionObservabilityHooks {
	return &statemachine.ConflictResolutionObservabilityHooks{
		OnStateTransition: func(from, to statemachine.ConflictResolutionState, metadata map[string]interface{}) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.stateTransitions = append(th.stateTransitions, stateTransitionCall{from: from, to: to, metadata: metadata})
		},
		OnStateTransitionFailed: func(from, to statemachine.ConflictResolutionState, err error) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.stateTransitionFailures = append(th.stateTransitionFailures, stateTransitionFailureCall{from: from, to: to, err: err})
		},
		OnWorkflowStarted: func(conflictID string, conflict Conflict) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.workflowsStarted = append(th.workflowsStarted, workflowStartedCall{conflictID: conflictID, conflict: conflict})
		},
		OnWorkflowCompleted: func(conflictID string, auditTrail *statemachine.ConflictAuditTrail) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.workflowsCompleted = append(th.workflowsCompleted, workflowCompletedCall{conflictID: conflictID, auditTrail: auditTrail})
		},
		OnWorkflowFailed: func(conflictID string, err error) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.workflowsFailed = append(th.workflowsFailed, workflowFailedCall{conflictID: conflictID, err: err})
		},
		OnRuleEvaluationStarted: func(conflictID, ruleName string) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.ruleEvaluationsStarted = append(th.ruleEvaluationsStarted, ruleEvaluationCall{conflictID: conflictID, ruleName: ruleName})
		},
		OnRuleEvaluationCompleted: func(conflictID, ruleName string, matched bool, duration time.Duration) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.ruleEvaluationsCompleted = append(th.ruleEvaluationsCompleted, ruleEvaluationCompletedCall{conflictID: conflictID, ruleName: ruleName, matched: matched, duration: duration})
		},
		OnRuleEvaluationFailed: func(conflictID, ruleName string, err error) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.ruleEvaluationsFailed = append(th.ruleEvaluationsFailed, ruleEvaluationFailedCall{conflictID: conflictID, ruleName: ruleName, err: err})
		},
		OnMetricsRecorded: func(metrics *statemachine.ResolverPerformanceMetrics) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.metricsRecorded = append(th.metricsRecorded, metricsRecordedCall{metrics: metrics})
		},
	}
}

// Test cases

func TestNewStatefulDynamicResolver_ValidInput(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "test"}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	options := statemachine.DefaultStatefulResolverOptions()
	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if statefulResolver == nil {
		t.Fatal("Expected stateful resolver to be created")
	}
	if !statefulResolver.IsStateMachineEnabled() {
		t.Error("Expected state machine to be enabled by default")
	}
}

func TestNewStatefulDynamicResolver_NilBaseResolver(t *testing.T) {
	options := statemachine.DefaultStatefulResolverOptions()
	_, err := NewStatefulDynamicResolver(nil, options)

	if err == nil {
		t.Fatal("Expected error for nil base resolver")
	}
	if err.Error() != "base resolver cannot be nil" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestNewStatefulDynamicResolver_NilOptions(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "test"}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, nil)

	if err != nil {
		t.Fatalf("Expected no error with nil options, got: %v", err)
	}
	if !statefulResolver.IsStateMachineEnabled() {
		t.Error("Expected default options to enable state machine")
	}
}

func TestStatefulDynamicResolver_BasicResolution(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "resolved", Reasons: []string{"test"}}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	options := statemachine.DefaultStatefulResolverOptions()
	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
		Metadata:    map[string]any{"test": true},
	}

	ctx := context.Background()
	result, err := statefulResolver.Resolve(ctx, conflict)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Decision != "resolved" {
		t.Errorf("Expected decision 'resolved', got: %s", result.Decision)
	}
	if baseResolver.calls != 1 {
		t.Errorf("Expected base resolver to be called once, got: %d", baseResolver.calls)
	}
}

func TestStatefulDynamicResolver_StateTransitions(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "resolved"}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	testHooks := newTestObservabilityHooks()
	options := &statemachine.StatefulResolverOptions{
		EnableStateMachine:       true,
		EnableWorkflowTracking:   true,
		EnablePerformanceMetrics: true,
		EnableAuditTrail:         true,
		ObservabilityHooks:      testHooks.toConflictResolutionObservabilityHooks(),
		WorkflowOptions:         statemachine.DefaultWorkflowOptions(),
		MaxStateHistorySize:     100,
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
	}

	ctx := context.Background()
	_, err = statefulResolver.Resolve(ctx, conflict)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Wait a bit for async observer notifications to complete
	time.Sleep(10 * time.Millisecond)

	// Verify state transitions were recorded
	func() {
		testHooks.mu.Lock()
		defer testHooks.mu.Unlock()
		if len(testHooks.stateTransitions) == 0 {
			t.Error("Expected state transitions to be recorded")
		}
	}()

	// Verify workflows were started and completed
	func() {
		testHooks.mu.Lock()
		defer testHooks.mu.Unlock()
		if len(testHooks.workflowsStarted) != 1 {
			t.Errorf("Expected 1 workflow to be started, got: %d", len(testHooks.workflowsStarted))
		}
		if len(testHooks.workflowsCompleted) != 1 {
			t.Errorf("Expected 1 workflow to be completed, got: %d", len(testHooks.workflowsCompleted))
		}
	}()

	// Verify metrics were recorded
	func() {
		testHooks.mu.Lock()
		defer testHooks.mu.Unlock()
		if len(testHooks.metricsRecorded) == 0 {
			t.Error("Expected performance metrics to be recorded")
		}
	}()
}

func TestStatefulDynamicResolver_RuleEvaluation(t *testing.T) {
	matchingResolver := &mockStatefulResolver{name: "matching", res: ResolvedConflict{Decision: "matched"}}
	fallbackResolver := &mockStatefulResolver{name: "fallback", res: ResolvedConflict{Decision: "fallback"}}

	dynamicResolver, err := NewDynamicResolver(
		WithRule("test_rule", EventTypeIs("TestEvent"), matchingResolver),
		WithFallback(fallbackResolver),
	)
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	testHooks := newTestObservabilityHooks()
	options := &statemachine.StatefulResolverOptions{
		EnableStateMachine:       true,
		EnableWorkflowTracking:   true,
		EnablePerformanceMetrics: true,
		ObservabilityHooks:      testHooks.toConflictResolutionObservabilityHooks(),
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
	}

	ctx := context.Background()
	result, err := statefulResolver.Resolve(ctx, conflict)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Decision != "matched" {
		t.Errorf("Expected decision 'matched', got: %s", result.Decision)
	}

	// Verify rule evaluation was tracked
	if len(testHooks.ruleEvaluationsStarted) != 1 {
		t.Errorf("Expected 1 rule evaluation to be started, got: %d", len(testHooks.ruleEvaluationsStarted))
	}
	if len(testHooks.ruleEvaluationsCompleted) != 1 {
		t.Errorf("Expected 1 rule evaluation to be completed, got: %d", len(testHooks.ruleEvaluationsCompleted))
	}

	// Verify the correct rule was evaluated
	if testHooks.ruleEvaluationsStarted[0].ruleName != "test_rule" {
		t.Errorf("Expected rule 'test_rule' to be evaluated, got: %s", testHooks.ruleEvaluationsStarted[0].ruleName)
	}
	if !testHooks.ruleEvaluationsCompleted[0].matched {
		t.Error("Expected rule to match")
	}

	// Verify fallback was not called
	if fallbackResolver.calls != 0 {
		t.Errorf("Expected fallback resolver not to be called, got: %d calls", fallbackResolver.calls)
	}
	if matchingResolver.calls != 1 {
		t.Errorf("Expected matching resolver to be called once, got: %d calls", matchingResolver.calls)
	}
}

func TestStatefulDynamicResolver_RuleFailure(t *testing.T) {
	failingResolver := &mockStatefulResolver{name: "failing", err: errors.New("resolver error")}
	fallbackResolver := &mockStatefulResolver{name: "fallback", res: ResolvedConflict{Decision: "fallback"}}

	dynamicResolver, err := NewDynamicResolver(
		WithRule("failing_rule", EventTypeIs("TestEvent"), failingResolver),
		WithFallback(fallbackResolver),
	)
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	testHooks := newTestObservabilityHooks()
	options := &statemachine.StatefulResolverOptions{
		EnableStateMachine:       true,
		EnableWorkflowTracking:   true,
		EnablePerformanceMetrics: true,
		ObservabilityHooks:      testHooks.toConflictResolutionObservabilityHooks(),
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
	}

	ctx := context.Background()
	_, err = statefulResolver.Resolve(ctx, conflict)

	// Rule should fail, but we should get the error
	if err == nil {
		t.Fatal("Expected error from failing rule")
	}
	if err.Error() != "resolver error" {
		t.Errorf("Expected 'resolver error', got: %v", err)
	}

	// Verify rule failure was tracked
	if len(testHooks.ruleEvaluationsFailed) != 1 {
		t.Errorf("Expected 1 rule evaluation failure, got: %d", len(testHooks.ruleEvaluationsFailed))
	}
	if testHooks.ruleEvaluationsFailed[0].ruleName != "failing_rule" {
		t.Errorf("Expected 'failing_rule' to fail, got: %s", testHooks.ruleEvaluationsFailed[0].ruleName)
	}
}

func TestStatefulDynamicResolver_FallbackUsage(t *testing.T) {
	fallbackResolver := &mockStatefulResolver{name: "fallback", res: ResolvedConflict{Decision: "fallback_used"}}

	dynamicResolver, err := NewDynamicResolver(
		WithRule("non_matching_rule", EventTypeIs("NonMatchingEvent"), &mockStatefulResolver{}),
		WithFallback(fallbackResolver),
	)
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	options := statemachine.DefaultStatefulResolverOptions()
	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent", // Won't match the rule
		AggregateID: "test-123",
	}

	ctx := context.Background()
	result, err := statefulResolver.Resolve(ctx, conflict)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Decision != "fallback_used" {
		t.Errorf("Expected decision 'fallback_used', got: %s", result.Decision)
	}
	if fallbackResolver.calls != 1 {
		t.Errorf("Expected fallback resolver to be called once, got: %d", fallbackResolver.calls)
	}
}

func TestStatefulDynamicResolver_PerformanceMetrics(t *testing.T) {
	// Use a slow resolver to guarantee measurable resolution time
	baseResolver := &slowResolver{
		name:  "base",
		res:   ResolvedConflict{Decision: "resolved"},
		delay: 5 * time.Millisecond, // Guaranteed measurable delay
	}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	options := &statemachine.StatefulResolverOptions{
		EnableStateMachine:       true,
		EnablePerformanceMetrics: true,
		EnableWorkflowTracking:   false, // Disable for simpler test
		EnableAuditTrail:         false,
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
	}

	ctx := context.Background()
	
	// Resolve multiple conflicts to generate metrics
	for i := 0; i < 3; i++ {
		_, err = statefulResolver.Resolve(ctx, conflict)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
	}

	// Check performance metrics
	metrics := statefulResolver.GetPerformanceMetrics()
	if metrics == nil {
		t.Fatal("Expected performance metrics to be available")
	}
	if metrics.TotalConflictsResolved != 3 {
		t.Errorf("Expected 3 conflicts resolved, got: %d", metrics.TotalConflictsResolved)
	}
	if metrics.AutoResolvedCount != 3 {
		t.Errorf("Expected 3 auto-resolved conflicts, got: %d", metrics.AutoResolvedCount)
	}
	if metrics.AverageResolutionTime == 0 {
		t.Error("Expected non-zero average resolution time")
	}
}

func TestStatefulDynamicResolver_WorkflowTracking(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "resolved", Reasons: []string{"test reason"}}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	options := &statemachine.StatefulResolverOptions{
		EnableStateMachine:     false, // Disable for simpler test
		EnableWorkflowTracking: true,
		EnableAuditTrail:       true,
		WorkflowOptions:        statemachine.DefaultWorkflowOptions(),
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
		ChangedFields: []string{"field1", "field2"},
	}

	ctx := context.Background()
	result, err := statefulResolver.Resolve(ctx, conflict)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Decision != "resolved" {
		t.Errorf("Expected decision 'resolved', got: %s", result.Decision)
	}

	// Check workflow manager
	workflowManager := statefulResolver.GetWorkflowManager()
	if workflowManager == nil {
		t.Fatal("Expected workflow manager to be available")
	}

	// Check active workflows (should be empty after completion)
	activeWorkflows := statefulResolver.GetActiveWorkflows()
	if len(activeWorkflows) != 0 {
		t.Errorf("Expected 0 active workflows after completion, got: %d", len(activeWorkflows))
	}
}

func TestStatefulDynamicResolver_StateChanges(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "resolved"}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	options := &statemachine.StatefulResolverOptions{
		EnableStateMachine:     true,
		EnableWorkflowTracking: false, // Disable for simpler test
		MaxStateHistorySize:    50,
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, options)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	// Check initial state
	initialState := statefulResolver.GetCurrentState()
	if initialState != statemachine.DCRIdle {
		t.Errorf("Expected initial state to be DCRIdle, got: %s", initialState.String())
	}

	// Subscribe to state changes
	var (
		stateChanges []statemachine.ConflictResolutionState
		stateMu      sync.Mutex
	)
	observer := &testStateObserver{
		onTransition: func(from, to statemachine.ConflictResolutionState, metadata map[string]interface{}) {
			stateMu.Lock()
			defer stateMu.Unlock()
			stateChanges = append(stateChanges, to)
		},
	}
	statefulResolver.SubscribeToStateChanges(observer)

	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
	}

	ctx := context.Background()
	_, err = statefulResolver.Resolve(ctx, conflict)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify state transitions were recorded
	stateHistory := statefulResolver.GetStateHistory()
	if len(stateHistory) == 0 {
		t.Error("Expected state transitions to be recorded in history")
	}

	// Verify state should be back to idle after resolution
	finalState := statefulResolver.GetCurrentState()
	if finalState != statemachine.DCRIdle {
		t.Errorf("Expected final state to be DCRIdle, got: %s", finalState.String())
	}
}

func TestStatefulDynamicResolver_Configuration(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "resolved"}}
	dynamicResolver, err := NewDynamicResolver(WithFallback(baseResolver))
	if err != nil {
		t.Fatalf("Failed to create base DynamicResolver: %v", err)
	}

	initialOptions := &statemachine.StatefulResolverOptions{
		EnableStateMachine: false,
	}

	statefulResolver, err := NewStatefulDynamicResolver(dynamicResolver, initialOptions)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver: %v", err)
	}

	// Check initial state
	if statefulResolver.IsStateMachineEnabled() {
		t.Error("Expected state machine to be disabled initially")
	}

	// Reconfigure
	newOptions := &statemachine.StatefulResolverOptions{
		EnableStateMachine: true,
	}

	err = statefulResolver.Configure(newOptions)
	if err != nil {
		t.Fatalf("Expected no error configuring, got: %v", err)
	}

	// Test nil options
	err = statefulResolver.Configure(nil)
	if err == nil {
		t.Error("Expected error when configuring with nil options")
	}
}

func TestNewStatefulDynamicResolverFromOptions(t *testing.T) {
	baseResolver := &mockStatefulResolver{name: "base", res: ResolvedConflict{Decision: "resolved"}}
	
	dynamicOpts := []Option{
		WithFallback(baseResolver),
		WithRule("test_rule", AlwaysMatch(), baseResolver),
	}
	
	statefulOpts := statemachine.DefaultStatefulResolverOptions()
	
	statefulResolver, err := NewStatefulDynamicResolverFromOptions(dynamicOpts, statefulOpts)
	if err != nil {
		t.Fatalf("Failed to create stateful resolver from options: %v", err)
	}
	
	if !statefulResolver.IsStateMachineEnabled() {
		t.Error("Expected state machine to be enabled")
	}
	
	// Test that it works
	conflict := Conflict{
		EventType:   "TestEvent",
		AggregateID: "test-123",
	}
	
	ctx := context.Background()
	result, err := statefulResolver.Resolve(ctx, conflict)
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Decision != "resolved" {
		t.Errorf("Expected decision 'resolved', got: %s", result.Decision)
	}
}

// Helper types for testing

type testStateObserver struct {
	onTransition       func(from, to statemachine.ConflictResolutionState, metadata map[string]interface{})
	onTransitionFailed func(from, to statemachine.ConflictResolutionState, err error, metadata map[string]interface{})
}

func (tso *testStateObserver) OnTransition(transition statemachine.StateTransition[statemachine.ConflictResolutionState]) {
	if tso.onTransition != nil {
		tso.onTransition(transition.From, transition.To, transition.Metadata)
	}
}

func (tso *testStateObserver) OnTransitionFailed(from, to statemachine.ConflictResolutionState, err error, metadata map[string]interface{}) {
	if tso.onTransitionFailed != nil {
		tso.onTransitionFailed(from, to, err, metadata)
	}
}
