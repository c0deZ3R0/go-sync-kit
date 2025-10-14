package health

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Mock health check for testing
type mockHealthCheck struct {
	name      string
	component string
	status    Status
	message   string
	duration  time.Duration
}

func (m *mockHealthCheck) Name() string      { return m.name }
func (m *mockHealthCheck) Component() string { return m.component }

func (m *mockHealthCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()

	if m.duration > 0 {
		// Respect context cancellation by using a timer instead of just sleeping
		timer := time.NewTimer(m.duration)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Duration completed normally
		case <-ctx.Done():
			// Context was cancelled, return early
			return CheckResult{
				Status:    StatusDown, // Context cancellation could indicate unhealthy state
				Component: m.component,
				Message:   "Health check cancelled due to context timeout",
				Details:   make(map[string]interface{}),
				Timestamp: start,
				Duration:  time.Since(start),
			}
		}
	}

	return CheckResult{
		Status:    m.status,
		Component: m.component,
		Message:   m.message,
		Details:   make(map[string]interface{}),
		Timestamp: start,
		Duration:  time.Since(start),
	}
}

func TestNewHealthChecker(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "with default config",
			config: DefaultConfig(),
		},
		{
			name: "with custom config",
			config: Config{
				Timeout:          10 * time.Second,
				CheckInterval:    15 * time.Second,
				FailureThreshold: 5,
				SuccessThreshold: 2,
			},
		},
		{
			name: "with zero timeout uses default",
			config: Config{
				Timeout: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewHealthChecker(tt.config)

			assert.NotNil(t, checker)
			assert.NotNil(t, checker.checks)
			assert.NotZero(t, checker.config.Timeout)
		})
	}
}

func TestHealthChecker_AddCheck(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	check1 := &mockHealthCheck{name: "check1", component: "component1", status: StatusUp}
	check2 := &mockHealthCheck{name: "check2", component: "component1", status: StatusUp}
	check3 := &mockHealthCheck{name: "check3", component: "component2", status: StatusUp}

	// Add checks
	checker.AddCheck(CheckTypeLiveness, check1)
	checker.AddCheck(CheckTypeLiveness, check2)
	checker.AddCheck(CheckTypeReadiness, check3)

	// Verify checks are stored
	assert.Len(t, checker.checks, 2) // 2 components
	assert.Len(t, checker.checks["component1"][CheckTypeLiveness], 2)
	assert.Len(t, checker.checks["component2"][CheckTypeReadiness], 1)
}

func TestHealthChecker_RemoveCheck(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	check1 := &mockHealthCheck{name: "check1", component: "component1", status: StatusUp}
	check2 := &mockHealthCheck{name: "check2", component: "component1", status: StatusUp}

	// Add checks
	checker.AddCheck(CheckTypeLiveness, check1)
	checker.AddCheck(CheckTypeLiveness, check2)

	// Verify checks are added
	assert.Len(t, checker.checks["component1"][CheckTypeLiveness], 2)

	// Remove one check
	checker.RemoveCheck(CheckTypeLiveness, "check1")

	// Verify check is removed
	assert.Len(t, checker.checks["component1"][CheckTypeLiveness], 1)
	assert.Equal(t, "check2", checker.checks["component1"][CheckTypeLiveness][0].Name())
}

