# HTTP Wire Protocol & API - Overlap Analysis

## Executive Summary

**Status:** 🟢 **NO MAJOR OVERLAP** - Suggestions are complementary enhancements

The proposed HTTP spec additions would **enhance** (not duplicate) the existing implementation. Current code has solid foundations but is missing several enterprise features.

---

## Current Implementation Status

### ✅ **What EXISTS Today**

#### 1. **Core Endpoints (Basic)**
```go
// transport/httptransport/http.go
switch path {
    case "/push":                // ✅ POST endpoint exists
    case "/pull":                // ✅ GET endpoint exists  
    case "/latest-version":      // ✅ Version endpoint exists
    case "/pull-cursor":         // ✅ Cursor-based pull exists
}
```

#### 2. **Wire Format with Codec Support**
```go
// transport/httptransport/wire_format.go
type WireEvent struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    AggregateID string                 `json:"aggregate_id"`
    Data        json.RawMessage        `json:"data"`
    DataKind    string                 `json:"data_kind,omitempty"` // ✅ Codec field exists!
    Metadata    map[string]interface{} `json:"metadata"`
}

// ✅ CodecAwareEncoder already implemented
type CodecAwareEncoder struct {
    registry *codec.Registry
    fallback bool
}
```

#### 3. **Compression Support**
```go
// transport/httptransport/compression.go
// ✅ Full gzip compression/decompression with:
// - Content-Encoding headers
// - Zip-bomb protection
// - Size limits (compressed + decompressed)
```

#### 4. **Server Options**
```go
type ServerOptions struct {
    MaxRequestSize          int64           // ✅ Size limits
    MaxDecompressedSize     int64           // ✅ Zip-bomb protection
    CompressionEnabled      bool            // ✅ Compression control
    RequestTimeout          time.Duration   // ✅ Timeout handling
    CodecAwareEncoder       *CodecAwareEncoder // ✅ Codec support
}
```

---

## ❌ **What's MISSING (Suggestions Add Value)**

### 1. **Structured Error Responses**

**Current:** Basic string errors
```go
// Current implementation
h.respondErr(w, r, http.StatusBadRequest, "invalid cursor")
```

**Suggested:** Structured JSON errors
```json
{
  "error": {
    "code": "INVALID_CURSOR",
    "message": "cursor format not recognized",
    "op": "pull"
  }
}
```

**Action:** ✅ **ADD THIS** - No overlap, pure enhancement

---

### 2. **Filtering & Pagination Query Params**

**Current:** Only `since` cursor support
```go
// /pull?since=<cursor> works
// /pull?type=Foo&tenant=T&limit=100 does NOT work ❌
```

**Suggested:** Rich query support
```go
type PullQuery struct {
    Since  types.Version
    Limit  int          // ❌ MISSING
    Filter []types.Filter // ❌ MISSING (but type exists!)
}
```

**Current State:**
- `types.Filter` struct EXISTS in `synckit/types/interfaces.go`
- NOT used in any Transport/Store signatures yet
- Documented as "forward-compatible"

**Action:** ✅ **ADD THIS** - Framework exists, need implementation

---

### 3. **Multitenancy Headers**

**Current:** No tenant support
```go
// X-SyncKit-Tenant header not recognized ❌
```

**Suggested:**
```go
// Headers
X-SyncKit-Tenant: <tenant-id>
```

**Storage Layer:**
```go
// Need to add to store queries:
func (s *Store) Load(ctx context.Context, since Version, filters ...Filter) 
```

**Action:** ✅ **ADD THIS** - No overlap, new feature

---

### 4. **Idempotency Support**

**Current:** No idempotency keys
```go
// Idempotency-Key header not checked ❌
```

**Suggested:**
```go
// Header for push operations
Idempotency-Key: <uuid>
```

**Action:** ✅ **ADD THIS** - Prevents duplicate event processing

---

### 5. **Authentication/Authorization Middleware**

**Current:** No auth layer
```
transport/httptransport/
  ├── No middleware/ directory ❌
  ├── No auth handlers ❌
```

**Suggested:**
```
transport/httptransport/middleware/
  ├── bearer.go      # Bearer token auth
  ├── hmac.go        # HMAC signing
  ├── tenant.go      # Tenant extraction
```

**Action:** ✅ **ADD THIS** - Critical for production use

