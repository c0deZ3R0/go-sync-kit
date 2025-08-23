package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

func TestSyncHandler_AfterCommitHook(t *testing.T) {
	t.Run("AfterCommitHookCalled", func(t *testing.T) {
		store := NewMockEventStore()
		var committedEvents []synckit.EventWithVersion
		var hookCalled bool
		var mu sync.Mutex

		hooks := &SyncHooks{
			AfterCommit: func(ctx context.Context, committed []synckit.EventWithVersion) {
				mu.Lock()
				defer mu.Unlock()
				hookCalled = true
				committedEvents = committed
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		// Create test events
		eventData := []JSONEventWithVersion{
			{
				Event: JSONEvent{
					ID:          "test-1",
					Type:        "UserCreated",
					AggregateID: "user-1",
					Data:        "test data",
				},
				Version: "1",
			},
			{
				Event: JSONEvent{
					ID:          "test-2",
					Type:        "UserUpdated",
					AggregateID: "user-1",
					Data:        "updated data",
				},
				Version: "2",
			},
		}

		jsonData, err := json.Marshal(eventData)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		// Wait for async hook to complete
		time.Sleep(100 * time.Millisecond)

		// Verify hook was called
		mu.Lock()
		assert.True(t, hookCalled, "AfterCommit hook should have been called")
		assert.Equal(t, 2, len(committedEvents), "Should have 2 committed events")
		assert.Equal(t, "test-1", committedEvents[0].Event.ID())
		assert.Equal(t, "UserCreated", committedEvents[0].Event.Type())
		assert.Equal(t, "test-2", committedEvents[1].Event.ID())
		assert.Equal(t, "UserUpdated", committedEvents[1].Event.Type())
		mu.Unlock()
	})

	t.Run("AfterCommitHookNotCalledWhenNoHooks", func(t *testing.T) {
		store := NewMockEventStore()
		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), nil)

		eventData := []JSONEventWithVersion{
			{
				Event: JSONEvent{
					ID:   "test-1",
					Type: "UserCreated",
				},
				Version: "1",
			},
		}

		jsonData, err := json.Marshal(eventData)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Should not panic and should return OK
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("AfterCommitHookNotCalledWhenNoCommittedEvents", func(t *testing.T) {
		store := &MockEventStoreWithErrors{}
		var hookCalled bool

		hooks := &SyncHooks{
			AfterCommit: func(ctx context.Context, committed []synckit.EventWithVersion) {
				hookCalled = true
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		eventData := []JSONEventWithVersion{
			{
				Event: JSONEvent{
					ID:   "test-1",
					Type: "UserCreated",
				},
				Version: "1",
			},
		}

		jsonData, err := json.Marshal(eventData)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		// Response should still be OK (we continue on storage errors)
		assert.Equal(t, http.StatusOK, w.Code)

		// Wait to ensure hook would have been called if there were committed events
		time.Sleep(100 * time.Millisecond)

		// Hook should not have been called since no events were successfully stored
		assert.False(t, hookCalled, "AfterCommit hook should not be called when no events are committed")
	})

	t.Run("AfterCommitHookOnlyForCommittedEvents", func(t *testing.T) {
		store := &MockEventStorePartialFailure{}
		var committedEvents []synckit.EventWithVersion
		var mu sync.Mutex

		hooks := &SyncHooks{
			AfterCommit: func(ctx context.Context, committed []synckit.EventWithVersion) {
				mu.Lock()
				defer mu.Unlock()
				committedEvents = committed
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		eventData := []JSONEventWithVersion{
			{
				Event: JSONEvent{
					ID:   "test-1",
					Type: "UserCreated",
				},
				Version: "1",
			},
			{
				Event: JSONEvent{
					ID:   "fail-me",
					Type: "UserCreated",
				},
				Version: "2",
			},
			{
				Event: JSONEvent{
					ID:   "test-2",
					Type: "UserUpdated",
				},
				Version: "3",
			},
		}

		jsonData, err := json.Marshal(eventData)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.handlePush(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Wait for async hook to complete
		time.Sleep(100 * time.Millisecond)

		// Should only have the successfully committed events
		mu.Lock()
		assert.Equal(t, 2, len(committedEvents), "Should only have successfully committed events")
		assert.Equal(t, "test-1", committedEvents[0].Event.ID())
		assert.Equal(t, "test-2", committedEvents[1].Event.ID())
		mu.Unlock()
	})
}

func TestSyncHandler_BeforePullHook(t *testing.T) {
	t.Run("BeforePullHookCalled", func(t *testing.T) {
		store := NewMockEventStore()
		var hookCalled bool
		var hookVersion synckit.Version
		var mu sync.Mutex

		hooks := &SyncHooks{
			BeforePull: func(ctx context.Context, since synckit.Version) {
				mu.Lock()
				defer mu.Unlock()
				hookCalled = true
				hookVersion = since
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		req := httptest.NewRequest(http.MethodGet, "/pull?since=5", nil)
		w := httptest.NewRecorder()

		handler.handlePull(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify hook was called
		mu.Lock()
		assert.True(t, hookCalled, "BeforePull hook should have been called")
		assert.Equal(t, cursor.IntegerCursor{Seq: 5}, hookVersion, "Hook should receive correct version")
		mu.Unlock()
	})

	t.Run("BeforePullHookNotCalledWhenNoHooks", func(t *testing.T) {
		store := NewMockEventStore()
		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), nil)

		req := httptest.NewRequest(http.MethodGet, "/pull?since=0", nil)
		w := httptest.NewRecorder()

		handler.handlePull(w, req)

		// Should not panic and should return OK
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("BeforePullHookCalledWithDefaultVersion", func(t *testing.T) {
		store := NewMockEventStore()
		var hookCalled bool
		var hookVersion synckit.Version
		var mu sync.Mutex

		hooks := &SyncHooks{
			BeforePull: func(ctx context.Context, since synckit.Version) {
				mu.Lock()
				defer mu.Unlock()
				hookCalled = true
				hookVersion = since
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		// Request without since parameter should default to "0"
		req := httptest.NewRequest(http.MethodGet, "/pull", nil)
		w := httptest.NewRecorder()

		handler.handlePull(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mu.Lock()
		assert.True(t, hookCalled, "BeforePull hook should have been called")
		assert.Equal(t, cursor.IntegerCursor{Seq: 0}, hookVersion, "Hook should receive default version 0")
		mu.Unlock()
	})

	t.Run("BeforePullHookNotCalledOnInvalidVersion", func(t *testing.T) {
		store := NewMockEventStore()
		var hookCalled bool

		hooks := &SyncHooks{
			BeforePull: func(ctx context.Context, since synckit.Version) {
				hookCalled = true
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		req := httptest.NewRequest(http.MethodGet, "/pull?since=invalid", nil)
		w := httptest.NewRecorder()

		handler.handlePull(w, req)

		// Should return bad request for invalid version
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Hook should not be called since version parsing failed
		assert.False(t, hookCalled, "BeforePull hook should not be called when version parsing fails")
	})
}

func TestSyncHandler_BothHooks(t *testing.T) {
	t.Run("BothHooksWork", func(t *testing.T) {
		store := NewMockEventStore()
		var afterCommitCalled, beforePullCalled bool
		var mu sync.Mutex

		hooks := &SyncHooks{
			AfterCommit: func(ctx context.Context, committed []synckit.EventWithVersion) {
				mu.Lock()
				defer mu.Unlock()
				afterCommitCalled = true
			},
			BeforePull: func(ctx context.Context, since synckit.Version) {
				mu.Lock()
				defer mu.Unlock()
				beforePullCalled = true
			},
		}

		handler := NewSyncHandlerWithHooks(store, slog.Default(), nil, DefaultServerOptions(), hooks)

		// Test push
		eventData := []JSONEventWithVersion{
			{
				Event: JSONEvent{ID: "test-1", Type: "UserCreated"},
				Version: "1",
			},
		}

		jsonData, err := json.Marshal(eventData)
		require.NoError(t, err)

		pushReq := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(jsonData)))
		pushReq.Header.Set("Content-Type", "application/json")
		pushW := httptest.NewRecorder()

		handler.handlePush(pushW, pushReq)
		assert.Equal(t, http.StatusOK, pushW.Code)

		// Test pull
		pullReq := httptest.NewRequest(http.MethodGet, "/pull?since=0", nil)
		pullW := httptest.NewRecorder()

		handler.handlePull(pullW, pullReq)
		assert.Equal(t, http.StatusOK, pullW.Code)

		// Wait for async hook
		time.Sleep(100 * time.Millisecond)

		// Verify both hooks were called
		mu.Lock()
		assert.True(t, afterCommitCalled, "AfterCommit hook should have been called")
		assert.True(t, beforePullCalled, "BeforePull hook should have been called")
		mu.Unlock()
	})
}

// Mock event store that always returns errors
type MockEventStoreWithErrors struct{}

func (m *MockEventStoreWithErrors) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	return assert.AnError // Always fail
}

func (m *MockEventStoreWithErrors) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, nil
}

func (m *MockEventStoreWithErrors) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, nil
}

func (m *MockEventStoreWithErrors) LatestVersion(ctx context.Context) (synckit.Version, error) {
	return cursor.IntegerCursor{Seq: 0}, nil
}

func (m *MockEventStoreWithErrors) ParseVersion(ctx context.Context, s string) (synckit.Version, error) {
	seq, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return cursor.IntegerCursor{Seq: seq}, nil
}

func (m *MockEventStoreWithErrors) Close() error {
	return nil
}

// Mock event store that fails on events with ID "fail-me"
type MockEventStorePartialFailure struct {
	events []synckit.EventWithVersion
	mux    sync.RWMutex
}

func (m *MockEventStorePartialFailure) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	if event.ID() == "fail-me" {
		return assert.AnError
	}

	m.mux.Lock()
	defer m.mux.Unlock()

	var seq uint64 = 1
	if len(m.events) > 0 {
		if lastVersion, ok := m.events[len(m.events)-1].Version.(cursor.IntegerCursor); ok {
			seq = lastVersion.Seq + 1
		}
	}

	m.events = append(m.events, synckit.EventWithVersion{
		Event:   event,
		Version: cursor.IntegerCursor{Seq: seq},
	})

	return nil
}

func (m *MockEventStorePartialFailure) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	var result []synckit.EventWithVersion
	for _, ev := range m.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (m *MockEventStorePartialFailure) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, nil
}

func (m *MockEventStorePartialFailure) LatestVersion(ctx context.Context) (synckit.Version, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	if len(m.events) == 0 {
		return cursor.IntegerCursor{Seq: 0}, nil
	}
	return m.events[len(m.events)-1].Version, nil
}

func (m *MockEventStorePartialFailure) ParseVersion(ctx context.Context, s string) (synckit.Version, error) {
	seq, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return cursor.IntegerCursor{Seq: seq}, nil
}

func (m *MockEventStorePartialFailure) Close() error {
	return nil
}
