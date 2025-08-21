# RabbitMQ Transport

A production-ready RabbitMQ transport implementation for go-sync-kit that provides reliable messaging with enterprise-grade features.

## Features

### ✅ Implemented (Phase 1)
- **Connection Management**: Robust connection/channel lifecycle with proper cleanup
- **Topology Setup**: Idempotent exchange/queue declaration with configurable bindings
- **Push Operations**: JSON serialization with persistent/transient delivery modes
- **Subscribe Operations**: Manual-ack consumer with error handling and requeuing
- **Configuration**: Comprehensive config with validation and sensible defaults
- **Thread Safety**: Concurrent operations with RWMutex protection
- **Observability**: Structured logging, OpenTelemetry tracing, and Prometheus metrics

### 🚧 Planned (Phase 2)
- Dead letter queues (DLQ) and retry policies
- Publisher confirms with correlation tracking
- Priority queues and message TTL
- Back-pressure handling and load balancing

### 🔮 Future (Phase 3)
- Multi-tenant routing patterns
- Conflict-resolution queues
- Event replay and audit trails
- Hybrid transport guidance

## Quick Start

### Basic Publisher

```go
package main

import (
    "context"
    "log"
    "log/slog"

    "github.com/c0deZ3R0/go-sync-kit/transport/rabbitmq"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func main() {
    // Create config with defaults
    cfg := rabbitmq.DefaultConfig()
    cfg.Logger = slog.Default()

    // Create and connect transport
    transport, err := rabbitmq.NewTransportWithValidation(cfg)
    if err != nil {
        log.Fatalf("Config validation failed: %v", err)
    }

    ctx := context.Background()
    if err := transport.Connect(ctx); err != nil {
        log.Fatalf("Connection failed: %v", err)
    }
    defer transport.Close()

    // Create mock events
    events := []synckit.EventWithVersion{
        {Event: &MyEvent{id: "1", eventType: "UserCreated"}, Version: &MyVersion{1}},
        {Event: &MyEvent{id: "2", eventType: "UserUpdated"}, Version: &MyVersion{2}},
    }

    // Push events
    if err := transport.Push(ctx, events); err != nil {
        log.Fatalf("Push failed: %v", err)
    }

    log.Println("Events published successfully!")
}
```

### Basic Consumer

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/c0deZ3R0/go-sync-kit/transport/rabbitmq"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func main() {
    // Create consumer config
    cfg := rabbitmq.DefaultConfig()
    cfg.QueueName = "go-sync-kit.consumer"
    cfg.PrefetchCount = 5
    cfg.Logger = slog.Default()

    // Create and connect transport
    transport, err := rabbitmq.NewTransportWithValidation(cfg)
    if err != nil {
        log.Fatalf("Config validation failed: %v", err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := transport.Connect(ctx); err != nil {
        log.Fatalf("Connection failed: %v", err)
    }
    defer transport.Close()

    // Subscribe to events
    err = transport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
        for _, event := range events {
            log.Printf("Received event: %s - %s", event.Event.Type(), event.Event.ID())
            // Process event here
        }
        return nil
    })
    if err != nil {
        log.Fatalf("Subscribe failed: %v", err)
    }

    log.Println("Consumer started, press Ctrl+C to exit")

    // Wait for interrupt
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c

    log.Println("Shutting down...")
}
```

### Custom Routing Keys

```go
cfg := rabbitmq.DefaultConfig()
cfg.RoutingKey = func(event synckit.Event) string {
    // Custom routing based on tenant and event type
    tenantID := event.Metadata()["tenant_id"]
    return fmt.Sprintf("tenant.%v.%s", tenantID, event.Type())
}
```

### Topic Exchange with Selective Binding

```go
cfg := rabbitmq.DefaultConfig()
cfg.QueueName = "user-service-queue"
cfg.ExchangeType = "topic"
cfg.BindingKeys = []string{
    "events.User*",      // All user events
    "events.*.high",     // All high priority events
    "tenant.acme.*",     // All events for tenant 'acme'
}
```

## Configuration

### Connection Settings

```go
cfg := &rabbitmq.Config{
    URL:            "amqp://user:pass@rabbitmq.example.com:5672/vhost",
    ConnectionName: "my-service",
    Heartbeat:      30 * time.Second,
}
```

### Exchange Configuration

```go
cfg.Exchange = "my-events"           // Exchange name
cfg.ExchangeType = "topic"           // topic, direct, fanout, headers
```

### Consumer Queue Settings

```go
cfg.QueueName = "my-consumer"        // Required for Subscribe()
cfg.QueueDurable = true              // Survive broker restarts
cfg.QueueAutoDelete = false          // Don't auto-delete when unused
cfg.QueueExclusive = false           // Allow multiple consumers
cfg.PrefetchCount = 50               // Flow control
```

### Message Options

```go
cfg.MessagePersistent = true         // Survive broker restarts
cfg.Priority = 10                    // Message priority (0-255)
cfg.ConfirmMode = true               // Publisher confirms
```

## Advanced Usage

### With go-sync-kit SyncManager

**Note**: RabbitMQ transport is optimized for Push/Subscribe operations. For Pull and GetLatestVersion operations, consider a hybrid approach with HTTP transport.

```go
// Use RabbitMQ for Push operations
rabbitTransport := rabbitmq.NewTransportWithValidation(rabbitCfg)
rabbitTransport.Connect(ctx)

