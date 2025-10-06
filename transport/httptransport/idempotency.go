package httptransport

import (
	"sync"
	"time"
)

// IdempotencyTracker tracks processed idempotency keys to prevent duplicate processing
type IdempotencyTracker struct {
	mu       sync.RWMutex
	keys     map[string]idempotencyEntry
	maxAge   time.Duration
	maxSize  int
	stopChan chan struct{}
}

type idempotencyEntry struct {
	timestamp time.Time
	response  interface{} // Cached response for this key
}

// NewIdempotencyTracker creates a new tracker with the specified max age and size
func NewIdempotencyTracker(maxAge time.Duration, maxSize int) *IdempotencyTracker {
	if maxAge == 0 {
		maxAge = 24 * time.Hour // Default: 24 hours
	}
	if maxSize == 0 {
		maxSize = 10000 // Default: 10k keys
	}

	tracker := &IdempotencyTracker{
		keys:     make(map[string]idempotencyEntry),
		maxAge:   maxAge,
		maxSize:  maxSize,
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	go tracker.cleanup()

	return tracker
}

// Check returns the cached response if the key was already processed
// Returns (response, true) if found and not expired, (nil, false) otherwise
func (t *IdempotencyTracker) Check(key string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, exists := t.keys[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.timestamp) > t.maxAge {
		return nil, false
	}

	return entry.response, true
}

// Record stores a processed idempotency key with its response
func (t *IdempotencyTracker) Record(key string, response interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Enforce max size (simple LRU eviction)
	if len(t.keys) >= t.maxSize {
		// Remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, v := range t.keys {
			if first || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
				first = false
			}
		}
		delete(t.keys, oldestKey)
	}

	t.keys[key] = idempotencyEntry{
		timestamp: time.Now(),
		response:  response,
	}
}

// cleanup removes expired keys periodically
func (t *IdempotencyTracker) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.mu.Lock()
			now := time.Now()
			for key, entry := range t.keys {
				if now.Sub(entry.timestamp) > t.maxAge {
					delete(t.keys, key)
				}
			}
			t.mu.Unlock()
		case <-t.stopChan:
			return
		}
	}
}

// Close stops the cleanup goroutine
func (t *IdempotencyTracker) Close() {
	close(t.stopChan)
}

// Size returns the current number of tracked keys (for testing/monitoring)
func (t *IdempotencyTracker) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.keys)
}

// Clear removes all tracked keys (for testing)
func (t *IdempotencyTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keys = make(map[string]idempotencyEntry)
}
