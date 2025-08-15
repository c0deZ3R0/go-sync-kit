package synckit

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestResolvers_EnhancedEdgeCases provides comprehensive testing of resolvers
// with edge cases, boundary conditions, and error scenarios.
func TestResolvers_EnhancedEdgeCases(t *testing.T) {
	t.Run("LastWriteWinsResolver", func(t *testing.T) {
		t.Run("EdgeCases", testLastWriteWinsEdgeCases)
		t.Run("BoundaryConditions", testLastWriteWinsBoundaryConditions)
		t.Run("ErrorConditions", testLastWriteWinsErrorConditions)
	})
	
	t.Run("AdditiveMergeResolver", func(t *testing.T) {
		t.Run("EdgeCases", testAdditiveMergeEdgeCases)
		t.Run("BoundaryConditions", testAdditiveMergeBoundaryConditions)
	})
	
	t.Run("ManualReviewResolver", func(t *testing.T) {
		t.Run("EdgeCases", testManualReviewEdgeCases)
		t.Run("ReasonHandling", testManualReviewReasonHandling)
	})
}

func testLastWriteWinsEdgeCases(t *testing.T) {
	resolver := &LastWriteWinsResolver{}
	ctx := context.Background()

	tests := []struct {
		name      string
		conflict  Conflict
		wantDecision string
		wantEventCount int
		wantError bool
	}{
		{
			name: "identical_versions_prefer_remote",
			conflict: Conflict{
			Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 5}},
			Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 5}},
			},
			wantDecision: "keep_remote",
			wantEventCount: 1,
		},
		{
			name: "zero_versions_both",
			conflict: Conflict{
			Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 0}},
			Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 0}},
			},
			wantDecision: "keep_remote",
			wantEventCount: 1,
		},
		{
			name: "very_large_version_numbers",
			conflict: Conflict{
			Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 999999999}},
			Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 1000000000}},
			},
			wantDecision: "keep_remote",
			wantEventCount: 1,
		},
		{
			name: "negative_version_numbers",
			conflict: Conflict{
				Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: -1}},
				Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 1}},
			},
			wantDecision: "keep_remote",
			wantEventCount: 1,
		},
		{
			name: "empty_events_with_versions",
			conflict: Conflict{
				Local:  EventWithVersion{Event: &testEvent{id: ""}, Version: &testVersion{v: 1}},
				Remote: EventWithVersion{Event: &testEvent{id: ""}, Version: &testVersion{v: 2}},
			},
			wantDecision: "keep_remote",
			wantEventCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(ctx, tt.conflict)
			
			if tt.wantError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result.Decision != tt.wantDecision {
				t.Errorf("Decision: got %s, want %s", result.Decision, tt.wantDecision)
			}
			if len(result.ResolvedEvents) != tt.wantEventCount {
				t.Errorf("Event count: got %d, want %d", len(result.ResolvedEvents), tt.wantEventCount)
			}
		})
	}
}

func testLastWriteWinsBoundaryConditions(t *testing.T) {
	resolver := &LastWriteWinsResolver{}
	ctx := context.Background()

	// Test context cancellation
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	conflict := Conflict{
		Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 1}},
		Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 2}},
	}
	
	// Should still work even with cancelled context (no async operations)
	result, err := resolver.Resolve(cancelCtx, conflict)
	if err != nil {
		t.Errorf("Unexpected error with cancelled context: %v", err)
	}
	if result.Decision != "keep_remote" {
		t.Errorf("Expected remote to win, got %s", result.Decision)
	}

	// Test with timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer timeoutCancel()
	time.Sleep(2 * time.Millisecond) // Ensure timeout
	
	result, err = resolver.Resolve(timeoutCtx, conflict)
	if err != nil {
		t.Errorf("Unexpected error with timeout context: %v", err)
	}
}

func testLastWriteWinsErrorConditions(t *testing.T) {
	resolver := &LastWriteWinsResolver{}
	
	// Test with invalid version comparison
	invalidVersionConflict := Conflict{
		Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &invalidVersion{}},
		Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 2}},
	}
	
	result, err := resolver.Resolve(context.Background(), invalidVersionConflict)
	// Should handle gracefully - invalid version should return 0 from Compare
	if err != nil {
		t.Errorf("Should handle invalid version gracefully, got error: %v", err)
	}
	// With invalid comparison returning 0, should default to keep_remote
	if result.Decision != "keep_remote" {
		t.Errorf("Expected keep_remote for invalid version comparison, got %s", result.Decision)
	}
}

