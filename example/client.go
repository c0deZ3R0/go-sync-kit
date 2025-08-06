package main

import (
    "context"
    "fmt"
    "log"
    "math/rand"
    "time"

    "github.com/c0deZ3R0/go-sync-kit"
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    transport "github.com/c0deZ3R0/go-sync-kit/transport/http"
)

// NoteEvent represents a change to a note
type NoteEvent struct {
    eventID      string
    eventType    string
    noteID       string
    content      string
    timestamp    time.Time
    eventMetadata map[string]interface{}
}

// Implement sync.Event interface
func (e *NoteEvent) ID() string { return e.eventID }
func (e *NoteEvent) Type() string { return e.eventType }
func (e *NoteEvent) AggregateID() string { return e.noteID }
func (e *NoteEvent) Data() interface{} { return e.content }
func (e *NoteEvent) Metadata() map[string]interface{} { return e.eventMetadata }

func main() {
    // Create SQLite store for this client
    storeConfig := &sqlite.Config{
        DataSourceName: "file:notes_client.db",
        EnableWAL:     true,
    }
    store, err := sqlite.New(storeConfig)
    if err != nil {
        log.Fatalf("Failed to create store: %v", err)
    }
    defer store.Close()

    // Create HTTP transport client
    clientTransport := transport.NewTransport("http://localhost:8080", nil)

    // Configure sync options
    syncOptions := &sync.SyncOptions{
        BatchSize:     10,
        SyncInterval: 5 * time.Second,
        // Use last-write-wins conflict resolution
        ConflictResolver: &LastWriteWinsResolver{},
    }

    // Create sync manager
    syncManager := sync.NewSyncManager(store, clientTransport, syncOptions)

    // Subscribe to sync events
    syncManager.Subscribe(func(result *sync.SyncResult) {
        log.Printf("Sync completed: %d pushed, %d pulled", 
            result.EventsPushed, result.EventsPulled)
    })

    // Start automatic sync
    ctx := context.Background()
    if err := syncManager.StartAutoSync(ctx); err != nil {
        log.Fatalf("Failed to start auto sync: %v", err)
    }

    // Example: Create a new note
    noteID := "note1"
    event := &NoteEvent{
    eventID:       fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), "create"),
        eventType:     "note_created",
        noteID:        noteID,
        content:       "Hello, this is a test note!",
        timestamp:     time.Now(),
        eventMetadata: map[string]interface{}{
            "author": "client1",
        },
    }

    // Store the event locally (it will be synced automatically)
    if err := store.Store(ctx, event, nil); err != nil {
        log.Printf("Failed to store event: %v", err)
    }

    // Example: Update the note
    updateEvent := &NoteEvent{
    eventID:       fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), "update"),
        eventType:     "note_updated",
        noteID:        noteID,
        content:       "Updated content for the test note!",
        timestamp:     time.Now(),
        eventMetadata: map[string]interface{}{
            "author": "client1",
        },
    }

    if err := store.Store(ctx, updateEvent, nil); err != nil {
        log.Printf("Failed to store update event: %v", err)
    }

    // Generate events every 10 seconds while disconnected
    go generateClientEvents(store)

    // Keep client running
    select {}
}

// generateClientEvents creates new events every few seconds while server is unavailable
func generateClientEvents(store *sqlite.SQLiteEventStore) {
    ctx := context.Background()
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    contents := []string{
        "Local note created during outage",
        "Offline event added",
        "Draft note created",
        "Note updated offline",
        "Temporary offline note",
    }

    eventCount := 1
    for range ticker.C {
        event := &NoteEvent{
            eventID:       fmt.Sprintf("local_evt_%d_%d", time.Now().UnixNano(), eventCount),
            eventType:     "offline_note",
            noteID:        fmt.Sprintf("local_note_%d", eventCount),
            content:       contents[rand.Intn(len(contents))],
            timestamp:     time.Now(),
            eventMetadata: map[string]interface{}{
                "author": "client",
                "status": "offline",
            },
        }

        // Store the event
        if err := store.Store(ctx, event, nil); err != nil {
            log.Printf("Failed to store local event: %v", err)
            continue
        }

        log.Printf("Local event created: %s (Note ID: %s)", event.content, event.noteID)
        eventCount++
    }
}

// LastWriteWinsResolver implements a simple conflict resolution strategy
type LastWriteWinsResolver struct{}

func (r *LastWriteWinsResolver) Resolve(ctx context.Context, local, remote []sync.EventWithVersion) ([]sync.EventWithVersion, error) {
    if len(local) == 0 {
        return remote, nil
    }
    if len(remote) == 0 {
        return local, nil
    }

    // Combine all events
    all := append([]sync.EventWithVersion{}, local...)
    all = append(all, remote...)

    // Group by noteID (AggregateID)
    noteEvents := make(map[string][]sync.EventWithVersion)
    for _, ev := range all {
        noteID := ev.Event.AggregateID()
        noteEvents[noteID] = append(noteEvents[noteID], ev)
    }

    // For each note, keep only the latest event
    var resolved []sync.EventWithVersion
    for _, events := range noteEvents {
        latest := events[0]
        for _, ev := range events[1:] {
if ev.Event.(*NoteEvent).timestamp.After(latest.Event.(*NoteEvent).timestamp) {
                latest = ev
            }
        }
        resolved = append(resolved, latest)
    }

    return resolved, nil
}
