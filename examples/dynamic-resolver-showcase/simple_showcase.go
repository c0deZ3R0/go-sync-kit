package main

import (
	"context"
	"fmt"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/version"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// SimpleEvent implements the Event interface for our demo
type SimpleEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
	timestamp   time.Time
	userID      string
}

func (e *SimpleEvent) ID() string                           { return e.id }
func (e *SimpleEvent) Type() string                         { return e.eventType }
func (e *SimpleEvent) AggregateID() string                  { return e.aggregateID }
func (e *SimpleEvent) Data() interface{}                    { return e.data }
func (e *SimpleEvent) Metadata() map[string]interface{}     { return e.metadata }
func (e *SimpleEvent) Timestamp() time.Time                 { return e.timestamp }
func (e *SimpleEvent) UserID() string                       { return e.userID }

// Document represents a simple document for our demo
type Document struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Version  int    `json:"version"`
	AuthorID string `json:"author_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BusinessHoursResolver prefers recent changes during business hours
type BusinessHoursResolver struct{}

func (r *BusinessHoursResolver) Resolve(ctx context.Context, c synckit.Conflict) (synckit.ResolvedConflict, error) {
	now := time.Now()
	isBusinessHours := now.Hour() >= 9 && now.Hour() < 17 && 
		now.Weekday() >= time.Monday && now.Weekday() <= time.Friday

	if isBusinessHours {
		// During business hours, prefer remote (more recent) changes
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{c.Remote},
			Decision:       "business_hours_recent",
			Reasons:        []string{"During business hours, preferring more recent changes"},
		}, nil
	} else {
		// Outside business hours, prefer local (offline work)
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{c.Local},
			Decision:       "offline_work_preference",
			Reasons:        []string{"Outside business hours, preferring local offline work"},
		}, nil
	}
}

// DocumentPriorityResolver handles document conflicts based on metadata
type DocumentPriorityResolver struct{}

// createVersion creates a VectorClock with a single node increment
func createVersion(nodeID string) *version.VectorClock {
	vc := version.NewVectorClock()
	vc.Increment(nodeID) // We ignore the error for demo purposes
	return vc
}

func (r *DocumentPriorityResolver) Resolve(ctx context.Context, c synckit.Conflict) (synckit.ResolvedConflict, error) {
	// Check if this is a high-priority document
	if priority, ok := c.Metadata["priority"]; ok && priority == "high" {
		// For high-priority documents, use manual review
		return synckit.ResolvedConflict{
			Decision: "manual_review_required",
			Reasons:  []string{"High-priority document requires manual review"},
		}, nil
	}

	// For normal priority, use last-write-wins
	fallback := &synckit.LastWriteWinsResolver{}
	return fallback.Resolve(ctx, c)
}

func main() {
	fmt.Printf("%s%s", colorBold, colorBlue)
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              🚀 SYNCKIT RESOLVER SHOWCASE 🚀                 ║")
	fmt.Println("║                                                              ║")
	fmt.Println("║   Simplified demonstration of synckit's conflict resolution ║")
	fmt.Println("║   with custom business rules and dynamic resolvers          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("%s\n", colorReset)

	ctx := context.Background()

	// Create a dynamic resolver with custom rules
	resolver, err := synckit.NewDynamicResolver(
		// Rule 1: Business hours rule for time-sensitive conflicts
		synckit.WithRule("BusinessHours", 
			synckit.MetadataEq("time_sensitive", true),
			&BusinessHoursResolver{},
		),
		// Rule 2: High priority documents require manual review
		synckit.WithRule("DocumentPriority",
			synckit.And(
				synckit.EventTypeIs("document_updated"),
				synckit.MetadataEq("priority", "high"),
			),
			&DocumentPriorityResolver{},
		),
		// Fallback to last-write-wins
		synckit.WithFallback(&synckit.LastWriteWinsResolver{}),
	)

	if err != nil {
		fmt.Printf("%sError creating resolver: %v%s\n", colorRed, err, colorReset)
		return
	}

	// Demo scenarios
	scenarios := []struct {
		name        string
		description string
		conflict    synckit.Conflict
	}{
		{
			name:        "📄 Document Conflict - Normal Priority",
			description: "Two users editing a normal document simultaneously",
			conflict: synckit.Conflict{
				EventType:   "document_updated",
				AggregateID: "doc_proposal",
				Metadata: map[string]any{
					"priority": "normal",
				},
				Local: synckit.EventWithVersion{
					Event: &SimpleEvent{
						id:          "evt_1_local",
						eventType:   "document_updated",
						aggregateID: "doc_proposal",
						data: &Document{
							ID:       "doc_proposal",
							Title:    "Project Proposal",
							Content:  "Initial draft of the project proposal",
							Version:  1,
							AuthorID: "alice",
							UpdatedAt: time.Now().Add(-10 * time.Minute),
						},
						timestamp: time.Now().Add(-10 * time.Minute),
						userID:    "alice",
					},
				Version: createVersion("alice"),
				},
				Remote: synckit.EventWithVersion{
					Event: &SimpleEvent{
						id:          "evt_1_remote",
						eventType:   "document_updated",
						aggregateID: "doc_proposal",
						data: &Document{
							ID:       "doc_proposal",
							Title:    "Project Proposal - Updated",
							Content:  "Updated project proposal with budget details",
							Version:  1,
							AuthorID: "bob",
							UpdatedAt: time.Now().Add(-5 * time.Minute),
						},
						timestamp: time.Now().Add(-5 * time.Minute),
						userID:    "bob",
					},
					Version: createVersion("bob"),
				},
			},
		},
		{
			name:        "⚡ High-Priority Document Conflict",
			description: "Conflict in a high-priority document requiring manual review",
			conflict: synckit.Conflict{
				EventType:   "document_updated",
				AggregateID: "doc_critical_spec",
				Metadata: map[string]any{
					"priority": "high",
				},
				Local: synckit.EventWithVersion{
					Event: &SimpleEvent{
						id:          "evt_2_local",
						eventType:   "document_updated",
						aggregateID: "doc_critical_spec",
						data: &Document{
							ID:       "doc_critical_spec",
							Title:    "Critical System Specification",
							Content:  "System requirements for critical infrastructure",
							Version:  2,
							AuthorID: "charlie",
							UpdatedAt: time.Now().Add(-15 * time.Minute),
						},
						timestamp: time.Now().Add(-15 * time.Minute),
						userID:    "charlie",
					},
					Version: createVersion("charlie"),
				},
				Remote: synckit.EventWithVersion{
					Event: &SimpleEvent{
						id:          "evt_2_remote",
						eventType:   "document_updated",
						aggregateID: "doc_critical_spec",
						data: &Document{
							ID:       "doc_critical_spec",
							Title:    "Critical System Specification v2",
							Content:  "Updated system requirements with security considerations",
							Version:  2,
							AuthorID: "diana",
							UpdatedAt: time.Now().Add(-5 * time.Minute),
						},
						timestamp: time.Now().Add(-5 * time.Minute),
						userID:    "diana",
					},
					Version: createVersion("diana"),
				},
			},
		},
		{
			name:        "⏰ Time-Sensitive Business Hours Conflict",
			description: "Conflict that considers business hours for resolution",
			conflict: synckit.Conflict{
				EventType:   "incident_report_updated",
				AggregateID: "incident_db_timeout",
				Metadata: map[string]any{
					"time_sensitive": true,
					"priority":       "critical",
				},
				Local: synckit.EventWithVersion{
					Event: &SimpleEvent{
						id:          "evt_3_local",
						eventType:   "incident_report_updated",
						aggregateID: "incident_db_timeout",
						data: &Document{
							ID:       "incident_db_timeout",
							Title:    "Database Timeout Incident",
							Content:  "Initial incident report: Database connection timeout detected",
							Version:  1,
							AuthorID: "ops_alice",
							UpdatedAt: time.Now().Add(-20 * time.Minute),
						},
						timestamp: time.Now().Add(-20 * time.Minute),
						userID:    "ops_alice",
					},
					Version: createVersion("ops_alice"),
				},
				Remote: synckit.EventWithVersion{
					Event: &SimpleEvent{
						id:          "evt_3_remote",
						eventType:   "incident_report_updated",
						aggregateID: "incident_db_timeout",
						data: &Document{
							ID:       "incident_db_timeout",
							Title:    "Database Timeout Incident - RESOLVED",
							Content:  "Updated: Database timeout resolved by increasing connection pool size",
							Version:  1,
							AuthorID: "ops_bob",
							UpdatedAt: time.Now().Add(-5 * time.Minute),
						},
						timestamp: time.Now().Add(-5 * time.Minute),
						userID:    "ops_bob",
					},
					Version: createVersion("ops_bob"),
				},
			},
		},
	}

	// Run through each scenario
	for i, scenario := range scenarios {
		fmt.Printf("\n%s%s=== Scenario %d: %s ===%s\n", colorBold, colorYellow, i+1, scenario.name, colorReset)
		fmt.Printf("%s%s%s\n", colorWhite, scenario.description, colorReset)

		fmt.Printf("\n%s🔍 Conflict Details:%s\n", colorCyan, colorReset)
		fmt.Printf("  Event Type: %s\n", scenario.conflict.EventType)
		fmt.Printf("  Aggregate ID: %s\n", scenario.conflict.AggregateID)
		fmt.Printf("  Metadata: %v\n", scenario.conflict.Metadata)

		// Resolve the conflict
		resolved, err := resolver.Resolve(ctx, scenario.conflict)
		if err != nil {
			fmt.Printf("%s❌ Error resolving conflict: %v%s\n", colorRed, err, colorReset)
			continue
		}

		// Display the resolution
		fmt.Printf("\n%s✅ Resolution Result:%s\n", colorBold+colorGreen, colorReset)
		fmt.Printf("  Decision: %s%s%s\n", colorPurple, resolved.Decision, colorReset)
		fmt.Printf("  Reasons:\n")
		for _, reason := range resolved.Reasons {
			fmt.Printf("    - %s\n", reason)
		}

		if len(resolved.ResolvedEvents) > 0 {
			fmt.Printf("  Resolved Events: %d\n", len(resolved.ResolvedEvents))
			for j, event := range resolved.ResolvedEvents {
				fmt.Printf("    [%d] Event ID: %s (User: %s)\n", j+1, event.Event.ID(), event.Event.(*SimpleEvent).UserID())
			}
		}

		fmt.Printf("\n%sPress Enter to continue to next scenario...%s", colorGreen, colorReset)
		fmt.Scanln()
	}

	fmt.Printf("\n%s%s🎉 Demo complete! Thanks for exploring the synckit resolver showcase!%s\n", colorBold, colorGreen, colorReset)
}
