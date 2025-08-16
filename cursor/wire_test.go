package cursor

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"math"
)

// TestWireFormat_IntegerCursor_KnownFormats tests that integer cursors marshal to expected JSON
func TestWireFormat_IntegerCursor_KnownFormats(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name     string
		cursor   IntegerCursor
		expected string
	}{
		{
			name:     "zero cursor",
			cursor:   IntegerCursor{Seq: 0},
			expected: `{"kind":"integer","data":0}`,
		},
		{
			name:     "small positive",
			cursor:   IntegerCursor{Seq: 42},
			expected: `{"kind":"integer","data":42}`,
		},
		{
			name:     "large positive",
			cursor:   IntegerCursor{Seq: 12345678901234567890},
			expected: `{"kind":"integer","data":12345678901234567890}`,
		},
		{
			name:     "max uint64",
			cursor:   IntegerCursor{Seq: math.MaxUint64},
			expected: `{"kind":"integer","data":18446744073709551615}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			wire, err := MarshalWire(tt.cursor)
			if err != nil {
				t.Fatalf("MarshalWire() error = %v", err)
			}

			wireJSON, err := json.Marshal(wire)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			if string(wireJSON) != tt.expected {
				t.Errorf("MarshalWire() JSON = %s, want %s", wireJSON, tt.expected)
			}

			// Test round-trip
			var wireCursor WireCursor
			if err := json.Unmarshal([]byte(tt.expected), &wireCursor); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			restored, err := UnmarshalWire(&wireCursor)
			if err != nil {
				t.Fatalf("UnmarshalWire() error = %v", err)
			}

			restoredIC, ok := restored.(IntegerCursor)
			if !ok {
				t.Fatalf("Expected IntegerCursor, got %T", restored)
			}

			if restoredIC.Seq != tt.cursor.Seq {
				t.Errorf("Round-trip failed: got Seq=%d, want %d", restoredIC.Seq, tt.cursor.Seq)
			}
		})
	}
}

// TestWireFormat_VectorCursor_KnownFormats tests that vector cursors marshal to expected JSON
func TestWireFormat_VectorCursor_KnownFormats(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name     string
		cursor   VectorCursor
		expected string
	}{
		{
			name:     "empty vector",
			cursor:   VectorCursor{Counters: map[string]uint64{}},
			expected: `{"kind":"vector","data":{}}`,
		},
		{
			name: "single node",
			cursor: VectorCursor{Counters: map[string]uint64{
				"node1": 100,
			}},
			expected: `{"kind":"vector","data":{"node1":100}}`,
		},
		{
			name: "multiple nodes sorted",
			cursor: VectorCursor{Counters: map[string]uint64{
				"a": 1,
				"b": 2,
				"c": 3,
			}},
			// Note: JSON object key order is not guaranteed, so we test round-trip separately
			expected: `{"kind":"vector","data":{"a":1,"b":2,"c":3}}`,
		},
		{
			name: "realistic node names",
			cursor: VectorCursor{Counters: map[string]uint64{
				"web-server-1": 150,
				"worker-node-2": 89,
			}},
			// We'll validate this one via round-trip since key order varies
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			wire, err := MarshalWire(tt.cursor)
			if err != nil {
				t.Fatalf("MarshalWire() error = %v", err)
			}

			wireJSON, err := json.Marshal(wire)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// For cases where we know the expected format, validate it
			if tt.expected != "" {
				// Parse both to normalize for comparison (handle key ordering)
				var expected WireCursor
				if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
					t.Fatalf("json.Unmarshal(expected) error = %v", err)
				}

				var actual WireCursor
				if err := json.Unmarshal(wireJSON, &actual); err != nil {
					t.Fatalf("json.Unmarshal(actual) error = %v", err)
				}

				if actual.Kind != expected.Kind {
					t.Errorf("Kind mismatch: got %s, want %s", actual.Kind, expected.Kind)
				}

				// Compare data by unmarshaling both
				var expectedData, actualData map[string]uint64
				if err := json.Unmarshal(expected.Data, &expectedData); err != nil {
					t.Fatalf("json.Unmarshal(expected.Data) error = %v", err)
				}
				if err := json.Unmarshal(actual.Data, &actualData); err != nil {
					t.Fatalf("json.Unmarshal(actual.Data) error = %v", err)
				}

				if len(actualData) != len(expectedData) {
					t.Errorf("Data length mismatch: got %d, want %d", len(actualData), len(expectedData))
				}

				for k, v := range expectedData {
					if actualData[k] != v {
						t.Errorf("Data mismatch for key %s: got %d, want %d", k, actualData[k], v)
					}
				}
			}

			// Test round-trip for all cases
			restored, err := UnmarshalWire(wire)
			if err != nil {
				t.Fatalf("UnmarshalWire() error = %v", err)
			}

			restoredVC, ok := restored.(VectorCursor)
			if !ok {
				t.Fatalf("Expected VectorCursor, got %T", restored)
			}

			if len(restoredVC.Counters) != len(tt.cursor.Counters) {
				t.Errorf("Round-trip counters length mismatch: got %d, want %d", 
					len(restoredVC.Counters), len(tt.cursor.Counters))
			}

			for k, v := range tt.cursor.Counters {
				if restoredVC.Counters[k] != v {
					t.Errorf("Round-trip failed for key %s: got %d, want %d", k, restoredVC.Counters[k], v)
				}
			}
		})
	}
}

// TestWireFormat_BackwardCompatibility tests that cursors can be unmarshaled from historical formats
func TestWireFormat_BackwardCompatibility(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name          string
		historicalJSON string
		expectedType   string
		validate       func(t *testing.T, cursor Cursor)
	}{
		{
			name:           "v1.0 integer cursor",
			historicalJSON: `{"kind":"integer","data":123}`,
			expectedType:   "IntegerCursor",
			validate: func(t *testing.T, cursor Cursor) {
				ic := cursor.(IntegerCursor)
				if ic.Seq != 123 {
					t.Errorf("Expected Seq=123, got %d", ic.Seq)
				}
			},
		},
		{
			name:           "v1.0 zero integer cursor",
			historicalJSON: `{"kind":"integer","data":0}`,
			expectedType:   "IntegerCursor",
			validate: func(t *testing.T, cursor Cursor) {
				ic := cursor.(IntegerCursor)
				if ic.Seq != 0 {
					t.Errorf("Expected Seq=0, got %d", ic.Seq)
				}
				if !ic.IsZero() {
					t.Error("Expected IsZero()=true for zero cursor")
				}
			},
		},
		{
			name:           "v1.0 vector cursor empty",
			historicalJSON: `{"kind":"vector","data":{}}`,
			expectedType:   "VectorCursor",
			validate: func(t *testing.T, cursor Cursor) {
				vc := cursor.(VectorCursor)
				if len(vc.Counters) != 0 {
					t.Errorf("Expected empty counters, got %d entries", len(vc.Counters))
				}
				if !vc.IsZero() {
					t.Error("Expected IsZero()=true for empty vector cursor")
				}
			},
		},
		{
			name:           "v1.0 vector cursor with data",
			historicalJSON: `{"kind":"vector","data":{"node1":50,"node2":75}}`,
			expectedType:   "VectorCursor",
			validate: func(t *testing.T, cursor Cursor) {
				vc := cursor.(VectorCursor)
				if len(vc.Counters) != 2 {
					t.Errorf("Expected 2 counters, got %d", len(vc.Counters))
				}
				if vc.Counters["node1"] != 50 {
					t.Errorf("Expected node1=50, got %d", vc.Counters["node1"])
				}
				if vc.Counters["node2"] != 75 {
					t.Errorf("Expected node2=75, got %d", vc.Counters["node2"])
				}
				if vc.IsZero() {
					t.Error("Expected IsZero()=false for non-empty vector cursor")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wireCursor WireCursor
			if err := json.Unmarshal([]byte(tt.historicalJSON), &wireCursor); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			cursor, err := UnmarshalWire(&wireCursor)
			if err != nil {
				t.Fatalf("UnmarshalWire() error = %v", err)
			}

			// Validate type
			switch tt.expectedType {
			case "IntegerCursor":
				if _, ok := cursor.(IntegerCursor); !ok {
					t.Fatalf("Expected IntegerCursor, got %T", cursor)
				}
			case "VectorCursor":
				if _, ok := cursor.(VectorCursor); !ok {
					t.Fatalf("Expected VectorCursor, got %T", cursor)
				}
			default:
				t.Fatalf("Unknown expected type: %s", tt.expectedType)
			}

			// Run custom validation
			tt.validate(t, cursor)
		})
	}
}

// TestWireFormat_ForwardCompatibility tests handling of unknown cursor kinds
func TestWireFormat_ForwardCompatibility(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name        string
		futureJSON  string
		expectError string
	}{
		{
			name:        "unknown cursor kind",
			futureJSON:  `{"kind":"future-cursor","data":{"version":2,"data":"base64encoded"}}`,
			expectError: "unknown cursor kind: future-cursor",
		},
		{
			name:        "empty kind",
			futureJSON:  `{"kind":"","data":42}`,
			expectError: "unknown cursor kind: ",
		},
		{
			name:        "missing kind field",
			futureJSON:  `{"data":42}`,
			expectError: "unknown cursor kind: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wireCursor WireCursor
			if err := json.Unmarshal([]byte(tt.futureJSON), &wireCursor); err != nil {
				// If JSON is malformed, that's also acceptable for forward compatibility
				return
			}

			_, err := UnmarshalWire(&wireCursor)
			if err == nil {
				t.Error("Expected error for unknown cursor kind, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("Expected error containing %q, got %q", tt.expectError, err.Error())
			}
		})
	}
}

// TestWireFormat_SizeLimits tests that size limitations are enforced
func TestWireFormat_SizeLimits(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name     string
		dataSize int
		wantErr  bool
	}{
		{
			name:     "within limit",
			dataSize: 1000,
			wantErr:  false,
		},
		{
			name:     "at limit",
			dataSize: maxWireCursorSize,
			wantErr:  false,
		},
		{
			name:     "exceeds limit",
			dataSize: maxWireCursorSize + 1,
			wantErr:  true,
		},
		{
			name:     "far exceeds limit",
			dataSize: maxWireCursorSize * 2,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a wire cursor with payload of specified size
			largeData := make([]byte, tt.dataSize)
			for i := range largeData {
				largeData[i] = 'x'
			}

			wireCursor := &WireCursor{
				Kind: KindInteger,
				Data: largeData,
			}

			err := ValidateWireCursor(wireCursor)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error for oversized cursor, got nil")
				} else if !strings.Contains(err.Error(), "cursor payload too large") {
					t.Errorf("Expected 'cursor payload too large' error, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid cursor size: %v", err)
				}
			}
		})
	}
}

// TestWireFormat_RoundTripConsistency ensures marshal/unmarshal consistency across cursor types
func TestWireFormat_RoundTripConsistency(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name   string
		cursor Cursor
	}{
		{
			name:   "integer zero",
			cursor: IntegerCursor{Seq: 0},
		},
		{
			name:   "integer max",
			cursor: IntegerCursor{Seq: math.MaxUint64},
		},
		{
			name:   "vector empty",
			cursor: VectorCursor{Counters: map[string]uint64{}},
		},
		{
			name: "vector single node",
			cursor: VectorCursor{Counters: map[string]uint64{
				"node1": 100,
			}},
		},
		{
			name: "vector multiple nodes",
			cursor: VectorCursor{Counters: map[string]uint64{
				"web-1":     150,
				"web-2":     143,
				"worker-1":  89,
				"worker-2":  91,
				"db-1":      200,
				"cache-1":   175,
			}},
		},
		{
			name: "vector with zero counters",
			cursor: VectorCursor{Counters: map[string]uint64{
				"active":   100,
				"inactive": 0,
			}},
		},
		{
			name: "vector with max values",
			cursor: VectorCursor{Counters: map[string]uint64{
				"node1": math.MaxUint64,
				"node2": math.MaxUint64 - 1,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Marshal to wire format
			wire, err := MarshalWire(tt.cursor)
			if err != nil {
				t.Fatalf("MarshalWire() error = %v", err)
			}

			// Step 2: Validate wire format
			if err := ValidateWireCursor(wire); err != nil {
				t.Fatalf("ValidateWireCursor() error = %v", err)
			}

			// Step 3: Unmarshal from wire format
			restored, err := UnmarshalWire(wire)
			if err != nil {
				t.Fatalf("UnmarshalWire() error = %v", err)
			}

			// Step 4: Validate type preservation
			if restored.Kind() != tt.cursor.Kind() {
				t.Errorf("Kind mismatch: got %s, want %s", restored.Kind(), tt.cursor.Kind())
			}

			// Step 5: Validate content preservation based on cursor type
			switch original := tt.cursor.(type) {
			case IntegerCursor:
				restored, ok := restored.(IntegerCursor)
				if !ok {
					t.Fatalf("Type mismatch after round-trip: expected IntegerCursor, got %T", restored)
				}
				if restored.Seq != original.Seq {
					t.Errorf("Seq mismatch: got %d, want %d", restored.Seq, original.Seq)
				}
				if restored.Compare(original) != 0 {
					t.Error("Compare() should return 0 for identical cursors")
				}
				if restored.String() != original.String() {
					t.Errorf("String() mismatch: got %s, want %s", restored.String(), original.String())
				}
				if restored.IsZero() != original.IsZero() {
					t.Errorf("IsZero() mismatch: got %t, want %t", restored.IsZero(), original.IsZero())
				}

			case VectorCursor:
				restored, ok := restored.(VectorCursor)
				if !ok {
					t.Fatalf("Type mismatch after round-trip: expected VectorCursor, got %T", restored)
				}
				if len(restored.Counters) != len(original.Counters) {
					t.Errorf("Counters length mismatch: got %d, want %d", 
						len(restored.Counters), len(original.Counters))
				}
				for k, v := range original.Counters {
					if restored.Counters[k] != v {
						t.Errorf("Counter mismatch for key %s: got %d, want %d", k, restored.Counters[k], v)
					}
				}
				if restored.Compare(original) != 0 {
					t.Error("Compare() should return 0 for identical cursors")
				}
				if restored.IsZero() != original.IsZero() {
					t.Errorf("IsZero() mismatch: got %t, want %t", restored.IsZero(), original.IsZero())
				}
			}
		})
	}
}

// TestWireFormat_JSONCompliance tests that wire format produces valid JSON
func TestWireFormat_JSONCompliance(t *testing.T) {
	InitDefaultCodecs()

	cursors := []Cursor{
		IntegerCursor{Seq: 42},
		VectorCursor{Counters: map[string]uint64{"test": 123}},
	}

	for i, cursor := range cursors {
		t.Run(fmt.Sprintf("cursor_%d", i), func(t *testing.T) {
			wire, err := MarshalWire(cursor)
			if err != nil {
				t.Fatalf("MarshalWire() error = %v", err)
			}

			// Marshal to JSON
			jsonBytes, err := json.Marshal(wire)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Validate JSON can be unmarshaled
			var decoded WireCursor
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Check structure
			if decoded.Kind != wire.Kind {
				t.Errorf("Kind mismatch after JSON round-trip: got %s, want %s", decoded.Kind, wire.Kind)
			}

			if len(decoded.Data) != len(wire.Data) {
				t.Errorf("Data length mismatch after JSON round-trip: got %d, want %d", 
					len(decoded.Data), len(wire.Data))
			}

			// Validate it can still be unmarshaled to cursor
			restoredCursor, err := UnmarshalWire(&decoded)
			if err != nil {
				t.Fatalf("UnmarshalWire() after JSON round-trip error = %v", err)
			}

			if restoredCursor.Kind() != cursor.Kind() {
				t.Errorf("Cursor kind mismatch after JSON round-trip: got %s, want %s", 
					restoredCursor.Kind(), cursor.Kind())
			}
		})
	}
}

// TestWireFormat_EdgeCases tests various edge cases for wire format handling
func TestWireFormat_EdgeCases(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name    string
		test    func(t *testing.T)
	}{
		{
			name: "nil wire cursor validation",
			test: func(t *testing.T) {
				err := ValidateWireCursor(nil)
				if err == nil || !strings.Contains(err.Error(), "nil wire cursor") {
					t.Errorf("Expected 'nil wire cursor' error, got %v", err)
				}
			},
		},
		{
			name: "empty JSON data",
			test: func(t *testing.T) {
				wire := &WireCursor{
					Kind: KindInteger,
					Data: json.RawMessage(``),
				}
				_, err := UnmarshalWire(wire)
				if err == nil {
					t.Error("Expected error for empty JSON data")
				}
			},
		},
		{
			name: "invalid JSON data",
			test: func(t *testing.T) {
				wire := &WireCursor{
					Kind: KindInteger,
					Data: json.RawMessage(`{invalid json`),
				}
				_, err := UnmarshalWire(wire)
				if err == nil {
					t.Error("Expected error for invalid JSON data")
				}
			},
		},
		{
			name: "wrong data type for integer",
			test: func(t *testing.T) {
				wire := &WireCursor{
					Kind: KindInteger,
					Data: json.RawMessage(`"not a number"`),
				}
				_, err := UnmarshalWire(wire)
				if err == nil {
					t.Error("Expected error for non-numeric data in integer cursor")
				}
			},
		},
		{
			name: "wrong data type for vector",
			test: func(t *testing.T) {
				wire := &WireCursor{
					Kind: KindVector,
					Data: json.RawMessage(`"not an object"`),
				}
				_, err := UnmarshalWire(wire)
				if err == nil {
					t.Error("Expected error for non-object data in vector cursor")
				}
			},
		},
		{
			name: "negative number in integer cursor",
			test: func(t *testing.T) {
				wire := &WireCursor{
					Kind: KindInteger,
					Data: json.RawMessage(`-123`),
				}
				// This should actually work since JSON numbers can represent negative values,
				// but the uint64 conversion will handle it
				_, err := UnmarshalWire(wire)
				if err == nil {
					t.Error("Expected error for negative number in integer cursor")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// TestWireFormat_VersionInteroperability tests interoperability scenarios
func TestWireFormat_VersionInteroperability(t *testing.T) {
	InitDefaultCodecs()

	// Simulate different library versions by testing various JSON formats
	scenarios := []struct {
		name   string
		json   string
		expect func(t *testing.T, cursor Cursor, err error)
	}{
		{
			name: "minimal integer cursor",
			json: `{"kind":"integer","data":0}`,
			expect: func(t *testing.T, cursor Cursor, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				ic := cursor.(IntegerCursor)
				if ic.Seq != 0 {
					t.Errorf("Expected Seq=0, got %d", ic.Seq)
				}
			},
		},
		{
			name: "vector with complex node names",
			json: `{"kind":"vector","data":{"us-east-1-web-01":100,"eu-west-1-worker-03":200}}`,
			expect: func(t *testing.T, cursor Cursor, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				vc := cursor.(VectorCursor)
				if vc.Counters["us-east-1-web-01"] != 100 {
					t.Errorf("Expected us-east-1-web-01=100, got %d", vc.Counters["us-east-1-web-01"])
				}
				if vc.Counters["eu-west-1-worker-03"] != 200 {
					t.Errorf("Expected eu-west-1-worker-03=200, got %d", vc.Counters["eu-west-1-worker-03"])
				}
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var wire WireCursor
			if err := json.Unmarshal([]byte(scenario.json), &wire); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			cursor, err := UnmarshalWire(&wire)
			scenario.expect(t, cursor, err)
		})
	}
}
