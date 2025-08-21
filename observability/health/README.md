# Health Checking System

The health checking system provides comprehensive health monitoring capabilities for go-sync-kit applications. It implements Kubernetes-style health probes (liveness, readiness, startup) and provides component-level health monitoring.

## Overview

The health checking system consists of several key components:

- **HealthChecker**: Central coordinator for managing and executing health checks
- **HealthCheck Interface**: Standard interface for implementing custom health checks
- **Built-in Checks**: Ready-to-use health checks for common scenarios
- **HTTP Handler**: HTTP endpoints for health check integration
- **Sync-Kit Integration**: Specific health checks for sync-kit components

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "github.com/yourusername/go-sync-kit/observability/health"
    "github.com/yourusername/go-sync-kit/synckit"
)

func main() {
    // Create health checker
    checker := health.NewHealthChecker(health.DefaultConfig())
    
    // Create sync manager with health checking
    manager, err := synckit.NewManager(
        synckit.WithStore(store),
        synckit.WithTransport(transport),
        synckit.WithHealthChecker(checker),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Add health checks
    syncCheck := health.NewSyncManagerCheck("sync_manager", manager)
    checker.AddCheck(health.CheckTypeLiveness, syncCheck)
    
    // Test health status
    ctx := context.Background()
    result := checker.CheckLiveness(ctx)
    fmt.Printf("Health Status: %s\n", result.Status)
}
```

### HTTP Integration

```go
// Create HTTP handler for health endpoints
handler := health.NewHTTPHandler(checker, 30*time.Second)

// Register routes
mux := http.NewServeMux()
handler.RegisterRoutes(mux)

// Available endpoints:
// GET /health       - Complete health status
// GET /health/live  - Liveness probe
// GET /health/ready - Readiness probe  
// GET /health/startup - Startup probe
// GET /health/components - List components

http.ListenAndServe(":8080", mux)
```

## Health Check Types

The system supports three types of health checks following Kubernetes conventions:

### Liveness Checks
Determine if the application is alive and should continue running. Failed liveness checks typically indicate the application should be restarted.

```go
checker.AddCheck(health.CheckTypeLiveness, healthCheck)
result := checker.CheckLiveness(ctx)
```

### Readiness Checks
Determine if the application is ready to serve traffic. Failed readiness checks indicate the application should not receive traffic but doesn't need to be restarted.

```go
checker.AddCheck(health.CheckTypeReadiness, healthCheck)
result := checker.CheckReadiness(ctx)
```

### Startup Checks
Determine if the application has finished starting up. Used to protect slow-starting containers with a separate check during startup.

```go
checker.AddCheck(health.CheckTypeStartup, healthCheck)
result := checker.CheckStartup(ctx)
```

## Built-in Health Checks

### Database Check
Tests database connectivity and optionally connection pool health:

```go
check := health.NewDatabaseCheck("postgres", db, 
    health.WithDatabaseQuery("SELECT 1"),
    health.WithDatabaseTimeout(5*time.Second),
    health.WithConnectionPoolCheck(),
)
checker.AddCheck(health.CheckTypeLiveness, check)
```

### HTTP Check
Tests HTTP endpoint availability:

```go
check := health.NewHTTPCheck("api", "https://api.example.com/health",
    health.WithHTTPMethod("GET"),
    health.WithHTTPTimeout(10*time.Second),
    health.WithExpectedStatus(http.StatusOK),
)
checker.AddCheck(health.CheckTypeReadiness, check)
```

### TCP Check
Tests TCP connectivity:

```go
check := health.NewTCPCheck("redis", "localhost:6379",
    health.WithTCPTimeout(5*time.Second),
)
checker.AddCheck(health.CheckTypeLiveness, check)
```

### Memory Check
Monitors memory usage:

```go
check := health.NewMemoryCheck("memory", 500) // 500 MB threshold
checker.AddCheck(health.CheckTypeLiveness, check)
```

### Composite Check
Combines multiple checks into one:

```go
check := health.NewCompositeCheck("database", "database",
    tcpCheck,
    sqlCheck,
    poolCheck,
)
checker.AddCheck(health.CheckTypeReadiness, check)
```

## Sync-Kit Specific Checks

### SyncManager Check
Tests SyncManager health and availability:

```go
check := health.NewSyncManagerCheck("sync_manager", manager)
checker.AddCheck(health.CheckTypeLiveness, check)
```

### Storage Check
Tests storage backend health with actual I/O operations:

```go
check := health.NewStorageCheck("storage", storage,
    health.WithStorageTestKey("health_check_test"),
)
checker.AddCheck(health.CheckTypeLiveness, check)
```

### Transport Check
Tests transport layer health and connectivity:

```go
check := health.NewTransportCheck("transport", transport,
    health.WithTransportTestPeer("peer1"),
)
checker.AddCheck(health.CheckTypeReadiness, check)
```

### Network Connectivity Check
Tests connectivity to sync peers:

```go
peers := []string{"peer1:8080", "peer2:8080"}
check := health.NewNetworkConnectivityCheck("network", peers,
    health.WithNetworkTimeout(5*time.Second),
)
checker.AddCheck(health.CheckTypeReadiness, check)
```

## Custom Health Checks

Implement the `HealthCheck` interface to create custom checks:

```go
type CustomCheck struct {
    name string
    // your fields
}

func (c *CustomCheck) Name() string {
    return c.name
}

func (c *CustomCheck) Component() string {
    return "custom_component"
}

func (c *CustomCheck) Check(ctx context.Context) health.CheckResult {
    start := time.Now()
    
    // Perform your health check logic here
    
    return health.CheckResult{
        Status:    health.StatusUp,
        Component: c.Component(),
        Message:   "Custom check passed",
        Details:   map[string]interface{}{
            "custom_metric": 42,
        },
        Timestamp: start,
        Duration:  time.Since(start),
    }
}

// Register the custom check
checker.AddCheck(health.CheckTypeLiveness, &CustomCheck{name: "my_check"})
```

## Configuration

Configure the health checker behavior:

```go
config := health.Config{
    Timeout:          30 * time.Second,  // Max time for all checks
    CheckInterval:    30 * time.Second,  // Periodic check interval
    FailureThreshold: 3,                 // Failures before marking down
    SuccessThreshold: 1,                 // Successes before marking up
}

checker := health.NewHealthChecker(config)
```

## Health Status Types

- **StatusUp**: Component is healthy and fully functional
- **StatusDown**: Component is unhealthy and not functional  
- **StatusDegraded**: Component is partially functional
- **StatusUnknown**: Component status cannot be determined

## HTTP Response Codes

The HTTP handler maps health status to appropriate HTTP status codes:

- **StatusUp**: 200 OK
- **StatusDegraded**: 200 OK (still serving but degraded)
- **StatusDown**: 503 Service Unavailable
- **StatusUnknown**: 503 Service Unavailable

## Integration with Kubernetes

The health endpoints are designed to work seamlessly with Kubernetes health checks:

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:latest
    ports:
    - containerPort: 8080
    livenessProbe:
      httpGet:
        path: /health/live
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /health/ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
    startupProbe:
      httpGet:
        path: /health/startup
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
      failureThreshold: 30
```

## Best Practices

1. **Use appropriate check types**: Liveness for critical failures, readiness for temporary issues
2. **Set reasonable timeouts**: Avoid checks that take too long and impact performance
3. **Monitor dependencies**: Include checks for databases, external APIs, etc.
4. **Test your checks**: Ensure health checks accurately reflect application health
5. **Use composite checks**: Group related checks together for better organization
6. **Handle errors gracefully**: Health checks should not panic or cause service disruption

## Examples

See `examples/health_check_example.go` for a complete working example demonstrating:
- Health checker setup
- Sync-kit integration
- Multiple check types
- HTTP server with health endpoints
- Component status monitoring

## Metrics Integration

Health check results can be integrated with the metrics system for monitoring:

```go
// Health check status can be exposed as Prometheus metrics
// This integration is handled automatically when using both systems
```
