package httptransport

import (
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

// WithMaxResponseSize sets the maximum allowed size of response bodies
func WithMaxResponseSize(size int64) ClientOption {
    return func(opts *ClientOptions) {
        opts.MaxResponseSize = size
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
