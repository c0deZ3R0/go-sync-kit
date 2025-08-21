package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/observability/health"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// This example demonstrates how to integrate health checking with sync-kit
func main() {
	ctx := context.Background()

	// Create a health checker
	healthChecker := health.NewHealthChecker(health.DefaultConfig())

	// Create a simple in-memory store and null transport for this example
	// In practice, you would use real implementations
	store := &MockEventStore{}
	transport := &MockTransport{}

	// Create sync manager with health checking
	manager, err := synckit.NewManager(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithHealthChecker(healthChecker),
	)
	if err != nil {
		log.Fatalf("Failed to create sync manager: %v", err)
	}

	// Add sync-kit specific health checks
	setupSyncKitHealthChecks(healthChecker, manager, store, transport)

	// Add some generic system checks
	setupSystemHealthChecks(healthChecker)

	// Test health checks
	fmt.Println("=== Testing Health Checks ===")

	// Test liveness checks
	fmt.Println("\n--- Liveness Checks ---")
	livenessResult := healthChecker.CheckLiveness(ctx)
	printHealthResult("Liveness", livenessResult)

	// Test readiness checks
	fmt.Println("\n--- Readiness Checks ---")
	readinessResult := healthChecker.CheckReadiness(ctx)
	printHealthResult("Readiness", readinessResult)

	// Test startup checks
	fmt.Println("\n--- Startup Checks ---")
	startupResult := healthChecker.CheckStartup(ctx)
	printHealthResult("Startup", startupResult)

	// Create HTTP server with health endpoints
	fmt.Println("\n=== Starting Health Check Server ===")
	startHealthCheckServer(healthChecker)

	// Keep the example running for a bit
	select {}
}

func setupSyncKitHealthChecks(checker *health.HealthChecker, manager synckit.SyncManager, store synckit.EventStore, transport synckit.Transport) {
	// Add SyncManager health check
	syncManagerCheck := health.NewSyncManagerCheck("sync_manager_check", manager)
	checker.AddCheck(health.CheckTypeLiveness, syncManagerCheck)
	checker.AddCheck(health.CheckTypeReadiness, syncManagerCheck)

	// Add storage health check (using mock)
	if mockStore, ok := store.(*MockEventStore); ok {
		storageCheck := health.NewStorageCheck("storage_check", mockStore)
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
	checker.AddCheck(health.CheckTypeStartup, conflictCheck)

	// Add network connectivity check
	peers := []string{"peer1:8080", "peer2:8080"}
	networkCheck := health.NewNetworkConnectivityCheck("network_check", peers)
	checker.AddCheck(health.CheckTypeReadiness, networkCheck)
}

func setupSystemHealthChecks(checker *health.HealthChecker) {
	// Add memory check
	memoryCheck := health.NewMemoryCheck("memory_check", 500) // 500 MB threshold
	checker.AddCheck(health.CheckTypeLiveness, memoryCheck)

	// Add HTTP endpoint check (example external dependency)
	httpCheck := health.NewHTTPCheck("external_api_check", "https://httpbin.org/status/200")
	checker.AddCheck(health.CheckTypeReadiness, httpCheck)

	// Add composite check combining multiple checks
	tcpCheck := health.NewTCPCheck("db_tcp_check", "localhost:5432")
	compositeCheck := health.NewCompositeCheck("database_health", "database", tcpCheck)
	checker.AddCheck(health.CheckTypeReadiness, compositeCheck)
}

func printHealthResult(checkType string, result health.OverallResult) {
	fmt.Printf("%s Status: %s\n", checkType, result.Status)
	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Printf("  Summary: %d total, %d up, %d down, %d degraded, %d unknown\n",
		result.Summary.Total, result.Summary.Up, result.Summary.Down,
		result.Summary.Degraded, result.Summary.Unknown)

	if len(result.Results) > 0 {
		fmt.Printf("  Individual Results:\n")
		for name, checkResult := range result.Results {
			fmt.Printf("    - %s: %s (%v) - %s\n",
				name, checkResult.Status, checkResult.Duration, checkResult.Message)
		}
	}
}

func startHealthCheckServer(checker *health.HealthChecker) {
	// Create HTTP handler for health checks
	handler := health.NewHTTPHandler(checker, 30*time.Second)

	// Create a new ServeMux for health endpoints
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Add a simple index handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Sync-Kit Health Check Server</title>
</head>
<body>
    <h1>Sync-Kit Health Check Server</h1>
    <p>Available endpoints:</p>
    <ul>
        <li><a href="/health">/health</a> - Complete health status</li>
        <li><a href="/health/live">/health/live</a> - Liveness probe</li>
        <li><a href="/health/ready">/health/ready</a> - Readiness probe</li>
        <li><a href="/health/startup">/health/startup</a> - Startup probe</li>
        <li><a href="/health/components">/health/components</a> - List components</li>
        <li><a href="/ping">/ping</a> - Simple ping</li>
    </ul>
</body>
</html>
`)
	})

	// Start server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Printf("Health check server starting on :8080\n")
		fmt.Printf("Visit http://localhost:8080 for available endpoints\n")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health check server error: %v", err)
		}
	}()
}

// Mock implementations for the example
type MockEventStore struct{}

func (m *MockEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	return nil
}

func (m *MockEventStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, nil
}

func (m *MockEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, nil
}

func (m *MockEventStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
	return &mockVersion{}, nil
}

func (m *MockEventStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	return &mockVersion{}, nil
}

func (m *MockEventStore) Close() error {
	return nil
}

// MockEventStore also implements the storage.Storage interface for health checks
func (m *MockEventStore) Put(ctx context.Context, key string, data []byte) error {
	return nil
}

func (m *MockEventStore) Get(ctx context.Context, key string) ([]byte, error) {
	return data, nil
}

func (m *MockEventStore) Delete(ctx context.Context, key string) error {
	return nil
}

type MockTransport struct{}

func (m *MockTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	return nil
}

func (m *MockTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, nil
}

func (m *MockTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return &mockVersion{}, nil
}

func (m *MockTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return nil
}

func (m *MockTransport) Close() error {
	return nil
}

type mockVersion struct{}

func (v *mockVersion) Compare(other synckit.Version) int { return 0 }
func (v *mockVersion) String() string                    { return "0" }
func (v *mockVersion) IsZero() bool                      { return true }
