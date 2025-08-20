# Observability Package

The observability package provides comprehensive monitoring, tracing, and health checking capabilities for go-sync-kit. This package implements Phase 3 enterprise features including OpenTelemetry distributed tracing, Prometheus metrics, and health checks.

## Features

### 🔍 Distributed Tracing

OpenTelemetry-based distributed tracing for sync operations:

- **Automatic span creation** for sync, transport, storage, and conflict resolution operations
- **Context propagation** across HTTP requests and gRPC calls  
- **Structured attributes** following OpenTelemetry semantic conventions
- **Error recording** with detailed error information and stack traces
- **HTTP middleware** for automatic request tracing
- **HTTP client transport** for outgoing request tracing

### 📊 Metrics (Coming Soon)

Prometheus-compatible metrics collection:

- Sync operation counters and histograms
- Transport layer metrics
- Storage operation metrics  
- Conflict resolution metrics
- Custom business metrics

### 🏥 Health Checks (Coming Soon)

Health check endpoints and monitoring:

- Liveness, readiness, and startup probes
- Component-level health checks
- Dependency health monitoring
- Custom health check integration

## Quick Start

### Basic Tracing Setup

```go
import (
    "github.com/c0deZ3R0/go-sync-kit/observability/tracing"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Create a tracer
tracer := tracing.NewTracer("my-sync-service",
    tracing.WithVersion("v1.0.0"),
)

// Create sync manager with tracing
manager, err := synckit.NewBuilder().
    WithTracer(tracer).
    Build()
```

### HTTP Middleware

```go
import (
    "net/http"
    "github.com/c0deZ3R0/go-sync-kit/observability/tracing"
)

// Create HTTP middleware
middleware := tracing.NewHTTPMiddleware("my-service")

// Apply to your HTTP handlers  
http.Handle("/api/sync", middleware.Handler(syncHandler))
```

### HTTP Client Tracing

```go
import (
    "net/http"
    "github.com/c0deZ3R0/go-sync-kit/observability/tracing"
)

// Create traced HTTP client
client := &http.Client{
    Transport: tracing.NewClientTransport(http.DefaultTransport),
}

// Use client for outgoing requests - spans will be created automatically
resp, err := client.Do(req.WithContext(ctx))
```

## Tracing Components

### SyncKitTracer

The core tracer provides methods for creating spans for different operation types:

```go
// Start a sync operation span
ctx, span := tracer.StartSyncOperation(ctx, "full_sync")
defer span.End()

// Start a transport operation span
ctx, span := tracer.StartTransportOperation(ctx, "push", "http")
defer span.End()

// Start a storage operation span  
ctx, span := tracer.StartStorageOperation(ctx, "store", "sqlite")
defer span.End()

// Start conflict resolution span
ctx, span := tracer.StartConflictResolution(ctx, "last_write_wins")  
defer span.End()
```

### Span Attributes

The tracing package defines semantic attributes for consistent span tagging:

```go
// Add event attributes
tracer.AddEventAttributes(span, eventCount, aggregateIDs)

// Record sync results
tracer.SetSyncResult(span, eventsPushed, eventsPulled, conflictsResolved)

// Record errors
tracer.RecordError(span, err, "Failed to sync data")

// Add custom events
tracer.AddSpanEvent(span, "data_validation_complete",
    attribute.String("validation_result", "success"),
    attribute.Int("validated_records", 100),
)
```

### Attribute Keys

The package provides strongly-typed attribute keys following OpenTelemetry conventions:

- **Sync attributes**: `synckit.sync.operation`, `synckit.sync.phase`, `synckit.events.count`
- **Transport attributes**: `synckit.transport.type`, `synckit.transport.operation`  
- **Storage attributes**: `synckit.storage.type`, `synckit.storage.operation`
- **Conflict attributes**: `synckit.conflict.strategy`, `synckit.conflicts.resolved`
- **Component attributes**: `synckit.component`, `synckit.health.status`

## Integration with OpenTelemetry

