package sse

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// ExampleEvent implements synckit.Event for testing
type ExampleEvent struct {
	IDValue         string                 `json:"id"`
	TypeValue       string                 `json:"type"`
	AggregateIDValue string                `json:"aggregate_id"`
	DataValue       interface{}            `json:"data"`
	MetadataValue   map[string]interface{} `json:"metadata"`
}

func (e ExampleEvent) ID() string                            { return e.IDValue }
func (e ExampleEvent) Type() string                          { return e.TypeValue }
func (e ExampleEvent) AggregateID() string                   { return e.AggregateIDValue }
func (e ExampleEvent) Data() interface{}                     { return e.DataValue }
func (e ExampleEvent) Metadata() map[string]interface{}      { return e.MetadataValue }

// MockEventStore implements synckit.EventStore for testing
type MockEventStore struct {
	events []synckit.EventWithVersion
}

func (m *MockEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	m.events = append(m.events, synckit.EventWithVersion{Event: event, Version: version})
	return nil
}

func (m *MockEventStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// For simplicity, return all events after the cursor
	var result []synckit.EventWithVersion
	var sinceSeq uint64 = 0
	if since != nil {
		if ic, ok := since.(cursor.IntegerCursor); ok {
			sinceSeq = ic.Seq
		}
	}
	
	for i, ev := range m.events {
		if ic, ok := ev.Version.(cursor.IntegerCursor); ok && ic.Seq > sinceSeq {
			result = append(result, ev)
		} else if since == nil || i >= int(sinceSeq) {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *MockEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	events, err := m.Load(ctx, since)
	if err != nil {
		return nil, err
	}
	
	var result []synckit.EventWithVersion
	for _, ev := range events {
		if ev.Event.AggregateID() == aggregateID {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *MockEventStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	if len(m.events) == 0 {
		return cursor.NewInteger(0), nil
	}
	return m.events[len(m.events)-1].Version, nil
}

func (m *MockEventStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	// Simple integer parsing for mock
	return cursor.NewInteger(0), nil
}

func (m *MockEventStore) Close() error {
	return nil
}

func ExampleTransport() {
	// Initialize cursor codecs
	cursor.InitDefaultCodecs()
	
	// Create mock store with some test events
	store := &MockEventStore{}
	store.Store(context.Background(), ExampleEvent{
		IDValue:         "event-1",
		TypeValue:       "UserCreated",
		AggregateIDValue: "user-123",
		DataValue:       map[string]interface{}{"name": "John Doe"},
	}, cursor.NewInteger(1))
	
	store.Store(context.Background(), ExampleEvent{
		IDValue:         "event-2", 
		TypeValue:       "UserUpdated",
		AggregateIDValue: "user-123",
		DataValue:       map[string]interface{}{"name": "John Smith"},
	}, cursor.NewInteger(2))

	// Create SSE server
	server := NewServer(store, log.New(os.Stdout, "SSE: ", log.LstdFlags))
	
	// Create test HTTP server
	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()
	
	// Create SSE client
	client := NewClient(testServer.URL, nil)
	
	// Subscribe to events
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	eventCount := 0
	err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
		for _, ev := range events {
			fmt.Printf("Received event: %s - %s\n", ev.Event.Type(), ev.Event.ID())
			eventCount++
		}
		return nil
	})
	
	if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") && err.Error() != "context canceled" {
		fmt.Printf("Subscribe error: %v\n", err)
	}
	
	fmt.Printf("Total events received: %d\n", eventCount)
	// Output: 
	// Received event: UserCreated - event-1
	// Received event: UserUpdated - event-2
	// Total events received: 2
}

func TestSSEBasicIntegration(t *testing.T) {
	// Initialize cursor codecs
	cursor.InitDefaultCodecs()
	
	// Create mock store
	store := &MockEventStore{}
	
	// Create SSE server
	server := NewServer(store, nil)
	
	// Test that server can be created
	if server == nil {
		t.Fatal("Failed to create SSE server")
	}
	
	if server.BatchSize != 100 {
		t.Errorf("Expected default batch size 100, got %d", server.BatchSize)
	}
	
	// Test client creation
	client := NewClient("http://localhost:8080", nil)
	if client == nil {
		t.Fatal("Failed to create SSE client")
	}
	
	if client.BaseURL != "http://localhost:8080" {
		t.Errorf("Expected BaseURL 'http://localhost:8080', got '%s'", client.BaseURL)
	}
}
