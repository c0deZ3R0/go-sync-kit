package httptransport

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyTracker_NewTracker(t *testing.T) {
	tracker := NewIdempotencyTracker(0, 0)
	defer tracker.Close()

	assert.NotNil(t, tracker)
	assert.Equal(t, 0, tracker.Size())
}

func TestIdempotencyTracker_RecordAndCheck(t *testing.T) {
	tracker := NewIdempotencyTracker(5*time.Second, 100)
	defer tracker.Close()

	// Record a key
	testResponse := map[string]interface{}{"status": "ok", "events": 5}
	tracker.Record("key-123", testResponse)

	// Check should find it
	response, found := tracker.Check("key-123")
	assert.True(t, found)
	assert.Equal(t, testResponse, response)

	// Non-existent key should not be found
	_, found = tracker.Check("nonexistent")
	assert.False(t, found)
}

func TestIdempotencyTracker_Expiration(t *testing.T) {
	// Create tracker with 100ms expiration
	tracker := NewIdempotencyTracker(100*time.Millisecond, 100)
	defer tracker.Close()

	// Record a key
	tracker.Record("expire-me", "response")

	// Should be found immediately
	_, found := tracker.Check("expire-me")
	assert.True(t, found)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not be found after expiration
	_, found = tracker.Check("expire-me")
	assert.False(t, found)
}

func TestIdempotencyTracker_MaxSizeEviction(t *testing.T) {
	// Create tracker with max size of 3
	tracker := NewIdempotencyTracker(1*time.Hour, 3)
	defer tracker.Close()

	// Record 3 keys
	tracker.Record("key-1", "response-1")
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	tracker.Record("key-2", "response-2")
	time.Sleep(10 * time.Millisecond)
	tracker.Record("key-3", "response-3")

	assert.Equal(t, 3, tracker.Size())

	// All 3 should be found
	_, found := tracker.Check("key-1")
	assert.True(t, found)
	_, found = tracker.Check("key-2")
	assert.True(t, found)
	_, found = tracker.Check("key-3")
	assert.True(t, found)

	// Add a 4th key - should evict oldest (key-1)
	time.Sleep(10 * time.Millisecond)
	tracker.Record("key-4", "response-4")

	assert.Equal(t, 3, tracker.Size())

	// key-1 should be evicted
	_, found = tracker.Check("key-1")
	assert.False(t, found, "Oldest key should be evicted")

	// Others should still be present
	_, found = tracker.Check("key-2")
	assert.True(t, found)
	_, found = tracker.Check("key-3")
	assert.True(t, found)
	_, found = tracker.Check("key-4")
	assert.True(t, found)
}

func TestIdempotencyTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewIdempotencyTracker(1*time.Hour, 1000)
	defer tracker.Close()

	// Concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 100
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := string(rune('A' + id))
			tracker.Record(key, map[string]int{"id": id})
		}(i)
	}

	wg.Wait()

	// All keys should be recorded
	assert.Equal(t, numGoroutines, tracker.Size())

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := string(rune('A' + id))
			_, found := tracker.Check(key)
			assert.True(t, found)
		}(i)
	}

	wg.Wait()
}

func TestIdempotencyTracker_Clear(t *testing.T) {
	tracker := NewIdempotencyTracker(1*time.Hour, 100)
	defer tracker.Close()

	// Record some keys
	tracker.Record("key-1", "response-1")
	tracker.Record("key-2", "response-2")
	tracker.Record("key-3", "response-3")

	assert.Equal(t, 3, tracker.Size())

	// Clear all keys
	tracker.Clear()

	assert.Equal(t, 0, tracker.Size())

	// No keys should be found
	_, found := tracker.Check("key-1")
	assert.False(t, found)
	_, found = tracker.Check("key-2")
	assert.False(t, found)
	_, found = tracker.Check("key-3")
	assert.False(t, found)
}

func TestIdempotencyTracker_AutoCleanup(t *testing.T) {
	// Create tracker with very short expiration and cleanup interval
	tracker := NewIdempotencyTracker(100*time.Millisecond, 100)
	defer tracker.Close()

	// Record some keys
	tracker.Record("key-1", "response-1")
	tracker.Record("key-2", "response-2")

	assert.Equal(t, 2, tracker.Size())

	// Wait for expiration (but not enough for cleanup cycle)
	time.Sleep(150 * time.Millisecond)

	// Keys should still be in map (cleanup hasn't run yet)
	// But Check() should return false due to expiration check
	_, found := tracker.Check("key-1")
	assert.False(t, found, "Key should be expired")

	// Note: We can't easily test the automatic cleanup goroutine
	// without modifying the cleanup interval (which is hardcoded to 1 hour)
	// In production, expired keys will be removed by the hourly cleanup
}

func TestIdempotencyTracker_DuplicateRecording(t *testing.T) {
	tracker := NewIdempotencyTracker(1*time.Hour, 100)
	defer tracker.Close()

	// Record same key multiple times
	tracker.Record("duplicate-key", "response-1")
	tracker.Record("duplicate-key", "response-2")
	tracker.Record("duplicate-key", "response-3")

	// Should only have 1 entry
	assert.Equal(t, 1, tracker.Size())

	// Should get the latest response
	response, found := tracker.Check("duplicate-key")
	assert.True(t, found)
	assert.Equal(t, "response-3", response)
}

func TestIdempotencyTracker_ResponseTypes(t *testing.T) {
	tracker := NewIdempotencyTracker(1*time.Hour, 100)
	defer tracker.Close()

	// Test different response types
	testCases := []struct {
		name     string
		key      string
		response interface{}
	}{
		{"string", "key-string", "string response"},
		{"int", "key-int", 42},
		{"map", "key-map", map[string]interface{}{"status": "ok", "count": 5}},
		{"slice", "key-slice", []string{"a", "b", "c"}},
		{"nil", "key-nil", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracker.Record(tc.key, tc.response)
			response, found := tracker.Check(tc.key)
			require.True(t, found)
			assert.Equal(t, tc.response, response)
		})
	}
}
