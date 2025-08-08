package cursor

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/bits"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentRegistryAccess(t *testing.T) {
	// Reset registry for test
	registryMu.Lock()
	registry = map[string]Codec{}
	registryMu.Unlock()

	const (
		numWorkers = 10
		numOps     = 100
	)

	var wg sync.WaitGroup
	wg.Add(numWorkers * 2) // Workers for both Register and Lookup

	// Launch workers to Register codecs
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				kind := fmt.Sprintf("test-codec-%d-%d", id, j)
				Register(ConcurrentTestCodec{ID: kind})
			}
		}(i)
	}

	// Launch workers to Lookup codecs
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				kind := fmt.Sprintf("test-codec-%d-%d", id, j)
				_, _ = Lookup(kind) // Ignore result, just test concurrency
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	registryMu.RLock()
	registrySize := len(registry)
	registryMu.RUnlock()

	// Each worker registers numOps unique codecs
	expectedSize := numWorkers * numOps
	if registrySize != expectedSize {
		t.Errorf("Registry size = %d; want %d", registrySize, expectedSize)
	}
}

func TestEncodeUvarint(t *testing.T) {
	tests := []struct {
		name  string
		input uint64
	}{
		{"zero", 0},
		{"small", 42},
		{"medium", 1234567},
		{"large", 1<<63 - 1},
		{"max", ^uint64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeUvarint(tt.input)

			// Check encoded result is non-empty
			if len(encoded) == 0 {
				t.Error("Encoded result is empty")
			}

			// Verify we can decode it back correctly
			decoded, n := binary.Uvarint(encoded)
			if n <= 0 {
				t.Error("Failed to decode uvarint")
			}
			if decoded != tt.input {
				t.Errorf("Decoded value = %d; want %d", decoded, tt.input)
			}

			// Verify encoded length matches expected for value
			expectedLen := (bits.Len64(tt.input) + 6) / 7
			if tt.input == 0 {
				expectedLen = 1
			}
			if len(encoded) != expectedLen {
				t.Errorf("Encoded length = %d; want %d", len(encoded), expectedLen)
			}
		})
	}
}

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

func TestValidateWireCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  *WireCursor
		wantErr string
	}{
		{
			name:    "nil cursor",
			cursor:  nil,
			wantErr: "nil wire cursor",
		},
		{
			name: "payload too large",
			cursor: &WireCursor{
				Kind: KindInteger,
				Data: make([]byte, maxWireCursorSize+1),
			},
			wantErr: "cursor payload too large",
		},
		{
			name: "unknown kind",
			cursor: &WireCursor{
				Kind: "unknown",
				Data: []byte(`{}`),
			},
			wantErr: "unknown cursor kind",
		},
		{
			name: "valid cursor",
			cursor: &WireCursor{
				Kind: KindInteger,
				Data: []byte(`42`),
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWireCursor(tt.cursor)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateWireCursor() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ValidateWireCursor() error = %v, wantErr %v", err, tt.wantErr)
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
