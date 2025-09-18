package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"go.opentelemetry.io/otel/trace"
)

// Config defines the RabbitMQ transport configuration.
type Config struct {
	// Connection
	URL            string
	ConnectionName string
	Heartbeat      time.Duration

	// Exchange & Routing
	Exchange     string
	ExchangeType string // topic, direct, fanout, headers
	RoutingKey   func(synckit.Event) string
	BindingKeys  []string // for consumer bindings

	// Queue (consumer)
	QueueName       string
	QueueDurable    bool
	QueueAutoDelete bool
	QueueExclusive  bool

	// Message options
	MessagePersistent bool
	MessageTTL        time.Duration
	Priority          uint8

	// Reliability
	ConfirmMode     bool
	PrefetchCount   int
	DeadLetterQueue string

	// Observability
	Tracer  SyncKitTracer
	Metrics MetricsCollector
	Logger  *slog.Logger
}

// DefaultConfig returns a RabbitMQ config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		URL:               "amqp://guest:guest@localhost:5672/",
		ConnectionName:    "go-sync-kit",
		Heartbeat:         60 * time.Second,
		Exchange:          "go-sync-kit.events",
		ExchangeType:      "topic",
		QueueDurable:      true,
		QueueAutoDelete:   false,
		QueueExclusive:    false,
		MessagePersistent: true,
		Priority:          0,
		ConfirmMode:       false,
		PrefetchCount:     10,
	}
}

// Validate checks that the config is valid and applies defaults where needed.
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("URL is required")
	}

	if c.Exchange == "" {
		return fmt.Errorf("Exchange name is required")
	}

	if c.ExchangeType == "" {
		c.ExchangeType = "topic" // Default to topic
	}

	// Validate exchange type
	validTypes := []string{"topic", "direct", "fanout", "headers"}
	valid := false
	for _, validType := range validTypes {
		if c.ExchangeType == validType {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid exchange type %s, must be one of: %s",
			c.ExchangeType, strings.Join(validTypes, ", "))
	}

	if c.ConnectionName == "" {
		c.ConnectionName = "go-sync-kit"
	}

	if c.Heartbeat <= 0 {
		c.Heartbeat = 60 * time.Second
	}

	if c.PrefetchCount < 0 {
		return fmt.Errorf("PrefetchCount cannot be negative")
	}
	if c.PrefetchCount == 0 {
		c.PrefetchCount = 10 // Default prefetch
	}

	// Priority is uint8, so Go's type system ensures it's <= 255

	// If QueueName is set for consumer, ensure we have binding keys
	if c.QueueName != "" && len(c.BindingKeys) == 0 {
		// Default binding key for topic exchange
		if c.ExchangeType == "topic" {
			c.BindingKeys = []string{"#"} // Bind to all messages
		} else if c.ExchangeType == "direct" {
			c.BindingKeys = []string{""} // Default routing key
		} else if c.ExchangeType == "fanout" {
			c.BindingKeys = []string{""} // Fanout ignores routing key
		}
	}

	return nil
}

// DefaultRoutingKey is a simple routing key function that uses event type.
func DefaultRoutingKey(event synckit.Event) string {
	return fmt.Sprintf("events.%s", event.Type())
}

// SyncKitTracer interface matches the tracer expected by SyncOptions.
// This allows RabbitMQ transport to work with the existing tracing infrastructure.
type SyncKitTracer interface {
	// StartTransportOperation starts a new span for transport operations
	StartTransportOperation(ctx context.Context, operation, transport string) (context.Context, trace.Span)
	// RecordError records an error on a span
	RecordError(span trace.Span, err error, description string)
}

// MetricsCollector interface matches the metrics expected by SyncOptions.
// This allows RabbitMQ transport to work with the existing metrics infrastructure.
type MetricsCollector interface {
	// RecordSyncDuration records how long a transport operation took
	RecordSyncDuration(operation string, duration time.Duration)
	// RecordSyncErrors records transport operation errors by type
	RecordSyncErrors(operation string, errorType string)
}
