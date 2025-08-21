package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yourusername/go-sync-kit/observability/health"
	"github.com/yourusername/go-sync-kit/observability/metrics"
	"github.com/yourusername/go-sync-kit/synckit"
)

// Enterprise observability example demonstrating production-ready configuration
func main() {
	fmt.Println("=== Go-Sync-Kit Enterprise Observability Example ===")
	fmt.Println()

	// Load configuration from environment
	config := loadConfig()
	fmt.Printf("📋 Configuration loaded: environment=%s, metrics_port=%d, health_port=%d\n", 
		config.Environment, config.MetricsPort, config.HealthPort)

	// Create enterprise-grade observability setup
	observability := setupEnterpriseObservability(config)
	fmt.Println("✅ Enterprise observability configured")

	// Create storage and transport with realistic components
	storage := &EnterpriseStorage{env: config.Environment}
	transport := &EnterpriseTransport{env: config.Environment}

	// Setup comprehensive health checks
	setupEnterpriseHealthChecks(observability.HealthChecker, storage, transport, config)
	fmt.Printf("✅ Enterprise health checks configured (%d components)\n", 
		len(observability.HealthChecker.ListComponents()))

	// Create sync manager with full observability
	manager, err := synckit.NewManager(
		synckit.WithStore(storage),
		synckit.WithTransport(transport),
		synckit.WithMetrics(observability.MetricsAdapter),
		synckit.WithHealthChecker(observability.HealthChecker),
		synckit.WithBatchSize(config.BatchSize),
		synckit.WithTimeout(config.SyncTimeout),
		synckit.WithLWW(), // Last-Write-Wins conflict resolution
	)
	if err != nil {
		log.Fatalf("Failed to create enterprise sync manager: %v", err)
	}

	fmt.Println("✅ Enterprise sync manager created")

	// Start observability servers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start metrics server
	go startEnterpriseMetricsServer(observability.Registry, config.MetricsPort)
	
	// Start dedicated health server
	go startEnterpriseHealthServer(observability.HealthChecker, config.HealthPort)

	// Start monitoring goroutine
	go startSystemMonitoring(observability.SyncMetrics)

	// Start business metrics collection
	go startBusinessMetricsCollection(observability.MetricsAdapter, config.Environment)

	// Setup graceful shutdown
	setupGracefulShutdown(cancel, manager)

	// Simulate enterprise sync workload
	fmt.Println("\n=== Running Enterprise Sync Workload ===")
	runEnterpriseWorkload(ctx, manager, observability.MetricsAdapter, config)

	fmt.Println("\n🏁 Enterprise example completed")
}

// Configuration represents enterprise configuration
type Configuration struct {
	Environment     string
	MetricsPort     int
	HealthPort      int
	BatchSize       int
	SyncTimeout     time.Duration
	WorkloadTenants []string
	WorkloadRate    time.Duration
}

