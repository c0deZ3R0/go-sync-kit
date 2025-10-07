package middleware

import (
	"context"
	"net/http"
	"strings"
)

// TokenValidator validates bearer tokens and returns user/tenant information.
// It should return the user ID, optional tenant ID, and any error encountered.
// If the token is invalid, it should return an error.
type TokenValidator func(token string) (userID, tenantID string, err error)

// BearerAuth creates middleware that validates bearer tokens in the Authorization header.
// It extracts the token, validates it using the provided validator function,
// and adds the user ID and tenant ID (if present) to the request context.
//
// The middleware expects the Authorization header to be in the format:
//   Authorization: Bearer <token>
//
// On successful validation, the following values are added to the context:
//   - ContextKeyUserID: The authenticated user's ID
//   - ContextKeyTenant: The user's tenant ID (if provided by validator)
//
// If authentication fails, the middleware responds with HTTP 401 Unauthorized
// and does not call the next handler.
func BearerAuth(validator TokenValidator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Authorization header
			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Parse Bearer token
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			if token == "" {
				http.Error(w, "empty bearer token", http.StatusUnauthorized)
				return
			}

			// Validate token
			userID, tenantID, err := validator(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Add user and tenant to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyUserID, userID)
			if tenantID != "" {
				ctx = context.WithValue(ctx, ContextKeyTenant, tenantID)
			}

			// Call next handler with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