func TestHealthChecker_CheckLiveness(t *testing.T) {
	tests := []struct {
		name           string
		checks         []*mockHealthCheck
		expectedStatus Status
		expectedUp     int
		expectedDown   int
	}{
		{
			name: "all checks up",
			checks: []*mockHealthCheck{
				{name: "check1", component: "comp1", status: StatusUp, message: "OK"},
				{name: "check2", component: "comp2", status: StatusUp, message: "OK"},
			},
			expectedStatus: StatusUp,
			expectedUp:     2,
			expectedDown:   0,
		},
		{
			name: "one check down fails overall",
			checks: []*mockHealthCheck{
				{name: "check1", component: "comp1", status: StatusUp, message: "OK"},
				{name: "check2", component: "comp2", status: StatusDown, message: "Failed"},
			},
			expectedStatus: StatusDown,
			expectedUp:     1,
			expectedDown:   1,
		},
		{
			name: "degraded takes precedence over up",
			checks: []*mockHealthCheck{
				{name: "check1", component: "comp1", status: StatusUp, message: "OK"},
				{name: "check2", component: "comp2", status: StatusDegraded, message: "Slow"},
			},
			expectedStatus: StatusDegraded,
			expectedUp:     1,
			expectedDown:   0,
		},
		{
			name:           "no checks returns unknown",
			checks:         []*mockHealthCheck{},
			expectedStatus: StatusUnknown,
			expectedUp:     0,
			expectedDown:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewHealthChecker(DefaultConfig())

			for _, check := range tt.checks {
				checker.AddCheck(CheckTypeLiveness, check)
			}

			ctx := context.Background()
			result := checker.CheckLiveness(ctx)

			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, CheckTypeLiveness, result.CheckType)
			assert.Equal(t, tt.expectedUp, result.Summary.Up)
			assert.Equal(t, tt.expectedDown, result.Summary.Down)
			assert.Equal(t, len(tt.checks), result.Summary.Total)
			assert.NotZero(t, result.Duration)
		})
	}
}

func TestHealthChecker_CheckReadiness(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	check1 := &mockHealthCheck{name: "check1", component: "comp1", status: StatusUp, message: "Ready"}
	check2 := &mockHealthCheck{name: "check2", component: "comp2", status: StatusDown, message: "Not ready"}

	checker.AddCheck(CheckTypeReadiness, check1)
	checker.AddCheck(CheckTypeReadiness, check2)

	ctx := context.Background()
	result := checker.CheckReadiness(ctx)

	assert.Equal(t, StatusDown, result.Status)
	assert.Equal(t, CheckTypeReadiness, result.CheckType)
	assert.Equal(t, 1, result.Summary.Up)
	assert.Equal(t, 1, result.Summary.Down)
	assert.Equal(t, 2, result.Summary.Total)
}

func TestHealthChecker_CheckStartup(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	check1 := &mockHealthCheck{name: "startup_check", component: "app", status: StatusUp, message: "Started"}

	checker.AddCheck(CheckTypeStartup, check1)

	ctx := context.Background()
	result := checker.CheckStartup(ctx)

	assert.Equal(t, StatusUp, result.Status)
	assert.Equal(t, CheckTypeStartup, result.CheckType)
	assert.Equal(t, 1, result.Summary.Up)
	assert.Equal(t, 0, result.Summary.Down)
}

func TestHealthChecker_CheckAll(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	livenessCheck := &mockHealthCheck{name: "liveness", component: "app", status: StatusUp}
	readinessCheck := &mockHealthCheck{name: "readiness", component: "app", status: StatusDegraded}
	startupCheck := &mockHealthCheck{name: "startup", component: "app", status: StatusUp}

	checker.AddCheck(CheckTypeLiveness, livenessCheck)
	checker.AddCheck(CheckTypeReadiness, readinessCheck)
	checker.AddCheck(CheckTypeStartup, startupCheck)

	ctx := context.Background()
	results := checker.CheckAll(ctx)

	assert.Len(t, results, 3)
	assert.Contains(t, results, CheckTypeLiveness)
	assert.Contains(t, results, CheckTypeReadiness)
	assert.Contains(t, results, CheckTypeStartup)

	assert.Equal(t, StatusUp, results[CheckTypeLiveness].Status)
	assert.Equal(t, StatusDegraded, results[CheckTypeReadiness].Status)
	assert.Equal(t, StatusUp, results[CheckTypeStartup].Status)
}

func TestHealthChecker_GetComponentStatus(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	livenessCheck := &mockHealthCheck{name: "liveness", component: "database", status: StatusUp}
	readinessCheck := &mockHealthCheck{name: "readiness", component: "database", status: StatusDown}

	checker.AddCheck(CheckTypeLiveness, livenessCheck)
	checker.AddCheck(CheckTypeReadiness, readinessCheck)

	ctx := context.Background()
	results := checker.GetComponentStatus(ctx, "database")

	assert.Len(t, results, 2)
	assert.Equal(t, StatusUp, results[CheckTypeLiveness].Status)
	assert.Equal(t, StatusDown, results[CheckTypeReadiness].Status)

	// Test non-existent component
	emptyResults := checker.GetComponentStatus(ctx, "nonexistent")
	assert.Len(t, emptyResults, 0)
}

