package version

import (
	"fmt"
	"testing"
)

// BenchmarkVectorClockOperations benchmarks core vector clock operations
// as mentioned in the code review
func BenchmarkVectorClockOperations(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   func(b *testing.B)
	}{
		{"Increment", benchmarkIncrement},
		{"Compare", benchmarkCompare},
		{"Merge", benchmarkMerge},
		{"Clone", benchmarkClone},
		{"Serialization", benchmarkSerialization},
		{"Deserialization", benchmarkDeserialization},
	}
	
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, benchmark.fn)
	}
}

func benchmarkIncrement(b *testing.B) {
	clockSizes := []int{1, 5, 10, 25, 50, 100}
	
	for _, size := range clockSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			clock := NewVectorClock()
			
			// Pre-populate with size-1 nodes to test increment on existing
			for i := 0; i < size-1; i++ {
				clock.Increment(fmt.Sprintf("node-%d", i))
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Alternate between existing and new nodes
				nodeID := fmt.Sprintf("node-%d", i%size)
				clock.Increment(nodeID)
			}
		})
	}
}

func benchmarkCompare(b *testing.B) {
	scenarios := []struct {
		name     string
		setupFn  func() (*VectorClock, *VectorClock)
	}{
		{"Identical", setupIdenticalClocks},
		{"HappenedBefore", setupHappenedBeforeClocks},
		{"Concurrent", setupConcurrentClocks},
		{"Large", setupLargeClocks},
		{"Sparse", setupSparseClocks},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			vc1, vc2 := scenario.setupFn()
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				vc1.Compare(vc2)
			}
		})
	}
}

func benchmarkMerge(b *testing.B) {
	mergeScenarios := []struct {
		name    string
		setupFn func() (*VectorClock, *VectorClock)
	}{
		{"SmallClocks", setupSmallMergeClocks},
		{"MediumClocks", setupMediumMergeClocks},
		{"LargeClocks", setupLargeMergeClocks},
		{"DisjointClocks", setupDisjointMergeClocks},
		{"OverlappingClocks", setupOverlappingMergeClocks},
	}
	
	for _, scenario := range mergeScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			originalVc1, originalVc2 := scenario.setupFn()
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Clone to avoid modifying the original clocks
				vc1 := originalVc1.Clone()
				vc1.Merge(originalVc2)
			}
		})
	}
}

func benchmarkClone(b *testing.B) {
	clockSizes := []int{1, 10, 50, 100, 250}
	
	for _, size := range clockSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			clock := createClockWithSize(size)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = clock.Clone()
			}
		})
	}
}

func benchmarkSerialization(b *testing.B) {
	clockSizes := []int{1, 10, 50, 100, 250}
	
	for _, size := range clockSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			clock := createClockWithSize(size)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = clock.String()
			}
		})
	}
}

func benchmarkDeserialization(b *testing.B) {
	clockSizes := []int{1, 10, 50, 100, 250}
	
	for _, size := range clockSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			clock := createClockWithSize(size)
			serialized := clock.String()
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := NewVectorClockFromString(serialized)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkVectorClockConcurrentOperations benchmarks vector clock operations
// under concurrent access patterns
func BenchmarkVectorClockConcurrentOperations(b *testing.B) {
	scenarios := []struct {
		name string
		fn   func(b *testing.B)
	}{
		{"ConcurrentIncrement", benchmarkConcurrentIncrement},
		{"ConcurrentCompare", benchmarkConcurrentCompare},
		{"ConcurrentMerge", benchmarkConcurrentMerge},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, scenario.fn)
	}
}

func benchmarkConcurrentIncrement(b *testing.B) {
	const numGoroutines = 10
	const clockSize = 50
	
	b.RunParallel(func(pb *testing.PB) {
		clock := createClockWithSize(clockSize)
		nodeCounter := 0
		
		for pb.Next() {
			nodeID := fmt.Sprintf("concurrent-node-%d", nodeCounter%clockSize)
			nodeCounter++
			
			// Each goroutine increments different nodes
			clock.Increment(nodeID)
		}
	})
}

func benchmarkConcurrentCompare(b *testing.B) {
	clock1 := createClockWithSize(100)
	clock2 := createClockWithSize(100)
	
	// Make clock2 happen after clock1
	for i := 0; i < 50; i++ {
		clock2.Increment(fmt.Sprintf("node-%d", i))
	}
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			clock1.Compare(clock2)
		}
	})
}

func benchmarkConcurrentMerge(b *testing.B) {
	baseClock := createClockWithSize(50)
	
	b.RunParallel(func(pb *testing.PB) {
		mergeClock := createClockWithSize(25)
		
		for pb.Next() {
			clock := baseClock.Clone()
			clock.Merge(mergeClock)
		}
	})
}

// BenchmarkVectorClockRealWorldScenarios benchmarks realistic usage patterns
func BenchmarkVectorClockRealWorldScenarios(b *testing.B) {
	scenarios := []struct {
		name string
		fn   func(b *testing.B)
	}{
		{"DistributedSystem", benchmarkDistributedSystemScenario},
		{"EventSourcing", benchmarkEventSourcingScenario},
		{"CRDTOperations", benchmarkCRDTOperations},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, scenario.fn)
	}
}

