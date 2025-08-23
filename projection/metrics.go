// Package projection provides observability and metrics for projection operations.
// This package integrates with the main observability system following synckit conventions.
package projection

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ProjectionMetrics holds all projection-related Prometheus metrics.
// This follows the same pattern as SyncKitMetrics in observability/metrics.
type ProjectionMetrics struct {
	// Operations metrics
	opsTotal         *prometheus.CounterVec
	opsDuration      *prometheus.HistogramVec
	eventsProcessed  *prometheus.CounterVec
	batchSize        *prometheus.HistogramVec
	errorsTotal      *prometheus.CounterVec
	lastSuccess      *prometheus.GaugeVec

	// Performance metrics
	lag              *prometheus.GaugeVec
	health           *prometheus.GaugeVec

	registry *prometheus.Registry
	labels   prometheus.Labels
}

// ProjectionMetricsOption allows for functional configuration of ProjectionMetrics.
type ProjectionMetricsOption func(*ProjectionMetrics)

// WithProjectionRegistry sets a custom Prometheus registry.
func WithProjectionRegistry(registry *prometheus.Registry) ProjectionMetricsOption {
	return func(m *ProjectionMetrics) {
		m.registry = registry
	}
}

// WithProjectionLabels adds custom labels to all projection metrics.
func WithProjectionLabels(labels prometheus.Labels) ProjectionMetricsOption {
	return func(m *ProjectionMetrics) {
		if m.labels == nil {
			m.labels = make(prometheus.Labels)
		}
		for k, v := range labels {
			m.labels[k] = v
		}
	}
}

