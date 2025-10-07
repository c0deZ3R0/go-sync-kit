package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantExtractor(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value(ContextKeyTenant)

		w.WriteHeader(http.StatusOK)
		if tenantID != nil {
			w.Header().Set("X-Tenant-ID", tenantID.(string))
		}
	})

	t.Run("ExtractFromHeader", func(t *testing.T) {
		middleware := TenantExtractor("X-Tenant-ID")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Tenant-ID", "tenant123")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Header().Get("X-Tenant-ID") != "tenant123" {
			t.Errorf("Expected tenant ID 'tenant123', got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("NoTenantHeader", func(t *testing.T) {
		middleware := TenantExtractor("X-Tenant-ID")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Header().Get("X-Tenant-ID") != "" {
			t.Errorf("Expected no tenant ID, got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("TenantAlreadyInContext", func(t *testing.T) {
		// Create a middleware that sets tenant in context first
		authMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ContextKeyTenant, "auth-tenant")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}

		middleware := TenantExtractor("X-Tenant-ID")
		handler := Chain(testHandler, middleware, authMiddleware)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Tenant-ID", "header-tenant")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		// Should use the auth-set tenant, not the header
		if rec.Header().Get("X-Tenant-ID") != "auth-tenant" {
			t.Errorf("Expected tenant ID 'auth-tenant', got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("CustomHeaderName", func(t *testing.T) {
		middleware := TenantExtractor("X-Custom-Tenant")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Custom-Tenant", "custom-tenant")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Header().Get("X-Tenant-ID") != "custom-tenant" {
			t.Errorf("Expected tenant ID 'custom-tenant', got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("EmptyTenantHeader", func(t *testing.T) {
		middleware := TenantExtractor("X-Tenant-ID")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Tenant-ID", "")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		// Empty header should not set context value
		if rec.Header().Get("X-Tenant-ID") != "" {
			t.Errorf("Expected no tenant ID, got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})
}

func TestTenantExtractor_Integration(t *testing.T) {
	// Test integration with BearerAuth middleware
	validator := func(token string) (string, string, error) {
		if token == "with-tenant" {
			return "user123", "tenant-from-auth", nil
		}
		return "user456", "", nil
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(ContextKeyUserID)
		tenantID := r.Context().Value(ContextKeyTenant)

		w.WriteHeader(http.StatusOK)
		if userID != nil {
			w.Header().Set("X-User-ID", userID.(string))
		}
		if tenantID != nil {
			w.Header().Set("X-Tenant-ID", tenantID.(string))
		}
	})

	t.Run("AuthWithTenant_IgnoresHeader", func(t *testing.T) {
		// Chain: BearerAuth (sets tenant) -> TenantExtractor (should not override)
		handler := Chain(testHandler, TenantExtractor("X-Tenant-ID"), BearerAuth(validator))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer with-tenant")
		req.Header.Set("X-Tenant-ID", "tenant-from-header")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		// Should use tenant from auth, not header
		if rec.Header().Get("X-Tenant-ID") != "tenant-from-auth" {
			t.Errorf("Expected tenant ID 'tenant-from-auth', got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("AuthWithoutTenant_UsesHeader", func(t *testing.T) {
		handler := Chain(testHandler, TenantExtractor("X-Tenant-ID"), BearerAuth(validator))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer no-tenant")
		req.Header.Set("X-Tenant-ID", "tenant-from-header")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		// Should use tenant from header
		if rec.Header().Get("X-Tenant-ID") != "tenant-from-header" {
			t.Errorf("Expected tenant ID 'tenant-from-header', got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})
}
