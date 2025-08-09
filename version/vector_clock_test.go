package version

import (
	"testing"
)

func TestNewVectorClock(t *testing.T) {
	vc := NewVectorClock()

	if vc == nil {
		t.Fatal("NewVectorClock() returned nil")
	}

	if !vc.IsZero() {
		t.Error("New vector clock should be zero")
	}

	if vc.Size() != 0 {
		t.Errorf("New vector clock size should be 0, got %d", vc.Size())
	}

	if vc.String() != "{}" {
		t.Errorf("New vector clock string should be '{}', got '%s'", vc.String())
	}
}

func TestNewVectorClockFromString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    map[string]uint64
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: false,
			expected:    map[string]uint64{},
		},
		{
			name:        "empty JSON object",
			input:       "{}",
			expectError: false,
			expected:    map[string]uint64{},
		},
		{
			name:        "single node",
			input:       `{"node-1":5}`,
			expectError: false,
			expected:    map[string]uint64{"node-1": 5},
		},
		{
			name:        "multiple nodes",
			input:       `{"node-1":5,"node-2":3}`,
			expectError: false,
			expected:    map[string]uint64{"node-1": 5, "node-2": 3},
		},
		{
			name:        "invalid JSON",
			input:       `{"node-1":}`,
			expectError: true,
			expected:    nil,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			expectError: false,
			expected:    map[string]uint64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc, err := NewVectorClockFromString(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if vc == nil {
				t.Fatal("Expected vector clock but got nil")
			}

			clocks := vc.GetAllClocks()
			if len(clocks) != len(tt.expected) {
				t.Errorf("Expected %d clocks, got %d", len(tt.expected), len(clocks))
			}

			for nodeID, expectedValue := range tt.expected {
				if actualValue := vc.GetClock(nodeID); actualValue != expectedValue {
					t.Errorf("Expected clock for %s to be %d, got %d", nodeID, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestNewVectorClockFromMap(t *testing.T) {
	input := map[string]uint64{
		"node-1": 5,
		"node-2": 3,
	}

	vc := NewVectorClockFromMap(input)

	if vc.GetClock("node-1") != 5 {
		t.Errorf("Expected node-1 clock to be 5, got %d", vc.GetClock("node-1"))
	}

	if vc.GetClock("node-2") != 3 {
		t.Errorf("Expected node-2 clock to be 3, got %d", vc.GetClock("node-2"))
	}

	// Modify original map to ensure it was copied
	input["node-1"] = 10
	if vc.GetClock("node-1") != 5 {
		t.Error("Vector clock was not properly isolated from input map")
	}
}

func TestVectorClock_Increment(t *testing.T) {
	vc := NewVectorClock()

	// Test incrementing a new node
	err := vc.Increment("node-1")
	if err != nil {
		t.Errorf("Unexpected error on increment: %v", err)
	}
	if vc.GetClock("node-1") != 1 {
		t.Errorf("Expected node-1 clock to be 1, got %d", vc.GetClock("node-1"))
	}

	// Test incrementing an existing node
	err = vc.Increment("node-1")
	if err != nil {
		t.Errorf("Unexpected error on increment: %v", err)
	}
	if vc.GetClock("node-1") != 2 {
		t.Errorf("Expected node-1 clock to be 2, got %d", vc.GetClock("node-1"))
	}

	// Test incrementing a different node
	err = vc.Increment("node-2")
	if err != nil {
		t.Errorf("Unexpected error on increment: %v", err)
	}
	if vc.GetClock("node-2") != 1 {
		t.Errorf("Expected node-2 clock to be 1, got %d", vc.GetClock("node-2"))
	}

	// Ensure first node wasn't affected
	if vc.GetClock("node-1") != 2 {
		t.Errorf("Expected node-1 clock to remain 2, got %d", vc.GetClock("node-1"))
	}

	// Test incrementing empty node ID (should be ignored)
	err = vc.Increment("")
	if err == nil {
		t.Errorf("Expected error on empty node ID, got none")
	}
	if vc.Size() != 2 {
		t.Errorf("Empty node ID increment should be ignored, size should be 2, got %d", vc.Size())
	}
}

func TestVectorClock_Merge(t *testing.T) {
	tests := []struct {
		name     string
		clock1   map[string]uint64
		clock2   map[string]uint64
		expected map[string]uint64
	}{
		{
			name:     "merge with empty clock",
			clock1:   map[string]uint64{"node-1": 2},
			clock2:   map[string]uint64{},
			expected: map[string]uint64{"node-1": 2},
		},
		{
			name:     "merge empty with non-empty",
			clock1:   map[string]uint64{},
			clock2:   map[string]uint64{"node-1": 2},
			expected: map[string]uint64{"node-1": 2},
		},
		{
			name:     "merge with higher values",
			clock1:   map[string]uint64{"node-1": 2, "node-2": 1},
			clock2:   map[string]uint64{"node-1": 1, "node-2": 3},
			expected: map[string]uint64{"node-1": 2, "node-2": 3},
		},
		{
			name:     "merge with new nodes",
			clock1:   map[string]uint64{"node-1": 2},
			clock2:   map[string]uint64{"node-2": 3},
			expected: map[string]uint64{"node-1": 2, "node-2": 3},
		},
		{
			name:     "merge complex case",
			clock1:   map[string]uint64{"node-1": 5, "node-2": 2, "node-3": 1},
			clock2:   map[string]uint64{"node-1": 3, "node-2": 4, "node-4": 2},
			expected: map[string]uint64{"node-1": 5, "node-2": 4, "node-3": 1, "node-4": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc1 := NewVectorClockFromMap(tt.clock1)
			vc2 := NewVectorClockFromMap(tt.clock2)

			err := vc1.Merge(vc2)
			if err != nil {
				t.Errorf("Unexpected error on merge: %v", err)
			}

			for nodeID, expectedValue := range tt.expected {
				if actualValue := vc1.GetClock(nodeID); actualValue != expectedValue {
					t.Errorf("Expected clock for %s to be %d, got %d", nodeID, expectedValue, actualValue)
				}
			}

			if vc1.Size() != len(tt.expected) {
				t.Errorf("Expected size %d, got %d", len(tt.expected), vc1.Size())
			}
		})
	}

	// Test merging with nil
	t.Run("merge with nil", func(t *testing.T) {
		vc := NewVectorClockFromMap(map[string]uint64{"node-1": 5})
		originalSize := vc.Size()

		vc.Merge(nil)

		if vc.Size() != originalSize {
			t.Errorf("Merging with nil should not change size, expected %d, got %d", originalSize, vc.Size())
		}

		if vc.GetClock("node-1") != 5 {
			t.Error("Merging with nil should not change values")
		}
	})
}

func TestVectorClock_Compare(t *testing.T) {
	tests := []struct {
		name     string
		clock1   map[string]uint64
		clock2   map[string]uint64
		expected int
		desc     string
	}{
		{
			name:     "identical clocks",
			clock1:   map[string]uint64{"node-1": 2, "node-2": 3},
			clock2:   map[string]uint64{"node-1": 2, "node-2": 3},
			expected: 0,
			desc:     "identical",
		},
		{
			name:     "both empty",
			clock1:   map[string]uint64{},
			clock2:   map[string]uint64{},
			expected: 0,
			desc:     "both empty",
		},
		{
			name:     "happened-before",
			clock1:   map[string]uint64{"node-1": 1, "node-2": 2},
			clock2:   map[string]uint64{"node-1": 2, "node-2": 3},
			expected: -1,
			desc:     "clock1 happened-before clock2",
		},
		{
			name:     "happened-after",
			clock1:   map[string]uint64{"node-1": 2, "node-2": 3},
			clock2:   map[string]uint64{"node-1": 1, "node-2": 2},
			expected: 1,
			desc:     "clock1 happened-after clock2",
		},
		{
			name:     "concurrent - different nodes",
			clock1:   map[string]uint64{"node-1": 2},
			clock2:   map[string]uint64{"node-2": 2},
			expected: 0,
			desc:     "concurrent (different nodes)",
		},
		{
			name:     "concurrent - mixed values",
			clock1:   map[string]uint64{"node-1": 3, "node-2": 1},
			clock2:   map[string]uint64{"node-1": 1, "node-2": 3},
			expected: 0,
			desc:     "concurrent (mixed values)",
		},
		{
			name:     "subset relationship",
			clock1:   map[string]uint64{"node-1": 2},
			clock2:   map[string]uint64{"node-1": 2, "node-2": 1},
			expected: -1,
			desc:     "clock1 is subset (happened-before)",
		},
		{
			name:     "superset relationship",
			clock1:   map[string]uint64{"node-1": 2, "node-2": 1},
			clock2:   map[string]uint64{"node-1": 2},
			expected: 1,
			desc:     "clock1 is superset (happened-after)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc1 := NewVectorClockFromMap(tt.clock1)
			vc2 := NewVectorClockFromMap(tt.clock2)

			result := vc1.Compare(vc2)
			if result != tt.expected {
				t.Errorf("Expected %s (%d), got %d", tt.desc, tt.expected, result)
			}
		})
	}
}

func TestVectorClock_String(t *testing.T) {
	tests := []struct {
		name     string
		clocks   map[string]uint64
		expected string
	}{
		{
			name:     "empty clock",
			clocks:   map[string]uint64{},
			expected: "{}",
		},
		{
			name:     "single node",
			clocks:   map[string]uint64{"node-1": 5},
			expected: `{"node-1":5}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := NewVectorClockFromMap(tt.clocks)
			result := vc.String()

			if tt.name == "empty clock" {
				if result != tt.expected {
					t.Errorf("Expected '%s', got '%s'", tt.expected, result)
				}
			} else {
				// For non-empty clocks, just verify it's valid JSON and can be parsed back
				parsed, err := NewVectorClockFromString(result)
				if err != nil {
					t.Errorf("String() produced invalid JSON: %v", err)
				}

				if !vc.IsEqual(parsed) {
					t.Error("String() serialization is not reversible")
				}
			}
		})
	}
}

func TestVectorClock_HelperMethods(t *testing.T) {
	t.Run("Clone", func(t *testing.T) {
		original := NewVectorClockFromMap(map[string]uint64{"node-1": 5, "node-2": 3})
		clone := original.Clone()

		if !original.IsEqual(clone) {
			t.Error("Clone should be equal to original")
		}

		// Modify clone and ensure original is unaffected
		clone.Increment("node-1")
		if original.GetClock("node-1") != 5 {
			t.Error("Modifying clone affected original")
		}

		if clone.GetClock("node-1") != 6 {
			t.Error("Clone was not properly independent")
		}
	})

	t.Run("IsEqual", func(t *testing.T) {
		vc1 := NewVectorClockFromMap(map[string]uint64{"node-1": 5})
		vc2 := NewVectorClockFromMap(map[string]uint64{"node-1": 5})
		vc3 := NewVectorClockFromMap(map[string]uint64{"node-1": 6})

		if !vc1.IsEqual(vc2) {
			t.Error("Equal clocks should return true")
		}

		if vc1.IsEqual(vc3) {
			t.Error("Different clocks should return false")
		}

		// Test with nil
		if !NewVectorClock().IsEqual(nil) {
			t.Error("Empty clock should equal nil")
		}
	})

	t.Run("HappenedBefore and HappenedAfter", func(t *testing.T) {
		vc1 := NewVectorClockFromMap(map[string]uint64{"node-1": 1})
		vc2 := NewVectorClockFromMap(map[string]uint64{"node-1": 2})

		if !vc1.HappenedBefore(vc2) {
			t.Error("vc1 should have happened before vc2")
		}

		if !vc2.HappenedAfter(vc1) {
			t.Error("vc2 should have happened after vc1")
		}

		if vc1.HappenedAfter(vc2) {
			t.Error("vc1 should not have happened after vc2")
		}
	})

	t.Run("IsConcurrentWith", func(t *testing.T) {
		vc1 := NewVectorClockFromMap(map[string]uint64{"node-1": 2})
		vc2 := NewVectorClockFromMap(map[string]uint64{"node-2": 2})
		vc3 := NewVectorClockFromMap(map[string]uint64{"node-1": 2})

		if !vc1.IsConcurrentWith(vc2) {
			t.Error("vc1 and vc2 should be concurrent")
		}

		if vc1.IsConcurrentWith(vc3) {
			t.Error("equal clocks should not be considered concurrent")
		}
	})
}

func TestVectorClock_CompareWithDifferentVersionType(t *testing.T) {
	vc := NewVectorClock()

	// Test comparison with different version type (nil in this case represents a different type)
	result := vc.Compare(nil)
	if result != 0 {
		t.Errorf("Comparison with different version type should return 0, got %d", result)
	}
}

func TestVectorClock_RealWorldScenario(t *testing.T) {
	// Simulate a real-world distributed scenario
	t.Run("distributed scenario", func(t *testing.T) {
		nodeA := "node-A"
		nodeB := "node-B"
		nodeC := "node-C"

		// All nodes start with empty clocks
		vcA := NewVectorClock()
		vcB := NewVectorClock()
		vcC := NewVectorClock()

		// Node A creates first event
		vcA.Increment(nodeA)
		if vcA.String() != `{"node-A":1}` {
			t.Errorf("Expected node A to have clock {\"node-A\":1}, got %s", vcA.String())
		}

		// Node B creates event independently
		err := vcB.Increment(nodeB)
		if err != nil {
			t.Errorf("Unexpected error on increment: %v", err)
		}

		// A and B should be concurrent
		if !vcA.IsConcurrentWith(vcB) {
			t.Error("Node A and B should be concurrent")
		}

		// Node B receives A's event and merges
		err = vcB.Merge(vcA)
		if err != nil {
			t.Errorf("Unexpected error on merge: %v", err)
		}
		err = vcB.Increment(nodeB) // B creates another event
		if err != nil {
			t.Errorf("Unexpected error on increment: %v", err)
		}

		// Now B should have happened after A's original state
		if !vcB.HappenedAfter(vcA) {
			t.Error("Node B should have happened after A")
		}

		// Node C joins and creates an event
		err = vcC.Increment(nodeC)
		if err != nil {
			t.Errorf("Unexpected error on increment: %v", err)
		}

		// C should be concurrent with both A and B
		if !vcC.IsConcurrentWith(vcA) || !vcC.IsConcurrentWith(vcB) {
			t.Error("Node C should be concurrent with both A and B")
		}

		// Simulate a sync: C receives updates from A and B
		err = vcC.Merge(vcA)
		if err != nil {
			t.Errorf("Unexpected error on merge: %v", err)
		}
		err = vcC.Merge(vcB)
		if err != nil {
			t.Errorf("Unexpected error on merge: %v", err)
		}
		err = vcC.Increment(nodeC) // C creates another event after sync
		if err != nil {
			t.Errorf("Unexpected error on increment: %v", err)
		}

		// Now C should have the most recent state
		if !vcC.HappenedAfter(vcA) || !vcC.HappenedAfter(vcB) {
			t.Error("Node C should have happened after both A and B")
		}
	})
}

// Benchmark tests
func BenchmarkVectorClock_Increment(b *testing.B) {
	vc := NewVectorClock()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		vc.Increment("node-1")
	}
}

func BenchmarkVectorClock_Merge(b *testing.B) {
	vc1 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 100, "node-2": 200, "node-3": 300,
	})
	vc2 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 150, "node-4": 400, "node-5": 500,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone1 := vc1.Clone()
		clone1.Merge(vc2)
	}
}

func BenchmarkVectorClock_Compare(b *testing.B) {
	vc1 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 100, "node-2": 200, "node-3": 300,
	})
	vc2 := NewVectorClockFromMap(map[string]uint64{
		"node-1": 150, "node-2": 180, "node-3": 350,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vc1.Compare(vc2)
	}
}

func BenchmarkVectorClock_String(b *testing.B) {
	vc := NewVectorClockFromMap(map[string]uint64{
		"node-1": 100, "node-2": 200, "node-3": 300,
		"node-4": 400, "node-5": 500,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vc.String()
	}
}
