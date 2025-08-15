// Example 4: Conflict Resolution Demo
// 
// This example demonstrates:
// - Different conflict resolution strategies (LWW, FWW, Additive Merge)
// - Simulating concurrent modifications from multiple clients
// - Understanding how conflicts are detected and resolved
// - Custom conflict resolution logic

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
)

// DocumentEvent represents a document editing event
type DocumentEvent struct {
	EventID     string    `json:"id"`
	EventType   string    `json:"event_type"`
	DocumentID  string    `json:"document_id"`
	Content     string    `json:"content"`
	AuthorID    string    `json:"author_id"`
	AuthorName  string    `json:"author_name"`
	EditTime    time.Time `json:"edit_time"`
	ClientID    string    `json:"client_id"`
	VersionNum  int       `json:"version_num"`
}

// Implement the Event interface
func (e *DocumentEvent) ID() string {
	return e.EventID
}

func (e *DocumentEvent) Type() string {
	return e.EventType
}

func (e *DocumentEvent) AggregateID() string {
	return e.DocumentID
}

func (e *DocumentEvent) Data() interface{} {
	return e
}

func (e *DocumentEvent) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"author_id":   e.AuthorID,
		"author_name": e.AuthorName,
		"client_id":   e.ClientID,
		"edit_time":   e.EditTime,
		"version":     e.VersionNum,
	}
}

// CustomConflictResolver demonstrates custom conflict resolution logic
type CustomConflictResolver struct {
	name string
}

func (r *CustomConflictResolver) Resolve(ctx context.Context, conflict synckit.Conflict) (synckit.ResolvedConflict, error) {
	fmt.Printf("🔧 Custom resolver '%s' handling conflict for aggregate: %s\n", r.name, conflict.AggregateID)
	
	// For this example, we'll implement a "latest author wins" strategy
	// The Conflict struct has Local and Remote single events, not slices
	
	var selectedEvent synckit.EventWithVersion
	var reason string
	
	// Get the local and remote events
	localDocEvent, localOk := conflict.Local.Event.Data().(*DocumentEvent)
	remoteDocEvent, remoteOk := conflict.Remote.Event.Data().(*DocumentEvent)
	
	if !localOk || !remoteOk {
		// Fallback to remote if we can't parse
		selectedEvent = conflict.Remote
		reason = "Unable to parse events, selected remote"
	} else {
		// Compare edit times and select the latest
		if remoteDocEvent.EditTime.After(localDocEvent.EditTime) {
			selectedEvent = conflict.Remote
			reason = fmt.Sprintf("Selected remote event from %s (%v) over local from %s (%v)", 
				remoteDocEvent.AuthorName, remoteDocEvent.EditTime.Format("15:04:05"),
				localDocEvent.AuthorName, localDocEvent.EditTime.Format("15:04:05"))
		} else {
			selectedEvent = conflict.Local
			reason = fmt.Sprintf("Selected local event from %s (%v) over remote from %s (%v)",
				localDocEvent.AuthorName, localDocEvent.EditTime.Format("15:04:05"),
				remoteDocEvent.AuthorName, remoteDocEvent.EditTime.Format("15:04:05"))
		}
	}

	return synckit.ResolvedConflict{
		ResolvedEvents: []synckit.EventWithVersion{selectedEvent},
		Decision:       "latest_author_wins",
		Reasons:        []string{reason},
	}, nil
}

