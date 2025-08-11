package codec

import (
	"encoding/json"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// mockCodec implements Codec for testing
type mockCodec struct {
	kind string
}

func (m *mockCodec) Kind() string {
	return m.kind
}

func (m *mockCodec) Encode(v any) (json.RawMessage, error) {
	return json.Marshal(v)
}

func (m *mockCodec) Decode(raw json.RawMessage) (any, error) {
	var result map[string]interface{}
	err := json.Unmarshal(raw, &result)
	return result, err
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if registry.codecs == nil {
		t.Fatal("Registry codecs map not initialized")
	}
	if len(registry.codecs) != 0 {
		t.Fatal("New registry should be empty")
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	codec := &mockCodec{kind: "test"}
	
	registry.Register(codec)
	
	if len(registry.codecs) != 1 {
		t.Fatalf("Expected 1 codec, got %d", len(registry.codecs))
	}
	
	stored, ok := registry.codecs["test"]
	if !ok {
		t.Fatal("Codec not stored with correct key")
	}
	if stored != codec {
		t.Fatal("Stored codec is not the same instance")
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	codec := &mockCodec{kind: "test"}
	registry.Register(codec)
	
	// Test existing codec
	retrieved, ok := registry.Get("test")
	if !ok {
		t.Fatal("Failed to retrieve existing codec")
	}
	if retrieved != codec {
		t.Fatal("Retrieved codec is not the same instance")
	}
	
	// Test non-existing codec
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Fatal("Should not retrieve non-existent codec")
	}
}

func TestRegistry_Kinds(t *testing.T) {
	registry := NewRegistry()
	
	// Test empty registry
	kinds := registry.Kinds()
	if len(kinds) != 0 {
		t.Fatal("Empty registry should return empty kinds slice")
	}
	
	// Test with multiple codecs
	codec1 := &mockCodec{kind: "type1"}
	codec2 := &mockCodec{kind: "type2"}
	codec3 := &mockCodec{kind: "type3"}
	
	registry.Register(codec1)
	registry.Register(codec2)
	registry.Register(codec3)
	
	kinds = registry.Kinds()
	if len(kinds) != 3 {
		t.Fatalf("Expected 3 kinds, got %d", len(kinds))
	}
	
	// Sort for consistent comparison
	sort.Strings(kinds)
	expected := []string{"type1", "type2", "type3"}
	sort.Strings(expected)
	
	if !reflect.DeepEqual(kinds, expected) {
		t.Fatalf("Expected kinds %v, got %v", expected, kinds)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	
	// Test concurrent registration and retrieval
	var wg sync.WaitGroup
	numGoroutines := 100
	
	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			codec := &mockCodec{kind: string(rune('a' + id))}
			registry.Register(codec)
		}(i)
	}
	
	wg.Wait()
	
	// Verify all codecs were registered
	if len(registry.codecs) != numGoroutines {
		t.Fatalf("Expected %d codecs, got %d", numGoroutines, len(registry.codecs))
	}
	
	// Concurrent retrieval
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			kind := string(rune('a' + id))
			_, ok := registry.Get(kind)
			if !ok {
				t.Errorf("Failed to retrieve codec %s", kind)
			}
		}(i)
	}
	
	wg.Wait()
}

func TestDefaultRegistry(t *testing.T) {
	// Save original state
	originalCodecs := make(map[string]Codec)
	DefaultRegistry.mu.RLock()
	for k, v := range DefaultRegistry.codecs {
		originalCodecs[k] = v
	}
	DefaultRegistry.mu.RUnlock()
	
	// Clean up after test
	defer func() {
		DefaultRegistry.mu.Lock()
		DefaultRegistry.codecs = originalCodecs
		DefaultRegistry.mu.Unlock()
	}()
	
	// Clear default registry for test
	DefaultRegistry.mu.Lock()
	DefaultRegistry.codecs = make(map[string]Codec)
	DefaultRegistry.mu.Unlock()
	
	codec := &mockCodec{kind: "default-test"}
	
	// Test global Register function
	Register(codec)
	
	// Test global Get function
	retrieved, ok := Get("default-test")
	if !ok {
		t.Fatal("Failed to retrieve from default registry")
	}
	if retrieved != codec {
		t.Fatal("Retrieved codec is not the same instance")
	}
	
	// Test non-existent
	_, ok = Get("nonexistent")
	if ok {
		t.Fatal("Should not retrieve non-existent codec from default registry")
	}
}

func TestCodec_Interface(t *testing.T) {
	codec := &mockCodec{kind: "interface-test"}
	
	// Test Kind method
	if codec.Kind() != "interface-test" {
		t.Fatalf("Expected kind 'interface-test', got %s", codec.Kind())
	}
	
	// Test Encode/Decode roundtrip
	original := map[string]interface{}{
		"test": "value",
		"num":  42,
	}
	
	encoded, err := codec.Encode(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	
	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	
	decodedMap, ok := decoded.(map[string]interface{})
	if !ok {
		t.Fatal("Decoded value is not map[string]interface{}")
	}
	
	if decodedMap["test"] != "value" || decodedMap["num"].(float64) != 42 {
		t.Fatalf("Roundtrip failed, got %v", decodedMap)
	}
}
