// Package badger provides BadgerDB-based implementations of projection interfaces.
package badger

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dgraph-io/badger/v4"

	"github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// OffsetStore implements projection.OffsetStore using BadgerDB as the backend.
// It provides high-performance offset persistence with excellent concurrent access patterns.
type OffsetStore struct {
	db     *badger.DB
	parseVersion func(ctx context.Context, s string) (synckit.Version, error)
	logger *slog.Logger
	mu     sync.RWMutex
	closed bool
}

// OffsetStoreOption configures an OffsetStore using the functional options pattern.
type OffsetStoreOption func(*OffsetStore)

// WithLogger sets a custom structured logger for the offset store.
// If not provided, uses the default logger from the logging package.
func WithLogger(logger *slog.Logger) OffsetStoreOption {
	return func(o *OffsetStore) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// Config holds configuration options for the BadgerDB offset store.
type Config struct {
	// Path is the directory where BadgerDB files will be stored
	Path string

	// BadgerOptions allows customization of BadgerDB behavior
	// If nil, sensible defaults will be used
	BadgerOptions *badger.Options
}

// DefaultConfig returns a Config with production-ready defaults for BadgerDB.
func DefaultConfig(path string) *Config {
	return &Config{
		Path: path,
		// Use BadgerDB's default options - they are well-tested and production-ready
		BadgerOptions: nil,
	}
}

// NewOffsetStore creates a new BadgerDB-backed offset store.
func NewOffsetStore(config *Config, parseVersion func(ctx context.Context, s string) (synckit.Version, error), opts ...OffsetStoreOption) (*OffsetStore, error) {
	if config == nil {
		return nil, errors.E(
			errors.Op("NewOffsetStore"),
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("config cannot be nil"),
		)
	}

	if config.Path == "" {
		return nil, errors.E(
			errors.Op("NewOffsetStore"),
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("config.Path cannot be empty"),
		)
	}

	if parseVersion == nil {
		return nil, errors.E(
			errors.Op("NewOffsetStore"),
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("parseVersion function cannot be nil"),
		)
	}

	// Set defaults - use BadgerDB's default options if not provided
	var options badger.Options
	if config.BadgerOptions != nil {
		options = *config.BadgerOptions
	} else {
		options = badger.DefaultOptions(config.Path)
	}
	options.Dir = config.Path
	options.ValueDir = config.Path

	// Open BadgerDB
	db, err := badger.Open(options)
	if err != nil {
		return nil, errors.E(
			errors.Op("NewOffsetStore"),
			errors.Component("projection/badger"),
			fmt.Errorf("failed to open BadgerDB: %w", err),
		)
	}

	// Create offset store
	store := &OffsetStore{
		db:           db,
		parseVersion: parseVersion,
		logger:       logging.Default().Logger,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(store)
	}

	store.logger.Debug("BadgerDB offset store initialized",
		slog.String("path", config.Path),
	)

	return store, nil
}

// Get retrieves the last applied version for a projection by name.
// Returns nil if no offset has been stored yet (indicating start from beginning).
func (o *OffsetStore) Get(ctx context.Context, name string) (synckit.Version, error) {
	if name == "" {
		return nil, errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("projection name cannot be empty"),
		)
	}

	o.mu.RLock()
	if o.closed {
		o.mu.RUnlock()
		return nil, errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("offset store is closed"),
		)
	}
	o.mu.RUnlock()

	var versionStr string
	err := o.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(name))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			versionStr = string(val)
			return nil
		})
	})

	if err == badger.ErrKeyNotFound {
		// No offset stored yet - this is normal for first-time projections
		o.logger.Debug("No offset found for projection, starting from beginning",
			slog.String("projection", name),
		)
		return nil, nil
	}

	if err != nil {
		return nil, errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			fmt.Errorf("failed to get offset for projection %s: %w", name, err),
		)
	}

	version, err := o.parseVersion(ctx, versionStr)
	if err != nil {
		return nil, errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			fmt.Errorf("failed to parse version for projection %s: %w", name, err),
		)
	}

	o.logger.Debug("Retrieved offset for projection",
		slog.String("projection", name),
		slog.String("version", versionStr),
	)

	return version, nil
}

// Set updates the last applied version for a projection.
// This operation is atomic and should be called after successfully applying events.
func (o *OffsetStore) Set(ctx context.Context, name string, v synckit.Version) error {
	if name == "" {
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("projection name cannot be empty"),
		)
	}

	if v == nil {
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("version cannot be nil"),
		)
	}

	o.mu.RLock()
	if o.closed {
		o.mu.RUnlock()
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("offset store is closed"),
		)
	}
	o.mu.RUnlock()

	versionStr := v.String()
	err := o.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(name), []byte(versionStr))
	})

	if err != nil {
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			fmt.Errorf("failed to set offset for projection %s to %s: %w", name, versionStr, err),
		)
	}

	o.logger.Debug("Updated offset for projection",
		slog.String("projection", name),
		slog.String("version", versionStr),
	)

	return nil
}

// ListProjections returns all projection names that have stored offsets.
// This is useful for administrative tasks and monitoring.
func (o *OffsetStore) ListProjections(ctx context.Context) ([]string, error) {
	o.mu.RLock()
	if o.closed {
		o.mu.RUnlock()
		return nil, errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("offset store is closed"),
		)
	}
	o.mu.RUnlock()

	var projections []string
	err := o.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // We only need keys
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			projections = append(projections, string(key))
		}
		return nil
	})

	if err != nil {
		return nil, errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			fmt.Errorf("failed to list projections: %w", err),
		)
	}

	return projections, nil
}

// Reset clears the offset for a specific projection, causing it to restart from the beginning.
// Use with caution - this will cause the projection to re-process all events.
func (o *OffsetStore) Reset(ctx context.Context, name string) error {
	if name == "" {
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("projection name cannot be empty"),
		)
	}

	o.mu.RLock()
	if o.closed {
		o.mu.RUnlock()
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("offset store is closed"),
		)
	}
	o.mu.RUnlock()

	err := o.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(name))
	})

	if err != nil && err != badger.ErrKeyNotFound {
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			fmt.Errorf("failed to reset offset for projection %s: %w", name, err),
		)
	}

	o.logger.Info("Reset projection offset",
		slog.String("projection", name),
	)

	return nil
}

// Close closes the BadgerDB instance and releases resources.
func (o *OffsetStore) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return nil
	}

	o.closed = true
	return o.db.Close()
}

// RunGC runs garbage collection on the BadgerDB.
// This should be called periodically to reclaim disk space.
func (o *OffsetStore) RunGC(ctx context.Context) error {
	o.mu.RLock()
	if o.closed {
		o.mu.RUnlock()
		return errors.E(
			errors.OpOffsetStore,
			errors.Component("projection/badger"),
			errors.KindInvalid,
			fmt.Errorf("offset store is closed"),
		)
	}
	o.mu.RUnlock()

	// Run value log garbage collection
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := o.db.RunValueLogGC(0.5) // Discard 50% or more
		if err != nil {
			if err == badger.ErrNoRewrite {
				// No GC needed
				break
			}
			return fmt.Errorf("GC failed: %w", err)
		}
	}

	return nil
}
