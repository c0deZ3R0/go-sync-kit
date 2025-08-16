package synckit

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPhase6CompositePattern(t *testing.T) {
	t.Run("RuleGroup_BasicOperations", func(t *testing.T) {
		group := NewRuleGroup("test-group",
			WithDescription("Test group for unit tests"),
			WithEnabled(true),
		)
		
		// Test basic properties
		if group.Name != "test-group" {
			t.Errorf("Expected name 'test-group', got %s", group.Name)
		}
		if group.Description != "Test group for unit tests" {
			t.Errorf("Expected description, got %s", group.Description)
		}
		if !group.IsEnabled() {
			t.Errorf("Group should be enabled")
		}
		
		// Test adding rules
		rule := Rule{
			Name:     "test-rule",
			Matcher:  EventTypeIs("UserUpdated"),
			Resolver: &LastWriteWinsResolver{},
		}
		group.AddRule(rule)
		
		rules := group.GetAllRules()
		if len(rules) != 1 {
			t.Fatalf("Expected 1 rule, got %d", len(rules))
		}
		if rules[0].Name != "test-group.test-rule" {
			t.Errorf("Expected namespaced rule name 'test-group.test-rule', got %s", rules[0].Name)
		}
	})
	
	t.Run("RuleGroup_NestedHierarchy", func(t *testing.T) {
		// Create parent group
		parent := NewRuleGroup("parent")
		parent.AddRule(Rule{
			Name:     "parent-rule",
			Matcher:  EventTypeIs("ParentEvent"),
			Resolver: &LastWriteWinsResolver{},
		})
		
		// Create child group
		child := NewRuleGroup("child")
		child.AddRule(Rule{
			Name:     "child-rule",
			Matcher:  EventTypeIs("ChildEvent"),
			Resolver: &AdditiveMergeResolver{},
		})
		
		// Add child to parent
		parent.AddSubGroup(child)
		
		// Test hierarchy
		allRules := parent.GetAllRules()
		if len(allRules) != 2 {
			t.Fatalf("Expected 2 rules total, got %d", len(allRules))
		}
		
		// Check namespacing
		expectedNames := []string{"parent.parent-rule", "parent.child.child-rule"}
		for i, rule := range allRules {
			if rule.Name != expectedNames[i] {
				t.Errorf("Expected rule name %s, got %s", expectedNames[i], rule.Name)
			}
		}
		
		// Test stats
		stats := parent.GetStats()
		if stats.RuleCount != 1 {
			t.Errorf("Expected parent direct rule count 1, got %d", stats.RuleCount)
		}
		if stats.SubGroupCount != 1 {
			t.Errorf("Expected subgroup count 1, got %d", stats.SubGroupCount)
		}
		if stats.TotalRules != 2 {
			t.Errorf("Expected total rules 2, got %d", stats.TotalRules)
		}
	})
	
	t.Run("CompositeResolver_GroupResolution", func(t *testing.T) {
		// Create groups
		userGroup := NewRuleGroup("users")
		userGroup.AddRule(Rule{
			Name:     "user-lww",
			Matcher:  EventTypeIs("UserUpdated"),
			Resolver: &LastWriteWinsResolver{},
		})
		
		orderGroup := NewRuleGroup("orders")
		orderGroup.AddRule(Rule{
			Name:     "order-merge",
			Matcher:  EventTypeIs("OrderCreated"),
			Resolver: &AdditiveMergeResolver{},
		})
		
		// Create composite resolver
		composite := NewCompositeResolver(&ManualReviewResolver{},
			WithCompositeLogger(&mockLogger{}),
		)
		composite.AddGroup(userGroup).AddGroup(orderGroup)
		
		// Test user event resolution
		userConflict := Conflict{
			EventType:   "UserUpdated",
			AggregateID: "user-123",
			Local:       EventWithVersion{Event: &testEvent{eventType: "UserUpdated"}, Version: &testVersion{v: 1}},
			Remote:      EventWithVersion{Event: &testEvent{eventType: "UserUpdated"}, Version: &testVersion{v: 2}},
			Metadata:    make(map[string]any),
		}
		
		resolved, err := composite.Resolve(context.Background(), userConflict)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if resolved.Decision != "keep_remote" {
			t.Errorf("Expected LWW decision 'keep_remote', got %s", resolved.Decision)
		}
		
		// Test order event resolution
		orderConflict := Conflict{
			EventType:   "OrderCreated",
			AggregateID: "order-456",
			Local:       EventWithVersion{Event: &testEvent{eventType: "OrderCreated"}, Version: &testVersion{v: 1}},
			Remote:      EventWithVersion{Event: &testEvent{eventType: "OrderCreated"}, Version: &testVersion{v: 2}},
			Metadata:    make(map[string]any),
		}
		
		resolved, err = composite.Resolve(context.Background(), orderConflict)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if resolved.Decision != "merge" {
			t.Errorf("Expected merge decision 'merge', got %s", resolved.Decision)
		}
		
		// Test fallback for unknown event
		unknownConflict := Conflict{
			EventType:   "UnknownEvent",
			AggregateID: "unknown-789",
			Local:       EventWithVersion{Event: &testEvent{eventType: "UnknownEvent"}, Version: &testVersion{v: 1}},
			Remote:      EventWithVersion{Event: &testEvent{eventType: "UnknownEvent"}, Version: &testVersion{v: 2}},
			Metadata:    make(map[string]any),
		}
		
		resolved, err = composite.Resolve(context.Background(), unknownConflict)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if resolved.Decision != "manual_review" {
			t.Errorf("Expected fallback decision 'manual_review', got %s", resolved.Decision)
		}
	})
}

