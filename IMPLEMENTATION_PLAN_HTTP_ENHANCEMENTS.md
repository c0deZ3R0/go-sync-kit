# Implementation Plan: HTTP Enterprise Enhancements

**Branch:** `feature/http-enterprise-enhancements`  
**Base:** `dev`  
**Target Version:** v0.24.0 (all changes backward compatible)  
**Estimated Effort:** 3-5 days

---

## 🎯 Objectives

Transform the HTTP transport from a solid foundation into a production-ready, enterprise-grade API by adding:

1. Structured error responses with error codes
2. Rich filtering and pagination (type, tenant, limit)
3. Multitenancy support via headers
4. Idempotency key handling
5. Authentication/authorization middleware
6. Formal HTTP API specification document

**All changes are backward compatible** - existing code continues to work.

---

## 📋 Implementation Phases

### **Phase 1: Core Infrastructure** (Day 1)

Foundation for all other features.

#### 1.1 Structured Error Responses
**File:** `transport/httptransport/errors.go` (NEW)

```go
package httptransport

import (
	"fmt"
	"net/http"
)

// ErrorResponse wraps structured error information for HTTP responses
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail provides structured error information
type ErrorDetail struct {
	Code    string `json:"code"`    // e.g., INVALID_CURSOR, AUTH_REQUIRED
	Message string `json:"message"` // Human-readable message
	Op      string `json:"op"`      // Operation: push, pull, subscribe
}

// Error codes
const (
	ErrCodeInvalidCursor     = "INVALID_CURSOR"
	ErrCodeInvalidRequest    = "INVALID_REQUEST"
	ErrCodeAuthRequired      = "AUTH_REQUIRED"
	ErrCodeInvalidTenant     = "INVALID_TENANT"
	ErrCodeInvalidIdempotency = "INVALID_IDEMPOTENCY_KEY"
	ErrCodeConflict          = "CONFLICT"
	ErrCodeInternal          = "INTERNAL_ERROR"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeTooLarge          = "REQUEST_TOO_LARGE"
)

// NewErrorResponse creates a structured error response
func NewErrorResponse(op, code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Op:      op,
		},
	}
}

// HTTPStatusFromCode maps error codes to HTTP status codes
func HTTPStatusFromCode(code string) int {
	switch code {
	case ErrCodeInvalidCursor, ErrCodeInvalidRequest, ErrCodeInvalidTenant, ErrCodeInvalidIdempotency:
		return http.StatusBadRequest
	case ErrCodeAuthRequired:
		return http.StatusUnauthorized
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeTooLarge:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

// respondWithStructuredError sends a structured error response
func respondWithStructuredError(w http.ResponseWriter, r *http.Request, op, code, message string, opts *ServerOptions) {
	resp := NewErrorResponse(op, code, message)
	status := HTTPStatusFromCode(code)
	respondWithJSON(w, r, status, resp, opts)
}
```

**Tests:** `transport/httptransport/errors_test.go` (NEW)
- Test error code mapping
- Test HTTP status mapping
- Test JSON serialization

**Affected files to update:**
- `http.go` - Replace `respondErr` calls with `respondWithStructuredError`
- `helpers.go` - Update helper functions

---

#### 1.2 Query Parameter Parsing
**File:** `transport/httptransport/query.go` (NEW)

```go
package httptransport

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// PullQuery represents parsed query parameters for pull requests
type PullQuery struct {
	Since   types.Version   // Cursor position
	Limit   int             // Max events to return (default 100, max 1000)
	Filters []types.Filter  // Event filters (type, tenant, etc.)
}

// ParsePullQuery extracts and validates query parameters from an HTTP request
func ParsePullQuery(ctx context.Context, r *http.Request, parser VersionParser) (*PullQuery, error) {
	query := &PullQuery{
		Limit:   100, // Default limit
		Filters: make([]types.Filter, 0),
	}

	// Parse 'since' cursor
	if since := r.URL.Query().Get("since"); since != "" {
		version, err := parser(ctx, since)
		if err != nil {
			return nil, fmt.Errorf("invalid since cursor: %w", err)
		}
		query.Since = version
	}

	// Parse 'limit'
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid limit: must be integer")
		}
		if limit <= 0 {
			return nil, fmt.Errorf("invalid limit: must be positive")
		}
		if limit > 1000 {
			return nil, fmt.Errorf("invalid limit: maximum is 1000")
		}
		query.Limit = limit
	}

	// Parse 'type' filter
	if eventType := r.URL.Query().Get("type"); eventType != "" {
		query.Filters = append(query.Filters, types.Filter{
			Key:   "type",
			Value: eventType,
		})
	}

	// Parse 'tenant' filter
	if tenant := r.URL.Query().Get("tenant"); tenant != "" {
		query.Filters = append(query.Filters, types.Filter{
			Key:   "tenant",
			Value: tenant,
		})
	}

	// Parse 'aggregate_id' filter
	if aggregateID := r.URL.Query().Get("aggregate_id"); aggregateID != "" {
		query.Filters = append(query.Filters, types.Filter{
			Key:   "aggregate_id",
			Value: aggregateID,
		})
	}

	return query, nil
}

// GetFilter retrieves a filter value by key
func GetFilter(filters []types.Filter, key string) (string, bool) {
	for _, f := range filters {
		if f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}
```

