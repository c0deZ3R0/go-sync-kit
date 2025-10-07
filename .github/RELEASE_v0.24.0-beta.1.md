# v0.24.0-beta.1: Enterprise HTTP Transport Features [PRE-RELEASE]

Complete enterprise-grade HTTP transport with authentication, multitenancy, idempotency, security tests, and production-ready examples.

## ✨ Highlights
- **Enterprise HTTP Example** - Production-ready server and client with Bearer token auth, multitenancy, and idempotency (570+ lines)
- **Automated Demo Script** - PowerShell script showcasing all 5 enterprise features working end-to-end
- **Comprehensive Security Tests** - 20+ security scenarios including decompression bombs, header injection, path traversal, and JSON attacks
- **Middleware Ecosystem** - Bearer token, tenant extraction, and HMAC validation middleware with chaining support
- **Complete Documentation** - 408-line README with production deployment guide, JWT migration, architecture diagrams

## What's Changed

### Enterprise Features
- `feat(http)`: add structured error responses with error codes and operations
- `feat(http)`: add query parameter parsing for filtering (type, tenant, aggregate_id, limit)
- `feat(storage)`: add filtering support to EventStore interface with variadic filters
- `feat(http)`: add multitenancy header support with automatic tenant isolation
- `feat(http)`: add idempotency key tracking with 10-minute expiration and caching
- `feat(middleware)`: implement Bearer token authentication middleware
- `feat(middleware)`: implement tenant extractor middleware
- `feat(middleware)`: implement HMAC signature validation middleware

### Examples & Documentation
- `feat(examples)`: add enterprise HTTP server with auth, multitenancy, hooks (285 lines)
- `feat(examples)`: add enterprise HTTP client with 5 working examples (302 lines)
- `feat(examples)`: add PowerShell demo script for automated testing (81 lines)
- `docs(http)`: add complete HTTP API specification (docs/http-spec.md)
- `docs(http)`: add migration guide (docs/MIGRATION_GUIDE_HTTP.md)
- `docs`: add production disclaimer to README with breaking changes warning

### Testing & Quality
- `test(http)`: add comprehensive integration test suite (845 lines, 35+ scenarios)
- `test(http)`: add security test suite (503 lines, 20+ security tests)
- `test(http)`: add backward compatibility tests for v0.23 clients
- `test(http)`: add performance benchmarks for filtering and compression
- `fix(http)`: resolve client nil pointer in enterprise example
- `fix(http)`: fix integration test race conditions with t.Parallel()

**Full Changelog:** https://github.com/c0deZ3R0/go-sync-kit/compare/v0.23.0...v0.24.0-beta.1

## ⚙️ Compatibility

**Backward Compatible** - All changes are additive and optional. Existing v0.23 code continues to work without modifications.

- ✅ Variadic `filters ...Filter` parameters are backward compatible (calls without filters work unchanged)
- ✅ New middleware is opt-in via `middleware.Chain()` 
- ✅ Structured errors maintain HTTP status codes for compatibility
- ✅ All v0.23 clients tested and verified working against v0.24 servers
- ✅ No API removals or signature changes

## ⚠️ Breaking Changes

**None** - This is a fully backward-compatible release. All new features are optional enhancements.

## 🔁 Migration

**No migration required** for existing code. All changes are opt-in enhancements.

### To adopt new features:

**1. Add Authentication Middleware (Optional)**
```go
import "github.com/c0deZ3R0/go-sync-kit/transport/httptransport/middleware"

authValidator := func(token string) (userID, tenantID string, err error) {
    return validateJWT(token) // Your JWT validation
}

handler := middleware.Chain(
    baseHandler,
    middleware.TenantExtractor("X-Tenant-ID"),
    middleware.BearerAuth(authValidator),
)
```

**2. Use Event Filtering (Optional)**
```go
// Pull with filters
events, err := store.Load(ctx, since, 
    synckit.Filter{Key: "type", Value: "OrderCreated"},
    synckit.Filter{Key: "tenant", Value: "acme-corp"},
)
```

**3. Add Idempotency Keys (Optional)**
```go
req.Header.Set("Idempotency-Key", uuid.New().String())
```

## 🧪 Testing

```bash
# Run full test suite
go test ./... -v

# Run with race detection
go test ./... -race

# Run integration tests
go test ./transport/httptransport/... -v

# Run security tests
go test ./transport/httptransport -run Security -v

# Try the enterprise demo (Windows)
cd examples/http_enterprise
.\demo.ps1

# Try the enterprise demo (Linux/Mac)
cd examples/http_enterprise
go run server/main.go &
sleep 2
go run client/main.go
```

**Expected Results:**
- ✅ All package tests pass (100%)
- ✅ Integration tests: 35+ scenarios pass
- ✅ Security tests: 20+ scenarios pass
- ✅ Enterprise demo: All 5 examples working
  - Example 1: 6 events pulled with authentication
  - Example 2: 4 OrderCreated events filtered
  - Example 3: Both tenants see isolated events
  - Example 4: Idempotency prevents duplicates
  - Example 5: Full sync: 6 pulled, 0 conflicts

## 📦 Install / Upgrade

```bash
# Install pre-release
go get github.com/c0deZ3R0/go-sync-kit@v0.24.0-beta.1

# Or use latest stable
go get github.com/c0deZ3R0/go-sync-kit@v0.23.0
```

**Production Recommendation:**
```go
// Pin to specific version in go.mod
require github.com/c0deZ3R0/go-sync-kit v0.24.0-beta.1
```

