package main

import (
	"context"
	"fmt"
	"log"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// MyEvent is a simple domain event implementing the Event interface
type MyEvent struct {
	EventID   string
	EventType string
	UserID    string
	Name      string
}

func (e MyEvent) ID() string                      { return e.EventID }
func (e MyEvent) Type() string                    { return e.EventType }
func (e MyEvent) AggregateID() string             { return e.UserID }
func (e MyEvent) Data() interface{}               { return e }
func (e MyEvent) Metadata() map[string]interface{} { return nil }

func main() {
	ctx := context.Background()

	// In-memory store (durable stores like SQLite or Postgres can also be used)
	store := memstore.New()

	// Create a SyncNode with default resolver and null transport (local-only)
	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithNullTransport(), // Local-only, no network sync
		synckit.WithLWW(),           // Last-Write-Wins conflict resolution
	)
	if err != nil {
		log.Fatalf("create node: %v", err)
	}
	defer node.Close()

	// Create and store a local event
	event := MyEvent{
		EventID:   "1",
		EventType: "demo",
		UserID:    "user-123",
		Name:      "demo event",
	}

	// Store the event with a version (memstore auto-generates versions)
	version := cursor.IntegerCursor{Seq: 1}
	if err := store.Store(ctx, event, version); err != nil {
		log.Fatalf("store: %v", err)
	}

	fmt.Printf("📝 Stored event: %s (type: %s, user: %s)\n", 
		event.Name, event.Type(), event.AggregateID())

	// Perform a one-shot sync round (pull → resolve → push)
	res, err := node.Sync(ctx)
	if err != nil {
		log.Fatalf("sync: %v", err)
	}

	fmt.Printf("✅ Sync complete: EventsPushed=%d, EventsPulled=%d, ConflictsResolved=%d\n",
		res.EventsPushed, res.EventsPulled, res.ConflictsResolved)
}