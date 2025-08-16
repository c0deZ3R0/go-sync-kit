package httptransport

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSafeRequestReader_ContentTypeValidation tests strict Content-Type validation
func TestSafeRequestReader_ContentTypeValidation(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		expectedError  bool
		expectedStatus int
	}{
		{
			name:           "valid application/json",
			contentType:    "application/json",
			expectedError:  false,
			expectedStatus: 0,
		},
		{
			name:           "valid application/json with charset",
			contentType:    "application/json; charset=utf-8",
			expectedError:  false,
			expectedStatus: 0,
		},
		{
			name:           "no content type (allowed)",
			contentType:    "",
			expectedError:  false,
			expectedStatus: 0,
		},
		{
			name:           "invalid text/plain",
			contentType:    "text/plain",
			expectedError:  true,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "invalid application/xml",
			contentType:    "application/xml",
			expectedError:  true,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "invalid text/html",
			contentType:    "text/html",
			expectedError:  true,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "invalid application/octet-stream",
			contentType:    "application/octet-stream",
			expectedError:  true,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test": true}`))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()
			options := DefaultServerOptions()

			reader, cleanup, err := createSafeRequestReader(w, req, options)
			if cleanup != nil {
				defer cleanup()
			}

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				status := mapErrorToHTTPStatus(err)
				if status != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, status)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if reader == nil {
					t.Error("expected reader but got nil")
				}
			}
		})
	}
}

// TestSafeRequestReader_ContentEncodingValidation tests strict Content-Encoding validation
func TestSafeRequestReader_ContentEncodingValidation(t *testing.T) {
	tests := []struct {
		name            string
		contentEncoding string
		expectedError   bool
		expectedStatus  int
	}{
		{
			name:            "no content encoding (allowed)",
			contentEncoding: "",
			expectedError:   false,
			expectedStatus:  0,
		},
		{
			name:            "valid gzip lowercase",
			contentEncoding: "gzip",
			expectedError:   false,
			expectedStatus:  0,
		},
		{
			name:            "valid gzip uppercase",
			contentEncoding: "GZIP",
			expectedError:   false,
			expectedStatus:  0,
		},
		{
			name:            "valid gzip mixed case",
			contentEncoding: "GzIp",
			expectedError:   false,
			expectedStatus:  0,
		},
		{
			name:            "valid gzip with whitespace",
			contentEncoding: " gzip ",
			expectedError:   false,
			expectedStatus:  0,
		},
		{
			name:            "invalid deflate",
			contentEncoding: "deflate",
			expectedError:   true,
			expectedStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:            "invalid br (brotli)",
			contentEncoding: "br",
			expectedError:   true,
			expectedStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:            "invalid compress",
			contentEncoding: "compress",
			expectedError:   true,
			expectedStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:            "invalid multiple encodings",
			contentEncoding: "gzip, deflate",
			expectedError:   true,
			expectedStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:            "invalid identity",
			contentEncoding: "identity",
			expectedError:   true,
			expectedStatus:  http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create appropriate request body based on content encoding
			var bodyReader *bytes.Reader

			if tt.contentEncoding != "" && strings.ToLower(strings.TrimSpace(tt.contentEncoding)) == "gzip" && !tt.expectedError {
				// Create valid gzip data
				var buf bytes.Buffer
				gzWriter := gzip.NewWriter(&buf)
				gzWriter.Write([]byte(`{"test": true}`))
				gzWriter.Close()
				bodyReader = bytes.NewReader(buf.Bytes())
			} else if tt.contentEncoding != "" && !tt.expectedError {
				// This shouldn't happen based on our test cases, but just in case
				bodyReader = bytes.NewReader([]byte(`{"test": true}`))
			} else {
				// Regular JSON or error cases
				bodyReader = bytes.NewReader([]byte(`{"test": true}`))
			}

			req := httptest.NewRequest("POST", "/test", bodyReader)
			req.Header.Set("Content-Type", "application/json")
			if tt.contentEncoding != "" {
				req.Header.Set("Content-Encoding", tt.contentEncoding)
			}

			w := httptest.NewRecorder()
			options := DefaultServerOptions()

			reader, cleanup, err := createSafeRequestReader(w, req, options)
			if cleanup != nil {
				defer cleanup()
			}

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				status := mapErrorToHTTPStatus(err)
				if status != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, status)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if reader == nil {
					t.Error("expected reader but got nil")
				}
			}
		})
	}
}

