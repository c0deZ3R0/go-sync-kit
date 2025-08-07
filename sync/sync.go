// Package sync provides a generic event-driven synchronization system
// for distributed applications. It supports offline-first architectures
// with conflict resolution and pluggable storage backends.
package sync

import (
	"context"
	"time"

	"github.com/c0deZ3R0/go-sync-kit-agent2/cursor"
)

// Event represents
// This interface should be implemented by user's event types.
type Event interface {
	// ID returns a unique identifier for this event
	ID() string

	// Type returns the event type (e.g., "UserCreated", "OrderUpdated")
	Type() string

	// AggregateID returns the ID of the aggregate this event belongs to
	AggregateID() string

	// Data returns the event payload
	Data() interface{}

	// Metadata returns additional event metadata
	Metadata() map[string]interface{}
}

// Version represents a point-in-time snapshot for sync operations.
// Users can implement different versioning strategies (timestamps, hashes, vector clocks).
type Version interface {
	// Compare returns -1 if this version is before other, 0 if equal, 1 if after
	Compare(other Version) int

	// String returns a string representation of the version
	String() string

	// IsZero returns true if this is the zero/initial version
	IsZero() bool
}

// EventStore provides persistence for events.
// Implementations can use any storage backend (SQLite, BadgerDB, PostgreSQL, etc.).
type EventStore interface {
	// Store persists an event to the store
	Store(ctx context.Context, event Event, version Version) error

	// Load retrieves all events since the given version
	Load(ctx context.Context, since Version) ([]EventWithVersion, error)

	// LoadByAggregate retrieves events for a specific aggregate since the given version
	LoadByAggregate(ctx context.Context, aggregateID string, since Version) ([]EventWithVersion, error)

	// LatestVersion returns the latest version in the store
	LatestVersion(ctx context.Context) (Version, error)

	// ParseVersion converts a string representation into a Version
	// This allows different storage implementations to handle their own version formats
	ParseVersion(ctx context.Context, versionStr string) (Version, error)

	// Close closes the store and releases resources
	Close() error
}

// EventWithVersion pairs an event with its version information
type EventWithVersion struct {
	Event   Event
	Version Version
}

// ConflictResolver handles conflicts when the same data is modified concurrently.
// Different strategies can be plugged in (last-write-wins, merge, user-prompt, etc.).
type ConflictResolver interface {
	// Resolve handles a conflict between local and remote events
	// Returns the resolved events to keep
	Resolve(ctx context.Context, local, remote []EventWithVersion) ([]EventWithVersion, error)
}

// Transport handles the actual network communication between clients and servers.
// Implementations can use HTTP, gRPC, WebSockets, NATS, etc.
type Transport interface {
	// Push sends events to the remote endpoint
	Push(ctx context.Context, events []EventWithVersion) error

	// Pull retrieves events from the remote endpoint since the given version
	Pull(ctx context.Context, since Version) ([]EventWithVersion, error)

	// GetLatestVersion efficiently retrieves the latest version from remote without pulling events
	GetLatestVersion(ctx context.Context) (Version, error)

	// Subscribe listens for real-time updates (optional for polling-based transports)
	Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error

	// Close closes the transport connection
	Close() error
}

// RetryConfig configures the retry behavior for sync operations
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int

	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases
	Multiplier float64
}

// SyncOptions configures the synchronization behavior
type SyncOptions struct {
    // LastCursorLoader loads the last saved cursor for cursor-based syncs
    LastCursorLoader func() cursor.Cursor

    // CursorSaver saves the latest cursor after a successful sync
    CursorSaver func(cursor.Cursor) error
	// PushOnly indicates this client should only push events, not pull
	PushOnly bool

	// PullOnly indicates this client should only pull events, not push
	PullOnly bool

	// ConflictResolver to use for handling conflicts
	ConflictResolver ConflictResolver

	// Filter can be used to sync only specific events
	Filter func(Event) bool

	// BatchSize limits how many events to sync at once
	BatchSize int

	// SyncInterval for automatic periodic sync (0 disables)
	SyncInterval time.Duration

	// RetryConfig configures retry behavior for sync operations
	RetryConfig *RetryConfig

	// EnableValidation enables additional validation checks during sync
	EnableValidation bool

	// Timeout sets the maximum duration for sync operations
	Timeout time.Duration

	// EnableCompression enables data compression during transport
	EnableCompression bool

	// MetricsCollector for observability hooks (optional)
	MetricsCollector MetricsCollector
}

// SyncManager coordinates the synchronization process between local and remote stores.
// This is the main entry point for the sync package.
type SyncManager interface {
	// Sync performs a bidirectional sync operation
	Sync(ctx context.Context) (*SyncResult, error)

	// Push sends local events to remote
	Push(ctx context.Context) (*SyncResult, error)

	// Pull retrieves remote events to local
	Pull(ctx context.Context) (*SyncResult, error)

	// StartAutoSync begins automatic synchronization at the configured interval
	StartAutoSync(ctx context.Context) error

	// StopAutoSync stops automatic synchronization
	StopAutoSync() error

	// Subscribe to sync events (optional)
	Subscribe(handler func(*SyncResult)) error

	// Close shuts down the sync manager
	Close() error
}

// SyncResult provides information about a completed sync operation
type SyncResult struct {
	// EventsPushed is the number of events sent to remote
	EventsPushed int

	// EventsPulled is the number of events received from remote
	EventsPulled int

	// ConflictsResolved is the number of conflicts that were resolved
	ConflictsResolved int

	// Errors contains any non-fatal errors that occurred during sync
	Errors []error

	// StartTime is when the sync operation began
	StartTime time.Time

	// Duration is how long the sync took
	Duration time.Duration

	// LocalVersion is the local version after sync
	LocalVersion Version

	// RemoteVersion is the remote version after sync
	RemoteVersion Version
}

// NewSyncManager creates a new sync manager with the provided components
func NewSyncManager(store EventStore, transport Transport, opts *SyncOptions) SyncManager {
	if opts == nil {
		opts = &SyncOptions{
			BatchSize: 100,
			RetryConfig: &RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     5 * time.Second,
				Multiplier:   2.0,
			},
			MetricsCollector: &NoOpMetricsCollector{}, // Default no-op collector
		}
	}

	// Set default metrics collector if none provided
	if opts.MetricsCollector == nil {
		opts.MetricsCollector = &NoOpMetricsCollector{}
	}

	return &syncManager{
		store:     store,
		transport: transport,
		options:   *opts,
	}
}
