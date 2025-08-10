package httptransport

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/codec"
)

// Test codec that adds a prefix to strings
type testCodec struct {
	kind string
}

func (t *testCodec) Kind() string {
	return t.kind
}

func (t *testCodec) Encode(v any) (json.RawMessage, error) {
	if str, ok := v.(string); ok {
		prefixed := "PREFIX:" + str
		return json.Marshal(prefixed)
	}
	return json.Marshal(v)
}

func (t *testCodec) Decode(raw json.RawMessage) (any, error) {
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return nil, err
	}
	if len(str) > 7 && str[:7] == "PREFIX:" {
		return str[7:], nil
	}
	return str, nil
}

// Test event implementation
type testEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (e *testEvent) ID() string                       { return e.id }
func (e *testEvent) Type() string                     { return e.eventType }
func (e *testEvent) AggregateID() string              { return e.aggregateID }
func (e *testEvent) Data() interface{}                { return e.data }
func (e *testEvent) Metadata() map[string]interface{} { return e.metadata }

func TestNewCodecAwareEncoder(t *testing.T) {
	// Test with custom registry
	registry := codec.NewRegistry()
	encoder := NewCodecAwareEncoder(registry, true)
	if encoder.registry != registry {
		t.Fatal("Expected encoder to use provided registry")
	}
	if !encoder.fallback {
		t.Fatal("Expected fallback to be true")
	}

	// Test with nil registry (should use default)
	encoder2 := NewCodecAwareEncoder(nil, false)
	if encoder2.registry != codec.DefaultRegistry {
		t.Fatal("Expected encoder to use default registry when nil provided")
	}
	if encoder2.fallback {
		t.Fatal("Expected fallback to be false")
	}
}

func TestCodecAwareEncoder_EncodeEvent_WithCodec(t *testing.T) {
	registry := codec.NewRegistry()
	testCodec := &testCodec{kind: "test"}
	registry.Register(testCodec)
	
	encoder := NewCodecAwareEncoder(registry, true)
	
	event := &testEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "hello",
		metadata: map[string]interface{}{
			"kind": "test",
		},
	}
	
	wireEvent, err := encoder.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}
	
	if wireEvent.ID != "test-1" {
		t.Fatalf("Expected ID 'test-1', got %s", wireEvent.ID)
	}
	if wireEvent.DataKind != "test" {
		t.Fatalf("Expected DataKind 'test', got %s", wireEvent.DataKind)
	}
	
	// Verify the data was encoded with our test codec
	var encoded string
	if err := json.Unmarshal(wireEvent.Data, &encoded); err != nil {
		t.Fatalf("Failed to unmarshal encoded data: %v", err)
	}
	if encoded != "PREFIX:hello" {
		t.Fatalf("Expected 'PREFIX:hello', got %s", encoded)
	}
}

func TestCodecAwareEncoder_EncodeEvent_Fallback(t *testing.T) {
	registry := codec.NewRegistry()
	encoder := NewCodecAwareEncoder(registry, true)
	
	event := &testEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "hello",
		metadata:    map[string]interface{}{},
	}
	
	wireEvent, err := encoder.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}
	
	if wireEvent.DataKind != "" {
		t.Fatalf("Expected no DataKind, got %s", wireEvent.DataKind)
	}
	
	// Verify the data was encoded with JSON
	var data interface{}
	if err := json.Unmarshal(wireEvent.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}
	if data != "hello" {
		t.Fatalf("Expected 'hello', got %v", data)
	}
}

func TestCodecAwareEncoder_EncodeEvent_NoFallback(t *testing.T) {
	registry := codec.NewRegistry()
	encoder := NewCodecAwareEncoder(registry, false)
	
	event := &testEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "hello",
		metadata: map[string]interface{}{
			"kind": "unknown",
		},
	}
	
	_, err := encoder.EncodeEvent(event)
	if err == nil {
		t.Fatal("Expected error when codec not found and fallback disabled")
	}
}

