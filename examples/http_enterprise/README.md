# Enterprise HTTP Server Example

A production-ready example demonstrating how to build an enterprise-grade HTTP sync server using go-sync-kit with authentication, multitenancy, compression, idempotency, and middleware chaining.

## Features Demonstrated

✅ **Authentication & Authorization**
- Bearer token authentication
- HMAC signature validation (optional)
- Multi-tenant isolation
- User context propagation

✅ **Security**
- Request size limits (10MB)
- Decompression bomb protection (20MB limit)
- HMAC-based request signing
- Tenant isolation

✅ **Performance**
- Response compression (>1KB threshold)
- Efficient event filtering
- Request timeouts (30s)
- Graceful shutdown (10s)

✅ **Reliability**
- Idempotency key support
- Structured error responses
- Request/response hooks
- Structured logging (slog)

## Quick Start

### 1. Run the server

```bash
cd examples/http_enterprise/server
go run main.go
```

The server will start on `http://localhost:8080` and seed demo data:
- **acme-corp** tenant: 4 events
- **globex-inc** tenant: 2 events

### 2. Try the API

#### Pull events (with authentication)

```bash
# Pull all events for acme-corp tenant
curl -H "Authorization: Bearer admin-token" \
     http://localhost:8080/pull

# Pull filtered events
curl -H "Authorization: Bearer admin-token" \
     "http://localhost:8080/pull?type=OrderCreated&limit=10"

# Pull events for different tenant
curl -H "Authorization: Bearer globex-token" \
     http://localhost:8080/pull
```

#### Push events (with idempotency)

```bash
curl -X POST \
     -H "Authorization: Bearer admin-token" \
     -H "Content-Type: application/json" \
     -H "Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000" \
     -d '[{
       "event": {
         "id": "evt-123",
         "type": "OrderCreated",
         "aggregate_id": "order-999",
         "data": {"amount": 199.99, "customer": "eve"}
       },
       "version": "5"
     }]' \
     http://localhost:8080/push
```

#### Get latest version

```bash
curl -H "Authorization: Bearer admin-token" \
     http://localhost:8080/latest-version
```

## Authentication

The example includes three demo tokens:

| Token | User ID | Tenant | Description |
|-------|---------|--------|-------------|
| `admin-token` | admin-user | acme-corp | Admin access to acme-corp |
| `user-token` | regular-user | acme-corp | Regular user for acme-corp |
| `globex-token` | globex-user | globex-inc | Access to globex-inc tenant |

### Production Authentication

In production, replace the simple token validation with:

```go
authValidator := func(token string) (userID, tenantID string, err error) {
    // Validate JWT token
    claims, err := validateJWT(token)
    if err != nil {
        return "", "", err
    }
    
    // Extract user and tenant from claims
    return claims.UserID, claims.TenantID, nil
}
```

## Middleware Chain

The server uses a layered middleware approach:

```go
handler := middleware.Chain(
    baseHandler,                                              // Core sync handler
    middleware.TenantExtractor("X-Tenant-ID"),               // Extract tenant
    middleware.BearerAuth(authValidator),                     // Authenticate user
    middleware.HMACValidator([]byte(secret), "X-HMAC-Signature"), // Verify signature
)
```

Middleware executes in **reverse order**:
1. HMAC validation (optional)
2. Bearer token authentication (required)
3. Tenant extraction
4. Base sync handler

## Multitenancy

Events are automatically filtered by tenant. Each tenant can only see their own events:

```go
// acme-corp tenant sees only their events
curl -H "Authorization: Bearer admin-token" http://localhost:8080/pull
// Returns: OrderCreated, OrderUpdated, PaymentProcessed (4 events)

// globex-inc tenant sees only their events
curl -H "Authorization: Bearer globex-token" http://localhost:8080/pull
// Returns: OrderCreated (2 events)
```

### How It Works

1. Bearer token is validated → returns `userID` and `tenantID`
2. Context is enriched with tenant info
3. Query filters automatically apply tenant constraint
4. Events returned only for authenticated tenant

## Idempotency

Prevent duplicate event processing with idempotency keys:

```bash
# First request processes normally
curl -X POST \
     -H "Authorization: Bearer admin-token" \
     -H "Idempotency-Key: my-unique-key-123" \
     -H "Content-Type: application/json" \
     -d '[{"event":{...}, "version":"1"}]' \
     http://localhost:8080/push

# Duplicate request returns cached response
curl -X POST \
     -H "Authorization: Bearer admin-token" \
     -H "Idempotency-Key: my-unique-key-123" \
     -H "Content-Type: application/json" \
     -d '[{"event":{...}, "version":"1"}]' \
     http://localhost:8080/push
```

Keys expire after 10 minutes (configurable).

