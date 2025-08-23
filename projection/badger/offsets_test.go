package badger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// createTestStore creates a temporary BadgerDB store for testing
func createTestStore(t *testing.T) (*OffsetStore, func()) {
	tempDir, err := os.MkdirTemp("", "badger_offset_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := DefaultConfig(tempDir)
	store, err := NewOffsetStore(config, testParseVersion)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create BadgerDB store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

// testParseVersion is a test version parser similar to SQLite store
func testParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	if versionStr == "" || versionStr == "0" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	val, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer version string '%s': %w", versionStr, err)
	}

	return cursor.IntegerCursor{Seq: uint64(val)}, nil
}

func TestNewOffsetStore(t *testing.T) {
	t.Run("ValidConstruction", func(t *testing.T) {
		store, cleanup := createTestStore(t)
		defer cleanup()

		if store == nil {
			t.Fatal("Expected offset store, got nil")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := NewOffsetStore(nil, testParseVersion)
		if err == nil {
			t.Fatal("Expected error for nil config")
		}
		if !containsString(err.Error(), "config cannot be nil") {
			t.Fatalf("Expected config nil error, got: %v", err)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		config := &Config{Path: ""}
		_, err := NewOffsetStore(config, testParseVersion)
		if err == nil {
			t.Fatal("Expected error for empty path")
		}
		if !containsString(err.Error(), "config.Path cannot be empty") {
			t.Fatalf("Expected path empty error, got: %v", err)
		}
	})

	t.Run("NilParseVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "badger_test_*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		config := DefaultConfig(tempDir)
		_, err = NewOffsetStore(config, nil)
		if err == nil {
			t.Fatal("Expected error for nil parseVersion")
		}
		if !containsString(err.Error(), "parseVersion function cannot be nil") {
			t.Fatalf("Expected parseVersion nil error, got: %v", err)
		}
	})

	t.Run("WithOptions", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "badger_test_*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logger := slog.Default()
		config := DefaultConfig(tempDir)
		store, err := NewOffsetStore(config, testParseVersion, WithLogger(logger))
		if err != nil {
			t.Fatalf("Expected no error creating offset store with options, got: %v", err)
		}
		defer store.Close()

		if store.logger != logger {
			t.Fatalf("Expected custom logger to be set")
		}
	})
}

func TestOffsetStore_GetSet(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	projectionName := "test-projection"

	// Test Get when no offset exists
	t.Run("GetNoOffset", func(t *testing.T) {
		offset, err := store.Get(ctx, projectionName)
		if err != nil {
			t.Fatalf("Expected no error for missing offset, got: %v", err)
		}
		if offset != nil {
			t.Fatalf("Expected nil offset, got: %v", offset)
		}
	})

	// Test Set and Get
	t.Run("SetAndGet", func(t *testing.T) {
		testVersion := cursor.IntegerCursor{Seq: 42}

		// Set the offset
		err := store.Set(ctx, projectionName, testVersion)
		if err != nil {
			t.Fatalf("Expected no error setting offset, got: %v", err)
		}

		// Get the offset back
		offset, err := store.Get(ctx, projectionName)
		if err != nil {
			t.Fatalf("Expected no error getting offset, got: %v", err)
		}
		if offset == nil {
			t.Fatal("Expected offset, got nil")
		}

		// Verify the offset value
		intOffset, ok := offset.(cursor.IntegerCursor)
		if !ok {
			t.Fatalf("Expected IntegerCursor, got: %T", offset)
		}
		if intOffset.Seq != 42 {
			t.Fatalf("Expected seq 42, got: %d", intOffset.Seq)
		}
	})

	// Test Update existing offset
	t.Run("UpdateOffset", func(t *testing.T) {
		newVersion := cursor.IntegerCursor{Seq: 100}

		// Update the offset
		err := store.Set(ctx, projectionName, newVersion)
		if err != nil {
			t.Fatalf("Expected no error updating offset, got: %v", err)
		}

		// Verify the updated value
		offset, err := store.Get(ctx, projectionName)
		if err != nil {
			t.Fatalf("Expected no error getting updated offset, got: %v", err)
		}

		intOffset := offset.(cursor.IntegerCursor)
		if intOffset.Seq != 100 {
			t.Fatalf("Expected seq 100, got: %d", intOffset.Seq)
		}
	})
}

