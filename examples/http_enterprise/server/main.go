package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport/middleware"
	"github.com/google/uuid"
)

const (
	serverAddr = ":8080"
	hmacSecret = "your-hmac-secret-key-change-in-production"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting go-sync-kit enterprise HTTP server",
		slog.String("addr", serverAddr),
		slog.String("version", "v0.24.0"))

	// Create event store (in production, use SQLite or Postgres)
	store := memstore.New()

	// Seed some initial events for demonstration
	seedDemoEvents(store)

	// Configure server options
	serverOpts := &httptransport.ServerOptions{
		MaxRequestSize:       10 * 1024 * 1024,  // 10MB
		MaxDecompressedSize:  20 * 1024 * 1024,  // 20MB
		CompressionEnabled:   true,
		CompressionThreshold: 1024, // Compress responses > 1KB
		RequestTimeout:       30 * time.Second,
		ShutdownTimeout:      10 * time.Second,
	}

	// Setup hooks for observability
	hooks := &httptransport.SyncHooks{
		BeforePull: func(ctx context.Context, since synckit.Version) {
			userID, _ := middleware.UserIDFromContext(ctx)
			tenant, _ := middleware.TenantFromContext(ctx)
			logger.Info("Pull request started",
				slog.String("user_id", userID),
				slog.String("tenant", tenant),
				slog.Any("since", since))
		},
		AfterCommit: func(ctx context.Context, events []synckit.EventWithVersion) {
			userID, _ := middleware.UserIDFromContext(ctx)
			tenant, _ := middleware.TenantFromContext(ctx)
			logger.Info("Events committed",
				slog.String("user_id", userID),
				slog.String("tenant", tenant),
				slog.Int("count", len(events)))
		},
	}

	// Create base sync handler
	baseHandler := httptransport.NewSyncHandlerWithHooks(
		store,
		logger,
		nil, // Use default version parser
		serverOpts,
		hooks,
	)

	// Setup authentication middleware
	// In production, validate against your user database
	authValidator := func(token string) (userID, tenantID string, err error) {
		// Demo token validation
		switch token {
		case "admin-token":
			return "admin-user", "acme-corp", nil
		case "user-token":
			return "regular-user", "acme-corp", nil
		case "globex-token":
			return "globex-user", "globex-inc", nil
		default:
			return "", "", fmt.Errorf("invalid authentication token")
		}
	}

	// Build middleware chain
	// Note: HMAC validator is commented out for simpler demos
	// Uncomment if you want to require HMAC signatures
	handler := middleware.Chain(
		baseHandler,
		middleware.TenantExtractor("X-Tenant-ID"),  // Extract tenant from header
		middleware.BearerAuth(authValidator),        // Require Bearer token
		// middleware.HMACValidator([]byte(hmacSecret), "X-HMAC-Signature"), // Optional HMAC
	)

	// Setup HTTP server
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening",
			slog.String("addr", serverAddr),
			slog.String("endpoints", "/push, /pull, /latest-version"))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Print usage instructions
	printUsageInstructions()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), serverOpts.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
	}

	logger.Info("Server stopped")
}