**Tests:** `transport/httptransport/query_test.go` (NEW)
- Test valid query parsing
- Test limit validation (0, negative, > 1000)
- Test filter extraction
- Test missing parameters (defaults)

---

### **Phase 2: Storage Layer Updates** (Day 2)

Enable filtering in storage backends.

#### 2.1 Update EventStore Interface
**File:** `synckit/types/interfaces.go` (MODIFY)

```go
// EventStore provides persistence for events.
type EventStore interface {
	// Store persists an event with the provided version
	Store(ctx context.Context, event Event, version Version) error

	// Load retrieves all events strictly after the provided version.
	// Optional filters can be provided for type, tenant, aggregate_id, etc.
	Load(ctx context.Context, since Version, filters ...Filter) ([]EventWithVersion, error)

	// LoadByAggregate retrieves events for a specific aggregate after the version.
	// Optional filters can be provided for type, tenant, etc.
	LoadByAggregate(ctx context.Context, aggregateID string, since Version, filters ...Filter) ([]EventWithVersion, error)

	// LatestVersion returns the latest version in the store.
	LatestVersion(ctx context.Context) (Version, error)

	// ParseVersion converts a string representation into a Version implementation.
	ParseVersion(ctx context.Context, versionStr string) (Version, error)

	// Close releases resources.
	Close() error
}
```

**Note:** Variadic `filters ...Filter` is backward compatible - existing calls work without changes.

---

#### 2.2 Implement Filtering in MemStore
**File:** `storage/memstore/memstore.go` (MODIFY)

```go
// Load retrieves all events since a given version with optional filters.
func (s *MemStore) Load(ctx context.Context, since synckit.Version, filters ...synckit.Filter) ([]synckit.EventWithVersion, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok && !since.IsZero() {
		return nil, fmt.Errorf("incompatible version type: expected cursor.IntegerCursor")
	}

	var result []synckit.EventWithVersion

	// Build filter map for quick lookup
	filterMap := make(map[string]string)
	for _, f := range filters {
		filterMap[f.Key] = f.Value
	}

	// Find all events with version > since and matching filters
	for _, ev := range s.events {
		if evCursor, ok := ev.Version.(cursor.IntegerCursor); ok {
			if evCursor.Seq > sinceCursor.Seq {
				// Apply filters
				if !matchesFilters(ev.Event, filterMap) {
					continue
				}
				result = append(result, ev)
			}
		}
	}

	return result, nil
}

// matchesFilters checks if an event matches all provided filters
func matchesFilters(event synckit.Event, filters map[string]string) bool {
	// Check type filter
	if eventType, ok := filters["type"]; ok {
		if event.Type() != eventType {
			return false
		}
	}

	// Check aggregate_id filter
	if aggregateID, ok := filters["aggregate_id"]; ok {
		if event.AggregateID() != aggregateID {
			return false
		}
	}

	// Check tenant filter (from metadata)
	if tenant, ok := filters["tenant"]; ok {
		meta := event.Metadata()
		if meta == nil {
			return false
		}
		eventTenant, exists := meta["tenant"]
		if !exists || eventTenant != tenant {
			return false
		}
	}

	return true
}
```

**Tests:** Update `storage/memstore/memstore_test.go`
- Test filtering by type
- Test filtering by tenant
- Test filtering by aggregate_id
- Test multiple filters
- Test backward compatibility (no filters)

---

#### 2.3 Implement Filtering in SQLite
**File:** `storage/sqlite/store.go` (MODIFY)

