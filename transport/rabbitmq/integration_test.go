package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// MockEvent implements synckit.Event for testing
type MockEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        map[string]interface{}
	metadata    map[string]interface{}
}

func (e MockEvent) ID() string                            { return e.id }
func (e MockEvent) Type() string                          { return e.eventType }
func (e MockEvent) AggregateID() string                   { return e.aggregateID }
func (e MockEvent) Data() interface{}                     { return e.data }
func (e MockEvent) Metadata() map[string]interface{}      { return e.metadata }
func (e MockEvent) MarshalJSON() ([]byte, error)          { return json.Marshal(e.data) }
func (e *MockEvent) UnmarshalJSON(data []byte) error      { return json.Unmarshal(data, &e.data) }

// MockVersion implements synckit.Version
type MockVersion struct {
	value int64
}

func (v MockVersion) String() string                  { return fmt.Sprintf("%d", v.value) }
func (v MockVersion) Compare(other synckit.Version) int {
	if otherMock, ok := other.(MockVersion); ok {
		if v.value < otherMock.value {
			return -1
		} else if v.value > otherMock.value {
			return 1
		}
		return 0
	}
	return -1
}
func (v MockVersion) IsZero() bool                    { return v.value == 0 }

// Test configuration
func getTestConfig() *Config {
	// Check if running in Docker environment
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://synckit_user:synckit_pass@localhost:5672/"
	}

	cfg := DefaultConfig()
	cfg.URL = url
	cfg.Exchange = "test-synckit-events"
	cfg.QueueName = "test-synckit-queue"
	cfg.BindingKeys = []string{"events.#"}
	cfg.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	cfg.RoutingKey = func(event synckit.Event) string {
		return fmt.Sprintf("events.%s", event.Type())
	}

	return cfg
}

// skipIfNoRabbitMQ skips the test if RabbitMQ is not available
func skipIfNoRabbitMQ(t *testing.T) {
	cfg := getTestConfig()
	transport := NewTransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Skipf("RabbitMQ not available: %v", err)
	}
	transport.Close()
}

func TestIntegration_ConnectAndClose(t *testing.T) {
	skipIfNoRabbitMQ(t)

	cfg := getTestConfig()
	transport := NewTransport(cfg)

	ctx := context.Background()

	// Test Connect
	err := transport.Connect(ctx)
	require.NoError(t, err)

	// Test Close
	err = transport.Close()
	assert.NoError(t, err)

	// Test double close is safe
	err = transport.Close()
	assert.NoError(t, err)
}

