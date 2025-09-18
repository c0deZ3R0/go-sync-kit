// Package metrics provides Prometheus metrics collection for go-sync-kit.
// It enables monitoring of sync operations, transport performance, storage operations,
// and conflict resolution with structured metrics following Prometheus conventions.
package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// SyncKitMetrics provides comprehensive metrics collection for sync operations.
// It implements the metrics collection interface used by SyncManager and provides
// Prometheus-compatible metrics for monitoring and alerting.
type SyncKitMetrics struct {
	// Sync operation metrics
	syncOpsTotal        *prometheus.CounterVec
	syncOpsDuration     *prometheus.HistogramVec
	syncEventsProcessed *prometheus.CounterVec
	syncConflictsTotal  *prometheus.CounterVec
	syncErrorsTotal     *prometheus.CounterVec
	syncLastSuccess     *prometheus.GaugeVec

	// Transport metrics
	transportOpsTotal    *prometheus.CounterVec
	transportDuration    *prometheus.HistogramVec
	transportBytesTotal  *prometheus.CounterVec
	transportErrorsTotal *prometheus.CounterVec

	// Storage metrics
	storageOpsTotal    *prometheus.CounterVec
	storageDuration    *prometheus.HistogramVec
	storageErrorsTotal *prometheus.CounterVec

	// Conflict resolution metrics
	conflictOpsTotal *prometheus.CounterVec
	conflictDuration *prometheus.HistogramVec

	// Projection metrics
	projectionOpsTotal        *prometheus.CounterVec
	projectionOpsDuration     *prometheus.HistogramVec
	projectionEventsProcessed *prometheus.CounterVec
	projectionBatchSize       *prometheus.HistogramVec
	projectionErrorsTotal     *prometheus.CounterVec
	projectionLastSuccess     *prometheus.GaugeVec
	projectionLag             *prometheus.GaugeVec
	projectionHealth          *prometheus.GaugeVec

	// System metrics
	activeConnections prometheus.Gauge
	memoryUsage       prometheus.Gauge
	goroutines        prometheus.Gauge

	registry *prometheus.Registry
	labels   prometheus.Labels
}

// MetricsOption allows for functional configuration of SyncKitMetrics.
type MetricsOption func(*SyncKitMetrics)

// WithRegistry sets a custom Prometheus registry.
func WithRegistry(registry *prometheus.Registry) MetricsOption {
	return func(m *SyncKitMetrics) {
		m.registry = registry
	}
}

// WithLabels adds custom labels to all metrics.
func WithLabels(labels prometheus.Labels) MetricsOption {
	return func(m *SyncKitMetrics) {
		if m.labels == nil {
			m.labels = make(prometheus.Labels)
		}
		for k, v := range labels {
			m.labels[k] = v
		}
	}
}

// NewMetrics creates a new SyncKitMetrics collector with the given service name and options.
func NewMetrics(serviceName string, opts ...MetricsOption) *SyncKitMetrics {
	m := &SyncKitMetrics{
		registry: prometheus.NewRegistry(),
		labels: prometheus.Labels{
			"service": serviceName,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	// Initialize all metrics
	m.initSyncMetrics()
	m.initTransportMetrics()
	m.initStorageMetrics()
	m.initConflictMetrics()
	m.initProjectionMetrics()
	m.initSystemMetrics()

	// Register all metrics
	m.registerMetrics()

	return m
}

// initSyncMetrics initializes sync operation metrics.
func (m *SyncKitMetrics) initSyncMetrics() {
	m.syncOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "sync",
			Name:        "operations_total",
			Help:        "Total number of sync operations.",
			ConstLabels: m.labels,
		},
		[]string{"operation", "status"},
	)

	m.syncOpsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "synckit",
			Subsystem:   "sync",
			Name:        "operation_duration_seconds",
			Help:        "Duration of sync operations in seconds.",
			ConstLabels: m.labels,
			Buckets:     prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
		},
		[]string{"operation"},
	)

	m.syncEventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "sync",
			Name:        "events_processed_total",
			Help:        "Total number of events processed during sync operations.",
			ConstLabels: m.labels,
		},
		[]string{"operation", "direction"},
	)

	m.syncConflictsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "sync",
			Name:        "conflicts_total",
			Help:        "Total number of conflicts encountered during sync operations.",
			ConstLabels: m.labels,
		},
		[]string{"strategy", "resolution"},
	)

	m.syncErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "sync",
			Name:        "errors_total",
			Help:        "Total number of sync operation errors.",
			ConstLabels: m.labels,
		},
		[]string{"operation", "error_type"},
	)

	m.syncLastSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "sync",
			Name:        "last_success_timestamp",
			Help:        "Timestamp of the last successful sync operation.",
			ConstLabels: m.labels,
		},
		[]string{"operation"},
	)
}

