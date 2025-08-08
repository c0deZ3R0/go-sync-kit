package sync

import "time"

// MetricsCollector provides hooks for observability.
type MetricsCollector interface {
	// RecordSyncDuration records how long a sync operation took
	RecordSyncDuration(op string, d time.Duration)

	// RecordSyncEvents records how many events were synced
	RecordSyncEvents(pushed, pulled int)

	// RecordConflicts records how many conflicts were resolved
	RecordConflicts(count int)

	// RecordSyncErrors records sync operation errors
	RecordSyncErrors(op, reason string)
}

// NoOpMetricsCollector is a stub implementation that discards metrics.
type NoOpMetricsCollector struct{}

func (*NoOpMetricsCollector) RecordSyncDuration(op string, d time.Duration) {}
func (*NoOpMetricsCollector) RecordSyncEvents(pushed, pulled int)           {}
func (*NoOpMetricsCollector) RecordConflicts(count int)                     {}
func (*NoOpMetricsCollector) RecordSyncErrors(op, reason string)            {}
