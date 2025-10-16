// Package health provides health checking capabilities for go-sync-kit.
// It implements liveness, readiness, and startup probes following Kubernetes
// conventions and provides component-level health monitoring.
package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Status represents the health status of a component.
type Status string

const (
	// StatusUp indicates the component is healthy and fully functional
	StatusUp Status = "up"
	// StatusDown indicates the component is unhealthy and not functional
	StatusDown Status = "down"
	// StatusDegraded indicates the component is partially functional
	StatusDegraded Status = "degraded"
	// StatusUnknown indicates the component status cannot be determined
	StatusUnknown Status = "unknown"
)

// CheckType represents the type of health check.
type CheckType string

const (
	// CheckTypeLiveness indicates a liveness probe (is the service alive?)
	CheckTypeLiveness CheckType = "liveness"
	// CheckTypeReadiness indicates a readiness probe (is the service ready to serve?)
	CheckTypeReadiness CheckType = "readiness"
	// CheckTypeStartup indicates a startup probe (has the service finished starting?)
	CheckTypeStartup CheckType = "startup"
)

// CheckResult represents the result of a health check.
type CheckResult struct {
	// Status is the health status of the component
	Status Status `json:"status"`
	// Component is the name of the component being checked
	Component string `json:"component"`
	// Message provides additional information about the health status
	Message string `json:"message,omitempty"`
	// Details provides structured details about the check
	Details map[string]interface{} `json:"details,omitempty"`
	// Timestamp is when the check was performed
	Timestamp time.Time `json:"timestamp"`
	// Duration is how long the check took
	Duration time.Duration `json:"duration"`
}

// HealthCheck represents a single health check function.
type HealthCheck interface {
	// Check performs the health check and returns the result
	Check(ctx context.Context) CheckResult
	// Name returns the name of the health check
	Name() string
	// Component returns the component name this check is for
	Component() string
}

// HealthChecker manages and executes health checks for sync-kit components.
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]map[CheckType][]HealthCheck // component -> check type -> checks
	config Config
}

// Config configures the health checker behavior.
type Config struct {
	// Timeout is the maximum time to wait for all health checks
	Timeout time.Duration
	// CheckInterval is how often to run periodic health checks (0 disables)
	CheckInterval time.Duration
	// FailureThreshold is how many consecutive failures before marking as down
	FailureThreshold int
	// SuccessThreshold is how many consecutive successes before marking as up
	SuccessThreshold int
}

// DefaultConfig returns a default configuration for health checking.
func DefaultConfig() Config {
	return Config{
		Timeout:          30 * time.Second,
		CheckInterval:    30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 1,
	}
}

// NewHealthChecker creates a new health checker with the given configuration.
func NewHealthChecker(config Config) *HealthChecker {
	if config.Timeout == 0 {
		config = DefaultConfig()
	}

	return &HealthChecker{
		checks: make(map[string]map[CheckType][]HealthCheck),
		config: config,
	}
}

// AddCheck adds a health check for a specific component and check type.
func (h *HealthChecker) AddCheck(checkType CheckType, check HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()

	component := check.Component()
	if h.checks[component] == nil {
		h.checks[component] = make(map[CheckType][]HealthCheck)
	}

	h.checks[component][checkType] = append(h.checks[component][checkType], check)
}

// RemoveCheck removes a specific health check.
func (h *HealthChecker) RemoveCheck(checkType CheckType, checkName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for component := range h.checks {
		if checks, exists := h.checks[component][checkType]; exists {
			for i, check := range checks {
				if check.Name() == checkName {
					h.checks[component][checkType] = append(checks[:i], checks[i+1:]...)
					return
				}
			}
		}
	}
}

// CheckLiveness performs all liveness checks and returns the overall result.
func (h *HealthChecker) CheckLiveness(ctx context.Context) OverallResult {
	return h.runChecks(ctx, CheckTypeLiveness)
}

// CheckReadiness performs all readiness checks and returns the overall result.
func (h *HealthChecker) CheckReadiness(ctx context.Context) OverallResult {
	return h.runChecks(ctx, CheckTypeReadiness)
}

// CheckStartup performs all startup checks and returns the overall result.
func (h *HealthChecker) CheckStartup(ctx context.Context) OverallResult {
	return h.runChecks(ctx, CheckTypeStartup)
}

