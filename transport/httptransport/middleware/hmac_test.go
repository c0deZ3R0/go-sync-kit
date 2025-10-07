package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHMACValidator(t *testing.T) {
	secret := []byte("test-secret-key")

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body to verify it was restored correctly
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read body in handler: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
		w.Header().Set("X-Body-Length", string(rune(len(body))))
	})

	// Helper function to compute HMAC signature
	computeSignature := func(body []byte, secret []byte) string {
		mac := hmac.New(sha256.New, secret)
		mac.Write(body)
		return hex.EncodeToString(mac.Sum(nil))
	}

	t.Run("ValidSignature", func(t *testing.T) {
		body := []byte(`{"data":"test"}`)
		signature := computeSignature(body, secret)

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "authenticated" {
			t.Errorf("Expected body 'authenticated', got %s", rec.Body.String())
		}
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		body := []byte(`{"data":"test"}`)
		wrongSignature := "invalid-signature"

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Signature", wrongSignature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "invalid signature") {
			t.Errorf("Expected error message about invalid signature, got %s", rec.Body.String())
		}
	})

	t.Run("MissingSignatureHeader", func(t *testing.T) {
		body := []byte(`{"data":"test"}`)

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "missing signature header") {
			t.Errorf("Expected error message about missing signature, got %s", rec.Body.String())
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		body := []byte{}
		signature := computeSignature(body, secret)

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("LargeBody", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), 10000)
		signature := computeSignature(body, secret)

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("BodyIsRestoredForHandler", func(t *testing.T) {
		expectedBody := []byte(`{"important":"data"}`)
		signature := computeSignature(expectedBody, secret)

		bodyChecker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actualBody, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("Failed to read body: %v", err)
			}
			if !bytes.Equal(actualBody, expectedBody) {
				t.Errorf("Body not restored correctly. Expected %s, got %s", expectedBody, actualBody)
			}
			w.WriteHeader(http.StatusOK)
		})

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(bodyChecker)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(expectedBody))
		req.Header.Set("X-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("CustomHeaderName", func(t *testing.T) {
		body := []byte(`{"data":"test"}`)
		signature := computeSignature(body, secret)

		middleware := HMACValidator(secret, "X-Custom-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Custom-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("WrongSecret", func(t *testing.T) {
		body := []byte(`{"data":"test"}`)
		wrongSecret := []byte("wrong-secret")
		signature := computeSignature(body, wrongSecret)

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("ModifiedBody", func(t *testing.T) {
		originalBody := []byte(`{"data":"original"}`)
		modifiedBody := []byte(`{"data":"modified"}`)
		signature := computeSignature(originalBody, secret)

		middleware := HMACValidator(secret, "X-Signature")
		handler := middleware(testHandler)

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(modifiedBody))
		req.Header.Set("X-Signature", signature)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for modified body, got %d", rec.Code)
		}
	})
}

func TestHMACValidator_Integration(t *testing.T) {
	secret := []byte("shared-secret")

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(ContextKeyUserID)
		body, _ := io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
		if userID != nil {
			w.Header().Set("X-User-ID", userID.(string))
		}
		w.Write(body)
	})

	// Test HMAC with other middleware
	t.Run("HMACWithBearerAuth", func(t *testing.T) {
		validator := func(token string) (string, string, error) {
			if token == "valid" {
				return "user123", "", nil
			}
			return "", "", nil
		}

		body := []byte(`{"request":"data"}`)
		mac := hmac.New(sha256.New, secret)
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))

		// Chain: HMACValidator -> BearerAuth -> handler
		handler := Chain(testHandler, BearerAuth(validator), HMACValidator(secret, "X-Signature"))

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("X-Signature", signature)
		req.Header.Set("Authorization", "Bearer valid")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Header().Get("X-User-ID") != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", rec.Header().Get("X-User-ID"))
		}
		if rec.Body.String() != string(body) {
			t.Errorf("Expected body %s, got %s", body, rec.Body.String())
		}
	})
}
