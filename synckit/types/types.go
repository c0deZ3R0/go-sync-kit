// Package types contains shared types used across the synckit ecosystem.
// This package exists to prevent import cycles between synckit and its subpackages.
package types

// Event represents a syncable event in the system.
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

// EventWithVersion pairs an event with its version information.
type EventWithVersion struct {
	Event   Event
	Version Version
}
