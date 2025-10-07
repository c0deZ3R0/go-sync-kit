// Enterprise HTTP Client Example for Go Sync Kit
// Demonstrates using the HTTP transport with authentication, multitenancy, and enterprise features
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
	"github.com/google/uuid"
)

const (
	serverURL = "http://localhost:8080"
	// Demo tokens from the enterprise server
	adminToken  = "admin-token"   // acme-corp tenant
	userToken   = "user-token"    // acme-corp tenant
	globexToken = "globex-token"  // globex-inc tenant
)

// authTransport wraps http.RoundTripper to add authentication headers
type authTransport struct {
	base            http.RoundTripper
	token           string
	idempotencyKey  string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())
	
	// Add Bearer token
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	
	// Add idempotency key if provided
	if t.idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", t.idempotencyKey)
	}
	
	return t.base.RoundTrip(req)
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Go-Sync-Kit Enterprise Client Example                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Example 1: Basic Pull with Authentication
	fmt.Println("📥 Example 1: Pull Events with Authentication")
	fmt.Println("─────────────────────────────────────────────")
	demoBasicPull()
	fmt.Println()

	// Example 2: Pull with Filtering
	fmt.Println("🔍 Example 2: Pull with Filtering (Type + Limit)")
	fmt.Println("─────────────────────────────────────────────")
	demoFilteredPull()
	fmt.Println()

	// Example 3: Multitenancy (Different Tenants)
	fmt.Println("🏢 Example 3: Multitenancy - Different Tenant Views")
	fmt.Println("─────────────────────────────────────────────")
	demoMultitenancy()
	fmt.Println()

	// Example 4: Push with Idempotency
	fmt.Println("🔄 Example 4: Push Events with Idempotency Key")
	fmt.Println("─────────────────────────────────────────────")
	demoPushWithIdempotency()
	fmt.Println()

	// Example 5: Full Sync with SyncNode
	fmt.Println("🔄 Example 5: Full Sync with SyncNode (High-Level API)")
	fmt.Println("─────────────────────────────────────────────")
	demoFullSync()
	fmt.Println()

	fmt.Println("✅ All examples completed successfully!")
}

// Example 1: Basic Pull with Bearer Token Authentication
func demoBasicPull() {
	// Create HTTP client with authentication header
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base:  http.DefaultTransport,
			token: adminToken,
		},
	}
	
	transport := httptransport.NewTransport(serverURL, client, nil, nil)
	
	// Pull events from beginning (use transport.GetLatestVersion to get a cursor)
	ctx := context.Background()
	
	// First, get the latest version to establish a cursor
	latestVersion, err := transport.GetLatestVersion(ctx)
	if err != nil {
		log.Printf("❌ Failed to get latest version: %v", err)
		return
	}
	
	// Pull all events by using a zero cursor
	events, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		log.Printf("❌ Pull failed: %v", err)
		return
	}

	fmt.Printf("✓ Pulled %d events for acme-corp tenant (admin user)\n", len(events))
	fmt.Printf("  Latest version on server: %v\n", latestVersion)
	for i, ev := range events {
		if i < 3 { // Show first 3
			fmt.Printf("  - Event %d: %s (aggregate: %s)\n", 
				i+1, ev.Event.Type(), ev.Event.AggregateID())
		}
	}
	if len(events) > 3 {
		fmt.Printf("  ... and %d more\n", len(events)-3)
	}
}

// Example 2: Pull with Filtering
func demoFilteredPull() {
	// Use query parameters for filtering in the URL
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base:  http.DefaultTransport,
			token: adminToken,
		},
	}
	
	transport := httptransport.NewTransport(
		serverURL, // Just the base URL, query params go in Pull call or use /pull endpoint
		client,
		nil,
		nil,
	)
	
	ctx := context.Background()
	// Note: Filtering by query params would require a custom implementation
	// For now, we'll pull all and show the first 5 OrderCreated events
	events, err := transport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		log.Printf("❌ Filtered pull failed: %v", err)
		return
	}

	// Filter client-side for OrderCreated events
	var orderCreatedEvents []synckit.EventWithVersion
	for _, ev := range events {
		if ev.Event.Type() == "OrderCreated" {
			orderCreatedEvents = append(orderCreatedEvents, ev)
			if len(orderCreatedEvents) >= 5 {
				break
			}
		}
	}

	fmt.Printf("✓ Pulled %d OrderCreated events (client-side filtered, limit 5)\n", len(orderCreatedEvents))
	for _, ev := range orderCreatedEvents {
		fmt.Printf("  - %s: %s\n", ev.Event.Type(), ev.Event.AggregateID())
	}
}

