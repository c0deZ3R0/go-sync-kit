package main

import (
    "context"
    "fmt"
    "log"
    "math/rand"
    "net/http"
    "os"
    "time"

    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
    transport "github.com/c0deZ3R0/go-sync-kit/transport/http"
)

func main() {
    // Create SQLite event store
    storeConfig := &sqlite.Config{DataSourceName: "file:notes.db", EnableWAL: true}
    store, err := sqlite.New(storeConfig)
    if err != nil {
        log.Fatalf("Failed to create SQLite store: %v", err)
    }
    defer store.Close()

    // Set up HTTP server with SyncHandler
    logger := log.New(os.Stdout, "[SyncHandler] ", log.LstdFlags)
    handler := transport.NewSyncHandler(store, logger)
    server := &http.Server{Addr: ":8080", Handler: handler}

    go func() {
        log.Println("Starting sync server on :8080")
        if err := server.ListenAndServe(); err != nil {
            log.Fatalf("Server error: %v", err)
        }
    }()

    // Start generating server-side events
    go generateServerEvents(store)

    // Keep server running
    select {}
}

// ServerEvent implements the Event interface for server-generated events
type ServerEvent struct {
    eventID      string
    eventType    string
    noteID       string
    content      string
    timestamp    time.Time
    eventMetadata map[string]interface{}
}

func (e *ServerEvent) ID() string { return e.eventID }
func (e *ServerEvent) Type() string { return e.eventType }
func (e *ServerEvent) AggregateID() string { return e.noteID }
func (e *ServerEvent) Data() interface{} { return e.content }
func (e *ServerEvent) Metadata() map[string]interface{} { return e.eventMetadata }

// generateServerEvents creates new events every few seconds
func generateServerEvents(store *sqlite.SQLiteEventStore) {
    ctx := context.Background()
    ticker := time.NewTicker(10 * time.Second) // Generate event every 10 seconds
    defer ticker.Stop()

    // List of random content to choose from
    contents := []string{
        "Important server update!",
        "Breaking news from server",
        "Server notification",
        "System status update",
        "Maintenance notification",
    }

    noteCount := 1
    for range ticker.C {
        // Create a new event
        event := &ServerEvent{
            eventID:       fmt.Sprintf("server_evt_%d_%d", time.Now().UnixNano(), noteCount),
            eventType:     "server_notification",
            noteID:        fmt.Sprintf("server_note_%d", noteCount),
            content:       contents[rand.Intn(len(contents))],
            timestamp:     time.Now(),
            eventMetadata: map[string]interface{}{
                "author": "server",
                "priority": rand.Intn(5) + 1,
            },
        }

        // Store the event
        if err := store.Store(ctx, event, nil); err != nil {
            log.Printf("Failed to store server event: %v", err)
            continue
        }

        log.Printf("Server generated new event: %s (Note ID: %s)", event.content, event.noteID)
        noteCount++
    }
}
