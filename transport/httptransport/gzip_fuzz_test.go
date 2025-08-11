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

// FuzzCreateSafeRequestReader fuzzes the createSafeRequestReader function to test its robustness
// against malformed or malicious gzip input, as mentioned in the code review
func FuzzCreateSafeRequestReader(f *testing.F) {
	// Seed the fuzzer with some valid gzip data
	validPayload := "This is valid JSON data for testing"
	var validGzipBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&validGzipBuf)
	gzWriter.Write([]byte(validPayload))
	gzWriter.Close()
	
	f.Add(validGzipBuf.Bytes(), "application/json", "gzip")
	f.Add([]byte(validPayload), "application/json", "")
	f.Add([]byte(""), "application/json", "gzip")
	f.Add([]byte("not gzip"), "application/json", "gzip")
	
	// Add some malformed gzip headers
	f.Add([]byte{0x1f, 0x8b, 0x08, 0x00}, "application/json", "gzip") // Truncated gzip header
	f.Add([]byte{0x1f, 0x8b}, "application/json", "gzip")             // Very short
	f.Add([]byte{0x00, 0x00, 0x00, 0x00}, "application/json", "gzip") // Wrong magic numbers
	
	f.Fuzz(func(t *testing.T, data []byte, contentType, contentEncoding string) {
		// Test that createSafeRequestReader doesn't panic on arbitrary input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("createSafeRequestReader panicked on input len=%d, ct=%q, ce=%q: %v", 
					len(data), contentType, contentEncoding, r)
			}
		}()
		
		// Create a test request with the fuzzed data
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(data))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
		
		w := httptest.NewRecorder()
		
		// Test with default server options
		serverOpts := DefaultServerOptions()
		
		// Create the safe request reader
		reader, cleanup, err := createSafeRequestReader(w, req, serverOpts)
		
		// cleanup should always be callable, even if err != nil
		if cleanup != nil {
			defer cleanup()
		}
		
		if err != nil {
			// Errors are acceptable for invalid input
			return
		}
		
		if reader == nil {
			t.Error("createSafeRequestReader returned nil reader with no error")
			return
		}
		
		// Try to read from the reader without panicking
		buffer := make([]byte, 1024)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				// Successfully read some data
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				// Read errors are acceptable for malformed data
				break
			}
		}
	})
}

// FuzzGzipDecompression specifically fuzzes gzip decompression logic
func FuzzGzipDecompression(f *testing.F) {
	// Valid gzip data
	validData := "Hello, world! This is test data for gzip compression."
	var validBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&validBuf)
	gzWriter.Write([]byte(validData))
	gzWriter.Close()
	
	// Seed with valid and some invalid patterns
	f.Add(validBuf.Bytes())
	f.Add([]byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}) // Minimal gzip header
	f.Add([]byte{0x1f, 0x8b}) // Just magic numbers
	f.Add([]byte{}) // Empty data
	f.Add([]byte("not gzip at all"))
	
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Gzip decompression panicked on input len=%d: %v", len(data), r)
			}
		}()
		
		// Create a request with gzip content-encoding
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		
		w := httptest.NewRecorder()
		serverOpts := &ServerOptions{
			MaxRequestSize:      10 * 1024,    // 10KB
			MaxDecompressedSize: 20 * 1024,    // 20KB
			CompressionEnabled:  true,
		}
		
		reader, cleanup, err := createSafeRequestReader(w, req, serverOpts)
		if cleanup != nil {
			defer cleanup()
		}
		
		if err != nil {
			// Errors are expected for invalid gzip data
			return
		}
		
		if reader == nil {
			t.Error("Expected reader or error, got nil reader with no error")
			return
		}
		
		// Try to read all data
		output, readErr := io.ReadAll(reader)
		
		// ReadAll should either succeed or fail gracefully
		if readErr != nil && readErr != io.EOF {
			// Read errors are acceptable for malformed gzip
			return
		}
		
		// If we got output, it should be reasonable
		if int64(len(output)) > serverOpts.MaxDecompressedSize {
			t.Errorf("Decompressed data size %d exceeds limit %d", len(output), serverOpts.MaxDecompressedSize)
		}
	})
}