func TestOffsetStore_Validation(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("EmptyProjectionName_Get", func(t *testing.T) {
		_, err := store.Get(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty projection name")
		}
	})

	t.Run("EmptyProjectionName_Set", func(t *testing.T) {
		err := store.Set(ctx, "", cursor.IntegerCursor{Seq: 1})
		if err == nil {
			t.Fatal("Expected error for empty projection name")
		}
	})

	t.Run("NilVersion_Set", func(t *testing.T) {
		err := store.Set(ctx, "test", nil)
		if err == nil {
			t.Fatal("Expected error for nil version")
		}
	})
}

func TestOffsetStore_ListProjections(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should be empty
	projections, err := store.ListProjections(ctx)
	if err != nil {
		t.Fatalf("Expected no error listing empty projections, got: %v", err)
	}
	if len(projections) != 0 {
		t.Fatalf("Expected empty list, got: %v", projections)
	}

	// Add some projections
	testProjections := []string{"proj1", "proj2", "proj3"}
	for i, name := range testProjections {
		version := cursor.IntegerCursor{Seq: uint64(i + 1)}
		err := store.Set(ctx, name, version)
		if err != nil {
			t.Fatalf("Failed to set projection %s: %v", name, err)
		}
	}

	// List should contain all projections
	projections, err = store.ListProjections(ctx)
	if err != nil {
		t.Fatalf("Expected no error listing projections, got: %v", err)
	}
	if len(projections) != len(testProjections) {
		t.Fatalf("Expected %d projections, got %d", len(testProjections), len(projections))
	}

	// Verify all projections are present (order might be different)
	projectionSet := make(map[string]bool)
	for _, p := range projections {
		projectionSet[p] = true
	}
	for _, expected := range testProjections {
		if !projectionSet[expected] {
			t.Fatalf("Expected projection %s not found in list", expected)
		}
	}
}

func TestOffsetStore_Reset(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	projectionName := "test-projection"
	version := cursor.IntegerCursor{Seq: 42}

	// Set an offset
	err := store.Set(ctx, projectionName, version)
	if err != nil {
		t.Fatalf("Failed to set offset: %v", err)
	}

	// Verify it exists
	offset, err := store.Get(ctx, projectionName)
	if err != nil || offset == nil {
		t.Fatalf("Expected offset to exist")
	}

	// Reset the projection
	err = store.Reset(ctx, projectionName)
	if err != nil {
		t.Fatalf("Expected no error resetting projection, got: %v", err)
	}

	// Verify it's gone
	offset, err = store.Get(ctx, projectionName)
	if err != nil {
		t.Fatalf("Expected no error getting reset projection, got: %v", err)
	}
	if offset != nil {
		t.Fatalf("Expected nil offset after reset, got: %v", offset)
	}

	// Test reset non-existent projection
	err = store.Reset(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Expected no error resetting non-existent projection, got: %v", err)
	}
}