// NewProjectionMetrics creates a new ProjectionMetrics collector following synckit conventions.
func NewProjectionMetrics(serviceName string, opts ...ProjectionMetricsOption) *ProjectionMetrics {
	m := &ProjectionMetrics{
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
	m.initProjectionMetrics()

	// Register all metrics
	m.registerMetrics()

	return m
}

// initProjectionMetrics initializes projection operation metrics following synckit conventions.
func (m *ProjectionMetrics) initProjectionMetrics() {
	m.opsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "operations_total",
			Help:        "Total number of projection operations.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation", "status"},
	)

	m.opsDuration = prometheus.NewHistogramVec(
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

	m.eventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "events_processed_total",
			Help:        "Total number of events processed during projection operations.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation"},
	)

	m.batchSize = prometheus.NewHistogramVec(
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

	m.errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "errors_total",
			Help:        "Total number of projection errors.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation", "error_type"},
	)

	m.lastSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "last_success_timestamp",
			Help:        "Timestamp of the last successful projection operation.",
			ConstLabels: m.labels,
		},
		[]string{"projection", "operation"},
	)

	m.lag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "synckit",
			Subsystem:   "projection",
			Name:        "lag_seconds",
			Help:        "Lag between event creation and projection processing in seconds.",
			ConstLabels: m.labels,
		},
		[]string{"projection"},
	)

	m.health = prometheus.NewGaugeVec(
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

// registerMetrics registers all metrics with the registry.
func (m *ProjectionMetrics) registerMetrics() {
	m.registry.MustRegister(
		m.opsTotal,
		m.opsDuration,
		m.eventsProcessed,
		m.batchSize,
		m.errorsTotal,
		m.lastSuccess,
		m.lag,
		m.health,
	)
}

// Registry returns the Prometheus registry containing all projection metrics.
func (m *ProjectionMetrics) Registry() *prometheus.Registry {
	return m.registry
}

// RecordProjectionOperation records metrics for a projection operation.
func (m *ProjectionMetrics) RecordProjectionOperation(projection, operation string, duration time.Duration, success bool, eventsProcessed int) {
	status := "success"
	if !success {
		status = "error"
	}

	m.opsTotal.WithLabelValues(projection, operation, status).Inc()
	m.opsDuration.WithLabelValues(projection, operation).Observe(duration.Seconds())

	if eventsProcessed > 0 {
		m.eventsProcessed.WithLabelValues(projection, operation).Add(float64(eventsProcessed))
		m.batchSize.WithLabelValues(projection).Observe(float64(eventsProcessed))
	}

	if success {
		m.lastSuccess.WithLabelValues(projection, operation).SetToCurrentTime()
		m.health.WithLabelValues(projection).Set(1)
	}
}

// RecordProjectionError records a projection operation error.
func (m *ProjectionMetrics) RecordProjectionError(projection, operation, errorType string) {
	m.errorsTotal.WithLabelValues(projection, operation, errorType).Inc()
	m.health.WithLabelValues(projection).Set(0)
}

// UpdateProjectionLag updates the lag metric.
func (m *ProjectionMetrics) UpdateProjectionLag(projection string, lag time.Duration) {
	m.lag.WithLabelValues(projection).Set(lag.Seconds())
}

// SetProjectionHealth manually sets the health status of a projection.
func (m *ProjectionMetrics) SetProjectionHealth(projection string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.health.WithLabelValues(projection).Set(value)
}

// GetProjectionMetrics returns placeholder metric values for debugging/testing.
// This is a simplified implementation for testing purposes.
// In production, metrics should be accessed via the Prometheus HTTP endpoint.
func (m *ProjectionMetrics) GetProjectionMetrics(projection string) map[string]float64 {
	metrics := make(map[string]float64)
	
	// Note: This is a simplified implementation that returns placeholder values.
	// For full metric access, use the Prometheus /metrics endpoint.
	metrics["operations_total"] = 0
	metrics["errors_total"] = 0
	metrics["lag_seconds"] = 0
	metrics["health"] = 1
	
	return metrics
}

// ResetProjectionMetrics resets all metrics for a given projection (useful for testing).
func (m *ProjectionMetrics) ResetProjectionMetrics(projection string) {
	// Delete all metric series for this projection
	m.opsTotal.DeletePartialMatch(prometheus.Labels{"projection": projection})
	m.eventsProcessed.DeletePartialMatch(prometheus.Labels{"projection": projection})
	m.errorsTotal.DeletePartialMatch(prometheus.Labels{"projection": projection})
	m.lastSuccess.DeletePartialMatch(prometheus.Labels{"projection": projection})
	m.batchSize.DeleteLabelValues(projection)
	m.health.DeleteLabelValues(projection)
	m.lag.DeleteLabelValues(projection)
	
	// Delete histogram series for all operations
	m.opsDuration.DeletePartialMatch(prometheus.Labels{"projection": projection})
}

// Error type constants for consistent error reporting
const (
	ErrorTypeApply     = "apply_error"
	ErrorTypeOffset    = "offset_error"
	ErrorTypeLoad      = "load_error"
	ErrorTypeTimeout   = "timeout_error"
	ErrorTypeContext   = "context_error"
)

// Operation type constants for metrics labeling
const (
	OperationApplySince = "apply_since"
	OperationApplyBatch = "apply_batch"
)

// Global default instance for backward compatibility
var defaultProjectionMetrics *ProjectionMetrics

// InitDefaultProjectionMetrics initializes the default projection metrics instance.
// This should be called during application startup.
func InitDefaultProjectionMetrics(serviceName string, opts ...ProjectionMetricsOption) {
	defaultProjectionMetrics = NewProjectionMetrics(serviceName, opts...)
}

// GetDefaultProjectionMetrics returns the default projection metrics instance.
func GetDefaultProjectionMetrics() *ProjectionMetrics {
	return defaultProjectionMetrics
}

// Backward compatibility functions for existing code

// RecordProjectionApplied records successful projection application using default metrics.
func RecordProjectionApplied(name string, count int, duration time.Duration, operation string) {
	if defaultProjectionMetrics != nil {
		defaultProjectionMetrics.RecordProjectionOperation(name, operation, duration, true, count)
	}
}

// RecordProjectionError records a projection error using default metrics.
func RecordProjectionError(name string, errorType string) {
	if defaultProjectionMetrics != nil {
		defaultProjectionMetrics.RecordProjectionError(name, "unknown", errorType)
	}
}

// UpdateProjectionLag updates the lag metric using default metrics.
func UpdateProjectionLag(name string, lag time.Duration) {
	if defaultProjectionMetrics != nil {
		defaultProjectionMetrics.UpdateProjectionLag(name, lag)
	}
}

// SetProjectionHealth manually sets the health status using default metrics.
func SetProjectionHealth(name string, healthy bool) {
	if defaultProjectionMetrics != nil {
		defaultProjectionMetrics.SetProjectionHealth(name, healthy)
	}
}