## Compression

Responses larger than 1KB are automatically compressed:

```bash
# Server compresses response if client supports it
curl -H "Accept-Encoding: gzip" \
     -H "Authorization: Bearer admin-token" \
     http://localhost:8080/pull
```

## HMAC Signature Validation

For additional security, verify request integrity with HMAC:

```bash
# Calculate HMAC-SHA256 signature
REQUEST_BODY='{"data":"value"}'
SIGNATURE=$(echo -n "$REQUEST_BODY" | openssl dgst -sha256 -hmac "your-hmac-secret-key-change-in-production" -binary | base64)

# Include signature in request
curl -X POST \
     -H "Authorization: Bearer admin-token" \
     -H "X-HMAC-Signature: $SIGNATURE" \
     -H "Content-Type: application/json" \
     -d "$REQUEST_BODY" \
     http://localhost:8080/push
```

## Observability Hooks

The server logs key operations:

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "Pull request started",
  "user_id": "admin-user",
  "tenant": "acme-corp",
  "since": "4"
}

{
  "time": "2024-01-15T10:30:05Z",
  "level": "INFO",
  "msg": "Events committed",
  "user_id": "admin-user",
  "tenant": "acme-corp",
  "count": 2
}
```

## Error Handling

All errors return structured JSON responses:

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid authentication token",
    "details": {
      "timestamp": "2024-01-15T10:30:00Z"
    }
  }
}
```

Common error codes:
- `UNAUTHORIZED` - Missing or invalid auth token
- `FORBIDDEN` - Valid auth but insufficient permissions
- `BAD_REQUEST` - Malformed request body
- `PAYLOAD_TOO_LARGE` - Request exceeds size limit
- `INTERNAL_ERROR` - Server-side error

## Configuration

### Server Options

```go
serverOpts := &httptransport.ServerOptions{
    MaxRequestSize:       10 * 1024 * 1024,  // 10MB max request
    MaxDecompressedSize:  20 * 1024 * 1024,  // 20MB max decompressed
    CompressionEnabled:   true,               // Enable response compression
    CompressionThreshold: 1024,               // Compress if >1KB
    RequestTimeout:       30 * time.Second,   // 30s request timeout
    ShutdownTimeout:      10 * time.Second,   // 10s graceful shutdown
}
```

### HTTP Server

```go
srv := &http.Server{
    Addr:         ":8080",
    Handler:      handler,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

## Production Deployment

### 1. Environment Variables

```bash
export SERVER_ADDR=":8080"
export HMAC_SECRET="your-production-secret"
export DB_CONNECTION="postgres://user:pass@localhost/db"
export LOG_LEVEL="info"
```

### 2. Database Connection

Replace in-memory store with persistent storage:

```go
// Replace memstore with SQLite or Postgres
import "github.com/c0deZ3R0/go-sync-kit/storage/sqlstore"

db, err := sql.Open("postgres", os.Getenv("DB_CONNECTION"))
if err != nil {
    log.Fatal(err)
}

store := sqlstore.New(db)
```

### 3. JWT Authentication

```go
import "github.com/golang-jwt/jwt/v5"

authValidator := func(token string) (userID, tenantID string, err error) {
    claims := &jwt.RegisteredClaims{}
    _, err = jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
        return []byte(os.Getenv("JWT_SECRET")), nil
    })
    
    if err != nil {
        return "", "", fmt.Errorf("invalid JWT: %w", err)
    }
    
    return claims.Subject, claims.Audience[0], nil
}
```

### 4. Structured Logging

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
    AddSource: true,
}))
slog.SetDefault(logger)
```

### 5. TLS/HTTPS

```go
srv.ListenAndServeTLS("cert.pem", "key.pem")
```

## Testing

Run integration tests:

```bash
# Run all tests
go test ./... -v

# Run with race detection
go test ./... -race

# Run specific test
go test -run TestAuthenticationMiddleware
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP Request                         │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  HMAC Validator       │ (Optional)
         │  Verify signature     │
         └───────────┬───────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  Bearer Auth          │ (Required)
         │  Validate token       │
         │  Extract user/tenant  │
         └───────────┬───────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  Tenant Extractor     │
         │  Enrich context       │
         └───────────┬───────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │  Sync Handler         │
         │  - Parse request      │
         │  - Apply filters      │
         │  - Query store        │
         │  - Compress response  │
         └───────────┬───────────┘
                     │
                     ▼
┌────────────────────────────────────────────────────────────┐
│                       HTTP Response                        │
└────────────────────────────────────────────────────────────┘
```

## Related Examples

- **Simple HTTP Server**: Basic sync without authentication
- **WebSocket Server**: Real-time event streaming
- **gRPC Server**: High-performance binary protocol

## License

MIT
