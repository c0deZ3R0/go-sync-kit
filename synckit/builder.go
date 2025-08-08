package synckit

import (
	"fmt"
	"time"
)

// SyncManagerBuilder provides a fluent interface for constructing SyncManager instances.
type SyncManagerBuilder struct {
	store       EventStore
	transport   Transport
	options     *SyncOptions
	pushOnlySet bool // Track if PushOnly was explicitly set
	pullOnlySet bool // Track if PullOnly was explicitly set
}

// NewSyncManagerBuilder creates a new builder with default options.
func NewSyncManagerBuilder() *SyncManagerBuilder {
	return &SyncManagerBuilder{
		options: &SyncOptions{
			BatchSize:         100, // Default batch size
			EnableValidation:  false,
			Timeout:           0, // No timeout by default
			EnableCompression: false,
		},
	}
}

// WithStore sets the EventStore for the SyncManager.
func (b *SyncManagerBuilder) WithStore(store EventStore) *SyncManagerBuilder {
	b.store = store
	return b
}

// WithTransport sets the Transport for the SyncManager.
func (b *SyncManagerBuilder) WithTransport(transport Transport) *SyncManagerBuilder {
	b.transport = transport
	return b
}

// WithBatchSize sets the batch size for sync operations.
func (b *SyncManagerBuilder) WithBatchSize(size int) *SyncManagerBuilder {
	b.options.BatchSize = size
	return b
}

// WithPushOnly configures the SyncManager to only push events.
func (b *SyncManagerBuilder) WithPushOnly() *SyncManagerBuilder {
	b.options.PushOnly = true
	b.options.PullOnly = false
	b.pushOnlySet = true
	return b
}

// WithPullOnly configures the SyncManager to only pull events.
func (b *SyncManagerBuilder) WithPullOnly() *SyncManagerBuilder {
	b.options.PullOnly = true
	b.options.PushOnly = false
	b.pullOnlySet = true
	return b
}

// WithConflictResolver sets the conflict resolution strategy.
func (b *SyncManagerBuilder) WithConflictResolver(resolver ConflictResolver) *SyncManagerBuilder {
	b.options.ConflictResolver = resolver
	return b
}

// WithFilter sets an event filter function.
func (b *SyncManagerBuilder) WithFilter(filter func(Event) bool) *SyncManagerBuilder {
	b.options.Filter = filter
	return b
}

// WithSyncInterval sets the interval for automatic synchronization.
func (b *SyncManagerBuilder) WithSyncInterval(interval time.Duration) *SyncManagerBuilder {
	b.options.SyncInterval = interval
	return b
}

// WithValidation enables additional validation checks during sync operations.
func (b *SyncManagerBuilder) WithValidation() *SyncManagerBuilder {
	b.options.EnableValidation = true
	return b
}

// WithTimeout sets the maximum duration for sync operations.
func (b *SyncManagerBuilder) WithTimeout(timeout time.Duration) *SyncManagerBuilder {
	b.options.Timeout = timeout
	return b
}

// WithCompression enables data compression during transport.
func (b *SyncManagerBuilder) WithCompression(enabled bool) *SyncManagerBuilder {
	b.options.EnableCompression = enabled
	return b
}

// Build creates a new SyncManager instance with the configured options.
func (b *SyncManagerBuilder) Build() (SyncManager, error) {
	// Validate required components
	if b.store == nil {
		return nil, fmt.Errorf("EventStore is required")
	}
	if b.transport == nil {
		return nil, fmt.Errorf("Transport is required")
	}

	// Validate push/pull settings - check if both were explicitly set
	if b.pushOnlySet && b.pullOnlySet {
		return nil, fmt.Errorf("cannot set both PushOnly and PullOnly to true")
	}

	// Validate batch size
	if b.options.BatchSize <= 0 {
		return nil, fmt.Errorf("BatchSize must be positive, got %d", b.options.BatchSize)
	}

	// Create a new SyncManager instance
	return NewSyncManager(b.store, b.transport, b.options), nil
}

// Reset clears the builder, allowing reuse.
func (b *SyncManagerBuilder) Reset() *SyncManagerBuilder {
	b.store = nil
	b.transport = nil
	b.options = &SyncOptions{
		BatchSize:         100,
		EnableValidation:  false,
		Timeout:           0,
		EnableCompression: false,
	}
	b.pushOnlySet = false
	b.pullOnlySet = false
	return b
}
