// Example 6: Custom Events and Filters Demo
// 
// This example demonstrates:
// - Creating various custom event types for different domains
// - Implementing event filtering to sync only specific event types
// - Using metadata for advanced filtering strategies
// - Demonstrating conditional sync based on business logic
// - Performance optimization through selective synchronization

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// BaseEvent provides common fields for all events
type BaseEvent struct {
	EventID       string            `json:"id"`
	EventType     string            `json:"event_type"`
	AggregateID   string            `json:"aggregate_id"`
	Timestamp     time.Time         `json:"timestamp"`
	UserID        string            `json:"user_id"`
	TenantID      string            `json:"tenant_id"`
	EventMetadata map[string]string `json:"metadata"`
}

// UserEvent represents user management events
type UserEvent struct {
	BaseEvent
	Username     string `json:"username"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Action       string `json:"action"` // created, updated, deleted, login
	Department   string `json:"department"`
	IsActive     bool   `json:"is_active"`
}

func (e *UserEvent) ID() string                           { return e.EventID }
func (e *UserEvent) Type() string                         { return e.EventType }
func (e *UserEvent) AggregateID() string                  { return e.BaseEvent.AggregateID }
func (e *UserEvent) Data() interface{}                    { return e }
func (e *UserEvent) Metadata() map[string]interface{} {
	meta := make(map[string]interface{})
	meta["user_id"] = e.UserID
	meta["tenant_id"] = e.TenantID
	meta["timestamp"] = e.Timestamp
	meta["username"] = e.Username
	meta["role"] = e.Role
	meta["action"] = e.Action
	meta["department"] = e.Department
	meta["is_active"] = e.IsActive
	for k, v := range e.EventMetadata {
		meta[k] = v
	}
	return meta
}

// ProductEvent represents e-commerce product events
type ProductEvent struct {
	BaseEvent
	ProductName  string  `json:"product_name"`
	Category     string  `json:"category"`
	Price        float64 `json:"price"`
	SKU          string  `json:"sku"`
	Action       string  `json:"action"` // created, updated, price_changed, discontinued
	StockLevel   int     `json:"stock_level"`
	IsAvailable  bool    `json:"is_available"`
}

func (e *ProductEvent) ID() string                           { return e.EventID }
func (e *ProductEvent) Type() string                         { return e.EventType }
func (e *ProductEvent) AggregateID() string                  { return e.BaseEvent.AggregateID }
func (e *ProductEvent) Data() interface{}                    { return e }
func (e *ProductEvent) Metadata() map[string]interface{} {
	meta := make(map[string]interface{})
	meta["user_id"] = e.UserID
	meta["tenant_id"] = e.TenantID
	meta["timestamp"] = e.Timestamp
	meta["product_name"] = e.ProductName
	meta["category"] = e.Category
	meta["price"] = e.Price
	meta["sku"] = e.SKU
	meta["action"] = e.Action
	meta["stock_level"] = e.StockLevel
	meta["is_available"] = e.IsAvailable
	for k, v := range e.EventMetadata {
		meta[k] = v
	}
	return meta
}

// OrderEvent represents order processing events
type OrderEvent struct {
	BaseEvent
	OrderID      string  `json:"order_id"`
	CustomerID   string  `json:"customer_id"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"` // created, paid, shipped, delivered, cancelled
	PaymentType  string  `json:"payment_type"`
	ShippingType string  `json:"shipping_type"`
	ItemCount    int     `json:"item_count"`
	Priority     string  `json:"priority"` // low, normal, high, urgent
}

func (e *OrderEvent) ID() string                           { return e.EventID }
func (e *OrderEvent) Type() string                         { return e.EventType }
func (e *OrderEvent) AggregateID() string                  { return e.BaseEvent.AggregateID }
func (e *OrderEvent) Data() interface{}                    { return e }
func (e *OrderEvent) Metadata() map[string]interface{} {
	meta := make(map[string]interface{})
	meta["user_id"] = e.UserID
	meta["tenant_id"] = e.TenantID
	meta["timestamp"] = e.Timestamp
	meta["order_id"] = e.OrderID
	meta["customer_id"] = e.CustomerID
	meta["total_amount"] = e.TotalAmount
	meta["status"] = e.Status
	meta["payment_type"] = e.PaymentType
	meta["shipping_type"] = e.ShippingType
	meta["item_count"] = e.ItemCount
	meta["priority"] = e.Priority
	for k, v := range e.EventMetadata {
		meta[k] = v
	}
	return meta
}

