package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusAdapter(t *testing.T) {
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
			name:     "with nil registry",
			registry: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewPrometheusAdapter(tt.registry)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, adapter)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, adapter)
			assert.NotNil(t, adapter.metrics)
		})
	}
}

func TestPrometheusAdapter_RecordSyncDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test sync duration recording
	duration := 150 * time.Millisecond
	adapter.RecordSyncDuration(duration)

	// Verify histogram observation
	histogram := testutil.ToFloat64(adapter.metrics.SyncDurationHistogram.WithLabelValues("sync", "success"))
	assert.Equal(t, float64(1), histogram)
}

func TestPrometheusAdapter_RecordSyncEvent(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test different event types
	adapter.RecordSyncEvent("push_start")
	adapter.RecordSyncEvent("pull_complete") 
	adapter.RecordSyncEvent("sync_success")

	// Verify event counters
	pushStart := testutil.ToFloat64(adapter.metrics.EventsProcessedTotal.WithLabelValues("push_start"))
	assert.Equal(t, float64(1), pushStart)

	pullComplete := testutil.ToFloat64(adapter.metrics.EventsProcessedTotal.WithLabelValues("pull_complete"))
	assert.Equal(t, float64(1), pullComplete)

	syncSuccess := testutil.ToFloat64(adapter.metrics.EventsProcessedTotal.WithLabelValues("sync_success"))
	assert.Equal(t, float64(1), syncSuccess)
}

func TestPrometheusAdapter_RecordError(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test error recording
	adapter.RecordError("timeout")
	adapter.RecordError("network_failure")
	adapter.RecordError("timeout") // Same error again

	// Verify error counters
	timeout := testutil.ToFloat64(adapter.metrics.ErrorsTotal.WithLabelValues("sync", "timeout"))
	assert.Equal(t, float64(2), timeout)

	networkFailure := testutil.ToFloat64(adapter.metrics.ErrorsTotal.WithLabelValues("sync", "network_failure"))
	assert.Equal(t, float64(1), networkFailure)
}

func TestPrometheusAdapter_RecordConflictResolved(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test conflict resolution recording
	adapter.RecordConflictResolved("last_write_wins")
	adapter.RecordConflictResolved("first_write_wins")
	adapter.RecordConflictResolved("last_write_wins") // Same strategy again

	// Verify conflict counters
	lww := testutil.ToFloat64(adapter.metrics.ConflictResolutionTotal.WithLabelValues("last_write_wins", "success"))
	assert.Equal(t, float64(2), lww)

	fww := testutil.ToFloat64(adapter.metrics.ConflictResolutionTotal.WithLabelValues("first_write_wins", "success"))
	assert.Equal(t, float64(1), fww)
}

func TestPrometheusAdapter_ExtendedTransportMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test extended transport metrics
	duration := 75 * time.Millisecond
	adapter.RecordTransportOperation("http", "push", duration, 1024, true)
	adapter.RecordTransportOperation("grpc", "pull", duration*2, 2048, false)

	// Verify transport operation counters
	httpPush := testutil.ToFloat64(adapter.metrics.TransportOperationsTotal.WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(1), httpPush)

	grpcPull := testutil.ToFloat64(adapter.metrics.TransportOperationsTotal.WithLabelValues("grpc", "pull", "failure"))
	assert.Equal(t, float64(1), grpcPull)

	// Verify bytes transferred
	httpBytes := testutil.ToFloat64(adapter.metrics.TransportBytesTotal.WithLabelValues("http", "sent"))
	assert.Equal(t, float64(1024), httpBytes)

	grpcBytes := testutil.ToFloat64(adapter.metrics.TransportBytesTotal.WithLabelValues("grpc", "received"))
	assert.Equal(t, float64(2048), grpcBytes)
}

func TestPrometheusAdapter_ExtendedStorageMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test extended storage metrics
	duration := 25 * time.Millisecond
	adapter.RecordStorageOperation("sqlite", "write", duration, 10, 512, true)
	adapter.RecordStorageOperation("postgresql", "read", duration*3, 25, 1024, false)

	// Verify storage operation counters
	sqliteWrite := testutil.ToFloat64(adapter.metrics.StorageOperationsTotal.WithLabelValues("sqlite", "write", "success"))
	assert.Equal(t, float64(1), sqliteWrite)

	pgRead := testutil.ToFloat64(adapter.metrics.StorageOperationsTotal.WithLabelValues("postgresql", "read", "failure"))
	assert.Equal(t, float64(1), pgRead)

	// Verify records processed
	sqliteRecords := testutil.ToFloat64(adapter.metrics.StorageRecordsTotal.WithLabelValues("sqlite", "written"))
	assert.Equal(t, float64(10), sqliteRecords)

	pgRecords := testutil.ToFloat64(adapter.metrics.StorageRecordsTotal.WithLabelValues("postgresql", "read"))
	assert.Equal(t, float64(25), pgRecords)

	// Verify bytes processed
	sqliteBytes := testutil.ToFloat64(adapter.metrics.StorageBytesTotal.WithLabelValues("sqlite", "written"))
	assert.Equal(t, float64(512), sqliteBytes)

	pgBytes := testutil.ToFloat64(adapter.metrics.StorageBytesTotal.WithLabelValues("postgresql", "read"))
	assert.Equal(t, float64(1024), pgBytes)
}

