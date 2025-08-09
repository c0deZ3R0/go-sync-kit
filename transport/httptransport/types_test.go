package httptransport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerOptions_Validate(t *testing.T) {
	t.Run("ValidDefaultOptions", func(t *testing.T) {
		opts := DefaultServerOptions()
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("NilOptions", func(t *testing.T) {
		var opts *ServerOptions
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil server options")
	})

	t.Run("ZeroMaxRequestSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      0,
			MaxDecompressedSize: 1024,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxRequestSize must be positive")
	})

	t.Run("NegativeMaxRequestSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      -1,
			MaxDecompressedSize: 1024,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxRequestSize must be positive")
	})

	t.Run("ZeroMaxDecompressedSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 0,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDecompressedSize must be positive")
	})

	t.Run("NegativeMaxDecompressedSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: -1,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDecompressedSize must be positive")
	})

	t.Run("MaxDecompressedSize_SmallerThan_MaxRequestSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      2048, // 2KB
			MaxDecompressedSize: 1024, // 1KB - smaller than MaxRequestSize
			CompressionEnabled:  true,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxDecompressedSize (1024) must be >= MaxRequestSize (2048) to prevent zip-bomb attacks")
	})

	t.Run("MaxDecompressedSize_EqualTo_MaxRequestSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 1024, // Equal to MaxRequestSize - should be valid
			CompressionEnabled:  true,
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("MaxDecompressedSize_LargerThan_MaxRequestSize", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 2048, // Larger than MaxRequestSize - should be valid
			CompressionEnabled:  true,
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("NegativeCompressionThreshold_WithCompressionEnabled", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:       1024,
			MaxDecompressedSize:  2048,
			CompressionEnabled:   true,
			CompressionThreshold: -1,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CompressionThreshold cannot be negative when compression is enabled")
	})

	t.Run("NegativeCompressionThreshold_WithCompressionDisabled", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:       1024,
			MaxDecompressedSize:  2048,
			CompressionEnabled:   false,
			CompressionThreshold: -1, // Should be ignored when compression is disabled
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("ZeroCompressionThreshold_WithCompressionEnabled", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:       1024,
			MaxDecompressedSize:  2048,
			CompressionEnabled:   true,
			CompressionThreshold: 0, // Zero should be valid
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("NegativeRequestTimeout", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 2048,
			RequestTimeout:      -1 * time.Second,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RequestTimeout cannot be negative")
	})

	t.Run("ZeroRequestTimeout", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 2048,
			RequestTimeout:      0, // Zero timeout should be valid
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("NegativeShutdownTimeout", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 2048,
			ShutdownTimeout:     -1 * time.Second,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ShutdownTimeout cannot be negative")
	})

	t.Run("ZeroShutdownTimeout", func(t *testing.T) {
		opts := &ServerOptions{
			MaxRequestSize:      1024,
			MaxDecompressedSize: 2048,
			ShutdownTimeout:     0, // Zero timeout should be valid
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})
}

func TestDefaultServerOptions_Values(t *testing.T) {
	opts := DefaultServerOptions()

	// Test specific default values
	assert.Equal(t, int64(10*1024*1024), opts.MaxRequestSize, "MaxRequestSize should be 10 MiB")
	assert.Equal(t, int64(20*1024*1024), opts.MaxDecompressedSize, "MaxDecompressedSize should be 20 MiB")
	assert.True(t, opts.CompressionEnabled, "CompressionEnabled should be true")
	assert.Equal(t, int64(1024), opts.CompressionThreshold, "CompressionThreshold should be 1 KiB")
	assert.Equal(t, 30*time.Second, opts.RequestTimeout, "RequestTimeout should be 30s")
	assert.Equal(t, 10*time.Second, opts.ShutdownTimeout, "ShutdownTimeout should be 10s")

	// Test that the defaults satisfy the validation rules
	require.NoError(t, opts.Validate(), "Default server options should be valid")

	// Test the critical relationship: MaxDecompressedSize >= MaxRequestSize
	assert.GreaterOrEqual(t, opts.MaxDecompressedSize, opts.MaxRequestSize,
		"MaxDecompressedSize should be >= MaxRequestSize to prevent zip-bomb attacks")
}

func TestDefaultServerOptions_DocumentedInterplay(t *testing.T) {
	opts := DefaultServerOptions()

	// Test documented interplay with WireCursor validation
	// WireCursor has a max size of 64 KiB (from cursor package)
	const maxWireCursorSize = 64 * 1024 // 64 KiB

	t.Run("MaxDecompressedSize_ComfortablyExceedsWireCursorLimit", func(t *testing.T) {
		// MaxDecompressedSize should be much larger than WireCursor limit to handle JSON overhead
		assert.Greater(t, opts.MaxDecompressedSize, int64(maxWireCursorSize*10),
			"MaxDecompressedSize should comfortably exceed WireCursor limit (64 KiB) to handle JSON overhead in batch pulls")
	})

	t.Run("HeadroomForBatchOperations", func(t *testing.T) {
		// Test that we have comfortable headroom for batch operations with JSON serialization overhead
		// A typical batch pull might have multiple events, each with JSON overhead
		// Our 20 MiB limit should comfortably handle this vs the 64 KiB cursor limit
		expectedOverhead := opts.MaxDecompressedSize / maxWireCursorSize
		assert.Greater(t, expectedOverhead, int64(100),
			"MaxDecompressedSize should provide >100x headroom over WireCursor limit for JSON overhead")
	})
}

func TestDefaultServerOptions_PanicsOnInvalidDefaults(t *testing.T) {
	// This test is more for documentation purposes - it shows that if we ever
	// accidentally create invalid defaults, the DefaultServerOptions function will panic
	// This is intentional to catch configuration errors early in development

	// We can't easily test this without modifying the source, but we can document the expectation
	t.Run("ValidDefaults", func(t *testing.T) {
		// This should not panic
		assert.NotPanics(t, func() {
			DefaultServerOptions()
		}, "DefaultServerOptions should not panic with valid configuration")
	})
}
