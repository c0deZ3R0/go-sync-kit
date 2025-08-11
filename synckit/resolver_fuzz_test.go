package synckit

import (
	"context"
	"testing"
	"unicode/utf8"
)

// FuzzDynamicResolver_Resolve ensures that DynamicResolver.Resolve is deterministic
// and panic-free for any input conflict data.
func FuzzDynamicResolver_Resolve(f *testing.F) {
	// Seed with some initial test cases
	f.Add("UserUpdated", "user-123", "field1,field2", "key1", "value1", int64(1), int64(2))
	f.Add("OrderCreated", "", "", "", "", int64(0), int64(1))
	f.Add("", "agg-456", "status", "role", "admin", int64(2), int64(1))
	f.Add("ProductDeleted", "product-789", "", "source", "", int64(1), int64(1))

	f.Fuzz(func(t *testing.T, eventType, aggregateID, fieldsStr, metaKey, metaValue string, localVersion, remoteVersion int64) {
		// Skip invalid UTF-8 strings to focus on logical fuzz testing
		if !utf8.ValidString(eventType) || !utf8.ValidString(aggregateID) || 
		   !utf8.ValidString(fieldsStr) || !utf8.ValidString(metaKey) || !utf8.ValidString(metaValue) {
			return
		}

		// Parse changed fields from comma-separated string
		var changedFields []string
		if fieldsStr != "" {
			changedFields = []string{fieldsStr} // Simplified for fuzzing
		}

		// Build metadata map
		metadata := map[string]any{}
		if metaKey != "" {
			metadata[metaKey] = metaValue
		}

		// Create test versions
		var localVer, remoteVer Version
		if localVersion > 0 {
			localVer = testVersion{v: int(localVersion)}
		}
		if remoteVersion > 0 {
			remoteVer = testVersion{v: int(remoteVersion)}
		}

		// Construct conflict
		conflict := Conflict{
			EventType:     eventType,
			AggregateID:   aggregateID,
			ChangedFields: changedFields,
			Metadata:      metadata,
			Local:         EventWithVersion{Event: testEvent{id: "local", typ: eventType, agg: aggregateID}, Version: localVer},
			Remote:        EventWithVersion{Event: testEvent{id: "remote", typ: eventType, agg: aggregateID}, Version: remoteVer},
		}

		// Test multiple resolver configurations
		resolvers := []struct {
			name     string
			resolver *DynamicResolver
		}{
			{
				name: "lww-only",
				resolver: mustNewResolver(t, WithFallback(&LastWriteWinsResolver{})),
			},
			{
				name: "additive-only",
				resolver: mustNewResolver(t, WithFallback(&AdditiveMergeResolver{})),
			},
			{
				name: "manual-only", 
				resolver: mustNewResolver(t, WithFallback(&ManualReviewResolver{Reason: "fuzz"})),
			},
			{
				name: "event-type-rule",
				resolver: mustNewResolver(t, 
					WithEventTypeRule("match", eventType, &LastWriteWinsResolver{}),
					WithFallback(&AdditiveMergeResolver{}),
				),
			},
			{
				name: "metadata-rule",
				resolver: mustNewResolver(t,
					WithRule("meta", MetadataEq(metaKey, metaValue), &LastWriteWinsResolver{}),
					WithFallback(&ManualReviewResolver{}),
				),
			},
		}

		for _, tc := range resolvers {
			t.Run(tc.name, func(t *testing.T) {
				// The main fuzz test: ensure no panics and deterministic behavior
				result1, err1 := tc.resolver.Resolve(context.Background(), conflict)
				result2, err2 := tc.resolver.Resolve(context.Background(), conflict)

				// Check for panics (would stop execution above)
				// Both calls should return the same result (deterministic)
				if (err1 == nil) != (err2 == nil) {
					t.Fatalf("Non-deterministic error behavior: err1=%v, err2=%v", err1, err2)
				}
				if err1 != nil && err2 != nil && err1.Error() != err2.Error() {
					t.Fatalf("Different error messages: %v vs %v", err1, err2)
				}
				if err1 == nil && err2 == nil {
					// Results should be identical
					if result1.Decision != result2.Decision {
						t.Fatalf("Non-deterministic decision: %s vs %s", result1.Decision, result2.Decision)
					}
					if len(result1.ResolvedEvents) != len(result2.ResolvedEvents) {
						t.Fatalf("Non-deterministic event count: %d vs %d", len(result1.ResolvedEvents), len(result2.ResolvedEvents))
					}
					if len(result1.Reasons) != len(result2.Reasons) {
						t.Fatalf("Non-deterministic reason count: %d vs %d", len(result1.Reasons), len(result2.Reasons))
					}
				}
			})
		}
	})
}

