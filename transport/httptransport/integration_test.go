package httptransport

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/event"
	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport/middleware"
	"github.com/google/uuid"
)

// TestEndToEndPullWithFiltering tests complete pull flow with various filters
func TestEndToEndPullWithFiltering(t *testing.T) {
	t.Parallel() // Safe to run in parallel as each test creates its own server
	store := memstore.New()
	ctx := context.Background()

	// Setup: Store events with different types and tenants
	events := []struct {
		eventType   string
		aggregateID string
		tenant      string
	}{
		{"OrderCreated", "order-1", "acme"},
		{"OrderCreated", "order-2", "acme"},
		{"OrderUpdated", "order-1", "acme"},
		{"OrderCreated", "order-3", "globex"},
		{"UserCreated", "user-1", "acme"},
	}

	for i, ev := range events {
		data, _ := json.Marshal(map[string]interface{}{"index": i})
		metadata := map[string]interface{}{"tenant": ev.tenant}
		e := event.NewWithMetadata(
			uuid.New().String(),
			ev.eventType,
			ev.aggregateID,
			data,
			metadata,
		)
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		if err := store.Store(ctx, e, version); err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	handler := NewSyncHandler(store, nil, nil, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name           string
		queryParams    string
		expectedCount  int
		expectedTypes  []string
		expectedTenant string
	}{
		{
			name:          "no filters - all events",
			queryParams:   "",
			expectedCount: 5,
		},
		{
			name:          "filter by type",
			queryParams:   "?type=OrderCreated",
			expectedCount: 3,
			expectedTypes: []string{"OrderCreated", "OrderCreated", "OrderCreated"},
		},
		{
			name:           "filter by tenant",
			queryParams:    "?tenant=acme",
			expectedCount:  4,
			expectedTenant: "acme",
		},
		{
			name:          "filter by type and tenant",
			queryParams:   "?type=OrderCreated&tenant=acme",
			expectedCount: 2,
		},
		{
			name:          "filter by aggregate_id",
			queryParams:   "?aggregate_id=order-1",
			expectedCount: 2,
		},
		{
			name:          "limit results",
			queryParams:   "?limit=2",
			expectedCount: 2,
		},
		{
			name:          "filter with since cursor",
			queryParams:   "?since=2&type=OrderCreated",
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't use t.Parallel() here as all subtests share the same server
			url := server.URL + "/pull" + tt.queryParams
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("GET request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			var events []JSONEventWithVersion
			if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(events) != tt.expectedCount {
				t.Errorf("expected %d events, got %d", tt.expectedCount, len(events))
			}

			// Verify types if specified
			if len(tt.expectedTypes) > 0 {
				for i, ev := range events {
					if i < len(tt.expectedTypes) && ev.Event.Type != tt.expectedTypes[i] {
						t.Errorf("event %d: expected type %s, got %s", i, tt.expectedTypes[i], ev.Event.Type)
					}
				}
			}

			// Verify tenant if specified
			if tt.expectedTenant != "" {
				for i, ev := range events {
					if tenant, ok := ev.Event.Metadata["tenant"]; !ok || tenant != tt.expectedTenant {
						t.Errorf("event %d: expected tenant %s, got %v", i, tt.expectedTenant, tenant)
					}
				}
			}
		})
	}
}

// TestMultitenancyIsolation ensures tenant isolation works correctly
func TestMultitenancyIsolation(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()
	ctx := context.Background()

	// Store events for different tenants
	tenants := []string{"acme", "globex", "initech"}
	for _, tenant := range tenants {
		for i := 0; i < 3; i++ {
			data, _ := json.Marshal(map[string]interface{}{"amount": i * 100})
			metadata := map[string]interface{}{"tenant": tenant}
			e := event.NewWithMetadata(
				uuid.New().String(),
				"OrderCreated",
				fmt.Sprintf("order-%d", i),
				data,
				metadata,
			)
			version := cursor.IntegerCursor{Seq: uint64(len(tenants)*i + 1)}
			if err := store.Store(ctx, e, version); err != nil {
				t.Fatalf("Failed to store event: %v", err)
			}
		}
	}

	handler := NewSyncHandler(store, nil, nil, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test: Each tenant should only see their own events
	for _, tenant := range tenants {
		t.Run(fmt.Sprintf("tenant_%s", tenant), func(t *testing.T) {
			req, _ := http.NewRequest("GET", server.URL+"/pull", nil)
			req.Header.Set("X-SyncKit-Tenant", tenant)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			var events []JSONEventWithVersion
			if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Should get 3 events for this tenant
			if len(events) != 3 {
				t.Errorf("expected 3 events for tenant %s, got %d", tenant, len(events))
			}

			// Verify all events belong to this tenant
			for i, ev := range events {
				if eventTenant, ok := ev.Event.Metadata["tenant"]; !ok || eventTenant != tenant {
					t.Errorf("event %d: expected tenant %s, got %v", i, tenant, eventTenant)
				}
			}
		})
	}
}

// TestIdempotencyKeyHandling tests idempotency across multiple requests
func TestIdempotencyKeyHandling(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()
	handler := NewSyncHandler(store, nil, nil, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create test event
	testEvent := JSONEventWithVersion{
		Event: JSONEvent{
			ID:          uuid.New().String(),
			Type:        "TestEvent",
			AggregateID: "test-1",
			Data:        map[string]interface{}{"value": 123},
			Metadata:    map[string]interface{}{},
		},
		Version: "1",
	}

	body, _ := json.Marshal([]JSONEventWithVersion{testEvent})
	idempotencyKey := uuid.New().String()

	// First request: Should process
	req1, _ := http.NewRequest("POST", server.URL+"/push", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", resp1.StatusCode)
	}

	body1, _ := io.ReadAll(resp1.Body)

	// Second request with same idempotency key: Should return cached response
	req2, _ := http.NewRequest("POST", server.URL+"/push", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Second request: expected status 200, got %d", resp2.StatusCode)
	}

	body2, _ := io.ReadAll(resp2.Body)

	// Responses should be identical
	if !bytes.Equal(body1, body2) {
		t.Error("Idempotent requests returned different responses")
	}

	// Verify event was only stored once
	ctx := context.Background()
	events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if err != nil {
		t.Fatalf("Failed to load events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event in store, got %d", len(events))
	}

	// Third request with different key: Should process as new
	req3, _ := http.NewRequest("POST", server.URL+"/push", bytes.NewBuffer(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Idempotency-Key", uuid.New().String())

	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("Third request failed: %v", err)
	}
	defer resp3.Body.Close()

	// Should now have 2 events (duplicate with different key)
	events, _ = store.Load(ctx, cursor.IntegerCursor{Seq: 0})
	if len(events) != 2 {
		t.Errorf("Expected 2 events after third request, got %d", len(events))
	}
}

// TestMiddlewareChainAuthentication tests complete middleware chain
func TestMiddlewareChainAuthentication(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()

	// Token validator
	validator := func(token string) (userID, tenantID string, err error) {
		switch token {
		case "admin-token":
			return "admin", "acme", nil
		case "user-token":
			return "user1", "acme", nil
		case "globex-token":
			return "user2", "globex", nil
		default:
			return "", "", fmt.Errorf("invalid token")
		}
	}

	baseHandler := NewSyncHandler(store, nil, nil, nil)
	handler := middleware.Chain(
		baseHandler,
		middleware.BearerAuth(validator),
		middleware.TenantExtractor("X-SyncKit-Tenant"),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		expectAuth     bool
	}{
		{
			name:           "valid admin token",
			token:          "admin-token",
			expectedStatus: http.StatusOK,
			expectAuth:     true,
		},
		{
			name:           "valid user token",
			token:          "user-token",
			expectedStatus: http.StatusOK,
			expectAuth:     true,
		},
		{
			name:           "invalid token",
			token:          "bad-token",
			expectedStatus: http.StatusUnauthorized,
			expectAuth:     false,
		},
		{
			name:           "missing token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", server.URL+"/pull", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, body)
			}
		})
	}
}

// TestHMACSignatureValidation tests HMAC authentication
func TestHMACSignatureValidation(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()
	secret := []byte("test-secret-key")

	baseHandler := NewSyncHandler(store, nil, nil, nil)
	handler := middleware.Chain(
		baseHandler,
		middleware.HMACValidator(secret, "X-SyncKit-Signature"),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Create test event
	testEvent := JSONEventWithVersion{
		Event: JSONEvent{
			ID:          uuid.New().String(),
			Type:        "TestEvent",
			AggregateID: "test-1",
			Data:        map[string]interface{}{"value": 456},
			Metadata:    map[string]interface{}{},
		},
		Version: "1",
	}

	body, _ := json.Marshal([]JSONEventWithVersion{testEvent})

	// Compute valid HMAC
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name           string
		signature      string
		body           []byte
		expectedStatus int
	}{
		{
			name:           "valid signature",
			signature:      validSignature,
			body:           body,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid signature",
			signature:      "deadbeef1234567890abcdef",
			body:           body,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing signature",
			signature:      "",
			body:           body,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "signature for different body",
			signature:      validSignature,
			body:           []byte(`{"different":"data"}`),
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", server.URL+"/push", bytes.NewBuffer(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set("X-SyncKit-Signature", tt.signature)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, body)
			}
		})
	}
}

