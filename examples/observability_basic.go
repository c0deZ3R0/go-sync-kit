package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/c0deZ3R0/go-sync-kit/observability/health"
	"github.com/c0deZ3R0/go-sync-kit/observability/metrics"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// This example demonstrates basic observability integration with sync-kit
func main() {
	fmt.Println("=== Go-Sync-Kit Basic Observability Example ===")
	fmt.Println()

	// Create a custom Prometheus registry for clean metrics isolation
	registry := prometheus.NewRegistry()

	// Create Prometheus metrics collector
	metricsCollector, err := metrics.NewPrometheusAdapter(registry)
	if err != nil {
		log.Fatalf("Failed to create metrics collector: %v", err)
	}

	// Create health checker
	healthChecker := health.NewHealthChecker(health.DefaultConfig())

	// Create mock storage and transport for this example
	storage := &MockStorage{}
	transport := &MockTransport{}

	// Add health checks for our components
	setupHealthChecks(healthChecker, storage, transport)

	// Create sync manager with observability
	manager, err := synckit.NewManager(
		synckit.WithStore(storage),
		synckit.WithTransport(transport),
		synckit.WithMetrics(metricsCollector),
		synckit.WithHealthChecker(healthChecker),
	)
	if err != nil {
		log.Fatalf("Failed to create sync manager: %v", err)
	}

	fmt.Println("✅ Sync manager created with observability features")

	// Start observability HTTP server
	go startObservabilityServer(registry, healthChecker)

	// Simulate sync operations with observability
	fmt.Println("\n=== Running Sync Operations ===")
	ctx := context.Background()

	// Perform sync operations
	for i := 0; i < 3; i++ {
		fmt.Printf("Performing sync operation %d...\n", i+1)

		result, err := manager.Sync(ctx)
		if err != nil {
			fmt.Printf("❌ Sync failed: %v\n", err)
			// Record error in metrics
			metricsCollector.RecordError("sync_failed")
		} else {
			fmt.Printf("✅ Sync completed: pushed=%d, pulled=%d, conflicts=%d\n",
				result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
		}

		time.Sleep(2 * time.Second)
	}

	// Check health status
	fmt.Println("\n=== Health Status ===")
	checkHealthStatus(healthChecker)

	// Display metrics information
	fmt.Println("\n=== Observability Endpoints ===")
	fmt.Println("🔗 Prometheus metrics: http://localhost:8080/metrics")
	fmt.Println("🔗 Health status:      http://localhost:8080/health")
	fmt.Println("🔗 Liveness probe:     http://localhost:8080/health/live")
	fmt.Println("🔗 Readiness probe:    http://localhost:8080/health/ready")
	fmt.Println("🔗 Component status:   http://localhost:8080/health/components")

	fmt.Println("\n=== Server Running ===")
	fmt.Println("Press Ctrl+C to exit")

	// Keep the server running
	select {}
}

func setupHealthChecks(checker *health.HealthChecker, storage synckit.EventStore, transport synckit.Transport) {
	// Add storage health check
	if mockStorage, ok := storage.(*MockStorage); ok {
		storageCheck := health.NewStorageCheck("storage_check", mockStorage)
		checker.AddCheck(health.CheckTypeLiveness, storageCheck)
		checker.AddCheck(health.CheckTypeReadiness, storageCheck)
	}

	// Add transport health check
	transportCheck := health.NewTransportCheck("transport_check", transport)
	checker.AddCheck(health.CheckTypeLiveness, transportCheck)
	checker.AddCheck(health.CheckTypeReadiness, transportCheck)

	// Add conflict resolver check
	conflictCheck := health.NewConflictResolverCheck("conflict_resolver_check")
	checker.AddCheck(health.CheckTypeLiveness, conflictCheck)

	// Add memory usage check
	memoryCheck := health.NewMemoryCheck("memory_check", 200) // 200 MB threshold
	checker.AddCheck(health.CheckTypeLiveness, memoryCheck)

	// Add HTTP endpoint check (example external dependency)
	httpCheck := health.NewHTTPCheck("external_service", "https://httpbin.org/status/200")
	checker.AddCheck(health.CheckTypeReadiness, httpCheck)

	fmt.Printf("✅ Added %d health checks\n", len(checker.ListComponents()))
}

func checkHealthStatus(checker *health.HealthChecker) {
	ctx := context.Background()

	// Check liveness
	livenessResult := checker.CheckLiveness(ctx)
	fmt.Printf("Liveness Status: %s (%d up, %d down, %d degraded)\n",
		livenessResult.Status,
		livenessResult.Summary.Up,
		livenessResult.Summary.Down,
		livenessResult.Summary.Degraded)

	// Check readiness
	readinessResult := checker.CheckReadiness(ctx)
	fmt.Printf("Readiness Status: %s (%d up, %d down, %d degraded)\n",
		readinessResult.Status,
		readinessResult.Summary.Up,
		readinessResult.Summary.Down,
		readinessResult.Summary.Degraded)

	// Show individual check details if any failed
	if livenessResult.Status != health.StatusUp {
		fmt.Println("\nFailed Health Checks:")
		for name, result := range livenessResult.Results {
			if result.Status != health.StatusUp {
				fmt.Printf("  ❌ %s: %s - %s\n", name, result.Status, result.Message)
			}
		}
	}
}

func startObservabilityServer(registry *prometheus.Registry, healthChecker *health.HealthChecker) {
	mux := http.NewServeMux()

	// Add Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// Add health check endpoints
	healthHandler := health.NewHTTPHandler(healthChecker, 30*time.Second)
	healthHandler.RegisterRoutes(mux)

	// Add a simple index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Go-Sync-Kit Observability</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 10px 0; }
        .endpoint a { color: #0066cc; text-decoration: none; }
        .endpoint a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>Go-Sync-Kit Observability Dashboard</h1>
    <h2>Available Endpoints:</h2>
    <div class="endpoint">📊 <a href="/metrics">Prometheus Metrics</a> - Raw metrics in Prometheus format</div>
    <div class="endpoint">🔍 <a href="/health">Complete Health Status</a> - Full health report</div>
    <div class="endpoint">💓 <a href="/health/live">Liveness Probe</a> - Kubernetes liveness check</div>
    <div class="endpoint">✅ <a href="/health/ready">Readiness Probe</a> - Kubernetes readiness check</div>
    <div class="endpoint">🚀 <a href="/health/startup">Startup Probe</a> - Kubernetes startup check</div>
    <div class="endpoint">🧩 <a href="/health/components">Component List</a> - Available components</div>

    <h2>Quick Status:</h2>
    <p>This basic example shows how to integrate Prometheus metrics and health checks with go-sync-kit.</p>
    <p>View the metrics endpoint to see real-time sync operation data.</p>
</body>
</html>
`)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Println("🚀 Starting observability server on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Observability server error: %v", err)
	}
}

// Mock implementations for the example
type MockStorage struct{}

func (m *MockStorage) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	return nil
}

func (m *MockStorage) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Return some mock events
	return []synckit.EventWithVersion{
		{Event: &mockEvent{id: "1"}, Version: &mockVersion{v: 1}},
		{Event: &mockEvent{id: "2"}, Version: &mockVersion{v: 2}},
	}, nil
}

func (m *MockStorage) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return m.Load(ctx, since)
}

func (m *MockStorage) LatestVersion(ctx context.Context) (synckit.Version, error) {
	return &mockVersion{v: 2}, nil
}

func (m *MockStorage) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	return &mockVersion{v: 1}, nil
}

func (m *MockStorage) Close() error {
	return nil
}

// MockStorage also implements storage.Storage for health checks
func (m *MockStorage) Put(ctx context.Context, key string, data []byte) error {
	return nil
}

func (m *MockStorage) Get(ctx context.Context, key string) ([]byte, error) {
	return []byte("mock_data"), nil
}

func (m *MockStorage) Delete(ctx context.Context, key string) error {
	return nil
}

type MockTransport struct{}

func (m *MockTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	// Simulate successful push
	return nil
}

func (m *MockTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Return some mock remote events
	return []synckit.EventWithVersion{
		{Event: &mockEvent{id: "remote_1"}, Version: &mockVersion{v: 3}},
	}, nil
}

func (m *MockTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return &mockVersion{v: 3}, nil
}

func (m *MockTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return nil
}

func (m *MockTransport) Close() error {
	return nil
}

type mockEvent struct {
	id string
}

func (e *mockEvent) ID() string                       { return e.id }
func (e *mockEvent) AggregateID() string              { return "test_aggregate" }
func (e *mockEvent) Type() string                     { return "test_event" }
func (e *mockEvent) Data() interface{}                { return map[string]string{"test": "data"} }
func (e *mockEvent) Timestamp() time.Time             { return time.Now() }
func (e *mockEvent) Metadata() map[string]interface{} { return nil }

type mockVersion struct {
	v int
}

func (v *mockVersion) Compare(other synckit.Version) int {
	if otherMock, ok := other.(*mockVersion); ok {
		if v.v < otherMock.v {
			return -1
		} else if v.v > otherMock.v {
			return 1
		}
	}
	return 0
}

func (v *mockVersion) String() string {
	return fmt.Sprintf("%d", v.v)
}

func (v *mockVersion) IsZero() bool {
	return v.v == 0
}