```go
// Load retrieves all events since a given version with optional filters.
func (s *Store) Load(ctx context.Context, since synckit.Version, filters ...synckit.Filter) ([]synckit.EventWithVersion, error) {
	// Build base query
	query := `SELECT id, type, aggregate_id, data, metadata, sequence 
	          FROM events 
	          WHERE sequence > ?`
	
	args := []interface{}{since.String()}

	// Build filter conditions
	filterMap := make(map[string]string)
	for _, f := range filters {
		filterMap[f.Key] = f.Value
	}

	// Add type filter
	if eventType, ok := filterMap["type"]; ok {
		query += " AND type = ?"
		args = append(args, eventType)
	}

	// Add aggregate_id filter
	if aggregateID, ok := filterMap["aggregate_id"]; ok {
		query += " AND aggregate_id = ?"
		args = append(args, aggregateID)
	}

	// Add tenant filter (stored in metadata JSON)
	if tenant, ok := filterMap["tenant"]; ok {
		query += " AND json_extract(metadata, '$.tenant') = ?"
		args = append(args, tenant)
	}

	query += " ORDER BY sequence ASC"

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	// ... rest of implementation
}
```

**Tests:** Update `storage/sqlite/store_test.go`
- Test SQL filtering by type
- Test SQL filtering by tenant
- Test combined filters
- Test performance with filters

---

#### 2.4 Implement Filtering in PostgreSQL
**File:** `storage/postgres/store.go` (MODIFY)

Similar to SQLite, using PostgreSQL's JSON operators (`->>`) for metadata filtering.

---

### **Phase 3: HTTP Handler Updates** (Day 3)

Integrate filtering into HTTP endpoints.

#### 3.1 Update Pull Handler
**File:** `transport/httptransport/http.go` (MODIFY)

```go
func (h *SyncHandler) handlePull(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Handling pull request",
		slog.String("method", r.Method),
		slog.String("remote_addr", r.RemoteAddr))

	if r.Method != http.MethodGet {
		respondWithStructuredError(w, r, opPull, ErrCodeInvalidRequest, "method not allowed", h.options)
		return
	}

	// Parse query parameters
	query, err := ParsePullQuery(r.Context(), r, h.versionParser)
	if err != nil {
		h.logger.Warn("Invalid pull query parameters",
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		respondWithStructuredError(w, r, opPull, ErrCodeInvalidCursor, err.Error(), h.options)
		return
	}

	// Call hooks if registered
	if h.hooks != nil && h.hooks.BeforePull != nil {
		h.hooks.BeforePull(r.Context(), query.Since)
	}

	// Load events with filters
	events, err := h.store.Load(r.Context(), query.Since, query.Filters...)
	if err != nil {
		h.logger.Error("Failed to load events",
			slog.String("error", err.Error()),
			slog.String("remote_addr", r.RemoteAddr))
		respondWithStructuredError(w, r, opPull, ErrCodeInternal, "failed to load events", h.options)
		return
	}

	// Apply limit
	if len(events) > query.Limit {
		events = events[:query.Limit]
	}

	// Convert to JSON format
	jsonEvents := make([]JSONEventWithVersion, len(events))
	for i, ev := range events {
		jsonEvents[i] = toJSONEventWithVersion(ev)
	}

	h.respond(w, r, http.StatusOK, jsonEvents)
}
```

**Tests:** Update `transport/httptransport/http_test.go`
- Test pull with type filter
- Test pull with tenant filter
- Test pull with limit
- Test pull with combined filters
- Test backward compatibility (no filters)

---

### **Phase 4: Multitenancy & Headers** (Day 3-4)

Add tenant support and custom headers.

#### 4.1 Header Constants
**File:** `transport/httptransport/headers.go` (NEW)

```go
package httptransport

// Custom HTTP headers for synckit
const (
	// HeaderSyncKitCursor contains the cursor value for client-side caching
	HeaderSyncKitCursor = "X-SyncKit-Cursor"

	// HeaderSyncKitTenant specifies the tenant ID for multitenancy
	HeaderSyncKitTenant = "X-SyncKit-Tenant"

	// HeaderIdempotencyKey provides idempotency for push operations
	HeaderIdempotencyKey = "Idempotency-Key"

	// HeaderSyncKitVersion contains the server version
	HeaderSyncKitVersion = "X-SyncKit-Version"
)

// ExtractTenant extracts the tenant ID from request headers or query params
func ExtractTenant(r *http.Request) string {
	// Try header first
	if tenant := r.Header.Get(HeaderSyncKitTenant); tenant != "" {
		return tenant
	}
	// Fall back to query param
	return r.URL.Query().Get("tenant")
}
```