// initTransportMetrics initializes transport layer metrics.
func (m *SyncKitMetrics) initTransportMetrics() {
	m.transportOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "transport",
			Name:        "operations_total",
			Help:        "Total number of transport operations.",
			ConstLabels: m.labels,
		},
		[]string{"protocol", "operation", "status"},
	)

	m.transportDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "synckit",
			Subsystem:   "transport",
			Name:        "operation_duration_seconds",
			Help:        "Duration of transport operations in seconds.",
			ConstLabels: m.labels,
			Buckets:     []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"protocol", "operation"},
	)

	m.transportBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "transport",
			Name:        "bytes_total",
			Help:        "Total number of bytes transferred.",
			ConstLabels: m.labels,
		},
		[]string{"protocol", "direction"},
	)

	m.transportErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "transport",
			Name:        "errors_total",
			Help:        "Total number of transport errors.",
			ConstLabels: m.labels,
		},
		[]string{"protocol", "operation", "error_type"},
	)
}

// initStorageMetrics initializes storage operation metrics.
func (m *SyncKitMetrics) initStorageMetrics() {
	m.storageOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "storage",
			Name:        "operations_total",
			Help:        "Total number of storage operations.",
			ConstLabels: m.labels,
		},
		[]string{"backend", "operation", "status"},
	)

	m.storageDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "synckit",
			Subsystem:   "storage",
			Name:        "operation_duration_seconds",
			Help:        "Duration of storage operations in seconds.",
			ConstLabels: m.labels,
			Buckets:     []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"backend", "operation"},
	)

	m.storageErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "storage",
			Name:        "errors_total",
			Help:        "Total number of storage errors.",
			ConstLabels: m.labels,
		},
		[]string{"backend", "operation", "error_type"},
	)
}

// initConflictMetrics initializes conflict resolution metrics.
func (m *SyncKitMetrics) initConflictMetrics() {
	m.conflictOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "conflict",
			Name:        "operations_total",
			Help:        "Total number of conflict resolution operations.",
			ConstLabels: m.labels,
		},
		[]string{"strategy", "result"},
	)

	m.conflictDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "synckit",
			Subsystem:   "conflict",
			Name:        "resolution_duration_seconds",
			Help:        "Duration of conflict resolution operations in seconds.",
			ConstLabels: m.labels,
			Buckets:     []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"strategy"},
	)
}

// initProjectionMetrics initializes projection operation metrics.
func (m *SyncKitMetrics) initProjectionMetrics() {
	m.projectionOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "operations_total",
			Help:        "Total number of projection operations.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation", "status"},
	)

	m.projectionOpsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "operation_duration_seconds",
			Help:        "Duration of projection operations in seconds.",
			ConstLabels: m.labels,
			Buckets:     []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, // Similar to sync operations
		},
		[]string{"projection", "operation"},
	)

	m.projectionEventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "events_processed_total",
			Help:        "Total number of events processed during projection operations.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation"},
	)

	m.projectionBatchSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "batch_size",
			Help:        "Size of projection batches.",
			ConstLabels: m.labels,
			Buckets:     []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000},
		},
		[]string{"projection"},
	)

	m.projectionErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "errors_total",
			Help:        "Total number of projection errors.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation", "error_type"},
	)

	m.projectionLastSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "last_success_timestamp",
			Help:        "Timestamp of the last successful projection operation.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation"},
	)

	m.projectionLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "lag_seconds",
			Help:        "Lag between event creation and projection processing in seconds.",
			ConstLabels: m.labels,
		},
		[]string{"projection"},
	)

	m.projectionHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "health",
			Help:        "Health status of projections (0=unhealthy, 1=healthy).",
			ConstLabels: m.labels,
		},
		[]string{"projection"},
	)
}

