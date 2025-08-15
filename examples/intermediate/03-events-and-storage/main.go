// Example 3: Event Creation and Storage
// 
// This example demonstrates:
// - Creating custom events with proper Event interface
// - Storing events locally with versioning
// - Retrieving events from storage
// - Using the new functional options API

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// UserEvent represents a custom event type
type UserEvent struct {
	EventID     string                 `json:"id"`
	EventType   string                 `json:"event_type"`
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	EventMetadata map[string]interface{} `json:"metadata,omitempty"`
}

// Implement the Event interface
func (e *UserEvent) ID() string {
	return e.EventID
}

func (e *UserEvent) Type() string {
	return e.EventType
}

func (e *UserEvent) AggregateID() string {
	return e.UserID
}

func (e *UserEvent) Data() interface{} {
	return e
}

func (e *UserEvent) Metadata() map[string]interface{} {
	return e.EventMetadata
}

func main() {
	fmt.Println("=== Go Sync Kit Example 3: Event Creation and Storage ===\n")

	// Create SQLite store
	store, err := sqlite.NewWithDataSource("events-simple.db")
	if err != nil {
		log.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Create sync manager with null transport (local-only)
	mgr, err := synckit.NewManager(
		synckit.WithStore(store),
		synckit.WithNullTransport(), // No network - local only
		synckit.WithLWW(),
	)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	fmt.Println("📝 Creating and storing events...")

	// Create sample events
	events := []*UserEvent{
		{
			EventID:   "evt-001",
			EventType: "user.registered",
			UserID:    "user-001", 
			Username:  "alice",
			Email:     "alice@example.com",
			Timestamp: time.Now(),
			EventMetadata: map[string]interface{}{
				"source": "web",
				"plan":   "free",
			},
		},
		{
			EventID:   "evt-002", 
			EventType: "user.updated",
			UserID:    "user-001",
			Username:  "alice_updated",
			Email:     "alice.new@example.com",
			Timestamp: time.Now().Add(1 * time.Minute),
			EventMetadata: map[string]interface{}{
				"source": "api",
				"fields": []string{"username", "email"},
			},
		},
	}

	// Store events with sequential versions
	fmt.Println("\n💾 Storing events:")
	for i, event := range events {
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		
		err := store.Store(ctx, event, version)
		if err != nil {
			log.Printf("Failed to store event %d: %v", i+1, err)
			continue
		}

		fmt.Printf("  ✅ Stored %s (v%d) - %s\n", 
			event.EventType, i+1, event.Username)

		// Show event data
		dataBytes, _ := json.MarshalIndent(event, "     ", "  ")
		fmt.Printf("     📄 Data: %s\n", string(dataBytes))
		fmt.Println()
	}

	// Retrieve events from storage
	fmt.Println("📚 Retrieving events from storage...")
	
	// Load all events since version 0
	zeroVersion := cursor.IntegerCursor{Seq: 0}
	storedEvents, err := store.Load(ctx, zeroVersion)
	if err != nil {
		log.Fatalf("Failed to load events: %v", err)
	}

	fmt.Printf("\n📊 Retrieved %d events:\n", len(storedEvents))
	for _, ev := range storedEvents {
		fmt.Printf("  📝 Version %s - Type: %s - Aggregate: %s\n",
			ev.Version.String(),
			ev.Event.Type(),
			ev.Event.AggregateID())
		
		// Show stored event data 
		evData, _ := json.MarshalIndent(ev.Event.Data(), "     ", "  ")
		fmt.Printf("     📄 Data: %s\n", string(evData))
		fmt.Println()
	}

	// Get latest version
	latestVersion, err := store.LatestVersion(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest version: %v", err)
	}

	fmt.Printf("📈 Latest version: %s\n", latestVersion.String())

	// Perform sync (will be no-op with null transport)
	fmt.Println("\n🔄 Performing sync...")
	result, err := mgr.Sync(ctx)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	fmt.Printf("✅ Sync completed!\n")
	fmt.Printf("   Duration: %v\n", result.Duration)
	fmt.Printf("   Events pushed: %d\n", result.EventsPushed) 
	fmt.Printf("   Events pulled: %d\n", result.EventsPulled)

	fmt.Println("\n🎉 Example completed! Check 'events-simple.db' to see stored events.")
	fmt.Println("\n💡 Key Takeaways:")
	fmt.Println("   • Custom events implement the Event interface") 
	fmt.Println("   • Events are stored with sequential version numbers")
	fmt.Println("   • You can retrieve all events or query by version range")
	fmt.Println("   • Sync operations work with local-only (null transport) setup")
}
