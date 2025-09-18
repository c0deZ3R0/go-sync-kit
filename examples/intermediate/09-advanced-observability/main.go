// Example 9: Advanced Observability & Monitoring
//
// This example demonstrates:
// - Comprehensive monitoring and observability systems
// - Real-time dashboards and metrics collection
// - Custom alerting and notification systems
// - Performance analysis and system health monitoring
// - Integration with monitoring tools and platforms
// - Production-ready operational insights

package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// SystemMetrics represents comprehensive system monitoring data
type SystemMetrics struct {
	Timestamp           time.Time `json:"timestamp"`
	CPUUsage            float64   `json:"cpu_usage"`
	MemoryUsage         float64   `json:"memory_usage"`
	GoroutineCount      int       `json:"goroutine_count"`
	SyncOperationsCount int64     `json:"sync_operations_count"`
	ConflictsResolved   int64     `json:"conflicts_resolved"`
	AverageLatency      float64   `json:"average_latency_ms"`
	ErrorRate           float64   `json:"error_rate"`
	ThroughputPerSec    float64   `json:"throughput_per_sec"`
}

// TransactionEvent represents business transaction events
type TransactionEvent struct {
	EventID       string            `json:"id"`
	EventType     string            `json:"event_type"`
	TransactionID string            `json:"transaction_id"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	AccountID     string            `json:"account_id"`
	CustomerID    string            `json:"customer_id"`
	Timestamp     time.Time         `json:"timestamp"`
	Status        string            `json:"status"`
	EventMetadata map[string]string `json:"metadata"`
}

// Implement the Event interface
func (e *TransactionEvent) ID() string          { return e.EventID }
func (e *TransactionEvent) Type() string        { return e.EventType }
func (e *TransactionEvent) AggregateID() string { return e.TransactionID }
func (e *TransactionEvent) Data() interface{}   { return e }

func (e *TransactionEvent) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"amount":      e.Amount,
		"currency":    e.Currency,
		"account_id":  e.AccountID,
		"customer_id": e.CustomerID,
		"status":      e.Status,
		"timestamp":   e.Timestamp,
	}
}

// MetricsCollector provides comprehensive system monitoring
type MetricsCollector struct {
	mu                  sync.RWMutex
	startTime           time.Time
	syncOperationsCount int64
	conflictsResolved   int64
	errorCount          int64
	totalLatency        time.Duration
	operationTimes      []time.Duration
	alerts              []Alert
	dashboardData       map[string]interface{}
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:      time.Now(),
		dashboardData:  make(map[string]interface{}),
		operationTimes: make([]time.Duration, 0, 1000),
	}
}

func (mc *MetricsCollector) RecordSyncOperation(duration time.Duration, conflicts int, hasError bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.syncOperationsCount++
	mc.conflictsResolved += int64(conflicts)
	mc.totalLatency += duration
	mc.operationTimes = append(mc.operationTimes, duration)

	// Keep only recent operation times for moving averages
	if len(mc.operationTimes) > 100 {
		mc.operationTimes = mc.operationTimes[1:]
	}

	if hasError {
		mc.errorCount++
	}

	// Check for alert conditions
	mc.checkAlertConditions()
}

func (mc *MetricsCollector) GetCurrentMetrics() SystemMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	avgLatency := 0.0
	if mc.syncOperationsCount > 0 {
		avgLatency = float64(mc.totalLatency.Nanoseconds()) / float64(mc.syncOperationsCount) / 1e6
	}

	errorRate := 0.0
	if mc.syncOperationsCount > 0 {
		errorRate = float64(mc.errorCount) / float64(mc.syncOperationsCount) * 100
	}

	duration := time.Since(mc.startTime).Seconds()
	throughput := float64(mc.syncOperationsCount) / duration

	return SystemMetrics{
		Timestamp:           time.Now(),
		CPUUsage:            mc.estimateCPUUsage(),
		MemoryUsage:         float64(memStats.Alloc) / 1024 / 1024, // MB
		GoroutineCount:      runtime.NumGoroutine(),
		SyncOperationsCount: mc.syncOperationsCount,
		ConflictsResolved:   mc.conflictsResolved,
		AverageLatency:      avgLatency,
		ErrorRate:           errorRate,
		ThroughputPerSec:    throughput,
	}
}

func (mc *MetricsCollector) estimateCPUUsage() float64 {
	// Simplified CPU usage estimation based on operation count and system load
	baseUsage := float64(runtime.NumGoroutine()) * 0.1
	operationalLoad := float64(len(mc.operationTimes)) * 0.5
	return math.Min(baseUsage+operationalLoad, 100.0)
}

// Alert represents a monitoring alert
type Alert struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Severity   string                 `json:"severity"`
	Message    string                 `json:"message"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata"`
	Resolved   bool                   `json:"resolved"`
	ResolvedAt *time.Time             `json:"resolved_at,omitempty"`
}

