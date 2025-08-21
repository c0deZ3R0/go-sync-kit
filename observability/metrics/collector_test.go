package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncKitMetrics(t *testing.T) {
	tests := []struct {
		name     string
		registry *prometheus.Registry
		wantErr  bool
	}{
		{
			name:     "with custom registry",
			registry: prometheus.NewRegistry(),
			wantErr:  false,
		},
		{
			name:     "with nil registry uses default",
			registry: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := NewSyncKitMetrics(tt.registry)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, metrics)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, metrics)

			// Verify all metrics are initialized
			assert.NotNil(t, metrics.SyncOperationsTotal)
			assert.NotNil(t, metrics.SyncDurationHistogram)
			assert.NotNil(t, metrics.EventsProcessedTotal)
			assert.NotNil(t, metrics.ConflictsTotal)
			assert.NotNil(t, metrics.ErrorsTotal)
			assert.NotNil(t, metrics.TransportOperationsTotal)
			assert.NotNil(t, metrics.StorageOperationsTotal)
			assert.NotNil(t, metrics.ActiveSyncOperationsGauge)
		})
	}
}

func TestSyncKitMetrics_RecordSyncOperation(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	// Test successful sync operation
	duration := 100 * time.Millisecond
	metrics.RecordSyncOperation("push", "success", duration, 10, 5, 2)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.SyncOperationsTotal.WithLabelValues("push", "success"))
	assert.Equal(t, float64(1), counter)

	// Verify histogram observation
	histogram := testutil.ToFloat64(metrics.SyncDurationHistogram.WithLabelValues("push", "success"))
	assert.Equal(t, float64(1), histogram)

	// Verify events processed
	events := testutil.ToFloat64(metrics.EventsProcessedTotal.WithLabelValues("pushed"))
	assert.Equal(t, float64(10), events)

	events = testutil.ToFloat64(metrics.EventsProcessedTotal.WithLabelValues("pulled"))
	assert.Equal(t, float64(5), events)

	// Verify conflicts
	conflicts := testutil.ToFloat64(metrics.ConflictsTotal.WithLabelValues("resolved"))
	assert.Equal(t, float64(2), conflicts)
}

func TestSyncKitMetrics_RecordTransportOperation(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	duration := 50 * time.Millisecond
	metrics.RecordTransportOperation("http", "push", "success", duration, 100)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.TransportOperationsTotal.WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(1), counter)

	// Verify duration histogram
	histogram := testutil.ToFloat64(metrics.TransportDurationHistogram.WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(1), histogram)

	// Verify bytes transferred
	bytes := testutil.ToFloat64(metrics.TransportBytesTotal.WithLabelValues("http", "sent"))
	assert.Equal(t, float64(100), bytes)
}

func TestSyncKitMetrics_RecordStorageOperation(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	duration := 25 * time.Millisecond
	metrics.RecordStorageOperation("sqlite", "write", "success", duration, 5, 200)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.StorageOperationsTotal.WithLabelValues("sqlite", "write", "success"))
	assert.Equal(t, float64(1), counter)

	// Verify duration histogram
	histogram := testutil.ToFloat64(metrics.StorageDurationHistogram.WithLabelValues("sqlite", "write", "success"))
	assert.Equal(t, float64(1), histogram)

	// Verify records processed
	records := testutil.ToFloat64(metrics.StorageRecordsTotal.WithLabelValues("sqlite", "written"))
	assert.Equal(t, float64(5), records)

	// Verify bytes processed
	bytes := testutil.ToFloat64(metrics.StorageBytesTotal.WithLabelValues("sqlite", "written"))
	assert.Equal(t, float64(200), bytes)
}

func TestSyncKitMetrics_RecordConflictResolution(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	duration := 15 * time.Millisecond
	metrics.RecordConflictResolution("last_write_wins", "success", duration, 3)

	// Verify counter increment
	counter := testutil.ToFloat64(metrics.ConflictResolutionTotal.WithLabelValues("last_write_wins", "success"))
	assert.Equal(t, float64(1), counter)

	// Verify duration histogram
	histogram := testutil.ToFloat64(metrics.ConflictResolutionDurationHistogram.WithLabelValues("last_write_wins", "success"))
	assert.Equal(t, float64(1), histogram)

	// Verify conflicts resolved
	resolved := testutil.ToFloat64(metrics.ConflictsTotal.WithLabelValues("resolved"))
	assert.Equal(t, float64(3), resolved)
}

