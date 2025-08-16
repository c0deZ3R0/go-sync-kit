package httptransport

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// TestNewClientDefaults tests that NewTransportClient creates a client with the expected default values
func TestNewClientDefaults(t *testing.T) {
	baseURL := "http://example.com/sync"
	client := NewTransportClient(baseURL)

	if client.BaseURL() != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, client.BaseURL())
	}

	// Check HTTP client defaults
	httpClient := client.HTTPClient()
	if httpClient == nil {
		t.Fatal("Expected HTTP client to be set")
	}
	if httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", httpClient.Timeout)
	}

	// Check limits defaults
	limits := client.Limits()
	expectedLimits := Limits{
		MaxBodyBytes:         8 << 20,  // 8MB
		MaxDecompressedBytes: 64 << 20, // 64MB
		EnableGzip:           true,
		GzipMinBytes:         2 << 20, // 2MB
	}

	if limits.MaxBodyBytes != expectedLimits.MaxBodyBytes {
		t.Errorf("Expected MaxBodyBytes %d, got %d", expectedLimits.MaxBodyBytes, limits.MaxBodyBytes)
	}
	if limits.MaxDecompressedBytes != expectedLimits.MaxDecompressedBytes {
		t.Errorf("Expected MaxDecompressedBytes %d, got %d", expectedLimits.MaxDecompressedBytes, limits.MaxDecompressedBytes)
	}
	if limits.EnableGzip != expectedLimits.EnableGzip {
		t.Errorf("Expected EnableGzip %t, got %t", expectedLimits.EnableGzip, limits.EnableGzip)
	}
	if limits.GzipMinBytes != expectedLimits.GzipMinBytes {
		t.Errorf("Expected GzipMinBytes %d, got %d", expectedLimits.GzipMinBytes, limits.GzipMinBytes)
	}

	// Check that version parser is nil by default
	if client.VersionParser() != nil {
		t.Error("Expected default version parser to be nil")
	}
}

// TestWithHTTPClientOverride tests that WithHTTPClient option properly overrides the default HTTP client
func TestWithHTTPClientOverride(t *testing.T) {
	baseURL := "http://example.com/sync"
	customClient := &http.Client{Timeout: 60 * time.Second}

	client := NewTransportClient(baseURL, WithHTTPClient(customClient))

	if client.HTTPClient() != customClient {
		t.Error("Expected custom HTTP client to be set")
	}
	if client.HTTPClient().Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.HTTPClient().Timeout)
	}
}

// TestWithLimitsOverride tests that WithLimits option properly overrides the default limits
func TestWithLimitsOverride(t *testing.T) {
	baseURL := "http://example.com/sync"
	customLimits := Limits{
		MaxBodyBytes:         16 << 20, // 16MB
		MaxDecompressedBytes: 32 << 20, // 32MB
		EnableGzip:           false,
		GzipMinBytes:         4 << 20, // 4MB
	}

	client := NewTransportClient(baseURL, WithLimits(customLimits))

	limits := client.Limits()
	if limits.MaxBodyBytes != customLimits.MaxBodyBytes {
		t.Errorf("Expected MaxBodyBytes %d, got %d", customLimits.MaxBodyBytes, limits.MaxBodyBytes)
	}
	if limits.MaxDecompressedBytes != customLimits.MaxDecompressedBytes {
		t.Errorf("Expected MaxDecompressedBytes %d, got %d", customLimits.MaxDecompressedBytes, limits.MaxDecompressedBytes)
	}
	if limits.EnableGzip != customLimits.EnableGzip {
		t.Errorf("Expected EnableGzip %t, got %t", customLimits.EnableGzip, limits.EnableGzip)
	}
	if limits.GzipMinBytes != customLimits.GzipMinBytes {
		t.Errorf("Expected GzipMinBytes %d, got %d", customLimits.GzipMinBytes, limits.GzipMinBytes)
	}
}