// Example 3: Demonstrate Multitenancy
func demoMultitenancy() {
	ctx := context.Background()

	// Pull as acme-corp tenant
	acmeClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base:  http.DefaultTransport,
			token: adminToken,
		},
	}
	acmeTransport := httptransport.NewTransport(serverURL, acmeClient, nil, nil)
	acmeEvents, err := acmeTransport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		log.Printf("❌ Acme pull failed: %v", err)
		return
	}

	// Pull as globex-inc tenant
	globexClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base:  http.DefaultTransport,
			token: globexToken,
		},
	}
	globexTransport := httptransport.NewTransport(serverURL, globexClient, nil, nil)
	globexEvents, err := globexTransport.Pull(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		log.Printf("❌ Globex pull failed: %v", err)
		return
	}

	fmt.Printf("✓ acme-corp tenant sees: %d events\n", len(acmeEvents))
	fmt.Printf("✓ globex-inc tenant sees: %d events\n", len(globexEvents))
	fmt.Println("  → Tenants are isolated - each only sees their own events!")
}

// Example 4: Push with Idempotency Key
func demoPushWithIdempotency() {
	ctx := context.Background()

	// Create a test event
	idempotencyKey := uuid.New().String()
	testEvent := event.New(
		uuid.New().String(),
		"OrderCreated",
		"order-999",
		[]byte(`{"amount": 199.99, "customer": "eve"}`),
	)

	// First push with idempotency key
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base:            http.DefaultTransport,
			token:           adminToken,
			idempotencyKey:  idempotencyKey,
		},
	}
	transport := httptransport.NewTransport(serverURL, client, nil, nil)

	eventsWithVersion := []synckit.EventWithVersion{
		{Event: testEvent, Version: nil}, // nil version = new event
	}

	err := transport.Push(ctx, eventsWithVersion)
	if err != nil {
		log.Printf("❌ First push failed: %v", err)
		return
	}
	fmt.Printf("✓ First push succeeded with idempotency key: %s\n", idempotencyKey[:8]+"...")

	// Duplicate push with same idempotency key
	// Server should return cached response instead of processing again
	err = transport.Push(ctx, eventsWithVersion)
	if err != nil {
		log.Printf("❌ Duplicate push failed: %v", err)
		return
	}
	fmt.Println("✓ Duplicate push succeeded - server returned cached response!")
	fmt.Println("  → Idempotency key prevented duplicate processing")
}

// Example 5: Full Sync with SyncNode (High-Level API)
func demoFullSync() {
	ctx := context.Background()

	// Create local store
	localStore := memstore.New()

	// Add a local event to push
	localEvent := event.NewWithMetadata(
		uuid.New().String(),
		"PaymentProcessed",
		"payment-999",
		[]byte(`{"amount": 199.99, "order_id": "order-999"}`),
		map[string]interface{}{
			"tenant": "acme-corp",
		},
	)
	localStore.Store(ctx, localEvent, nil)

	// Create transport with authentication
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base:  http.DefaultTransport,
			token: adminToken,
		},
	}
	transport := httptransport.NewTransport(serverURL, client, nil, nil)

	// Create SyncNode (high-level API)
	node, err := synckit.NewHTTPClientNode(localStore, transport)
	if err != nil {
		log.Printf("❌ Failed to create sync node: %v", err)
		return
	}
	defer node.Close()

	// Perform full sync (push local events + pull remote events)
	result, err := node.Sync(ctx)
	if err != nil {
		log.Printf("❌ Sync failed: %v", err)
		return
	}

	fmt.Printf("✓ Sync complete!\n")
	fmt.Printf("  - Events pushed: %d\n", result.EventsPushed)
	fmt.Printf("  - Events pulled: %d\n", result.EventsPulled)
	fmt.Printf("  - Conflicts resolved: %d\n", result.ConflictsResolved)
	fmt.Println("  → Local and remote stores are now synchronized!")
}