### Tracer Provider Setup

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

// Set up Jaeger exporter
exporter, err := jaeger.New(jaeger.WithCollectorEndpoint())
if err != nil {
    log.Fatal(err)
}

// Create tracer provider
tp := trace.NewTracerProvider(
    trace.WithBatcher(exporter),
    trace.WithResource(resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String("sync-service"),
        semconv.ServiceVersionKey.String("v1.0.0"),
    )),
)

// Set global tracer provider
otel.SetTracerProvider(tp)
```

### Context Propagation

The package automatically handles context propagation for HTTP requests using OpenTelemetry propagators:

```go
// Incoming requests extract trace context from headers
// Outgoing requests inject trace context into headers
```

## Configuration

### Tracer Options

```go
tracer := tracing.NewTracer("my-service",
    tracing.WithVersion("v1.2.3"),
    tracing.WithAttributes(
        attribute.String("environment", "production"),
        attribute.String("region", "us-east-1"),
    ),
)
```

### HTTP Middleware Configuration

```go
config := tracing.TraceConfig{
    ServiceName:     "sync-api",
    SkipHealthCheck: true,
    SkipUserAgent:   []string{"kube-probe"},
}

middleware := tracing.NewHTTPMiddlewareWithConfig(config)
```

## Testing

The package includes comprehensive tests for all tracing components:

```bash
# Run all tracing tests
go test ./observability/tracing/... -v

# Run specific test suites
go test ./observability/tracing -run TestTracer -v
go test ./observability/tracing -run TestHTTPMiddleware -v
go test ./observability/tracing -run TestAttributes -v
```

## Architecture

### Package Structure

```
observability/
├── README.md              # This file
├── tracing/               # Distributed tracing
│   ├── tracer.go         # Core tracer implementation  
│   ├── attributes.go     # Semantic attribute definitions
│   ├── middleware.go     # HTTP tracing middleware
│   ├── tracer_test.go    # Tracer tests
│   ├── attributes_test.go # Attribute tests
│   └── middleware_test.go # Middleware tests
├── metrics/              # Prometheus metrics (planned)
└── health/               # Health checks (planned)
```

### Design Principles

1. **Standards Compliance**: Follows OpenTelemetry semantic conventions
2. **Performance**: Minimal overhead with efficient span creation
3. **Flexibility**: Configurable via functional options
4. **Compatibility**: Works with existing OpenTelemetry infrastructure  
5. **Testing**: Comprehensive test coverage for reliability

## Examples

### Complete Integration Example

```go
package main

import (
    "context"
    "log"
    "net/http"
    
    "github.com/c0deZ3R0/go-sync-kit/observability/tracing"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
    // Set up OpenTelemetry
    setupTracing()
    
    // Create tracer
    tracer := tracing.NewTracer("sync-service")
    
    // Create sync manager with tracing
    manager, err := synckit.NewBuilder().
        WithTracer(tracer).
        Build()
    if err != nil {
        log.Fatal(err)
    }
    
    // Set up HTTP server with tracing middleware
    middleware := tracing.NewHTTPMiddleware("sync-api")
    
    http.Handle("/sync", middleware.Handler(http.HandlerFunc(
        func(w http.ResponseWriter, r *http.Request) {
            // Sync operation will be automatically traced
            result, err := manager.Sync(r.Context(), syncOptions)
            if err != nil {
                http.Error(w, err.Error(), 500)
                return
            }
            
            // Return results
            writeJSON(w, result)
        },
    )))
    
    log.Println("Starting server on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupTracing() {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint())
    if err != nil {
        log.Fatal(err)
    }
    
    tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
    otel.SetTracerProvider(tp)
}
```

## Contributing

When adding new tracing functionality:

1. Follow OpenTelemetry semantic conventions
2. Add appropriate tests for new components
3. Update attribute definitions in `attributes.go`  
4. Document new features in this README
5. Ensure backward compatibility

## License

This package is part of the go-sync-kit project and follows the same license terms.