// TestSafeRequestReader_SizeLimits tests both compressed and decompressed size limits
func TestSafeRequestReader_SizeLimits(t *testing.T) {
	tests := []struct {
		name                string
		maxRequestSize      int64
		maxDecompressedSize int64
		contentEncoding     string
		dataSize            int
		expectedError       bool
		expectedStatus      int
		testReadingData     bool
	}{
		{
			name:                "small uncompressed within limits",
			maxRequestSize:      1000,
			maxDecompressedSize: 1000,
			contentEncoding:     "",
			dataSize:            500,
			expectedError:       false,
			expectedStatus:      0,
		},
		{
			name:                "uncompressed exceeds request size limit",
			maxRequestSize:      100,
			maxDecompressedSize: 1000,
			contentEncoding:     "",
			dataSize:            200,
			expectedError:       true,
			expectedStatus:      http.StatusRequestEntityTooLarge,
		},
		{
			name:                "compressed within limits",
			maxRequestSize:      1000,
			maxDecompressedSize: 2000,
			contentEncoding:     "gzip",
			dataSize:            500,
			expectedError:       false,
			expectedStatus:      0,
		},
		{
			name:                "compressed size exceeds request limit",
			maxRequestSize:      25, // Very small limit
			maxDecompressedSize: 2000,
			contentEncoding:     "gzip",
			dataSize:            100, // Will compress to about 27 bytes, exceeding 25
			expectedError:       true,
			expectedStatus:      http.StatusRequestEntityTooLarge,
		},
		{
			name:                "decompressed size exceeds limit",
			maxRequestSize:      1000,
			maxDecompressedSize: 100,
			contentEncoding:     "gzip",
			dataSize:            500, // Will decompress to 500 bytes, exceeding 100 limit
			expectedError:       true,
			expectedStatus:      http.StatusRequestEntityTooLarge,
			testReadingData:     true, // Need to test by reading the stream
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			testData := strings.Repeat("x", tt.dataSize)
			var bodyReader *bytes.Reader

			if tt.contentEncoding == "gzip" {
				// Create gzip compressed data
				var buf bytes.Buffer
				gzWriter := gzip.NewWriter(&buf)
				gzWriter.Write([]byte(testData))
				gzWriter.Close()
				bodyReader = bytes.NewReader(buf.Bytes())
			} else {
				bodyReader = bytes.NewReader([]byte(testData))
			}

			req := httptest.NewRequest("POST", "/test", bodyReader)
			req.Header.Set("Content-Type", "application/json")
			if tt.contentEncoding != "" {
				req.Header.Set("Content-Encoding", tt.contentEncoding)
			}

			w := httptest.NewRecorder()
			options := &ServerOptions{
				MaxRequestSize:      tt.maxRequestSize,
				MaxDecompressedSize: tt.maxDecompressedSize,
			}

			reader, cleanup, err := createSafeRequestReader(w, req, options)
			if cleanup != nil {
				defer cleanup()
			}

			if tt.expectedError && !tt.testReadingData {
				// Test cases that should fail at creation time
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				status := mapErrorToHTTPStatus(err)
				if status != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, status)
				}
			} else if tt.testReadingData {
				// Test cases that need to read the stream to trigger limits
				if err != nil {
					t.Errorf("unexpected error at creation: %v", err)
					return
				}
				if reader == nil {
					t.Error("expected reader but got nil")
					return
				}
				
				// Try to read all the data, expecting an error
				buf := make([]byte, 1024)
				totalRead := 0
				var readErr error
				var n int
				
				for readErr == nil {
					n, readErr = reader.Read(buf)
					totalRead += n
					if readErr != nil {
						if tt.expectedError {
							// Check if we got the expected decompressed size limit error
							status := mapErrorToHTTPStatus(readErr)
							if status != tt.expectedStatus {
								t.Errorf("expected status %d during read, got %d (error: %v)", tt.expectedStatus, status, readErr)
							}
						} else {
							// Unexpected error
							if readErr != io.EOF {
								t.Errorf("unexpected read error: %v", readErr)
							}
						}
						break
					}
				}
				
				if tt.expectedError && readErr == nil {
					t.Error("expected error during reading but got none")
				}
			} else {
				// Test cases that should succeed
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if reader == nil {
					t.Error("expected reader but got nil")
				}
			}
		})
	}
}

// TestSafeRequestReader_InvalidGzipData tests handling of corrupted gzip data
func TestSafeRequestReader_InvalidGzipData(t *testing.T) {
	tests := []struct {
		name          string
		gzipData      []byte
		expectedError bool
	}{
		{
			name:          "completely invalid data",
			gzipData:      []byte("not gzip at all"),
			expectedError: true,
		},
		{
			name:          "truncated gzip header",
			gzipData:      []byte{0x1f},
			expectedError: true,
		},
		{
			name:          "invalid gzip header",
			gzipData:      []byte{0x1f, 0x8a, 0x00}, // Wrong compression method
			expectedError: true,
		},
		{
			name:          "empty data",
			gzipData:      []byte{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(tt.gzipData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")

			w := httptest.NewRecorder()
			options := DefaultServerOptions()

			reader, cleanup, err := createSafeRequestReader(w, req, options)
			if cleanup != nil {
				defer cleanup()
			}

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				status := mapErrorToHTTPStatus(err)
				if status != http.StatusBadRequest {
					t.Errorf("expected status %d, got %d", http.StatusBadRequest, status)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if reader == nil {
					t.Error("expected reader but got nil")
				}
			}
		})
	}
}

// TestSafeRequestReader_HandlerIntegration tests that handlers properly use the safe reader
func TestSafeRequestReader_HandlerIntegration(t *testing.T) {
	// Create a test store and handler
	store := &MockEventStore{}
	handler := NewSyncHandler(store, nil, nil, nil)

	tests := []struct {
		name            string
		method          string
		path            string
		contentType     string
		contentEncoding string
		body            string
		expectedStatus  int
	}{
		{
			name:           "valid push request",
			method:         "POST",
			path:           "/push",
			contentType:    "application/json",
			body:           `[]`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "push with invalid content type",
			method:         "POST",
			path:           "/push",
			contentType:    "text/plain",
			body:           `[]`,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:            "push with invalid content encoding",
			method:          "POST",
			path:            "/push",
			contentType:     "application/json",
			contentEncoding: "deflate",
			body:            `[]`,
			expectedStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:           "valid pull-cursor request",
			method:         "POST",
			path:           "/pull-cursor",
			contentType:    "application/json",
			body:           `{}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "pull-cursor with invalid content type",
			method:         "POST",
			path:           "/pull-cursor",
			contentType:    "application/xml",
			body:           `<request/>`,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.contentEncoding != "" {
				req.Header.Set("Content-Encoding", tt.contentEncoding)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