func TestPrometheusAdapter_ConflictResolutionMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test conflict resolution metrics
	duration := 30 * time.Millisecond
	adapter.RecordConflictResolution("last_write_wins", duration, 5, true)
	adapter.RecordConflictResolution("custom_merge", duration*2, 3, false)

	// Verify conflict resolution counters
	lww := testutil.ToFloat64(adapter.metrics.ConflictResolutionTotal.WithLabelValues("last_write_wins", "success"))
	assert.Equal(t, float64(1), lww)

	custom := testutil.ToFloat64(adapter.metrics.ConflictResolutionTotal.WithLabelValues("custom_merge", "failure"))
	assert.Equal(t, float64(1), custom)

	// Verify conflicts resolved counts
	resolved := testutil.ToFloat64(adapter.metrics.ConflictsTotal.WithLabelValues("resolved"))
	assert.Equal(t, float64(5), resolved)
}

func TestPrometheusAdapter_SystemMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test system metrics
	adapter.UpdateSystemMetrics(8, 128*1024*1024, 45, 7200*time.Second)

	// Verify system gauges
	active := testutil.ToFloat64(adapter.metrics.ActiveSyncOperationsGauge)
	assert.Equal(t, float64(8), active)

	memory := testutil.ToFloat64(adapter.metrics.MemoryUsageGauge)
	assert.Equal(t, float64(128*1024*1024), memory)

	goroutines := testutil.ToFloat64(adapter.metrics.GoroutineCountGauge)
	assert.Equal(t, float64(45), goroutines)

	uptime := testutil.ToFloat64(adapter.metrics.UptimeGauge)
	assert.Equal(t, float64(7200), uptime)
}

func TestPrometheusAdapter_BusinessMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test business metrics
	labels := map[string]string{
		"tenant":     "acme_corp",
		"region":     "us-west",
		"datacenter": "dc1",
	}

	adapter.RecordBusinessMetric("user_sync_operations", labels, 150.5)
	adapter.RecordBusinessMetric("data_volume_processed", labels, 1024000.0)

	// Verify business metrics
	userOps := testutil.ToFloat64(adapter.metrics.CustomMetricsCounter.WithLabelValues("user_sync_operations", "acme_corp", "us-west"))
	assert.Equal(t, float64(1), userOps)

	dataVolume := testutil.ToFloat64(adapter.metrics.CustomMetricsCounter.WithLabelValues("data_volume_processed", "acme_corp", "us-west"))
	assert.Equal(t, float64(1), dataVolume)
}

func TestPrometheusAdapter_BackwardCompatibility(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	// Test that adapter implements the basic MetricsCollector interface
	duration := 100 * time.Millisecond

	// These methods should work without error (basic interface)
	adapter.RecordSyncDuration(duration)
	adapter.RecordSyncEvent("test_event")
	adapter.RecordError("test_error")
	adapter.RecordConflictResolved("test_strategy")

	// Verify basic functionality still works
	event := testutil.ToFloat64(adapter.metrics.EventsProcessedTotal.WithLabelValues("test_event"))
	assert.Equal(t, float64(1), event)

	error := testutil.ToFloat64(adapter.metrics.ErrorsTotal.WithLabelValues("sync", "test_error"))
	assert.Equal(t, float64(1), error)
}

func TestPrometheusAdapter_ConcurrentAccess(t *testing.T) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(t, err)

	const numGoroutines = 20
	const numOperations = 50

	done := make(chan bool, numGoroutines)

	// Test concurrent access from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				duration := time.Duration(j) * time.Millisecond

				// Test all adapter methods concurrently
				adapter.RecordSyncDuration(duration)
				adapter.RecordSyncEvent("concurrent_event")
				adapter.RecordError("concurrent_error")
				adapter.RecordConflictResolved("lww")

				// Extended methods
				adapter.RecordTransportOperation("http", "push", duration, 100, true)
				adapter.RecordStorageOperation("sqlite", "write", duration, 1, 50, true)
				adapter.RecordConflictResolution("lww", duration, 1, true)
				adapter.UpdateSystemMetrics(int64(id), 1024*1024, 10, time.Hour)

				labels := map[string]string{"worker": string(rune('A' + id))}
				adapter.RecordBusinessMetric("worker_ops", labels, 1.0)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final counts
	events := testutil.ToFloat64(adapter.metrics.EventsProcessedTotal.WithLabelValues("concurrent_event"))
	assert.Equal(t, float64(numGoroutines*numOperations), events)

	errors := testutil.ToFloat64(adapter.metrics.ErrorsTotal.WithLabelValues("sync", "concurrent_error"))
	assert.Equal(t, float64(numGoroutines*numOperations), errors)

	conflicts := testutil.ToFloat64(adapter.metrics.ConflictResolutionTotal.WithLabelValues("lww", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations*2), conflicts) // Called twice per iteration

	transport := testutil.ToFloat64(adapter.metrics.TransportOperationsTotal.WithLabelValues("http", "push", "success"))
	assert.Equal(t, float64(numGoroutines*numOperations), transport)
}

func BenchmarkPrometheusAdapter_RecordSyncDuration(b *testing.B) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(b, err)

	duration := 100 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			adapter.RecordSyncDuration(duration)
		}
	})
}

func BenchmarkPrometheusAdapter_RecordTransportOperation(b *testing.B) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(b, err)

	duration := 50 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			adapter.RecordTransportOperation("http", "push", duration, 1024, true)
		}
	})
}

func BenchmarkPrometheusAdapter_RecordBusinessMetric(b *testing.B) {
	registry := prometheus.NewRegistry()
	adapter, err := NewPrometheusAdapter(registry)
	require.NoError(b, err)

	labels := map[string]string{
		"tenant": "test",
		"region": "us-west",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			adapter.RecordBusinessMetric("test_metric", labels, 42.0)
		}
	})
}
