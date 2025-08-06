// example/metrics/metrics.go
package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// HTTPMetricsCollector implements sync.MetricsCollector with HTTP endpoint
type HTTPMetricsCollector struct {
	syncDurations  map[string]*atomic.Int64
	eventsPushed   atomic.Int64
	eventsPulled   atomic.Int64
	syncErrors     map[string]*atomic.Int64
	conflictsTotal atomic.Int64
	lastSyncTime   atomic.Value // stores time.Time
}

func NewHTTPMetricsCollector() *HTTPMetricsCollector {
	return &HTTPMetricsCollector{
		syncDurations: map[string]*atomic.Int64{
			"push": &atomic.Int64{},
			"pull": &atomic.Int64{},
			"sync": &atomic.Int64{},
			"store": &atomic.Int64{}, // Added for server event generation
		},
		syncErrors: map[string]*atomic.Int64{
			"network":  &atomic.Int64{},
			"storage":  &atomic.Int64{},
			"conflict": &atomic.Int64{},
		},
	}
}

func (m *HTTPMetricsCollector) RecordSyncDuration(operation string, duration time.Duration) {
	if counter, ok := m.syncDurations[operation]; ok {
		counter.Add(duration.Milliseconds())
	}
	m.lastSyncTime.Store(time.Now())
}

func (m *HTTPMetricsCollector) RecordSyncEvents(pushed, pulled int) {
	m.eventsPushed.Add(int64(pushed))
	m.eventsPulled.Add(int64(pulled))
}

func (m *HTTPMetricsCollector) RecordSyncErrors(operation string, errorType string) {
	if counter, ok := m.syncErrors[errorType]; ok {
		counter.Add(1)
	}
}

func (m *HTTPMetricsCollector) RecordConflicts(resolved int) {
	m.conflictsTotal.Add(int64(resolved))
}

// ServeHTTP exposes metrics via HTTP endpoint
func (m *HTTPMetricsCollector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lastSync := m.lastSyncTime.Load()
	var lastSyncStr string
	if lastSync != nil {
		lastSyncStr = lastSync.(time.Time).Format(time.RFC3339)
	}

	metrics := map[string]interface{}{
		"events": map[string]int64{
			"pushed": m.eventsPushed.Load(),
			"pulled": m.eventsPulled.Load(),
		},
		"durations_ms": map[string]int64{
			"push_total":  m.syncDurations["push"].Load(),
			"pull_total":  m.syncDurations["pull"].Load(),
			"sync_total":  m.syncDurations["sync"].Load(),
			"store_total": m.syncDurations["store"].Load(),
		},
		"errors": map[string]int64{
			"network":  m.syncErrors["network"].Load(),
			"storage":  m.syncErrors["storage"].Load(),
			"conflict": m.syncErrors["conflict"].Load(),
		},
		"conflicts_resolved": m.conflictsTotal.Load(),
		"last_sync":          lastSyncStr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