func seedDemoEvents(store *memstore.MemStore) {
	ctx := context.Background()

	// Seed events for acme-corp tenant
	acmeEvents := []struct {
		eventType   string
		aggregateID string
		data        map[string]interface{}
	}{
		{"OrderCreated", "order-1", map[string]interface{}{"amount": 99.99, "customer": "alice"}},
		{"OrderCreated", "order-2", map[string]interface{}{"amount": 149.99, "customer": "bob"}},
		{"OrderUpdated", "order-1", map[string]interface{}{"status": "shipped"}},
		{"PaymentProcessed", "payment-1", map[string]interface{}{"amount": 99.99, "order_id": "order-1"}},
	}

	for i, ev := range acmeEvents {
		data, _ := json.Marshal(ev.data)
		metadata := map[string]interface{}{
			"tenant":    "acme-corp",
			"timestamp": time.Now().Add(-time.Hour * time.Duration(4-i)).Format(time.RFC3339),
		}

		e := event.NewWithMetadata(
			uuid.New().String(),
			ev.eventType,
			ev.aggregateID,
			data,
			metadata,
		)

		store.Store(ctx, e, nil) // memstore auto-assigns versions
	}

	// Seed events for globex-inc tenant
	globexEvents := []struct {
		eventType   string
		aggregateID string
		data        map[string]interface{}
	}{
		{"OrderCreated", "order-100", map[string]interface{}{"amount": 299.99, "customer": "charlie"}},
		{"OrderCreated", "order-101", map[string]interface{}{"amount": 199.99, "customer": "diana"}},
	}

	for i, ev := range globexEvents {
		data, _ := json.Marshal(ev.data)
		metadata := map[string]interface{}{
			"tenant":    "globex-inc",
			"timestamp": time.Now().Add(-time.Hour * time.Duration(2-i)).Format(time.RFC3339),
		}

		e := event.NewWithMetadata(
			uuid.New().String(),
			ev.eventType,
			ev.aggregateID,
			data,
			metadata,
		)

		store.Store(ctx, e, nil)
	}

	log.Printf("✓ Seeded %d demo events (4 acme-corp, 2 globex-inc)\n", len(acmeEvents)+len(globexEvents))
}

func printUsageInstructions() {
	fmt.Print(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                   Go-Sync-Kit Enterprise Server Running                      ║
╚══════════════════════════════════════════════════════════════════════════════╝

Server: http://localhost:8080

📋 AVAILABLE ENDPOINTS:
  • POST /push             - Push events to server
  • GET  /pull             - Pull events from server
  • GET  /latest-version   - Get latest version/cursor

🔑 AUTHENTICATION:
  Required: Bearer token in Authorization header
  
  Demo Tokens:
  • "admin-token"   → acme-corp tenant (admin-user)
  • "user-token"    → acme-corp tenant (regular-user)
  • "globex-token"  → globex-inc tenant (globex-user)

🌐 EXAMPLE REQUESTS:

1️⃣  Pull All Events (with auth):
   curl -H "Authorization: Bearer admin-token" \
        http://localhost:8080/pull

2️⃣  Pull with Filtering (tenant + type):
   curl -H "Authorization: Bearer admin-token" \
        "http://localhost:8080/pull?type=OrderCreated&limit=10"

3️⃣  Pull for Different Tenant:
   curl -H "Authorization: Bearer globex-token" \
        http://localhost:8080/pull

4️⃣  Push Events (with idempotency):
   curl -X POST \
        -H "Authorization: Bearer admin-token" \
        -H "Content-Type: application/json" \
        -H "Idempotency-Key: $(uuidgen)" \
        -d '[{"event":{"id":"evt-1","type":"TestEvent","aggregate_id":"test","data":{"test":true}},"version":"1"}]' \
        http://localhost:8080/push

5️⃣  Pull with HMAC Signature (additional security):
   # Calculate HMAC-SHA256 of request body with secret: your-hmac-secret-key-change-in-production
   # Add as X-HMAC-Signature header

🏢 MULTITENANCY:
  • Events are automatically filtered by tenant (extracted from auth token)
  • acme-corp tenant has 4 events
  • globex-inc tenant has 2 events
  • Tenants are isolated - each only sees their own events

🔄 IDEMPOTENCY:
  • Include "Idempotency-Key" header in POST /push requests
  • Duplicate requests with same key return cached response
  • Keys expire after 10 minutes

📊 FEATURES DEMONSTRATED:
  ✓ Bearer token authentication
  ✓ Multitenancy with tenant isolation
  ✓ Event filtering (type, aggregate_id, limit)
  ✓ Idempotency key support
  ✓ HMAC signature validation (optional)
  ✓ Structured error responses
  ✓ Request/response compression (>1KB)
  ✓ Size limits (10MB request, 20MB decompressed)

Press Ctrl+C to shutdown server gracefully...
`)
}
