package rabbitmq

import (
	"log/slog"
	"time"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
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
	Tracer  interface{}
	Metrics interface{}
	Logger  *slog.Logger
}
