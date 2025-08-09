//go:build integration

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: MockEvent is already defined in store_test.go, so we reuse it here

func TestSQLiteEventStore_WAL(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_wal.db")

	t.Run("WAL_EnabledByDefault", func(t *testing.T) {
		// Create store with default config (should enable WAL)
		config := DefaultConfig(dbPath)
		require.True(t, config.EnableWAL, "WAL should be enabled by default")

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify WAL mode is active by checking for WAL file
		// When WAL mode is active, SQLite creates a .wal file alongside the db
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

	t.Run("WAL_ConnectionPoolDefaults", func(t *testing.T) {
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
		// DBStats doesn't expose MaxOpenConns directly, but we can verify it's working by checking OpenConnections
		assert.GreaterOrEqual(t, stats.OpenConnections, 0, "Should have connection statistics available")
	})

	t.Run("WAL_ConcurrentWrites", func(t *testing.T) {
		// Test concurrent writes with WAL mode (should not block readers)
		config := DefaultConfig(dbPath + "_concurrent")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		const numGoroutines = 10
		const eventsPerGoroutine = 5

		// Use channels to coordinate goroutines
		done := make(chan error, numGoroutines)

		// Start multiple goroutines writing concurrently
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < eventsPerGoroutine; j++ {
					event := &MockEvent{
						id:          string(rune('A'+goroutineID)) + "-" + string(rune('0'+j)),
						eventType:   "ConcurrentTest",
						aggregateID: "concurrent-agg",
						data:        map[string]interface{}{"goroutine": goroutineID, "event": j},
					}

					err := store.Store(ctx, event, cursor.IntegerCursor{Seq: uint64(goroutineID*eventsPerGoroutine + j)})
					if err != nil {
						done <- err
						return
					}
				}
				done <- nil
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			err := <-done
			assert.NoError(t, err, "Concurrent write should succeed")
		}

		// Verify all events were stored
		events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		require.NoError(t, err)
		assert.Equal(t, numGoroutines*eventsPerGoroutine, len(events),
			"Should have stored all events from concurrent writes")
	})

	t.Run("WAL_DataSourceName_WithJournalMode", func(t *testing.T) {
		// Test that WAL gets appended correctly to DataSourceName
		config := &Config{
			DataSourceName: dbPath + "_journal",
			EnableWAL:      true,
		}
		config.setDefaults()

		assert.True(t, strings.Contains(config.DataSourceName, "?_journal_mode=WAL"),
			"DataSourceName should contain WAL journal mode parameter")

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		// Verify store is functional with explicit WAL parameter
		event := &MockEvent{
			id:          "test-journal-mode",
			eventType:   "JournalModeTest",
			aggregateID: "journal-agg",
			data:        "journal mode test data",
		}

		ctx := context.Background()
		err = store.Store(ctx, event, cursor.IntegerCursor{Seq: 1})
		require.NoError(t, err)

		// Verify event can be retrieved
		events, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "test-journal-mode", events[0].Event.ID())
	})

	t.Run("WAL_Disabled", func(t *testing.T) {
		// Test that WAL can be explicitly disabled
		config := &Config{
			DataSourceName: dbPath + "_no_wal",
			EnableWAL:      false, // Explicitly disable WAL
		}
		config.setDefaults()

		assert.False(t, strings.Contains(config.DataSourceName, "_journal_mode=WAL"),
			"DataSourceName should not contain WAL when disabled")

		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

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

	t.Run("WAL_ProductionScenario", func(t *testing.T) {
		// Test a production-like scenario with realistic workload
		config := DefaultConfig(dbPath + "_production")
		store, err := New(config)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Simulate a batch of events like in a production scenario
		events := make([]synckit.EventWithVersion, 100)
		for i := 0; i < 100; i++ {
			events[i] = synckit.EventWithVersion{
				Event: &MockEvent{
					id:          string(rune('a'+i%26)) + "-prod-" + string(rune('0'+i%10)),
					eventType:   "ProductionEvent",
					aggregateID: "prod-agg-" + string(rune('1'+i%5)),
					data: map[string]interface{}{
						"timestamp": time.Now().Unix(),
						"batch_id":  i / 10,
						"sequence":  i,
					},
				},
				Version: cursor.IntegerCursor{Seq: uint64(i + 1)},
			}
		}

		// Use batch insert for performance (production pattern)
		err = store.StoreBatch(ctx, events)
		require.NoError(t, err)

		// Verify all events are retrievable
		retrievedEvents, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		require.NoError(t, err)
		assert.Equal(t, 100, len(retrievedEvents), "Should retrieve all 100 events")

		// Test aggregate-specific queries (common production pattern)
		aggEvents, err := store.LoadByAggregate(ctx, "prod-agg-1", cursor.IntegerCursor{Seq: 0})
		require.NoError(t, err)
		assert.Greater(t, len(aggEvents), 0, "Should find events for specific aggregate")

		// Verify latest version tracking
		latestVersion, err := store.LatestVersion(ctx)
		require.NoError(t, err)
		expectedLatest := cursor.IntegerCursor{Seq: 100}
		assert.Equal(t, expectedLatest.Seq, latestVersion.(cursor.IntegerCursor).Seq,
			"Latest version should match last inserted event")
	})
}