---

### 6. **Standardized Endpoint Paths**

**Current:** Inconsistent naming
```go
/push              // ✅ RESTful
/pull              // ✅ RESTful  
/latest-version    // ❌ Not RESTful (kebab-case)
/pull-cursor       // ❌ Not RESTful (kebab-case)
```

**Suggested:** Consistent REST-style
```
POST   /v1/events:push
GET    /v1/events:pull
GET    /v1/events:latest
GET    /v1/events:subscribe  (SSE)
```

**Action:** ⚠️ **BREAKING CHANGE** - Consider for v1.0.0 or v2.0.0

---

### 7. **Formal HTTP Spec Document**

**Current:** No formal API spec
```
docs/
  ├── No http-spec.md ❌
  ├── No OpenAPI/Swagger ❌
```

**Suggested:**
```
docs/
  ├── http-spec.md          # Formal wire protocol spec
  └── openapi.yaml          # (Optional) OpenAPI spec
```

**Action:** ✅ **ADD THIS** - Essential for users

---

## 📊 Feature Comparison Matrix

| Feature | Current Status | Suggested | Overlap? | Action |
|---------|---------------|-----------|----------|--------|
| **Endpoints** |
| POST /push | ✅ Exists | POST /v1/events:push | ⚠️ Path | Rename (breaking) |
| GET /pull | ✅ Exists | GET /v1/events:pull | ⚠️ Path | Rename (breaking) |
| GET /latest-version | ✅ Exists | GET /v1/events:latest | ⚠️ Path | Rename (breaking) |
| SSE /subscribe | ❌ Missing | GET /v1/events:subscribe | ✅ No | Add (non-breaking) |
| **Headers** |
| Content-Encoding | ✅ Exists | Same | ✅ Yes | Keep as-is |
| X-SyncKit-Cursor | ❌ Missing | X-SyncKit-Cursor | ✅ No | Add (non-breaking) |
| Idempotency-Key | ❌ Missing | Idempotency-Key | ✅ No | Add (non-breaking) |
| X-SyncKit-Tenant | ❌ Missing | X-SyncKit-Tenant | ✅ No | Add (non-breaking) |
| **Wire Format** |
| Codec field | ✅ Exists (`data_kind`) | `codec` field | ⚠️ Name | Alias or rename |
| JSON events | ✅ Exists | Same | ✅ Yes | Keep as-is |
| Version encoding | ✅ Exists | Enhanced format | ⚠️ Partial | Document better |
| **Errors** |
| String errors | ✅ Exists | Structured JSON | ✅ No | Add (non-breaking) |
| HTTP status codes | ✅ Exists | Same + 409 | ⚠️ Partial | Add 409 handling |
| **Filtering** |
| Query params | ❌ Only `since` | Full filtering | ✅ No | Add (non-breaking) |
| Store filters | ❌ Type exists, unused | Implement in stores | ✅ No | Add (non-breaking) |
| **Auth/Multitenancy** |
| Middleware | ❌ Missing | Full middleware | ✅ No | Add (non-breaking) |
| Tenant filtering | ❌ Missing | Tenant header | ✅ No | Add (non-breaking) |
| **Documentation** |
| HTTP spec doc | ❌ Missing | `/docs/http-spec.md` | ✅ No | Add |

---

## 🎯 Recommendations

### **Phase 1: Non-Breaking Additions** (v0.23.0 or v0.24.0)

1. ✅ **Add structured error responses**
   - Create `ErrorResponse` type
   - Map errors to codes (INVALID_CURSOR, etc.)
   - Add `op` field for operation tracking

2. ✅ **Add filtering support**
   - Implement query param parsing: `?type=Foo&tenant=T&limit=100`
   - Update Store interfaces to accept `filters ...types.Filter`
   - Implement in Postgres/SQLite stores

3. ✅ **Add multitenancy headers**
   - Parse `X-SyncKit-Tenant` header
   - Pass to store as filter
   - Add middleware for tenant extraction

4. ✅ **Add idempotency support**
   - Parse `Idempotency-Key` header
   - Track processed keys (in-memory cache or store)
   - Return 200 for duplicate keys

5. ✅ **Add middleware package**
   ```
   transport/httptransport/middleware/
     ├── auth.go          # Bearer + HMAC
     ├── tenant.go        # Tenant extraction
     ├── idempotency.go   # Idempotency checking
   ```

