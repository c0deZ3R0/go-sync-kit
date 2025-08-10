package codec

import (
	"encoding/json"
	"sync"
)

// Codec defines an interface for encoding and decoding event data
// with a specific kind identifier for registry lookup.
type Codec interface {
	// Kind returns the unique identifier for this codec type
	Kind() string
	// Encode converts any value to JSON raw message
	Encode(any) (json.RawMessage, error)
	// Decode converts JSON raw message back to typed value
	Decode(json.RawMessage) (any, error)
}

// Registry manages codec registration and lookup with thread safety.
// It provides a centralized way to register domain-specific codecs
// and retrieve them by kind for event data encoding/decoding.
type Registry struct {
	mu     sync.RWMutex
	codecs map[string]Codec
}

// NewRegistry creates a new codec registry instance.
func NewRegistry() *Registry {
	return &Registry{
		codecs: make(map[string]Codec),
	}
}

// Register adds a codec to the registry using its Kind() as the key.
// This allows domain-specific types to be encoded/decoded without
// double-encoding through generic JSON marshaling.
func (r *Registry) Register(c Codec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codecs[c.Kind()] = c
}

// Get retrieves a codec by its kind identifier.
// Returns the codec and true if found, nil and false otherwise.
func (r *Registry) Get(kind string) (Codec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.codecs[kind]
	return c, ok
}

// Kinds returns all registered codec kinds for debugging/introspection.
func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	kinds := make([]string, 0, len(r.codecs))
	for kind := range r.codecs {
		kinds = append(kinds, kind)
	}
	return kinds
}

// DefaultRegistry provides a global registry instance for convenience.
// Applications can use this directly or create their own registry instances.
var DefaultRegistry = NewRegistry()

// Register is a convenience function that registers a codec with the default registry.
func Register(c Codec) {
	DefaultRegistry.Register(c)
}

// Get is a convenience function that retrieves a codec from the default registry.
func Get(kind string) (Codec, bool) {
	return DefaultRegistry.Get(kind)
}
