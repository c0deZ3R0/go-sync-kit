// Example 8: Stateful Conflict Resolvers
//
// This example demonstrates:
// - Stateful dynamic resolvers with state machine integration
// - Performance monitoring and metrics collection
// - Advanced rule-based conflict resolution
// - Workflow tracking and audit trails
// - Custom observability hooks and monitoring
// - Real-time state transitions and rule evaluation

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// UserAccountEvent represents user account management events
type UserAccountEvent struct {
	EventID       string            `json:"id"`
	EventType     string            `json:"event_type"`
	UserID        string            `json:"user_id"`
	AccountType   string            `json:"account_type"`
	Email         string            `json:"email"`
	DisplayName   string            `json:"display_name"`
	Permissions   []string          `json:"permissions"`
	LastActivity  time.Time         `json:"last_activity"`
	Priority      int               `json:"priority"`
	EventMetadata map[string]string `json:"metadata"`
	ModifiedBy    string            `json:"modified_by"`
	ModifiedAt    time.Time         `json:"modified_at"`
}

// Implement the Event interface
func (e *UserAccountEvent) ID() string          { return e.EventID }
func (e *UserAccountEvent) Type() string        { return e.EventType }
func (e *UserAccountEvent) AggregateID() string { return e.UserID }
func (e *UserAccountEvent) Data() interface{}   { return e }

func (e *UserAccountEvent) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"account_type":  e.AccountType,
		"email":         e.Email,
		"display_name":  e.DisplayName,
		"permissions":   e.Permissions,
		"last_activity": e.LastActivity,
		"priority":      e.Priority,
		"modified_by":   e.ModifiedBy,
		"modified_at":   e.ModifiedAt,
	}
}

// Priority-based resolver that considers user permissions and account types
type PriorityResolver struct {
	name        string
	description string
}

func (r *PriorityResolver) Resolve(ctx context.Context, conflict synckit.Conflict) (synckit.ResolvedConflict, error) {
	fmt.Printf("🎯 Priority resolver '%s' evaluating conflict...\n", r.name)

	localEvent, localOk := conflict.Local.Event.Data().(*UserAccountEvent)
	remoteEvent, remoteOk := conflict.Remote.Event.Data().(*UserAccountEvent)

	if !localOk || !remoteOk {
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{conflict.Remote},
			Decision:       "fallback_to_remote",
			Reasons:        []string{"Could not parse events, using remote as fallback"},
		}, nil
	}

	// Priority-based resolution logic
	reasons := []string{}
	selectedEvent := conflict.Remote
	decision := "remote_priority"

	// Check account type priority (admin > premium > standard)
	localPriority := getAccountTypePriority(localEvent.AccountType)
	remotePriority := getAccountTypePriority(remoteEvent.AccountType)

	if localPriority > remotePriority {
		selectedEvent = conflict.Local
		decision = "local_account_priority"
		reasons = append(reasons, fmt.Sprintf("Local account type '%s' has higher priority than remote '%s'",
			localEvent.AccountType, remoteEvent.AccountType))
	} else if remotePriority > localPriority {
		selectedEvent = conflict.Remote
		decision = "remote_account_priority"
		reasons = append(reasons, fmt.Sprintf("Remote account type '%s' has higher priority than local '%s'",
			remoteEvent.AccountType, localEvent.AccountType))
	} else {
		// Same account type, check permissions count
		if len(localEvent.Permissions) > len(remoteEvent.Permissions) {
			selectedEvent = conflict.Local
			decision = "local_permissions"
			reasons = append(reasons, fmt.Sprintf("Local has more permissions (%d vs %d)",
				len(localEvent.Permissions), len(remoteEvent.Permissions)))
		} else if len(remoteEvent.Permissions) > len(localEvent.Permissions) {
			selectedEvent = conflict.Remote
			decision = "remote_permissions"
			reasons = append(reasons, fmt.Sprintf("Remote has more permissions (%d vs %d)",
				len(remoteEvent.Permissions), len(localEvent.Permissions)))
		} else {
			// Fall back to timestamp
			if remoteEvent.ModifiedAt.After(localEvent.ModifiedAt) {
				selectedEvent = conflict.Remote
				decision = "remote_timestamp"
				reasons = append(reasons, "Remote event is more recent")
			} else {
				selectedEvent = conflict.Local
				decision = "local_timestamp"
				reasons = append(reasons, "Local event is more recent")
			}
		}
	}

	return synckit.ResolvedConflict{
		ResolvedEvents: []synckit.EventWithVersion{selectedEvent},
		Decision:       decision,
		Reasons:        reasons,
	}, nil
}

