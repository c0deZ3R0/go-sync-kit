package middleware

import (
	"context"
	"net/http"
)

// TenantExtractor creates middleware that extracts tenant information from HTTP headers.
// If a tenant ID is already present in the request context (e.g., from authentication middleware),
// it will not be overwritten. Otherwise, it attempts to extract the tenant ID from the specified header.
//
// This middleware is useful for multitenancy scenarios where the tenant ID can be provided
// either through authentication (JWT claims, etc.) or directly via a custom header.
//
// Parameters:
//   - headerName: The HTTP header name to extract the tenant ID from (e.g., "X-SyncKit-Tenant")
//
// The extracted tenant ID is stored in the context under ContextKeyTenant.
func TenantExtractor(headerName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Check if tenant is already in context (from auth middleware)
			if existingTenant := ctx.Value(ContextKeyTenant); existingTenant != nil {
				// Tenant already set by authentication, don't override
				next.ServeHTTP(w, r)
				return
			}

			// Extract tenant from header
			if tenant := r.Header.Get(headerName); tenant != "" {
				ctx = context.WithValue(ctx, ContextKeyTenant, tenant)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
