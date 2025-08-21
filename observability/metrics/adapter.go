package metrics

import (
	"time"
)

// MetricsCollectorAdapter adapts SyncKitMetrics to implement the synckit.MetricsCollector interface.
// This allows seamless integration with the existing SyncManager metrics system while providing
// comprehensive Prometheus metrics collection.
type MetricsCollectorAdapter struct {
	metrics *SyncKitMetrics
}

// NewAdapter creates a new adapter that implements synckit.MetricsCollector using SyncKitMetrics.
func NewAdapter(metrics *SyncKitMetrics) *MetricsCollectorAdapter {
	return &MetricsCollectorAdapter{
		metrics: metrics,
	}
}

// RecordSyncDuration records how long a sync operation took.
// This implements the synckit.MetricsCollector interface.
func (a *MetricsCollectorAdapter) RecordSyncDuration(operation string, duration time.Duration) {
	// Record the sync operation as successful (since we only get duration for successful operations)
	a.metrics.RecordSyncOperation(operation, duration, true, 0, 0, 0)
}

// RecordSyncEvents records the number of events pushed and pulled.
// This implements the synckit.MetricsCollector interface.
func (a *MetricsCollectorAdapter) RecordSyncEvents(pushed, pulled int) {
	// We need to record these with a sync operation
	// Since we don't have the operation context here, we'll use "sync" as default
	operation := "sync"
	duration := time.Duration(0) // No duration information available in this interface

	// Record the operation with event counts
	a.metrics.RecordSyncOperation(operation, duration, true, pushed, pulled, 0)
}

// RecordSyncErrors records sync operation errors by type.
// This implements the synckit.MetricsCollector interface.
func (a *MetricsCollectorAdapter) RecordSyncErrors(operation string, errorType string) {
	a.metrics.RecordSyncError(operation, errorType)
}

// RecordConflicts records the number of conflicts resolved.
// This implements the synckit.MetricsCollector interface.
func (a *MetricsCollectorAdapter) RecordConflicts(resolved int) {
	// Record as part of a sync operation with conflicts resolved
	operation := "sync"
	duration := time.Duration(0) // No duration information available in this interface

	a.metrics.RecordSyncOperation(operation, duration, true, 0, 0, resolved)
}

// Metrics returns the underlying SyncKitMetrics for direct access.
func (a *MetricsCollectorAdapter) Metrics() *SyncKitMetrics {
	return a.metrics
}

// ExtendedMetricsCollector provides additional methods beyond the basic synckit.MetricsCollector interface.
// This can be used when you need more detailed metrics collection while still being compatible with
// the existing interface.
type ExtendedMetricsCollector interface {
	// Basic synckit.MetricsCollector methods
	RecordSyncDuration(operation string, duration time.Duration)
	RecordSyncEvents(pushed, pulled int)
	RecordSyncErrors(operation string, errorType string)
	RecordConflicts(resolved int)

	// Extended methods for more detailed metrics
	RecordSyncOperationComplete(operation string, duration time.Duration, success bool, eventsPushed, eventsPulled, conflictsResolved int)
	RecordTransportOperation(protocol, operation string, duration time.Duration, success bool, bytesTransferred int64)
	RecordStorageOperation(backend, operation string, duration time.Duration, success bool)
	RecordConflictResolution(strategy string, duration time.Duration, result string)

	// System metrics
	SetActiveConnections(count int)
	SetMemoryUsage(bytes int64)
	SetGoroutines(count int)
}

// ExtendedAdapter implements ExtendedMetricsCollector with full access to SyncKitMetrics capabilities.
type ExtendedAdapter struct {
	*MetricsCollectorAdapter
}

// NewExtendedAdapter creates a new extended adapter with full metrics capabilities.
func NewExtendedAdapter(metrics *SyncKitMetrics) *ExtendedAdapter {
	return &ExtendedAdapter{
		MetricsCollectorAdapter: NewAdapter(metrics),
	}
}

// RecordSyncOperationComplete records comprehensive sync operation metrics.
func (e *ExtendedAdapter) RecordSyncOperationComplete(operation string, duration time.Duration, success bool, eventsPushed, eventsPulled, conflictsResolved int) {
	e.metrics.RecordSyncOperation(operation, duration, success, eventsPushed, eventsPulled, conflictsResolved)
}

// RecordTransportOperation records transport layer metrics.
func (e *ExtendedAdapter) RecordTransportOperation(protocol, operation string, duration time.Duration, success bool, bytesTransferred int64) {
	e.metrics.RecordTransportOperation(protocol, operation, duration, success, bytesTransferred)
}

// RecordStorageOperation records storage operation metrics.
func (e *ExtendedAdapter) RecordStorageOperation(backend, operation string, duration time.Duration, success bool) {
	e.metrics.RecordStorageOperation(backend, operation, duration, success)
}

// RecordConflictResolution records conflict resolution metrics.
func (e *ExtendedAdapter) RecordConflictResolution(strategy string, duration time.Duration, result string) {
	e.metrics.RecordConflictResolution(strategy, duration, result)
}

// SetActiveConnections updates active connections gauge.
func (e *ExtendedAdapter) SetActiveConnections(count int) {
	e.metrics.SetActiveConnections(count)
}

// SetMemoryUsage updates memory usage gauge.
func (e *ExtendedAdapter) SetMemoryUsage(bytes int64) {
	e.metrics.SetMemoryUsage(bytes)
}

// SetGoroutines updates goroutines count gauge.
func (e *ExtendedAdapter) SetGoroutines(count int) {
	e.metrics.SetGoroutines(count)
}
