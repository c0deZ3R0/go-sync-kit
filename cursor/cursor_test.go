package cursor

import (
	"encoding/json"
	"testing"
)

func TestIntegerCursor(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name    string
		cursor  IntegerCursor
		wantErr bool
	}{
		{
			name:    "zero",
			cursor:  IntegerCursor{Seq: 0},
			wantErr: false,
		},
		{
			name:    "positive",
			cursor:  IntegerCursor{Seq: 123},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			wire, err := MarshalWire(tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalWire() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Test unmarshaling
			got, err := UnmarshalWire(wire)
			if err != nil {
				t.Errorf("UnmarshalWire() error = %v", err)
				return
			}

			// Compare results
			ic, ok := got.(IntegerCursor)
			if !ok {
				t.Errorf("UnmarshalWire() got = %T, want IntegerCursor", got)
				return
			}
			if ic.Seq != tt.cursor.Seq {
				t.Errorf("UnmarshalWire() got = %v, want %v", ic.Seq, tt.cursor.Seq)
			}
		})
	}
}

func TestVectorCursor(t *testing.T) {
	InitDefaultCodecs()

	tests := []struct {
		name    string
		cursor  VectorCursor
		wantErr bool
	}{
		{
			name: "empty",
			cursor: VectorCursor{
				Counters: map[string]uint64{},
			},
			wantErr: false,
		},
		{
			name: "single counter",
			cursor: VectorCursor{
				Counters: map[string]uint64{"node1": 123},
			},
			wantErr: false,
		},
		{
			name: "multiple counters",
			cursor: VectorCursor{
				Counters: map[string]uint64{
					"node1": 123,
					"node2": 456,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			wire, err := MarshalWire(tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalWire() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Test unmarshaling
			got, err := UnmarshalWire(wire)
			if err != nil {
				t.Errorf("UnmarshalWire() error = %v", err)
				return
			}

			// Compare results
			vc, ok := got.(VectorCursor)
			if !ok {
				t.Errorf("UnmarshalWire() got = %T, want VectorCursor", got)
				return
			}

			// Compare maps
			if len(vc.Counters) != len(tt.cursor.Counters) {
				t.Errorf("UnmarshalWire() got len = %v, want %v", len(vc.Counters), len(tt.cursor.Counters))
				return
			}
			for k, v := range tt.cursor.Counters {
				if got := vc.Counters[k]; got != v {
					t.Errorf("UnmarshalWire() counter[%s] = %v, want %v", k, got, v)
				}
			}
		})
	}
}

func TestWireCursor_InvalidKind(t *testing.T) {
	InitDefaultCodecs()

	wire := &WireCursor{
		Kind: "invalid",
		Data: json.RawMessage(`123`),
	}

	_, err := UnmarshalWire(wire)
	if err == nil {
		t.Error("UnmarshalWire() expected error for invalid kind")
	}
}
