package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// TestSubscribe_ContextCancellation tests immediate exit on context cancellation
func TestSubscribe_ContextCancellation(t *testing.T) {
	// Create a server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server by sleeping
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Track when the Subscribe method returns
	done := make(chan error, 1)
	go func() {
		err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			t.Error("Handler should not be called when context is cancelled")
			return nil
		})
		done <- err
	}()

	// Subscribe should return quickly due to context cancellation
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Subscribe did not return quickly after context cancellation")
	}
}

// TestSubscribe_ServerReconnection tests reconnection behavior when server drops connection
func TestSubscribe_ServerReconnection(t *testing.T) {
	connectionAttempts := 0
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionAttempts++
		attempt := connectionAttempts
		mu.Unlock()

		if attempt <= 2 {
			// First two attempts fail
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Third attempt succeeds
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send a test event
		eventData := struct {
			Events     []JSONEventWithVersion `json:"events"`
			NextCursor cursor.WireCursor       `json:"next_cursor"`
		}{
			Events: []JSONEventWithVersion{
				{
					Event: JSONEvent{
						ID:          "test-event-1",
						Type:        "TestEvent",
						AggregateID: "aggregate-1",
						Data:        map[string]interface{}{"test": true},
						Metadata:    map[string]interface{}{"version": "1"},
					},
					Version: "1",
				},
			},
		}

		jsonData, _ := json.Marshal(eventData)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)

		// Keep connection alive for a short time then close
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventReceived := make(chan bool, 1)

	go func() {
		err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			if len(events) > 0 && events[0].Event.ID() == "test-event-1" {
				select {
				case eventReceived <- true:
				default:
				}
			}
			return nil
		})
		// Since we cancel context after success, expect context.Canceled
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe returned unexpected error: %v", err)
		}
	}()

	// Wait for the event to be received or timeout
	select {
	case <-eventReceived:
		// Success! Event was received after reconnection
		// Cancel context to stop the Subscribe method
		cancel()
	case <-time.After(1500 * time.Millisecond):
		cancel()
		t.Error("Event was not received after server reconnection")
	}

	// Wait a bit for goroutine to finish
	time.Sleep(100 * time.Millisecond)

	// Verify that multiple connection attempts were made
	mu.Lock()
	finalAttempts := connectionAttempts
	mu.Unlock()

	if finalAttempts < 3 {
		t.Errorf("Expected at least 3 connection attempts, got %d", finalAttempts)
	}
}

// TestSubscribe_ExponentialBackoff tests that backoff delay increases exponentially
func TestSubscribe_ExponentialBackoff(t *testing.T) {
	connectionTimes := []time.Time{}
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionTimes = append(connectionTimes, time.Now())
		mu.Unlock()

		// Always fail to test backoff
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			return nil
		})
	}()

	// Wait for multiple connection attempts
	time.Sleep(1800 * time.Millisecond)

	mu.Lock()
	times := make([]time.Time, len(connectionTimes))
	copy(times, connectionTimes)
	mu.Unlock()

	// We should have at least 3 attempts
	if len(times) < 3 {
		t.Errorf("Expected at least 3 connection attempts, got %d", len(times))
		return
	}

	// Check that delays are increasing (allowing some tolerance for timing variations)
	delay1 := times[1].Sub(times[0])
	delay2 := times[2].Sub(times[1])

	// First delay should be around 250ms, second should be around 500ms
	if delay1 < 200*time.Millisecond || delay1 > 400*time.Millisecond {
		t.Errorf("First delay %v is not in expected range 200-400ms", delay1)
	}

	if delay2 < 400*time.Millisecond || delay2 > 800*time.Millisecond {
		t.Errorf("Second delay %v is not in expected range 400-800ms", delay2)
	}

	// Second delay should be roughly twice the first (allowing 50% tolerance)
	ratio := float64(delay2) / float64(delay1)
	if ratio < 1.5 || ratio > 2.5 {
		t.Errorf("Backoff ratio %v is not in expected range 1.5-2.5", ratio)
	}
}

// TestSubscribe_SuccessfulConnection tests normal operation with successful connection
func TestSubscribe_SuccessfulConnection(t *testing.T) {
	connectionCount := 0
	mu := sync.Mutex{}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		currConnection := connectionCount
		mu.Unlock()
		
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send multiple test events only on the first connection
		if currConnection == 1 {
			for i := 1; i <= 3; i++ {
				eventData := struct {
					Events     []JSONEventWithVersion `json:"events"`
					NextCursor cursor.WireCursor       `json:"next_cursor"`
				}{
					Events: []JSONEventWithVersion{
						{
							Event: JSONEvent{
								ID:          fmt.Sprintf("test-event-%d", i),
								Type:        "TestEvent",
								AggregateID: "aggregate-1",
								Data:        map[string]interface{}{"counter": i},
								Metadata:    map[string]interface{}{"version": fmt.Sprintf("%d", i)},
							},
							Version: fmt.Sprintf("%d", i),
						},
					},
				}

				jsonData, _ := json.Marshal(eventData)
				fmt.Fprintf(w, "data: %s\n\n", jsonData)
				
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				time.Sleep(50 * time.Millisecond)
			}
		} else {
			// On reconnection, just wait and close to avoid infinite events
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receivedEvents := []synckit.EventWithVersion{}
	eventMu := sync.Mutex{}
	expectedEventsReceived := make(chan bool, 1)

	go func() {
		err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			eventMu.Lock()
			receivedEvents = append(receivedEvents, events...)
			// Check if we have received all expected events
			if len(receivedEvents) >= 3 {
				select {
				case expectedEventsReceived <- true:
				default:
				}
			}
			eventMu.Unlock()
			return nil
		})
		// Since we cancel context after success, expect context.Canceled
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe returned unexpected error: %v", err)
		}
	}()

	// Wait for events to be received or timeout
	select {
	case <-expectedEventsReceived:
		// Success! All events received, cancel to stop reconnection
		cancel()
	case <-time.After(1 * time.Second):
		cancel()
		t.Error("Did not receive all expected events in time")
	}

	// Wait a bit for goroutine to finish
	time.Sleep(100 * time.Millisecond)

	eventMu.Lock()
	eventCount := len(receivedEvents)
	finalEvents := make([]synckit.EventWithVersion, len(receivedEvents))
	copy(finalEvents, receivedEvents)
	eventMu.Unlock()

	if eventCount < 3 {
		t.Errorf("Expected at least 3 events, got %d", eventCount)
	}

	// Verify first event details
	if eventCount > 0 {
		firstEvent := finalEvents[0]
		if firstEvent.Event.ID() != "test-event-1" {
			t.Errorf("Expected first event ID 'test-event-1', got '%s'", firstEvent.Event.ID())
		}
		if firstEvent.Event.Type() != "TestEvent" {
			t.Errorf("Expected first event type 'TestEvent', got '%s'", firstEvent.Event.Type())
		}
	}
}

