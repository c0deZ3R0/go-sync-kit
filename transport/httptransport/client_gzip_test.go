package httptransport

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// TestPushGzipCompressionThreshold tests that outbound requests are compressed when payload exceeds threshold
func TestPushGzipCompressionThreshold(t *testing.T) {
	tests := []struct {
		name             string
		gzipMinBytes     int
		payloadSize      int
		compressionEnabled bool
		expectCompressed bool
	}{
		{
			name:             "small payload, default threshold - not compressed",
			gzipMinBytes:     0, // Default 1KB
			payloadSize:      500,
			compressionEnabled: true,
			expectCompressed: false,
		},
		{
			name:             "large payload, default threshold - compressed",
			gzipMinBytes:     0, // Default 1KB
			payloadSize:      2000,
			compressionEnabled: true,
			expectCompressed: true,
		},
		{
			name:             "small payload, custom low threshold - compressed",
			gzipMinBytes:     100,
			payloadSize:      200,
			compressionEnabled: true,
			expectCompressed: true,
		},
		{
			name:             "large payload, custom high threshold - not compressed",
			gzipMinBytes:     5000,
			payloadSize:      2000,
			compressionEnabled: true,
			expectCompressed: false,
		},
		{
			name:             "large payload, compression disabled - not compressed",
			gzipMinBytes:     0,
			payloadSize:      2000,
			compressionEnabled: false,
			expectCompressed: false,
		},
		{
			name:             "zero threshold, large payload - compressed",
			gzipMinBytes:     0,
			payloadSize:      1500,
			compressionEnabled: true,
			expectCompressed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server that captures request details
			var receivedContentEncoding string
			var receivedBody []byte
			var isBodyGzipped bool

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedContentEncoding = r.Header.Get("Content-Encoding")
				
				// Read the body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("Failed to read request body: %v", err)
					return
				}
				receivedBody = body

				// Check if body is gzipped by attempting to decompress
				if receivedContentEncoding == "gzip" {
					reader, err := gzip.NewReader(bytes.NewReader(body))
					if err != nil {
						t.Errorf("Failed to create gzip reader: %v", err)
						return
					}
					defer reader.Close()
					
					_, err = io.ReadAll(reader)
					isBodyGzipped = err == nil
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			}))
			defer server.Close()

			// Create client options
			options := &ClientOptions{
				CompressionEnabled: tt.compressionEnabled,
				GzipMinBytes:       tt.gzipMinBytes,
			}

			// Create transport client
			transport := NewTransport(server.URL, nil, nil, options)

			// Create events with approximately the desired payload size
			events := createEventsWithSize(tt.payloadSize)

			// Perform Push operation
			err := transport.Push(context.Background(), events)
			if err != nil {
				t.Fatalf("Push failed: %v", err)
			}

			// Verify compression behavior
			if tt.expectCompressed {
				if receivedContentEncoding != "gzip" {
					t.Errorf("Expected Content-Encoding: gzip, got: %s", receivedContentEncoding)
				}
				if !isBodyGzipped {
					t.Error("Expected body to be gzipped, but decompression failed")
				}
				// Verify the body is actually smaller when compressed (for reasonable payloads)
				if len(receivedBody) >= tt.payloadSize {
					t.Error("Expected compressed body to be smaller than original payload")
				}
			} else {
				if receivedContentEncoding == "gzip" {
					t.Error("Expected no compression, but received Content-Encoding: gzip")
				}
				if isBodyGzipped {
					t.Error("Expected body not to be gzipped, but it appears to be compressed")
				}
			}
		})
	}
}

// TestPushGzipWithCustomThreshold tests the WithGzipThreshold option
func TestPushGzipWithCustomThreshold(t *testing.T) {
	var receivedContentEncoding string
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create client with custom threshold
	options := &ClientOptions{
		CompressionEnabled: true,
	}
	// Apply custom threshold using option function
	WithGzipThreshold(200)(options)

	transport := NewTransport(server.URL, nil, nil, options)

	// Create events that result in ~300 bytes payload (above our 200 byte threshold)
	events := createEventsWithSize(300)

	err := transport.Push(context.Background(), events)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Should be compressed since payload > 200 bytes
	if receivedContentEncoding != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip with custom threshold, got: %s", receivedContentEncoding)
	}
}

