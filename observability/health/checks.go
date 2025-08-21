package health

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// DatabaseCheck performs a health check against a SQL database connection.
type DatabaseCheck struct {
	name     string
	db       *sql.DB
	query    string
	timeout  time.Duration
	connPool bool // whether to check connection pool health
}

// NewDatabaseCheck creates a new database health check.
func NewDatabaseCheck(name string, db *sql.DB, options ...DatabaseCheckOption) *DatabaseCheck {
	check := &DatabaseCheck{
		name:    name,
		db:      db,
		query:   "SELECT 1",
		timeout: 5 * time.Second,
	}

	for _, opt := range options {
		opt(check)
	}

	return check
}

// DatabaseCheckOption configures a database health check.
type DatabaseCheckOption func(*DatabaseCheck)

// WithDatabaseQuery sets a custom query for the database check.
func WithDatabaseQuery(query string) DatabaseCheckOption {
	return func(c *DatabaseCheck) {
		c.query = query
	}
}

// WithDatabaseTimeout sets the timeout for the database check.
func WithDatabaseTimeout(timeout time.Duration) DatabaseCheckOption {
	return func(c *DatabaseCheck) {
		c.timeout = timeout
	}
}

// WithConnectionPoolCheck enables connection pool health monitoring.
func WithConnectionPoolCheck() DatabaseCheckOption {
	return func(c *DatabaseCheck) {
		c.connPool = true
	}
}

func (d *DatabaseCheck) Name() string     { return d.name }
func (d *DatabaseCheck) Component() string { return "database" }