// EventFilter definitions for different scenarios
type EventFilters struct{}

// OnlyUserEvents filters to synchronize only user-related events
func (f *EventFilters) OnlyUserEvents(event synckit.Event) bool {
	return strings.HasPrefix(event.Type(), "user.")
}

// OnlyProductEvents filters to synchronize only product-related events
func (f *EventFilters) OnlyProductEvents(event synckit.Event) bool {
	return strings.HasPrefix(event.Type(), "product.")
}

// OnlyOrderEvents filters to synchronize only order-related events
func (f *EventFilters) OnlyOrderEvents(event synckit.Event) bool {
	return strings.HasPrefix(event.Type(), "order.")
}

// HighPriorityEvents filters to sync only high-priority or urgent events
func (f *EventFilters) HighPriorityEvents(event synckit.Event) bool {
	metadata := event.Metadata()
	if priority, exists := metadata["priority"]; exists {
		priorityStr, ok := priority.(string)
		if !ok {
			return false
		}
		return priorityStr == "high" || priorityStr == "urgent"
	}
	
	// Also include user role changes and login events as high priority
	if strings.HasPrefix(event.Type(), "user.") {
		if action, exists := metadata["action"]; exists {
			actionStr, ok := action.(string)
			if !ok {
				return false
			}
			return actionStr == "login" || actionStr == "role_changed"
		}
	}
	
	return false
}

// RecentEventsOnly filters events from the last hour only
func (f *EventFilters) RecentEventsOnly(event synckit.Event) bool {
	metadata := event.Metadata()
	if timestamp, exists := metadata["timestamp"]; exists {
		if timestampVal, ok := timestamp.(time.Time); ok {
			return time.Since(timestampVal) < time.Hour
		}
	}
	return false
}

// TenantSpecificEvents creates a filter for a specific tenant
func (f *EventFilters) TenantSpecificEvents(tenantID string) func(synckit.Event) bool {
	return func(event synckit.Event) bool {
		metadata := event.Metadata()
		if tenant, exists := metadata["tenant_id"]; exists {
			tenantStr, ok := tenant.(string)
			if !ok {
				return false
			}
			return tenantStr == tenantID
		}
		return false
	}
}

// CombinedFilter allows combining multiple filter conditions
func (f *EventFilters) CombinedFilter(filters ...func(synckit.Event) bool) func(synckit.Event) bool {
	return func(event synckit.Event) bool {
		for _, filter := range filters {
			if !filter(event) {
				return false
			}
		}
		return true
	}
}