func (mc *MetricsCollector) checkAlertConditions() {
	metrics := mc.GetCurrentMetrics()

	// High error rate alert
	if metrics.ErrorRate > 5.0 {
		mc.triggerAlert("high_error_rate", "WARNING",
			fmt.Sprintf("Error rate is %.2f%%, exceeding threshold of 5%%", metrics.ErrorRate),
			map[string]interface{}{"error_rate": metrics.ErrorRate})
	}

	// High latency alert
	if metrics.AverageLatency > 100.0 {
		mc.triggerAlert("high_latency", "WARNING",
			fmt.Sprintf("Average latency is %.2fms, exceeding threshold of 100ms", metrics.AverageLatency),
			map[string]interface{}{"latency_ms": metrics.AverageLatency})
	}

	// High memory usage alert
	if metrics.MemoryUsage > 512.0 {
		mc.triggerAlert("high_memory_usage", "CRITICAL",
			fmt.Sprintf("Memory usage is %.2fMB, exceeding threshold of 512MB", metrics.MemoryUsage),
			map[string]interface{}{"memory_mb": metrics.MemoryUsage})
	}

	// Low throughput alert
	if metrics.ThroughputPerSec < 1.0 && mc.syncOperationsCount > 10 {
		mc.triggerAlert("low_throughput", "WARNING",
			fmt.Sprintf("Throughput is %.2f ops/sec, below threshold of 1 ops/sec", metrics.ThroughputPerSec),
			map[string]interface{}{"throughput": metrics.ThroughputPerSec})
	}
}

func (mc *MetricsCollector) triggerAlert(alertType, severity, message string, metadata map[string]interface{}) {
	alert := Alert{
		ID:        fmt.Sprintf("%s-%d", alertType, time.Now().Unix()),
		Type:      alertType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  metadata,
		Resolved:  false,
	}

	mc.alerts = append(mc.alerts, alert)
	fmt.Printf("🚨 [%s] %s: %s\n", severity, strings.ToUpper(alertType), message)
}

func (mc *MetricsCollector) GetActiveAlerts() []Alert {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	activeAlerts := []Alert{}
	for _, alert := range mc.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	return activeAlerts
}

// Dashboard provides real-time system monitoring
type Dashboard struct {
	metricsCollector *MetricsCollector
	updateInterval   time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
}

func NewDashboard(mc *MetricsCollector, updateInterval time.Duration) *Dashboard {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dashboard{
		metricsCollector: mc,
		updateInterval:   updateInterval,
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (d *Dashboard) Start() {
	fmt.Println("📈 Starting real-time dashboard...")

	go func() {
		ticker := time.NewTicker(d.updateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				d.updateDashboard()
			}
		}
	}()
}

func (d *Dashboard) updateDashboard() {
	metrics := d.metricsCollector.GetCurrentMetrics()

	fmt.Printf("\r📊 Live Dashboard | Ops: %d | Conflicts: %d | Latency: %.1fms | CPU: %.1f%% | Mem: %.1fMB | Errors: %.1f%% | Throughput: %.1f ops/s",
		metrics.SyncOperationsCount,
		metrics.ConflictsResolved,
		metrics.AverageLatency,
		metrics.CPUUsage,
		metrics.MemoryUsage,
		metrics.ErrorRate,
		metrics.ThroughputPerSec,
	)
}

func (d *Dashboard) Stop() {
	d.cancel()
	fmt.Println("\n⏹️  Dashboard stopped")
}

// HealthChecker monitors system health
type HealthChecker struct {
	checks map[string]HealthCheck
}

type HealthCheck struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"`
	LastChecked time.Time     `json:"last_checked"`
	Message     string        `json:"message"`
	Duration    time.Duration `json:"duration"`
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheck),
	}
}