func TestSyncKitMetrics_UpdateSystemMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	// Update system metrics
	metrics.SetActiveSyncOperations(5)
	metrics.UpdateMemoryUsage(1024 * 1024 * 100) // 100 MB
	metrics.UpdateGoroutineCount(50)
	metrics.UpdateUptime(3600 * time.Second) // 1 hour

	// Verify gauges
	active := testutil.ToFloat64(metrics.ActiveSyncOperationsGauge)
	assert.Equal(t, float64(5), active)

	memory := testutil.ToFloat64(metrics.MemoryUsageGauge)
	assert.Equal(t, float64(1024*1024*100), memory)

	goroutines := testutil.ToFloat64(metrics.GoroutineCountGauge)
	assert.Equal(t, float64(50), goroutines)

	uptime := testutil.ToFloat64(metrics.UptimeGauge)
	assert.Equal(t, float64(3600), uptime)
}

func TestSyncKitMetrics_RecordError(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	// Record different types of errors
	metrics.RecordError("sync", "timeout")
	metrics.RecordError("transport", "connection_failed")
	metrics.RecordError("storage", "write_error")

	// Verify error counters
	syncErrors := testutil.ToFloat64(metrics.ErrorsTotal.WithLabelValues("sync", "timeout"))
	assert.Equal(t, float64(1), syncErrors)

	transportErrors := testutil.ToFloat64(metrics.ErrorsTotal.WithLabelValues("transport", "connection_failed"))
	assert.Equal(t, float64(1), transportErrors)

	storageErrors := testutil.ToFloat64(metrics.ErrorsTotal.WithLabelValues("storage", "write_error"))
	assert.Equal(t, float64(1), storageErrors)
}

func TestSyncKitMetrics_RecordCustomMetric(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	// Record custom business metrics
	labels := map[string]string{
		"tenant":    "customer1",
		"operation": "data_sync",
	}
	
	metrics.RecordCustomMetric("business_operation", labels, 42.5)
	metrics.RecordCustomMetric("business_operation", labels, 10.0)

	// Verify custom metric counter
	counter := testutil.ToFloat64(metrics.CustomMetricsCounter.WithLabelValues("business_operation", "customer1", "data_sync"))
	assert.Equal(t, float64(2), counter)
}

func TestSyncKitMetrics_ConcurrentAccess(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(t, err)

	// Test concurrent access to metrics
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines recording metrics
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			for j := 0; j < numOperations; j++ {
				duration := time.Duration(j) * time.Millisecond
				
				// Record various metrics concurrently
				metrics.RecordSyncOperation("push", "success", duration, 1, 0, 0)
				metrics.RecordTransportOperation("http", "push", "success", duration, 50)
				metrics.RecordStorageOperation("sqlite", "write", "success", duration, 1, 50)
				metrics.SetActiveSyncOperations(int64(id))
				metrics.RecordError("sync", "test_error")
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final counts
	syncOps := testutil.ToFloat64(metrics.SyncOperationsTotal.WithLabelValues("push", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations), syncOps)

	transportOps := testutil.ToFloat64(metrics.TransportOperationsTotal.WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations), transportOps)

	storageOps := testutil.ToFloat64(metrics.StorageOperationsTotal.WithLabelValues("sqlite", "write", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations), storageOps)

	errors := testutil.ToFloat64(metrics.ErrorsTotal.WithLabelValues("sync", "test_error"))
	assert.Equal(t, float64(numGoroutines*numOperations), errors)
}

func BenchmarkSyncKitMetrics_RecordSyncOperation(b *testing.B) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(b, err)

	duration := 100 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.RecordSyncOperation("push", "success", duration, 10, 5, 2)
		}
	})
}

func BenchmarkSyncKitMetrics_RecordTransportOperation(b *testing.B) {
	registry := prometheus.NewRegistry()
	metrics, err := NewSyncKitMetrics(registry)
	require.NoError(b, err)

	duration := 50 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.RecordTransportOperation("http", "push", "success", duration, 100)
		}
	})
}