func main() {
	fmt.Println("=== Go Sync Kit Example 6: Custom Events and Filters Demo ===\n")

	// Create store for the demo
	fmt.Println("🏗️ Setting up event store...")
	
	store, err := sqlite.NewWithDataSource("filtered-events.db")
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	filters := &EventFilters{}

	// Create sample events of different types
	fmt.Println("📝 Creating sample events of different types...")
	
	events := []synckit.Event{
		// User events
		&UserEvent{
			BaseEvent:   BaseEvent{EventID: "user-1", EventType: "user.created", AggregateID: "user-001", Timestamp: time.Now().Add(-2*time.Hour), UserID: "admin", TenantID: "tenant-a"},
			Username:    "alice",
			Email:       "alice@company.com",
			Role:        "admin",
			Action:      "created",
			Department:  "engineering",
			IsActive:    true,
		},
		&UserEvent{
			BaseEvent:   BaseEvent{EventID: "user-2", EventType: "user.login", AggregateID: "user-001", Timestamp: time.Now().Add(-30*time.Minute), UserID: "alice", TenantID: "tenant-a"},
			Username:    "alice",
			Email:       "alice@company.com",
			Role:        "admin",
			Action:      "login",
			Department:  "engineering",
			IsActive:    true,
		},
		&UserEvent{
			BaseEvent:   BaseEvent{EventID: "user-3", EventType: "user.updated", AggregateID: "user-002", Timestamp: time.Now().Add(-1*time.Hour), UserID: "admin", TenantID: "tenant-b"},
			Username:    "bob",
			Email:       "bob@company.com",
			Role:        "user",
			Action:      "updated",
			Department:  "sales",
			IsActive:    true,
		},
		
		// Product events
		&ProductEvent{
			BaseEvent:    BaseEvent{EventID: "prod-1", EventType: "product.created", AggregateID: "prod-001", Timestamp: time.Now().Add(-3*time.Hour), UserID: "admin", TenantID: "tenant-a"},
			ProductName:  "Wireless Headphones",
			Category:     "electronics",
			Price:        99.99,
			SKU:          "WH-001",
			Action:       "created",
			StockLevel:   100,
			IsAvailable:  true,
		},
		&ProductEvent{
			BaseEvent:    BaseEvent{EventID: "prod-2", EventType: "product.price_changed", AggregateID: "prod-001", Timestamp: time.Now().Add(-20*time.Minute), UserID: "manager", TenantID: "tenant-a"},
			ProductName:  "Wireless Headphones",
			Category:     "electronics",
			Price:        89.99,
			SKU:          "WH-001",
			Action:       "price_changed",
			StockLevel:   95,
			IsAvailable:  true,
		},
		
		// Order events
		&OrderEvent{
			BaseEvent:    BaseEvent{EventID: "order-1", EventType: "order.created", AggregateID: "order-001", Timestamp: time.Now().Add(-45*time.Minute), UserID: "system", TenantID: "tenant-a", EventMetadata: map[string]string{"priority": "urgent"}},
			OrderID:      "ORD-001",
			CustomerID:   "cust-001",
			TotalAmount:  299.99,
			Status:       "created",
			PaymentType:  "credit_card",
			ShippingType: "express",
			ItemCount:    3,
			Priority:     "urgent",
		},
		&OrderEvent{
			BaseEvent:    BaseEvent{EventID: "order-2", EventType: "order.paid", AggregateID: "order-001", Timestamp: time.Now().Add(-15*time.Minute), UserID: "system", TenantID: "tenant-a", EventMetadata: map[string]string{"priority": "urgent"}},
			OrderID:      "ORD-001",
			CustomerID:   "cust-001",
			TotalAmount:  299.99,
			Status:       "paid",
			PaymentType:  "credit_card",
			ShippingType: "express",
			ItemCount:    3,
			Priority:     "urgent",
		},
		&OrderEvent{
			BaseEvent:    BaseEvent{EventID: "order-3", EventType: "order.created", AggregateID: "order-002", Timestamp: time.Now().Add(-2*time.Hour), UserID: "system", TenantID: "tenant-b", EventMetadata: map[string]string{"priority": "low"}},
			OrderID:      "ORD-002",
			CustomerID:   "cust-002",
			TotalAmount:  49.99,
			Status:       "created",
			PaymentType:  "debit_card",
			ShippingType: "standard",
			ItemCount:    1,
			Priority:     "low",
		},
	}

	// Store all events
	for i, event := range events {
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		err := store.Store(ctx, event, version)
		if err != nil {
			log.Printf("Failed to store event %s: %v", event.ID(), err)
			continue
		}
		fmt.Printf("  ✅ Stored %s: %s\n", event.Type(), event.ID())
	}

	fmt.Printf("\n📊 Total events stored: %d\n", len(events))

	// Demonstrate different filter scenarios
	filterScenarios := []struct {
		name        string
		description string
		filter      func(synckit.Event) bool
	}{
		{
			"User Events Only",
			"Synchronize only user management events",
			filters.OnlyUserEvents,
		},
		{
			"Product Events Only",
			"Synchronize only product catalog events",
			filters.OnlyProductEvents,
		},
		{
			"Order Events Only",
			"Synchronize only order processing events",
			filters.OnlyOrderEvents,
		},
		{
			"High Priority Only",
			"Synchronize only high-priority and urgent events",
			filters.HighPriorityEvents,
		},
		{
			"Recent Events Only",
			"Synchronize only events from the last hour",
			filters.RecentEventsOnly,
		},
		{
			"Tenant A Only",
			"Synchronize only events from tenant-a",
			filters.TenantSpecificEvents("tenant-a"),
		},
		{
			"Combined Filter",
			"Recent high-priority events from tenant-a",
			filters.CombinedFilter(
				filters.RecentEventsOnly,
				filters.HighPriorityEvents,
				filters.TenantSpecificEvents("tenant-a"),
			),
		},
	}

	for i, scenario := range filterScenarios {
		fmt.Printf("\n%s\n", strings.Repeat("=", 60))
		fmt.Printf("🔍 Filter Scenario %d: %s\n", i+1, scenario.name)
		fmt.Printf("📝 %s\n", scenario.description)
		fmt.Printf("%s\n", strings.Repeat("=", 60))

		// Create a new store instance for this scenario to avoid conflicts
		scenarioStore, err := sqlite.NewWithDataSource("filtered-events.db")
		if err != nil {
			log.Printf("Failed to create store for scenario %s: %v", scenario.name, err)
			continue
		}
		defer scenarioStore.Close()
		
		// Create a sync manager with the current filter
		manager, err := synckit.NewManager(
			synckit.WithStore(scenarioStore),
			synckit.WithNullTransport(),
			synckit.WithFilter(scenario.filter),
			synckit.WithLWW(),
		)
		if err != nil {
			log.Printf("Failed to create manager for scenario %s: %v", scenario.name, err)
			continue
		}

		// Perform a sync operation (which will apply the filter)
		result, err := manager.Sync(ctx)
		if err != nil {
			fmt.Printf("❌ Sync failed: %v\n", err)
		} else {
			fmt.Printf("✅ Sync completed: %d events processed\n", result.EventsPushed+result.EventsPulled)
		}

		// Load and display filtered events
		zeroVersion := cursor.IntegerCursor{Seq: 0}
		allEvents, err := store.Load(ctx, zeroVersion)
		if err != nil {
			fmt.Printf("❌ Failed to load events: %v\n", err)
			continue
		}

		filteredCount := 0
		fmt.Println("\n📋 Events that would be synchronized:")
		for _, eventWithVersion := range allEvents {
			if scenario.filter(eventWithVersion.Event) {
				filteredCount++
				metadata := eventWithVersion.Event.Metadata()
				
				switch e := eventWithVersion.Event.Data().(type) {
				case *UserEvent:
					fmt.Printf("  👤 %s: %s (%s) - %s [%s]\n", 
						eventWithVersion.Event.Type(), e.Username, e.Role, e.Action, e.TenantID)
				case *ProductEvent:
					fmt.Printf("  📦 %s: %s ($%.2f) - %s [%s]\n", 
						eventWithVersion.Event.Type(), e.ProductName, e.Price, e.Action, e.TenantID)
				case *OrderEvent:
					fmt.Printf("  🛒 %s: %s ($%.2f) - %s (%s priority) [%s]\n", 
						eventWithVersion.Event.Type(), e.OrderID, e.TotalAmount, e.Status, e.Priority, e.TenantID)
				default:
					fmt.Printf("  🔍 %s: %s [%v]\n", 
						eventWithVersion.Event.Type(), eventWithVersion.Event.ID(), metadata["tenant_id"])
				}
			}
		}
		
		fmt.Printf("\n📊 Filter Results: %d out of %d events match the filter (%.1f%%)\n",
			filteredCount, len(allEvents), float64(filteredCount)/float64(len(allEvents))*100)

		// Close manager
		manager.Close()

		time.Sleep(100 * time.Millisecond) // Brief pause for readability
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Println("🎉 Custom Events and Filters Demo Complete!")
	fmt.Println("\n💡 Key Takeaways:")
	fmt.Println("   • Custom events can model any domain-specific data")
	fmt.Println("   • Filters enable selective synchronization for performance")
	fmt.Println("   • Metadata-based filtering provides flexible business logic")
	fmt.Println("   • Combined filters allow complex synchronization rules")
	fmt.Println("   • Tenant-based filtering supports multi-tenant architectures")
	fmt.Println("   • Priority-based filtering optimizes critical data sync")
	fmt.Println("   • Time-based filtering reduces bandwidth and storage")
}