func testAdditiveMergeEdgeCases(t *testing.T) {
	resolver := &AdditiveMergeResolver{}
	ctx := context.Background()

	tests := []struct {
		name           string
		conflict       Conflict
		wantEventCount int
		checkOrder     bool
	}{
		{
			name: "both_empty_events",
			conflict: Conflict{
				Local:  EventWithVersion{},
				Remote: EventWithVersion{},
			},
			wantEventCount: 0,
		},
		{
			name: "local_empty_event_with_version",
			conflict: Conflict{
				Local:  EventWithVersion{Version: &testVersion{v: 1}},
				Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 2}},
			},
			wantEventCount: 2, // AdditiveMerge includes both even if one is nil
		},
		{
			name: "identical_events",
			conflict: Conflict{
				Local:  EventWithVersion{Event: &testEvent{id: "same", data: "data"}, Version: &testVersion{v: 1}},
				Remote: EventWithVersion{Event: &testEvent{id: "same", data: "data"}, Version: &testVersion{v: 1}},
			},
			wantEventCount: 2, // Both included even if identical
		},
		{
			name: "large_event_data",
			conflict: Conflict{
				Local: EventWithVersion{
					Event: &testEvent{
						id:   "large-local",
						data: strings.Repeat("x", 10000),
					},
					Version: &testVersion{v: 1},
				},
				Remote: EventWithVersion{
					Event: &testEvent{
						id:   "large-remote", 
						data: strings.Repeat("y", 10000),
					},
					Version: &testVersion{v: 2},
				},
			},
			wantEventCount: 2,
			checkOrder:     true,
		},
		{
			name: "nil_event_data",
			conflict: Conflict{
				Local:  EventWithVersion{Event: &testEvent{id: "local", data: nil}, Version: &testVersion{v: 1}},
				Remote: EventWithVersion{Event: &testEvent{id: "remote", data: nil}, Version: &testVersion{v: 2}},
			},
			wantEventCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.Resolve(ctx, tt.conflict)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if len(result.ResolvedEvents) != tt.wantEventCount {
				t.Errorf("Event count: got %d, want %d", len(result.ResolvedEvents), tt.wantEventCount)
			}
			
			if result.Decision != "merge" && tt.wantEventCount > 0 {
				t.Errorf("Expected merge decision for non-empty result, got %s", result.Decision)
			}
			
			// Verify consistent ordering (local first, then remote)
			if tt.checkOrder && len(result.ResolvedEvents) == 2 {
				if result.ResolvedEvents[0].Event.ID() != "large-local" {
					t.Errorf("Expected local event first, got %s", result.ResolvedEvents[0].Event.ID())
				}
				if result.ResolvedEvents[1].Event.ID() != "large-remote" {
					t.Errorf("Expected remote event second, got %s", result.ResolvedEvents[1].Event.ID())
				}
			}
		})
	}
}

func testAdditiveMergeBoundaryConditions(t *testing.T) {
	resolver := &AdditiveMergeResolver{}
	
	// Test with many concurrent resolves (checking for race conditions)
	const numResolves = 100
	results := make(chan ResolvedConflict, numResolves)
	
	conflict := Conflict{
		Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 1}},
		Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 2}},
	}
	
	for i := 0; i < numResolves; i++ {
		go func() {
			result, err := resolver.Resolve(context.Background(), conflict)
			if err != nil {
				t.Errorf("Concurrent resolve failed: %v", err)
				return
			}
			results <- result
		}()
	}
	
	// Collect all results and verify consistency
	for i := 0; i < numResolves; i++ {
		result := <-results
		if len(result.ResolvedEvents) != 2 {
			t.Errorf("Concurrent resolve %d: expected 2 events, got %d", i, len(result.ResolvedEvents))
		}
		if result.Decision != "merge" {
			t.Errorf("Concurrent resolve %d: expected merge, got %s", i, result.Decision)
		}
	}
}

func testManualReviewEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		reason       string
		wantReasons  []string
		checkReason  string
	}{
		{
			name:        "empty_reason",
			reason:      "",
			wantReasons: []string{"manual review required"},
		},
		{
			name:        "normal_reason",
			reason:      "policy violation",
			wantReasons: []string{"manual review required", "policy violation"},
			checkReason: "policy violation",
		},
		{
			name:        "very_long_reason",
			reason:      strings.Repeat("very long reason ", 100),
			wantReasons: []string{"manual review required", strings.Repeat("very long reason ", 100)},
			checkReason: strings.Repeat("very long reason ", 100),
		},
		{
			name:        "special_characters",
			reason:      "unicode: 🚨 alert 中文 test",
			wantReasons: []string{"manual review required", "unicode: 🚨 alert 中文 test"},
			checkReason: "unicode: 🚨 alert 中文 test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &ManualReviewResolver{Reason: tt.reason}
			
			conflict := Conflict{
				Local:  EventWithVersion{Event: &testEvent{id: "local"}},
				Remote: EventWithVersion{Event: &testEvent{id: "remote"}},
			}
			
			result, err := resolver.Resolve(context.Background(), conflict)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result.Decision != "manual_review" {
				t.Errorf("Expected manual_review, got %s", result.Decision)
			}
			
			if len(result.ResolvedEvents) != 0 {
				t.Errorf("Expected no resolved events, got %d", len(result.ResolvedEvents))
			}
			
			if len(result.Reasons) != len(tt.wantReasons) {
				t.Errorf("Reason count: got %d, want %d", len(result.Reasons), len(tt.wantReasons))
			}
			
			// Check specific reason if provided
			if tt.checkReason != "" {
				found := false
				for _, reason := range result.Reasons {
					if reason == tt.checkReason {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected reason %q not found in %v", tt.checkReason, result.Reasons)
				}
			}
		})
	}
}

func testManualReviewReasonHandling(t *testing.T) {
	resolver := &ManualReviewResolver{Reason: "test reason"}
	
	// Test multiple sequential resolves maintain consistent reasons
	conflict := Conflict{
		Local:  EventWithVersion{Event: &testEvent{id: "local"}},
		Remote: EventWithVersion{Event: &testEvent{id: "remote"}},
	}
	
	const numResolves = 10
	for i := 0; i < numResolves; i++ {
		result, err := resolver.Resolve(context.Background(), conflict)
		if err != nil {
			t.Errorf("Resolve %d failed: %v", i, err)
			continue
		}
		
		expectedReasons := []string{"manual review required", "test reason"}
		if len(result.Reasons) != len(expectedReasons) {
			t.Errorf("Resolve %d: got %d reasons, want %d", i, len(result.Reasons), len(expectedReasons))
		}
		
		for j, expected := range expectedReasons {
			if j >= len(result.Reasons) || result.Reasons[j] != expected {
				t.Errorf("Resolve %d: reason %d got %q, want %q", i, j, 
					safeGetReason(result.Reasons, j), expected)
			}
		}
	}
}

// TestSpec_EnhancedEdgeCases provides comprehensive testing of spec predicates
// with edge cases and boundary conditions.
func TestSpec_EnhancedEdgeCases(t *testing.T) {
	t.Run("EventTypeIs", testEventTypeIsEdgeCases)
	t.Run("AnyFieldIn", testAnyFieldInEdgeCases)
	t.Run("MetadataEq", testMetadataEqEdgeCases)
	t.Run("Combinators", testCombinatorsEdgeCases)
}

func testEventTypeIsEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		matchType   string
		conflict    Conflict
		wantMatch   bool
	}{
		{
			name:      "empty_match_type",
			matchType: "",
			conflict:  Conflict{EventType: ""},
			wantMatch: true,
		},
		{
			name:      "empty_conflict_type",
			matchType: "SomeType",
			conflict:  Conflict{EventType: ""},
			wantMatch: false,
		},
		{
			name:      "case_sensitive",
			matchType: "UserUpdated",
			conflict:  Conflict{EventType: "userupdated"},
			wantMatch: false,
		},
		{
			name:      "unicode_types",
			matchType: "用户更新",
			conflict:  Conflict{EventType: "用户更新"},
			wantMatch: true,
		},
		{
			name:      "very_long_type",
			matchType: strings.Repeat("VeryLongEventTypeName", 100),
			conflict:  Conflict{EventType: strings.Repeat("VeryLongEventTypeName", 100)},
			wantMatch: true,
		},
		{
			name:      "special_characters",
			matchType: "Event-Type_With.Special@Characters#",
			conflict:  Conflict{EventType: "Event-Type_With.Special@Characters#"},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := EventTypeIs(tt.matchType)
			result := spec(tt.conflict)
			if result != tt.wantMatch {
				t.Errorf("EventTypeIs(%q) on %q: got %v, want %v", 
					tt.matchType, tt.conflict.EventType, result, tt.wantMatch)
			}
		})
	}
}

func testAnyFieldInEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		fields      []string
		conflict    Conflict
		wantMatch   bool
	}{
		{
			name:      "no_fields_to_match",
			fields:    []string{},
			conflict:  Conflict{ChangedFields: []string{"field1", "field2"}},
			wantMatch: false,
		},
		{
			name:      "no_changed_fields",
			fields:    []string{"field1", "field2"},
			conflict:  Conflict{ChangedFields: []string{}},
			wantMatch: false,
		},
		{
			name:      "nil_changed_fields",
			fields:    []string{"field1"},
			conflict:  Conflict{ChangedFields: nil},
			wantMatch: false,
		},
		{
			name:      "empty_field_names",
			fields:    []string{"", "field2"},
			conflict:  Conflict{ChangedFields: []string{"", "field1"}},
			wantMatch: true,
		},
		{
			name:      "duplicate_fields_in_spec",
			fields:    []string{"field1", "field1", "field2"},
			conflict:  Conflict{ChangedFields: []string{"field1"}},
			wantMatch: true,
		},
		{
			name:      "duplicate_fields_in_conflict",
			fields:    []string{"field1"},
			conflict:  Conflict{ChangedFields: []string{"field1", "field1", "field2"}},
			wantMatch: true,
		},
		{
			name:      "unicode_fields",
			fields:    []string{"字段名", "フィールド"},
			conflict:  Conflict{ChangedFields: []string{"字段名", "other"}},
			wantMatch: true,
		},
		{
			name: "large_field_list",
			fields:    generateFieldList(1000, "match_"),
			conflict:  Conflict{ChangedFields: []string{"match_0", "other"}}, // match_0 exists in generated list
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := AnyFieldIn(tt.fields...)
			result := spec(tt.conflict)
			if result != tt.wantMatch {
				t.Errorf("AnyFieldIn(%v) on %v: got %v, want %v", 
					tt.fields, tt.conflict.ChangedFields, result, tt.wantMatch)
			}
		})
	}
}

func testMetadataEqEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     interface{}
		conflict  Conflict
		wantMatch bool
	}{
		{
			name:      "nil_metadata",
			key:       "key",
			value:     "value",
			conflict:  Conflict{Metadata: nil},
			wantMatch: false,
		},
		{
			name:      "empty_key",
			key:       "",
			value:     "value",
			conflict:  Conflict{Metadata: map[string]interface{}{"": "value"}},
			wantMatch: true,
		},
		{
			name:      "nil_value_match",
			key:       "key",
			value:     nil,
			conflict:  Conflict{Metadata: map[string]interface{}{"key": nil}},
			wantMatch: true,
		},
		{
			name:      "different_types_same_value",
			key:       "number",
			value:     42,
			conflict:  Conflict{Metadata: map[string]interface{}{"number": int64(42)}},
			wantMatch: false, // Different types don't match
		},
		{
			name:      "string_comparison",
			key:       "text",
			value:     "hello",
			conflict:  Conflict{Metadata: map[string]interface{}{"text": "hello"}},
			wantMatch: true,
		},
		{
			name:      "bool_comparison",
			key:       "flag",
			value:     true,
			conflict:  Conflict{Metadata: map[string]interface{}{"flag": true}},
			wantMatch: true,
		},
		{
			name:      "int64_comparison",
			key:       "count",
			value:     int64(100),
			conflict:  Conflict{Metadata: map[string]interface{}{"count": int64(100)}},
			wantMatch: true,
		},
		{
			name:      "uint64_comparison",
			key:       "size",
			value:     uint64(1024),
			conflict:  Conflict{Metadata: map[string]interface{}{"size": uint64(1024)}},
			wantMatch: true,
		},
		{
			name:      "custom_type_fallback",
			key:       "custom",
			value:     customType{value: "test"},
			conflict:  Conflict{Metadata: map[string]interface{}{"custom": customType{value: "test"}}},
			wantMatch: true, // Falls back to interface{} comparison
		},
		{
			name:      "unicode_key_value",
			key:       "键名",
			value:     "值",
			conflict:  Conflict{Metadata: map[string]interface{}{"键名": "值"}},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := MetadataEq(tt.key, tt.value)
			result := spec(tt.conflict)
			if result != tt.wantMatch {
				t.Errorf("MetadataEq(%q, %v) on %v: got %v, want %v",
					tt.key, tt.value, tt.conflict.Metadata, result, tt.wantMatch)
			}
		})
	}
}

