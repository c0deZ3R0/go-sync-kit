package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChain(t *testing.T) {
	// Create tracking middleware to verify execution order
	var executionOrder []string

	makeTrackerMiddleware := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionOrder = append(executionOrder, name+"-before")
				next.ServeHTTP(w, r)
				executionOrder = append(executionOrder, name+"-after")
			})
		}
	}

	t.Run("SingleMiddleware", func(t *testing.T) {
		executionOrder = nil

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "handler")
			w.WriteHeader(http.StatusOK)
		})

		chained := Chain(handler, makeTrackerMiddleware("m1"))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		chained.ServeHTTP(rec, req)

		expected := []string{"m1-before", "handler", "m1-after"}
		if !equalSlices(executionOrder, expected) {
			t.Errorf("execution order = %v, want %v", executionOrder, expected)
		}
	})

	t.Run("MultipleMiddleware", func(t *testing.T) {
		executionOrder = nil

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "handler")
			w.WriteHeader(http.StatusOK)
		})

		chained := Chain(handler,
			makeTrackerMiddleware("m1"),
			makeTrackerMiddleware("m2"),
			makeTrackerMiddleware("m3"),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		chained.ServeHTTP(rec, req)

		// Middleware are applied in reverse order (last wraps first)
		// So execution order is: m1 -> m2 -> m3 -> handler
		expected := []string{
			"m1-before",
			"m2-before",
			"m3-before",
			"handler",
			"m3-after",
			"m2-after",
			"m1-after",
		}
		if !equalSlices(executionOrder, expected) {
			t.Errorf("execution order = %v, want %v", executionOrder, expected)
		}
	})

	t.Run("NoMiddleware", func(t *testing.T) {
		executionOrder = nil

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "handler")
			w.WriteHeader(http.StatusOK)
		})

		chained := Chain(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		chained.ServeHTTP(rec, req)

		expected := []string{"handler"}
		if !equalSlices(executionOrder, expected) {
			t.Errorf("execution order = %v, want %v", executionOrder, expected)
		}
	})

	t.Run("MiddlewareShortCircuit", func(t *testing.T) {
		executionOrder = nil

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "handler")
			w.WriteHeader(http.StatusOK)
		})

		// Middleware that stops the chain
		stopMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionOrder = append(executionOrder, "stop-before")
				w.WriteHeader(http.StatusUnauthorized)
				// Don't call next.ServeHTTP - short circuit
				executionOrder = append(executionOrder, "stop-after")
			})
		}

		chained := Chain(handler,
			makeTrackerMiddleware("m1"),
			stopMiddleware,
			makeTrackerMiddleware("m2"),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		chained.ServeHTTP(rec, req)

		// m1 runs, then stop middleware terminates, preventing handler
		// m2 is outermost but never runs because stop terminates the chain
		expected := []string{
			"m1-before",
			"stop-before",
			"stop-after",
			"m1-after",
		}
		if !equalSlices(executionOrder, expected) {
			t.Errorf("execution order = %v, want %v", executionOrder, expected)
		}

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestChain_RealMiddleware(t *testing.T) {
	// Test chaining real middleware implementations
	validator := func(token string) (string, string, error) {
		if token == "valid" {
			return "user123", "tenant456", nil
		}
		return "", "", http.ErrNoCookie
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, hasUser := UserIDFromContext(r.Context())
		tenantID, hasTenant := TenantFromContext(r.Context())

		w.WriteHeader(http.StatusOK)
		if hasUser {
			w.Header().Set("X-User-ID", userID)
		}
		if hasTenant {
			w.Header().Set("X-Tenant-ID", tenantID)
		}
	})

	t.Run("AuthAndTenantChain", func(t *testing.T) {
		chained := Chain(handler,
			TenantExtractor("X-Tenant-ID"),
			BearerAuth(validator),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid")
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		if got := rec.Header().Get("X-User-ID"); got != "user123" {
			t.Errorf("expected user ID 'user123', got %q", got)
		}

		if got := rec.Header().Get("X-Tenant-ID"); got != "tenant456" {
			t.Errorf("expected tenant ID 'tenant456', got %q", got)
		}
	})

	t.Run("AuthFailureStopsChain", func(t *testing.T) {
		chained := Chain(handler,
			TenantExtractor("X-Tenant-ID"),
			BearerAuth(validator),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid")
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req)

		// Auth should fail and stop chain
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}

		// Handler should not have executed
		if rec.Header().Get("X-User-ID") != "" {
			t.Error("handler executed despite auth failure")
		}
	})

	t.Run("TenantHeaderOverriddenByAuth", func(t *testing.T) {
		chained := Chain(handler,
			TenantExtractor("X-Tenant-ID"),
			BearerAuth(validator),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid")
		req.Header.Set("X-Tenant-ID", "header-tenant")
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		// Auth-provided tenant should override header
		if got := rec.Header().Get("X-Tenant-ID"); got != "tenant456" {
			t.Errorf("expected tenant from auth 'tenant456', got %q", got)
		}
	})
}

func TestChain_ContextPropagation(t *testing.T) {
	// Test that context values propagate correctly through the chain
	var capturedValues []string

	captureMiddleware := func(key, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Capture any existing values before adding ours
				if val := r.Context().Value(key); val != nil {
					capturedValues = append(capturedValues, "found:"+val.(string))
				}

				// Add our value
				ctx := r.Context()
				ctx = contextWithValue(ctx, key, value)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture final context state
		if val := r.Context().Value("key1"); val != nil {
			capturedValues = append(capturedValues, "handler:key1:"+val.(string))
		}
		if val := r.Context().Value("key2"); val != nil {
			capturedValues = append(capturedValues, "handler:key2:"+val.(string))
		}
		w.WriteHeader(http.StatusOK)
	})

	capturedValues = nil
	chained := Chain(handler,
		captureMiddleware("key1", "value1"),
		captureMiddleware("key2", "value2"),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	chained.ServeHTTP(rec, req)

	// Should see both values in handler
	if !contains(capturedValues, "handler:key1:value1") {
		t.Error("key1 not found in handler context")
	}
	if !contains(capturedValues, "handler:key2:value2") {
		t.Error("key2 not found in handler context")
	}
}

// Helper functions
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item || strings.Contains(s, item) {
			return true
		}
	}
	return false
}

func contextWithValue(ctx context.Context, key, value string) context.Context {
	return context.WithValue(ctx, key, value) //nolint:staticcheck // Using string keys in test helper is acceptable
}
