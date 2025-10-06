package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain(t *testing.T) {
	// Create a simple handler that writes a response
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final"))
	})

	// Create middleware that appends to a header
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Middleware", "1")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Middleware", "2")
			next.ServeHTTP(w, r)
		})
	}

	middleware3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Middleware", "3")
			next.ServeHTTP(w, r)
		})
	}

	t.Run("NoMiddleware", func(t *testing.T) {
		handler := Chain(finalHandler)
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "final" {
			t.Errorf("Expected body 'final', got %s", rec.Body.String())
		}
	})

	t.Run("SingleMiddleware", func(t *testing.T) {
		handler := Chain(finalHandler, middleware1)
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		headers := rec.Header().Values("X-Middleware")
		if len(headers) != 1 || headers[0] != "1" {
			t.Errorf("Expected header [1], got %v", headers)
		}
	})

	t.Run("MultipleMiddleware", func(t *testing.T) {
		// Chain applies middleware in reverse order internally
		// So middleware1, middleware2, middleware3 means:
		// middleware3 wraps middleware2 wraps middleware1 wraps finalHandler
		// Execution order: 1 -> 2 -> 3 -> final
		handler := Chain(finalHandler, middleware1, middleware2, middleware3)
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		
		// Headers added in execution order: 1, 2, 3
		headers := rec.Header().Values("X-Middleware")
		if len(headers) != 3 {
			t.Errorf("Expected 3 headers, got %d", len(headers))
		}
		if headers[0] != "1" || headers[1] != "2" || headers[2] != "3" {
			t.Errorf("Expected headers [1, 2, 3], got %v", headers)
		}
	})

	t.Run("MiddlewareCanTerminate", func(t *testing.T) {
		// Middleware that terminates the chain
		terminatingMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("terminated"))
				// Don't call next.ServeHTTP
			})
		}

		// Chain: terminatingMiddleware wraps middleware1, but execution order is:
		// middleware1 runs first, then terminatingMiddleware (which terminates)
		// So we need terminatingMiddleware BEFORE middleware1 in the chain args
		handler := Chain(finalHandler, terminatingMiddleware, middleware1)
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
		if rec.Body.String() != "terminated" {
			t.Errorf("Expected body 'terminated', got %s", rec.Body.String())
		}
		
		// terminatingMiddleware runs and terminates, so middleware1 never runs
		headers := rec.Header().Values("X-Middleware")
		if len(headers) != 0 {
			t.Errorf("Expected no X-Middleware headers, got %v", headers)
		}
	})
}

func TestContextKey(t *testing.T) {
	// Test that context keys are unique strings
	if ContextKeyTenant == ContextKeyUserID {
		t.Error("ContextKeyTenant and ContextKeyUserID should be different")
	}
	
	if string(ContextKeyTenant) == "" {
		t.Error("ContextKeyTenant should not be empty")
	}
	
	if string(ContextKeyUserID) == "" {
		t.Error("ContextKeyUserID should not be empty")
	}
}