// Use HTTP for Pull operations (hybrid approach)
httpTransport := httptransport.NewTransport("http://localhost:8080/sync", nil, nil, nil)

// Configure SyncManager with HTTP transport
// Use RabbitMQ transport separately for real-time subscription
manager, err := synckit.NewManager(
    synckit.WithStore(store),
    synckit.WithTransport(httpTransport),  // For Pull/Push operations
    synckit.WithLWW(),
)

// Subscribe to real-time events via RabbitMQ
go rabbitTransport.Subscribe(ctx, func(events []synckit.EventWithVersion) error {
    // Handle real-time events
    log.Printf("Received %d real-time events", len(events))
    return nil
})
```

### Error Handling

The transport uses manual acknowledgments with intelligent error handling:

- **Malformed messages**: NACK without requeue (permanent failure)
- **Handler errors**: NACK with requeue (temporary failure, retry)
- **Connection issues**: Automatic cleanup and proper error propagation

### Observability Integration

#### Logging

```go
cfg.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
```

#### OpenTelemetry Tracing

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

// Custom tracer that implements SyncKitTracer interface
type CustomTracer struct {
    tracer trace.Tracer
}

func (t *CustomTracer) StartTransportOperation(ctx context.Context, operation, transport string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, fmt.Sprintf("%s.%s", transport, operation))
}

func (t *CustomTracer) RecordError(span trace.Span, err error, description string) {
    span.RecordError(err)
    span.SetStatus(codes.Error, description)
}

// Configure transport with tracing
cfg.Tracer = &CustomTracer{
    tracer: otel.Tracer("go-sync-kit/rabbitmq"),
}
```

#### Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "time"
)

// Custom metrics collector
type CustomMetrics struct {
    duration prometheus.HistogramVec
    errors   prometheus.CounterVec
}

func (m *CustomMetrics) RecordSyncDuration(operation string, duration time.Duration) {
    m.duration.WithLabelValues(operation).Observe(duration.Seconds())
}

func (m *CustomMetrics) RecordSyncErrors(operation string, errorType string) {
    m.errors.WithLabelValues(operation, errorType).Inc()
}

// Configure transport with metrics
cfg.Metrics = &CustomMetrics{
    duration: *prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name: "rabbitmq_operation_duration_seconds",
        Help: "Duration of RabbitMQ operations",
    }, []string{"operation"}),
    errors: *prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "rabbitmq_operation_errors_total",
        Help: "Total number of RabbitMQ operation errors",
    }, []string{"operation", "error_type"}),
}
```

## Testing

### Unit Tests

```bash
# Run unit tests
go test ./transport/rabbitmq/

# With verbose output
go test -v ./transport/rabbitmq/
```

### Integration Tests with Docker

The package includes comprehensive Docker Compose setup for integration testing:

#### Linux/macOS

```bash
# Run all integration tests (auto-start/stop RabbitMQ)
./transport/rabbitmq/test-integration.sh test

# Start RabbitMQ for manual testing
./transport/rabbitmq/test-integration.sh start

# Run tests against existing RabbitMQ
./transport/rabbitmq/test-integration.sh test-local

# View RabbitMQ logs
./transport/rabbitmq/test-integration.sh logs

# Clean up everything
./transport/rabbitmq/test-integration.sh cleanup
```

#### Windows PowerShell

```powershell
# Run all integration tests
.\transport\rabbitmq\test-integration.ps1 test

# Start RabbitMQ for manual testing
.\transport\rabbitmq\test-integration.ps1 start

# Run tests against existing RabbitMQ
.\transport\rabbitmq\test-integration.ps1 test-local

# View logs and cleanup
.\transport\rabbitmq\test-integration.ps1 logs
.\transport\rabbitmq\test-integration.ps1 cleanup
```

#### Manual Docker Setup

```bash
# Start RabbitMQ with management UI
docker-compose -f transport/rabbitmq/docker-compose.test.yml up -d

# Run integration tests
RABBITMQ_URL="amqp://synckit_user:synckit_pass@localhost:5672/" go test -v -tags=integration ./transport/rabbitmq -run TestIntegration

# Clean up
docker-compose -f transport/rabbitmq/docker-compose.test.yml down -v
```

#### Integration Test Features

- **Connection lifecycle testing**
- **Push/Subscribe message flow verification**
- **Multiple publisher concurrency testing**
- **Error handling and retry behavior**
- **Handler failure simulation with requeue testing**
- **Automatic RabbitMQ health checks**
- **Management UI access** (http://localhost:15672, synckit_user/synckit_pass)

## Local Development

Start RabbitMQ with Docker:

```bash
docker run -d --name rabbitmq \
  -p 5672:5672 -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER=admin \
  -e RABBITMQ_DEFAULT_PASS=admin \
  rabbitmq:3-management
```

Access management UI at http://localhost:15672 (admin/admin)

## Roadmap

See [RABBITMQ_ROADMAP.md](../../RABBITMQ_ROADMAP.md) for detailed implementation phases and milestones.

## Design Decisions

- **No Pull Implementation**: RabbitMQ is message-oriented, not query-oriented. Use HTTP transport for Pull operations.
- **Manual Acknowledgments**: Ensures reliable processing with proper error handling.
- **JSON Serialization**: Simple, cross-platform compatibility.
- **Thread Safety**: Full concurrent operation support.
- **Hybrid Recommended**: Combine with HTTP transport for complete sync functionality.
