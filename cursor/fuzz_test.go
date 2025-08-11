package cursor

import (
	"encoding/json"
	"testing"
)

// FuzzUnmarshalWire fuzzes the cursor.UnmarshalWire function to test its robustness
// against malformed or malicious input, as mentioned in the code review
func FuzzUnmarshalWire(f *testing.F) {
	// Seed the fuzzer with some valid inputs
	f.Add([]byte(`{"kind":"integer","data":1}`))
	f.Add([]byte(`{"kind":"bytes","data":"dGVzdA=="}`))
	f.Add([]byte(`{"kind":"string","data":"test"}`))
	f.Add([]byte(`{"kind":"integer","data":0}`))
	f.Add([]byte(`{"kind":"integer","data":18446744073709551615}`)) // max uint64
	
	// Add some edge cases
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"kind":"","data":null}`))
	f.Add([]byte(`{"kind":"unknown","data":"test"}`))
	f.Add([]byte(`{"kind":"integer","data":-1}`))
	f.Add([]byte(`{"kind":"integer","data":"not a number"}`))
	f.Add([]byte(`{"kind":"bytes","data":"invalid base64!!!"}`))
	
	f.Fuzz(func(t *testing.T, data []byte) {
		// Test that UnmarshalWire doesn't panic on arbitrary input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("UnmarshalWire panicked on input %q: %v", data, r)
			}
		}()
		
		// First check if it's valid JSON at all
		var raw json.RawMessage
		if json.Unmarshal(data, &raw) != nil {
			// If it's not valid JSON, UnmarshalWire should return an error
			var wireCursor WireCursor
			if json.Unmarshal(data, &wireCursor) == nil {
				t.Errorf("Expected JSON unmarshal error for invalid JSON: %q", data)
			}
			return
		}
		
		// Try to unmarshal into WireCursor
		var wireCursor WireCursor
		err := json.Unmarshal(data, &wireCursor)
		if err != nil {
			// If JSON unmarshaling fails, that's acceptable
			return
		}
		
		// Now test UnmarshalWire
		cursor, err := UnmarshalWire(&wireCursor)
		
		// The function should either return a valid cursor or an error
		// It should never panic
		if err == nil && cursor == nil {
			t.Errorf("UnmarshalWire returned nil cursor with no error for input: %q", data)
		}
		
		// If we got a cursor back, it should implement the Cursor interface properly
		if cursor != nil {
			// Test that Kind() doesn't panic
			_ = cursor.Kind()
			
			// If it's an IntegerCursor, test the Version interface methods
			if ic, ok := cursor.(IntegerCursor); ok {
				// Test that String() doesn't panic
				_ = ic.String()
				
				// Test that IsZero() doesn't panic
				_ = ic.IsZero()
				
				// Test that Compare doesn't panic with itself
				result := ic.Compare(ic)
				if result != 0 {
					t.Errorf("IntegerCursor should be equal to itself, got comparison result: %d", result)
				}
			}
			
			// If it's a VectorCursor, test the Version interface methods
			if vc, ok := cursor.(VectorCursor); ok {
				// Test that String() doesn't panic
				_ = vc.String()
				
				// Test that IsZero() doesn't panic
				_ = vc.IsZero()
				
				// Test that Compare doesn't panic with itself
				result := vc.Compare(vc)
				if result != 0 {
					t.Errorf("VectorCursor should be equal to itself, got comparison result: %d", result)
				}
			}
		}
	})
}

// FuzzMarshalWire fuzzes the round-trip behavior of MarshalWire/UnmarshalWire
func FuzzMarshalWire(f *testing.F) {
	// Seed with some valid cursors
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(100))
	f.Add(uint64(18446744073709551615)) // max uint64
	
	f.Fuzz(func(t *testing.T, seq uint64) {
		// Create an IntegerCursor
		originalCursor := IntegerCursor{Seq: seq}
		
		// Marshal it
		wireCursor, err := MarshalWire(originalCursor)
		if err != nil {
			t.Fatalf("MarshalWire failed: %v", err)
		}
		
		if wireCursor == nil {
			t.Fatal("MarshalWire returned nil WireCursor")
		}
		
		// Unmarshal it
		restoredCursor, err := UnmarshalWire(wireCursor)
		if err != nil {
			t.Fatalf("UnmarshalWire failed: %v", err)
		}
		
		// Cast to IntegerCursor for comparison
		restoredIC, ok := restoredCursor.(IntegerCursor)
		if !ok {
			t.Fatalf("Expected IntegerCursor, got %T", restoredCursor)
		}
		
		// They should be equal
		if originalCursor.Compare(restoredIC) != 0 {
			t.Errorf("Round-trip failed: original %v != restored %v", 
				originalCursor, restoredIC)
		}
		
		// String representations should be equal
		if originalCursor.String() != restoredIC.String() {
			t.Errorf("String representation mismatch: original %q != restored %q",
				originalCursor.String(), restoredIC.String())
		}
		
		// IsZero should match
		if originalCursor.IsZero() != restoredIC.IsZero() {
			t.Errorf("IsZero mismatch: original %v != restored %v",
				originalCursor.IsZero(), restoredIC.IsZero())
		}
	})
}

// FuzzWireCursorJSON fuzzes JSON marshaling/unmarshaling of WireCursor directly
func FuzzWireCursorJSON(f *testing.F) {
	// Seed with some valid JSON
	f.Add(`{"kind":"integer","data":42}`)
	f.Add(`{"kind":"bytes","data":"aGVsbG8="}`)
	f.Add(`{"kind":"string","data":"hello world"}`)
	
	f.Fuzz(func(t *testing.T, jsonData string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("JSON operations panicked on input %q: %v", jsonData, r)
			}
		}()
		
		// Try to unmarshal the fuzzed JSON
		var wireCursor WireCursor
		err := json.Unmarshal([]byte(jsonData), &wireCursor)
		if err != nil {
			// Invalid JSON is acceptable
			return
		}
		
		// If unmarshaling succeeded, try to marshal it back
		marshalledData, err := json.Marshal(&wireCursor)
		if err != nil {
			t.Errorf("Failed to marshal WireCursor back to JSON: %v", err)
			return
		}
		
		// Try to unmarshal the marshalled data again
		var wireCursor2 WireCursor
		err = json.Unmarshal(marshalledData, &wireCursor2)
		if err != nil {
			t.Errorf("Failed to unmarshal re-marshalled WireCursor: %v", err)
			return
		}
		
		// The Kind should be preserved
		if wireCursor.Kind != wireCursor2.Kind {
			t.Errorf("Kind mismatch after round-trip: %q != %q", 
				wireCursor.Kind, wireCursor2.Kind)
		}
		
		// The Data should be equivalent (though might be formatted differently)
		if wireCursor.Kind == "integer" || wireCursor.Kind == "string" {
			// For these types, the data should be exactly equal
			data1, _ := json.Marshal(wireCursor.Data)
			data2, _ := json.Marshal(wireCursor2.Data)
			if string(data1) != string(data2) {
				t.Errorf("Data mismatch after round-trip: %q != %q", data1, data2)
			}
		}
	})
}

// FuzzCursorComparison fuzzes cursor comparison operations
func FuzzCursorComparison(f *testing.F) {
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(1), uint64(2))
	f.Add(uint64(100), uint64(50))
	f.Add(uint64(18446744073709551615), uint64(0))
	f.Add(uint64(18446744073709551615), uint64(18446744073709551615))
	
	f.Fuzz(func(t *testing.T, seq1, seq2 uint64) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Cursor comparison panicked: %v", r)
			}
		}()
		
		cursor1 := IntegerCursor{Seq: seq1}
		cursor2 := IntegerCursor{Seq: seq2}
		
		// Test Compare method
		result := cursor1.Compare(cursor2)
		
		// Verify comparison properties
		if seq1 == seq2 && result != 0 {
			t.Errorf("Equal cursors should compare to 0, got %d", result)
		}
		if seq1 < seq2 && result >= 0 {
			t.Errorf("cursor1 < cursor2 should return negative, got %d", result)
		}
		if seq1 > seq2 && result <= 0 {
			t.Errorf("cursor1 > cursor2 should return positive, got %d", result)
		}
		
		// Test symmetry: Compare(a, b) should be opposite of Compare(b, a)
		reverseResult := cursor2.Compare(cursor1)
		if result != 0 && reverseResult != 0 {
			if (result > 0 && reverseResult > 0) || (result < 0 && reverseResult < 0) {
				t.Errorf("Comparison symmetry violated: Compare(%d, %d) = %d, Compare(%d, %d) = %d",
					seq1, seq2, result, seq2, seq1, reverseResult)
			}
		}
		
		// Test reflexivity: cursor should equal itself
		selfResult := cursor1.Compare(cursor1)
		if selfResult != 0 {
			t.Errorf("Cursor should be equal to itself, got %d", selfResult)
		}
		
		// Test that IsZero is consistent
		if seq1 == 0 && !cursor1.IsZero() {
			t.Error("Cursor with Seq=0 should be zero")
		}
		if seq1 != 0 && cursor1.IsZero() {
			t.Error("Cursor with non-zero Seq should not be zero")
		}
	})
}

// FuzzIntegerCursorEdgeCases fuzzes IntegerCursor with edge cases
func FuzzIntegerCursorEdgeCases(f *testing.F) {
	// Add some edge cases for uint64
	f.Add(uint64(0))                        // minimum
	f.Add(uint64(1))                        // minimum non-zero
	f.Add(uint64(18446744073709551615))     // maximum uint64
	f.Add(uint64(9223372036854775807))      // max int64
	f.Add(uint64(9223372036854775808))      // max int64 + 1
	
	f.Fuzz(func(t *testing.T, seq uint64) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("IntegerCursor operations panicked with seq=%d: %v", seq, r)
			}
		}()
		
		cursor := IntegerCursor{Seq: seq}
		
		// Test String method
		str := cursor.String()
		if str == "" {
			t.Error("String() should not return empty string")
		}
		
		// Test IsZero method
		isZero := cursor.IsZero()
		if seq == 0 && !isZero {
			t.Error("Seq=0 should be zero")
		}
		if seq != 0 && isZero {
			t.Errorf("Seq=%d should not be zero", seq)
		}
		
		// Test that we can marshal and unmarshal
		wireCursor, err := MarshalWire(cursor)
		if err != nil {
			t.Errorf("MarshalWire failed for seq=%d: %v", seq, err)
			return
		}
		
		restoredCursor, err := UnmarshalWire(wireCursor)
		if err != nil {
			t.Errorf("UnmarshalWire failed for seq=%d: %v", seq, err)
			return
		}
		
		// Cast to IntegerCursor for comparison
		restoredIC, ok := restoredCursor.(IntegerCursor)
		if !ok {
			t.Errorf("Expected IntegerCursor, got %T", restoredCursor)
			return
		}
		
		if cursor.Compare(restoredIC) != 0 {
			t.Errorf("Round-trip failed for seq=%d", seq)
		}
	})
}