// FuzzSpec_Combinators tests that Spec combinators (And, Or, Not) behave correctly
// for any input conflict without panicking.
func FuzzSpec_Combinators(f *testing.F) {
	f.Add("TestEvent", "agg-123", "field1", "key", "value")
	f.Add("", "", "", "", "")
	f.Add("LongEventTypeName", "very-long-aggregate-id-with-dashes", "field1,field2,field3", "metadata-key", "metadata-value")

	f.Fuzz(func(t *testing.T, eventType, aggregateID, fieldsStr, metaKey, metaValue string) {
		// Skip invalid UTF-8
		if !utf8.ValidString(eventType) || !utf8.ValidString(aggregateID) || 
		   !utf8.ValidString(fieldsStr) || !utf8.ValidString(metaKey) || !utf8.ValidString(metaValue) {
			return
		}

		var changedFields []string
		if fieldsStr != "" {
			changedFields = []string{fieldsStr}
		}

		metadata := map[string]any{}
		if metaKey != "" {
			metadata[metaKey] = metaValue
		}

		conflict := Conflict{
			EventType:     eventType,
			AggregateID:   aggregateID,
			ChangedFields: changedFields,
			Metadata:      metadata,
		}

		// Test basic specs
		eventSpec := EventTypeIs(eventType)
		fieldSpec := AnyFieldIn(fieldsStr)
		metaSpec := MetadataEq(metaKey, metaValue)

		// Test combinators don't panic and are deterministic
		andSpec := And(eventSpec, fieldSpec)
		orSpec := Or(eventSpec, metaSpec)
		notSpec := Not(eventSpec)
		complexSpec := And(Or(eventSpec, fieldSpec), Not(metaSpec))

		specs := []struct {
			name string
			spec Spec
		}{
			{"event", eventSpec},
			{"field", fieldSpec}, 
			{"meta", metaSpec},
			{"and", andSpec},
			{"or", orSpec},
			{"not", notSpec},
			{"complex", complexSpec},
		}

		for _, tc := range specs {
			t.Run(tc.name, func(t *testing.T) {
				// Should not panic and be deterministic
				result1 := tc.spec(conflict)
				result2 := tc.spec(conflict)
				
				if result1 != result2 {
					t.Fatalf("Non-deterministic spec result for %s: %v vs %v", tc.name, result1, result2)
				}
			})
		}
	})
}

// FuzzConflictResolvers tests individual conflict resolvers for panic-free operation.
func FuzzConflictResolvers(f *testing.F) {
	f.Add(int64(1), int64(2), "test-event", "agg-1")
	f.Add(int64(0), int64(1), "", "")
	f.Add(int64(2), int64(1), "UserEvent", "user-123")
	f.Add(int64(1), int64(1), "OrderEvent", "order-456")

	f.Fuzz(func(t *testing.T, localVer, remoteVer int64, eventType, aggregateID string) {
		if !utf8.ValidString(eventType) || !utf8.ValidString(aggregateID) {
			return
		}

		// Build versions
		var localVersion, remoteVersion Version
		if localVer > 0 {
			localVersion = testVersion{v: int(localVer)}
		}
		if remoteVer > 0 {
			remoteVersion = testVersion{v: int(remoteVer)}
		}

		conflict := Conflict{
			EventType:   eventType,
			AggregateID: aggregateID,
			Local:       EventWithVersion{Event: testEvent{id: "local", typ: eventType, agg: aggregateID}, Version: localVersion},
			Remote:      EventWithVersion{Event: testEvent{id: "remote", typ: eventType, agg: aggregateID}, Version: remoteVersion},
		}

		resolvers := []ConflictResolver{
			&LastWriteWinsResolver{},
			&AdditiveMergeResolver{},
			&ManualReviewResolver{Reason: "fuzz-test"},
		}

		for i, resolver := range resolvers {
			t.Run(getResolverName(i), func(t *testing.T) {
				// Should not panic
				result1, err1 := resolver.Resolve(context.Background(), conflict)
				result2, err2 := resolver.Resolve(context.Background(), conflict)

				// Should be deterministic
				if (err1 == nil) != (err2 == nil) {
					t.Fatalf("Non-deterministic error behavior")
				}
				if err1 == nil && err2 == nil {
					if result1.Decision != result2.Decision {
						t.Fatalf("Non-deterministic decision")
					}
					if len(result1.ResolvedEvents) != len(result2.ResolvedEvents) {
						t.Fatalf("Non-deterministic event resolution")
					}
				}
			})
		}
	})
}

// Helper functions for fuzz tests

func mustNewResolver(t *testing.T, opts ...Option) *DynamicResolver {
	resolver, err := NewDynamicResolver(opts...)
	if err != nil {
		t.Fatalf("Failed to create resolver: %v", err)
	}
	return resolver
}

func getResolverName(index int) string {
	names := []string{"lww", "additive", "manual"}
	if index < len(names) {
		return names[index]
	}
	return "unknown"
}

// testVersion is a simple Version implementation for fuzz testing
type testVersion struct{ v int }

func (t testVersion) Compare(other Version) int {
	if other == nil {
		return 1
	}
	o, ok := other.(testVersion)
	if !ok {
		return 0
	}
	if t.v < o.v {
		return -1
	}
	if t.v > o.v {
		return 1
	}
	return 0
}
func (t testVersion) String() string { return "" }
func (t testVersion) IsZero() bool  { return t.v == 0 }

// testEvent is a simple Event implementation for fuzz testing
type testEvent struct {
	id   string
	typ  string
	agg  string
	data any
	meta map[string]any
}

func (e testEvent) ID() string               { return e.id }
func (e testEvent) Type() string             { return e.typ }
func (e testEvent) AggregateID() string      { return e.agg }
func (e testEvent) Data() any                { return e.data }
func (e testEvent) Metadata() map[string]any { return e.meta }