func getAccountTypePriority(accountType string) int {
	switch strings.ToLower(accountType) {
	case "admin":
		return 3
	case "premium":
		return 2
	case "standard":
		return 1
	default:
		return 0
	}
}

// Administrative override resolver for critical operations
type AdminOverrideResolver struct {
	approvedAdmins []string
}

func (r *AdminOverrideResolver) Resolve(ctx context.Context, conflict synckit.Conflict) (synckit.ResolvedConflict, error) {
	fmt.Printf("🔒 Admin override resolver checking for administrative authority...\n")

	localEvent, localOk := conflict.Local.Event.Data().(*UserAccountEvent)
	remoteEvent, remoteOk := conflict.Remote.Event.Data().(*UserAccountEvent)

	if !localOk || !remoteOk {
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{conflict.Remote},
			Decision:       "no_admin_override",
			Reasons:        []string{"Could not determine admin status"},
		}, nil
	}

	// Check if either modifier is an approved admin
	localIsAdmin := r.isApprovedAdmin(localEvent.ModifiedBy)
	remoteIsAdmin := r.isApprovedAdmin(remoteEvent.ModifiedBy)

	if localIsAdmin && !remoteIsAdmin {
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{conflict.Local},
			Decision:       "local_admin_override",
			Reasons:        []string{fmt.Sprintf("Local modifier '%s' has administrative authority", localEvent.ModifiedBy)},
		}, nil
	}

	if remoteIsAdmin && !localIsAdmin {
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{conflict.Remote},
			Decision:       "remote_admin_override",
			Reasons:        []string{fmt.Sprintf("Remote modifier '%s' has administrative authority", remoteEvent.ModifiedBy)},
		}, nil
	}

	// If both or neither are admins, fall back to another strategy
	return synckit.ResolvedConflict{
		ResolvedEvents: []synckit.EventWithVersion{conflict.Remote},
		Decision:       "no_admin_override",
		Reasons:        []string{"No administrative override applies, using default resolution"},
	}, nil
}

func (r *AdminOverrideResolver) isApprovedAdmin(userID string) bool {
	for _, admin := range r.approvedAdmins {
		if admin == userID {
			return true
		}
	}
	return false
}

// MockTransport simulates a remote source with conflicting events
type MockTransport struct {
	remoteEvents []synckit.EventWithVersion
	pushCount    int
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		remoteEvents: make([]synckit.EventWithVersion, 0),
	}
}

func (mt *MockTransport) AddRemoteEvent(event synckit.Event, version synckit.Version) {
	mt.remoteEvents = append(mt.remoteEvents, synckit.EventWithVersion{
		Event:   event,
		Version: version,
	})
}

func (mt *MockTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Return all remote events that are newer than the given version
	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok {
		sinceCursor = cursor.IntegerCursor{Seq: 0}
	}

	var events []synckit.EventWithVersion
	for _, event := range mt.remoteEvents {
		if eventCursor, ok := event.Version.(cursor.IntegerCursor); ok {
			if eventCursor.Seq > sinceCursor.Seq {
				events = append(events, event)
			}
		}
	}
	return events, nil
}

func (mt *MockTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	mt.pushCount += len(events)
	fmt.Printf("    📤 MockTransport: Pushed %d events (total: %d)\n", len(events), mt.pushCount)
	return nil
}

func (mt *MockTransport) Close() error {
	fmt.Println("    🔌 MockTransport: Connection closed")
	return nil
}