func TestCodecAwareEncoder_DecodeEvent_WithCodec(t *testing.T) {
	registry := codec.NewRegistry()
	testCodec := &testCodec{kind: "test"}
	registry.Register(testCodec)
	
	encoder := NewCodecAwareEncoder(registry, true)
	
	// Create a wire event that was encoded with our test codec
	data, _ := json.Marshal("PREFIX:hello")
	wireEvent := WireEvent{
		ID:          "test-1",
		Type:        "TestEvent",
		AggregateID: "agg-1",
		Data:        data,
		DataKind:    "test",
		Metadata:    map[string]interface{}{},
	}
	
	event, err := encoder.DecodeEvent(wireEvent)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}
	
	if event.ID() != "test-1" {
		t.Fatalf("Expected ID 'test-1', got %s", event.ID())
	}
	if event.Data() != "hello" {
		t.Fatalf("Expected 'hello', got %v", event.Data())
	}
}

func TestCodecAwareEncoder_DecodeEvent_JSONFallback(t *testing.T) {
	registry := codec.NewRegistry()
	encoder := NewCodecAwareEncoder(registry, true)
	
	// Create a wire event without codec info
	data, _ := json.Marshal("hello")
	wireEvent := WireEvent{
		ID:          "test-1",
		Type:        "TestEvent",
		AggregateID: "agg-1",
		Data:        data,
		Metadata:    map[string]interface{}{},
	}
	
	event, err := encoder.DecodeEvent(wireEvent)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}
	
	if event.Data() != "hello" {
		t.Fatalf("Expected 'hello', got %v", event.Data())
	}
}

func TestCodecAwareEncoder_RoundTrip(t *testing.T) {
	registry := codec.NewRegistry()
	testCodec := &testCodec{kind: "test"}
	registry.Register(testCodec)
	
	encoder := NewCodecAwareEncoder(registry, true)
	
	original := &testEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "hello",
		metadata: map[string]interface{}{
			"kind": "test",
			"other": "metadata",
		},
	}
	
	// Encode
	wireEvent, err := encoder.EncodeEvent(original)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}
	
	// Decode
	decoded, err := encoder.DecodeEvent(wireEvent)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}
	
	// Verify round-trip preserved data correctly
	if decoded.ID() != original.ID() {
		t.Fatalf("ID mismatch: expected %s, got %s", original.ID(), decoded.ID())
	}
	if decoded.Type() != original.Type() {
		t.Fatalf("Type mismatch: expected %s, got %s", original.Type(), decoded.Type())
	}
	if decoded.AggregateID() != original.AggregateID() {
		t.Fatalf("AggregateID mismatch: expected %s, got %s", original.AggregateID(), decoded.AggregateID())
	}
	if decoded.Data() != original.Data() {
		t.Fatalf("Data mismatch: expected %v, got %v", original.Data(), decoded.Data())
	}
	if !reflect.DeepEqual(decoded.Metadata(), original.Metadata()) {
		t.Fatalf("Metadata mismatch: expected %v, got %v", original.Metadata(), decoded.Metadata())
	}
}

func TestCodecAwareEncoder_EncodeEventWithVersion(t *testing.T) {
	registry := codec.NewRegistry()
	encoder := NewCodecAwareEncoder(registry, true)
	
	event := &testEvent{
		id:          "test-1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "hello",
		metadata:    map[string]interface{}{},
	}
	
	version := cursor.IntegerCursor{Seq: 42}
	ev := synckit.EventWithVersion{
		Event:   event,
		Version: version,
	}
	
	wireEv, err := encoder.EncodeEventWithVersion(ev)
	if err != nil {
		t.Fatalf("EncodeEventWithVersion failed: %v", err)
	}
	
	if wireEv.Version != "42" {
		t.Fatalf("Expected version '42', got %s", wireEv.Version)
	}
	if wireEv.Event.ID != "test-1" {
		t.Fatalf("Expected ID 'test-1', got %s", wireEv.Event.ID)
	}
}

