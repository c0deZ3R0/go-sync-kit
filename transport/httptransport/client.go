package httptransport

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
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
			GzipMinBytes:         1024, // 1KB (updated default)
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
		GzipMinBytes:                c.limits.GzipMinBytes,
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

// HTTPTransport implements the synckit.Transport interface for communicating over HTTP.
type HTTPTransport struct {
	client        *http.Client
	baseURL       string // e.g., "http://remote-server.com/sync"
	versionParser VersionParser
	logger        *slog.Logger
	options       *ClientOptions
}

// newHTTPClient creates a custom HTTP client based on ClientOptions.
func newHTTPClient(opts *ClientOptions) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()

	// If DisableAutoDecompression is true, disable Go's automatic decompression
	// so we can enforce both compressed and decompressed size limits
	tr.DisableCompression = opts != nil && opts.DisableAutoDecompression

	return &http.Client{Transport: tr}
}

// NewTransport creates a new HTTPTransport client.
// If a custom http.Client is not provided, one will be created based on the options.
func NewTransport(baseURL string, client *http.Client, parser VersionParser, options *ClientOptions) *HTTPTransport {
	return NewTransportWithLogger(baseURL, client, parser, options, logging.Default().Logger)
}

// NewTransportWithLogger creates a new HTTPTransport client with a custom logger.
func NewTransportWithLogger(baseURL string, client *http.Client, parser VersionParser, options *ClientOptions, logger *slog.Logger) *HTTPTransport {
	if options == nil {
		options = DefaultClientOptions()
	}

	// Fill in zero values with defaults before validation
	if options.MaxResponseSize == 0 {
		options.MaxResponseSize = 10 * 1024 * 1024
	}
	if options.MaxDecompressedResponseSize == 0 {
		options.MaxDecompressedResponseSize = 20 * 1024 * 1024
	}
	// Note: GzipMinBytes can be 0 intentionally, so we don't set a default here
	
	// Validate client options
	if err := ValidateClientOptions(options); err != nil {
		// For backward compatibility, we'll just use default options if validation fails
		// In a future version, we might want to panic or return an error
		options = DefaultClientOptions()
	}

	if client == nil {
		client = newHTTPClient(options)
	}

	if parser == nil {
		// default to integer parser for backward compatibility
		parser = func(ctx context.Context, s string) (synckit.Version, error) {
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, err
			}
			if v == 0 {
				return nil, fmt.Errorf("invalid version: zero is not a valid version")
			}
			return cursor.IntegerCursor{Seq: uint64(v)}, nil
		}
	}

	return &HTTPTransport{
		client:        client,
		baseURL:       baseURL,
		versionParser: parser,
		logger:        logger,
		options:       options,
	}
}

// Push sends a batch of events to the remote server via an HTTP POST request.
//
// Note: If CompressionEnabled is true in ClientOptions and the payload size exceeds 
// GzipMinBytes, the client will compress JSON request bodies using gzip and set the 
// "Content-Encoding: gzip" header. It also sets "Accept-Encoding" header to
// indicate support for gzip and deflate compressed responses.
//
// The server is responsible for decompressing gzip-compressed requests and compressing responses if requested.
//
// This explicit behavior is needed because Go's default http.Client automatically decompresses gzip responses.
// Our client disables that implicit decompression by managing compression explicitly for more control and security.
func (c *HTTPTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	if len(events) == 0 {
		c.logger.Debug("Push called with no events, skipping")
		return nil // Nothing to push
	}

	c.logger.Debug("Starting push operation",
		slog.Int("event_count", len(events)),
		slog.String("base_url", c.baseURL))

	jsonData := make([]JSONEventWithVersion, 0, len(events))
	for _, ev := range events {
		jsonData = append(jsonData, toJSONEventWithVersion(ev))
	}

	payload, err := json.Marshal(jsonData)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to marshal events: %w", err))
	}

	// Prepare the request body (with optional compression)
	var body io.Reader = bytes.NewReader(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/push", body)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to create request: %w", err))
	}

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")

	// Compress request body if enabled and payload size exceeds threshold
	gzipMinBytes := c.options.GzipMinBytes
	if gzipMinBytes == 0 {
		gzipMinBytes = 1024 // Default to 1KB if not set
	}

	if c.options.CompressionEnabled && len(payload) > gzipMinBytes {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(payload); err != nil {
			c.logger.Error("Failed to compress push request",
				slog.String("error", err.Error()),
				slog.Int("original_size", len(payload)))
			return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to compress request: %w", err))
		}
		if err := gw.Close(); err != nil {
			c.logger.Error("Failed to close gzip writer for push request",
				slog.String("error", err.Error()))
			return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("failed to close gzip writer: %w", err))
		}
		
		req.Body = io.NopCloser(&buf)
		req.Header.Set("Content-Encoding", "gzip")
		// Optionally set Content-Length
		req.ContentLength = int64(buf.Len())
		
		c.logger.Debug("Compressed push request",
			slog.Int("original_size", len(payload)),
			slog.Int("compressed_size", buf.Len()),
			slog.Float64("compression_ratio", float64(buf.Len())/float64(len(payload))))
	}

	// Set compression headers for response
	if c.options.CompressionEnabled {
		if c.options.DisableAutoDecompression {
			// Explicitly request gzip when auto-decompression is disabled
			req.Header.Set("Accept-Encoding", "gzip")
		} else {
			// Normal case: let Go handle Accept-Encoding automatically
			req.Header.Set("Accept-Encoding", "gzip, deflate")
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Push request failed",
			slog.String("error", err.Error()),
			slog.String("url", c.baseURL+"/push"))
		return syncErrors.NewRetryable(syncErrors.OpPush, fmt.Errorf("network error: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("Push request returned error status",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
			slog.String("url", c.baseURL+"/push"))
		return syncErrors.NewWithComponent(syncErrors.OpPush, "transport", fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body)))
	}

	c.logger.Debug("Push operation completed successfully",
		slog.Int("event_count", len(events)))
	return nil
}