// initSystemMetrics initializes system-level metrics.
func (m *SyncKitMetrics) initSystemMetrics() {
	m.activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "system",
			Name:        "active_connections",
			Help:        "Number of active connections.",
			ConstLabels: m.labels,
		},
	)

	m.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "system",
			Name:        "memory_usage_bytes",
			Help:        "Current memory usage in bytes.",
			ConstLabels: m.labels,
		},
	)

	m.goroutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "system",
			Name:        "goroutines",
			Help:        "Current number of goroutines.",
			ConstLabels: m.labels,
		},
	)
}

// registerMetrics registers all metrics with the registry.
func (m *SyncKitMetrics) registerMetrics() {
	// Sync metrics
	m.registry.MustRegister(
		m.syncOpsTotal,
		m.syncOpsDuration,
		m.syncEventsProcessed,
		m.syncConflictsTotal,
		m.syncErrorsTotal,
		m.syncLastSuccess,
	)

	// Transport metrics
	m.registry.MustRegister(
		m.transportOpsTotal,
		m.transportDuration,
		m.transportBytesTotal,
		m.transportErrorsTotal,
	)

	// Storage metrics
	m.registry.MustRegister(
		m.storageOpsTotal,
		m.storageDuration,
		m.storageErrorsTotal,
	)

	// Conflict metrics
	m.registry.MustRegister(
		m.conflictOpsTotal,
		m.conflictDuration,
	)

	// Projection metrics
	m.registry.MustRegister(
		m.projectionOpsTotal,
		m.projectionOpsDuration,
		m.projectionEventsProcessed,
		m.projectionBatchSize,
		m.projectionErrorsTotal,
		m.projectionLastSuccess,
		m.projectionLag,
		m.projectionHealth,
	)

	// System metrics
	m.registry.MustRegister(
		m.activeConnections,
		m.memoryUsage,
		m.goroutines,
	)
}

// Registry returns the Prometheus registry containing all sync-kit metrics.
func (m *SyncKitMetrics) Registry() *prometheus.Registry {
	return m.registry
}

// RecordSyncOperation records metrics for a sync operation.
func (m *SyncKitMetrics) RecordSyncOperation(operation string, duration time.Duration, success bool, eventsPushed, eventsPulled, conflictsResolved int) {
	status := "success"
	if !success {
		status = "error"
	}

	m.syncOpsTotal.WithLabelValues(operation, status).Inc()
	m.syncOpsDuration.WithLabelValues(operation).Observe(duration.Seconds())

	if eventsPushed > 0 {
		m.syncEventsProcessed.WithLabelValues(operation, "push").Add(float64(eventsPushed))
	}
	if eventsPulled > 0 {
		m.syncEventsProcessed.WithLabelValues(operation, "pull").Add(float64(eventsPulled))
	}
	if conflictsResolved > 0 {
		m.syncConflictsTotal.WithLabelValues("auto", "resolved").Add(float64(conflictsResolved))
	}

	if success {
		m.syncLastSuccess.WithLabelValues(operation).SetToCurrentTime()
	}
}

// RecordSyncError records a sync operation error.
func (m *SyncKitMetrics) RecordSyncError(operation, errorType string) {
	m.syncErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// RecordTransportOperation records metrics for a transport operation.
func (m *SyncKitMetrics) RecordTransportOperation(protocol, operation string, duration time.Duration, success bool, bytesTransferred int64) {
	status := "success"
	if !success {
		status = "error"
	}

	m.transportOpsTotal.WithLabelValues(protocol, operation, status).Inc()
	m.transportDuration.WithLabelValues(protocol, operation).Observe(duration.Seconds())

	if bytesTransferred > 0 {
		direction := "unknown"
		switch operation {
		case "push":
			direction = "send"
		case "pull":
			direction = "receive"
		}
		m.transportBytesTotal.WithLabelValues(protocol, direction).Add(float64(bytesTransferred))
	}
}

// RecordTransportError records a transport operation error.
func (m *SyncKitMetrics) RecordTransportError(protocol, operation, errorType string) {
	m.transportErrorsTotal.WithLabelValues(protocol, operation, errorType).Inc()
}

// RecordStorageOperation records metrics for a storage operation.
func (m *SyncKitMetrics) RecordStorageOperation(backend, operation string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
	}

	m.storageOpsTotal.WithLabelValues(backend, operation, status).Inc()
	m.storageDuration.WithLabelValues(backend, operation).Observe(duration.Seconds())
}

