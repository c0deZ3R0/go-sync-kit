package storage

import (
	"context"
	"errors"
)

// Common storage errors
var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrStorageClosed = errors.New("storage is closed")
	ErrInvalidKey    = errors.New("invalid key")
	ErrInvalidValue  = errors.New("invalid value")
)

// Storage defines the interface for storage backends.
type Storage interface {
	// Get retrieves data for the given key.
	Get(ctx context.Context, key string) ([]byte, error)

	// Put stores data for the given key.
	Put(ctx context.Context, key string, value []byte) error

	// Delete removes the data for the given key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in storage.
	Exists(ctx context.Context, key string) (bool, error)

	// Close closes the storage backend.
	Close() error
}

// MemoryStorage is a simple in-memory storage implementation for testing.
type MemoryStorage struct {
	data   map[string][]byte
	closed bool
}

// NewMemoryStorage creates a new in-memory storage backend.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string][]byte),
	}
}

func (m *MemoryStorage) Get(ctx context.Context, key string) ([]byte, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	if key == "" {
		return nil, ErrInvalidKey
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	value, exists := m.data[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	// Return a copy to prevent modification
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

func (m *MemoryStorage) Put(ctx context.Context, key string, value []byte) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidKey
	}
	if value == nil {
		return ErrInvalidValue
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Store a copy to prevent modification
	stored := make([]byte, len(value))
	copy(stored, value)
	m.data[key] = stored
	return nil
}

func (m *MemoryStorage) Delete(ctx context.Context, key string) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	delete(m.data, key)
	return nil
}

func (m *MemoryStorage) Exists(ctx context.Context, key string) (bool, error) {
	if m.closed {
		return false, ErrStorageClosed
	}
	if key == "" {
		return false, ErrInvalidKey
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	_, exists := m.data[key]
	return exists, nil
}

func (m *MemoryStorage) Close() error {
	m.closed = true
	m.data = nil
	return nil
}