func (mt *MockTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	// Return the highest version from remote events
	var maxVersion uint64 = 0
	for _, event := range mt.remoteEvents {
		if eventCursor, ok := event.Version.(cursor.IntegerCursor); ok {
			if eventCursor.Seq > maxVersion {
				maxVersion = eventCursor.Seq
			}
		}
	}
	return cursor.IntegerCursor{Seq: maxVersion}, nil
}

func (mt *MockTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	// Mock implementation - no real-time subscriptions for this demo
	fmt.Println("    🔔 MockTransport: Subscription enabled (mock)")
	return nil
}

// Custom observability hooks for monitoring
type MonitoringHooks struct {
	stateTransitions    int
	rulesEvaluated      int
	workflowsCompleted  int
	totalResolutionTime time.Duration
}

func (h *MonitoringHooks) createObservabilityHooks() *statemachine.ConflictResolutionObservabilityHooks {
	return &statemachine.ConflictResolutionObservabilityHooks{
		OnStateTransition: func(from, to statemachine.ConflictResolutionState, metadata map[string]interface{}) {
			h.stateTransitions++
			fmt.Printf("  🔄 State: %s → %s\n", from.String(), to.String())
		},

		OnWorkflowStarted: func(conflictID string, conflict synckit.Conflict) {
			fmt.Printf("  📋 Workflow started for aggregate: %s\n", conflict.AggregateID)
		},

		OnWorkflowCompleted: func(conflictID string, auditTrail *statemachine.ConflictAuditTrail) {
			h.workflowsCompleted++
			fmt.Printf("  ✅ Workflow completed for conflict: %s\n", conflictID)
		},

		OnRuleEvaluationStarted: func(conflictID, ruleName string) {
			fmt.Printf("  🔍 Evaluating rule: %s\n", ruleName)
		},

		OnRuleEvaluationCompleted: func(conflictID, ruleName string, matched bool, duration time.Duration) {
			h.rulesEvaluated++
			h.totalResolutionTime += duration
			status := "✅"
			if !matched {
				status = "❌"
			}
			fmt.Printf("  %s Rule '%s' evaluation: %v (took %v)\n", status, ruleName, matched, duration)
		},

		OnRuleEvaluationFailed: func(conflictID, ruleName string, err error) {
			fmt.Printf("  ❌ Rule '%s' failed: %v\n", ruleName, err)
		},

		OnMetricsRecorded: func(metrics *statemachine.ResolverPerformanceMetrics) {
			fmt.Printf("  📊 Performance metrics updated: %d conflicts resolved, avg time: %v\n",
				metrics.TotalConflictsResolved, metrics.AverageResolutionTime)
		},
	}
}

func (h *MonitoringHooks) printSummary() {
	fmt.Printf("\n📈 Monitoring Summary:\n")
	fmt.Printf("  • State transitions: %d\n", h.stateTransitions)
	fmt.Printf("  • Rules evaluated: %d\n", h.rulesEvaluated)
	fmt.Printf("  • Workflows completed: %d\n", h.workflowsCompleted)
	fmt.Printf("  • Total rule evaluation time: %v\n", h.totalResolutionTime)
	if h.rulesEvaluated > 0 {
		avgTime := h.totalResolutionTime / time.Duration(h.rulesEvaluated)
		fmt.Printf("  • Average rule evaluation time: %v\n", avgTime)
	}
}

