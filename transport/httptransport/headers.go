package httptransport

import "net/http"

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
// Priority: 1) X-SyncKit-Tenant header, 2) 'tenant' query parameter
func ExtractTenant(r *http.Request) string {
	// Try header first (preferred method)
	if tenant := r.Header.Get(HeaderSyncKitTenant); tenant != "" {
		return tenant
	}
	// Fall back to query param for backward compatibility
	return r.URL.Query().Get("tenant")
}
