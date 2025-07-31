package version_test

import (
	"fmt"
	"log"

	"github.com/c0deZ3R0/go-sync-kit/version"
)

// Example_basic demonstrates basic vector clock operations.
func Example_basic() {
	// Create a new vector clock
	clock := version.NewVectorClock()
	fmt.Printf("New clock: %s\n", clock.String())

	// Increment when a node creates an event
	clock.Increment("node-1")
	fmt.Printf("After increment: %s\n", clock.String())

	// Create another clock from JSON
	otherClock, err := version.NewVectorClockFromString(`{"node-2": 3, "node-1": 1}`)
	if err != nil {
		log.Fatal(err)
	}

	// Compare clocks to determine causality
	relationship := clock.Compare(otherClock)
	switch relationship {
	case -1:
		fmt.Println("clock happened-before otherClock")
	case 1:
		fmt.Println("clock happened-after otherClock")
	case 0:
		fmt.Println("clocks are concurrent or equal")
	}

	// Output:
	// New clock: {}
	// After increment: {"node-1":1}
	// clock happened-before otherClock
}

// Example_distributedScenario demonstrates how vector clocks work in a
// distributed system with multiple nodes.
func Example_distributedScenario() {
	// Three nodes in a distributed system
	nodeA := version.NewVectorClock()
	nodeB := version.NewVectorClock()
	nodeC := version.NewVectorClock()

	fmt.Println("=== Initial State ===")
	fmt.Printf("Node A: %s\n", nodeA.String())
	fmt.Printf("Node B: %s\n", nodeB.String())
	fmt.Printf("Node C: %s\n", nodeC.String())

	fmt.Println("\n=== Node A creates an event ===")
	nodeA.Increment("A")
	fmt.Printf("Node A: %s\n", nodeA.String())

	fmt.Println("\n=== Node B creates an event independently ===")
	nodeB.Increment("B")
	fmt.Printf("Node B: %s\n", nodeB.String())
	fmt.Printf("A concurrent with B: %t\n", nodeA.IsConcurrentWith(nodeB))

	fmt.Println("\n=== Node B receives A's state and creates another event ===")
	nodeB.Merge(nodeA)
	nodeB.Increment("B")
	fmt.Printf("Node B after merge and increment: %s\n", nodeB.String())
	fmt.Printf("B happened after A: %t\n", nodeB.HappenedAfter(nodeA))

	fmt.Println("\n=== Node C joins and creates an event ===")
	nodeC.Increment("C")
	fmt.Printf("Node C: %s\n", nodeC.String())
	fmt.Printf("C concurrent with A: %t\n", nodeC.IsConcurrentWith(nodeA))
	fmt.Printf("C concurrent with B: %t\n", nodeC.IsConcurrentWith(nodeB))

	fmt.Println("\n=== Node C syncs with A and B ===")
	nodeC.Merge(nodeA)
	nodeC.Merge(nodeB)
	nodeC.Increment("C")
	fmt.Printf("Node C after sync: %s\n", nodeC.String())
	fmt.Printf("C happened after A: %t\n", nodeC.HappenedAfter(nodeA))
	fmt.Printf("C happened after B: %t\n", nodeC.HappenedAfter(nodeB))

	// Output:
	// === Initial State ===
	// Node A: {}
	// Node B: {}
	// Node C: {}
	//
	// === Node A creates an event ===
	// Node A: {"A":1}
	//
	// === Node B creates an event independently ===
	// Node B: {"B":1}
	// A concurrent with B: true
	//
	// === Node B receives A's state and creates another event ===
	// Node B after merge and increment: {"A":1,"B":2}
	// B happened after A: true
	//
	// === Node C joins and creates an event ===
	// Node C: {"C":1}
	// C concurrent with A: true
	// C concurrent with B: true
	//
	// === Node C syncs with A and B ===
	// Node C after sync: {"A":1,"B":2,"C":2}
	// C happened after A: true
	// C happened after B: true
}