func benchmarkDistributedSystemScenario(b *testing.B) {
	// Simulate a distributed system with 5 nodes
	const numNodes = 5
	nodes := make([]*VectorClock, numNodes)
	
	for i := 0; i < numNodes; i++ {
		nodes[i] = NewVectorClock()
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodeIndex := i % numNodes
		nodeID := fmt.Sprintf("node-%d", nodeIndex)
		
		// Each node creates an event
		nodes[nodeIndex].Increment(nodeID)
		
		// Periodically sync with other nodes
		if i%10 == 0 {
			otherNodeIndex := (nodeIndex + 1) % numNodes
			nodes[nodeIndex].Merge(nodes[otherNodeIndex])
		}
	}
}

func benchmarkEventSourcingScenario(b *testing.B) {
	// Simulate an event sourcing scenario where we track causality
	aggregateClock := NewVectorClock()
	eventClocks := make([]*VectorClock, 0, b.N)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a new event with current aggregate state
		eventClock := aggregateClock.Clone()
		eventClock.Increment(fmt.Sprintf("aggregate-%d", i%10))
		
		// Store the event clock
		eventClocks = append(eventClocks, eventClock)
		
		// Update aggregate clock
		aggregateClock.Merge(eventClock)
	}
}

func benchmarkCRDTOperations(b *testing.B) {
	// Simulate CRDT operations where we need to detect concurrent updates
	const numReplicas = 3
	replicas := make([]*VectorClock, numReplicas)
	
	for i := 0; i < numReplicas; i++ {
		replicas[i] = NewVectorClock()
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replicaIndex := i % numReplicas
		replicaID := fmt.Sprintf("replica-%d", replicaIndex)
		
		// Each replica performs an operation
		replicas[replicaIndex].Increment(replicaID)
		
		// Check for conflicts with other replicas
		if i%5 == 0 {
			for j := 0; j < numReplicas; j++ {
				if j != replicaIndex {
					if replicas[replicaIndex].IsConcurrentWith(replicas[j]) {
						// Resolve conflict by merging
						replicas[replicaIndex].Merge(replicas[j])
					}
				}
			}
		}
	}
}

// Helper functions for setting up test scenarios

func setupIdenticalClocks() (*VectorClock, *VectorClock) {
	clock1 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 5,
		"node-2": 3,
		"node-3": 7,
	})
	clock2 := clock1.Clone()
	return clock1, clock2
}

func setupHappenedBeforeClocks() (*VectorClock, *VectorClock) {
	clock1 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 3,
		"node-2": 2,
		"node-3": 1,
	})
	clock2 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 5,
		"node-2": 4,
		"node-3": 3,
	})
	return clock1, clock2
}

func setupConcurrentClocks() (*VectorClock, *VectorClock) {
	clock1 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 5,
		"node-2": 2,
	})
	clock2 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 3,
		"node-2": 4,
	})
	return clock1, clock2
}

func setupLargeClocks() (*VectorClock, *VectorClock) {
	clock1 := createClockWithSize(100)
	clock2 := createClockWithSize(100)
	
	// Make them have some overlap but also some differences
	for i := 0; i < 50; i++ {
		clock2.Increment(fmt.Sprintf("node-%d", i))
	}
	
	return clock1, clock2
}

func setupSparseClocks() (*VectorClock, *VectorClock) {
	// Create clocks with only a few nodes but large values
	clock1 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 1000,
		"node-5": 2000,
	})
	clock2 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 1500,
		"node-3": 3000,
	})
	return clock1, clock2
}

func setupSmallMergeClocks() (*VectorClock, *VectorClock) {
	clock1 := createClockWithSize(5)
	clock2 := createClockWithSize(3)
	return clock1, clock2
}

func setupMediumMergeClocks() (*VectorClock, *VectorClock) {
	clock1 := createClockWithSize(25)
	clock2 := createClockWithSize(20)
	return clock1, clock2
}

func setupLargeMergeClocks() (*VectorClock, *VectorClock) {
	clock1 := createClockWithSize(100)
	clock2 := createClockWithSize(75)
	return clock1, clock2
}

func setupDisjointMergeClocks() (*VectorClock, *VectorClock) {
	clock1 := NewVectorClock()
	clock2 := NewVectorClock()
	
	// Completely disjoint node sets
	for i := 0; i < 10; i++ {
		clock1.Increment(fmt.Sprintf("set-a-node-%d", i))
		clock2.Increment(fmt.Sprintf("set-b-node-%d", i))
	}
	
	return clock1, clock2
}

func setupOverlappingMergeClocks() (*VectorClock, *VectorClock) {
	clock1 := NewVectorClock()
	clock2 := NewVectorClock()
	
	// Overlapping node sets
	for i := 0; i < 15; i++ {
		clock1.Increment(fmt.Sprintf("node-%d", i))
	}
	for i := 5; i < 20; i++ {
		clock2.Increment(fmt.Sprintf("node-%d", i))
	}
	
	return clock1, clock2
}

func createClockWithSize(size int) *VectorClock {
	clock := NewVectorClock()
	for i := 0; i < size; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		// Create some variation in clock values
		for j := 0; j < (i%5)+1; j++ {
			clock.Increment(nodeID)
		}
	}
	return clock
}