// TestWithVersionParser tests that WithVersionParser option properly sets the version parser
func TestWithVersionParser(t *testing.T) {
	baseURL := "http://example.com/sync"
	
	// Create a custom version parser function
	customParser := func(ctx context.Context, s string) (synckit.Version, error) {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		return cursor.IntegerCursor{Seq: uint64(v)}, nil
	}

	client := NewTransportClient(baseURL, WithVersionParser(customParser))

	parser := client.VersionParser()
	if parser == nil {
		t.Fatal("Expected version parser to be set")
	}

	// Test that the parser works correctly
	ctx := context.Background()
	version, err := parser(ctx, "123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	intCursor, ok := version.(cursor.IntegerCursor)
	if !ok {
		t.Fatal("Expected IntegerCursor type")
	}
	if intCursor.Seq != 123 {
		t.Errorf("Expected sequence 123, got %d", intCursor.Seq)
	}
}

// TestMultipleOptions tests that multiple options can be applied together
func TestMultipleOptions(t *testing.T) {
	baseURL := "http://example.com/sync"
	customClient := &http.Client{Timeout: 45 * time.Second}
	customLimits := Limits{
		MaxBodyBytes:         12 << 20, // 12MB
		MaxDecompressedBytes: 48 << 20, // 48MB
		EnableGzip:           false,
		GzipMinBytes:         3 << 20, // 3MB
	}
	customParser := func(ctx context.Context, s string) (synckit.Version, error) {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		return cursor.IntegerCursor{Seq: uint64(v * 10)}, nil
	}

	client := NewTransportClient(baseURL,
		WithHTTPClient(customClient),
		WithLimits(customLimits),
		WithVersionParser(customParser),
	)

	// Verify base URL
	if client.BaseURL() != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, client.BaseURL())
	}

	// Verify HTTP client
	if client.HTTPClient() != customClient {
		t.Error("Expected custom HTTP client to be set")
	}

	// Verify limits
	limits := client.Limits()
	if limits != customLimits {
		t.Errorf("Expected custom limits %+v, got %+v", customLimits, limits)
	}

	// Verify version parser
	parser := client.VersionParser()
	if parser == nil {
		t.Fatal("Expected version parser to be set")
	}

	ctx := context.Background()
	version, err := parser(ctx, "5")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	intCursor, ok := version.(cursor.IntegerCursor)
	if !ok {
		t.Fatal("Expected IntegerCursor type")
	}
	if intCursor.Seq != 50 {
		t.Errorf("Expected sequence 50 (5*10), got %d", intCursor.Seq)
	}
}

// TestToHTTPTransport tests that the new Client can be converted to the existing HTTPTransport
func TestToHTTPTransport(t *testing.T) {
	baseURL := "http://example.com/sync"
	customLimits := Limits{
		MaxBodyBytes:         10 << 20, // 10MB
		MaxDecompressedBytes: 20 << 20, // 20MB
		EnableGzip:           true,
		GzipMinBytes:         1 << 20, // 1MB
	}

	client := NewTransportClient(baseURL, WithLimits(customLimits))
	httpTransport := client.ToHTTPTransport()

	if httpTransport == nil {
		t.Fatal("Expected HTTPTransport to be created")
	}

	// Verify that the HTTPTransport has the expected properties
	if httpTransport.baseURL != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, httpTransport.baseURL)
	}

	// Verify client options are properly converted
	options := httpTransport.options
	if options == nil {
		t.Fatal("Expected client options to be set")
	}

	if options.CompressionEnabled != customLimits.EnableGzip {
		t.Errorf("Expected CompressionEnabled %t, got %t", customLimits.EnableGzip, options.CompressionEnabled)
	}
	if options.MaxResponseSize != customLimits.MaxBodyBytes {
		t.Errorf("Expected MaxResponseSize %d, got %d", customLimits.MaxBodyBytes, options.MaxResponseSize)
	}
	if options.MaxDecompressedResponseSize != customLimits.MaxDecompressedBytes {
		t.Errorf("Expected MaxDecompressedResponseSize %d, got %d", customLimits.MaxDecompressedBytes, options.MaxDecompressedResponseSize)
	}
}

// TestOptionsOrdering tests that the last option takes precedence when options conflict
func TestOptionsOrdering(t *testing.T) {
	baseURL := "http://example.com/sync"
	
	firstLimits := Limits{
		MaxBodyBytes:         8 << 20,
		MaxDecompressedBytes: 64 << 20,
		EnableGzip:           true,
		GzipMinBytes:         2 << 20,
	}
	
	secondLimits := Limits{
		MaxBodyBytes:         16 << 20,
		MaxDecompressedBytes: 128 << 20,
		EnableGzip:           false,
		GzipMinBytes:         4 << 20,
	}

	client := NewTransportClient(baseURL,
		WithLimits(firstLimits),
		WithLimits(secondLimits), // This should override the first
	)

	limits := client.Limits()
	if limits != secondLimits {
		t.Errorf("Expected second limits %+v, got %+v", secondLimits, limits)
	}
}

// TestEmptyOptions tests that NewClient works correctly with no options
func TestEmptyOptions(t *testing.T) {
	baseURL := "http://example.com/sync"
	client := NewTransportClient(baseURL)

	// Should be the same as TestNewClientDefaults, but explicitly testing empty slice
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.BaseURL() != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, client.BaseURL())
	}
}