// RecordStorageError records a storage operation error.
func (m *SyncKitMetrics) RecordStorageError(backend, operation, errorType string) {
	m.storageErrorsTotal.WithLabelValues(backend, operation, errorType).Inc()
}

// RecordConflictResolution records metrics for conflict resolution.
func (m *SyncKitMetrics) RecordConflictResolution(strategy string, duration time.Duration, result string) {
	m.conflictOpsTotal.WithLabelValues(strategy, result).Inc()
	m.conflictDuration.WithLabelValues(strategy).Observe(duration.Seconds())
}

// RecordProjectionOperation records metrics for a projection operation.
func (m *SyncKitMetrics) RecordProjectionOperation(projection, operation string, duration time.Duration, success bool, eventsProcessed int) {
	status := "success"
	if !success {
		status = "error"
	}

	m.projectionOpsTotal.WithLabelValues(projection, operation, status).Inc()
	m.projectionOpsDuration.WithLabelValues(projection, operation).Observe(duration.Seconds())

	if eventsProcessed > 0 {
		m.projectionEventsProcessed.WithLabelValues(projection, operation).Add(float64(eventsProcessed))
		m.projectionBatchSize.WithLabelValues(projection).Observe(float64(eventsProcessed))
	}

	if success {
		m.projectionLastSuccess.WithLabelValues(projection, operation).SetToCurrentTime()
		m.projectionHealth.WithLabelValues(projection).Set(1)
	}
}

// RecordProjectionError records a projection operation error.
func (m *SyncKitMetrics) RecordProjectionError(projection, operation, errorType string) {
	m.projectionErrorsTotal.WithLabelValues(projection, operation, errorType).Inc()
	m.projectionHealth.WithLabelValues(projection).Set(0)
}

// UpdateProjectionLag updates the projection lag metric.
func (m *SyncKitMetrics) UpdateProjectionLag(projection string, lag time.Duration) {
	m.projectionLag.WithLabelValues(projection).Set(lag.Seconds())
}

// SetProjectionHealth manually sets the health status of a projection.
func (m *SyncKitMetrics) SetProjectionHealth(projection string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.projectionHealth.WithLabelValues(projection).Set(value)
}

// UpdateSystemMetrics updates system-level metrics.
func (m *SyncKitMetrics) UpdateSystemMetrics(ctx context.Context) {
	// This would typically be called periodically to update system metrics
	// Implementation depends on specific system monitoring requirements
}

// SetActiveConnections updates the active connections gauge.
func (m *SyncKitMetrics) SetActiveConnections(count int) {
	m.activeConnections.Set(float64(count))
}

// SetMemoryUsage updates the memory usage gauge.
func (m *SyncKitMetrics) SetMemoryUsage(bytes int64) {
	m.memoryUsage.Set(float64(bytes))
}

// SetGoroutines updates the goroutines count gauge.
func (m *SyncKitMetrics) SetGoroutines(count int) {
	m.goroutines.Set(float64(count))
}

// NewSyncKitMetrics is an alias for NewMetrics for backward compatibility.
func NewSyncKitMetrics(serviceName string, opts ...MetricsOption) *SyncKitMetrics {
	return NewMetrics(serviceName, opts...)
}

// NewPrometheusAdapter creates a new Prometheus metrics adapter.
// This function creates both the metrics collector and the adapter that
// implements the synckit.MetricsCollector interface.
func NewPrometheusAdapter(serviceName string, opts ...MetricsOption) *MetricsCollectorAdapter {
	metrics := NewSyncKitMetrics(serviceName, opts...)
	return NewAdapter(metrics)
}

// Getter methods for testing

// SyncOperationsTotal returns the sync operations counter for testing.
func (m *SyncKitMetrics) SyncOperationsTotal() *prometheus.CounterVec {
	return m.syncOpsTotal
}

// TransportOperationsTotal returns the transport operations counter for testing.
func (m *SyncKitMetrics) TransportOperationsTotal() *prometheus.CounterVec {
	return m.transportOpsTotal
}

// StorageOperationsTotal returns the storage operations counter for testing.
func (m *SyncKitMetrics) StorageOperationsTotal() *prometheus.CounterVec {
	return m.storageOpsTotal
}

// ProjectionOperationsTotal returns the projection operations counter for testing.
func (m *SyncKitMetrics) ProjectionOperationsTotal() *prometheus.CounterVec {
	return m.projectionOpsTotal
}