func TestPhase6MementoPattern(t *testing.T) {
	t.Run("InMemoryMementoCaretaker_BasicOperations", func(t *testing.T) {
		caretaker := NewInMemoryMementoCaretaker()
		ctx := context.Background()
		
		// Create test memento
		memento := &ResolutionMemento{
			ID:        "test-memento-1",
			Timestamp: time.Now(),
			OriginalConflict: Conflict{
				EventType:   "TestEvent",
				AggregateID: "test-aggregate",
			},
			ResolvedConflict: ResolvedConflict{
				Decision: "keep_local",
				Reasons:  []string{"test reason"},
			},
			ResolverName: "TestResolver",
			UserID:       "user123",
		}
		
		// Test save and get
		err := caretaker.Save(ctx, memento)
		if err != nil {
			t.Fatalf("Expected no error saving memento, got %v", err)
		}
		
		retrieved, err := caretaker.Get(ctx, "test-memento-1")
		if err != nil {
			t.Fatalf("Expected no error getting memento, got %v", err)
		}
		
		if retrieved.ID != memento.ID {
			t.Errorf("Expected ID %s, got %s", memento.ID, retrieved.ID)
		}
		if retrieved.UserID != "user123" {
			t.Errorf("Expected UserID user123, got %s", retrieved.UserID)
		}
	})
	
	t.Run("AuditableResolver_Integration", func(t *testing.T) {
		caretaker := NewInMemoryMementoCaretaker()
		baseResolver := &LastWriteWinsResolver{}
		logger := &mockLogger{}
		
		auditableResolver := NewAuditableResolver(baseResolver, caretaker,
			WithAuditLogger(logger),
			WithUserIDExtractor(func(ctx context.Context) string {
				if uid := ctx.Value("user_id"); uid != nil {
					return uid.(string)
				}
				return ""
			}),
		)
		
		// Create context with user ID
		ctx := context.WithValue(context.Background(), "user_id", "test-user")
		
		// Create test conflict
		conflict := Conflict{
			EventType:   "TestEvent",
			AggregateID: "test-aggregate",
			Local:       EventWithVersion{Event: &testEvent{eventType: "TestEvent"}, Version: &testVersion{v: 1}},
			Remote:      EventWithVersion{Event: &testEvent{eventType: "TestEvent"}, Version: &testVersion{v: 2}},
			Metadata:    make(map[string]any),
		}
		
		// Resolve conflict
		resolved, err := auditableResolver.Resolve(ctx, conflict)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Verify resolution worked
		if resolved.Decision != "keep_remote" {
			t.Errorf("Expected decision 'keep_remote', got %s", resolved.Decision)
		}
		
		// Check audit trail
		trail, err := auditableResolver.GetAuditTrail(ctx, "test-aggregate")
		if err != nil {
			t.Fatalf("Expected no error getting audit trail, got %v", err)
		}
		
		
		if len(trail) != 1 {
			t.Fatalf("Expected 1 audit entry, got %d", len(trail))
		}
		
		entry := trail[0]
		if entry.UserID != "test-user" {
			t.Errorf("Expected UserID 'test-user', got %s", entry.UserID)
		}
		if entry.AggregateID != "test-aggregate" {
			t.Errorf("Expected AggregateID 'test-aggregate', got %s", entry.AggregateID)
		}
	})
	
	t.Run("RollbackCapability_Analysis", func(t *testing.T) {
		caretaker := NewInMemoryMementoCaretaker()
		rollback := NewRollbackCapability(caretaker)
		ctx := context.Background()
		
		// Create multiple mementos for the same aggregate
		baseTime := time.Now()
		mementos := []*ResolutionMemento{
			{
				ID:          "memento-1",
				Timestamp:   baseTime,
				AggregateID: "agg-1",
			},
			{
				ID:          "memento-2", 
				Timestamp:   baseTime.Add(1 * time.Minute),
				AggregateID: "agg-1",
			},
			{
				ID:          "memento-3",
				Timestamp:   baseTime.Add(2 * time.Minute),
				AggregateID: "agg-1",
			},
		}
		
		// Save all mementos
		for _, memento := range mementos {
			caretaker.Save(ctx, memento)
		}
		
		// Analyze rollback for first memento
		analysis, err := rollback.AnalyzeRollback(ctx, "memento-1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if analysis.TargetMemento.ID != "memento-1" {
			t.Errorf("Expected target memento ID 'memento-1', got %s", analysis.TargetMemento.ID)
		}
		if len(analysis.AffectedResolutions) != 2 {
			t.Errorf("Expected 2 affected resolutions, got %d", len(analysis.AffectedResolutions))
		}
		if analysis.RollbackComplexity != "moderate" {
			t.Errorf("Expected complexity 'moderate', got %s", analysis.RollbackComplexity)
		}
		if !analysis.RequiresReprocessing {
			t.Errorf("Expected RequiresReprocessing to be true")
		}
	})
}