// TestPushGzipDisabled tests that compression is disabled when CompressionEnabled is false
func TestPushGzipDisabled(t *testing.T) {
	var receivedContentEncoding string
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create client with compression disabled
	options := &ClientOptions{
		CompressionEnabled: false,
		GzipMinBytes:       100, // Low threshold, but compression is disabled
	}

	transport := NewTransport(server.URL, nil, nil, options)

	// Create large events that would normally be compressed
	events := createEventsWithSize(5000)

	err := transport.Push(context.Background(), events)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Should NOT be compressed since compression is disabled
	if receivedContentEncoding != "" {
		t.Errorf("Expected no Content-Encoding header when compression disabled, got: %s", receivedContentEncoding)
	}
}

// TestPushGzipContentType tests that Content-Type is correctly set with gzip
func TestPushGzipContentType(t *testing.T) {
	var receivedContentType string
	var receivedContentEncoding string
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedContentEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	options := &ClientOptions{
		CompressionEnabled: true,
		GzipMinBytes:       100,
	}

	transport := NewTransport(server.URL, nil, nil, options)
	events := createEventsWithSize(2000)

	err := transport.Push(context.Background(), events)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify both Content-Type and Content-Encoding are set correctly
	if receivedContentType != "application/json" {
		t.Errorf("Expected Content-Type: application/json, got: %s", receivedContentType)
	}
	if receivedContentEncoding != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got: %s", receivedContentEncoding)
	}
}

// TestPushGzipAcceptEncoding tests that Accept-Encoding header is set correctly
func TestPushGzipAcceptEncoding(t *testing.T) {
	tests := []struct {
		name                       string
		disableAutoDecompression   bool
		expectedAcceptEncoding     string
	}{
		{
			name:                     "auto decompression enabled",
			disableAutoDecompression: false,
			expectedAcceptEncoding:   "gzip, deflate",
		},
		{
			name:                     "auto decompression disabled",
			disableAutoDecompression: true,
			expectedAcceptEncoding:   "gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedAcceptEncoding string
			
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedAcceptEncoding = r.Header.Get("Accept-Encoding")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			}))
			defer server.Close()

			options := &ClientOptions{
				CompressionEnabled:        true,
				DisableAutoDecompression:  tt.disableAutoDecompression,
				GzipMinBytes:              100,
			}

			transport := NewTransport(server.URL, nil, nil, options)
			events := createEventsWithSize(2000)

			err := transport.Push(context.Background(), events)
			if err != nil {
				t.Fatalf("Push failed: %v", err)
			}

			if receivedAcceptEncoding != tt.expectedAcceptEncoding {
				t.Errorf("Expected Accept-Encoding: %s, got: %s", tt.expectedAcceptEncoding, receivedAcceptEncoding)
			}
		})
	}
}

// TestGzipValidation tests that GzipMinBytes validation works correctly
func TestGzipValidation(t *testing.T) {
	tests := []struct {
		name         string
		gzipMinBytes int
		expectError  bool
	}{
		{
			name:         "valid positive threshold",
			gzipMinBytes: 1024,
			expectError:  false,
		},
		{
			name:         "valid zero threshold",
			gzipMinBytes: 0,
			expectError:  false,
		},
		{
			name:         "invalid negative threshold",
			gzipMinBytes: -100,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &ClientOptions{
				CompressionEnabled: true,
				GzipMinBytes:       tt.gzipMinBytes,
				MaxResponseSize:    10 * 1024 * 1024, // Required field
				MaxDecompressedResponseSize: 20 * 1024 * 1024, // Required field
			}

			err := ValidateClientOptions(options)
			
			if tt.expectError && err == nil {
				t.Error("Expected validation error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, but got: %v", err)
			}
		})
	}
}

// Helper function to create events that result in approximately the desired payload size
func createEventsWithSize(targetSize int) []synckit.EventWithVersion {
	// Estimate bytes per event (JSON overhead + data)
	bytesPerEvent := 200 // Rough estimate including JSON structure
	
	numEvents := max(1, targetSize/bytesPerEvent)
	
	events := make([]synckit.EventWithVersion, numEvents)
	
	for i := 0; i < numEvents; i++ {
		// Create event with some data to reach target size
		dataSize := max(10, (targetSize/numEvents)-150) // Account for JSON overhead
		data := strings.Repeat("x", dataSize)
		
		event := &SimpleEvent{
			IDValue:          fmt.Sprintf("event-%d", i),
			TypeValue:        "test.event",
			AggregateIDValue: fmt.Sprintf("aggregate-%d", i),
			DataValue:        data,
			MetadataValue:    map[string]interface{}{"test": true},
		}
		
		events[i] = synckit.EventWithVersion{
			Event:   event,
			Version: cursor.IntegerCursor{Seq: uint64(i + 1)},
		}
	}
	
	return events
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