func TestCodecAwareEncoder_DecodeEventWithVersion(t *testing.T) {
	registry := codec.NewRegistry()
	encoder := NewCodecAwareEncoder(registry, true)
	
	data, _ := json.Marshal("hello")
	wireEv := WireEventWithVersion{
		Event: WireEvent{
			ID:          "test-1",
			Type:        "TestEvent",
			AggregateID: "agg-1",
			Data:        data,
			Metadata:    map[string]interface{}{},
		},
		Version: "42",
	}
	
	parser := func(ctx context.Context, s string) (synckit.Version, error) {
		return cursor.IntegerCursor{Seq: 42}, nil
	}
	
	ev, err := encoder.DecodeEventWithVersion(context.Background(), parser, wireEv)
	if err != nil {
		t.Fatalf("DecodeEventWithVersion failed: %v", err)
	}
	
	if ev.Event.ID() != "test-1" {
		t.Fatalf("Expected ID 'test-1', got %s", ev.Event.ID())
	}
	if ev.Version.String() != "42" {
		t.Fatalf("Expected version '42', got %s", ev.Version.String())
	}
}

func TestCodecKindDetection(t *testing.T) {
	registry := codec.NewRegistry()
	testCodec := &testCodec{kind: "test"}
	registry.Register(testCodec)
	encoder := NewCodecAwareEncoder(registry, true)
	
	testCases := []struct {
		name     string
		metadata map[string]interface{}
		expected string
	}{
		{
			name:     "kind metadata",
			metadata: map[string]interface{}{"kind": "test"},
			expected: "test",
		},
		{
			name:     "codec_kind metadata",
			metadata: map[string]interface{}{"codec_kind": "test"},
			expected: "test",
		},
		{
			name:     "data_kind metadata",
			metadata: map[string]interface{}{"data_kind": "test"},
			expected: "test",
		},
		{
			name:     "no metadata",
			metadata: map[string]interface{}{},
			expected: "",
		},
		{
			name:     "wrong type",
			metadata: map[string]interface{}{"kind": 123},
			expected: "",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := &testEvent{
				id:          "test-1",
				eventType:   "TestEvent",
				aggregateID: "agg-1",
				data:        "hello",
				metadata:    tc.metadata,
			}
			
			wireEvent, err := encoder.EncodeEvent(event)
			if err != nil {
				t.Fatalf("EncodeEvent failed: %v", err)
			}
			
			if wireEvent.DataKind != tc.expected {
				t.Fatalf("Expected DataKind '%s', got '%s'", tc.expected, wireEvent.DataKind)
			}
		})
	}
}

func TestLegacyConversion(t *testing.T) {
	jsonEvent := JSONEvent{
		ID:          "test-1",
		Type:        "TestEvent",
		AggregateID: "agg-1",
		Data:        "hello",
		Metadata:    map[string]interface{}{"test": "value"},
	}
	
	// Convert to wire format
	wireEvent, err := toWireEvent(jsonEvent)
	if err != nil {
		t.Fatalf("toWireEvent failed: %v", err)
	}
	
	if wireEvent.ID != jsonEvent.ID {
		t.Fatalf("ID mismatch: expected %s, got %s", jsonEvent.ID, wireEvent.ID)
	}
	
	// Convert back to JSON format
	converted, err := fromWireEvent(wireEvent)
	if err != nil {
		t.Fatalf("fromWireEvent failed: %v", err)
	}
	
	if converted.ID != jsonEvent.ID {
		t.Fatalf("ID mismatch: expected %s, got %s", jsonEvent.ID, converted.ID)
	}
	if converted.Data != jsonEvent.Data {
		t.Fatalf("Data mismatch: expected %v, got %v", jsonEvent.Data, converted.Data)
	}
	if !reflect.DeepEqual(converted.Metadata, jsonEvent.Metadata) {
		t.Fatalf("Metadata mismatch: expected %v, got %v", jsonEvent.Metadata, converted.Metadata)
	}
}