// TestStructuredErrorResponses tests error response format
func TestStructuredErrorResponses(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()
	handler := NewSyncHandler(store, nil, nil, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
		errorCode    string
	}{
		{
			name:         "invalid cursor",
			method:       "GET",
			path:         "/pull?since=invalid",
			expectedCode: http.StatusBadRequest,
			errorCode:    "INVALID_CURSOR",
		},
		{
			name:         "invalid limit",
			method:       "GET",
			path:         "/pull?limit=-1",
			expectedCode: http.StatusBadRequest,
			errorCode:    "INVALID_CURSOR", // Query parsing error returns INVALID_CURSOR
		},
		{
			name:         "limit too large",
			method:       "GET",
			path:         "/pull?limit=2000",
			expectedCode: http.StatusBadRequest,
			errorCode:    "INVALID_CURSOR", // Query parsing error returns INVALID_CURSOR
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req, _ = http.NewRequest(tt.method, server.URL+tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(tt.method, server.URL+tt.path, nil)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			// Verify structured error response
			var errorResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
					Op      string `json:"op"`
				} `json:"error"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if errorResp.Error.Code != tt.errorCode {
				t.Errorf("expected error code %s, got %s", tt.errorCode, errorResp.Error.Code)
			}

			if errorResp.Error.Message == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

// TestConcurrentRequests tests handling of concurrent requests
func TestConcurrentRequests(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()
	handler := NewSyncHandler(store, nil, nil, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Setup: Store some events
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		data, _ := json.Marshal(map[string]interface{}{"index": i})
		e := event.New(
			uuid.New().String(),
			"TestEvent",
			fmt.Sprintf("agg-%d", i),
			data,
		)
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		if err := store.Store(ctx, e, version); err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	// Test: Concurrent pull requests
	const concurrency = 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			resp, err := http.Get(server.URL + "/pull")
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("expected status 200, got %d", resp.StatusCode)
				return
			}

			var events []JSONEventWithVersion
			if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
				errors <- fmt.Errorf("decode error: %w", err)
				return
			}

			if len(events) != 10 {
				errors <- fmt.Errorf("expected 10 events, got %d", len(events))
				return
			}

			done <- true
		}()
	}

	// Wait for all goroutines
	timeout := time.After(5 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Errorf("Concurrent request error: %v", err)
		case <-timeout:
			t.Fatal("Test timeout")
		}
	}
}

// TestBackwardCompatibility_v023_Client tests that old clients work with new servers
func TestBackwardCompatibility_v023_Client(t *testing.T) {
	t.Parallel() // Safe to run in parallel
	store := memstore.New()
	ctx := context.Background()

	// Store some events
	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(map[string]interface{}{"index": i})
		e := event.New(
			uuid.New().String(),
			"TestEvent",
			fmt.Sprintf("agg-%d", i),
			data,
		)
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		if err := store.Store(ctx, e, version); err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	handler := NewSyncHandler(store, nil, nil, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("old_pull_without_params", func(t *testing.T) {
		// v0.23 client: No query params
		resp, err := http.Get(server.URL + "/pull")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var events []JSONEventWithVersion
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should get all events (default behavior)
		if len(events) != 5 {
			t.Errorf("expected 5 events, got %d", len(events))
		}
	})

	t.Run("old_push_without_idempotency", func(t *testing.T) {
		// v0.23 client: No idempotency key
		testEvent := JSONEventWithVersion{
			Event: JSONEvent{
				ID:          uuid.New().String(),
				Type:        "OldClientEvent",
				AggregateID: "test-old",
				Data:        map[string]interface{}{"value": 999},
				Metadata:    map[string]interface{}{},
			},
			Version: "6",
		}

		body, _ := json.Marshal([]JSONEventWithVersion{testEvent})
		resp, err := http.Post(server.URL+"/push", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 200, got %d. Body: %s", resp.StatusCode, body)
		}
	})

	t.Run("old_pull_with_since_only", func(t *testing.T) {
		// v0.23 client: Only 'since' parameter
		resp, err := http.Get(server.URL + "/pull?since=3")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var events []JSONEventWithVersion
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should get events after sequence 3 (events 4, 5, 6)
		if len(events) < 2 {
			t.Errorf("expected at least 2 events after cursor 3, got %d", len(events))
		}
	})
}

// TestMiddlewareCombinations tests various middleware stacks
func TestMiddlewareCombinations(t *testing.T) {
	t.Parallel() // Safe to run in parallel - each subtest creates its own server
	store := memstore.New()

	tests := []struct {
		name           string
		middleware     []middleware.Middleware
		setupRequest   func(*http.Request)
		expectedStatus int
	}{
		{
			name: "bearer_only",
			middleware: []middleware.Middleware{
				middleware.BearerAuth(func(token string) (string, string, error) {
					if token == "valid" {
						return "user1", "tenant1", nil
					}
					return "", "", fmt.Errorf("invalid")
				}),
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "bearer_plus_tenant",
			middleware: []middleware.Middleware{
				middleware.TenantExtractor("X-SyncKit-Tenant"),
				middleware.BearerAuth(func(token string) (string, string, error) {
					if token == "valid" {
						return "user1", "from-token", nil
					}
					return "", "", fmt.Errorf("invalid")
				}),
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid")
				req.Header.Set("X-SyncKit-Tenant", "from-header") // Should be overridden by token
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "hmac_only",
			middleware: []middleware.Middleware{
				middleware.HMACValidator([]byte("secret"), "X-SyncKit-Signature"),
			},
			setupRequest: func(req *http.Request) {
				body := []byte(`[]`)
				mac := hmac.New(sha256.New, []byte("secret"))
				mac.Write(body)
				req.Header.Set("X-SyncKit-Signature", hex.EncodeToString(mac.Sum(nil)))
				req.Body = io.NopCloser(bytes.NewBuffer(body))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "bearer_plus_hmac",
			middleware: []middleware.Middleware{
				middleware.HMACValidator([]byte("secret"), "X-SyncKit-Signature"),
				middleware.BearerAuth(func(token string) (string, string, error) {
					if token == "valid" {
						return "user1", "tenant1", nil
					}
					return "", "", fmt.Errorf("invalid")
				}),
			},
			setupRequest: func(req *http.Request) {
				body := []byte(`[]`)
				mac := hmac.New(sha256.New, []byte("secret"))
				mac.Write(body)
				req.Header.Set("X-SyncKit-Signature", hex.EncodeToString(mac.Sum(nil)))
				req.Header.Set("Authorization", "Bearer valid")
				req.Body = io.NopCloser(bytes.NewBuffer(body))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "all_middleware",
			middleware: []middleware.Middleware{
				middleware.TenantExtractor("X-SyncKit-Tenant"),
				middleware.HMACValidator([]byte("secret"), "X-SyncKit-Signature"),
				middleware.BearerAuth(func(token string) (string, string, error) {
					if token == "valid" {
						return "user1", "tenant1", nil
					}
					return "", "", fmt.Errorf("invalid")
				}),
			},
			setupRequest: func(req *http.Request) {
				body := []byte(`[]`)
				mac := hmac.New(sha256.New, []byte("secret"))
				mac.Write(body)
				req.Header.Set("X-SyncKit-Signature", hex.EncodeToString(mac.Sum(nil)))
				req.Header.Set("Authorization", "Bearer valid")
				req.Header.Set("X-SyncKit-Tenant", "header-tenant")
				req.Body = io.NopCloser(bytes.NewBuffer(body))
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseHandler := NewSyncHandler(store, nil, nil, nil)
			handler := middleware.Chain(baseHandler, tt.middleware...)
			server := httptest.NewServer(handler)
			defer server.Close()

			req, _ := http.NewRequest("POST", server.URL+"/push", bytes.NewBuffer([]byte(`[]`)))
			req.Header.Set("Content-Type", "application/json")
			tt.setupRequest(req)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, body)
			}
		})
	}
}
