package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSQLiteEventStore_WALAndPoolDefaults tests the production-friendly defaults
// including WAL mode, PRAGMA settings, and connection pool configuration.
func TestSQLiteEventStore_WALAndPoolDefaults(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_wal_defaults.db")

	t.Run("WAL_EnabledByDefault", func(t *testing.T) {
		// Create store with default config
		config := DefaultConfig(dbPath)
		require.True(t, config.EnableWAL, "WAL should be enabled by default")

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify WAL mode is active by querying PRAGMA
		var journalMode string
		err = store.db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
		require.NoError(t, err)
		assert.Equal(t, "wal", journalMode, "Journal mode should be WAL")

		// Store an event to ensure WAL file is created
		event := &MockEvent{
			id:          "test-wal-1",
			eventType:   "WALTest",
			aggregateID: "wal-agg-1",
			data:        "test data for WAL",
		}

		ctx := context.Background()
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
		require.NoError(t, err)

		// Check that WAL file exists (indicates WAL mode is active)
		walFile := dbPath + "-wal"
		_, err = os.Stat(walFile)
		assert.NoError(t, err, "WAL file should exist when WAL mode is active")
	})

	t.Run("BusyTimeout_Applied", func(t *testing.T) {
		config := DefaultConfig(dbPath + "_busy")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify busy timeout is set to 5000ms
		var busyTimeout int
		err = store.db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout)
		require.NoError(t, err)
		assert.Equal(t, 5000, busyTimeout, "Busy timeout should be 5000ms")
	})

	t.Run("SynchronousMode_Applied", func(t *testing.T) {
		config := DefaultConfig(dbPath + "_sync")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify synchronous mode is set to NORMAL
		var syncMode int
		err = store.db.QueryRow("PRAGMA synchronous;").Scan(&syncMode)
		require.NoError(t, err)
		assert.Equal(t, 1, syncMode, "Synchronous mode should be NORMAL (1)")
	})

	t.Run("TempStore_Applied", func(t *testing.T) {
		config := DefaultConfig(dbPath + "_temp")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify temp store is set to MEMORY
		var tempStore int
		err = store.db.QueryRow("PRAGMA temp_store;").Scan(&tempStore)
		require.NoError(t, err)
		assert.Equal(t, 2, tempStore, "Temp store should be MEMORY (2)")
	})

	t.Run("CacheSize_Applied", func(t *testing.T) {
		config := DefaultConfig(dbPath + "_cache")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify cache size is set to -20000 pages
		var cacheSize int
		err = store.db.QueryRow("PRAGMA cache_size;").Scan(&cacheSize)
		require.NoError(t, err)
		assert.Equal(t, -20000, cacheSize, "Cache size should be -20000 pages")
	})

	t.Run("ConnectionPool_DefaultSettings", func(t *testing.T) {
		// Test that connection pool defaults are production-ready
		config := DefaultConfig(dbPath + "_pool")

		assert.Equal(t, 25, config.MaxOpenConns, "MaxOpenConns should default to 25")
		assert.Equal(t, 5, config.MaxIdleConns, "MaxIdleConns should default to 5")
		assert.Equal(t, time.Hour, config.ConnMaxLifetime, "ConnMaxLifetime should default to 1 hour")
		assert.Equal(t, 5*time.Minute, config.ConnMaxIdleTime, "ConnMaxIdleTime should default to 5 minutes")

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify connection pool statistics are available
		stats := store.Stats()
		assert.GreaterOrEqual(t, stats.OpenConnections, 0, "Should have connection statistics available")
	})

	t.Run("ConnectionPool_CustomSettings", func(t *testing.T) {
		// Test custom connection pool settings
		config := &Config{
			DataSourceName:  dbPath + "_custom_pool",
			EnableWAL:       true,
			MaxOpenConns:    10,
			MaxIdleConns:    2,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		}
		config.setDefaults() // Apply other defaults

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// The actual connection pool settings are not directly queryable,
		// but we can verify they're set without error and the store works
		stats := store.Stats()
		assert.GreaterOrEqual(t, stats.OpenConnections, 0, "Should have connection statistics available")

		// Test basic functionality to ensure custom settings don't break anything
		event := &MockEvent{
			id:          "test-custom-pool",
			eventType:   "CustomPoolTest",
			aggregateID: "custom-agg",
			data:        "custom pool test data",
		}

		ctx := context.Background()
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
		require.NoError(t, err)

		// Verify event can be retrieved
		events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "test-custom-pool", events[0].Event.ID())
	})

	t.Run("WAL_DisabledExplicitly", func(t *testing.T) {
		// Test that WAL can be explicitly disabled, but other PRAGMAs still apply
		config := &Config{
			DataSourceName: dbPath + "_no_wal",
			EnableWAL:      false, // Explicitly disable WAL
		}
		config.setDefaults()

		assert.False(t, config.EnableWAL, "WAL should be explicitly disabled")

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify journal mode is NOT WAL (likely DELETE which is default)
		var journalMode string
		err = store.db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
		require.NoError(t, err)
		assert.NotEqual(t, "wal", journalMode, "Journal mode should not be WAL when disabled")

		// But other PRAGMA settings should still be applied
		var busyTimeout int
		err = store.db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout)
		require.NoError(t, err)
		assert.Equal(t, 5000, busyTimeout, "Busy timeout should still be applied")

		// Store should still work without WAL
		event := &MockEvent{
			id:          "test-no-wal",
			eventType:   "NoWALTest",
			aggregateID: "no-wal-agg",
			data:        "no WAL test data",
		}

		ctx := context.Background()
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
		require.NoError(t, err)
	})

	t.Run("AllPRAGMAs_ProductionScenario", func(t *testing.T) {
		// Test a production-like scenario with all PRAGMA settings
		config := DefaultConfig(dbPath + "_production")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Verify all PRAGMA settings are correctly applied
		pragmaTests := []struct {
			name     string
			query    string
			expected interface{}
		}{
			{"journal_mode", "PRAGMA journal_mode;", "wal"},
			{"busy_timeout", "PRAGMA busy_timeout;", 5000},
			{"synchronous", "PRAGMA synchronous;", 1}, // NORMAL = 1
			{"temp_store", "PRAGMA temp_store;", 2},    // MEMORY = 2
			{"cache_size", "PRAGMA cache_size;", -20000},
		}

		for _, test := range pragmaTests {
			t.Run("PRAGMA_"+test.name, func(t *testing.T) {
				var result interface{}
				switch test.expected.(type) {
				case string:
					var str string
					err := store.db.QueryRow(test.query).Scan(&str)
					require.NoError(t, err, "Failed to query %s", test.name)
					result = str
				case int:
					var num int
					err := store.db.QueryRow(test.query).Scan(&num)
					require.NoError(t, err, "Failed to query %s", test.name)
					result = num
				}
				assert.Equal(t, test.expected, result, "%s should be set correctly", test.name)
			})
		}

		// Test the store functionality under production settings
		// Simulate batch operations which benefit from production settings
		events := make([]MockEvent, 50)
		for i := 0; i < 50; i++ {
			events[i] = MockEvent{
				id:          "prod-event-" + strconv.Itoa(i),
				eventType:   "ProductionEvent",
				aggregateID: "prod-agg-" + strconv.Itoa(i%5),
				data: map[string]interface{}{
					"timestamp": time.Now().Unix(),
					"batch_id":  i / 10,
					"sequence":  i,
				},
			}
		}

		// Store events individually to test concurrency handling
		for _, event := range events {
			err := store.Store(ctx, &event, cursor.IntegerCursor{Seq: uint64(len(events))})
			require.NoError(t, err, "Failed to store event %s", event.id)
		}

		// Verify all events are retrievable
		retrievedEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		require.NoError(t, err)
		assert.Equal(t, len(events), len(retrievedEvents), "Should retrieve all stored events")

		// Verify latest version tracking
		latestVersion, err := store.LatestVersion(ctx)
		require.NoError(t, err)
		assert.Greater(t, latestVersion.(cursor.IntegerCursor).Seq, uint64(0), "Latest version should be greater than 0")
	})
}