func main() {
	fmt.Println("=== Go Sync Kit Example 4: Conflict Resolution Demo ===\n")

	// We'll simulate conflicts using two different stores (representing different clients)
	fmt.Println("🏗️ Setting up two client scenarios...")

	// Client A store
	storeA, err := sqlite.NewWithDataSource("client-a.db")
	if err != nil {
		log.Fatalf("Failed to create Client A store: %v", err)
	}
	defer storeA.Close()

	// Client B store  
	storeB, err := sqlite.NewWithDataSource("client-b.db")
	if err != nil {
		log.Fatalf("Failed to create Client B store: %v", err)
	}
	defer storeB.Close()

	ctx := context.Background()

	// Demonstrate different conflict resolution strategies
	strategies := []struct {
		name        string
		option      synckit.ManagerOption
		description string
	}{
		{"Last-Write-Wins", synckit.WithLWW(), "Most recent event wins based on timestamps"},
		{"First-Write-Wins", synckit.WithFWW(), "First event encountered wins"},
		{"Additive Merge", synckit.WithAdditiveMerge(), "Combines events when possible"},
		{"Custom Resolver", synckit.WithConflictResolver(&CustomConflictResolver{"latest_author"}), "Custom logic: latest author wins"},
	}

	for i, strategy := range strategies {
		fmt.Printf("\n%s\n", strings.Repeat("=", 60))
		fmt.Printf("📋 Strategy %d: %s\n", i+1, strategy.name)
		fmt.Printf("📝 %s\n", strategy.description)
		fmt.Printf("%s\n", strings.Repeat("=", 60))

		// Create managers for both clients with the current strategy
		mgrA, err := synckit.NewManager(
			synckit.WithStore(storeA),
			synckit.WithNullTransport(),
			strategy.option,
		)
		if err != nil {
			log.Fatalf("Failed to create manager A: %v", err)
		}

		mgrB, err := synckit.NewManager(
			synckit.WithStore(storeB),
			synckit.WithNullTransport(), 
			strategy.option,
		)
		if err != nil {
			log.Fatalf("Failed to create manager B: %v", err)
		}

		// Simulate conflicting edits to the same document
		baseTime := time.Now()
		
		// Client A edits the document first
		eventA := &DocumentEvent{
			EventID:    fmt.Sprintf("edit-a-%d", i),
			EventType:  "document.edited",
			DocumentID: "shared-doc-001",
			Content:    fmt.Sprintf("Content from Client A (strategy %d): This is the A version", i+1),
			AuthorID:   "user-alice",
			AuthorName: "Alice",
			EditTime:   baseTime,
			ClientID:   "client-a",
			VersionNum: 1,
		}

		// Client B edits the same document slightly later (should create conflict)
		eventB := &DocumentEvent{
			EventID:    fmt.Sprintf("edit-b-%d", i),
			EventType:  "document.edited",
			DocumentID: "shared-doc-001", // Same document!
			Content:    fmt.Sprintf("Content from Client B (strategy %d): This is the B version", i+1),
			AuthorID:   "user-bob",
			AuthorName: "Bob",
			EditTime:   baseTime.Add(30 * time.Second), // Later edit
			ClientID:   "client-b",
			VersionNum: 1,
		}

		// Store events in their respective stores
		versionA := cursor.IntegerCursor{Seq: uint64(i*10 + 1)}
		err = storeA.Store(ctx, eventA, versionA)
		if err != nil {
			log.Printf("Failed to store event A: %v", err)
			continue
		}

		versionB := cursor.IntegerCursor{Seq: uint64(i*10 + 2)}
		err = storeB.Store(ctx, eventB, versionB)
		if err != nil {
			log.Printf("Failed to store event B: %v", err)
			continue
		}

		fmt.Println("\n📅 Conflict Scenario:")
		fmt.Printf("  🅰️  Client A: '%s' by %s at %s\n", 
			eventA.Content[:50]+"...", eventA.AuthorName, eventA.EditTime.Format("15:04:05"))
		fmt.Printf("  🅱️  Client B: '%s' by %s at %s\n",
			eventB.Content[:50]+"...", eventB.AuthorName, eventB.EditTime.Format("15:04:05"))

		// Now simulate sync where conflicts need to be resolved
		fmt.Println("\n🔄 Performing sync operations...")
		
		// Sync client A (this will work fine)
		resultA, err := mgrA.Sync(ctx)
		if err != nil {
			log.Printf("Sync A failed: %v", err)
		} else {
			fmt.Printf("  🅰️  Client A sync: %d pushed, %d pulled, %d conflicts resolved\n",
				resultA.EventsPushed, resultA.EventsPulled, resultA.ConflictsResolved)
		}

		// Sync client B (this may encounter conflicts)
		resultB, err := mgrB.Sync(ctx)
		if err != nil {
			log.Printf("Sync B failed: %v", err)
		} else {
			fmt.Printf("  🅱️  Client B sync: %d pushed, %d pulled, %d conflicts resolved\n",
				resultB.EventsPushed, resultB.EventsPulled, resultB.ConflictsResolved)
		}

		// Show what was resolved
		fmt.Println("\n✅ Resolution Results:")
		
		// Load events from both stores to see final state
		zeroVersion := cursor.IntegerCursor{Seq: 0}
		
		eventsA, err := storeA.LoadByAggregate(ctx, "shared-doc-001", zeroVersion)
		if err == nil {
			fmt.Printf("  📄 Client A final state (%d events):\n", len(eventsA))
			for _, ev := range eventsA {
				if docEvent, ok := ev.Event.Data().(*DocumentEvent); ok {
					fmt.Printf("    📝 %s: '%s' by %s\n", 
						ev.Version.String(), docEvent.Content[:40]+"...", docEvent.AuthorName)
				}
			}
		}

		eventsB, err := storeB.LoadByAggregate(ctx, "shared-doc-001", zeroVersion)
		if err == nil {
			fmt.Printf("  📄 Client B final state (%d events):\n", len(eventsB))
			for _, ev := range eventsB {
				if docEvent, ok := ev.Event.Data().(*DocumentEvent); ok {
					fmt.Printf("    📝 %s: '%s' by %s\n",
						ev.Version.String(), docEvent.Content[:40]+"...", docEvent.AuthorName)
				}
			}
		}

		// Explanation of what happened
		fmt.Println("\n💡 What happened:")
		switch strategy.name {
		case "Last-Write-Wins":
			fmt.Println("  • LWW selected Bob's edit (later timestamp)")
		case "First-Write-Wins":
			fmt.Println("  • FWW selected Alice's edit (encountered first)")  
		case "Additive Merge":
			fmt.Println("  • Additive merge combined both edits if possible")
		case "Custom Resolver":
			fmt.Println("  • Custom resolver used latest author logic")
		}

		time.Sleep(100 * time.Millisecond) // Brief pause for readability
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Println("🎉 Conflict Resolution Demo Complete!")
	fmt.Println("\n💡 Key Takeaways:")
	fmt.Println("   • Different strategies handle conflicts in different ways")
	fmt.Println("   • LWW: Uses timestamps to determine winner")
	fmt.Println("   • FWW: First encountered event wins")
	fmt.Println("   • Additive: Tries to merge events when possible")
	fmt.Println("   • Custom: Implement your own business logic")
	fmt.Println("   • Conflicts are automatically detected by aggregate ID")
	fmt.Println("   • Resolution happens transparently during sync")
}
