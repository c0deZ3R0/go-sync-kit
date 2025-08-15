# Example 7: Structured Logging Demo

This example demonstrates Go Sync Kit's comprehensive structured logging capabilities using Go's built-in `slog` package.

## What You'll Learn

- **Structured Logging**: Using `slog` for consistent, machine-readable logs
- **Component-based Logging**: Creating child loggers for different components
- **Operation Tracking**: Logging operations with automatic duration tracking
- **Error Context**: Rich error logging with metadata
- **Dynamic Configuration**: Runtime log level adjustments
- **Context-aware Logging**: Correlation IDs and trace information

## Features Demonstrated

### Basic Structured Logging
- Application lifecycle events with metadata
- Grouped attributes for organized data
- Different log levels (Debug, Info, Warn, Error, Trace)

### Component Loggers
- Database connection logging
- Service-specific child loggers
- Component isolation

### Operation Logging
- Automatic duration tracking
- Success/failure logging
- Performance monitoring

### Error Handling
- Custom error types with rich metadata
- Structured error information
- Retry and troubleshooting context

### Context Integration
- Request ID correlation
- Trace ID propagation
- User session tracking

## Running the Example

```bash
# Basic execution
go run main.go

# With debug logging enabled
SYNC_LOG_LEVEL=debug go run main.go

# With dynamic level changes
DEMO_DYNAMIC_LEVEL=true go run main.go

# Different output formats
SYNC_LOG_FORMAT=json go run main.go
```

## Sample Output

The example produces structured logs like:

```
time=2025-08-16T09:32:00.000Z level=INFO msg="Application starting" version=1.0.0 environment=development start_time=2025-08-16T09:32:00.000Z

time=2025-08-16T09:32:00.001Z level=INFO msg="Database connection established" component=database host=localhost port=5432 database=sync_kit

time=2025-08-16T09:32:00.002Z level=INFO msg="HTTP request completed" request.method=POST request.path="/api/sync" request.user_id=user123 response.status=200 response.latency=150ms response.body_size=2048 metrics.db_queries=3 metrics.db_time=45ms
```

## Key Concepts

### Environment Configuration
The logging system automatically configures based on environment variables:
- `SYNC_LOG_LEVEL`: Set log level (debug, info, warn, error)
- `SYNC_LOG_FORMAT`: Set output format (text, json)
- `SYNC_LOG_ENVIRONMENT`: Set environment context

### Component Isolation
Each component gets its own logger with appropriate context:
```go
dbLogger := logging.WithComponent(logging.Component("database"))
authLogger := logging.WithComponent(logging.Component("auth-service"))
```

### Operation Tracking
Automatic timing and success tracking:
```go
err := logging.LogOperation(ctx, 
    logging.Operation("user-authentication"), 
    logging.Component("auth-service"),
    func() error {
        // Your operation here
        return performAuth()
    },
)
```

### Rich Error Context
Structured error information with metadata:
```go
syncErr := &errors.SyncError{
    Op:        errors.OpStore,
    Component: "sqlite",
    Code:      errors.ErrCodeStorageFailure,
    Metadata: map[string]interface{}{
        "disk_usage": "95%",
        "required_space": "100MB",
    },
}
logging.LogError(ctx, syncErr, "Storage operation failed")
```

This example shows how to implement production-ready logging in your Go Sync Kit applications, providing excellent observability and debugging capabilities.
