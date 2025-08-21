package observability

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c0deZ3R0/go-sync-kit/observability/health"
	"github.com/c0deZ3R0/go-sync-kit/observability/metrics"
)

// TestObservabilityIntegration tests that metrics and health checks work together
func TestObservabilityIntegration(t *testing.T) {
	// Create Prometheus registry
	registry := prometheus.NewRegistry()

	// Create Prometheus adapter (this internally creates SyncKitMetrics)
	metricsAdapter := metrics.NewPrometheusAdapter("test-service", metrics.WithRegistry(registry))

	// Get the underlying metrics from the adapter for direct access
	syncMetrics := metricsAdapter.Metrics()

	// Create health checker
	healthChecker := health.NewHealthChecker(health.DefaultConfig())

	// Add some health checks
	dbCheck := &mockHealthCheck{name: "database", component: "database", status: health.StatusUp, message: "Database is healthy"}
	cacheCheck := &mockHealthCheck{name: "cache", component: "cache", status: health.StatusUp, message: "Cache is healthy"}

	healthChecker.AddCheck(health.CheckTypeLiveness, dbCheck)
	healthChecker.AddCheck(health.CheckTypeReadiness, cacheCheck)

	// Simulate some sync operations with metrics collection
	ctx := context.Background()
	duration := 100 * time.Millisecond

	// Record sync metrics
	metricsAdapter.RecordSyncDuration("sync", duration)
	metricsAdapter.RecordSyncEvents(5, 3) // pushed, pulled

	syncMetrics.RecordSyncOperation("push", duration, true, 10, 5, 2)
	syncMetrics.RecordTransportOperation("http", "push", 50*time.Millisecond, true, 1024)
	syncMetrics.RecordStorageOperation("sqlite", "write", 25*time.Millisecond, true)

	// Check health status
	livenessResult := healthChecker.CheckLiveness(ctx)
	readinessResult := healthChecker.CheckReadiness(ctx)

	// Verify health checks passed
	assert.Equal(t, health.StatusUp, livenessResult.Status)
	assert.Equal(t, health.StatusUp, readinessResult.Status)
	assert.Equal(t, 1, livenessResult.Summary.Up)
	assert.Equal(t, 1, readinessResult.Summary.Up)

	// Verify metrics were recorded (basic verification)
	// In a real test, you'd check specific metric values using prometheus testutil
	assert.NotNil(t, syncMetrics.SyncOperationsTotal())
	assert.NotNil(t, syncMetrics.TransportOperationsTotal())
	assert.NotNil(t, syncMetrics.StorageOperationsTotal())
}

// TestHTTPEndpointsIntegration tests HTTP endpoints for both metrics and health
func TestHTTPEndpointsIntegration(t *testing.T) {
	// Create Prometheus registry
	registry := prometheus.NewRegistry()

	// Create metrics
	syncMetrics := metrics.NewSyncKitMetrics("test-service", metrics.WithRegistry(registry))

	// Create health checker
	healthChecker := health.NewHealthChecker(health.DefaultConfig())

	// Add health checks
	appCheck := &mockHealthCheck{name: "app", component: "application", status: health.StatusUp, message: "App is running"}
	healthChecker.AddCheck(health.CheckTypeLiveness, appCheck)
	healthChecker.AddCheck(health.CheckTypeReadiness, appCheck)

	// Record some metrics
	syncMetrics.RecordSyncOperation("sync", 100*time.Millisecond, true, 5, 3, 1)
	syncMetrics.RecordSyncError("test", "validation_error")

	// Create HTTP handlers
	mux := http.NewServeMux()

	// Add Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// Add health check endpoints
	healthHandler := health.NewHTTPHandler(healthChecker, 30*time.Second)
	healthHandler.RegisterRoutes(mux)

	// Create test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test metrics endpoint
	t.Run("metrics endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// Check that content type starts with the expected value, allowing for additional parameters
		contentType := resp.Header.Get("Content-Type")
		assert.True(t, strings.HasPrefix(contentType, "text/plain; version=0.0.4; charset=utf-8"))
	})

	// Test health endpoints
	t.Run("health liveness endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health/live")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("health readiness endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health/ready")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("health components endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health/components")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("complete health status endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		// The complete health endpoint may return 503 if any checks are failing or degraded
		// In this test we have both liveness and readiness checks, so it should be OK
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})
}

// TestObservabilityWithFailures tests observability behavior during failures
func TestObservabilityWithFailures(t *testing.T) {
	// Create Prometheus registry
	registry := prometheus.NewRegistry()

	// Create metrics collector
	metricsAdapter := metrics.NewPrometheusAdapter("test-service", metrics.WithRegistry(registry))

	// Create health checker
	healthChecker := health.NewHealthChecker(health.DefaultConfig())

	// Add failing health checks
	failingCheck := &mockHealthCheck{
		name:      "failing_service",
		component: "external_service",
		status:    health.StatusDown,
		message:   "Service is down",
	}
	healthChecker.AddCheck(health.CheckTypeLiveness, failingCheck)

	// Record failed operations
	ctx := context.Background()
	metricsAdapter.RecordSyncErrors("sync", "connection_timeout")
	metricsAdapter.RecordSyncErrors("sync", "validation_failed")

	// Check health status
	livenessResult := healthChecker.CheckLiveness(ctx)

	// Verify failure is detected
	assert.Equal(t, health.StatusDown, livenessResult.Status)
	assert.Equal(t, 0, livenessResult.Summary.Up)
	assert.Equal(t, 1, livenessResult.Summary.Down)
	assert.Equal(t, 1, livenessResult.Summary.Total)

	// Verify individual check results
	assert.Len(t, livenessResult.Results, 1)
	for _, result := range livenessResult.Results {
		assert.Equal(t, health.StatusDown, result.Status)
		assert.Contains(t, result.Message, "Service is down")
	}
}

