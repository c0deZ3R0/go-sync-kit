package synckit

import (
	"context"
	"errors"
	"log/slog"
	"time"

	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
)

// ManagerOption is a functional option for configuring a SyncManager via NewManager.
type ManagerOption func(*SyncManagerBuilder) error

// NewManager constructs a SyncManager using functional options on top of the existing builder.
// It keeps your builder for advanced use while offering a concise, discoverable API.
func NewManager(opts ...ManagerOption) (SyncManager, error) {
	const op = "synckit.NewManager"

	b := NewSyncManagerBuilder()

	for _, opt := range opts {
		if err := opt(b); err != nil {
			return nil, syncErrors.NewWithComponent(
				syncErrors.OpStore, // Using OpStore as closest available
				"synckit",
				err,
			)
		}
	}

	// Minimal validation for a good UX
	if b.store == nil {
		return nil, syncErrors.E(
			syncErrors.Op("NewManager"),
			syncErrors.Component("synckit"),
			syncErrors.KindInvalid,
			errors.New("store is required (use WithStore(...))"),
		)
	}

	return b.Build()
}

// WithStore injects a pre-built EventStore.
func WithStore(s EventStore) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithStore(s)
		return nil
	}
}

// WithSQLite constructs and injects a SQLite store.
// Note: This function is provided as an example. In practice, you should
// create your SQLite store separately and use WithStore() to avoid import cycles.
// 
// Example:
//   store, err := sqlite.NewWithDataSource("app.db")
//   if err != nil { /* handle */ }
//   mgr, err := synckit.NewManager(synckit.WithStore(store), ...)
func WithSQLite(path string) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		return syncErrors.E(
			syncErrors.Op("WithSQLite"),
			syncErrors.Component("synckit"),
			syncErrors.KindInvalid,
			errors.New("WithSQLite is not available to avoid import cycles. Create SQLite store separately and use WithStore()"),
			"suggestion: Use WithStore(sqlite.NewWithDataSource(path))",
		)
	}
}

// WithHTTPTransport configures the HTTP transport.
// Note: This function is not available to avoid import cycles. Create HTTP transport
// separately and use WithTransport() instead.
//
// Example:
//   transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)
//   mgr, err := synckit.NewManager(synckit.WithTransport(transport), ...)
func WithHTTPTransport(baseURL string) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		return syncErrors.E(
			syncErrors.Op("WithHTTPTransport"),
			syncErrors.Component("synckit"),
			syncErrors.KindInvalid,
			errors.New("WithHTTPTransport is not available to avoid import cycles. Create HTTP transport separately and use WithTransport()"),
			"suggestion: Use WithTransport(httptransport.NewTransport(baseURL, nil, nil, nil))",
		)
	}
}

// WithTransport sets a pre-built Transport.
func WithTransport(t Transport) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithTransport(t)
		return nil
	}
}

// WithConflictResolver sets the conflict resolution strategy.
func WithConflictResolver(r ConflictResolver) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithConflictResolver(r)
		return nil
	}
}

// WithBatchSize sets the batch size in SyncOptions.
func WithBatchSize(n int) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithBatchSize(n)
		return nil
	}
}

// WithTimeout sets a per-sync timeout in SyncOptions.
func WithTimeout(d time.Duration) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithTimeout(d)
		return nil
	}
}

// WithSyncInterval sets the interval for automatic synchronization.
func WithSyncInterval(interval time.Duration) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithSyncInterval(interval)
		return nil
	}
}

// WithFilter sets an event filter function.
func WithFilter(filter func(Event) bool) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithFilter(filter)
		return nil
	}
}

// WithValidation enables additional validation checks during sync operations.
func WithValidation() ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithValidation()
		return nil
	}
}

// WithCompression enables data compression during transport.
func WithCompression(enabled bool) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithCompression(enabled)
		return nil
	}
}

// WithPushOnly configures the SyncManager to only push events.
func WithPushOnly() ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithPushOnly()
		return nil
	}
}

// WithPullOnly configures the SyncManager to only pull events.
func WithPullOnly() ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithPullOnly()
		return nil
	}
}

// WithManagerLogger sets a custom logger for the SyncManager.
func WithManagerLogger(logger *slog.Logger) ManagerOption {
	return func(b *SyncManagerBuilder) error {
		b.WithLogger(logger)
		return nil
	}
}

// Convenience options for common strategies

// WithLWW is convenience for the common Last-Write-Wins strategy.
func WithLWW() ManagerOption {
	return WithConflictResolver(&LastWriteWinsResolver{})
}

// WithFWW is convenience for the First-Write-Wins strategy.
func WithFWW() ManagerOption {
	return WithConflictResolver(&FirstWriteWinsResolver{})
}

// WithAdditiveMerge is convenience for the Additive Merge strategy.
func WithAdditiveMerge() ManagerOption {
	return WithConflictResolver(&AdditiveMergeResolver{})
}

// nullTransport is a no-op transport for local-only scenarios.
type nullTransport struct{}

func (n *nullTransport) Push(ctx context.Context, events []EventWithVersion) error {
	return nil // No-op: doesn't push anywhere
}

func (n *nullTransport) Pull(ctx context.Context, since Version) ([]EventWithVersion, error) {
	return nil, nil // No-op: no remote events to pull
}

func (n *nullTransport) GetLatestVersion(ctx context.Context) (Version, error) {
	return &nullVersion{}, nil // Return zero version
}

func (n *nullTransport) Subscribe(ctx context.Context, handler func([]EventWithVersion) error) error {
	return nil // No-op: no real-time events
}

func (n *nullTransport) Close() error {
	return nil // No-op: nothing to close
}

// nullVersion for the null transport
type nullVersion struct{}

func (v *nullVersion) Compare(other Version) int { return 0 }
func (v *nullVersion) String() string { return "0" }
func (v *nullVersion) IsZero() bool { return true }

// WithNullTransport configures a no-op transport for local-only scenarios.
// Use this when you only need local event storage without remote synchronization.
func WithNullTransport() ManagerOption {
	return WithTransport(&nullTransport{})
}