func TestIntegration_PushAndSubscribe(t *testing.T) {
	skipIfNoRabbitMQ(t)

	cfg := getTestConfig()
	cfg.QueueName = fmt.Sprintf("test-push-subscribe-%d", time.Now().UnixNano())

	// Create publisher transport
	publisher := NewTransport(cfg)
	ctx := context.Background()

	err := publisher.Connect(ctx)
	require.NoError(t, err)
	defer publisher.Close()

	// Create subscriber transport
	subscriber := NewTransport(cfg)
	err = subscriber.Connect(ctx)
	require.NoError(t, err)
	defer subscriber.Close()

	// Setup test data
	events := []synckit.EventWithVersion{
		{
			Event:   MockEvent{
				id:          "event-1", 
				eventType:   "user.created", 
				aggregateID: "user-123",
				data:        map[string]interface{}{"name": "John"},
				metadata:    map[string]interface{}{"source": "test"},
			},
			Version: MockVersion{value: 1},
		},
		{
			Event:   MockEvent{
				id:          "event-2", 
				eventType:   "user.updated", 
				aggregateID: "user-123",
				data:        map[string]interface{}{"name": "Jane"},
				metadata:    map[string]interface{}{"source": "test"},
			},
			Version: MockVersion{value: 2},
		},
	}

	// Channel to collect received events
	receivedEvents := make(chan []synckit.EventWithVersion, 10)
	var receivedMu sync.Mutex
	var allReceived []synckit.EventWithVersion

	// Handler function
	handler := func(events []synckit.EventWithVersion) error {
		receivedMu.Lock()
		allReceived = append(allReceived, events...)
		receivedMu.Unlock()
		
		receivedEvents <- events
		return nil
	}

	// Start subscriber
	err = subscriber.Subscribe(ctx, handler)
	require.NoError(t, err)

	// Give subscriber time to setup
	time.Sleep(100 * time.Millisecond)

	// Push events
	err = publisher.Push(ctx, events)
	require.NoError(t, err)

	// Wait for events to be received
	timeout := time.After(5 * time.Second)
	receivedCount := 0

	for receivedCount < len(events) {
		select {
		case received := <-receivedEvents:
			receivedCount += len(received)
			t.Logf("Received %d events, total: %d", len(received), receivedCount)
		case <-timeout:
			t.Fatalf("Timeout waiting for events. Received: %d, Expected: %d", receivedCount, len(events))
		}
	}

	// Verify received events
	receivedMu.Lock()
	defer receivedMu.Unlock()

	assert.Equal(t, len(events), len(allReceived))
	
	// Create maps for easy comparison (order may vary)
	expectedMap := make(map[string]synckit.EventWithVersion)
	for _, event := range events {
		expectedMap[event.Event.ID()] = event
	}

	receivedMap := make(map[string]synckit.EventWithVersion)
	for _, event := range allReceived {
		receivedMap[event.Event.ID()] = event
	}

	for id, expected := range expectedMap {
		received, exists := receivedMap[id]
		assert.True(t, exists, "Event %s not received", id)
		if exists {
			assert.Equal(t, expected.Event.ID(), received.Event.ID())
			assert.Equal(t, expected.Event.Type(), received.Event.Type())
			assert.Equal(t, expected.Version.String(), received.Version.String())
		}
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	skipIfNoRabbitMQ(t)

	cfg := getTestConfig()
	cfg.QueueName = fmt.Sprintf("test-error-handling-%d", time.Now().UnixNano())

	transport := NewTransport(cfg)
	ctx := context.Background()

	// Test push without connection
	events := []synckit.EventWithVersion{
		{Event: MockEvent{
			id:          "test", 
			eventType:   "test",
			aggregateID: "agg-test", 
			data:        map[string]interface{}{},
		}},
	}
	
	err := transport.Push(ctx, events)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Test subscribe without connection
	handler := func([]synckit.EventWithVersion) error { return nil }
	err = transport.Subscribe(ctx, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Connect and test operations
	err = transport.Connect(ctx)
	require.NoError(t, err)
	defer transport.Close()

	// Test push after connection
	err = transport.Push(ctx, events)
	assert.NoError(t, err)

	// Test subscribe after connection
	err = transport.Subscribe(ctx, handler)
	assert.NoError(t, err)
}

func TestIntegration_MultiplePublishers(t *testing.T) {
	skipIfNoRabbitMQ(t)

	cfg := getTestConfig()
	cfg.QueueName = fmt.Sprintf("test-multi-pub-%d", time.Now().UnixNano())

	// Create subscriber
	subscriber := NewTransport(cfg)
	ctx := context.Background()
	err := subscriber.Connect(ctx)
	require.NoError(t, err)
	defer subscriber.Close()

	// Collect received events
	var receivedMu sync.Mutex
	var allReceived []synckit.EventWithVersion
	receivedEvents := make(chan []synckit.EventWithVersion, 100)

	handler := func(events []synckit.EventWithVersion) error {
		receivedMu.Lock()
		allReceived = append(allReceived, events...)
		receivedMu.Unlock()
		receivedEvents <- events
		return nil
	}

	err = subscriber.Subscribe(ctx, handler)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Setup time

	// Create multiple publishers
	const numPublishers = 3
	const eventsPerPublisher = 5
	
	var publisherWg sync.WaitGroup
	publisherWg.Add(numPublishers)

	// Start publishers concurrently
	for i := 0; i < numPublishers; i++ {
		go func(publisherId int) {
			defer publisherWg.Done()

			publisher := NewTransport(cfg)
			err := publisher.Connect(ctx)
			if err != nil {
				t.Errorf("Publisher %d failed to connect: %v", publisherId, err)
				return
			}
			defer publisher.Close()

			// Publish events
			for j := 0; j < eventsPerPublisher; j++ {
				events := []synckit.EventWithVersion{
					{
						Event: MockEvent{
							id:          fmt.Sprintf("publisher-%d-event-%d", publisherId, j),
							eventType:   "test.multi",
							aggregateID: fmt.Sprintf("agg-%d", publisherId),
							data:        map[string]interface{}{"publisher": publisherId, "sequence": j},
							metadata:    map[string]interface{}{"source": "integration-test"},
						},
						Version: MockVersion{value: int64(publisherId*100 + j)},
					},
				}

				if err := publisher.Push(ctx, events); err != nil {
					t.Errorf("Publisher %d failed to push event %d: %v", publisherId, j, err)
				}
			}
		}(i)
	}

	publisherWg.Wait()

	// Wait for all events to be received
	expectedTotal := numPublishers * eventsPerPublisher
	timeout := time.After(10 * time.Second)
	receivedCount := 0

	for receivedCount < expectedTotal {
		select {
		case received := <-receivedEvents:
			receivedCount += len(received)
			t.Logf("Received batch of %d events, total: %d/%d", len(received), receivedCount, expectedTotal)
		case <-timeout:
			t.Fatalf("Timeout waiting for all events. Received: %d, Expected: %d", receivedCount, expectedTotal)
		}
	}

	// Verify all events received
	receivedMu.Lock()
	defer receivedMu.Unlock()

	assert.Equal(t, expectedTotal, len(allReceived))

	// Verify events from each publisher
	publisherCounts := make(map[int]int)
	for _, event := range allReceived {
		if data, ok := event.Event.Data().(map[string]interface{}); ok {
			if publisherId, ok := data["publisher"].(float64); ok { // JSON unmarshals numbers as float64
				publisherCounts[int(publisherId)]++
			}
		}
	}

	for i := 0; i < numPublishers; i++ {
		assert.Equal(t, eventsPerPublisher, publisherCounts[i], 
			"Publisher %d should have %d events", i, eventsPerPublisher)
	}
}

func TestIntegration_HandlerError(t *testing.T) {
	skipIfNoRabbitMQ(t)

	cfg := getTestConfig()
	cfg.QueueName = fmt.Sprintf("test-handler-error-%d", time.Now().UnixNano())

	publisher := NewTransport(cfg)
	subscriber := NewTransport(cfg)

	ctx := context.Background()

	err := publisher.Connect(ctx)
	require.NoError(t, err)
	defer publisher.Close()

	err = subscriber.Connect(ctx)
	require.NoError(t, err)
	defer subscriber.Close()

	// Setup handler that fails initially then succeeds
	attemptCount := 0
	var mu sync.Mutex
	successEvents := make(chan []synckit.EventWithVersion, 10)

	handler := func(events []synckit.EventWithVersion) error {
		mu.Lock()
		attemptCount++
		currentAttempt := attemptCount
		mu.Unlock()

		// Fail first 2 attempts, succeed on 3rd
		if currentAttempt <= 2 {
			return fmt.Errorf("simulated handler error (attempt %d)", currentAttempt)
		}

		successEvents <- events
		return nil
	}

	err = subscriber.Subscribe(ctx, handler)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Push an event
	events := []synckit.EventWithVersion{
		{
			Event: MockEvent{
				id:          "error-test",
				eventType:   "error.test",
				aggregateID: "agg-error",
				data:        map[string]interface{}{"test": true},
				metadata:    map[string]interface{}{"source": "error-test"},
			},
			Version: MockVersion{value: 1},
		},
	}

	err = publisher.Push(ctx, events)
	require.NoError(t, err)

	// Wait for successful processing (with retries)
	timeout := time.After(10 * time.Second)
	select {
	case received := <-successEvents:
		assert.Len(t, received, 1)
		assert.Equal(t, "error-test", received[0].Event.ID())
		
		// Verify multiple attempts were made
		mu.Lock()
		finalAttemptCount := attemptCount
		mu.Unlock()
		assert.GreaterOrEqual(t, finalAttemptCount, 3, "Handler should have been called at least 3 times")
		
	case <-timeout:
		mu.Lock()
		finalAttemptCount := attemptCount
		mu.Unlock()
		t.Fatalf("Timeout waiting for successful event processing. Attempts made: %d", finalAttemptCount)
	}
}
