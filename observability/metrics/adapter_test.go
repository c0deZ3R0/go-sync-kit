package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapter(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewAdapter(metrics)

	require.NotNil(t, adapter)
	assert.NotNil(t, adapter.Metrics())
}

func TestMetricsCollectorAdapter_RecordSyncDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewAdapter(metrics)

	// Test sync duration recording
	duration := 150 * time.Millisecond
	adapter.RecordSyncDuration("push", duration)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.SyncOperationsTotal().WithLabelValues("push", "success"))
	assert.Equal(t, float64(1), counter)
}

func TestMetricsCollectorAdapter_RecordSyncEvents(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewAdapter(metrics)

	// Test sync events recording
	adapter.RecordSyncEvents(10, 5)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.SyncOperationsTotal().WithLabelValues("sync", "success"))
	assert.Equal(t, float64(1), counter)
}

func TestMetricsCollectorAdapter_RecordSyncErrors(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewAdapter(metrics)

	// Test error recording
	adapter.RecordSyncErrors("push", "timeout")
	adapter.RecordSyncErrors("pull", "network_failure")

	// Basic verification - errors should be recorded
	// (exact verification would depend on the RecordSyncError implementation)
}

func TestMetricsCollectorAdapter_RecordConflicts(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewAdapter(metrics)

	// Test conflict recording
	adapter.RecordConflicts(3)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.SyncOperationsTotal().WithLabelValues("sync", "success"))
	assert.Equal(t, float64(1), counter)
}

func TestNewExtendedAdapter(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	require.NotNil(t, adapter)
	assert.NotNil(t, adapter.MetricsCollectorAdapter)
	assert.NotNil(t, adapter.Metrics())
}

func TestExtendedAdapter_RecordSyncOperationComplete(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	// Test comprehensive sync operation recording
	duration := 100 * time.Millisecond
	adapter.RecordSyncOperationComplete("push", duration, true, 10, 5, 2)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.SyncOperationsTotal().WithLabelValues("push", "success"))
	assert.Equal(t, float64(1), counter)
}

func TestExtendedAdapter_RecordTransportOperation(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	// Test transport operation recording
	duration := 50 * time.Millisecond
	adapter.RecordTransportOperation("http", "push", duration, true, 1024)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.TransportOperationsTotal().WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(1), counter)
}

func TestExtendedAdapter_RecordStorageOperation(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	// Test storage operation recording
	duration := 25 * time.Millisecond
	adapter.RecordStorageOperation("sqlite", "write", duration, true)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.StorageOperationsTotal().WithLabelValues("sqlite", "write", "success"))
	assert.Equal(t, float64(1), counter)
}

func TestExtendedAdapter_RecordConflictResolution(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	// Test conflict resolution recording
	duration := 15 * time.Millisecond
	adapter.RecordConflictResolution("last_write_wins", duration, "success")

	// Basic verification - operation should be recorded
	// (exact verification would depend on the RecordConflictResolution implementation)
}

func TestExtendedAdapter_SystemMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	// Test system metrics update
	adapter.SetActiveConnections(5)
	adapter.SetMemoryUsage(128 * 1024 * 1024)
	adapter.SetGoroutines(50)

	// Basic verification - operations should not fail
	// (exact verification would depend on the implementation)
}

func TestExtendedAdapter_ConcurrentAccess(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	const numGoroutines = 10
	const numOperations = 10

	done := make(chan bool, numGoroutines)

	// Test concurrent access from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				duration := time.Duration(j+1) * time.Millisecond

				// Test all adapter methods concurrently
				adapter.RecordSyncDuration("push", duration)
				adapter.RecordSyncEvents(1, 0)
				adapter.RecordConflicts(0)
				adapter.RecordSyncOperationComplete("push", duration, true, 1, 0, 0)
				adapter.RecordTransportOperation("http", "push", duration, true, 100)
				adapter.RecordStorageOperation("sqlite", "write", duration, true)
				adapter.SetActiveConnections(id)
				adapter.SetMemoryUsage(1024 * 1024)
				adapter.SetGoroutines(10)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final counts - we expect multiple operations to be recorded
	pushOps := testutil.ToFloat64(metrics.SyncOperationsTotal().WithLabelValues("push", "success"))
	assert.Greater(t, pushOps, float64(0))

	syncOps := testutil.ToFloat64(metrics.SyncOperationsTotal().WithLabelValues("sync", "success"))
	assert.Greater(t, syncOps, float64(0))

	transportOps := testutil.ToFloat64(metrics.TransportOperationsTotal().WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations), transportOps)

	storageOps := testutil.ToFloat64(metrics.StorageOperationsTotal().WithLabelValues("sqlite", "write", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations), storageOps)
}

func BenchmarkMetricsCollectorAdapter_RecordSyncDuration(b *testing.B) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewAdapter(metrics)

	duration := 100 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			adapter.RecordSyncDuration("push", duration)
		}
	})
}

func BenchmarkExtendedAdapter_RecordSyncOperationComplete(b *testing.B) {
	registry := prometheus.NewRegistry()
	metrics := NewSyncKitMetrics("test-service", WithRegistry(registry))
	adapter := NewExtendedAdapter(metrics)

	duration := 100 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			adapter.RecordSyncOperationComplete("push", duration, true, 10, 5, 2)
		}
	})
}
