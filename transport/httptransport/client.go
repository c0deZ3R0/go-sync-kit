package httptransport

import (
	"context"
	"net/http"
	"time"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Limits defines size and compression limits for HTTP client
type Limits struct {
	MaxBodyBytes         int64 // Maximum body size in bytes
	MaxDecompressedBytes int64 // Maximum decompressed response size
	EnableGzip           bool  // Whether to enable gzip compression
	GzipMinBytes         int   // Minimum bytes before applying gzip compression
}

// ClientVersionParser converts a version string into synckit.Version.
// This is for the new client implementation to avoid conflicts with existing VersionParser
type ClientVersionParser func(ctx context.Context, s string) (synckit.Version, error)

// TransportClient represents an HTTP transport client with configurable options
type TransportClient struct {
	baseURL      string
	http         *http.Client
	limits       Limits
	parseVersion ClientVersionParser
}

// TransportClientOption configures a TransportClient using the functional options pattern
type TransportClientOption func(*TransportClient)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(cl *http.Client) TransportClientOption {
	return func(c *TransportClient) {
		c.http = cl
	}
}

// WithLimits sets the size and compression limits
func WithLimits(l Limits) TransportClientOption {
	return func(c *TransportClient) {
		c.limits = l
	}
}

// WithVersionParser sets a custom version parser
func WithVersionParser(vp ClientVersionParser) TransportClientOption {
	return func(c *TransportClient) {
		c.parseVersion = vp
	}
}

// NewTransportClient creates a new HTTP transport client with functional options.
// This is the new preferred way to create HTTP transport clients.
func NewTransportClient(baseURL string, opts ...TransportClientOption) *TransportClient {
	c := &TransportClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
		limits: Limits{
			MaxBodyBytes:         8 << 20,  // 8MB
			MaxDecompressedBytes: 64 << 20, // 64MB
			EnableGzip:           true,
			GzipMinBytes:         2 << 20, // 2MB
		},
	}

	// Apply all options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// BaseURL returns the base URL for the client
func (c *TransportClient) BaseURL() string {
	return c.baseURL
}

// HTTPClient returns the underlying HTTP client
func (c *TransportClient) HTTPClient() *http.Client {
	return c.http
}

// Limits returns the current limits configuration
func (c *TransportClient) Limits() Limits {
	return c.limits
}

// VersionParser returns the version parser function
func (c *TransportClient) VersionParser() ClientVersionParser {
	return c.parseVersion
}

// ToHTTPTransport converts the new TransportClient to the existing HTTPTransport for backward compatibility.
// This allows the new TransportClient to be used with existing code that expects HTTPTransport.
func (c *TransportClient) ToHTTPTransport() *HTTPTransport {
	// Convert our Limits to the existing ClientOptions format
	clientOptions := &ClientOptions{
		CompressionEnabled:          c.limits.EnableGzip,
		MaxResponseSize:             c.limits.MaxBodyBytes,
		MaxDecompressedResponseSize: c.limits.MaxDecompressedBytes,
		RequestTimeout:              c.http.Timeout,
		// Set other defaults from DefaultClientOptions
		RetryMax:     3,
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 30 * time.Second,
	}

	// Convert our ClientVersionParser to VersionParser for compatibility
	var versionParser VersionParser
	if c.parseVersion != nil {
		versionParser = VersionParser(c.parseVersion)
	}
	
	// Use the existing NewTransportWithLogger to create HTTPTransport
	return NewTransportWithLogger(c.baseURL, c.http, versionParser, clientOptions, nil)
}