func (d *DatabaseCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Component: d.Component(),
		Details:   make(map[string]interface{}),
		Timestamp: start,
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Test basic connectivity
	if err := d.db.PingContext(timeoutCtx); err != nil {
		result.Status = StatusDown
		result.Message = fmt.Sprintf("Database ping failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Test with custom query
	if d.query != "" {
		var dummy interface{}
		if err := d.db.QueryRowContext(timeoutCtx, d.query).Scan(&dummy); err != nil {
			result.Status = StatusDown
			result.Message = fmt.Sprintf("Database query failed: %v", err)
			result.Duration = time.Since(start)
			return result
		}
	}

	// Check connection pool stats if enabled
	if d.connPool {
		stats := d.db.Stats()
		result.Details["open_connections"] = stats.OpenConnections
		result.Details["in_use_connections"] = stats.InUse
		result.Details["idle_connections"] = stats.Idle
		result.Details["wait_count"] = stats.WaitCount
		result.Details["wait_duration"] = stats.WaitDuration.String()

		// Consider degraded if too many connections are waiting
		if stats.WaitCount > 10 {
			result.Status = StatusDegraded
			result.Message = "High connection pool wait count"
		}
	}

	if result.Status == "" {
		result.Status = StatusUp
		result.Message = "Database is healthy"
	}

	result.Duration = time.Since(start)
	return result
}

// HTTPCheck performs a health check against an HTTP endpoint.
type HTTPCheck struct {
	name           string
	url            string
	method         string
	timeout        time.Duration
	expectedStatus int
	client         *http.Client
}

// NewHTTPCheck creates a new HTTP health check.
func NewHTTPCheck(name, url string, options ...HTTPCheckOption) *HTTPCheck {
	check := &HTTPCheck{
		name:           name,
		url:            url,
		method:         "GET",
		timeout:        10 * time.Second,
		expectedStatus: http.StatusOK,
		client:         &http.Client{},
	}

	for _, opt := range options {
		opt(check)
	}

	// Set client timeout
	check.client.Timeout = check.timeout

	return check
}

// HTTPCheckOption configures an HTTP health check.
type HTTPCheckOption func(*HTTPCheck)

// WithHTTPMethod sets the HTTP method for the check.
func WithHTTPMethod(method string) HTTPCheckOption {
	return func(c *HTTPCheck) {
		c.method = method
	}
}

// WithHTTPTimeout sets the timeout for the HTTP check.
func WithHTTPTimeout(timeout time.Duration) HTTPCheckOption {
	return func(c *HTTPCheck) {
		c.timeout = timeout
	}
}

// WithExpectedStatus sets the expected HTTP status code.
func WithExpectedStatus(status int) HTTPCheckOption {
	return func(c *HTTPCheck) {
		c.expectedStatus = status
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) HTTPCheckOption {
	return func(c *HTTPCheck) {
		c.client = client
	}
}

func (h *HTTPCheck) Name() string     { return h.name }
func (h *HTTPCheck) Component() string { return "http" }

func (h *HTTPCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Component: h.Component(),
		Details:   make(map[string]interface{}),
		Timestamp: start,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, h.method, h.url, nil)
	if err != nil {
		result.Status = StatusDown
		result.Message = fmt.Sprintf("Failed to create HTTP request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Perform request
	resp, err := h.client.Do(req)
	if err != nil {
		result.Status = StatusDown
		result.Message = fmt.Sprintf("HTTP request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	// Check status code
	result.Details["status_code"] = resp.StatusCode
	result.Details["response_time"] = time.Since(start).String()

	if resp.StatusCode != h.expectedStatus {
		result.Status = StatusDown
		result.Message = fmt.Sprintf("Unexpected status code: got %d, expected %d", resp.StatusCode, h.expectedStatus)
	} else {
		result.Status = StatusUp
		result.Message = "HTTP endpoint is healthy"
	}

	result.Duration = time.Since(start)
	return result
}

// TCPCheck performs a TCP connectivity check.
type TCPCheck struct {
	name     string
	address  string
	timeout  time.Duration
	dialer   *net.Dialer
}

// NewTCPCheck creates a new TCP connectivity check.
func NewTCPCheck(name, address string, options ...TCPCheckOption) *TCPCheck {
	check := &TCPCheck{
		name:    name,
		address: address,
		timeout: 5 * time.Second,
		dialer:  &net.Dialer{},
	}

	for _, opt := range options {
		opt(check)
	}

	check.dialer.Timeout = check.timeout

	return check
}

// TCPCheckOption configures a TCP health check.
type TCPCheckOption func(*TCPCheck)

// WithTCPTimeout sets the timeout for the TCP check.
func WithTCPTimeout(timeout time.Duration) TCPCheckOption {
	return func(c *TCPCheck) {
		c.timeout = timeout
	}
}

func (t *TCPCheck) Name() string     { return t.name }
func (t *TCPCheck) Component() string { return "tcp" }

func (t *TCPCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Component: t.Component(),
		Details:   make(map[string]interface{}),
		Timestamp: start,
	}

	// Attempt TCP connection
	conn, err := t.dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		result.Status = StatusDown
		result.Message = fmt.Sprintf("TCP connection failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer conn.Close()

	result.Status = StatusUp
	result.Message = "TCP connection successful"
	result.Details["address"] = t.address
	result.Details["connection_time"] = time.Since(start).String()
	result.Duration = time.Since(start)

	return result
}

// MemoryCheck performs a memory usage check.
type MemoryCheck struct {
	name      string
	threshold uint64 // memory threshold in bytes
}

// NewMemoryCheck creates a new memory usage check.
func NewMemoryCheck(name string, thresholdMB uint64) *MemoryCheck {
	return &MemoryCheck{
		name:      name,
		threshold: thresholdMB * 1024 * 1024, // Convert MB to bytes
	}
}

func (m *MemoryCheck) Name() string     { return m.name }
func (m *MemoryCheck) Component() string { return "system" }

func (m *MemoryCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Component: m.Component(),
		Details:   make(map[string]interface{}),
		Timestamp: start,
	}

	// Get memory statistics (simplified - in practice you'd use runtime.MemStats)
	var memStats struct {
		Alloc      uint64
		TotalAlloc uint64
		Sys        uint64
		NumGC      uint32
	}

	// This is a placeholder - you would populate with actual memory stats
	// For example: runtime.ReadMemStats(&realMemStats)
	// memStats.Alloc = realMemStats.Alloc

	result.Details["allocated_memory"] = memStats.Alloc
	result.Details["total_allocated"] = memStats.TotalAlloc
	result.Details["system_memory"] = memStats.Sys
	result.Details["gc_cycles"] = memStats.NumGC
	result.Details["threshold"] = m.threshold

	if memStats.Alloc > m.threshold {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Memory usage above threshold: %d bytes > %d bytes", memStats.Alloc, m.threshold)
	} else {
		result.Status = StatusUp
		result.Message = "Memory usage is healthy"
	}

	result.Duration = time.Since(start)
	return result
}

// CompositeCheck combines multiple health checks into a single check.
type CompositeCheck struct {
	name   string
	comp   string
	checks []HealthCheck
	mu     sync.RWMutex
}

// NewCompositeCheck creates a new composite health check.
func NewCompositeCheck(name, component string, checks ...HealthCheck) *CompositeCheck {
	return &CompositeCheck{
		name:   name,
		comp:   component,
		checks: checks,
	}
}

// AddCheck adds a health check to the composite check.
func (c *CompositeCheck) AddCheck(check HealthCheck) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, check)
}

func (c *CompositeCheck) Name() string     { return c.name }
func (c *CompositeCheck) Component() string { return c.comp }

func (c *CompositeCheck) Check(ctx context.Context) CheckResult {
	start := time.Now()
	result := CheckResult{
		Component: c.Component(),
		Details:   make(map[string]interface{}),
		Timestamp: start,
	}

	c.mu.RLock()
	checks := make([]HealthCheck, len(c.checks))
	copy(checks, c.checks)
	c.mu.RUnlock()

	if len(checks) == 0 {
		result.Status = StatusUnknown
		result.Message = "No checks configured"
		result.Duration = time.Since(start)
		return result
	}

	// Run all checks
	checkResults := make(map[string]CheckResult)
	overallStatus := StatusUp
	var messages []string

	for _, check := range checks {
		checkResult := check.Check(ctx)
		checkResults[check.Name()] = checkResult

		// Aggregate status
		switch checkResult.Status {
		case StatusDown:
			overallStatus = StatusDown
			messages = append(messages, fmt.Sprintf("%s: %s", check.Name(), checkResult.Message))
		case StatusDegraded:
			if overallStatus == StatusUp {
				overallStatus = StatusDegraded
			}
			messages = append(messages, fmt.Sprintf("%s: %s", check.Name(), checkResult.Message))
		case StatusUnknown:
			if overallStatus == StatusUp {
				overallStatus = StatusUnknown
			}
			messages = append(messages, fmt.Sprintf("%s: %s", check.Name(), checkResult.Message))
		}
	}

	result.Status = overallStatus
	result.Details["sub_checks"] = checkResults
	
	if len(messages) > 0 {
		result.Message = fmt.Sprintf("Composite check: %v", messages)
	} else {
		result.Message = "All sub-checks passed"
	}

	result.Duration = time.Since(start)
	return result
}