6. ✅ **Write formal HTTP spec**
   - Create `/docs/http-spec.md`
   - Document all endpoints, headers, errors
   - Add examples

### **Phase 2: Breaking Changes** (v1.0.0)

7. ⚠️ **Standardize endpoint paths**
   ```go
   // Old (v0.x)
   /push, /pull, /latest-version
   
   // New (v1.0)
   /v1/events:push, /v1/events:pull, /v1/events:latest
   ```

8. ⚠️ **Rename wire format fields** (if needed)
   ```go
   // Consider: data_kind → codec
   // Or: keep data_kind and document
   ```

---

## 🔍 Detailed Code Impact

### 1. **Add Filtering to Store Interface**

**Current:**
```go
type EventStore interface {
    Load(ctx context.Context, since Version) ([]EventWithVersion, error)
}
```

**Enhanced (backward compatible via variadic):**
```go
type EventStore interface {
    // Add variadic filters - backward compatible!
    Load(ctx context.Context, since Version, filters ...types.Filter) ([]EventWithVersion, error)
}
```

### 2. **Add Query Parsing to HTTP Handler**

```go
// NEW: transport/httptransport/query.go
type PullQuery struct {
    Since  synckit.Version
    Limit  int
    Filter []types.Filter
}

func ParsePullQuery(r *http.Request, parser VersionParser) (*PullQuery, error) {
    query := &PullQuery{Limit: 100} // default
    
    // Parse since
    if since := r.URL.Query().Get("since"); since != "" {
        version, err := parser(r.Context(), since)
        if err != nil {
            return nil, fmt.Errorf("invalid since: %w", err)
        }
        query.Since = version
    }
    
    // Parse limit
    if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
        limit, err := strconv.Atoi(limitStr)
        if err != nil || limit <= 0 || limit > 1000 {
            return nil, fmt.Errorf("invalid limit (1-1000)")
        }
        query.Limit = limit
    }
    
    // Parse filters
    if eventType := r.URL.Query().Get("type"); eventType != "" {
        query.Filter = append(query.Filter, types.Filter{Key: "type", Value: eventType})
    }
    if tenant := r.URL.Query().Get("tenant"); tenant != "" {
        query.Filter = append(query.Filter, types.Filter{Key: "tenant", Value: tenant})
    }
    
    return query, nil
}
```

### 3. **Add Error Response Type**

```go
// NEW: transport/httptransport/errors.go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string `json:"code"`    // INVALID_CURSOR, AUTH_REQUIRED, etc.
    Message string `json:"message"`
    Op      string `json:"op"`      // push, pull, subscribe
}

// Map Go errors to structured responses
func toErrorResponse(op string, err error) ErrorResponse {
    code := "INTERNAL_ERROR"
    msg := err.Error()
    
    // Check for specific error types
    switch {
    case strings.Contains(msg, "cursor"):
        code = "INVALID_CURSOR"
    case strings.Contains(msg, "unauthorized"):
        code = "AUTH_REQUIRED"
    case strings.Contains(msg, "tenant"):
        code = "INVALID_TENANT"
    }
    
    return ErrorResponse{
        Error: ErrorDetail{
            Code:    code,
            Message: msg,
            Op:      op,
        },
    }
}
```

---

## ✅ **Final Verdict: NO OVERLAP**

### **Summary:**

1. **Existing code is solid foundation** - Wire format, codec, compression all done well
2. **Suggestions fill gaps** - Auth, filtering, multitenancy, errors, docs
3. **No duplication** - Everything suggested adds new capabilities
4. **Backward compatible** - Most changes can be additive (variadic params)

### **Recommendation:**

✅ **PROCEED with suggestions** as Phase 1 (non-breaking) enhancements

The suggested HTTP spec would:
- Document existing behavior
- Add missing enterprise features (auth, multitenancy, idempotency)
- Improve error handling
- Enable rich filtering

**All without breaking existing code!**

---

## 📝 Next Steps

1. **Create `/docs/http-spec.md`** - Document current + planned API
2. **Add filtering support** - Query params + store implementation
3. **Add middleware package** - Auth, tenant, idempotency
4. **Structured errors** - JSON error responses
5. **Tests** - Full integration tests for new features

Would you like me to start with any of these?