// FuzzCompressionSizeLimits fuzzes the size limit enforcement in gzip processing
func FuzzCompressionSizeLimits(f *testing.F) {
	// Create test data of various sizes
	smallData := strings.Repeat("a", 100)
	mediumData := strings.Repeat("b", 1000)
	largeData := strings.Repeat("c", 10000)
	
	// Create gzipped versions
	var smallBuf, mediumBuf, largeBuf bytes.Buffer
	
	gzSmall := gzip.NewWriter(&smallBuf)
	gzSmall.Write([]byte(smallData))
	gzSmall.Close()
	
	gzMedium := gzip.NewWriter(&mediumBuf)
	gzMedium.Write([]byte(mediumData))
	gzMedium.Close()
	
	gzLarge := gzip.NewWriter(&largeBuf)
	gzLarge.Write([]byte(largeData))
	gzLarge.Close()
	
	f.Add(smallBuf.Bytes(), int64(500), int64(1000))   // Within limits
	f.Add(mediumBuf.Bytes(), int64(2000), int64(4000)) // Within limits
	f.Add(largeBuf.Bytes(), int64(500), int64(1000))   // Exceeds limits
	f.Add(largeBuf.Bytes(), int64(15000), int64(5000)) // Decompressed exceeds limit
	
	f.Fuzz(func(t *testing.T, data []byte, maxRequestSize, maxDecompressedSize int64) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Size limit enforcement panicked: %v", r)
			}
		}()
		
		// Ensure positive limits
		if maxRequestSize <= 0 {
			maxRequestSize = 1024
		}
		if maxDecompressedSize <= 0 {
			maxDecompressedSize = 2048
		}
		
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		
		w := httptest.NewRecorder()
		serverOpts := &ServerOptions{
			MaxRequestSize:      maxRequestSize,
			MaxDecompressedSize: maxDecompressedSize,
			CompressionEnabled:  true,
		}
		
		reader, cleanup, err := createSafeRequestReader(w, req, serverOpts)
		if cleanup != nil {
			defer cleanup()
		}
		
		// Test size limit enforcement
		if int64(len(data)) > maxRequestSize {
			// Should get an error for oversized compressed data
			if err == nil {
				t.Errorf("Expected error for compressed data size %d > limit %d", 
					len(data), maxRequestSize)
			}
			return
		}
		
		if err != nil {
			// Other errors are acceptable (e.g., invalid gzip)
			return
		}
		
		if reader == nil {
			t.Error("Expected reader or error, got neither")
			return
		}
		
		// Try to read and verify decompression limit is enforced
		output, readErr := io.ReadAll(reader)
		
		if readErr != nil {
			// Check if it's a decompression limit error
			if strings.Contains(readErr.Error(), "decompressed") || 
			   strings.Contains(readErr.Error(), "exceeds") ||
			   strings.Contains(readErr.Error(), "limit") {
				// This is expected for data that exceeds decompression limits
				return
			}
			// Other read errors are acceptable for malformed gzip
			return
		}
		
		// If read succeeded, check the size
		if int64(len(output)) > maxDecompressedSize {
			t.Errorf("Decompressed output size %d exceeds limit %d", 
				len(output), maxDecompressedSize)
		}
	})
}

// FuzzMaxDecompressedReader fuzzes the maxDecompressedReader specifically
func FuzzMaxDecompressedReader(f *testing.F) {
	// Create some test patterns
	testData := []string{
		"Hello, World!",
		strings.Repeat("A", 1000),
		strings.Repeat("B", 5000),
		"",
		"Single byte",
	}
	
	for _, data := range testData {
		f.Add(data, int64(100), int64(500), int64(1000))
	}
	
	f.Fuzz(func(t *testing.T, data string, limit1, limit2, limit3 int64) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("maxDecompressedReader panicked: %v", r)
			}
		}()
		
		// Ensure positive limits
		limits := []int64{limit1, limit2, limit3}
		for i, limit := range limits {
			if limit <= 0 {
				limits[i] = 100
			}
		}
		
		for _, limit := range limits {
			// Create a limited reader
			reader := &maxDecompressedReader{
				reader: strings.NewReader(data),
				limit:  limit,
			}
			
			// Try to read all data
			output, err := io.ReadAll(reader)
			
			if err != nil {
				// Check if it's the expected limit error
				if err == errDecompressedTooLarge {
					// This is expected when data exceeds limit
					if int64(len(output)) > limit {
						t.Errorf("Read %d bytes with limit %d, but got limit error", 
							len(output), limit)
					}
				}
				// Other errors are acceptable
				return
			}
			
			// If no error, data should be within limit
			if int64(len(output)) > limit {
				t.Errorf("Read %d bytes exceeding limit %d without error", 
					len(output), limit)
			}
			
			// Verify the content is correct (up to the limit)
			expectedLen := len(data)
			if int64(expectedLen) > limit {
				expectedLen = int(limit)
			}
			
			if len(output) != expectedLen {
				t.Errorf("Expected output length %d, got %d", expectedLen, len(output))
			}
			
			if len(output) > 0 && expectedLen > 0 {
				expectedContent := data[:expectedLen]
				if string(output) != expectedContent {
					t.Errorf("Content mismatch: expected %q, got %q", 
						expectedContent, string(output))
				}
			}
		}
	})
}

// FuzzContentTypeValidation fuzzes content type validation
func FuzzContentTypeValidation(f *testing.F) {
	// Seed with various content types
	f.Add("application/json", "")
	f.Add("application/json; charset=utf-8", "")
	f.Add("text/plain", "")
	f.Add("application/xml", "")
	f.Add("", "")
	f.Add("invalid/content-type", "")
	f.Add("application/json", "gzip")
	f.Add("text/html", "gzip")
	
	f.Fuzz(func(t *testing.T, contentType, contentEncoding string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Content type validation panicked: %v", r)
			}
		}()
		
		// Create a simple request
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test data"))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
		
		w := httptest.NewRecorder()
		serverOpts := DefaultServerOptions()
		
		reader, cleanup, err := createSafeRequestReader(w, req, serverOpts)
		if cleanup != nil {
			defer cleanup()
		}
		
		// Validate behavior based on content type
		if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
			// Should get an error for unsupported media types
			if err == nil {
				t.Errorf("Expected error for unsupported content type %q", contentType)
			}
			// Should not return a reader
			if reader != nil {
				t.Errorf("Expected nil reader for unsupported content type %q", contentType)
			}
		} else {
			// For valid or empty content types, we might get a reader or an error
			// depending on the content encoding
			if err != nil {
				// Errors are acceptable (e.g., invalid gzip)
				return
			}
			if reader == nil {
				t.Error("Expected reader for valid content type")
			}
		}
	})
}
