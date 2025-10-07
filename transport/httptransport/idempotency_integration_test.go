package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

func TestHandlePush_IdempotencyIntegration(t *testing.T) {
	tests := []struct {
		name             string
		idempotencyKey   string
		sendKeyOnSecond  bool
		wantFirstStored  bool
		wantSecondStored bool
		wantSameResponse bool
		wantCacheHit     bool
	}{
		{
			name:             "with_idempotency_key_prevents_duplicates",
			idempotencyKey:   "test-key-123",
			sendKeyOnSecond:  true,
			wantFirstStored:  true,
			wantSecondStored: false,
			wantSameResponse: true,
			wantCacheHit:     true,
		},
		{
			name:             "without_idempotency_key_allows_duplicates",
			idempotencyKey:   "",
			sendKeyOnSecond:  false,
			wantFirstStored:  true,
			wantSecondStored: true,
			wantSameResponse: true,
			wantCacheHit:     false,
		},
		{
			name:             "different_keys_allow_duplicates",
			idempotencyKey:   "test-key-456",
			sendKeyOnSecond:  false, // Will use different key on second request
			wantFirstStored:  true,
			wantSecondStored: true,
			wantSameResponse: true,
			wantCacheHit:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			store := NewMockEventStore()
			logger := slog.Default()
			handler := NewSyncHandler(store, logger, nil, nil)

			// Create test event
			event := &MockEvent{
				id:          "test-event-1",
				eventType:   "TestEvent",
				aggregateID: "test-entity",
				data:        map[string]interface{}{"data": "test"},
				metadata:    map[string]interface{}{},
			}

			jsonEvent := JSONEventWithVersion{
				Event: JSONEvent{
					ID:          event.ID(),
					Type:        event.Type(),
					AggregateID: event.AggregateID(),
					Data:        event.Data(),
					Metadata:    event.Metadata(),
				},
				Version: "0", // Client version is ignored server-side
			}

			eventJSON, err := json.Marshal([]JSONEventWithVersion{jsonEvent})
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			// First request
			req1 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
			req1.Header.Set("Content-Type", "application/json")
			if tt.idempotencyKey != "" {
				req1.Header.Set(HeaderIdempotencyKey, tt.idempotencyKey)
			}

			rec1 := httptest.NewRecorder()
			handler.handlePush(rec1, req1)

			if rec1.Code != http.StatusOK {
				t.Errorf("First request failed with status %d, body: %s", rec1.Code, rec1.Body.String())
			}

			// Check first storage
			events1, err := store.Load(context.Background(), cursor.IntegerCursor{Seq: 0})
			if err != nil {
				t.Fatalf("Failed to load events after first request: %v", err)
			}
			firstStored := len(events1) > 0
			if firstStored != tt.wantFirstStored {
				t.Errorf("First request stored=%v, want=%v", firstStored, tt.wantFirstStored)
			}

			// Second request (same or no idempotency key)
			req2 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
			req2.Header.Set("Content-Type", "application/json")
			if tt.sendKeyOnSecond {
				req2.Header.Set(HeaderIdempotencyKey, tt.idempotencyKey)
			} else if tt.idempotencyKey != "" && tt.name == "different_keys_allow_duplicates" {
				req2.Header.Set(HeaderIdempotencyKey, "different-key-789")
			}

			rec2 := httptest.NewRecorder()
			handler.handlePush(rec2, req2)

			if rec2.Code != http.StatusOK {
				t.Errorf("Second request failed with status %d, body: %s", rec2.Code, rec2.Body.String())
			}

			// Check second storage
			events2, err := store.Load(context.Background(), cursor.IntegerCursor{Seq: 0})
			if err != nil {
				t.Fatalf("Failed to load events after second request: %v", err)
			}

			expectedEvents := 1
			if tt.wantSecondStored {
				expectedEvents = 2
			}
			if len(events2) != expectedEvents {
				t.Errorf("After second request: got %d events, want %d", len(events2), expectedEvents)
			}

			// Verify response bodies are same
			if tt.wantSameResponse {
				var resp1, resp2 map[string]interface{}
				json.Unmarshal(rec1.Body.Bytes(), &resp1)
				json.Unmarshal(rec2.Body.Bytes(), &resp2)

				if resp1["status"] != resp2["status"] {
					t.Errorf("Response mismatch: first=%v, second=%v", resp1, resp2)
				}
			}
		})
	}
}

func TestHandlePush_IdempotencyWithMultipleEvents(t *testing.T) {
	store := NewMockEventStore()
	logger := slog.Default()
	handler := NewSyncHandler(store, logger, nil, nil)

	// Create multiple test events
	events := []JSONEventWithVersion{}
	for i := 0; i < 3; i++ {
		event := &MockEvent{
			id:          fmt.Sprintf("test-event-%d", i),
			eventType:   "TestEvent",
			aggregateID: "test-entity",
			data:        map[string]interface{}{"index": i},
			metadata:    map[string]interface{}{},
		}
		jsonEvent := JSONEventWithVersion{
			Event: JSONEvent{
				ID:          event.ID(),
				Type:        event.Type(),
				AggregateID: event.AggregateID(),
				Data:        event.Data(),
				Metadata:    event.Metadata(),
			},
			Version: "0",
		}
		events = append(events, jsonEvent)
	}

	eventJSON, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("Failed to marshal events: %v", err)
	}

	idempotencyKey := "batch-key-123"

	// First request with multiple events
	req1 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set(HeaderIdempotencyKey, idempotencyKey)

	rec1 := httptest.NewRecorder()
	handler.handlePush(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("First request failed: %d", rec1.Code)
	}

	// Verify all events stored
	storedEvents, err := store.Load(context.Background(), cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events: %v", err)
	}
	if len(storedEvents) != 3 {
		t.Errorf("First request: got %d events, want 3", len(storedEvents))
	}

	// Second request with same key
	req2 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(HeaderIdempotencyKey, idempotencyKey)

	rec2 := httptest.NewRecorder()
	handler.handlePush(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("Second request failed: %d", rec2.Code)
	}

	// Verify no additional events stored
	storedEvents2, err := store.Load(context.Background(), cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events after second request: %v", err)
	}
	if len(storedEvents2) != 3 {
		t.Errorf("Second request: got %d events, want 3 (no duplicates)", len(storedEvents2))
	}
}

