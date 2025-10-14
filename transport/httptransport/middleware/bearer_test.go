package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth(t *testing.T) {
	// Mock token validator
	validTokenValidator := func(token string) (userID, tenantID string, err error) {
		if token == "valid-token" {
			return "user123", "tenant456", nil
		}
		if token == "valid-token-no-tenant" {
			return "user789", "", nil
		}
		return "", "", errors.New("invalid token")
	}

	// Test handler that checks context values
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(ContextKeyUserID)
		tenantID := r.Context().Value(ContextKeyTenant)

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("authenticated")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}

		// Store context values in headers for test verification
		if userID != nil {
			w.Header().Set("X-User-ID", userID.(string))
		}
		if tenantID != nil {
			w.Header().Set("X-Tenant-ID", tenantID.(string))
		}
	})

	t.Run("ValidToken", func(t *testing.T) {
		middleware := BearerAuth(validTokenValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "authenticated" {
			t.Errorf("Expected body 'authenticated', got %s", rec.Body.String())
		}
		if rec.Header().Get("X-User-ID") != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", rec.Header().Get("X-User-ID"))
		}
		if rec.Header().Get("X-Tenant-ID") != "tenant456" {
			t.Errorf("Expected tenant ID 'tenant456', got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("ValidTokenNoTenant", func(t *testing.T) {
		middleware := BearerAuth(validTokenValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token-no-tenant")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Header().Get("X-User-ID") != "user789" {
			t.Errorf("Expected user ID 'user789', got %s", rec.Header().Get("X-User-ID"))
		}
		if rec.Header().Get("X-Tenant-ID") != "" {
			t.Errorf("Expected no tenant ID, got %s", rec.Header().Get("X-Tenant-ID"))
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		middleware := BearerAuth(validTokenValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
		if rec.Body.String() != "invalid token\n" {
			t.Errorf("Expected error message, got %s", rec.Body.String())
		}
	})

	t.Run("MissingAuthorizationHeader", func(t *testing.T) {
		middleware := BearerAuth(validTokenValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
		if rec.Body.String() != "missing authorization header\n" {
			t.Errorf("Unexpected error message: %s", rec.Body.String())
		}
	})

	t.Run("InvalidAuthorizationFormat", func(t *testing.T) {
		middleware := BearerAuth(validTokenValidator)
		handler := middleware(testHandler)

		tests := []struct {
			name   string
			header string
		}{
			{"No Bearer prefix", "valid-token"},
			{"Wrong auth type", "Basic valid-token"},
			{"Multiple spaces", "Bearer  valid-token"},
			{"Trailing space", "Bearer valid-token "},
			{"Empty token", "Bearer "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Authorization", tt.header)
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("Expected status 401 for %s, got %d", tt.name, rec.Code)
				}
			})
		}
	})

	t.Run("EmptyBearerToken", func(t *testing.T) {
		middleware := BearerAuth(validTokenValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})
}

func TestBearerAuth_ValidatorErrors(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("ValidatorReturnsError", func(t *testing.T) {
		errorValidator := func(token string) (string, string, error) {
			return "", "", errors.New("database connection failed")
		}

		middleware := BearerAuth(errorValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer any-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("ValidatorPanics", func(t *testing.T) {
		panicValidator := func(token string) (string, string, error) {
			panic("validator panic")
		}

		middleware := BearerAuth(panicValidator)
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer any-token")
		rec := httptest.NewRecorder()

		// Should panic
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic, but didn't get one")
			}
		}()

		handler.ServeHTTP(rec, req)
	})
}
