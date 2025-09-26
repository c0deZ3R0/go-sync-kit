package synckit

import (
	"errors"
)

// Preset configurations for common SyncNode use cases.
// These functions provide convenient ways to create SyncNodes with typical configurations.
// To avoid import cycles, actual store/transport instances should be created by the caller.

// NewInMemoryNode creates a SyncNode with the provided in-memory store and transport.
// This is perfect for development, testing, and examples where no persistence is needed.
//
// Example usage:
//
//	store := memstore.New()
//	transport := memchan.New(16)
//	node, err := synckit.NewInMemoryNode(store, transport)
func NewInMemoryNode(store EventStore, transport Transport) (SyncNode, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if transport == nil {
		return nil, errors.New("transport cannot be nil")
	}
	return NewNode(
		WithStore(store),
		WithTransport(transport),
	)
}

// NewHTTPServerNode creates a SyncNode configured with the provided store and transport.
// The caller should provide an appropriate HTTP server transport implementation.
//
// Example usage:
//
//	store := sqlite.New("app.db")
//	transport := httptransport.NewTransport("", nil, nil, nil) // Configure for server use
//	node, err := synckit.NewHTTPServerNode(store, transport)
func NewHTTPServerNode(store EventStore, transport Transport) (SyncNode, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if transport == nil {
		return nil, errors.New("transport cannot be nil")
	}
	return NewNode(
		WithStore(store),
		WithTransport(transport),
	)
}

// NewHTTPClientNode creates a SyncNode configured as an HTTP client.
// The caller should provide an appropriate HTTP client transport configured for the server URL.
//
// Example usage:
//
//	store := sqlite.New("client.db")
//	transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)
//	node, err := synckit.NewHTTPClientNode(store, transport)
func NewHTTPClientNode(store EventStore, transport Transport) (SyncNode, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if transport == nil {
		return nil, errors.New("transport cannot be nil")
	}
	return NewNode(
		WithStore(store),
		WithTransport(transport),
	)
}