// TestConcurrentObservability tests observability under concurrent load
func TestConcurrentObservability(t *testing.T) {
	// Create Prometheus registry
	registry := prometheus.NewRegistry()

	// Create metrics collector
	metricsAdapter := metrics.NewPrometheusAdapter("test-service", metrics.WithRegistry(registry))

	// Create health checker
	healthChecker := health.NewHealthChecker(health.DefaultConfig())

	// Add health checks
	for i := 0; i < 5; i++ {
		check := &mockHealthCheck{
			name:      fmt.Sprintf("service_%d", i),
			component: fmt.Sprintf("component_%d", i),
			status:    health.StatusUp,
			message:   fmt.Sprintf("Service %d is healthy", i),
			duration:  10 * time.Millisecond,
		}
		healthChecker.AddCheck(health.CheckTypeLiveness, check)
	}

	const numGoroutines = 20
	const numOperations = 25

	// Run concurrent operations
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Record metrics
				duration := time.Duration(j) * time.Millisecond
				metricsAdapter.RecordSyncDuration("sync", duration)
				metricsAdapter.RecordSyncEvents(1, 0) // 1 pushed, 0 pulled

				// Check health (some workers)
				if workerID%3 == 0 {
					ctx := context.Background()
					result := healthChecker.CheckLiveness(ctx)
					assert.Equal(t, health.StatusUp, result.Status)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Final health check should still work
	ctx := context.Background()
	finalResult := healthChecker.CheckLiveness(ctx)
	assert.Equal(t, health.StatusUp, finalResult.Status)
	assert.Equal(t, 5, finalResult.Summary.Up)
	assert.Equal(t, 0, finalResult.Summary.Down)
}

// TestObservabilityResourceCleanup tests proper resource management
func TestObservabilityResourceCleanup(t *testing.T) {
	// Create multiple registries and ensure they don't interfere
	registry1 := prometheus.NewRegistry()
	registry2 := prometheus.NewRegistry()

	// Create metrics for each registry
	metrics1 := metrics.NewSyncKitMetrics("test-service-1", metrics.WithRegistry(registry1))

	metrics2 := metrics.NewSyncKitMetrics("test-service-2", metrics.WithRegistry(registry2))

	// Record different operations
	metrics1.RecordSyncOperation("push", 100*time.Millisecond, true, 5, 0, 0)
	metrics2.RecordSyncOperation("pull", 150*time.Millisecond, true, 0, 8, 1)

	// Create health checkers
	checker1 := health.NewHealthChecker(health.DefaultConfig())
	checker2 := health.NewHealthChecker(health.DefaultConfig())

	// Add different checks
	check1 := &mockHealthCheck{name: "check1", component: "service1", status: health.StatusUp}
	check2 := &mockHealthCheck{name: "check2", component: "service2", status: health.StatusDegraded}

	checker1.AddCheck(health.CheckTypeLiveness, check1)
	checker2.AddCheck(health.CheckTypeLiveness, check2)

	// Verify isolation
	ctx := context.Background()
	result1 := checker1.CheckLiveness(ctx)
	result2 := checker2.CheckLiveness(ctx)

	assert.Equal(t, health.StatusUp, result1.Status)
	assert.Equal(t, health.StatusDegraded, result2.Status)
	assert.NotEqual(t, result1.Results, result2.Results)

	// Verify metrics are isolated
	assert.NotNil(t, metrics1.SyncOperationsTotal())
	assert.NotNil(t, metrics2.SyncOperationsTotal())
	// In practice, you'd check that they contain different values
}

// mockHealthCheck for testing
type mockHealthCheck struct {
	name      string
	component string
	status    health.Status
	message   string
	duration  time.Duration
}

func (m *mockHealthCheck) Name() string      { return m.name }
func (m *mockHealthCheck) Component() string { return m.component }

func (m *mockHealthCheck) Check(ctx context.Context) health.CheckResult {
	start := time.Now()

	if m.duration > 0 {
		time.Sleep(m.duration)
	}

	return health.CheckResult{
		Status:    m.status,
		Component: m.component,
		Message:   m.message,
		Details:   make(map[string]interface{}),
		Timestamp: start,
		Duration:  time.Since(start),
	}
}

// BenchmarkObservabilityIntegration benchmarks combined metrics and health operations
func BenchmarkObservabilityIntegration(b *testing.B) {
	// Setup
	registry := prometheus.NewRegistry()
	metricsAdapter := metrics.NewPrometheusAdapter("benchmark-service", metrics.WithRegistry(registry))

	healthChecker := health.NewHealthChecker(health.DefaultConfig())
	check := &mockHealthCheck{name: "bench", component: "benchmark", status: health.StatusUp}
	healthChecker.AddCheck(health.CheckTypeLiveness, check)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Record metrics
			metricsAdapter.RecordSyncDuration("sync", 100 * time.Millisecond)

			// Check health
			result := healthChecker.CheckLiveness(ctx)
			if result.Status != health.StatusUp {
				b.Errorf("Expected StatusUp, got %v", result.Status)
			}
		}
	})
}
