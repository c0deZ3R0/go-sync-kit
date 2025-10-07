# HTTP Transport Migration Guide (v0.24+)

**Version:** 0.24.0  
**Status:** Stable  
**Last Updated:** 2025-10-07

## Overview

Version 0.24 introduces enterprise-grade features to the HTTP transport while maintaining **100% backward compatibility**. All existing code continues to work without changes.

This guide covers new features and optional migration paths to leverage enhanced capabilities.

---

## Table of Contents

- [What's New](#whats-new)
- [Backward Compatibility](#backward-compatibility)
- [Feature Adoption Guide](#feature-adoption-guide)
- [Breaking Changes](#breaking-changes-none)
- [Examples](#examples)

---

## What's New

### 1. Structured Error Responses

**Before (v0.23 and earlier):**
```json
HTTP/1.1 400 Bad Request
invalid cursor format
```

**After (v0.24+):**
```json
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": {
    "code": "INVALID_CURSOR",
    "message": "invalid cursor format: expected integer",
    "op": "pull"
  }
}
```

**Error Codes:**
- `INVALID_CURSOR` - Invalid cursor/version format
- `INVALID_REQUEST` - Malformed request body
- `AUTH_REQUIRED` - Missing or invalid authentication
- `INVALID_TENANT` - Invalid tenant ID
- `CONFLICT` - Event conflict detected
- `REQUEST_TOO_LARGE` - Request exceeds size limit
- `INTERNAL_ERROR` - Server error

**Migration:** No changes required. Update error parsing in clients if you want structured error handling.

---

### 2. Advanced Filtering & Pagination

**Query Parameters:**
- `since` - Cursor to start from (existing)
- `limit` - Max events per request (new, default: 100, max: 1000)
- `type` - Filter by event type (new)
- `tenant` - Filter by tenant (new)
- `aggregate_id` - Filter by aggregate ID (new)

**Example:**
```bash
# Pull only OrderCreated events for tenant "acme-corp", max 50 events
curl "http://localhost:8080/sync/pull?since=42&type=OrderCreated&tenant=acme-corp&limit=50"
```

**Migration:** Optional. Filtering is additive - omitting parameters returns all events (existing behavior).

---

### 3. Multitenancy Support

**Custom Headers:**
```
X-SyncKit-Tenant: <tenant-id>
X-SyncKit-Cursor: <cursor-value>
X-SyncKit-Version: <server-version>
```

**Go Client Example:**
```go
req, _ := http.NewRequest("GET", "http://localhost:8080/sync/pull", nil)
req.Header.Set("X-SyncKit-Tenant", "acme-corp")

resp, _ := client.Do(req)
```

**Tenant Filtering:**
- Header takes precedence over query param
- Events must have `tenant` in metadata to be filterable
- Middleware can enforce tenant isolation (see Authentication section)

**Migration:** Optional. Add `X-SyncKit-Tenant` header if using multitenancy.

---

### 4. Idempotency Keys

Prevent duplicate event processing with idempotency keys:

**Header:**
```
Idempotency-Key: <uuid-or-unique-string>
```

**Example:**
```go
import "github.com/google/uuid"

req, _ := http.NewRequest("POST", "http://localhost:8080/sync/push", body)
req.Header.Set("Idempotency-Key", uuid.New().String())
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

**Behavior:**
- First request with key: processes events, caches response
- Duplicate request with same key (within 24h): returns cached response
- Key expiration: 24 hours (default, configurable)
- Max keys tracked: 10,000 (default, configurable)

**Migration:** Optional but **highly recommended** for push operations to prevent duplicates.

---

### 5. Authentication & Authorization Middleware

**Middleware Types:**
1. **Bearer Token** - JWT or custom token validation
2. **HMAC Signature** - Request signing with shared secret
3. **Tenant Extraction** - Extract tenant from headers

**Example: Bearer Auth**
```go
import (
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport/middleware"
)

// Token validator function
validator := func(token string) (userID, tenantID string, err error) {
	// Your JWT validation or database lookup
	claims, err := jwt.Parse(token)
	if err != nil {
		return "", "", err
	}
	return claims.UserID, claims.TenantID, nil
}

// Create handler with middleware
handler := middleware.Chain(
	httptransport.NewSyncHandler(store, nil, nil, nil),
	middleware.BearerAuth(validator),
	middleware.TenantExtractor("X-SyncKit-Tenant"),
)

http.Handle("/sync/", handler)
```

**Example: HMAC Signature**
```go
// Server-side HMAC validation
secret := []byte("your-shared-secret")
handler := middleware.Chain(
	httptransport.NewSyncHandler(store, nil, nil, nil),
	middleware.HMACValidator(secret, "X-SyncKit-Signature"),
)

// Client-side HMAC signing
import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

body := []byte(`[{"event": {...}}]`)
mac := hmac.New(sha256.New, []byte("your-shared-secret"))
mac.Write(body)
signature := hex.EncodeToString(mac.Sum(nil))

req.Header.Set("X-SyncKit-Signature", signature)
```

**Migration:** Optional. Add middleware if authentication is required.

---

## Backward Compatibility

### ✅ No Breaking Changes

All v0.23 code works in v0.24 without modification:

**Old code still works:**
```go
// v0.23 - still works in v0.24
store, _ := sqlite.New(&sqlite.Config{DataSourceName: "client.db"})
transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)
node, _ := synckit.NewHTTPClientNode(store, transport)
res, _ := node.Sync(context.Background())
```

**Existing endpoints unchanged:**
- `GET /pull` - works without query params
- `POST /push` - works without idempotency key
- Error responses include structured format (clients tolerant of plain text still work)

---

## Feature Adoption Guide

### Recommended Migration Path

1. **Start:** Continue using existing code (no changes needed)
2. **Add filtering:** Gradually add query parameters to pull requests
3. **Add idempotency:** Add `Idempotency-Key` header to push operations
4. **Add multitenancy:** Use `X-SyncKit-Tenant` header if applicable
5. **Add authentication:** Wrap handlers with middleware when ready

### Example Progressive Migration

**Phase 1: Add Idempotency (5 minutes)**
```go
// Only change: add idempotency header to push requests
req.Header.Set("Idempotency-Key", uuid.New().String())
```

**Phase 2: Add Filtering (10 minutes)**
```go
// Add query parameters to pull requests
url := fmt.Sprintf("%s/pull?since=%s&type=OrderCreated&limit=100", baseURL, cursor)
```

**Phase 3: Add Authentication (30 minutes)**
```go
// Wrap handler with middleware
handler := middleware.Chain(
	httptransport.NewSyncHandler(store, nil, nil, nil),
	middleware.BearerAuth(yourTokenValidator),
)
```

---

## Breaking Changes (None)

**There are no breaking changes in v0.24.**

All existing applications continue to work without modification.

---

## Examples

### Complete Client Example with All Features

```go
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
	// Standard setup
	store, _ := sqlite.New(&sqlite.Config{DataSourceName: "client.db"})
	transport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)
	node, _ := synckit.NewHTTPClientNode(store, transport)

	// Push with idempotency and auth
	ctx := context.Background()
	events := []synckit.EventWithVersion{
		// your events...
	}

	body, _ := json.Marshal(events)
	req, _ := http.NewRequest("POST", "http://localhost:8080/sync/push", bytes.NewBuffer(body))
	
	// Add idempotency key
	req.Header.Set("Idempotency-Key", uuid.New().String())
	
	// Add bearer token
	req.Header.Set("Authorization", "Bearer your-jwt-token")
	
	// Add tenant
	req.Header.Set("X-SyncKit-Tenant", "acme-corp")
	
	// Add HMAC signature (optional, if server requires)
	secret := []byte("your-shared-secret")
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	req.Header.Set("X-SyncKit-Signature", hex.EncodeToString(mac.Sum(nil)))
	
	// Execute
	client := &http.Client{}
	resp, _ := client.Do(req)
	
	fmt.Printf("Push status: %d\n", resp.StatusCode)

	// Pull with filtering
	url := fmt.Sprintf("http://localhost:8080/sync/pull?since=0&type=OrderCreated&tenant=acme-corp&limit=50")
	pullReq, _ := http.NewRequest("GET", url, nil)
	pullReq.Header.Set("Authorization", "Bearer your-jwt-token")
	pullReq.Header.Set("X-SyncKit-Tenant", "acme-corp")
	
	pullResp, _ := client.Do(pullReq)
	fmt.Printf("Pull status: %d\n", pullResp.StatusCode)
}
```

### Complete Server Example with Middleware

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport/middleware"
)

func main() {
	// Setup store
	store, err := sqlite.New(&sqlite.Config{DataSourceName: "server.db"})
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Token validator (example)
	validator := func(token string) (userID, tenantID string, err error) {
		// Your validation logic here
		// For example: parse JWT, check database, etc.
		if token == "valid-token" {
			return "user-123", "acme-corp", nil
		}
		return "", "", fmt.Errorf("invalid token")
	}

	// Create handler with middleware chain
	baseHandler := httptransport.NewSyncHandler(store, nil, nil, nil)
	
	handler := middleware.Chain(
		baseHandler,
		middleware.BearerAuth(validator),
		middleware.TenantExtractor("X-SyncKit-Tenant"),
	)

	// Register routes
	http.Handle("/sync/", handler)
	
	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## Support & Resources

- **Full API Spec**: [`docs/http-spec.md`](http-spec.md)
- **Examples**: [`examples/http_client`](../examples/http_client), [`examples/http_server`](../examples/http_server)
- **Issues**: [GitHub Issues](https://github.com/c0deZ3R0/go-sync-kit/issues)

---

**Questions?** Open an issue on GitHub or check the examples directory for complete working code.