// Pull fetches events from the remote server since a given version via an HTTP GET request.
func (c *HTTPTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	c.logger.Debug("Starting pull operation",
		slog.String("since_version", since.String()),
		slog.String("base_url", c.baseURL))

	url := fmt.Sprintf("%s/pull?since=%s", c.baseURL, since.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to create request: %w", err))
	}

	// Set Accept-Encoding header if compression is enabled
	if c.options.CompressionEnabled {
		if c.options.DisableAutoDecompression {
			// Explicitly request gzip when auto-decompression is disabled
			req.Header.Set("Accept-Encoding", "gzip")
		} else {
			// Normal case: let Go handle Accept-Encoding automatically
			req.Header.Set("Accept-Encoding", "gzip, deflate")
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Pull request failed",
			slog.String("error", err.Error()),
			slog.String("url", url))
		return nil, syncErrors.NewRetryable(syncErrors.OpPull, fmt.Errorf("network error: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("Pull request returned error status",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
			slog.String("url", url))
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body)))
	}

	// Handle compressed response and enforce size limits
	reader, cleanup, err := createSafeResponseReader(resp, c.options)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to create safe response reader: %w", err))
	}
	defer cleanup()

	var jsonEvents []JSONEventWithVersion
	if err := json.NewDecoder(reader).Decode(&jsonEvents); err != nil {
		// Check if this is a size limit violation
		if errors.Is(err, errResponseDecompressedTooLarge) {
			return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("response decompressed size exceeds limit: %w", err))
		}
		return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("failed to decode response: %w", err))
	}

	events := make([]synckit.EventWithVersion, len(jsonEvents))
	for i, jev := range jsonEvents {
		// Use version parser to decode version
		version, err := c.versionParser(ctx, jev.Version)
		if err != nil {
			return nil, syncErrors.NewWithComponent(syncErrors.OpPull, "transport", fmt.Errorf("invalid version in response: %w", err))
		}

		event := &SimpleEvent{
			IDValue:          jev.Event.ID,
			TypeValue:        jev.Event.Type,
			AggregateIDValue: jev.Event.AggregateID,
			DataValue:        jev.Event.Data,
			MetadataValue:    jev.Event.Metadata,
		}

		events[i] = synckit.EventWithVersion{
			Event:   event,
			Version: version,
		}
	}

	c.logger.Debug("Pull operation completed successfully",
		slog.Int("event_count", len(events)),
		slog.String("since_version", since.String()))
	return events, nil
}

// GetLatestVersion fetches the latest version from the remote server.
func (c *HTTPTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	url := fmt.Sprintf("%s/latest-version", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("failed to create request for latest version: %w", err))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, syncErrors.NewRetryable(syncErrors.OpTransport, fmt.Errorf("network error while getting latest version: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("server error while fetching latest version (status %d): %s", resp.StatusCode, string(body)))
	}

	// Handle compressed response and enforce size limits
	reader, cleanup, err := createSafeResponseReader(resp, c.options)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("failed to create safe response reader: %w", err))
	}
	defer cleanup()

	var versionStr string
	if err := json.NewDecoder(reader).Decode(&versionStr); err != nil {
		// Check if this is a size limit violation
		if errors.Is(err, errResponseDecompressedTooLarge) {
			return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("response decompressed size exceeds limit: %w", err))
		}
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("failed to decode latest version: %w", err))
	}

	version, err := c.versionParser(ctx, versionStr)
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpTransport, "transport", fmt.Errorf("invalid version format: %w", err))
	}

	return version, nil
}

// Subscribe is not supported by this simple HTTP transport.
// Real-time subscriptions would require WebSockets or gRPC streams.
func (c *HTTPTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return syncErrors.New(syncErrors.OpTransport, fmt.Errorf("subscribe is not implemented for HTTP transport"))
}

// Close does nothing for this transport, as the underlying http.Client is managed externally.
func (c *HTTPTransport) Close() error {
	return nil
}