// Example_serialization demonstrates how to serialize and deserialize
// vector clocks for storage or network transmission.
func Example_serialization() {
	// Create and populate a vector clock
	clock := version.NewVectorClock()
	clock.Increment("server-1")
	clock.Increment("server-2")
	clock.Increment("server-1") // server-1 creates another event

	fmt.Printf("Original clock: %s\n", clock.String())

	// Serialize to JSON string (for storage/transmission)
	jsonStr := clock.String()
	fmt.Printf("Serialized: %s\n", jsonStr)

	// Deserialize from JSON string
	restored, err := version.NewVectorClockFromString(jsonStr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Restored clock: %s\n", restored.String())
	fmt.Printf("Are they equal? %t\n", clock.IsEqual(restored))

	// Output:
	// Original clock: {"server-1":2,"server-2":1}
	// Serialized: {"server-1":2,"server-2":1}
	// Restored clock: {"server-1":2,"server-2":1}
	// Are they equal? true
}

// Example_conflictDetection shows how vector clocks can detect conflicts
// in a distributed system scenario.
func Example_conflictDetection() {
	// Simulate two users editing the same document offline
	fmt.Println("=== Two users start with the same document version ===")
	userAlice, _ := version.NewVectorClockFromString(`{"server":5}`)
	userBob, _ := version.NewVectorClockFromString(`{"server":5}`)

	fmt.Printf("Alice's version: %s\n", userAlice.String())
	fmt.Printf("Bob's version: %s\n", userBob.String())

	fmt.Println("\n=== Both users make changes offline ===")
	userAlice.Increment("alice")
	userBob.Increment("bob")

	fmt.Printf("Alice after edit: %s\n", userAlice.String())
	fmt.Printf("Bob after edit: %s\n", userBob.String())

	fmt.Println("\n=== Detect conflict when they try to sync ===")
	if userAlice.IsConcurrentWith(userBob) {
		fmt.Println("⚠️  CONFLICT DETECTED: Both users made concurrent changes!")
		fmt.Println("Need conflict resolution strategy.")
	} else if userAlice.HappenedAfter(userBob) {
		fmt.Println("Alice's changes can be applied (happened after Bob's)")
	} else if userBob.HappenedAfter(userAlice) {
		fmt.Println("Bob's changes can be applied (happened after Alice's)")
	}

	fmt.Println("\n=== After conflict resolution (merge both changes) ===")
	// Simulate conflict resolution by merging both versions
	resolved := userAlice.Clone()
	resolved.Merge(userBob)
	resolved.Increment("server") // Server creates merge event

	fmt.Printf("Resolved version: %s\n", resolved.String())
	fmt.Printf("Resolved happened after Alice: %t\n", resolved.HappenedAfter(userAlice))
	fmt.Printf("Resolved happened after Bob: %t\n", resolved.HappenedAfter(userBob))

	// Output:
	// === Two users start with the same document version ===
	// Alice's version: {"server":5}
	// Bob's version: {"server":5}
	//
	// === Both users make changes offline ===
	// Alice after edit: {"alice":1,"server":5}
	// Bob after edit: {"bob":1,"server":5}
	//
	// === Detect conflict when they try to sync ===
	// ⚠️  CONFLICT DETECTED: Both users made concurrent changes!
	// Need conflict resolution strategy.
	//
	// === After conflict resolution (merge both changes) ===
	// Resolved version: {"alice":1,"bob":1,"server":6}
	// Resolved happened after Alice: true
	// Resolved happened after Bob: true
}

// Example_mapConstruction shows how to create vector clocks from maps,
// useful for testing or when you have clock data in map format.
func Example_mapConstruction() {
	// Create from a map (useful for testing)
	clockData := map[string]uint64{
		"node-1": 5,
		"node-2": 3,
		"node-3": 1,
	}

	clock := version.NewVectorClockFromMap(clockData)
	fmt.Printf("Clock from map: %s\n", clock.String())
	fmt.Printf("Size: %d nodes\n", clock.Size())
	fmt.Printf("Node-2 clock value: %d\n", clock.GetClock("node-2"))

	// Get all clocks as a map (returns a copy)
	allClocks := clock.GetAllClocks()
	fmt.Printf("All clocks: %+v\n", allClocks)

	// Modifying the returned map doesn't affect the original
	allClocks["node-1"] = 100
	fmt.Printf("After modifying returned map, original node-1: %d\n", clock.GetClock("node-1"))

	// Output:
	// Clock from map: {"node-1":5,"node-2":3,"node-3":1}
	// Size: 3 nodes
	// Node-2 clock value: 3
	// All clocks: map[node-1:5 node-2:3 node-3:1]
	// After modifying returned map, original node-1: 5
}
