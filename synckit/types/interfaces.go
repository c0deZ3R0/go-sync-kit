// Package types: interface surface for synckit implementors.
//
// This file centralizes the stable EventStore and Transport interfaces so that
// implementors can depend on a single location without import cycles.
//
// Implementors: add compile-time checks in your packages:
//   var _ types.EventStore = (*MyStore)(nil)
//   var _ types.Transport = (*MyTransport)(nil)
package types

import (
	"context"
)

// EventHandler handles batches of events delivered by a Transport subscription.
// It is an alias for the function type used by Subscribe and kept here to allow
// documentation and consistent type naming without changing the method signature.
type EventHandler = func([]EventWithVersion) error

// Filter is an intentionally simple key/value pair used for forward-compatible
// filtering across Store/Transport operations. Implementations may choose to
// support a subset (e.g., tenant, type, tag).
//
// Common filter keys:
//   - "type": Filter by event type
//   - "tenant": Filter by tenant ID (from metadata)
//   - "aggregate_id": Filter by aggregate ID
type Filter struct{ Key, Value string }

// EventStore provides persistence for events.
//
// Implementors should document version encoding/decoding behavior and ensure:
// - LatestVersion returns a valid non-zero version when events exist
// - ParseVersion accepts serialized forms used by transports/APIs
// - All methods are context-aware and cancel promptly
//
// Implementors:
//   var _ EventStore = (*MyStore)(nil)
type EventStore interface {
	// Store persists an event with the provided version (version may be ignored
	// by implementations that auto-generate versions, but must not fail).
	Store(ctx context.Context, event Event, version Version) error

	// Load retrieves all events strictly after the provided version.
	// Optional filters can be provided for type, tenant, aggregate_id, etc.
	// Variadic filters parameter is backward compatible - existing calls work unchanged.
	Load(ctx context.Context, since Version, filters ...Filter) ([]EventWithVersion, error)

	// LoadByAggregate retrieves events for a specific aggregate after the version.
	// Optional filters can be provided for type, tenant, etc.
	// Variadic filters parameter is backward compatible - existing calls work unchanged.
	LoadByAggregate(ctx context.Context, aggregateID string, since Version, filters ...Filter) ([]EventWithVersion, error)

	// LatestVersion returns the latest version in the store.
	LatestVersion(ctx context.Context) (Version, error)

	// ParseVersion converts a string representation into a Version implementation.
	ParseVersion(ctx context.Context, versionStr string) (Version, error)

	// Close releases resources.
	Close() error
}

// Transport handles communication with a remote endpoint.
//
// Implementors should document ordering, delivery guarantees, and any
// authorization/error semantics.
//
// Implementors:
//   var _ Transport = (*MyTransport)(nil)
type Transport interface {
	// Push sends a batch of events to the remote endpoint.
	Push(ctx context.Context, events []EventWithVersion) error

	// Pull retrieves events from the remote endpoint strictly after the version.
	Pull(ctx context.Context, since Version) ([]EventWithVersion, error)

	// GetLatestVersion efficiently retrieves the latest remote version without pulling events.
	GetLatestVersion(ctx context.Context) (Version, error)

	// Subscribe listens for real-time updates. Implementations may be streaming
	// or event-based callbacks. Handler should be invoked with batches.
	Subscribe(ctx context.Context, handler EventHandler) error

	// Close closes the transport connection and releases resources.
	Close() error
}