func main() {
	fmt.Println("=== Go Sync Kit Example 8: Stateful Conflict Resolvers ===\n")

	// Setup
	fmt.Println("🏗️ Setting up stateful resolution environment...")

	store, err := sqlite.NewWithDataSource("stateful-resolvers.db")
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create monitoring hooks
	monitoring := &MonitoringHooks{}

	fmt.Println("\n🎛️ Configuring advanced stateful resolvers...")

	// Scenario 1: Priority-based resolution with full state machine
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("📋 Scenario 1: Priority-Based Resolution with State Tracking")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	// Create priority-based resolver
	priorityResolver := &PriorityResolver{
		name:        "PriorityResolver",
		description: "Resolves conflicts based on account type and permissions",
	}

	// Create stateful dynamic resolver with comprehensive rules
	dynamicResolver, err := synckit.NewDynamicResolver(
		synckit.WithRule("admin_accounts",
			synckit.And(
				synckit.EventTypeIs("account.updated"),
				synckit.MetadataEq("account_type", "admin"),
			),
			&AdminOverrideResolver{approvedAdmins: []string{"admin-001", "admin-002"}},
		),
		synckit.WithRule("priority_accounts",
			synckit.Or(
				synckit.MetadataEq("account_type", "premium"),
				synckit.MetadataEq("account_type", "admin"),
			),
			priorityResolver,
		),
		synckit.WithFallback(priorityResolver),
	)
	if err != nil {
		log.Fatalf("Failed to create dynamic resolver: %v", err)
	}

	// Configure stateful resolver with all features enabled
	statefulOptions := &statemachine.StatefulResolverOptions{
		EnableStateMachine:       true,
		EnablePerformanceMetrics: true,
		EnableWorkflowTracking:   true,
		EnableAuditTrail:         true,
		ObservabilityHooks:       monitoring.createObservabilityHooks(),
		WorkflowOptions:          statemachine.DefaultWorkflowOptions(),
		MaxStateHistorySize:      100,
	}

	statefulResolver, err := synckit.NewStatefulDynamicResolver(dynamicResolver, statefulOptions)
	if err != nil {
		log.Fatalf("Failed to create stateful resolver: %v", err)
	}

	// Create test conflicts
	baseTime := time.Now()
	timestamp := baseTime.Unix()

	// Create mock transport with remote conflicting events
	mockTransport := NewMockTransport()

	// Add remote events that will conflict with local events
	remoteStandardEvent := &UserAccountEvent{
		EventID:      fmt.Sprintf("remote-standard-%d", timestamp),
		EventType:    "account.updated",
		UserID:       "user-001", // Same user = conflict
		AccountType:  "standard",
		Email:        "john.doe@company.com", // Different email
		DisplayName:  "John D.",              // Different name
		Permissions:  []string{"read"},       // Fewer permissions
		LastActivity: baseTime.Add(-2 * time.Hour),
		Priority:     1,
		ModifiedBy:   "user-001",
		ModifiedAt:   baseTime.Add(-5 * time.Second), // Older
	}
	mockTransport.AddRemoteEvent(remoteStandardEvent, cursor.IntegerCursor{Seq: 1000})

	remotePremiumEvent := &UserAccountEvent{
		EventID:      fmt.Sprintf("remote-premium-%d", timestamp+1),
		EventType:    "account.updated",
		UserID:       "user-002", // Different conflict
		AccountType:  "premium",
		Email:        "jane@example.com",
		DisplayName:  "Jane Smith (Premium)",
		Permissions:  []string{"read", "write", "admin"},
		LastActivity: baseTime.Add(-1 * time.Hour),
		Priority:     2,
		ModifiedBy:   "admin-001",
		ModifiedAt:   baseTime.Add(15 * time.Second), // Newer
	}
	mockTransport.AddRemoteEvent(remotePremiumEvent, cursor.IntegerCursor{Seq: 1001})

	remoteAdminEvent := &UserAccountEvent{
		EventID:      fmt.Sprintf("remote-admin-%d", timestamp+2),
		EventType:    "account.updated",
		UserID:       "user-001", // Same user = conflict with local admin
		AccountType:  "admin",
		Email:        "john@example.com",
		DisplayName:  "John Doe (Remote Admin)",
		Permissions:  []string{"read", "write", "admin", "delete", "system", "super"},
		LastActivity: baseTime.Add(-10 * time.Minute),
		Priority:     3,
		ModifiedBy:   "admin-001",                    // Different admin
		ModifiedAt:   baseTime.Add(25 * time.Second), // Even newer
	}
	mockTransport.AddRemoteEvent(remoteAdminEvent, cursor.IntegerCursor{Seq: 1002})

	fmt.Printf("  📡 Set up %d remote conflicting events\n", len(mockTransport.remoteEvents))

	// Create sync manager with mock transport
	manager, err := synckit.NewManager(
		synckit.WithStore(store),
		synckit.WithTransport(mockTransport),
		synckit.WithConflictResolver(statefulResolver),
	)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// Standard user event
	standardUserEvent := &UserAccountEvent{
		EventID:      fmt.Sprintf("event-standard-%d", timestamp+10),
		EventType:    "account.updated",
		UserID:       "user-001",
		AccountType:  "standard",
		Email:        "john@example.com",
		DisplayName:  "John Doe",
		Permissions:  []string{"read", "write"},
		LastActivity: baseTime.Add(-1 * time.Hour),
		Priority:     1,
		ModifiedBy:   "user-001",
		ModifiedAt:   baseTime,
	}

	// Premium user event (same user, creates conflict)
	premiumUserEvent := &UserAccountEvent{
		EventID:      fmt.Sprintf("event-premium-%d", timestamp+11),
		EventType:    "account.updated",
		UserID:       "user-001", // Same user ID = conflict
		AccountType:  "premium",
		Email:        "john@example.com",
		DisplayName:  "John Doe (Premium)",
		Permissions:  []string{"read", "write", "admin", "delete"},
		LastActivity: baseTime.Add(-30 * time.Minute),
		Priority:     2,
		ModifiedBy:   "admin-001",
		ModifiedAt:   baseTime.Add(10 * time.Second),
	}

	// Admin user event (same user, creates another conflict)
	adminUserEvent := &UserAccountEvent{
		EventID:      fmt.Sprintf("event-admin-%d", timestamp+12),
		EventType:    "account.updated",
		UserID:       "user-001", // Same user ID = conflict
		AccountType:  "admin",
		Email:        "john@example.com",
		DisplayName:  "John Doe (Admin)",
		Permissions:  []string{"read", "write", "admin", "delete", "system"},
		LastActivity: baseTime.Add(-15 * time.Minute),
		Priority:     3,
		ModifiedBy:   "admin-002",
		ModifiedAt:   baseTime.Add(20 * time.Second),
	}

	// Store events to create conflicts
	fmt.Println("\n📦 Storing conflicting events...")

	events := []*UserAccountEvent{standardUserEvent, premiumUserEvent, adminUserEvent}
	for i, event := range events {
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		err = store.Store(ctx, event, version)
		if err != nil {
			log.Printf("Failed to store event %s: %v", event.EventID, err)
			continue
		}
		fmt.Printf("  💾 Stored %s account event for user %s\n", event.AccountType, event.UserID)
	}

	// Trigger conflict resolution
	fmt.Println("\n🔄 Triggering conflict resolution...")

	result, err := manager.Sync(ctx)
	if err != nil {
		log.Printf("Sync failed: %v", err)
	} else {
		fmt.Printf("✅ Sync completed: %d conflicts resolved\n", result.ConflictsResolved)
	}

	// Display state machine status
	fmt.Println("\n🎛️ State Machine Status:")
	currentState := statefulResolver.GetCurrentState()
	fmt.Printf("  • Current State: %s\n", currentState.String())

	stateHistory := statefulResolver.GetStateHistory()
	fmt.Printf("  • State History: %d transitions\n", len(stateHistory))

	// Display performance metrics
	fmt.Println("\n📊 Performance Metrics:")
	metrics := statefulResolver.GetPerformanceMetrics()
	if metrics != nil {
		fmt.Printf("  • Total Conflicts Resolved: %d\n", metrics.TotalConflictsResolved)
		fmt.Printf("  • Auto-Resolved Count: %d\n", metrics.AutoResolvedCount)
		fmt.Printf("  • Manual Review Count: %d\n", metrics.ManualReviewCount)
		fmt.Printf("  • Average Resolution Time: %v\n", metrics.AverageResolutionTime)
		fmt.Printf("  • Total Resolution Time: %v\n", metrics.TotalResolutionTime)
	}

	// Display workflow status
	fmt.Println("\n📋 Workflow Status:")
	workflowManager := statefulResolver.GetWorkflowManager()
	if workflowManager != nil {
		activeWorkflows := statefulResolver.GetActiveWorkflows()
		fmt.Printf("  • Active Workflows: %d\n", len(activeWorkflows))
		fmt.Printf("  • Workflow Manager: Available\n")
	}

	// Scenario 2: Real-time state monitoring
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("📋 Scenario 2: Real-time State Monitoring")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	// Create state observer for real-time monitoring
	stateObserver := &StateObserver{name: "RealTimeMonitor"}
	statefulResolver.SubscribeToStateChanges(stateObserver)

	// Create more conflicts to demonstrate state transitions
	fmt.Println("\n🔄 Creating additional conflicts for state monitoring...")

	for i := 0; i < 3; i++ {
		userEvent := &UserAccountEvent{
			EventID:      fmt.Sprintf("monitor-event-%d-%d", timestamp+100+int64(i), i),
			EventType:    "account.updated",
			UserID:       fmt.Sprintf("monitor-user-%d", i),
			AccountType:  []string{"standard", "premium", "admin"}[i],
			Email:        fmt.Sprintf("user%d@example.com", i),
			DisplayName:  fmt.Sprintf("User %d", i),
			Permissions:  []string{"read", "write", "admin"}[:i+1],
			LastActivity: time.Now(),
			Priority:     i + 1,
			ModifiedBy:   fmt.Sprintf("user-%d", i),
			ModifiedAt:   time.Now(),
		}

		version := cursor.IntegerCursor{Seq: uint64(10 + i)}
		err = store.Store(ctx, userEvent, version)
		if err == nil {
			fmt.Printf("  💾 Stored monitoring event for user %s\n", userEvent.UserID)
		}
	}

	// Trigger another sync to see state transitions
	fmt.Println("\n🔄 Triggering sync with state monitoring...")
	result, err = manager.Sync(ctx)
	if err != nil {
		log.Printf("Monitoring sync failed: %v", err)
	} else {
		fmt.Printf("✅ Monitoring sync completed: %d conflicts resolved\n", result.ConflictsResolved)
	}

	// Final statistics
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("📈 Final Statistics")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	monitoring.printSummary()

	// Final metrics
	finalMetrics := statefulResolver.GetPerformanceMetrics()
	if finalMetrics != nil {
		fmt.Printf("\n🏁 Final Performance Metrics:\n")
		fmt.Printf("  • Total Operations: %d\n", finalMetrics.TotalConflictsResolved)
		fmt.Printf("  • Success Rate: %.2f%%\n",
			float64(finalMetrics.AutoResolvedCount)/float64(finalMetrics.TotalConflictsResolved)*100)
		fmt.Printf("  • Average Processing Time: %v\n", finalMetrics.AverageResolutionTime)
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("🎉 Stateful Resolvers Demo Complete!")
	fmt.Println("\n💡 Key Achievements:")
	fmt.Println("   ✅ Demonstrated stateful conflict resolution")
	fmt.Println("   ✅ Showed real-time state machine transitions")
	fmt.Println("   ✅ Collected comprehensive performance metrics")
	fmt.Println("   ✅ Tracked complete workflow lifecycles")
	fmt.Println("   ✅ Integrated custom monitoring hooks")
	fmt.Println("   ✅ Showcased advanced rule evaluation")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
}

// StateObserver for real-time monitoring
type StateObserver struct {
	name            string
	transitionCount int
}

func (so *StateObserver) OnTransition(transition statemachine.StateTransition[statemachine.ConflictResolutionState]) {
	so.transitionCount++
	fmt.Printf("  🔄 [%s] Observed transition #%d: %s → %s\n",
		so.name, so.transitionCount, transition.From.String(), transition.To.String())
}

func (so *StateObserver) OnTransitionFailed(from, to statemachine.ConflictResolutionState, err error, metadata map[string]interface{}) {
	fmt.Printf("  ❌ [%s] Transition failed: %s → %s (error: %v)\n",
		so.name, from.String(), to.String(), err)
}
