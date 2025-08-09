package httptransport

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// validateContentType validates that the request Content-Type is appropriate for JSON endpoints
// Returns true if valid, false if invalid (and writes appropriate error response)
func validateContentType(w http.ResponseWriter, r *http.Request, options *ServerOptions) bool {
	// Only validate Content-Type for requests with a body (POST, PUT, PATCH)
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete {
		return true
	}

	// Get Content-Type header
	contentType := r.Header.Get("Content-Type")
	
	// If no Content-Type is provided, we'll be lenient and allow it
	// (some clients might not set it)
	if contentType == "" {
		return true
	}
	
	// Check if Content-Type starts with "application/json"
	// This allows for charset parameters like "application/json; charset=utf-8"
	if !strings.HasPrefix(contentType, "application/json") {
		respondWithError(w, r, http.StatusUnsupportedMediaType, "unsupported media type", options)
		return false
	}
	
	return true
}

// negotiateCompression determines if compression should be used based on client capabilities
func negotiateCompression(r *http.Request, options *ServerOptions, responseSize int) bool {
	if options == nil || !options.CompressionEnabled {
		return false
	}
	
	// Only compress responses above the threshold
	if responseSize < int(options.CompressionThreshold) {
		return false
	}
	
	// Check if client accepts gzip compression
	acceptEncoding := r.Header.Get("Accept-Encoding")
	return strings.Contains(strings.ToLower(acceptEncoding), "gzip")
}

// respondWithJSON responds to an HTTP request with a JSON payload
func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}, options *ServerOptions) {
	response, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, r, http.StatusInternalServerError, "failed to marshal response", options)
		return
	}

	// Use the helper function to determine if compression should be used
	useCompression := negotiateCompression(r, options, len(response))

	w.Header().Set("Content-Type", "application/json")
	
	if useCompression {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(code)
		
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gz.Write(response)
	} else {
		w.WriteHeader(code)
		w.Write(response)
	}
}

// respondWithError responds to an HTTP request with an error message
func respondWithError(w http.ResponseWriter, r *http.Request, code int, message string, options *ServerOptions) {
	respondWithJSON(w, r, code, map[string]string{"error": message}, options)
}

// errDecompressedTooLarge is returned when decompressed body exceeds the limit
var errDecompressedTooLarge = errors.New("decompressed body exceeds limit")

// maxDecompressedReader wraps an io.Reader and returns errDecompressedTooLarge
// when the number of bytes read exceeds the specified limit
type maxDecompressedReader struct {
	r io.Reader
	n int64
}

func (m *maxDecompressedReader) Read(p []byte) (int, error) {
	if m.n <= 0 {
		return 0, errDecompressedTooLarge
	}
	if int64(len(p)) > m.n {
		p = p[:m.n]
	}
	n, err := m.r.Read(p)
	m.n -= int64(n)
	if m.n <= 0 && err == nil {
		// Next Read will return errDecompressedTooLarge; this read succeeds.
		// Do not spuriously fail a successful read.
	}
	return n, err
}

func newMaxDecompressedReader(r io.Reader, limit int64) io.Reader {
	return &maxDecompressedReader{r: r, n: limit}
}

// errResponseDecompressedTooLarge is returned when decompressed response body exceeds the limit
var errResponseDecompressedTooLarge = errors.New("response decompressed body exceeds limit")

// createSafeResponseReader wraps resp.Body first with a compressed size limit,
// then conditionally a gzip.Reader and a max-decompressed limiter.
// Returns the reader and a cleanup func to close any internal readers.
func createSafeResponseReader(resp *http.Response, opts *ClientOptions) (io.Reader, func(), error) {
	cleanup := func() { _ = resp.Body.Close() }

	// Enforce compressed (on-the-wire) limit
	limited := io.LimitReader(resp.Body, opts.MaxResponseSize)
	var r io.Reader = limited

	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(limited)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		// Ensure both readers get closed
		oldCleanup := cleanup
		cleanup = func() {
			_ = gz.Close()
			oldCleanup()
		}
		// Enforce decompressed limit with a separate sentinel error for responses
		r = &maxResponseDecompressedReader{r: gz, n: opts.MaxDecompressedResponseSize}
	}

	return r, cleanup, nil
}

// maxResponseDecompressedReader wraps an io.Reader and returns errResponseDecompressedTooLarge
// when the number of bytes read exceeds the specified limit
type maxResponseDecompressedReader struct {
	r io.Reader
	n int64
}

func (m *maxResponseDecompressedReader) Read(p []byte) (int, error) {
	if m.n <= 0 {
		return 0, errResponseDecompressedTooLarge
	}
	if int64(len(p)) > m.n {
		p = p[:m.n]
	}
	n, err := m.r.Read(p)
	m.n -= int64(n)
	if m.n <= 0 && err == nil {
		// Next Read will return errResponseDecompressedTooLarge; this read succeeds.
		// Do not spuriously fail a successful read.
	}
	return n, err
}

// createSafeRequestReader wraps r.Body first with MaxBytesReader (compressed limit),
// then conditionally a gzip.Reader and a max-decompressed limiter.
// Returns the reader and a cleanup func to close any internal readers.
func createSafeRequestReader(
	w http.ResponseWriter,
	r *http.Request,
	opts *ServerOptions,
) (io.Reader, func(), error) {
	// Enforce compressed (on-the-wire) limit
	limited := http.MaxBytesReader(w, r.Body, opts.MaxRequestSize)
	cleanup := func() { _ = limited.Close() }

	enc := r.Header.Get("Content-Encoding")
	if strings.EqualFold(enc, "gzip") {
		gz, err := gzip.NewReader(limited)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		// Ensure both readers get closed
		oldCleanup := cleanup
		cleanup = func() {
			_ = gz.Close()
			oldCleanup()
		}
		// Enforce decompressed limit
		return newMaxDecompressedReader(gz, opts.MaxDecompressedSize), cleanup, nil
	}

	return limited, cleanup, nil
}