func TestOffsetStore_ConcurrentAccess(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	const numWorkers = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Test concurrent writes to different projections
	t.Run("ConcurrentWrites", func(t *testing.T) {
		for workerID := 0; workerID < numWorkers; workerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				projectionName := fmt.Sprintf("projection-%d", id)

				for i := 0; i < numOperations; i++ {
					version := cursor.IntegerCursor{Seq: uint64(i + 1)}
					err := store.Set(ctx, projectionName, version)
					if err != nil {
						t.Errorf("Worker %d failed to set offset at iteration %d: %v", id, i, err)
						return
					}
				}
			}(workerID)
		}
		wg.Wait()

		// Verify final state
		for workerID := 0; workerID < numWorkers; workerID++ {
			projectionName := fmt.Sprintf("projection-%d", workerID)
			offset, err := store.Get(ctx, projectionName)
			if err != nil {
				t.Fatalf("Failed to get offset for %s: %v", projectionName, err)
			}
			if offset == nil {
				t.Fatalf("Expected offset for %s, got nil", projectionName)
			}

			intOffset := offset.(cursor.IntegerCursor)
			if intOffset.Seq != numOperations {
				t.Fatalf("Expected final seq %d for %s, got: %d", numOperations, projectionName, intOffset.Seq)
			}
		}
	})

	// Test concurrent read/write to same projection
	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		projectionName := "shared-projection"
		var readCount, writeCount int64

		// Writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					version := cursor.IntegerCursor{Seq: uint64(time.Now().UnixNano() % 1000)}
					err := store.Set(ctx, projectionName, version)
					if err != nil {
						t.Errorf("Failed to write in concurrent test: %v", err)
					} else {
						atomic.AddInt64(&writeCount, 1)
					}
				}
			}()
		}

		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_, err := store.Get(ctx, projectionName)
					if err != nil {
						t.Errorf("Failed to read in concurrent test: %v", err)
					} else {
						atomic.AddInt64(&readCount, 1)
					}
				}
			}()
		}

		wg.Wait()

		t.Logf("Concurrent test completed: %d writes, %d reads", atomic.LoadInt64(&writeCount), atomic.LoadInt64(&readCount))
	})
}

func TestOffsetStore_ClosedOperations(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Close the store
	err := store.Close()
	if err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Test operations on closed store
	_, err = store.Get(ctx, "test")
	if err == nil {
		t.Fatal("Expected error on Get with closed store")
	}

	err = store.Set(ctx, "test", cursor.IntegerCursor{Seq: 1})
	if err == nil {
		t.Fatal("Expected error on Set with closed store")
	}

	_, err = store.ListProjections(ctx)
	if err == nil {
		t.Fatal("Expected error on ListProjections with closed store")
	}

	err = store.Reset(ctx, "test")
	if err == nil {
		t.Fatal("Expected error on Reset with closed store")
	}

	// Double close should be safe
	err = store.Close()
	if err != nil {
		t.Errorf("Expected no error on double close, got: %v", err)
	}
}

func TestOffsetStore_RunGC(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Add some data first
	for i := 0; i < 100; i++ {
		projectionName := fmt.Sprintf("proj-%d", i)
		version := cursor.IntegerCursor{Seq: uint64(i)}
		err := store.Set(ctx, projectionName, version)
		if err != nil {
			t.Fatalf("Failed to set projection %s: %v", projectionName, err)
		}
	}

	// Run GC
	err := store.RunGC(ctx)
	if err != nil {
		t.Fatalf("Failed to run GC: %v", err)
	}

	// Verify data is still accessible after GC
	offset, err := store.Get(ctx, "proj-50")
	if err != nil {
		t.Fatalf("Failed to get offset after GC: %v", err)
	}
	if offset == nil {
		t.Fatal("Expected offset to exist after GC")
	}

	intOffset := offset.(cursor.IntegerCursor)
	if intOffset.Seq != 50 {
		t.Fatalf("Expected seq 50 after GC, got: %d", intOffset.Seq)
	}
}

func TestOffsetStore_GC_ClosedStore(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Close the store
	err := store.Close()
	if err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// GC on closed store should fail
	err = store.RunGC(ctx)
	if err == nil {
		t.Fatal("Expected error on RunGC with closed store")
	}
}

// Helper function similar to SQLite tests
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			haystack[:len(needle)] == needle ||
			haystack[len(haystack)-len(needle):] == needle ||
			findInString(haystack, needle))
}

func findInString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