func TestHealthChecker_ListComponents(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	check1 := &mockHealthCheck{name: "check1", component: "database"}
	check2 := &mockHealthCheck{name: "check2", component: "cache"}
	check3 := &mockHealthCheck{name: "check3", component: "database"} // Same component

	checker.AddCheck(CheckTypeLiveness, check1)
	checker.AddCheck(CheckTypeLiveness, check2)
	checker.AddCheck(CheckTypeReadiness, check3)

	components := checker.ListComponents()

	assert.Len(t, components, 2)
	assert.Contains(t, components, "database")
	assert.Contains(t, components, "cache")
}

func TestHealthChecker_Timeout(t *testing.T) {
	config := Config{
		Timeout: 50 * time.Millisecond,
	}
	checker := NewHealthChecker(config)

	// Add a slow check
	slowCheck := &mockHealthCheck{
		name:      "slow_check",
		component: "slow",
		status:    StatusUp,
		duration:  100 * time.Millisecond, // Longer than timeout
	}

	checker.AddCheck(CheckTypeLiveness, slowCheck)

	ctx := context.Background()
	start := time.Now()
	result := checker.CheckLiveness(ctx)
	duration := time.Since(start)

	// Should complete around the timeout duration
	assert.True(t, duration < 200*time.Millisecond, "Check should respect timeout")

	// The check itself should still complete and be included in results
	assert.Equal(t, 1, result.Summary.Total)
}

func TestHealthChecker_ConcurrentChecks(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	// Add multiple checks that will run concurrently
	for i := 0; i < 10; i++ {
		check := &mockHealthCheck{
			name:      fmt.Sprintf("check_%d", i),
			component: fmt.Sprintf("component_%d", i),
			status:    StatusUp,
			duration:  10 * time.Millisecond,
		}
		checker.AddCheck(CheckTypeLiveness, check)
	}

	ctx := context.Background()

	// Run multiple checks concurrently
	const numConcurrent = 5
	done := make(chan OverallResult, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func() {
			result := checker.CheckLiveness(ctx)
			done <- result
		}()
	}

	// Collect all results
	results := make([]OverallResult, numConcurrent)
	for i := 0; i < numConcurrent; i++ {
		results[i] = <-done
	}

	// Verify all results are consistent
	for _, result := range results {
		assert.Equal(t, StatusUp, result.Status)
		assert.Equal(t, 10, result.Summary.Total)
		assert.Equal(t, 10, result.Summary.Up)
	}
}

func TestHealthChecker_ContextCancellation(t *testing.T) {
	checker := NewHealthChecker(DefaultConfig())

	longCheck := &mockHealthCheck{
		name:      "long_check",
		component: "slow",
		status:    StatusUp,
		duration:  200 * time.Millisecond,
	}

	checker.AddCheck(CheckTypeLiveness, longCheck)

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	result := checker.CheckLiveness(ctx)
	duration := time.Since(start)

	// Should return quickly due to context cancellation
	assert.True(t, duration < 150*time.Millisecond, "Should respect context cancellation")

	// But should still have some results
	assert.Equal(t, 1, result.Summary.Total)
}

func BenchmarkHealthChecker_CheckLiveness(b *testing.B) {
	checker := NewHealthChecker(DefaultConfig())

	// Add several fast checks
	for i := 0; i < 10; i++ {
		check := &mockHealthCheck{
			name:      fmt.Sprintf("check_%d", i),
			component: fmt.Sprintf("component_%d", i),
			status:    StatusUp,
		}
		checker.AddCheck(CheckTypeLiveness, check)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckLiveness(ctx)
	}
}

func BenchmarkHealthChecker_ConcurrentChecks(b *testing.B) {
	checker := NewHealthChecker(DefaultConfig())

	// Add several fast checks
	for i := 0; i < 10; i++ {
		check := &mockHealthCheck{
			name:      fmt.Sprintf("check_%d", i),
			component: fmt.Sprintf("component_%d", i),
			status:    StatusUp,
		}
		checker.AddCheck(CheckTypeLiveness, check)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			checker.CheckLiveness(ctx)
		}
	})
}
