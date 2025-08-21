// Package storage defines the base storage interfaces used by go-sync-kit.
// Concrete implementations are in subpackages like storage/sqlite, storage/postgres, etc.
package storage

import "context"

// Storage is the base interface for key-value storage operations used by health checks.
// This is separate from synckit.EventStore which is event-specific.
type Storage interface {
	// Put stores data with the given key
	Put(ctx context.Context, key string, data []byte) error

	// Get retrieves data for the given key
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes data for the given key
	Delete(ctx context.Context, key string) error
}
