package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// HMACValidator creates middleware that validates HMAC-SHA256 signatures on request bodies.
// This is useful for webhook endpoints or API requests that need to verify the authenticity
// of incoming requests.
//
// The middleware:
//   1. Reads the signature from the specified header
//   2. Reads and buffers the entire request body
//   3. Computes the HMAC-SHA256 of the body using the provided secret
//   4. Compares the computed signature with the provided signature
//   5. Restores the request body for the next handler
//
// Parameters:
//   - secret: The shared secret key used to compute HMAC signatures
//   - headerName: The HTTP header containing the HMAC signature (e.g., "X-Signature")
//
// The signature in the header should be the hex-encoded HMAC-SHA256 of the request body.
//
// If validation fails, the middleware responds with HTTP 401 Unauthorized
// and does not call the next handler.
//
// Note: This middleware buffers the entire request body in memory for verification.
// For large request bodies, consider implementing size limits upstream or using
// a streaming verification approach.
func HMACValidator(secret []byte, headerName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get signature from header
			providedSignature := r.Header.Get(headerName)
			if providedSignature == "" {
				http.Error(w, "missing signature header", http.StatusUnauthorized)
				return
			}

			// Read the entire request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}
			// Close the original body
			if err := r.Body.Close(); err != nil {
				http.Error(w, "failed to close request body", http.StatusInternalServerError)
				return
			}

			// Compute HMAC-SHA256 of the body
			mac := hmac.New(sha256.New, secret)
			mac.Write(body)
			expectedSignature := hex.EncodeToString(mac.Sum(nil))

			// Constant-time comparison to prevent timing attacks
			if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}

			// Restore the body for the next handler
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Signature is valid, proceed to next handler
			next.ServeHTTP(w, r)
		})
	}
}
