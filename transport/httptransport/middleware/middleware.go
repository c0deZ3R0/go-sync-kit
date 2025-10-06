package middleware

import (
	"net/http"
)

// Middleware wraps an http.Handler with additional functionality
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware in order (last to first).
// The first middleware in the list will be the outermost handler.
//
// Example:
//   handler := Chain(myHandler, Logger, Auth, RateLimit)
//   // Execution order: RateLimit -> Auth -> Logger -> myHandler
func Chain(h http.Handler, middleware ...Middleware) http.Handler {
	// Apply middleware in reverse order so the first middleware
	// in the list is the outermost handler
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// ContextKey is a typed key for context values to avoid collisions
type ContextKey string

const (
	// ContextKeyTenant stores the tenant ID in the request context
	ContextKeyTenant ContextKey = "tenant"

	// ContextKeyUserID stores the authenticated user ID in the request context
	ContextKeyUserID ContextKey = "user_id"
)