---

#### 4.2 Update Handlers with Tenant Support
**File:** `transport/httptransport/http.go` (MODIFY)

```go
func (h *SyncHandler) handlePull(w http.ResponseWriter, r *http.Request) {
	// ... existing code ...

	// Parse query parameters
	query, err := ParsePullQuery(r.Context(), r, h.versionParser)
	if err != nil {
		// ... error handling ...
	}

	// Extract tenant from header
	if tenant := ExtractTenant(r); tenant != "" {
		// Add tenant filter
		query.Filters = append(query.Filters, types.Filter{
			Key:   "tenant",
			Value: tenant,
		})
	}

	// Load events with filters (including tenant)
	events, err := h.store.Load(r.Context(), query.Since, query.Filters...)
	// ... rest of implementation ...
}
```

---

### **Phase 5: Idempotency** (Day 4)

Prevent duplicate event processing.

#### 5.1 Idempotency Key Tracker
**File:** `transport/httptransport/idempotency.go` (NEW)

```go
package httptransport

import (
	"sync"
	"time"
)

// IdempotencyTracker tracks processed idempotency keys
type IdempotencyTracker struct {
	mu      sync.RWMutex
	keys    map[string]idempotencyEntry
	maxAge  time.Duration
	maxSize int
}

type idempotencyEntry struct {
	timestamp time.Time
	response  interface{} // Cached response
}

// NewIdempotencyTracker creates a new tracker
func NewIdempotencyTracker(maxAge time.Duration, maxSize int) *IdempotencyTracker {
	if maxAge == 0 {
		maxAge = 24 * time.Hour // Default: 24 hours
	}
	if maxSize == 0 {
		maxSize = 10000 // Default: 10k keys
	}

	tracker := &IdempotencyTracker{
		keys:    make(map[string]idempotencyEntry),
		maxAge:  maxAge,
		maxSize: maxSize,
	}

	// Start cleanup goroutine
	go tracker.cleanup()

	return tracker
}

// Check returns true if key was already processed (with cached response)
func (t *IdempotencyTracker) Check(key string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, exists := t.keys[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.timestamp) > t.maxAge {
		return nil, false
	}

	return entry.response, true
}

// Record stores a processed idempotency key
func (t *IdempotencyTracker) Record(key string, response interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Enforce max size (simple eviction)
	if len(t.keys) >= t.maxSize {
		// Remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range t.keys {
			if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		delete(t.keys, oldestKey)
	}

	t.keys[key] = idempotencyEntry{
		timestamp: time.Now(),
		response:  response,
	}
}

// cleanup removes expired keys periodically
func (t *IdempotencyTracker) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		t.mu.Lock()
		for key, entry := range t.keys {
			if time.Since(entry.timestamp) > t.maxAge {
				delete(t.keys, key)
			}
		}
		t.mu.Unlock()
	}
}
```

---

#### 5.2 Add Idempotency to Push Handler
**File:** `transport/httptransport/http.go` (MODIFY)

```go
// SyncHandler add idempotency tracker field
type SyncHandler struct {
	store              synckit.EventStore
	logger             *slog.Logger
	versionParser      VersionParser
	options            *ServerOptions
	hooks              *SyncHooks
	idempotencyTracker *IdempotencyTracker // NEW
}

func (h *SyncHandler) handlePush(w http.ResponseWriter, r *http.Request) {
	// ... existing validation ...

	// Check idempotency key
	if idempotencyKey := r.Header.Get(HeaderIdempotencyKey); idempotencyKey != "" {
		if cachedResp, found := h.idempotencyTracker.Check(idempotencyKey); found {
			h.logger.Debug("Idempotency key already processed, returning cached response",
				slog.String("key", idempotencyKey))
			h.respond(w, r, http.StatusOK, cachedResp)
			return
		}
	}

	// ... process events ...

	// Record idempotency key if provided
	if idempotencyKey := r.Header.Get(HeaderIdempotencyKey); idempotencyKey != "" {
		response := map[string]interface{}{
			"success": true,
			"events":  len(committedEvents),
		}
		h.idempotencyTracker.Record(idempotencyKey, response)
	}

	// ... rest of implementation ...
}
```