func TestPhase6ObserverPattern(t *testing.T) {
	t.Run("ObservableResolver_BasicMetrics", func(t *testing.T) {
		collector := &mockExtendedMetricsCollector{}
		baseResolver := &LastWriteWinsResolver{}
		
		observableResolver := NewObservableResolver(baseResolver,
			WithMetricsCollector(collector),
			WithGroupName("test-group"),
		)
		
		// Create test conflict
		conflict := Conflict{
			EventType:   "TestEvent",
			AggregateID: "test-aggregate",
			Local:       EventWithVersion{Event: &testEvent{eventType: "TestEvent"}, Version: &testVersion{v: 1}},
			Remote:      EventWithVersion{Event: &testEvent{eventType: "TestEvent"}, Version: &testVersion{v: 2}},
			Metadata:    make(map[string]any),
		}
		
		// Resolve conflict
		resolved, err := observableResolver.Resolve(context.Background(), conflict)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Verify resolution worked
		if resolved.Decision != "keep_remote" {
			t.Errorf("Expected decision 'keep_remote', got %s", resolved.Decision)
		}
		
		// Verify metrics were collected
		if len(collector.resolutions) != 1 {
			t.Fatalf("Expected 1 resolution metric, got %d", len(collector.resolutions))
		}
		
		metric := collector.resolutions[0]
		if metric.GroupName != "test-group" {
			t.Errorf("Expected group name 'test-group', got %s", metric.GroupName)
		}
		if metric.Decision != "keep_remote" {
			t.Errorf("Expected decision 'keep_remote', got %s", metric.Decision)
		}
		if !metric.Success {
			t.Errorf("Expected success to be true")
		}
	})
	
	t.Run("ObservableResolver_WithHooks", func(t *testing.T) {
		var hookCalls []string
		hooks := &ResolutionHooks{
			OnConflict: func(ctx context.Context, conflict *Conflict) {
				hookCalls = append(hookCalls, fmt.Sprintf("conflict-%s", conflict.AggregateID))
			},
			OnResolution: func(ctx context.Context, result *ResolvedConflict, duration time.Duration) {
				hookCalls = append(hookCalls, fmt.Sprintf("resolution-%s", result.Decision))
			},
		}
		
		observableResolver := NewObservableResolver(&LastWriteWinsResolver{},
			WithResolutionHooks(hooks),
		)
		
		conflict := Conflict{
			EventType:   "TestEvent",
			AggregateID: "hook-test",
			Local:       EventWithVersion{Event: &testEvent{eventType: "TestEvent"}, Version: &testVersion{v: 1}},
			Remote:      EventWithVersion{Event: &testEvent{eventType: "TestEvent"}, Version: &testVersion{v: 2}},
			Metadata:    make(map[string]any),
		}
		
		_, err := observableResolver.Resolve(context.Background(), conflict)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Verify hooks were called
		if len(hookCalls) != 2 {
			t.Fatalf("Expected 2 hook calls, got %d", len(hookCalls))
		}
		if hookCalls[0] != "conflict-hook-test" {
			t.Errorf("Expected first hook call 'conflict-hook-test', got %s", hookCalls[0])
		}
		if hookCalls[1] != "resolution-keep_remote" {
			t.Errorf("Expected second hook call 'resolution-keep_remote', got %s", hookCalls[1])
		}
	})
}

