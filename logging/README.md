# Structured Logging with Go's log/slog

This package provides structured logging capabilities using Go's `log/slog` package, following best practices from the Better Stack Community Guide.

## Features

- ✅ **Structured logging with levels** (DEBUG, INFO, WARN, ERROR, plus custom TRACE/FATAL)
- ✅ **Strongly-typed attributes** using `slog.String()`, `slog.Int()`, etc.
- ✅ **Child loggers** for consistent contextual information
- ✅ **LogValuer interface** implementations for custom types
- ✅ **Stack traces** for error logging with source information
- ✅ **Environment-based configuration** (dev/prod/test)
- ✅ **Dynamic log level control** at runtime
- ✅ **Grouped contextual attributes** for structured data
- ✅ **Operation logging** with duration tracking
- ✅ **Context extraction** for request/trace IDs

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log/slog"
    "github.com/c0deZ3R0/go-sync-kit/logging"
)

func main() {
    // Initialize with environment-based configuration
    logging.Init(logging.GetConfigFromEnv())
    
    // Basic logging
    logging.Info("Application started",
        slog.String("version", "1.0.0"),
        slog.Int("port", 8080),
    )
    
    // Contextual logging
    ctx := context.Background()
    logging.InfoContext(ctx, "Processing request",
        slog.String("user_id", "123"),
        slog.Duration("timeout", time.Second*30),
    )
}
```

### Child Loggers

```go
// Create component-specific loggers
serverLogger := logging.WithComponent(logging.Component("http-server"))
dbLogger := logging.WithComponent(logging.Component("database"))

serverLogger.Info("Server starting", slog.String("address", ":8080"))
dbLogger.Info("Connected to database", slog.String("host", "localhost"))
```

### Error Logging with Stack Traces

```go
err := fmt.Errorf("database connection failed")
logging.LogError(ctx, err, "Failed to initialize database",
    slog.String("host", "localhost"),
    slog.Int("port", 5432),
)
```

### Operation Logging

```go
err := logging.LogOperation(ctx, 
    logging.Operation("user-authentication"), 
    logging.Component("auth-service"),
    func() error {
        // Your operation logic here
        return authenticateUser(userID)
    },
)
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (trace, debug, info, warn, error, fatal) | `info` |
| `LOG_FORMAT` | Output format (text, json) | `json` |
| `LOG_ADD_SOURCE` | Include source file information (true, false) | `true` |
| `ENVIRONMENT` | Environment (development, production, test) | `dev` |

### Configuration Examples

```bash
# Development
export ENVIRONMENT=development
export LOG_LEVEL=debug
export LOG_FORMAT=text
export LOG_ADD_SOURCE=true

# Production
export ENVIRONMENT=production
export LOG_LEVEL=info
export LOG_FORMAT=json
export LOG_ADD_SOURCE=false
```

### Programmatic Configuration

```go
config := logging.Config{
    Level:       "debug",
    Format:      "json",
    AddSource:   true,
    Environment: "production",
}

logging.Init(config)
```

## Advanced Features

### Custom Log Levels

```go
// Use custom TRACE level (more verbose than DEBUG)
logging.Trace("Detailed debugging information",
    slog.Any("request", detailedRequest),
)

// Use FATAL level (exits program)
logging.Fatal("Critical system error",
    slog.String("component", "core"),
)
```

### Dynamic Level Changes

```go
logger, levelVar := logging.NewLoggerWithDynamicLevel(config)

// Change level at runtime
levelVar.SetFromString("debug")  // Enable debug logging
levelVar.SetFromString("error")  // Only show errors
```

### Structured Error Logging

The logging package automatically detects and structures SyncError types:

```go
syncErr := &errors.SyncError{
    Op:        errors.OpStore,
    Component: "sqlite",
    Code:      errors.ErrCodeStorageFailure,
    Kind:      errors.KindInternal,
    Err:       fmt.Errorf("disk full"),
    Retryable: true,
    Metadata: map[string]interface{}{
        "disk_usage": "95%",
        "table":      "events",
    },
}

logging.LogError(ctx, syncErr, "Storage operation failed")
```