**Tests:** `transport/httptransport/idempotency_test.go` (NEW)
- Test duplicate detection
- Test expiration
- Test max size eviction
- Test concurrent access

---

### **Phase 6: Middleware** (Day 5)

Authentication and authorization layer.

#### 6.1 Middleware Interface
**File:** `transport/httptransport/middleware/middleware.go` (NEW)

```go
package middleware

import (
	"context"
	"net/http"
)

// Middleware wraps an http.Handler with additional functionality
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware in order
func Chain(h http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// ContextKey is a typed key for context values
type ContextKey string

const (
	// ContextKeyTenant stores the tenant ID in context
	ContextKeyTenant ContextKey = "tenant"

	// ContextKeyUserID stores the authenticated user ID
	ContextKeyUserID ContextKey = "user_id"
)
```

---

#### 6.2 Bearer Token Auth
**File:** `transport/httptransport/middleware/bearer.go` (NEW)

```go
package middleware

import (
	"context"
	"net/http"
	"strings"
)

// TokenValidator validates bearer tokens and returns user/tenant info
type TokenValidator func(token string) (userID, tenantID string, err error)

// BearerAuth middleware validates bearer tokens
func BearerAuth(validator TokenValidator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// Validate token
			userID, tenantID, err := validator(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Add to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyUserID, userID)
			if tenantID != "" {
				ctx = context.WithValue(ctx, ContextKeyTenant, tenantID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

---

#### 6.3 Tenant Extraction Middleware
**File:** `transport/httptransport/middleware/tenant.go` (NEW)

```go
package middleware

import (
	"context"
	"net/http"
)

// TenantExtractor middleware extracts tenant from header or context
func TenantExtractor(headerName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Check if tenant already in context (from auth)
			if tenant := ctx.Value(ContextKeyTenant); tenant != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Extract from header
			if tenant := r.Header.Get(headerName); tenant != "" {
				ctx = context.WithValue(ctx, ContextKeyTenant, tenant)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

---

#### 6.4 HMAC Signature Validation
**File:** `transport/httptransport/middleware/hmac.go` (NEW)

```go
package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// HMACValidator validates HMAC signatures
func HMACValidator(secret []byte, headerName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get signature from header
			signature := r.Header.Get(headerName)
			if signature == "" {
				http.Error(w, "missing signature", http.StatusUnauthorized)
				return
			}

			// Read body (must buffer for verification)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			// Compute HMAC
			mac := hmac.New(sha256.New, secret)
			mac.Write(body)
			expectedMAC := hex.EncodeToString(mac.Sum(nil))

			// Verify
			if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}

			// Restore body for handler
			r.Body = io.NopCloser(bytes.NewReader(body))

			next.ServeHTTP(w, r)
		})
	}
}
```

**Tests:** `transport/httptransport/middleware/*_test.go`
- Test bearer auth validation
- Test tenant extraction
- Test HMAC signature validation
- Test middleware chaining

---

### **Phase 7: Documentation** (Day 5)

Formal HTTP API specification.

#### 7.1 HTTP Spec Document
**File:** `docs/http-spec.md` (NEW)

```markdown
# HTTP Transport API Specification

Version: 1.0  
Status: Stable  
Last Updated: 2025-01-06

## Overview

The go-sync-kit HTTP transport provides a RESTful API for event synchronization
between clients and servers.

## Base URL

```
https://api.example.com/sync
```

## Authentication

### Bearer Token
```
Authorization: Bearer <token>
```

### HMAC Signature
```
X-SyncKit-Signature: <hmac-sha256-hex>
```

## Endpoints

### 1. Push Events

**Endpoint:** `POST /push`

Pushes a batch of events to the server.

**Headers:**
- `Content-Type: application/json`
- `Authorization: Bearer <token>` (optional)
- `Idempotency-Key: <uuid>` (optional, recommended)
- `X-SyncKit-Tenant: <tenant-id>` (optional, for multitenancy)
- `Content-Encoding: gzip` (optional)

**Request Body:**
```json
[
  {
    "event": {
      "id": "evt-123",
      "type": "OrderCreated",
      "aggregate_id": "order-456",
      "data": {"amount": 99.99},
      "metadata": {"tenant": "acme-corp"}
    },
    "version": "42"
  }
]
```

**Response:** `200 OK`
```json
{
  "success": true,
  "events": 1
}
```

**Error Response:** `400 Bad Request`
```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "invalid event format",
    "op": "push"
  }
}
```

---

### 2. Pull Events

**Endpoint:** `GET /pull`

Retrieves events from the server.

**Query Parameters:**
- `since` (string, optional): Cursor position to start from
- `limit` (int, optional): Max events to return (default: 100, max: 1000)
- `type` (string, optional): Filter by event type
- `tenant` (string, optional): Filter by tenant
- `aggregate_id` (string, optional): Filter by aggregate

**Headers:**
- `Authorization: Bearer <token>` (optional)
- `X-SyncKit-Tenant: <tenant-id>` (optional)

**Response:** `200 OK`
```json
[
  {
    "event": {
      "id": "evt-789",
      "type": "OrderUpdated",
      "aggregate_id": "order-456",
      "data": {"status": "shipped"},
      "metadata": {}
    },
    "version": "43"
  }
]
```

**Response Headers:**
- `X-SyncKit-Cursor: 43` (latest cursor)

---

### 3. Get Latest Version

**Endpoint:** `GET /latest-version`

Returns the latest version/cursor without pulling events.

**Response:** `200 OK`
```json
{
  "version": "43"
}
```

---

### 4. Subscribe (SSE)

**Endpoint:** `GET /subscribe` (TODO - Phase 8)

Server-Sent Events (SSE) stream for real-time updates.

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_CURSOR` | 400 | Invalid cursor format |
| `INVALID_REQUEST` | 400 | Malformed request body |
| `AUTH_REQUIRED` | 401 | Missing or invalid authentication |
| `INVALID_TENANT` | 400 | Invalid tenant ID |
| `CONFLICT` | 409 | Event conflict detected |
| `REQUEST_TOO_LARGE` | 413 | Request body exceeds size limit |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Wire Format