func loadConfig() Configuration {
	// In a real application, use viper, env vars, config files, etc.
	return Configuration{
		Environment:     getEnv("SYNC_KIT_ENV", "production"),
		MetricsPort:     8080,
		HealthPort:      8081,
		BatchSize:       250,
		SyncTimeout:     30 * time.Second,
		WorkloadTenants: []string{"acme-corp", "globex-ltd", "initech-inc"},
		WorkloadRate:    5 * time.Second,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// EnterpriseObservability encapsulates all observability components
type EnterpriseObservability struct {
	Registry       *prometheus.Registry
	SyncMetrics    *metrics.SyncKitMetrics
	MetricsAdapter *metrics.PrometheusAdapter
	HealthChecker  *health.HealthChecker
}

func setupEnterpriseObservability(config Configuration) *EnterpriseObservability {
	// Create isolated registry for clean metrics
	registry := prometheus.NewRegistry()

	// Add standard Go runtime metrics
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Create sync-kit specific metrics
	syncMetrics, err := metrics.NewSyncKitMetrics(registry)
	if err != nil {
		log.Fatalf("Failed to create sync metrics: %v", err)
	}

	// Create metrics adapter with enhanced capabilities
	metricsAdapter, err := metrics.NewPrometheusAdapter(registry)
	if err != nil {
		log.Fatalf("Failed to create metrics adapter: %v", err)
	}

	// Create health checker with production-ready configuration
	healthConfig := health.Config{
		Timeout:          10 * time.Second,
		CheckInterval:    30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
	healthChecker := health.NewHealthChecker(healthConfig)

	return &EnterpriseObservability{
		Registry:       registry,
		SyncMetrics:    syncMetrics,
		MetricsAdapter: metricsAdapter,
		HealthChecker:  healthChecker,
	}
}

func setupEnterpriseHealthChecks(healthChecker *health.HealthChecker, storage *EnterpriseStorage, transport *EnterpriseTransport, config Configuration) {
	// Core application health checks
	appCheck := health.NewSyncManagerCheck("application", nil)
	healthChecker.AddCheck(health.CheckTypeLiveness, appCheck)
	healthChecker.AddCheck(health.CheckTypeReadiness, appCheck)
	healthChecker.AddCheck(health.CheckTypeStartup, appCheck)

	// Storage health checks
	storageCheck := health.NewStorageCheck("primary_storage", storage,
		health.WithStorageTestKey("health_check_primary"),
	)
	healthChecker.AddCheck(health.CheckTypeLiveness, storageCheck)
	healthChecker.AddCheck(health.CheckTypeReadiness, storageCheck)

	// Transport health checks
	transportCheck := health.NewTransportCheck("primary_transport", transport,
		health.WithTransportTestPeer("primary_peer"),
	)
	healthChecker.AddCheck(health.CheckTypeLiveness, transportCheck)
	healthChecker.AddCheck(health.CheckTypeReadiness, transportCheck)

	// External service dependencies
	if config.Environment == "production" {
		// Production external services
		authServiceCheck := health.NewHTTPCheck("auth_service", "https://auth.company.com/health",
			health.WithHTTPTimeout(5*time.Second),
			health.WithExpectedStatus(http.StatusOK),
		)
		healthChecker.AddCheck(health.CheckTypeReadiness, authServiceCheck)

		dbCheck := health.NewTCPCheck("primary_database", "db.company.com:5432",
			health.WithTCPTimeout(3*time.Second),
		)
		healthChecker.AddCheck(health.CheckTypeLiveness, dbCheck)
	} else {
		// Development/staging external services
		testServiceCheck := health.NewHTTPCheck("test_service", "https://httpbin.org/status/200")
		healthChecker.AddCheck(health.CheckTypeReadiness, testServiceCheck)
	}

	// System resource health checks
	memoryCheck := health.NewMemoryCheck("memory_usage", 1024) // 1GB threshold
	healthChecker.AddCheck(health.CheckTypeLiveness, memoryCheck)

	// Business logic health checks
	conflictResolverCheck := health.NewConflictResolverCheck("conflict_resolution")
	healthChecker.AddCheck(health.CheckTypeLiveness, conflictResolverCheck)

	// Network connectivity checks for multiple peers
	peers := []string{"peer1.company.com:8080", "peer2.company.com:8080"}
	networkCheck := health.NewNetworkConnectivityCheck("peer_connectivity", peers,
		health.WithNetworkTimeout(5*time.Second),
	)
	healthChecker.AddCheck(health.CheckTypeReadiness, networkCheck)
}

func startEnterpriseMetricsServer(registry *prometheus.Registry, port int) {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint with enterprise configuration
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: 10,
		Timeout:            10 * time.Second,
	}))

	// Metrics endpoint with additional metadata
	mux.HandleFunc("/metrics/metadata", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"service": "go-sync-kit",
			"version": "1.0.0",
			"environment": "%s",
			"metrics_endpoint": "/metrics",
			"collection_interval": "15s",
			"retention": "15d"
		}`, getEnv("SYNC_KIT_ENV", "production"))
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	fmt.Printf("🚀 Starting enterprise metrics server on port %d\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Metrics server error: %v", err)
	}
}

func startEnterpriseHealthServer(healthChecker *health.HealthChecker, port int) {
	// Create dedicated health server for isolation
	server := health.NewHealthCheckServer(healthChecker, fmt.Sprintf(":%d", port))

	fmt.Printf("🚀 Starting enterprise health server on port %d\n", port)
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Printf("Health server error: %v", err)
	}
}

func startSystemMonitoring(syncMetrics *metrics.SyncKitMetrics) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Update system metrics
		syncMetrics.UpdateGoroutineCount(int64(len(make(chan struct{}, 1000)))) // Simplified
		syncMetrics.UpdateMemoryUsage(128 * 1024 * 1024)                        // 128 MB
		syncMetrics.UpdateUptime(time.Since(time.Now().Add(-time.Hour)))        // Simplified uptime
	}
}

func startBusinessMetricsCollection(adapter *metrics.PrometheusAdapter, environment string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Collect business metrics
		labels := map[string]string{
			"environment": environment,
			"service":     "sync-service",
		}

		adapter.RecordBusinessMetric("active_users", labels, 1250.0)
		adapter.RecordBusinessMetric("data_volume_gb", labels, 45.8)
		adapter.RecordBusinessMetric("api_requests_per_minute", labels, 850.0)
	}
}

func runEnterpriseWorkload(ctx context.Context, manager synckit.SyncManager, metricsAdapter *metrics.PrometheusAdapter, config Configuration) {
	workloadTicker := time.NewTicker(config.WorkloadRate)
	defer workloadTicker.Stop()

	tenantIndex := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("🛑 Stopping enterprise workload")
			return
		case <-workloadTicker.C:
			// Simulate tenant-specific sync operations
			tenant := config.WorkloadTenants[tenantIndex%len(config.WorkloadTenants)]
			tenantIndex++

			fmt.Printf("🔄 Processing sync for tenant: %s\n", tenant)

			// Record tenant-specific metrics
			tenantLabels := map[string]string{
				"tenant":      tenant,
				"operation":   "tenant_sync",
				"environment": config.Environment,
			}
			metricsAdapter.RecordBusinessMetric("tenant_sync_operations", tenantLabels, 1.0)

			// Perform sync operation
			syncCtx, cancel := context.WithTimeout(ctx, config.SyncTimeout)
			result, err := manager.Sync(syncCtx)
			cancel()

			if err != nil {
				fmt.Printf("❌ Sync failed for tenant %s: %v\n", tenant, err)
				metricsAdapter.RecordError("tenant_sync_failed")
				
				errorLabels := map[string]string{
					"tenant":      tenant,
					"error_type":  "sync_failure",
					"environment": config.Environment,
				}
				metricsAdapter.RecordBusinessMetric("tenant_errors", errorLabels, 1.0)
			} else {
				fmt.Printf("✅ Sync completed for tenant %s: pushed=%d, pulled=%d, conflicts=%d, duration=%v\n",
					tenant, result.EventsPushed, result.EventsPulled, result.ConflictsResolved, result.Duration)

				// Record detailed business metrics
				successLabels := map[string]string{
					"tenant":      tenant,
					"environment": config.Environment,
				}
				metricsAdapter.RecordBusinessMetric("tenant_events_processed", successLabels, float64(result.EventsPushed+result.EventsPulled))
				metricsAdapter.RecordBusinessMetric("tenant_conflicts_resolved", successLabels, float64(result.ConflictsResolved))
			}
		}
	}
}

func setupGracefulShutdown(cancel context.CancelFunc, manager synckit.SyncManager) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\n🚨 Received signal %v, initiating graceful shutdown...\n", sig)

		// Cancel context to stop workload
		cancel()

		// Close sync manager
		if err := manager.Close(); err != nil {
			fmt.Printf("⚠️  Error closing sync manager: %v\n", err)
		}

		fmt.Println("✅ Graceful shutdown completed")
		os.Exit(0)
	}()
}

// Enterprise storage implementation
type EnterpriseStorage struct {
	env string
}

func (e *EnterpriseStorage) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	// Simulate enterprise storage with proper error handling
	return nil
}

func (e *EnterpriseStorage) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Return mock enterprise events
	events := make([]synckit.EventWithVersion, 0, 10)
	for i := 0; i < 10; i++ {
		events = append(events, synckit.EventWithVersion{
			Event:   &enterpriseEvent{id: fmt.Sprintf("enterprise_%d", i), env: e.env},
			Version: &enterpriseVersion{v: int64(i + 1)},
		})
	}
	return events, nil
}

func (e *EnterpriseStorage) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return e.Load(ctx, since)
}

func (e *EnterpriseStorage) LatestVersion(ctx context.Context) (synckit.Version, error) {
	return &enterpriseVersion{v: 10}, nil
}

func (e *EnterpriseStorage) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	return &enterpriseVersion{v: 1}, nil
}

func (e *EnterpriseStorage) Close() error {
	return nil
}

// EnterpriseStorage also implements storage.Storage for health checks
func (e *EnterpriseStorage) Put(ctx context.Context, key string, data []byte) error {
	return nil
}

func (e *EnterpriseStorage) Get(ctx context.Context, key string) ([]byte, error) {
	return []byte(fmt.Sprintf("enterprise_data_%s", e.env)), nil
}

func (e *EnterpriseStorage) Delete(ctx context.Context, key string) error {
	return nil
}

// Enterprise transport implementation
type EnterpriseTransport struct {
	env string
}

func (e *EnterpriseTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	// Simulate enterprise transport
	return nil
}

func (e *EnterpriseTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	// Return mock remote events
	return []synckit.EventWithVersion{
		{Event: &enterpriseEvent{id: "remote_enterprise", env: e.env}, Version: &enterpriseVersion{v: 11}},
	}, nil
}

func (e *EnterpriseTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return &enterpriseVersion{v: 11}, nil
}

func (e *EnterpriseTransport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return nil
}

func (e *EnterpriseTransport) Close() error {
	return nil
}

type enterpriseEvent struct {
	id  string
	env string
}

func (e *enterpriseEvent) ID() string                        { return e.id }
func (e *enterpriseEvent) AggregateID() string              { return "enterprise_aggregate" }
func (e *enterpriseEvent) Type() string                     { return "enterprise_event" }
func (e *enterpriseEvent) Data() interface{}                { return map[string]string{"env": e.env, "enterprise": "true"} }
func (e *enterpriseEvent) Timestamp() time.Time             { return time.Now() }
func (e *enterpriseEvent) Metadata() map[string]interface{} { return map[string]interface{}{"environment": e.env} }

type enterpriseVersion struct {
	v int64
}

func (v *enterpriseVersion) Compare(other synckit.Version) int {
	if otherEnt, ok := other.(*enterpriseVersion); ok {
		if v.v < otherEnt.v {
			return -1
		} else if v.v > otherEnt.v {
			return 1
		}
	}
	return 0
}

func (v *enterpriseVersion) String() string {
	return fmt.Sprintf("%d", v.v)
}

func (v *enterpriseVersion) IsZero() bool {
	return v.v == 0
}