Output (JSON format):
```json
{
  "time": "2023-12-07T10:30:00.000Z",
  "level": "ERROR",
  "msg": "Storage operation failed",
  "sync_error": {
    "operation": "store",
    "component": "sqlite",
    "code": "STORAGE_FAILURE",
    "kind": "INTERNAL",
    "retryable": true,
    "error": "disk full",
    "metadata": {
      "disk_usage": "95%",
      "table": "events"
    }
  },
  "caller": {
    "file": "/path/to/file.go",
    "line": 42,
    "function": "main.handleRequest"
  }
}
```

### Grouped Attributes

```go
logging.Info("HTTP request completed",
    slog.Group("request",
        slog.String("method", "POST"),
        slog.String("path", "/api/users"),
        slog.Int("status", 201),
    ),
    slog.Group("timing",
        slog.Duration("total", 150*time.Millisecond),
        slog.Duration("db_query", 50*time.Millisecond),
    ),
)
```

### Context Integration

```go
// Extract request/trace IDs from context
ctx = context.WithValue(ctx, "request_id", "req-123")
ctx = context.WithValue(ctx, "trace_id", "trace-456")

contextLogger := logging.WithContext(ctx)
contextLogger.Info("Processing with context")
// Automatically includes request_id and trace_id
```

## Best Practices

### 1. Use Strongly-Typed Attributes

```go
// ✅ Good - strongly typed
logging.Info("User created",
    slog.String("user_id", userID),
    slog.Int("age", user.Age),
    slog.Bool("verified", user.Verified),
)

// ❌ Avoid - loosely typed
logging.Info("User created", "user_id", userID, "age", user.Age)
```

### 2. Create Component Loggers

```go
// ✅ Good - component-specific logger
dbLogger := logging.WithComponent(logging.Component("database"))
dbLogger.Info("Connection established")

// ✅ Also good - operation-specific logger
syncLogger := logging.WithOperation(logging.Operation("data-sync"))
```

### 3. Use Structured Error Context

```go
// ✅ Good - structured error information
logging.LogError(ctx, err, "Failed to process payment",
    slog.String("payment_id", paymentID),
    slog.String("user_id", userID),
    slog.Float64("amount", amount),
)
```

### 4. Group Related Attributes

```go
// ✅ Good - grouped attributes
logging.Info("API call completed",
    slog.Group("request",
        slog.String("method", method),
        slog.String("endpoint", endpoint),
    ),
    slog.Group("response",
        slog.Int("status", status),
        slog.Duration("latency", latency),
    ),
)
```

## Performance Considerations

- The logging package uses slog's efficient attribute handling
- Structured logging has minimal performance overhead
- Child loggers reuse underlying logger instances
- JSON format is optimized for production log processing
- Source information (`AddSource: true`) has small performance cost

## Integration with Existing Code

To migrate from standard `log` package:

```go
// Before
log.Printf("User %s logged in", userID)

// After
logging.Info("User logged in", slog.String("user_id", userID))
```

To migrate from other logging libraries:

```go
// Before (logrus example)
logrus.WithFields(logrus.Fields{
    "user_id": userID,
    "action":  "login",
}).Info("User action")

// After
logging.Info("User action",
    slog.String("user_id", userID),
    slog.String("action", "login"),
)
```

## Testing

The package includes comprehensive tests:

```bash
go test ./logging -v
go test ./logging -bench=.  # Run benchmarks
```

## Example Output

### Development (Text Format)
```
time=2023-12-07T10:30:00.000Z level=INFO msg="HTTP server starting" component=server address=:8080 endpoints./.="Dashboard UI" endpoints./sync="Sync endpoint"
```

### Production (JSON Format)
```json
{
  "time": "2023-12-07T10:30:00.000Z",
  "level": "INFO",
  "msg": "HTTP server starting",
  "component": "server",
  "address": ":8080",
  "endpoints": {
    "/": "Dashboard UI",
    "/sync": "Sync endpoint",
    "/metrics": "Metrics endpoint",
    "/health": "Health check"
  }
}
```

This structured logging implementation provides a solid foundation for observability and debugging in production systems while maintaining excellent performance and developer experience.
