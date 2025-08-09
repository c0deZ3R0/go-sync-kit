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

func TestMapErrorToHTTPStatus(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "nil error returns 200",
			err:            nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "errDecompressedTooLarge returns 413",
			err:            errDecompressedTooLarge,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "http.MaxBytesError returns 413",
			err:            &http.MaxBytesError{Limit: 1000},
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "unsupported media type error returns 415",
			err:            newTestError("unsupported media type: text/plain"),
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "compressed size exceeded returns 413",
			err:            newTestError("compressed request body too large: 5000 bytes (max 1000)"),
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "invalid gzip error returns 400",
			err:            newTestError("invalid gzip data: unexpected EOF"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "gzip error returns 400",
			err:            newTestError("gzip: invalid header"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid version error returns 400",
			err:            newTestError("invalid version: not a number"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "version parse error returns 400",
			err:            newTestError("version parse failed"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "generic error returns 400",
			err:            newTestError("something went wrong"),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := mapErrorToHTTPStatus(tt.err)
			if status != tt.expectedStatus {
				t.Errorf("mapErrorToHTTPStatus() = %d, want %d", status, tt.expectedStatus)
			}
		})
	}
}

func TestCreateSafeRequestReader(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		options        *ServerOptions
		expectedError  bool
		expectedStatus int
	}{
		{
			name: "valid JSON request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test": true}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			options:        DefaultServerOptions(),
			expectedError:  false,
			expectedStatus: 0,
		},
		{
			name: "unsupported media type",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/test", strings.NewReader("test"))
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			options:        DefaultServerOptions(),
			expectedError:  true,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name: "compressed size limit exceeded",
			setupRequest: func() *http.Request {
				// Create a request larger than 100 bytes
				largeData := strings.Repeat("x", 200)
				req := httptest.NewRequest("POST", "/test", strings.NewReader(largeData))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			options: &ServerOptions{
				MaxRequestSize:      100,
				MaxDecompressedSize: 200,
			},
			expectedError:  true,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "valid gzip compressed request",
			setupRequest: func() *http.Request {
				// Create gzip compressed JSON
				var buf bytes.Buffer
				gzWriter := gzip.NewWriter(&buf)
				gzWriter.Write([]byte(`{"test": true}`))
				gzWriter.Close()

				req := httptest.NewRequest("POST", "/test", &buf)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Content-Encoding", "gzip")
				return req
			},
			options:        DefaultServerOptions(),
			expectedError:  false,
			expectedStatus: 0,
		},
		{
			name: "invalid gzip data",
			setupRequest: func() *http.Request {
				// Create invalid gzip data
				req := httptest.NewRequest("POST", "/test", strings.NewReader("not gzip data"))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Content-Encoding", "gzip")
				return req
			},
			options:        DefaultServerOptions(),
			expectedError:  true,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			w := httptest.NewRecorder()

			reader, cleanup, err := createSafeRequestReader(w, req, tt.options)
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

func TestMaxDecompressedReader(t *testing.T) {
	t.Run("respects decompression limit", func(t *testing.T) {
		// Create large data that will exceed limit when decompressed
		largeData := strings.Repeat("A", 1000) // 1KB of data
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		gzWriter.Write([]byte(largeData))
		gzWriter.Close()

		gzReader, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatal(err)
		}
		defer gzReader.Close()

		// Set limit to 500 bytes (less than decompressed size)
		limitedReader := &maxDecompressedReader{
			reader: gzReader,
			limit:  500,
		}

		// Try to read all data - should get error when limit exceeded
		data, err := io.ReadAll(limitedReader)
		if err != errDecompressedTooLarge {
			t.Errorf("expected errDecompressedTooLarge, got %v", err)
		}
		if len(data) > 500 {
			t.Errorf("read %d bytes, expected <= 500", len(data))
		}
	})

	t.Run("allows reading within limit", func(t *testing.T) {
		data := "small data"
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		gzWriter.Write([]byte(data))
		gzWriter.Close()

		gzReader, err := gzip.NewReader(&buf)
		if err != nil {
			t.Fatal(err)
		}
		defer gzReader.Close()

		limitedReader := &maxDecompressedReader{
			reader: gzReader,
			limit:  100, // More than enough
		}

		readData, err := io.ReadAll(limitedReader)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if string(readData) != data {
			t.Errorf("expected %q, got %q", data, string(readData))
		}
	})
}

func TestRespondWithMappedError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "decompressed size exceeded",
			err:            errDecompressedTooLarge,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "compressed size exceeded",
			err:            &http.MaxBytesError{Limit: 1000},
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "unsupported media type",
			err:            newTestError("unsupported media type: text/xml"),
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "invalid gzip",
			err:            newTestError("invalid gzip data: corrupted"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "version parse error",
			err:            newTestError("invalid version: cannot parse"),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			options := DefaultServerOptions()

			respondWithMappedError(w, req, tt.err, "fallback", options)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// Helper function to create test errors
func newTestError(message string) error {
	return &testError{message: message}
}

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