// Test helper types and mocks
type testEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
}

func (e *testEvent) ID() string                         { return e.id }
func (e *testEvent) Type() string                       { return e.eventType }
func (e *testEvent) AggregateID() string                { return e.aggregateID }
func (e *testEvent) Data() interface{}                  { return e.data }
func (e *testEvent) Metadata() map[string]interface{}   { return nil }

type testVersion struct {
	v int
}

func (v *testVersion) String() string { return fmt.Sprintf("v%d", v.v) }
func (v *testVersion) Compare(other Version) int {
	if other == nil { return 1 }
	if otherV, ok := other.(*testVersion); ok {
		if v.v < otherV.v { return -1 }
		if v.v > otherV.v { return 1 }
		return 0
	}
	return 0
}
func (v *testVersion) IsZero() bool { return v.v == 0 }

type mockLogger struct{
	errorMessages []string
	debugMessages []string
}
func (l *mockLogger) Debug(msg string, args ...interface{}) {
	l.debugMessages = append(l.debugMessages, fmt.Sprintf(msg, args...))
}
func (l *mockLogger) Error(msg string, args ...interface{}) {
	l.errorMessages = append(l.errorMessages, fmt.Sprintf(msg, args...))
}

type resolutionMetric struct {
	RuleName  string
	GroupName string
	Decision  string
	Duration  time.Duration
	Success   bool
}

type mockExtendedMetricsCollector struct {
	resolutions []resolutionMetric
}

func (c *mockExtendedMetricsCollector) RecordSyncDuration(op string, dur time.Duration) {}
func (c *mockExtendedMetricsCollector) RecordSyncEvents(pushed, pulled int) {}
func (c *mockExtendedMetricsCollector) RecordConflicts(count int) {}
func (c *mockExtendedMetricsCollector) RecordSyncErrors(op, errType string) {}
func (c *mockExtendedMetricsCollector) RecordResolution(rule, group, decision string, dur time.Duration, success bool) {
	c.resolutions = append(c.resolutions, resolutionMetric{
		RuleName:  rule,
		GroupName: group, 
		Decision:  decision,
		Duration:  dur,
		Success:   success,
	})
}
func (c *mockExtendedMetricsCollector) RecordRuleMatch(rule, group string) {}
func (c *mockExtendedMetricsCollector) RecordFallbackUsage(group string) {}
func (c *mockExtendedMetricsCollector) RecordResolutionError(rule, group, errType string) {}
func (c *mockExtendedMetricsCollector) RecordManualReview(reason string) {}
