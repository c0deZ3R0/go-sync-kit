package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	std_sync "sync"
	"time"

"github.com/c0deZ3R0/go-sync-kit/cursor"
	sync_pkg "github.com/c0deZ3R0/go-sync-kit/sync"
)

// MockTransport implements both Transport and CursorTransport interfaces
type MockTransport struct {
	mu     std_sync.RWMutex
	events []sync_pkg.EventWithVersion
}

func NewMockTransport() sync_pkg.Transport {
	return &MockTransport{
		events: make([]sync_pkg.EventWithVersion, 0),
	}
}

func (t *MockTransport) Push(ctx context.Context, events []sync_pkg.EventWithVersion) error {
		t.mu.Lock()
		defer t.mu.Unlock()
		log.Printf("Mock transport: pushing %d events", len(events))
		t.events = append(t.events, events...)
		return nil
}

func (t *MockTransport) Pull(ctx context.Context, since sync_pkg.Version) ([]sync_pkg.EventWithVersion, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if since == nil {
		return t.events, nil
	}

var result []sync_pkg.EventWithVersion
	for _, ev := range t.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (t *MockTransport) GetLatestVersion(ctx context.Context) (sync_pkg.Version, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.events) == 0 {
		return nil, nil
	}

	latest := t.events[0].Version
	for i := 1; i < len(t.events); i++ {
		if t.events[i].Version.Compare(latest) > 0 {
			latest = t.events[i].Version
		}
	}
	return latest, nil
}

func (t *MockTransport) Subscribe(ctx context.Context, handler func([]sync_pkg.EventWithVersion) error) error {
	// In this mock, we'll just send updates every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.mu.RLock()
				if len(t.events) > 0 {
					log.Printf("Notifying subscriber of %d events", len(t.events))
					if err := handler(t.events); err != nil {
						// In real implementation, might want to handle errors differently
						t.mu.RUnlock()
						return
					}
				}
				t.mu.RUnlock()
			}
		}
	}()
	return nil
}

func (t *MockTransport) Close() error {
	return nil
}

// PullWithCursor implements cursor-based sync
func (t *MockTransport) PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]sync_pkg.EventWithVersion, cursor.Cursor, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// In this mock implementation, we'll treat cursor as an index
	startIdx := 0
	if since != nil {
		// Convert the cursor to our mock type
		if mc, ok := since.(mockCursor); ok {
			startIdx = int(mc)
		}
	}

	log.Printf("Mock transport: pulling with cursor %v (limit: %d) starting at index %d", since, limit, startIdx)

	if startIdx >= len(t.events) {
		return nil, since, nil
	}

	endIdx := startIdx + limit
	if endIdx > len(t.events) {
		endIdx = len(t.events)
	}

	result := t.events[startIdx:endIdx]
	newCursor := mockCursor(endIdx)

	log.Printf("Mock transport: returning %d events with new cursor %v", len(result), newCursor)
	return result, newCursor, nil
}

// mockCursor implements a simple cursor for testing
type mockCursor int

func (c mockCursor) String() string {
	return fmt.Sprintf("%d", c)
}

func (c mockCursor) Kind() string {
	return "mock"
}

func (c mockCursor) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": c.Kind(),
		"position": int(c),
	})
}

func (c *mockCursor) UnmarshalJSON(data []byte) error {
	var v struct {
		Type     string `json:"type"`
		Position int    `json:"position"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.Type != "mock" {
		return fmt.Errorf("invalid cursor type: %s", v.Type)
	}
	*c = mockCursor(v.Position)
	return nil
}