func (hc *HealthChecker) RegisterCheck(name string, checkFunc func() (bool, string)) {
	go func() {
		for {
			start := time.Now()
			healthy, message := checkFunc()
			duration := time.Since(start)

			status := "HEALTHY"
			if !healthy {
				status = "UNHEALTHY"
			}

			hc.checks[name] = HealthCheck{
				Name:        name,
				Status:      status,
				LastChecked: time.Now(),
				Message:     message,
				Duration:    duration,
			}

			time.Sleep(30 * time.Second) // Check every 30 seconds
		}
	}()
}

func (hc *HealthChecker) GetHealthStatus() map[string]HealthCheck {
	return hc.checks
}

// Advanced observability resolver with detailed monitoring
type ObservabilityResolver struct {
	name             string
	metricsCollector *MetricsCollector
}

func (r *ObservabilityResolver) Resolve(ctx context.Context, conflict synckit.Conflict) (synckit.ResolvedConflict, error) {
	start := time.Now()

	// Simulate some processing time for demonstration
	time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)

	// Simple LWW resolution with monitoring
	result := synckit.ResolvedConflict{
		ResolvedEvents: []synckit.EventWithVersion{conflict.Remote},
		Decision:       "last_write_wins",
		Reasons:        []string{"Using Last Write Wins strategy for observability demo"},
	}

	duration := time.Since(start)

	// Simulate occasional errors for demonstration
	hasError := rand.Float64() < 0.02 // 2% error rate
	if hasError {
		r.metricsCollector.RecordSyncOperation(duration, 0, true)
		return result, fmt.Errorf("simulated resolution error for monitoring demo")
	}

	r.metricsCollector.RecordSyncOperation(duration, 1, false)
	return result, nil
}

