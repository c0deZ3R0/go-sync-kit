package httptransport

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTP status codes according to review requirements:
// - gzip invalid → 400 Bad Request
// - compressed limit exceeded → 413 Request Entity Too Large
// - decompressed limit exceeded → 413 Request Entity Too Large (via errDecompressedTooLarge)
// - unsupported media type → 415 Unsupported Media Type
// - version parse failure → 400 Bad Request (keep consistent with existing tests)

// errDecompressedTooLarge is a sentinel error for decompressed size limit violations
var errDecompressedTooLarge = errors.New("decompressed data exceeds maximum size limit")

// maxDecompressedReader wraps an io.Reader to enforce decompressed size limits
type maxDecompressedReader struct {
	reader   io.Reader
	limit    int64
	consumed int64
}

func (r *maxDecompressedReader) Read(p []byte) (int, error) {
	if r.consumed >= r.limit {
		return 0, errDecompressedTooLarge
	}

	// Limit read size to prevent exceeding limit
	maxRead := r.limit - r.consumed
	if int64(len(p)) > maxRead {
		p = p[:maxRead]
	}

	n, err := r.reader.Read(p)
	r.consumed += int64(n)

	if r.consumed >= r.limit && err == nil {
		// We've hit the limit but there might be more data
		// Try to read one more byte to trigger the limit error
		var dummy [1]byte
		if _, peekErr := r.reader.Read(dummy[:]); peekErr == nil {
			return n, errDecompressedTooLarge
		}
	}

	return n, err
}

// createSafeRequestReader creates a reader that enforces both compressed and decompressed size limits
// Returns the reader, cleanup function, and error
func createSafeRequestReader(w http.ResponseWriter, r *http.Request, options *ServerOptions) (io.Reader, func(), error) {
	// Set defaults
	maxRequestSize := options.MaxRequestSize
	if maxRequestSize == 0 {
		maxRequestSize = 10 * 1024 * 1024 // 10MB default
	}

	maxDecompressedSize := options.MaxDecompressedSize
	if maxDecompressedSize == 0 {
		maxDecompressedSize = 20 * 1024 * 1024 // 20MB default
	}

	// Check Content-Type for unsupported media types (415)
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		return nil, func() {}, fmt.Errorf("unsupported media type: %s", contentType)
	}

	// Check Content-Length if available (413 for compressed size)
	if r.ContentLength > 0 && r.ContentLength > maxRequestSize {
		return nil, func() {}, fmt.Errorf("compressed request body too large: %d bytes (max %d)", r.ContentLength, maxRequestSize)
	}

	// Create compressed size limited reader using http.MaxBytesReader (handles 413 automatically)
	limitedReader := http.MaxBytesReader(w, r.Body, maxRequestSize)

	// Check Content-Encoding strictly - only allow empty or "gzip" (case-insensitive)
	contentEncoding := strings.TrimSpace(strings.ToLower(r.Header.Get("Content-Encoding")))
	if contentEncoding != "" && contentEncoding != "gzip" {
		return nil, func() {}, fmt.Errorf("unsupported content encoding: %s (only gzip is supported)", contentEncoding)
	}

	if contentEncoding == "" {
		// No compression - enforce decompressed size limit (same as request size for uncompressed)
		// Use the stricter of the two limits
		maxSize := maxRequestSize
		if maxDecompressedSize < maxSize {
			maxSize = maxDecompressedSize
		}

		// If we need a stricter limit, re-create the reader
		if maxSize < maxRequestSize {
			// Close the original reader and create a new one with stricter limit
			_ = limitedReader.Close()
			limitedReader = http.MaxBytesReader(w, r.Body, maxSize)
		}

		return limitedReader, func() {}, nil
	}

	// Handle gzip decompression
	gzReader, err := gzip.NewReader(limitedReader)
	if err != nil {
		// Invalid gzip data → 400 Bad Request
		return nil, func() {}, fmt.Errorf("invalid gzip data: %w", err)
	}

	// Wrap in decompressed size limiter
	decompressedReader := &maxDecompressedReader{
		reader: gzReader,
		limit:  maxDecompressedSize,
	}

	cleanup := func() {
		gzReader.Close()
	}

	return decompressedReader, cleanup, nil
}

// mapErrorToHTTPStatus maps specific errors to appropriate HTTP status codes
func mapErrorToHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Decompressed size limit exceeded → 413
	if errors.Is(err, errDecompressedTooLarge) {
		return http.StatusRequestEntityTooLarge
	}

	// Compressed size limit exceeded → 413 (http.MaxBytesError)
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}

	// Check error messages for specific cases
	errMsg := err.Error()

	// Unsupported media type → 415
	if strings.Contains(errMsg, "unsupported media type") {
		return http.StatusUnsupportedMediaType
	}

	// Unsupported content encoding → 415
	if strings.Contains(errMsg, "unsupported content encoding") {
		return http.StatusUnsupportedMediaType
	}

	// Compressed size exceeded → 413
	if strings.Contains(errMsg, "compressed request body too large") {
		return http.StatusRequestEntityTooLarge
	}

	// Invalid gzip data → 400
	if strings.Contains(errMsg, "invalid gzip") || strings.Contains(errMsg, "gzip:") {
		return http.StatusBadRequest
	}

	// Version parse failures → 400 (keep consistent with existing tests)
	if strings.Contains(errMsg, "invalid version") || strings.Contains(errMsg, "version") {
		return http.StatusBadRequest
	}

	// Default to 400 for other client errors
	return http.StatusBadRequest
}

// respondWithMappedError responds with the appropriate HTTP status code based on the error
func respondWithMappedError(w http.ResponseWriter, r *http.Request, err error, fallbackMessage string, options *ServerOptions) {
	status := mapErrorToHTTPStatus(err)
	message := fallbackMessage
	if err != nil {
		message = err.Error()
	}
	respondWithError(w, r, status, message, options)
}
