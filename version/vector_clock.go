// Package version provides various version implementations for the go-sync-kit library.
// Vector clocks are particularly useful for tracking causality in distributed systems.
package version

import (
	"encoding/json"
	"fmt"
	"strings"

synckit "github.com/c0deZ3R0/go-sync-kit"
)

// VectorClockError represents errors that can occur during vector clock operations
type VectorClockError struct {
	Msg string
}

func (e *VectorClockError) Error() string {
	return e.Msg
}

// Vector clock constraints
const (
	// MaxNodeIDLength is the maximum allowed length for a node ID
	MaxNodeIDLength = 255

	// MaxNodes is the maximum number of nodes that can be tracked
	// This prevents memory issues from unbounded growth
	MaxNodes = 1000
)

// VectorClock implements the Version interface using a map of node IDs to logical clocks.
// It is used to determine the partial ordering of events in a distributed system.
// A vector clock can determine if one version happened-before, happened-after, or is
// concurrent with another version.
//
// Vector clocks are ideal for:
// - Offline-first applications
// - Multi-master replication
// - Conflict detection in distributed systems
// - Event sourcing with multiple writers
type VectorClock struct {
	// clocks maps a node (or replica) ID to its logical clock value.
	// Each node maintains its own clock and observes others' clocks.
	clocks map[string]uint64
}

// Compile-time check to ensure VectorClock satisfies the Version interface
var _ synckit.Version = (*VectorClock)(nil)

// NewVectorClock creates an empty VectorClock.
// This is the primary constructor for creating vector clocks.
func NewVectorClock() *VectorClock {
	return &VectorClock{
		clocks: make(map[string]uint64),
	}
}

// NewVectorClockFromString attempts to deserialize a JSON string into a VectorClock.
// This is useful for reconstructing a version received from storage or over the network.
//
// The expected format is a JSON object mapping node IDs to clock values:
// {"node-1": 5, "node-2": 3}
//
// Returns an error if the input string is not valid JSON or contains invalid data.
func NewVectorClockFromString(data string) (*VectorClock, error) {
	if strings.TrimSpace(data) == "" || data == "{}" {
		return NewVectorClock(), nil
	}

	vc := NewVectorClock()
	if err := json.Unmarshal([]byte(data), &vc.clocks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vector clock from '%s': %w", data, err)
	}

	// Validate that all values are non-negative
	for nodeID, clockValue := range vc.clocks {
		if nodeID == "" {
			return nil, fmt.Errorf("vector clock contains empty node ID")
		}
		// Note: uint64 is always non-negative, but we keep this check for clarity
		if clockValue < 0 {
			return nil, fmt.Errorf("vector clock contains negative value for node '%s': %d", nodeID, clockValue)
		}
	}

	return vc, nil
}

// NewVectorClockFromMap creates a VectorClock from a map of node IDs to clock values.
// This is useful for testing or when you have clock data in map format.
// The input map is copied to prevent external mutations.
func NewVectorClockFromMap(clocks map[string]uint64) *VectorClock {
	vc := NewVectorClock()
	for nodeID, clockValue := range clocks {
		vc.clocks[nodeID] = clockValue
	}
	return vc
}

// Increment increases the logical clock for a given node ID.
// This should be called whenever a node generates a new event.
//
// Example usage:
//   clock := NewVectorClock()
//   clock.Increment("node-1") // {"node-1": 1}
//   clock.Increment("node-1") // {"node-1": 2}
//
// Returns an error if:
// - The node ID is empty
// - The node ID exceeds MaxNodeIDLength
// - Adding this node would exceed MaxNodes
func (vc *VectorClock) Increment(nodeID string) error {
	if nodeID == "" {
		return &VectorClockError{Msg: "node ID cannot be empty"}
	}

	if len(nodeID) > MaxNodeIDLength {
		return &VectorClockError{Msg: fmt.Sprintf("node ID length exceeds maximum of %d characters", MaxNodeIDLength)}
	}

	// Check if adding this node would exceed the maximum
	if _, exists := vc.clocks[nodeID]; !exists && len(vc.clocks) >= MaxNodes {
		return &VectorClockError{Msg: fmt.Sprintf("cannot track more than %d nodes", MaxNodes)}
	}

	vc.clocks[nodeID]++
	return nil
}

// Merge combines this vector clock with another, taking the maximum clock value
// for each node present in either clock. This is essential for maintaining causal history
// when a node receives events from another node.
//
// The merge operation ensures that the resulting clock incorporates all the causal
// history that both clocks have observed.
//
// Example:
//   clock1 := {"node-1": 2, "node-2": 1}
//   clock2 := {"node-1": 1, "node-3": 2}
//   clock1.Merge(clock2) results in {"node-1": 2, "node-2": 1, "node-3": 2}
//
// Returns an error if:
// - The resulting merged clock would exceed MaxNodes
// - Any node ID in the other clock exceeds MaxNodeIDLength
func (vc *VectorClock) Merge(other *VectorClock) error {
	if other == nil {
		return nil
	}

	// Count how many new nodes would be added
	newNodeCount := 0
	for nodeID := range other.clocks {
		if len(nodeID) > MaxNodeIDLength {
			return &VectorClockError{Msg: fmt.Sprintf("other clock contains node ID exceeding maximum length of %d", MaxNodeIDLength)}
		}
		if _, exists := vc.clocks[nodeID]; !exists {
			newNodeCount++
		}
	}

	// Check if merging would exceed the maximum node limit
	if len(vc.clocks)+newNodeCount > MaxNodes {
		return &VectorClockError{Msg: fmt.Sprintf("merging would exceed maximum of %d nodes", MaxNodes)}
	}

	// Perform the merge
	for nodeID, otherClock := range other.clocks {
		if currentClock, exists := vc.clocks[nodeID]; !exists || otherClock > currentClock {
			vc.clocks[nodeID] = otherClock
		}
	}

	return nil
}

