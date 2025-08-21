package rabbitmq

import (
	"fmt"
	"log/slog"
	"strings"
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

	if c.Priority > 255 {
		return fmt.Errorf("Priority cannot exceed 255")
	}

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
