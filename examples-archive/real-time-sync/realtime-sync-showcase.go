package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// DocumentEvent represents collaborative document changes
type DocumentEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        DocumentData
	metadata    map[string]interface{}
	timestamp   time.Time
}

type DocumentData struct {
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Section   string            `json:"section"` // "introduction", "body", "conclusion"
	Author    string            `json:"author"`
	Version   int               `json:"version"`
	Tags      []string          `json:"tags"`
	Status    string            `json:"status"` // "draft", "review", "published"
	Metadata  map[string]string `json:"metadata"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (e *DocumentEvent) ID() string                       { return e.id }
func (e *DocumentEvent) Type() string                     { return e.eventType }
func (e *DocumentEvent) AggregateID() string              { return e.aggregateID }
func (e *DocumentEvent) Data() interface{}                { return e.data }
func (e *DocumentEvent) Metadata() map[string]interface{} { return e.metadata }

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	demo, err := NewOfflineFirstDemo(logger)
	if err != nil {
		logger.Error("Failed to create demo", "error", err)
		os.Exit(1)
	}
	defer demo.Cleanup()

	// Run the demo scenarios
	fmt.Println("🚀 Starting Offline-First Dynamic Conflict Resolution Demo")
	fmt.Println("=" * 60)

	if err := demo.RunScenarios(); err != nil {
		logger.Error("Demo failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n🎉 Demo completed successfully!")
}