### WireEvent (with Codec Support)

```json
{
  "id": "evt-123",
  "type": "OrderCreated",
  "aggregate_id": "order-456",
  "data": "<encoded-data>",
  "data_kind": "protobuf:order",
  "metadata": {"tenant": "acme-corp"}
}
```

**Fields:**
- `id` (string, required): Unique event identifier
- `type` (string, required): Event type name
- `aggregate_id` (string, required): Aggregate identifier
- `data` (json, required): Event payload (codec-encoded or JSON)
- `data_kind` (string, optional): Codec identifier (e.g., "protobuf:order", "json")
- `metadata` (object, optional): Additional metadata

---

## Compression

Both request and response bodies support gzip compression.

**Request compression:**
```
Content-Encoding: gzip
```

**Response compression:**
```
Accept-Encoding: gzip, deflate
Content-Encoding: gzip
```

---

## Limits

- **Max request size:** 10 MiB (compressed)
- **Max decompressed size:** 20 MiB
- **Max events per pull:** 1000
- **Request timeout:** 30 seconds
- **Idempotency key TTL:** 24 hours

---

## Examples

See `/examples/http_client` and `/examples/http_server` for full examples.
```

---

### **Phase 8: Testing & Integration** (Day 5)

Comprehensive tests for all new features.

#### 8.1 Integration Tests
**File:** `transport/httptransport/integration_test.go` (NEW)

- Test end-to-end pull with filters
- Test multitenancy isolation
- Test idempotency
- Test middleware chain
- Test error responses

#### 8.2 Update Existing Tests
- Update all existing tests to use structured errors
- Add backward compatibility tests

---

## 📊 Implementation Checklist

### Phase 1: Core Infrastructure ✅
- [ ] Create `errors.go` with structured error types
- [ ] Create `errors_test.go`
- [ ] Create `query.go` with PullQuery parsing
- [ ] Create `query_test.go`
- [ ] Update `http.go` to use structured errors

### Phase 2: Storage Layer ✅ **COMPLETE**
- [x] Update `synckit/types/interfaces.go` with variadic filters
- [x] Implement filtering in `storage/memstore/memstore.go`
- [x] Add tests for memstore filtering
- [x] Implement filtering in `storage/sqlite/store.go`
- [x] Add tests for SQLite filtering
- [x] Implement filtering in `storage/postgres/store.go`
- [x] Add tests for Postgres filtering
- [x] Update all test mocks to support new interface signature

**Commit:** `feat(storage): add filtering support to EventStore interface` (b643b4d)

### Phase 3: HTTP Handler Updates ✅
- [ ] Update `handlePull` with query parsing
- [ ] Update `handlePull` with filter support
- [ ] Add limit enforcement
- [ ] Add tests for filtered pull

### Phase 4: Multitenancy & Headers ✅
- [ ] Create `headers.go` with constants
- [ ] Add `ExtractTenant` helper
- [ ] Update handlers to use tenant header
- [ ] Add tests for tenant extraction

### Phase 5: Idempotency ✅ **COMPLETE**
- [x] Create `idempotency.go` with tracker
- [x] Create `idempotency_test.go`
- [x] Update `handlePush` with idempotency
- [x] Add integration tests

**Commit:** `feat(http): Phase 5 - Add idempotency support` (00297ef)

### Phase 6: Middleware ✅ **COMPLETE**
- [x] Create `middleware/middleware.go` with Chain function and context keys
- [x] Create `middleware/bearer.go` with BearerAuth middleware
- [x] Create `middleware/tenant.go` with TenantExtractor middleware
- [x] Create `middleware/hmac.go` with HMACValidator middleware
- [x] Add comprehensive tests for all middleware components
- [x] Add tests for middleware chaining and integration
- [x] Add context helper functions (UserIDFromContext, TenantFromContext)
- [x] Create HTTP API specification document (`docs/http-spec.md`)
- [ ] Add example usage in examples/ (deferred)

**Commit:** `feat(middleware): Implement authentication and authorization middleware (Phase 6)` (856a800)

### Phase 7: Documentation ✅ **COMPLETE**
- [x] Create `docs/http-spec.md` (completed in Phase 6)
- [x] Update README with HTTP enterprise features section
- [x] Add migration guide (`docs/MIGRATION_GUIDE_HTTP.md`)
- [x] Add code examples for all new features
- [x] Document backward compatibility guarantees

**Commit:** `docs(http): Phase 7 - Complete documentation and migration guide` (pending)

### Phase 8: Testing & Integration ✅
- [ ] Create integration test suite
- [ ] Update all existing tests
- [ ] Add backward compatibility tests
- [ ] Add performance benchmarks
- [ ] Test with real-world scenarios

---

## 🧪 Testing Strategy

### Unit Tests
- Each new file gets companion `*_test.go`
- Test happy paths and error cases
- Test edge cases (limits, validation)

### Integration Tests
- End-to-end scenarios with real HTTP server
- Test middleware combinations
- Test filter combinations
- Test multitenancy isolation

### Backward Compatibility Tests
- Ensure existing code works without changes
- Test with old query params (no filters)
- Test with old error handling

### Performance Tests
- Benchmark filtering queries
- Benchmark idempotency lookup
- Benchmark middleware overhead

---

## 📝 Commit Strategy

Follow your rule: "commit on 100% passing tests"

**Suggested commit sequence:**

1. `feat(http): add structured error responses`
2. `feat(http): add query parameter parsing for filters`
3. `feat(storage): add filtering support to EventStore interface`
4. `feat(storage): implement filtering in memstore`
5. `feat(storage): implement filtering in SQLite`
6. `feat(storage): implement filtering in Postgres`
7. `feat(http): integrate filtering into pull handler`
8. `feat(http): add multitenancy header support`
9. `feat(http): add idempotency key tracking`
10. `feat(http): add authentication middleware`
11. `docs(http): add formal HTTP API specification`
12. `test(http): add comprehensive integration tests`

---

## 🚀 Rollout Plan

### v0.24.0-alpha.1
- Phase 1-3 (Core + Filtering)
- Alpha release for testing

### v0.24.0-beta.1
- Phase 4-6 (Multitenancy + Middleware)
- Beta release for enterprise testing

### v0.24.0
- Phase 7-8 (Docs + Testing)
- Stable release

---

## 🎯 Success Criteria

- [ ] All tests passing (100%)
- [ ] Backward compatibility maintained
- [ ] Documentation complete
- [ ] Examples updated
- [ ] Performance benchmarks show no regression
- [ ] Code review approved

---

## 📚 References

- [HTTP Protocol Overlap Analysis](./HTTP_PROTOCOL_OVERLAP_ANALYSIS.md)
- [Original Suggestions](./WARP.md) (if available)
- [Go HTTP Best Practices](https://go.dev/doc/effective_go#web_server)

---

**Ready to implement!** Start with Phase 1 and work through systematically.
