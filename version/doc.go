// Package version provides various version implementations for the synckit library.
//
// This package contains implementations of the sync.Version interface, which is used
// to track causality and ordering of events in distributed systems.
//
// # Vector Clocks
//
// The primary implementation is VectorClock, which provides robust causality tracking
// for distributed systems. Vector clocks can determine if one version happened-before,
// happened-after, or is concurrent with another version.
//
// # Basic Usage
//
//	import "github.com/c0deZ3R0/go-sync-kit/version"
//
//	// Create a new vector clock
//	clock := version.NewVectorClock()
//
//	// Increment when a node creates an event
//	clock.Increment("node-1")
//	
//	// Merge when receiving events from another node
//	otherClock := version.NewVectorClockFromString(`{"node-2": 3}`)
//	clock.Merge(otherClock)
//
//	// Compare clocks to determine causality
//	relationship := clock.Compare(otherClock)
//	switch relationship {
//	case -1:
//		fmt.Println("clock happened-before otherClock")
//	case 1:
//		fmt.Println("clock happened-after otherClock")
//	case 0:
//		fmt.Println("clocks are concurrent or equal")
//	}
//
// # Distributed System Example
//
// Vector clocks are particularly useful in distributed systems where nodes
// may be offline and need to sync later:
//
//	// Three nodes in a distributed system
//	nodeA := version.NewVectorClock()
//	nodeB := version.NewVectorClock()
//	nodeC := version.NewVectorClock()
//
//	// Node A creates an event
//	nodeA.Increment("A")
//
//	// Node B creates an event independently (concurrent)
//	nodeB.Increment("B")
//
//	// Node A and B are concurrent at this point
//	fmt.Println("A concurrent with B:", nodeA.IsConcurrentWith(nodeB))
//
//	// Node B receives A's state and creates another event
//	nodeB.Merge(nodeA)
//	nodeB.Increment("B")
//
//	// Now B happened-after A
//	fmt.Println("B happened after A:", nodeB.HappenedAfter(nodeA))
//
// # Serialization
//
// Vector clocks can be serialized to JSON for storage or network transmission:
//
//	clock := version.NewVectorClock()
//	clock.Increment("node-1")
//	clock.Increment("node-2")
//
//	// Serialize to JSON string
//	jsonStr := clock.String() // {"node-1":1,"node-2":1}
//
//	// Deserialize from JSON string
//	restored, err := version.NewVectorClockFromString(jsonStr)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Integration with go-sync-kit
//
// Vector clocks implement the sync.Version interface and can be used with
// any EventStore or Transport implementation:
//
//	import (
//		"github.com/c0deZ3R0/go-sync-kit"
//		"github.com/c0deZ3R0/go-sync-kit/version"
//		"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
//	)
//
//	// Create a vector clock version
//	currentVersion := version.NewVectorClock()
//	currentVersion.Increment("my-node-id")
//
//	// Use with EventStore
//	store, _ := sqlite.NewWithDataSource("events.db")
//	err := store.Store(ctx, event, currentVersion)
//
//	// The EventStore will handle version parsing automatically
//	events, err := store.Load(ctx, currentVersion)
//
// # Performance Considerations
//
// Vector clocks grow with the number of nodes in the system. In systems with
// many nodes, consider:
//
// - Using node ID prefixes to group related nodes
// - Implementing periodic cleanup of inactive nodes
// - Monitoring vector clock size in production
//
// The implementation is optimized for typical distributed system usage with
// reasonable numbers of nodes (dozens to hundreds).
package version
