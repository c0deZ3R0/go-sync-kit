package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/codec"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

// UserProfile represents a complex user profile structure
type UserProfile struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Settings map[string]string `json:"settings"`
}

// UserProfileCodec implements codec.Codec for UserProfile
type UserProfileCodec struct{}

func (c *UserProfileCodec) Kind() string {
	return "user_profile"
}

func (c *UserProfileCodec) Encode(v any) (json.RawMessage, error) {
	// Add some custom processing or validation before encoding
	profile, ok := v.(*UserProfile)
	if !ok {
		return nil, fmt.Errorf("expected *UserProfile, got %T", v)
	}
	
	// Custom encoding logic - could add encryption, compression, etc.
	log.Printf("Encoding user profile: %s", profile.ID)
	return json.Marshal(profile)
}

func (c *UserProfileCodec) Decode(raw json.RawMessage) (any, error) {
	var profile UserProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("failed to decode user profile: %w", err)
	}
	
	// Custom decoding logic - could add decryption, decompression, etc.
	log.Printf("Decoded user profile: %s", profile.ID)
	return &profile, nil
}

// OrderData represents an order structure
type OrderData struct {
	OrderID   string  `json:"order_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// OrderCodec implements codec.Codec for OrderData
type OrderCodec struct{}

func (c *OrderCodec) Kind() string {
	return "order"
}

func (c *OrderCodec) Encode(v any) (json.RawMessage, error) {
	order, ok := v.(*OrderData)
	if !ok {
		return nil, fmt.Errorf("expected *OrderData, got %T", v)
	}
	
	log.Printf("Encoding order: %s", order.OrderID)
	return json.Marshal(order)
}

func (c *OrderCodec) Decode(raw json.RawMessage) (any, error) {
	var order OrderData
	if err := json.Unmarshal(raw, &order); err != nil {
		return nil, fmt.Errorf("failed to decode order: %w", err)
	}
	
	log.Printf("Decoded order: %s", order.OrderID)
	return &order, nil
}

// TestEvent implements synckit.Event
type TestEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func (e *TestEvent) ID() string                       { return e.id }
func (e *TestEvent) Type() string                     { return e.eventType }
func (e *TestEvent) AggregateID() string              { return e.aggregateID }
func (e *TestEvent) Data() interface{}                { return e.data }
func (e *TestEvent) Metadata() map[string]interface{} { return e.metadata }

func main() {
	log.Println("=== Codec-Aware HTTP Transport Example ===")
	
	// 1. Set up codec registry
	registry := codec.NewRegistry()
	registry.Register(&UserProfileCodec{})
	registry.Register(&OrderCodec{})
	
	log.Printf("Registered codecs: %v", registry.Kinds())
	
	// 2. Create codec-aware encoder
	encoder := httptransport.NewCodecAwareEncoder(registry, true)
	
	// 3. Test events with different data types
	
	// Event with user profile data (uses custom codec)
	userEvent := &TestEvent{
		id:          "event-1",
		eventType:   "UserProfileUpdated", 
		aggregateID: "user-123",
		data: &UserProfile{
			ID:    "user-123",
			Name:  "John Doe",
			Email: "john@example.com",
			Settings: map[string]string{
				"theme":    "dark",
				"language": "en",
			},
		},
		metadata: map[string]interface{}{
			"kind": "user_profile", // This tells the encoder which codec to use
			"timestamp": time.Now().Unix(),
		},
	}
	
	// Event with order data (uses custom codec)
	orderEvent := &TestEvent{
		id:          "event-2",
		eventType:   "OrderCreated",
		aggregateID: "order-456",
		data: &OrderData{
			OrderID:   "order-456",
			ProductID: "product-789",
			Quantity:  2,
			Price:     99.99,
		},
		metadata: map[string]interface{}{
			"kind": "order", // This tells the encoder which codec to use
			"source": "web-app",
		},
	}
	
	// Event with plain JSON data (falls back to JSON encoding)
	plainEvent := &TestEvent{
		id:          "event-3",
		eventType:   "SimpleEvent",
		aggregateID: "misc-789",
		data:        map[string]interface{}{"message": "Hello, World!"},
		metadata:    map[string]interface{}{"source": "test"},
	}
	
	// 4. Demonstrate encoding
	log.Println("\n=== Encoding Events ===")
	
	events := []*TestEvent{userEvent, orderEvent, plainEvent}
	for _, event := range events {
		log.Printf("\nEncoding event: %s (type: %s)", event.ID(), event.Type())
		
		wireEvent, err := encoder.EncodeEvent(event)
		if err != nil {
			log.Printf("Error encoding event: %v", err)
			continue
		}
		
		log.Printf("Wire event DataKind: %s", wireEvent.DataKind)
		log.Printf("Wire event Data size: %d bytes", len(wireEvent.Data))
		
		// Show raw encoded data for demonstration
		if len(wireEvent.Data) < 200 { // Only show if not too large
			log.Printf("Raw data: %s", string(wireEvent.Data))
		}
	}
	
	// 5. Demonstrate round-trip encoding/decoding
	log.Println("\n=== Round-trip Test ===")
	
	for _, originalEvent := range events {
		log.Printf("\nTesting round-trip for event: %s", originalEvent.ID())
		
		// Encode
		wireEvent, err := encoder.EncodeEvent(originalEvent)
		if err != nil {
			log.Printf("Error encoding: %v", err)
			continue
		}
		
		// Decode
		decodedEvent, err := encoder.DecodeEvent(wireEvent)
		if err != nil {
			log.Printf("Error decoding: %v", err)
			continue
		}
		
		// Verify
		if decodedEvent.ID() != originalEvent.ID() {
			log.Printf("ERROR: ID mismatch - original: %s, decoded: %s", 
				originalEvent.ID(), decodedEvent.ID())
			continue
		}
		
		if decodedEvent.Type() != originalEvent.Type() {
			log.Printf("ERROR: Type mismatch - original: %s, decoded: %s", 
				originalEvent.Type(), decodedEvent.Type())
			continue
		}
		
		log.Printf("✓ Round-trip successful for event: %s", originalEvent.ID())
		
		// Compare data (this is simplified - in real usage you'd do deeper comparison)
		switch originalEvent.data.(type) {
		case *UserProfile:
			if profile, ok := decodedEvent.Data().(*UserProfile); ok {
				log.Printf("  User profile ID: %s -> %s", 
					originalEvent.data.(*UserProfile).ID, profile.ID)
			}
		case *OrderData:
			if order, ok := decodedEvent.Data().(*OrderData); ok {
				log.Printf("  Order ID: %s -> %s", 
					originalEvent.data.(*OrderData).OrderID, order.OrderID)
			}
		default:
			log.Printf("  Data preserved: %v", decodedEvent.Data())
		}
	}
	
	// 6. Demonstrate EventWithVersion encoding
	log.Println("\n=== EventWithVersion Test ===")
	
	eventWithVersion := synckit.EventWithVersion{
		Event:   userEvent,
		Version: cursor.IntegerCursor{Seq: 42},
	}
	
	wireEventWithVersion, err := encoder.EncodeEventWithVersion(eventWithVersion)
	if err != nil {
		log.Printf("Error encoding EventWithVersion: %v", err)
	} else {
		log.Printf("Encoded EventWithVersion - Version: %s, DataKind: %s", 
			wireEventWithVersion.Version, wireEventWithVersion.Event.DataKind)
		
		// Decode back
		parser := func(ctx context.Context, s string) (synckit.Version, error) {
			return cursor.IntegerCursor{Seq: 42}, nil // Simplified parser
		}
		
		decodedEv, err := encoder.DecodeEventWithVersion(context.Background(), parser, wireEventWithVersion)
		if err != nil {
			log.Printf("Error decoding EventWithVersion: %v", err)
		} else {
			log.Printf("✓ EventWithVersion round-trip successful - Version: %s", 
				decodedEv.Version.String())
		}
	}
	
	// 7. Show how to integrate with HTTP transport (conceptual)
	log.Println("\n=== HTTP Transport Integration ===")
	log.Println("To use codec-aware encoding with HTTP transport:")
	log.Println("1. Create ServerOptions with CodecAwareEncoder:")
	log.Println("   serverOpts := httptransport.DefaultServerOptions()")
	log.Println("   serverOpts.CodecAwareEncoder = encoder")
	log.Println("")
	log.Println("2. Create ClientOptions with CodecAwareEncoder:")
	log.Println("   clientOpts := httptransport.DefaultClientOptions()")
	log.Println("   clientOpts.CodecAwareEncoder = encoder") 
	log.Println("")
	log.Println("3. Both client and server will use codec-aware encoding for events")
	log.Println("   that have codec kinds in metadata, falling back to JSON for others.")
	
	log.Println("\n=== Example Complete ===")
}