## 📈 Observability / Monitoring

**Server-Side Hooks:**
```go
hooks := &httptransport.SyncHooks{
    BeforePull: func(ctx context.Context, since synckit.Version) {
        userID, _ := middleware.UserIDFromContext(ctx)
        tenant, _ := middleware.TenantFromContext(ctx)
        log.Info("pull_started", "user", userID, "tenant", tenant)
    },
    AfterCommit: func(ctx context.Context, events []synckit.EventWithVersion) {
        log.Info("events_committed", "count", len(events))
    },
}
```

**Metrics to Monitor:**
- `http_requests_total{endpoint="/pull"}` - Pull request rate
- `http_requests_total{endpoint="/push"}` - Push request rate
- `idempotency_cache_hits_total` - Duplicate prevention effectiveness
- `authentication_failures_total` - Auth issues
- `tenant_events_total{tenant="..."}` - Per-tenant activity
- `filter_queries_duration_seconds` - Filtering performance

**Error Codes to Alert On:**
- `INVALID_CURSOR` - Client cursor/version issues
- `AUTH_REQUIRED` - Authentication failures
- `INVALID_TENANT` - Tenant isolation violations
- `PAYLOAD_TOO_LARGE` - Size limit violations
- `INTERNAL_ERROR` - Server-side errors

## 🧳 Examples / Docs

### Enterprise HTTP Example (NEW)
**Location:** `examples/http_enterprise/`

**Quick Start:**
```bash
cd examples/http_enterprise
.\demo.ps1  # Automated demo on Windows
```

**What It Demonstrates:**
1. ✅ Pull with Bearer token authentication (6 events)
2. ✅ Pull with client-side filtering (4 OrderCreated)
3. ✅ Multitenancy isolation (acme-corp vs globex-inc)
4. ✅ Idempotency key handling (duplicate prevention)
5. ✅ Full bidirectional sync (6 events synced)

**Documentation:**
- 📖 [Enterprise Example README](examples/http_enterprise/README.md) - Complete guide with curl examples
- 📖 [HTTP API Specification](docs/http-spec.md) - Full API reference
- 📖 [Migration Guide](docs/MIGRATION_GUIDE_HTTP.md) - Upgrade instructions
- 📖 [Implementation Plan](IMPLEMENTATION_PLAN_HTTP_ENHANCEMENTS.md) - Phase 8 complete

### Other Resources:
- 📖 [Main README](README.md) - Now includes production disclaimer
- 📖 [Basic HTTP Examples](examples/HTTP_EXAMPLES.md) - Simplified patterns
- 💻 [Server Code](examples/http_enterprise/server/main.go) - 285 lines
- 💻 [Client Code](examples/http_enterprise/client/main.go) - 302 lines

## ⚠️ Pre-release Notice

**This is a beta/pre-release version** seeking production feedback before the stable v0.24.0 release.

### Why Beta?

While all tests pass and the examples work well:
- ✅ **Feature complete** - All planned Phase 8 features implemented
- ✅ **Well tested** - 35+ integration tests, 20+ security tests passing
- ✅ **Backward compatible** - No breaking changes, all optional
- ⚠️ **Limited production usage** - Needs real-world validation
- ⚠️ **Performance unknown at scale** - Not tested with high load
- ⚠️ **Security not audited** - Middleware patterns need expert review

### Feedback Requested:

Please test and report on:

1. **Authentication Patterns** - Does Bearer token + middleware meet your needs?
2. **Multitenancy Isolation** - Is tenant filtering sufficient for your use case?
3. **Idempotency Keys** - Does 10-minute expiration work for your workflows?
4. **Performance** - How does filtering perform with your data volumes?
5. **Security** - Any vulnerabilities in the middleware chain?
6. **API Ergonomics** - Is the middleware.Chain() pattern intuitive?
7. **Documentation** - Are the examples clear and production-ready?

### How to Provide Feedback:

- 🐛 **Bug Reports:** https://github.com/c0deZ3R0/go-sync-kit/issues
- 💡 **Feature Requests:** Label with `enhancement`
- 📊 **Performance Data:** Share benchmarks or profiling results
- 🔒 **Security Issues:** Email privately or use GitHub Security tab
- 💬 **General Feedback:** Comment on PR #64

### Timeline to Stable:

- **Beta Period:** 2-4 weeks for feedback collection
- **Stable Release:** After addressing critical feedback
- **v0.24.0:** Target mid-late January 2025

### Production Use:

If you deploy this beta to production:
- ✅ Pin this exact version in `go.mod`
- ✅ Test thoroughly in staging first
- ✅ Monitor error rates and performance
- ✅ Have rollback plan to v0.23.0
- ✅ Report issues immediately

---

## 🎯 Try It Now

```bash
# Quick test
git clone https://github.com/c0deZ3R0/go-sync-kit.git
cd go-sync-kit
git checkout v0.24.0-beta.1
cd examples/http_enterprise
.\demo.ps1

# Expected: All 5 examples pass with colored output
```

## 💬 Questions?

- 📖 Documentation: `examples/http_enterprise/README.md`
- 💻 Example code: `examples/http_enterprise/client/main.go`
- 🐛 Report issues: https://github.com/c0deZ3R0/go-sync-kit/issues
- 💬 PR Discussion: https://github.com/c0deZ3R0/go-sync-kit/pull/64

---

**Released:** January 7, 2025  
**Pre-release:** Beta 1  
**Target Stable:** Mid-late January 2025  
**Contributors:** @c0deZ3R0 and community  
**License:** MIT
