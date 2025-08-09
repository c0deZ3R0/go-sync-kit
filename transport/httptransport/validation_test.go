package httptransport

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientOptionsValidation(t *testing.T) {
	t.Run("ValidOptions", func(t *testing.T) {
		opts := &ClientOptions{
			MaxResponseSize:             10 * 1024 * 1024,
			MaxDecompressedResponseSize: 20 * 1024 * 1024,
			CompressionEnabled:          true,
			DisableAutoDecompression:    false,
		}
		
		err := ValidateClientOptions(opts)
		assert.NoError(t, err)
	})
	
	t.Run("InvalidMaxResponseSize", func(t *testing.T) {
		opts := &ClientOptions{
			MaxResponseSize:             0, // Invalid
			MaxDecompressedResponseSize: 20 * 1024 * 1024,
		}
		
		err := ValidateClientOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxResponseSize must be > 0")
	})
	
	t.Run("InvalidDecompressedSizeRelationship", func(t *testing.T) {
		opts := &ClientOptions{
			MaxResponseSize:             20 * 1024 * 1024,
			MaxDecompressedResponseSize: 10 * 1024 * 1024, // Smaller than MaxResponseSize
			DisableAutoDecompression:    true, // Only when this is true
		}
		
		err := ValidateClientOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDecompressedResponseSize must be >= MaxResponseSize when DisableAutoDecompression is true")
	})
	
	t.Run("ValidDecompressedSizeRelationshipWhenAutoDecompressionEnabled", func(t *testing.T) {
		opts := &ClientOptions{
			MaxResponseSize:             20 * 1024 * 1024,
			MaxDecompressedResponseSize: 10 * 1024 * 1024, // Smaller than MaxResponseSize
			DisableAutoDecompression:    false, // This allows the relationship
		}
		
		err := ValidateClientOptions(opts)
		assert.NoError(t, err)
	})
}

func TestServerOptionsValidation(t *testing.T) {
	t.Run("ValidOptions", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      10 * 1024 * 1024,
			MaxDecompressedSize: 20 * 1024 * 1024,
			CompressionEnabled:  true,
		}
		
		err := ValidateServerOptions(opts)
		assert.NoError(t, err)
	})
	
	t.Run("InvalidMaxRequestSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      0, // Invalid
			MaxDecompressedSize: 20 * 1024 * 1024,
		}
		
		err := ValidateServerOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxRequestSize must be > 0")
	})
	
	t.Run("InvalidDecompressedSizeRelationship", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      20 * 1024 * 1024,
			MaxDecompressedSize: 10 * 1024 * 1024, // Must be >= MaxRequestSize
		}
		
		err := ValidateServerOptions(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDecompressedSize must be >= MaxRequestSize to handle uncompressed requests")
	})
}

func TestNewHTTPClientWithDisableAutoDecompression(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		opts := DefaultClientOptions()
		client := newHTTPClient(opts)
		
		// DisableAutoDecompression should be false by default
		assert.NotNil(t, client)
		assert.NotNil(t, client.Transport)
		
		// Check that auto decompression is NOT disabled (default behavior)
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.False(t, transport.DisableCompression)
	})
	
	t.Run("DisableAutoDecompressionTrue", func(t *testing.T) {
		opts := &ClientOptions{
			DisableAutoDecompression: true,
		}
		client := newHTTPClient(opts)
		
		assert.NotNil(t, client)
		assert.NotNil(t, client.Transport)
		
		// Check that auto decompression is disabled
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.True(t, transport.DisableCompression)
	})
	
	t.Run("DisableAutoDecompressionFalse", func(t *testing.T) {
		opts := &ClientOptions{
			DisableAutoDecompression: false,
		}
		client := newHTTPClient(opts)
		
		assert.NotNil(t, client)
		assert.NotNil(t, client.Transport)
		
		// Check that auto decompression is NOT disabled
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.False(t, transport.DisableCompression)
	})
}

func TestNewTransportValidatesClientOptions(t *testing.T) {
	t.Run("ValidOptions", func(t *testing.T) {
		opts := &ClientOptions{
			MaxResponseSize:             10 * 1024 * 1024,
			MaxDecompressedResponseSize: 20 * 1024 * 1024,
			CompressionEnabled:          true,
		}
		
		transport := NewTransport("http://example.com", nil, nil, opts)
		assert.NotNil(t, transport)
		// Should use the provided options
		assert.Equal(t, opts, transport.options)
	})
	
	t.Run("InvalidOptionsUsesDefaults", func(t *testing.T) {
		opts := &ClientOptions{
			MaxResponseSize:             0, // Invalid
			MaxDecompressedResponseSize: 20 * 1024 * 1024,
		}
		
		transport := NewTransport("http://example.com", nil, nil, opts)
		assert.NotNil(t, transport)
		// Should fall back to default options
		expected := DefaultClientOptions()
		assert.Equal(t, expected.MaxResponseSize, transport.options.MaxResponseSize)
		assert.Equal(t, expected.MaxDecompressedResponseSize, transport.options.MaxDecompressedResponseSize)
	})
}