func testCombinatorsEdgeCases(t *testing.T) {
	// Create base specs for testing
	alwaysTrue := func(Conflict) bool { return true }
	alwaysFalse := func(Conflict) bool { return false }
	
	conflict := Conflict{EventType: "TestEvent"}

	t.Run("nil_specs", func(t *testing.T) {
		// And with nil specs
		andNilNil := And(nil, nil)
		if andNilNil(conflict) != false {
			t.Errorf("And(nil, nil) should return false")
		}
		
		andTrueNil := And(alwaysTrue, nil)
		if andTrueNil(conflict) != false {
			t.Errorf("And(true, nil) should return false")
		}
		
		// Or with nil specs
		orNilNil := Or(nil, nil)
		if orNilNil(conflict) != false {
			t.Errorf("Or(nil, nil) should return false")
		}
		
		orTrueNil := Or(alwaysTrue, nil)
		if orTrueNil(conflict) != true {
			t.Errorf("Or(true, nil) should return true")
		}
		
		// Not with nil spec
		notNil := Not(nil)
		if notNil(conflict) != true {
			t.Errorf("Not(nil) should return true")
		}
	})
	
	t.Run("complex_nesting", func(t *testing.T) {
		// Create deeply nested combinator
		eventTypeSpec := EventTypeIs("TestEvent")
		notEventType := Not(eventTypeSpec)
		orSpec := Or(eventTypeSpec, notEventType)
		andSpec := And(orSpec, alwaysTrue)
		finalSpec := Not(And(andSpec, alwaysFalse))
		
		// This should always be true due to the logic
		if !finalSpec(conflict) {
			t.Errorf("Complex nested spec should return true")
		}
	})

	t.Run("short_circuit_behavior", func(t *testing.T) {
		panicSpec := func(Conflict) bool {
			panic("should not be called")
		}
		
		// And should short-circuit on false
		andShortCircuit := And(alwaysFalse, panicSpec)
		if andShortCircuit(conflict) != false {
			t.Errorf("And should short-circuit on false")
		}
		
		// Or should short-circuit on true
		orShortCircuit := Or(alwaysTrue, panicSpec)
		if orShortCircuit(conflict) != true {
			t.Errorf("Or should short-circuit on true")
		}
	})
}

// Helper functions and types for enhanced tests

func safeGetReason(reasons []string, index int) string {
	if index >= 0 && index < len(reasons) {
		return reasons[index]
	}
	return "<out of bounds>"
}

func generateFieldList(count int, prefix string) []string {
	fields := make([]string, count)
	for i := 0; i < count; i++ {
		fields[i] = prefix + string(rune('0'+i%10))
	}
	return fields
}

type customType struct {
	value string
}

// invalidVersion implements Version but with broken Compare method
type invalidVersion struct{}

func (v *invalidVersion) Compare(other Version) int {
	// Return invalid comparison result
	return 0
}

func (v *invalidVersion) String() string {
	return "invalid"
}

func (v *invalidVersion) IsZero() bool {
	return false
}

// Benchmark tests for performance validation
func BenchmarkLastWriteWinsResolver(b *testing.B) {
	resolver := &LastWriteWinsResolver{}
	conflict := Conflict{
		Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 1}},
		Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 2}},
	}
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.Resolve(ctx, conflict)
	}
}

func BenchmarkAdditiveMergeResolver(b *testing.B) {
	resolver := &AdditiveMergeResolver{}
	conflict := Conflict{
		Local:  EventWithVersion{Event: &testEvent{id: "local"}, Version: &testVersion{v: 1}},
		Remote: EventWithVersion{Event: &testEvent{id: "remote"}, Version: &testVersion{v: 2}},
	}
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.Resolve(ctx, conflict)
	}
}

func BenchmarkEventTypeIsSpec(b *testing.B) {
	spec := EventTypeIs("TestEvent")
	conflict := Conflict{EventType: "TestEvent"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = spec(conflict)
	}
}

func BenchmarkComplexSpec(b *testing.B) {
	eventTypeSpec := EventTypeIs("TestEvent")
	fieldSpec := AnyFieldIn("field1", "field2", "field3")
	metaSpec := MetadataEq("priority", "high")
	complexSpec := And(Or(eventTypeSpec, fieldSpec), Not(metaSpec))
	
	conflict := Conflict{
		EventType:     "TestEvent",
		ChangedFields: []string{"field1", "other"},
		Metadata:      map[string]interface{}{"priority": "low"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = complexSpec(conflict)
	}
}
