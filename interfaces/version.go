// Package interfaces defines core interfaces used across the sync kit packages
// to avoid circular dependencies.
package interfaces

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
