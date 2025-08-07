package synckit

import "time"

// MetricsCollector provides hooks for collecting sync operation metrics
type MetricsCollector interface {
	// RecordSyncDuration records how long a sync operation took
	RecordSyncDuration(operation string, duration time.Duration)

	// RecordSyncEvents records the number of events pushed and pulled
	RecordSyncEvents(pushed, pulled int)

	// RecordSyncErrors records sync operation errors by type
	RecordSyncErrors(operation string, errorType string)

	// RecordConflicts records the number of conflicts resolved
	RecordConflicts(resolved int)
}

// NoOpMetricsCollector is a default implementation that does nothing
type NoOpMetricsCollector struct{}

func (n *NoOpMetricsCollector) RecordSyncDuration(operation string, duration time.Duration) {}
func (n *NoOpMetricsCollector) RecordSyncEvents(pushed, pulled int)                         {}
func (n *NoOpMetricsCollector) RecordSyncErrors(operation string, errorType string)         {}
func (n *NoOpMetricsCollector) RecordConflicts(resolved int)                                {}
