package dashboard

import (
	"encoding/json"
	"net/http"
	"sync"
)

// EventLog represents a single event in the log
type EventLog struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Timestamp string                 `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// Metrics represents the sync metrics
type Metrics struct {
	Events struct {
		Pushed int `json:"pushed"`
		Pulled int `json:"pulled"`
	} `json:"events"`
	ConflictsResolved int `json:"conflicts_resolved"`
	DurationsMs       struct {
		PushTotal int `json:"push_total"`
		PullTotal int `json:"pull_total"`
		SyncTotal int `json:"sync_total"`
	} `json:"durations_ms"`
	LastSync string         `json:"last_sync"`
	Errors   map[string]int `json:"errors"`
}

var (
	// Keep the last 100 events in memory
	eventLogs    = make([]EventLog, 0, 100)
	eventLogLock sync.RWMutex

	// Metrics data
	metrics     = Metrics{}
	metricsLock sync.RWMutex
)

// AddEvent adds a new event to the log
func AddEvent(event EventLog) {
	eventLogLock.Lock()
	defer eventLogLock.Unlock()

	// Add new event at the beginning
	eventLogs = append([]EventLog{event}, eventLogs...)

	// Keep only last 100 events
	if len(eventLogs) > 100 {
		eventLogs = eventLogs[:100]
	}
}

// UpdateMetrics updates the metrics data
func UpdateMetrics(m Metrics) {
	metricsLock.Lock()
	defer metricsLock.Unlock()
	metrics = m
}

// EventsHandler returns the event logs
func EventsHandler(w http.ResponseWriter, r *http.Request) {
	eventLogLock.RLock()
	defer eventLogLock.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eventLogs)
}

// MetricsHandler returns the current metrics
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	metricsLock.RLock()
	defer metricsLock.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// HTML template for the monitoring dashboard
const dashboardHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Sync Kit Monitoring</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
            margin-bottom: 30px;
        }
        .grid-container {
            display: grid;
            grid-template-columns: 2fr 1fr;
            gap: 20px;
            margin-bottom: 20px;
        }
        .metrics-container {
            display: flex;
            flex-wrap: wrap;
            gap: 20px;
        }
        .metric {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            min-width: 200px;
            text-align: center;
        }
        .terminal {
            background: #2c3e50;
            color: #ecf0f1;
            padding: 20px;
            border-radius: 8px;
            font-family: monospace;
            height: 500px;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
        }
        .terminal-title {
            color: #3498db;
            margin: 0 0 10px 0;
            font-size: 16px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .event-log {
            margin: 5px 0;
            font-size: 14px;
            line-height: 1.4;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
        .event-time {
            color: #95a5a6;
        }
        .event-type {
            color: #e74c3c;
            font-weight: bold;
        }
        .event-content {
            color: #2ecc71;
        }
        .event-metadata {
            color: #f1c40f;
            font-size: 12px;
            margin-left: 20px;
        }
        .metric-value {
            font-size: 32px;
            font-weight: bold;
            color: #2c3e50;
            margin: 10px 0;
        }
        .metric-label {
            color: #666;
            font-size: 14px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .error {
            color: #e74c3c;
            padding: 10px;
            margin: 10px 0;
            border-radius: 4px;
            background: #fadbd8;
        }
        .last-sync {
            font-family: monospace;
            color: #27ae60;
        }
        @media (max-width: 600px) {
            .metric {
                min-width: 100%;
            }
        }
    </style>
</head>
<body>
    <h1>Go Sync Kit - Real-time Monitoring</h1>
    <div class="grid-container">
        <div id="metrics" class="metrics-container"></div>
        <div class="terminal">
            <h2 class="terminal-title">Event Log</h2>
            <div id="event-log"></div>
        </div>
    </div>

    <script>
        function formatDuration(ms) {
            if (!ms || ms === 0) return '0ms';
            if (ms < 1000) return ms + 'ms';
            const s = ms / 1000;
            if (s < 60) return s.toFixed(1) + 's';
            const m = Math.floor(s / 60);
            return m + 'm ' + (s % 60).toFixed(0) + 's';
        }

        function formatTime(isoString) {
            if (!isoString) return 'Never';
            const date = new Date(isoString);
            return date.toLocaleTimeString();
        }

        function formatMetadata(metadata) {
            if (!metadata) return '';
            return Object.entries(metadata)
                .map(([key, value]) => key + ': ' + value)
                .join(', ');
        }

        let lastEventId = '';

        async function fetchEvents() {
            try {
                const response = await fetch('/events');
                if (!response.ok) {
                    throw new Error('Failed to fetch events: ' + response.status);
                }
                const events = await response.json();
                const log = document.getElementById('event-log');

                // Only update if we have new events
                if (events.length > 0 && events[0].id !== lastEventId) {
                    lastEventId = events[0].id;

                    let newHtml = '';
                    events.forEach(event => {
                        const time = new Date(event.timestamp).toLocaleTimeString();
                        const metadata = formatMetadata(event.metadata);
                        newHtml += '<div class="event-log">' +
                            '<span class="event-time">[' + time + ']</span> ' +
                            '<span class="event-type">' + event.type + '</span>: ' +
                            '<span class="event-content">' + event.content + '</span>' +
                            (metadata ? '<div class="event-metadata">' + metadata + '</div>' : '') +
                            '</div>';
                    });
                    log.innerHTML = newHtml;

                    // Fetch metrics when we see new events
                    await fetchMetrics();
                }
            } catch (error) {
                console.error('Failed to fetch events:', error);
                document.getElementById('event-log').innerHTML =
                    '<div class="error">Failed to fetch events: ' + error.message + '</div>';
            }
        }

        async function fetchMetrics() {
            try {
                const response = await fetch('/dashboard-metrics');
                if (!response.ok) {
                    throw new Error('Failed to fetch metrics: ' + response.status);
                }
                const data = await response.json();

                const html = [
                    createMetric('Events Pushed', data.events.pushed || 0),
                    createMetric('Events Pulled', data.events.pulled || 0),
                    createMetric('Conflicts Resolved', data.conflicts_resolved || 0),
                    createMetric('Push Duration', formatDuration(data.durations_ms?.push_total || 0)),
                    createMetric('Pull Duration', formatDuration(data.durations_ms?.pull_total || 0)),
                    createMetric('Sync Duration', formatDuration(data.durations_ms?.sync_total || 0)),
                    createMetric('Last Sync', formatTime(data.last_sync), 'last-sync')
                ].join('');

                // Add error section if there are any
                if (data.errors && Object.values(data.errors).some(count => count > 0)) {
                    html += '<div class="metric"><div class="metric-label">Errors</div>';
                    for (const [type, count] of Object.entries(data.errors)) {
                        if (count > 0) {
                            html += '<div class="error">' + type + ': ' + count + '</div>';
                        }
                    }
                    html += '</div>';
                }

                document.getElementById('metrics').innerHTML = html;
            } catch (error) {
                console.error('Failed to fetch metrics:', error);
                document.getElementById('metrics').innerHTML =
                    '<div class="error">Failed to fetch metrics: ' + error.message + '</div>';
            }
        }

        function createMetric(label, value, extraClass = '') {
            return '<div class="metric">' +
                '<div class="metric-label">' + label + '</div>' +
                '<div class="metric-value ' + extraClass + '">' + value + '</div>' +
                '</div>';
        }

        // Fetch events every 2 seconds
        setInterval(fetchEvents, 2000);
        fetchEvents(); // Initial fetch
    </script>
</body>
</html>
`

// Handler returns an http.HandlerFunc that serves the dashboard
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(dashboardHTML))
	}
}

// InitializeMetrics initializes the metrics with default values
func InitializeMetrics() {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	metrics = Metrics{
		Events: struct {
			Pushed int `json:"pushed"`
			Pulled int `json:"pulled"`
		}{
			Pushed: 0,
			Pulled: 0,
		},
		ConflictsResolved: 0,
		DurationsMs: struct {
			PushTotal int `json:"push_total"`
			PullTotal int `json:"pull_total"`
			SyncTotal int `json:"sync_total"`
		}{
			PushTotal: 0,
			PullTotal: 0,
			SyncTotal: 0,
		},
		LastSync: "",
		Errors:   make(map[string]int),
	}
}