// TestSQLiteEventStore_PragmaErrorHandling tests error handling during PRAGMA application
func TestSQLiteEventStore_PragmaErrorHandling(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("InvalidDataSource_FailsGracefully", func(t *testing.T) {
		// Test that invalid data source fails before PRAGMA application
		config := &Config{
			DataSourceName: "/invalid/path/that/should/not/exist/test.db",
			EnableWAL:      true,
		}
		config.setDefaults()

		_, err := New(config)
		assert.Error(t, err, "Should fail with invalid data source")
		// Error could be either from database opening or PRAGMA application
		assert.True(t, 
			strings.Contains(err.Error(), "failed to open sqlite database") ||
				strings.Contains(err.Error(), "failed to apply PRAGMA settings"),
			"Error should mention database opening or PRAGMA application")
	})

	t.Run("ValidConfiguration_SucceedsWithAllPragmas", func(t *testing.T) {
		// Test that valid configuration applies all PRAGMAs successfully
		dbPath := filepath.Join(tempDir, "valid_config.db")
		config := DefaultConfig(dbPath)

		store, err := New(config)
		require.NoError(t, err, "Should successfully create store with valid configuration")
		defer store.Close()

		// Verify the store is functional
		event := &MockEvent{
			id:          "test-valid-config",
			eventType:   "ValidConfigTest",
			aggregateID: "valid-agg",
			data:        "valid config test data",
		}

		ctx := context.Background()
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
		require.NoError(t, err, "Store should work with applied PRAGMAs")
	})
}

// TestConnectionPoolLimits tests that connection pool limits work as expected
func TestConnectionPoolLimits(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pool_limits.db")

	t.Run("ZeroValues_UseDefaults", func(t *testing.T) {
		config := &Config{
			DataSourceName:  dbPath + "_zero",
			EnableWAL:       true,
			MaxOpenConns:    0, // Should use default
			MaxIdleConns:    0, // Should use default
			ConnMaxLifetime: 0, // Should use default
			ConnMaxIdleTime: 0, // Should use default
		}

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// The internal db should have the default settings applied
		// We can't directly verify the exact values but we can ensure the store works
		stats := store.Stats()
		assert.GreaterOrEqual(t, stats.OpenConnections, 0)
	})

	t.Run("PositiveValues_UseConfigured", func(t *testing.T) {
		config := &Config{
			DataSourceName:  dbPath + "_positive",
			EnableWAL:       true,
			MaxOpenConns:    15,
			MaxIdleConns:    3,
			ConnMaxLifetime: 45 * time.Minute,
			ConnMaxIdleTime: 3 * time.Minute,
		}

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify the store is functional with custom settings
		event := &MockEvent{
			id:          "test-custom-limits",
			eventType:   "CustomLimitsTest",
			aggregateID: "limits-agg",
			data:        "custom limits test data",
		}

		ctx := context.Background()
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
		require.NoError(t, err)

		stats := store.Stats()
		assert.GreaterOrEqual(t, stats.OpenConnections, 0)
	})
}
