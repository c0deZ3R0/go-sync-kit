package httptransport

import (
	"fmt"
	"time"
)

// ServerOption is a function that configures a ServerOptions struct
type ServerOption func(*ServerOptions)

// WithMaxRequestSize sets the maximum allowed size of incoming request bodies
func WithMaxRequestSize(size int64) ServerOption {
	return func(opts *ServerOptions) {
		opts.MaxRequestSize = size
	}
}

// WithMaxDecompressedSize sets the maximum allowed size of decompressed request bodies
func WithMaxDecompressedSize(size int64) ServerOption {
	return func(opts *ServerOptions) {
		opts.MaxDecompressedSize = size
	}
}

// WithCompression enables or disables response compression
func WithCompression(enabled bool) ServerOption {
	return func(opts *ServerOptions) {
		opts.CompressionEnabled = enabled
	}
}

// WithCompressionThreshold sets the minimum size for response compression
func WithCompressionThreshold(size int64) ServerOption {
	return func(opts *ServerOptions) {
		opts.CompressionThreshold = size
	}
}

// WithRequestTimeout sets the maximum duration for request processing
func WithRequestTimeout(timeout time.Duration) ServerOption {
	return func(opts *ServerOptions) {
		opts.RequestTimeout = timeout
	}
}

// WithShutdownTimeout sets the maximum duration for graceful shutdown
func WithShutdownTimeout(timeout time.Duration) ServerOption {
	return func(opts *ServerOptions) {
		opts.ShutdownTimeout = timeout
	}
}

// ClientOption is a function that configures a ClientOptions struct
type ClientOption func(*ClientOptions)

// WithClientCompression enables or disables request/response compression
func WithClientCompression(enabled bool) ClientOption {
	return func(opts *ClientOptions) {
		opts.CompressionEnabled = enabled
	}
}

// WithDisableAutoDecompression disables Go's automatic response decompression
func WithDisableAutoDecompression(disabled bool) ClientOption {
	return func(opts *ClientOptions) {
		opts.DisableAutoDecompression = disabled
	}
}

// WithMaxResponseSize sets the maximum allowed size of response bodies
func WithMaxResponseSize(size int64) ClientOption {
	return func(opts *ClientOptions) {
		opts.MaxResponseSize = size
	}
}

// WithMaxDecompressedResponseSize sets the maximum allowed size of decompressed response bodies
func WithMaxDecompressedResponseSize(size int64) ClientOption {
	return func(opts *ClientOptions) {
		opts.MaxDecompressedResponseSize = size
	}
}

// WithRetryConfig sets the retry configuration for failed requests
func WithRetryConfig(maxAttempts int, waitMin, waitMax time.Duration) ClientOption {
	return func(opts *ClientOptions) {
		opts.RetryMax = maxAttempts
		opts.RetryWaitMin = waitMin
		opts.RetryWaitMax = waitMax
	}
}

// WithClientTimeout sets the timeout for all requests
func WithClientTimeout(timeout time.Duration) ClientOption {
	return func(opts *ClientOptions) {
		opts.RequestTimeout = timeout
	}
}

// applyServerOptions creates a new ServerOptions with the given options applied
func applyServerOptions(opts ...ServerOption) *ServerOptions {
	// Start with default options
	options := DefaultServerOptions()

	// Apply each option
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// applyClientOptions creates a new ClientOptions with the given options applied
func applyClientOptions(opts ...ClientOption) *ClientOptions {
	// Start with default options
	options := DefaultClientOptions()

	// Apply each option
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// ValidateServerOptions validates server options configuration
func ValidateServerOptions(opts *ServerOptions) error {
	if opts == nil {
		return nil
	}

	if opts.MaxRequestSize <= 0 {
		return fmt.Errorf("MaxRequestSize must be > 0")
	}

	if opts.MaxDecompressedSize <= 0 {
		return fmt.Errorf("MaxDecompressedSize must be > 0")
	}

	if opts.MaxDecompressedSize < opts.MaxRequestSize {
		return fmt.Errorf("MaxDecompressedSize must be >= MaxRequestSize to handle uncompressed requests")
	}

	if opts.CompressionThreshold < 0 {
		return fmt.Errorf("CompressionThreshold must be >= 0")
	}

	if opts.RequestTimeout < 0 {
		return fmt.Errorf("RequestTimeout must be >= 0")
	}

	if opts.ShutdownTimeout < 0 {
		return fmt.Errorf("ShutdownTimeout must be >= 0")
	}

	return nil
}

// ValidateClientOptions validates client options configuration
func ValidateClientOptions(opts *ClientOptions) error {
	if opts == nil {
		return nil
	}

	if opts.MaxResponseSize <= 0 {
		return fmt.Errorf("MaxResponseSize must be > 0")
	}

	if opts.MaxDecompressedResponseSize <= 0 {
		return fmt.Errorf("MaxDecompressedResponseSize must be > 0")
	}

	// Only enforce this relationship when DisableAutoDecompression is true
	// because that's when we can observe compressed bytes
	if opts.DisableAutoDecompression && opts.MaxDecompressedResponseSize < opts.MaxResponseSize {
		return fmt.Errorf("MaxDecompressedResponseSize must be >= MaxResponseSize when DisableAutoDecompression is true")
	}

	if opts.RequestTimeout < 0 {
		return fmt.Errorf("RequestTimeout must be >= 0")
	}

	if opts.RetryMax < 0 {
		return fmt.Errorf("RetryMax must be >= 0")
	}

	if opts.RetryWaitMin < 0 {
		return fmt.Errorf("RetryWaitMin must be >= 0")
	}

	if opts.RetryWaitMax < 0 {
		return fmt.Errorf("RetryWaitMax must be >= 0")
	}

	if opts.RetryWaitMax > 0 && opts.RetryWaitMin > opts.RetryWaitMax {
		return fmt.Errorf("RetryWaitMin must be <= RetryWaitMax")
	}

	return nil
}