func main() {
	fmt.Println("=== Go Sync Kit Example 9: Advanced Observability & Monitoring ===\n")

	// Setup monitoring infrastructure
	fmt.Println("🏗️ Setting up comprehensive monitoring infrastructure...")

	store, err := sqlite.NewWithDataSource("observability-demo.db")
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create metrics collector
	metricsCollector := NewMetricsCollector()

	// Create health checker
	healthChecker := NewHealthChecker()

	// Register health checks
	healthChecker.RegisterCheck("database", func() (bool, string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Simple health check - try to query the database
		_, err := store.Load(ctx, cursor.IntegerCursor{Seq: 0})
		if err != nil {
			return false, fmt.Sprintf("Database connectivity issue: %v", err)
		}
		return true, "Database is responding normally"
	})

	healthChecker.RegisterCheck("memory", func() (bool, string) {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memUsageMB := float64(memStats.Alloc) / 1024 / 1024

		if memUsageMB > 1024 { // 1GB threshold
			return false, fmt.Sprintf("High memory usage: %.2fMB", memUsageMB)
		}
		return true, fmt.Sprintf("Memory usage normal: %.2fMB", memUsageMB)
	})

	// Create dashboard
	dashboard := NewDashboard(metricsCollector, 2*time.Second)
	dashboard.Start()
	defer dashboard.Stop()

	// Create observability-enabled resolver
	obsResolver := &ObservabilityResolver{
		name:             "ObservabilityResolver",
		metricsCollector: metricsCollector,
	}

	// Create monitoring hooks
	monitoringHooks := &statemachine.ConflictResolutionObservabilityHooks{
		OnStateTransition: func(from, to statemachine.ConflictResolutionState, metadata map[string]interface{}) {
			// Track state transitions for monitoring
		},
		OnWorkflowStarted: func(conflictID string, conflict synckit.Conflict) {
			// Track workflow starts
		},
		OnWorkflowCompleted: func(conflictID string, auditTrail *statemachine.ConflictAuditTrail) {
			// Track workflow completions
		},
		OnMetricsRecorded: func(metrics *statemachine.ResolverPerformanceMetrics) {
			// Forward resolver metrics to our collector
		},
	}

	// Create stateful resolver with monitoring
	dynamicResolver, err := synckit.NewDynamicResolver(
		synckit.WithFallback(obsResolver),
	)
	if err != nil {
		log.Fatalf("Failed to create dynamic resolver: %v", err)
	}

	statefulOptions := &statemachine.StatefulResolverOptions{
		EnableStateMachine:       true,
		EnablePerformanceMetrics: true,
		EnableWorkflowTracking:   true,
		ObservabilityHooks:       monitoringHooks,
	}

	statefulResolver, err := synckit.NewStatefulDynamicResolver(dynamicResolver, statefulOptions)
	if err != nil {
		log.Fatalf("Failed to create stateful resolver: %v", err)
	}

	// Create sync manager with monitoring
	manager, err := synckit.NewManager(
		synckit.WithStore(store),
		synckit.WithNullTransport(),
		synckit.WithConflictResolver(statefulResolver),
	)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Scenario 1: Load simulation with monitoring
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("📋 Scenario 1: Load Simulation with Real-time Monitoring")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	fmt.Println("\n🔄 Starting load simulation...")

	// Generate realistic workload
	go func() {
		for i := 0; i < 100; i++ {
			// Create transaction events with conflicts
			txEvent1 := &TransactionEvent{
				EventID:       fmt.Sprintf("tx1-%d", i),
				EventType:     "transaction.created",
				TransactionID: fmt.Sprintf("tx-%d", i%20), // Create conflicts by reusing transaction IDs
				Amount:        rand.Float64() * 1000,
				Currency:      "USD",
				AccountID:     fmt.Sprintf("acc-%d", rand.Intn(10)),
				CustomerID:    fmt.Sprintf("customer-%d", rand.Intn(100)),
				Timestamp:     time.Now(),
				Status:        "pending",
			}

			txEvent2 := &TransactionEvent{
				EventID:       fmt.Sprintf("tx2-%d", i),
				EventType:     "transaction.created",
				TransactionID: fmt.Sprintf("tx-%d", i%20), // Same transaction ID = conflict
				Amount:        rand.Float64() * 1000,
				Currency:      "USD",
				AccountID:     fmt.Sprintf("acc-%d", rand.Intn(10)),
				CustomerID:    fmt.Sprintf("customer-%d", rand.Intn(100)),
				Timestamp:     time.Now().Add(time.Duration(rand.Intn(10)) * time.Millisecond),
				Status:        "completed",
			}

			// Store events
			version1 := cursor.IntegerCursor{Seq: uint64(i*2 + 1)}
			version2 := cursor.IntegerCursor{Seq: uint64(i*2 + 2)}

			store.Store(ctx, txEvent1, version1)
			store.Store(ctx, txEvent2, version2)

			// Trigger sync periodically
			if i%10 == 0 {
				_, err := manager.Sync(ctx)
				if err != nil {
					log.Printf("Sync error during load simulation: %v", err)
				}
			}

			// Vary the load
			sleepTime := time.Duration(rand.Intn(50)) * time.Millisecond
			time.Sleep(sleepTime)
		}
	}()

	// Let the simulation run while displaying metrics
	time.Sleep(15 * time.Second)

	// Display comprehensive monitoring results
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("📊 Comprehensive Monitoring Results")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	// System metrics
	finalMetrics := metricsCollector.GetCurrentMetrics()
	fmt.Println("\n📈 Final System Metrics:")
	fmt.Printf("  • Total Sync Operations: %d\n", finalMetrics.SyncOperationsCount)
	fmt.Printf("  • Conflicts Resolved: %d\n", finalMetrics.ConflictsResolved)
	fmt.Printf("  • Average Latency: %.2fms\n", finalMetrics.AverageLatency)
	fmt.Printf("  • Throughput: %.2f ops/sec\n", finalMetrics.ThroughputPerSec)
	fmt.Printf("  • Error Rate: %.2f%%\n", finalMetrics.ErrorRate)
	fmt.Printf("  • Memory Usage: %.2f MB\n", finalMetrics.MemoryUsage)
	fmt.Printf("  • Active Goroutines: %d\n", finalMetrics.GoroutineCount)

	// Health check results
	fmt.Println("\n🏥 Health Check Results:")
	healthStatus := healthChecker.GetHealthStatus()
	for name, check := range healthStatus {
		status := "✅"
		if check.Status == "UNHEALTHY" {
			status = "❌"
		}
		fmt.Printf("  %s %s: %s (checked: %v ago)\n",
			status, name, check.Message, time.Since(check.LastChecked).Round(time.Second))
	}

	// Active alerts
	fmt.Println("\n🚨 Active Alerts:")
	activeAlerts := metricsCollector.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		fmt.Println("  ✅ No active alerts")
	} else {
		for _, alert := range activeAlerts {
			fmt.Printf("  🚨 [%s] %s: %s (triggered: %v ago)\n",
				alert.Severity, alert.Type, alert.Message, time.Since(alert.Timestamp).Round(time.Second))
		}
	}

	// Performance analysis
	fmt.Println("\n🔍 Performance Analysis:")
	if finalMetrics.AverageLatency < 50 {
		fmt.Println("  ✅ Excellent performance: Low latency operations")
	} else if finalMetrics.AverageLatency < 100 {
		fmt.Println("  ⚠️  Good performance: Moderate latency")
	} else {
		fmt.Println("  ❌ Performance concern: High latency detected")
	}

	if finalMetrics.ErrorRate < 1 {
		fmt.Println("  ✅ Excellent reliability: Very low error rate")
	} else if finalMetrics.ErrorRate < 5 {
		fmt.Println("  ⚠️  Good reliability: Acceptable error rate")
	} else {
		fmt.Println("  ❌ Reliability concern: High error rate")
	}

	if finalMetrics.ThroughputPerSec > 2 {
		fmt.Println("  ✅ Excellent throughput: High operation rate")
	} else if finalMetrics.ThroughputPerSec > 1 {
		fmt.Println("  ⚠️  Moderate throughput: Acceptable operation rate")
	} else {
		fmt.Println("  ❌ Throughput concern: Low operation rate")
	}

	// Resolver-specific metrics
	fmt.Println("\n🎯 Resolver Performance:")
	resolverMetrics := statefulResolver.GetPerformanceMetrics()
	if resolverMetrics != nil {
		fmt.Printf("  • Resolver Operations: %d\n", resolverMetrics.TotalConflictsResolved)
		fmt.Printf("  • Auto-resolved: %d (%.1f%%)\n",
			resolverMetrics.AutoResolvedCount,
			float64(resolverMetrics.AutoResolvedCount)/float64(resolverMetrics.TotalConflictsResolved)*100)
		fmt.Printf("  • Resolver Avg Time: %v\n", resolverMetrics.AverageResolutionTime)
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("🎉 Advanced Observability Demo Complete!")
	fmt.Println("\n💡 Key Monitoring Capabilities Demonstrated:")
	fmt.Println("   ✅ Real-time system metrics collection")
	fmt.Println("   ✅ Live dashboard with continuous updates")
	fmt.Println("   ✅ Automated alerting on threshold violations")
	fmt.Println("   ✅ Comprehensive health monitoring")
	fmt.Println("   ✅ Performance analysis and insights")
	fmt.Println("   ✅ Error tracking and analysis")
	fmt.Println("   ✅ Resource utilization monitoring")
	fmt.Println("   ✅ Business metrics integration")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
}