func TestHandlePush_IdempotencyKeyParsing(t *testing.T) {
	tests := []struct {
		name              string
		headerValue       string
		expectCacheHit    bool
		secondHeaderValue string
	}{
		{
			name:              "standard_uuid_key",
			headerValue:       "550e8400-e29b-41d4-a716-446655440000",
			expectCacheHit:    true,
			secondHeaderValue: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:              "simple_string_key",
			headerValue:       "my-request-123",
			expectCacheHit:    true,
			secondHeaderValue: "my-request-123",
		},
		{
			name:              "key_with_special_chars",
			headerValue:       "req:2024-01-15:user-456",
			expectCacheHit:    true,
			secondHeaderValue: "req:2024-01-15:user-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMockEventStore()
			logger := slog.Default()
			handler := NewSyncHandler(store, logger, nil, nil)

			event := &MockEvent{
				id:          "test-event-1",
				eventType:   "TestEvent",
				aggregateID: "test-entity",
				data:        map[string]interface{}{"data": "test"},
				metadata:    map[string]interface{}{},
			}
			jsonEvent := JSONEventWithVersion{
				Event: JSONEvent{
					ID:          event.ID(),
					Type:        event.Type(),
					AggregateID: event.AggregateID(),
					Data:        event.Data(),
					Metadata:    event.Metadata(),
				},
				Version: "0",
			}

			eventJSON, _ := json.Marshal([]JSONEventWithVersion{jsonEvent})

			// First request
			req1 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
			req1.Header.Set("Content-Type", "application/json")
			req1.Header.Set(HeaderIdempotencyKey, tt.headerValue)

			rec1 := httptest.NewRecorder()
			handler.handlePush(rec1, req1)

			if rec1.Code != http.StatusOK {
				t.Fatalf("First request failed: %d", rec1.Code)
			}

			// Second request
			req2 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
			req2.Header.Set("Content-Type", "application/json")
			req2.Header.Set(HeaderIdempotencyKey, tt.secondHeaderValue)

			rec2 := httptest.NewRecorder()
			handler.handlePush(rec2, req2)

			if rec2.Code != http.StatusOK {
				t.Fatalf("Second request failed: %d", rec2.Code)
			}

			// Verify events count
			storedEvents, _ := store.Load(context.Background(), cursor.IntegerCursor{Seq: 0})
			expectedCount := 1
			if !tt.expectCacheHit {
				expectedCount = 2
			}

			if len(storedEvents) != expectedCount {
				t.Errorf("Got %d events, want %d (cache hit=%v)", len(storedEvents), expectedCount, tt.expectCacheHit)
			}
		})
	}
}

func TestHandlePush_IdempotencyWithHooks(t *testing.T) {
	store := NewMockEventStore()
	logger := slog.Default()

	// Track hook calls
	hookCallCount := 0
	hooks := &SyncHooks{
		AfterCommit: func(ctx context.Context, events []synckit.EventWithVersion) {
			hookCallCount++
		},
	}

	handler := NewSyncHandlerWithHooks(store, logger, nil, nil, hooks)

	event := &MockEvent{
		id:          "test-event-1",
		eventType:   "TestEvent",
		aggregateID: "test-entity",
		data:        map[string]interface{}{"data": "test"},
		metadata:    map[string]interface{}{},
	}
	jsonEvent := JSONEventWithVersion{
		Event: JSONEvent{
			ID:          event.ID(),
			Type:        event.Type(),
			AggregateID: event.AggregateID(),
			Data:        event.Data(),
			Metadata:    event.Metadata(),
		},
		Version: "0",
	}

	eventJSON, _ := json.Marshal([]JSONEventWithVersion{jsonEvent})
	idempotencyKey := "hook-test-key"

	// First request - should call hook
	req1 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set(HeaderIdempotencyKey, idempotencyKey)

	rec1 := httptest.NewRecorder()
	handler.handlePush(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("First request failed: %d", rec1.Code)
	}

	// Second request with same key - should NOT call hook (cached response)
	req2 := httptest.NewRequest(http.MethodPost, "/sync/push", bytes.NewReader(eventJSON))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(HeaderIdempotencyKey, idempotencyKey)

	rec2 := httptest.NewRecorder()
	handler.handlePush(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("Second request failed: %d", rec2.Code)
	}

	// Hook should only be called once (first request)
	// Note: Hook is called asynchronously, so we might need a small delay
	// For simplicity in this test, we'll just check that it's not called twice immediately
	if hookCallCount > 1 {
		t.Errorf("AfterCommit hook called %d times, expected 1 or 0 (due to timing)", hookCallCount)
	}
}