// TestSubscribe_HandlerError tests that handler errors are properly handled
func TestSubscribe_HandlerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		eventData := struct {
			Events     []JSONEventWithVersion `json:"events"`
			NextCursor cursor.WireCursor       `json:"next_cursor"`
		}{
			Events: []JSONEventWithVersion{
				{
					Event: JSONEvent{
						ID:          "test-event",
						Type:        "TestEvent",
						AggregateID: "aggregate-1",
						Data:        map[string]interface{}{"test": true},
					},
					Version: "1",
				},
			},
		}

		jsonData, _ := json.Marshal(eventData)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			return fmt.Errorf("handler error")
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected handler error to be returned")
		}
		if !strings.Contains(err.Error(), "handler") {
			t.Errorf("Expected error to contain 'handler', got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Subscribe did not return after handler error")
	}
}

// TestSubscribe_InvalidJSON tests handling of malformed JSON data
func TestSubscribe_InvalidJSON(t *testing.T) {
	connectionCount := 0
	mu := sync.Mutex{}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		currConnection := connectionCount
		mu.Unlock()
		
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Only send events on first connection
		if currConnection == 1 {
			// Send invalid JSON
			fmt.Fprintf(w, "data: {invalid json}\n\n")
			
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			
			time.Sleep(50 * time.Millisecond)
			
			// Then send valid JSON
			eventData := struct {
				Events     []JSONEventWithVersion `json:"events"`
				NextCursor cursor.WireCursor       `json:"next_cursor"`
			}{
				Events: []JSONEventWithVersion{
					{
						Event: JSONEvent{
							ID:          "valid-event",
							Type:        "TestEvent",
							AggregateID: "aggregate-1",
							Data:        map[string]interface{}{"valid": true},
						},
						Version: "1",
					},
				},
			}
			
			jsonData, _ := json.Marshal(eventData)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	validEventReceived := make(chan bool, 1)

	go func() {
		err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			if len(events) > 0 && events[0].Event.ID() == "valid-event" {
				select {
				case validEventReceived <- true:
				default:
				}
			}
			return nil
		})
		// Since we cancel context after success, expect context.Canceled
		if err != nil && err != context.Canceled {
			t.Errorf("Subscribe returned unexpected error: %v", err)
		}
	}()

	// Wait for valid event or timeout
	select {
	case <-validEventReceived:
		// Success! Cancel to stop reconnection
		cancel()
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Error("Valid event should be received even after invalid JSON")
	}

	// Wait a bit for goroutine to finish
	time.Sleep(100 * time.Millisecond)
}

// TestSubscribe_ReconnectAfterStreamEnd tests reconnection after normal stream end
func TestSubscribe_ReconnectAfterStreamEnd(t *testing.T) {
	connectionCount := 0
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		currentConnection := connectionCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send an event with connection number
		eventData := struct {
			Events     []JSONEventWithVersion `json:"events"`
			NextCursor cursor.WireCursor       `json:"next_cursor"`
		}{
			Events: []JSONEventWithVersion{
				{
					Event: JSONEvent{
						ID:          fmt.Sprintf("connection-%d", currentConnection),
						Type:        "TestEvent",
						AggregateID: "aggregate-1",
						Data:        map[string]interface{}{"connection": currentConnection},
					},
					Version: fmt.Sprintf("%d", currentConnection),
				},
			},
		}

		jsonData, _ := json.Marshal(eventData)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)

		// Close connection after short time to trigger reconnect
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	eventIds := []string{}
	eventMu := sync.Mutex{}

	go func() {
		err := client.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
			eventMu.Lock()
			for _, event := range events {
				eventIds = append(eventIds, event.Event.ID())
			}
			eventMu.Unlock()
			return nil
		})
		// With timeout context, we can get either DeadlineExceeded or Canceled
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("Subscribe returned unexpected error: %v", err)
		}
	}()

	time.Sleep(800 * time.Millisecond)

	eventMu.Lock()
	finalEventIds := make([]string, len(eventIds))
	copy(finalEventIds, eventIds)
	eventMu.Unlock()

	// Should have received events from multiple connections
	if len(finalEventIds) < 2 {
		t.Errorf("Expected at least 2 events from reconnections, got %d", len(finalEventIds))
	}

	// Events should be from different connections
	uniqueConnections := make(map[string]bool)
	for _, eventId := range finalEventIds {
		uniqueConnections[eventId] = true
	}

	if len(uniqueConnections) < 2 {
		t.Errorf("Expected events from at least 2 different connections, got %d unique events", len(uniqueConnections))
	}
}
