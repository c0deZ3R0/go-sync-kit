package utils

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// CursorBasedSync demonstrates how to set up cursor-based synchronization
func CursorBasedSync() {
	// Create an event store (using memory store for example)
	store := NewMemoryEventStore()

	// Create some test events
	testEvents := []struct {
		id          string
		eventType   string
		aggregateID string
		data        interface{}
	}{
		{"1", "UserCreated", "user1", map[string]string{"name": "Alice"}},
		{"2", "UserUpdated", "user1", map[string]string{"name": "Alice Smith"}},
		{"3", "UserCreated", "user2", map[string]string{"name": "Bob"}},
	}

	// Store test events
	for _, e := range testEvents {
		event := NewMockEvent(e.id, e.eventType, e.aggregateID, e.data)
		version := NewMockVersion(time.Now())
		if err := store.Store(context.Background(), event, version); err != nil {
			log.Printf("Failed to store test event: %v", err)
		}
	}

	// Load last cursor from disk if it exists
	var last cursor.Cursor
	if data, err := ioutil.ReadFile("last_cursor.json"); err == nil {
		// First unmarshal into a map to check the type
		var cursorData map[string]interface{}
		if err := json.Unmarshal(data, &cursorData); err != nil {
			log.Printf("Failed to unmarshal cursor data: %v", err)
		} else {
			// Create appropriate cursor type based on type field
			if cursorType, ok := cursorData["type"].(string); ok {
				switch cursorType {
				case "mock":
					var mc mockCursor
					if err := json.Unmarshal(data, &mc); err != nil {
						log.Printf("Failed to unmarshal mock cursor: %v", err)
					} else {
						last = mc
					}
				default:
					log.Printf("Unknown cursor type: %s", cursorType)
				}
			}
		}
	}

	// Configure sync options with cursor support
	opts := &synckit.SyncOptions{
		BatchSize: 200,
		LastCursorLoader: func() cursor.Cursor {
			return last
		},
		CursorSaver: func(c cursor.Cursor) error {
			last = c
			log.Printf("Saving cursor: %v", c)
			// Persist cursor to disk
			data, err := json.Marshal(c)
			if err != nil {
				return err
			}
			return ioutil.WriteFile("last_cursor.json", data, 0644)
		},
		// Configure other options as needed
		SyncInterval:      time.Second * 30,
		EnableValidation:  true,
		EnableCompression: true,
		Timeout:           time.Second * 10,
	}

	// Create HTTP transport with cursor support
	// Note: We'll need to implement our own transport
	//t := http.NewTransport("http://localhost:8080", nil)

	// For now, let's use a mock transport that simulates a server
	tm := NewMockTransport()
	t, _ := tm.(*MockTransport)

	// Simulate some events already on the server
	serverEvents := []struct {
		id          string
		eventType   string
		aggregateID string
		data        interface{}
	}{
		{"4", "UserCreated", "user3", map[string]string{"name": "Charlie"}},
		{"5", "UserCreated", "user4", map[string]string{"name": "Dave"}},
	}

	// Store server events in the mock transport
	for _, e := range serverEvents {
		event := NewMockEvent(e.id, e.eventType, e.aggregateID, e.data)
		version := NewMockVersion(time.Now())
		t.events = append(t.events, synckit.EventWithVersion{
			Event:   event,
			Version: version,
		})
	}

	// Create sync manager
	sm := synckit.NewSyncManager(store, t, opts, logging.Default().Logger)
	defer sm.Close()

	// Run initial sync
	ctx := context.Background()
	result, err := sm.Sync(ctx)
	if err != nil {
		log.Printf("Initial sync failed: %v", err)
		return
	}

	log.Printf("Initial sync complete: pulled %d events, pushed %d events",
		result.EventsPulled, result.EventsPushed)

	// Start automatic sync
	if err := sm.StartAutoSync(ctx); err != nil {
		log.Printf("Failed to start auto sync: %v", err)
		return
	}

	// Let it run for a short while
	log.Println("Running auto sync for 10 seconds...")
	time.Sleep(time.Second * 10)

	// Stop auto sync
	if err := sm.StopAutoSync(); err != nil {
		log.Printf("Failed to stop auto sync: %v", err)
	}
}

// Example usage:
func main() {
	CursorBasedSync()
}