// CheckAll performs all health checks and returns detailed results.
func (h *HealthChecker) CheckAll(ctx context.Context) map[CheckType]OverallResult {
	results := make(map[CheckType]OverallResult)

	results[CheckTypeLiveness] = h.CheckLiveness(ctx)
	results[CheckTypeReadiness] = h.CheckReadiness(ctx)
	results[CheckTypeStartup] = h.CheckStartup(ctx)

	return results
}

// OverallResult represents the result of multiple health checks.
type OverallResult struct {
	// Status is the overall health status
	Status Status `json:"status"`
	// CheckType is the type of checks performed
	CheckType CheckType `json:"check_type"`
	// Results contains individual check results by component
	Results map[string]CheckResult `json:"results"`
	// Summary provides a high-level summary
	Summary Summary `json:"summary"`
	// Timestamp is when the checks were performed
	Timestamp time.Time `json:"timestamp"`
	// Duration is how long all checks took
	Duration time.Duration `json:"duration"`
}

// Summary provides a summary of health check results.
type Summary struct {
	// Total is the total number of checks performed
	Total int `json:"total"`
	// Up is the number of checks that passed
	Up int `json:"up"`
	// Down is the number of checks that failed
	Down int `json:"down"`
	// Degraded is the number of checks that are degraded
	Degraded int `json:"degraded"`
	// Unknown is the number of checks with unknown status
	Unknown int `json:"unknown"`
}

// runChecks executes all checks of a specific type.
func (h *HealthChecker) runChecks(ctx context.Context, checkType CheckType) OverallResult {
	start := time.Now()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]CheckResult)
	summary := Summary{}
	overallStatus := StatusUp

	// Execute checks for each component
	for component, checksByType := range h.checks {
		if checks, exists := checksByType[checkType]; exists {
			for _, check := range checks {
				result := h.executeCheck(timeoutCtx, check)
				results[fmt.Sprintf("%s.%s", component, check.Name())] = result

				summary.Total++
				switch result.Status {
				case StatusUp:
					summary.Up++
				case StatusDown:
					summary.Down++
					overallStatus = StatusDown // Any down check fails overall
				case StatusDegraded:
					summary.Degraded++
					if overallStatus == StatusUp {
						overallStatus = StatusDegraded
					}
				case StatusUnknown:
					summary.Unknown++
					if overallStatus == StatusUp {
						overallStatus = StatusUnknown
					}
				}
			}
		}
	}

	// If no checks were run, status is unknown
	if summary.Total == 0 {
		overallStatus = StatusUnknown
	}

	// Calculate total duration and ensure it's not zero for timer precision issues
	duration := time.Since(start)
	if duration == 0 {
		duration = 1 * time.Nanosecond
	}

	return OverallResult{
		Status:    overallStatus,
		CheckType: checkType,
		Results:   results,
		Summary:   summary,
		Timestamp: start,
		Duration:  duration,
	}
}

// executeCheck runs a single health check with proper error handling.
func (h *HealthChecker) executeCheck(ctx context.Context, check HealthCheck) CheckResult {
	start := time.Now()

	// Recover from panics in health checks
	defer func() {
		if r := recover(); r != nil {
			// TODO: Return a failed CheckResult instead of printing
			// This requires changing the function signature to return an error as well
			// For now, we just log the panic
			fmt.Printf("Health check '%s' for component '%s' panicked: %v\n", check.Name(), check.Component(), r)
		}
	}()

	// Execute the check
	result := check.Check(ctx)

	// Calculate the duration of this check
	duration := time.Since(start)

	// Use the check's own duration if it set one and is greater, otherwise use our calculated duration
	// Ensure there's always at least some measurable duration to avoid timer precision issues
	if result.Duration == 0 || duration > result.Duration {
		result.Duration = duration
	}

	// Ensure minimum duration to avoid Windows timer precision issues in tests
	if result.Duration == 0 {
		result.Duration = 1 * time.Nanosecond
	}
	result.Timestamp = start

	return result
}

// GetComponentStatus returns the current status for a specific component.
func (h *HealthChecker) GetComponentStatus(ctx context.Context, component string) map[CheckType]CheckResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[CheckType]CheckResult)

	if checksByType, exists := h.checks[component]; exists {
		for checkType, checks := range checksByType {
			if len(checks) > 0 {
				// For simplicity, use the first check result
				// In practice, you might want to aggregate multiple checks
				result := h.executeCheck(ctx, checks[0])
				results[checkType] = result
			}
		}
	}

	return results
}

// ListComponents returns a list of all registered components.
func (h *HealthChecker) ListComponents() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	components := make([]string, 0, len(h.checks))
	for component := range h.checks {
		components = append(components, component)
	}

	return components
}
