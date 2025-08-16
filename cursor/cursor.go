package cursor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync" // NEW

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

const (
	KindInteger = "integer"
	KindVector  = "vector"
)

type Cursor interface {
	Kind() string
}

// Codec for marshaling/unmarshaling cursors to a stable wire form.
type Codec interface {
	Kind() string
	Marshal(c Cursor) (json.RawMessage, error)      // returns the Data part only
	Unmarshal(data json.RawMessage) (Cursor, error) // parse Data into a Cursor
}

var (
	registry   = map[string]Codec{}
	registryMu sync.RWMutex // NEW
)

func Register(c Codec) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[c.Kind()] = c
}

func Lookup(kind string) (Codec, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	cc, ok := registry[kind]
	return cc, ok
}

// Maximum allowed size for a wire cursor payload.
const maxWireCursorSize = 64 * 1024 // 64 KB

// WireCursor is the typed union for transport (HTTP JSON).
type WireCursor struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

func MarshalWire(c Cursor) (*WireCursor, error) {
	codec, ok := Lookup(c.Kind())
	if !ok {
		return nil, fmt.Errorf("unknown cursor kind: %s", c.Kind())
	}
	data, err := codec.Marshal(c)
	if err != nil {
		return nil, err
	}
	return &WireCursor{Kind: codec.Kind(), Data: data}, nil
}

func ValidateWireCursor(wc *WireCursor) error {
	if wc == nil {
		return errors.New("nil wire cursor")
	}
	if len(wc.Data) > maxWireCursorSize {
		return fmt.Errorf("cursor payload too large: %d bytes", len(wc.Data))
	}
	// Check if codec exists before trying to unmarshal
	_, ok := Lookup(wc.Kind)
	if !ok {
		return fmt.Errorf("unknown cursor kind: %s", wc.Kind)
	}
	return nil
}

func UnmarshalWire(wc *WireCursor) (Cursor, error) {
	if err := ValidateWireCursor(wc); err != nil {
		return nil, err
	}
	codec, ok := Lookup(wc.Kind)
	if !ok {
		return nil, fmt.Errorf("unknown cursor kind: %s", wc.Kind)
	}
	return codec.Unmarshal(wc.Data)
}

// IntegerCursor is a simple high-water mark (seq).
type IntegerCursor struct {
	Seq uint64
}

func (IntegerCursor) Kind() string { return KindInteger }

// Compare implements types.Version
func (ic IntegerCursor) Compare(other types.Version) int {
	// Handle nil case first
	if other == nil {
		return 1 // non-nil is greater than nil
	}
	// Handle type mismatch
	oc, ok := other.(IntegerCursor)
	if !ok {
		return 0 // incomparable across different version types
	}
	if ic.Seq < oc.Seq {
		return -1
	}
	if ic.Seq > oc.Seq {
		return 1
	}
	return 0
}

// String implements types.Version
func (ic IntegerCursor) String() string {
	return fmt.Sprintf("%d", ic.Seq)
}

// IsZero implements types.Version
func (ic IntegerCursor) IsZero() bool {
	return ic.Seq == 0
}

type integerCodec struct{}

func (integerCodec) Kind() string { return KindInteger }

func (integerCodec) Marshal(c Cursor) (json.RawMessage, error) {
	ic, ok := c.(IntegerCursor)
	if !ok {
		return nil, fmt.Errorf("expected IntegerCursor, got %T", c)
	}
	// Encode as JSON number for readability; could also be a small struct if preferred.
	return json.Marshal(ic.Seq)
}

func (integerCodec) Unmarshal(data json.RawMessage) (Cursor, error) {
	var seq uint64
	if err := json.Unmarshal(data, &seq); err != nil {
		return nil, err
	}
	return IntegerCursor{Seq: seq}, nil
}

// VectorCursor is a dotted-vector summary: map[node]counter
type VectorCursor struct {
	Counters map[string]uint64
}

func (VectorCursor) Kind() string { return KindVector }

// Compare implements types.Version
func (vc VectorCursor) Compare(other types.Version) int {
	// Handle nil case first
	if other == nil {
		return 1 // non-nil is greater than nil
	}

	// Handle type mismatch
	oc, ok := other.(VectorCursor)
	if !ok {
		return 0 // incomparable across different version types
	}

	// Convert both vectors to JSON for comparison
	vcJSON, _ := json.Marshal(vc.Counters)
	ocJSON, _ := json.Marshal(oc.Counters)

	// Compare byte-by-byte
	return bytes.Compare(vcJSON, ocJSON)
}

// String implements types.Version
func (vc VectorCursor) String() string {
	data, _ := json.Marshal(vc.Counters)
	return string(data)
}

// IsZero implements types.Version
func (vc VectorCursor) IsZero() bool {
	// Nil map or empty map is considered zero
	return vc.Counters == nil || len(vc.Counters) == 0
}

type vectorCodec struct{}

func (vectorCodec) Kind() string { return KindVector }

func (vectorCodec) Marshal(c Cursor) (json.RawMessage, error) {
	vc, ok := c.(VectorCursor)
	if !ok {
		return nil, fmt.Errorf("expected VectorCursor, got %T", c)
	}
	// Stable canonical map encoding via JSON
	return json.Marshal(vc.Counters)
}

func (vectorCodec) Unmarshal(data json.RawMessage) (Cursor, error) {
	var m map[string]uint64
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return VectorCursor{Counters: m}, nil
}

func InitDefaultCodecs() {
	Register(integerCodec{})
	Register(vectorCodec{})
}

// NewInteger creates a new IntegerCursor with the given sequence number
func NewInteger(seq uint64) IntegerCursor {
	return IntegerCursor{Seq: seq}
}

// NewVector creates a new VectorCursor with the given counters
func NewVector(counters map[string]uint64) VectorCursor {
	return VectorCursor{Counters: counters}
}

// MustMarshalWire marshals a cursor to WireCursor format, panicking on error
// This is useful for cases where the cursor is known to be valid
func MustMarshalWire(c Cursor) WireCursor {
	wc, err := MarshalWire(c)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal cursor: %v", err))
	}
	return *wc
}

// MustUnmarshalWire unmarshals a WireCursor to a Cursor, panicking on error
// This is useful for cases where the wire cursor is known to be valid
func MustUnmarshalWire(data []byte) Cursor {
	var wc WireCursor
	if err := json.Unmarshal(data, &wc); err != nil {
		panic(fmt.Sprintf("failed to unmarshal wire cursor: %v", err))
	}
	c, err := UnmarshalWire(&wc)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal cursor: %v", err))
	}
	return c
}

// Optional: compact binary helpers if you later want non-JSON wire payloads.
func EncodeUvarint(u uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, u)
	return buf[:n]
}

// ConcurrentTestCodec is a test-only codec for concurrent registry tests
type ConcurrentTestCodec struct {
	ID string
}

func (c ConcurrentTestCodec) Kind() string { return c.ID }

func (ConcurrentTestCodec) Marshal(c Cursor) (json.RawMessage, error) {
	return nil, nil
}

func (ConcurrentTestCodec) Unmarshal(data json.RawMessage) (Cursor, error) {
	return nil, nil
}