// Compare determines the causal relationship between two VectorClocks.
// It returns:
//   - -1: if this VectorClock happened-before the other (this ≺ other)
//   -  1: if this VectorClock happened-after the other (this ≻ other)  
//   -  0: if they are concurrent or identical (this || other)
//
// The comparison follows the standard vector clock partial ordering:
// - A ≺ B if A[i] ≤ B[i] for all i, and A[j] < B[j] for at least one j
// - A ≻ B if B ≺ A
// - A || B if neither A ≺ B nor B ≺ A (concurrent)
func (vc *VectorClock) Compare(other synckit.Version) int {
	otherVC, ok := other.(*VectorClock)
	if !ok {
		// Cannot compare with a different Version implementation.
		// Treat as concurrent to maintain consistency.
		return 0
	}

	if otherVC == nil {
		// Non-nil clock is always after nil
		if vc.IsZero() {
			return 0 // Both are effectively zero
		}
		return 1
	}

	// Collect all node IDs from both clocks
	allNodes := make(map[string]bool)
	for nodeID := range vc.clocks {
		allNodes[nodeID] = true
	}
	for nodeID := range otherVC.clocks {
		allNodes[nodeID] = true
	}


	thisHappenedBefore := false
	otherHappenedBefore := false
	
	for nodeID := range allNodes {
		thisClock := vc.clocks[nodeID]  // Defaults to 0 if not present
		otherClock := otherVC.clocks[nodeID]  // Defaults to 0 if not present

		if thisClock < otherClock {
			thisHappenedBefore = true
		} else if thisClock > otherClock {
			otherHappenedBefore = true
		}
	}

	if thisHappenedBefore && !otherHappenedBefore {
		return -1 // This happened-before other
	}
	if otherHappenedBefore && !thisHappenedBefore {
		return 1 // This happened-after other
	}

	return 0 // Concurrent or equal
}

// String serializes the VectorClock to a JSON string for storage or transport.
// The format is a JSON object mapping node IDs to their clock values.
//
// Examples:
//   - Empty clock: "{}"
//   - Single node: {"node-1":5}
//   - Multiple nodes: {"node-1":5,"node-2":3}
func (vc *VectorClock) String() string {
	if vc.IsZero() {
		return "{}"
	}

	data, err := json.Marshal(vc.clocks)
	if err != nil {
		// This should not happen with a map[string]uint64, but handle gracefully
		return fmt.Sprintf(`{"error":"serialization failed: %s"}`, err.Error())
	}

	return string(data)
}

// IsZero returns true if the VectorClock is empty (no nodes have been observed).
// This is equivalent to the initial state of a vector clock.
func (vc *VectorClock) IsZero() bool {
	return len(vc.clocks) == 0
}

// Clone creates a deep copy of the VectorClock.
// This is useful when you need to create a snapshot or avoid mutations.
func (vc *VectorClock) Clone() *VectorClock {
	clone := NewVectorClock()
	for nodeID, clockValue := range vc.clocks {
		clone.clocks[nodeID] = clockValue
	}
	return clone
}

// GetClock returns the clock value for a specific node ID.
// Returns 0 if the node ID has not been observed in this vector clock.
func (vc *VectorClock) GetClock(nodeID string) uint64 {
	return vc.clocks[nodeID] // Returns 0 if not present
}

// GetAllClocks returns a copy of the internal clock map.
// This prevents external mutation of the vector clock's internal state.
func (vc *VectorClock) GetAllClocks() map[string]uint64 {
	result := make(map[string]uint64)
	for nodeID, clockValue := range vc.clocks {
		result[nodeID] = clockValue
	}
	return result
}

// Size returns the number of nodes tracked by this vector clock.
func (vc *VectorClock) Size() int {
	return len(vc.clocks)
}

// IsConcurrentWith returns true if this vector clock is concurrent with another.
// Two vector clocks are concurrent if neither happened-before the other.
func (vc *VectorClock) IsConcurrentWith(other *VectorClock) bool {
	return vc.Compare(other) == 0 && !vc.IsEqual(other)
}

// IsEqual returns true if two vector clocks are identical.
func (vc *VectorClock) IsEqual(other *VectorClock) bool {
	if other == nil {
		return vc.IsZero()
	}

	if len(vc.clocks) != len(other.clocks) {
		return false
	}

	for nodeID, clockValue := range vc.clocks {
		if other.clocks[nodeID] != clockValue {
			return false
		}
	}

	return true
}

// HappenedBefore returns true if this vector clock happened-before the other.
func (vc *VectorClock) HappenedBefore(other *VectorClock) bool {
	return vc.Compare(other) == -1
}

// HappenedAfter returns true if this vector clock happened-after the other.
func (vc *VectorClock) HappenedAfter(other *VectorClock) bool {
	return vc.Compare(other) == 1
}
