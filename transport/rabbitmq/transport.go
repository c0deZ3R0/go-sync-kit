package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Transport implements synckit.Transport using RabbitMQ for reliable messaging.
type Transport struct {
	cfg *Config

	// Connection state
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.RWMutex
	closed  bool

	// Consumer state
	consumerTag string
	closeCh     chan struct{}
}

// NewTransport creates a RabbitMQ transport instance.
// Call Connect() to establish the connection.
func NewTransport(cfg *Config) *Transport {
	return &Transport{
		cfg:     cfg,
		closeCh: make(chan struct{}),
	}
}

// NewTransportWithValidation creates a RabbitMQ transport with config validation.
func NewTransportWithValidation(cfg *Config) (*Transport, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return NewTransport(cfg), nil
}

// Connect establishes connection to RabbitMQ and sets up topology.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	// Connect to RabbitMQ
	conn, err := amqp.Dial(t.cfg.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	t.conn = conn

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		t.conn.Close()
		return fmt.Errorf("failed to create channel: %w", err)
	}
	t.channel = ch

	// Set up topology
	if err := t.setupTopology(); err != nil {
		t.channel.Close()
		t.conn.Close()
		return fmt.Errorf("failed to setup topology: %w", err)
	}

	t.logInfo("Connected to RabbitMQ successfully")
	return nil
}

// setupTopology declares exchanges, queues, and bindings.
func (t *Transport) setupTopology() error {
	// Declare exchange (idempotent)
	err := t.channel.ExchangeDeclare(
		t.cfg.Exchange,     // name
		t.cfg.ExchangeType, // type (topic, direct, fanout, headers)
		true,               // durable
		false,              // auto-delete
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", t.cfg.Exchange, err)
	}

	// Declare queue for consumer (if QueueName is set)
	if t.cfg.QueueName != "" {
		_, err := t.channel.QueueDeclare(
			t.cfg.QueueName,       // name
			t.cfg.QueueDurable,    // durable
			t.cfg.QueueAutoDelete, // delete when unused
			t.cfg.QueueExclusive,  // exclusive
			false,                 // no-wait
			nil,                   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", t.cfg.QueueName, err)
		}

		// Bind queue to exchange with routing keys
		for _, bindingKey := range t.cfg.BindingKeys {
			err := t.channel.QueueBind(
				t.cfg.QueueName, // queue name
				bindingKey,      // routing key
				t.cfg.Exchange,  // exchange
				false,           // no-wait
				nil,             // arguments
			)
			if err != nil {
				return fmt.Errorf("failed to bind queue %s with key %s: %w", t.cfg.QueueName, bindingKey, err)
			}
		}
	}

	// Set QoS (prefetch) for consumer
	if t.cfg.PrefetchCount > 0 {
		err := t.channel.Qos(
			t.cfg.PrefetchCount, // prefetch count
			0,                   // prefetch size (0 = no limit)
			false,               // global
		)
		if err != nil {
			return fmt.Errorf("failed to set QoS: %w", err)
		}
	}

	// Enable publisher confirms if requested
	if t.cfg.ConfirmMode {
		err := t.channel.Confirm(false)
		if err != nil {
			return fmt.Errorf("failed to enable confirm mode: %w", err)
		}
	}

	return nil
}

// Push publishes events to the RabbitMQ exchange.
func (t *Transport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.channel == nil {
		return fmt.Errorf("transport not connected")
	}

	for _, event := range events {
		// Serialize event
		body, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to serialize event %s: %w", event.Event.ID(), err)
		}

		// Determine routing key
		routingKey := ""
		if t.cfg.RoutingKey != nil {
			routingKey = t.cfg.RoutingKey(event.Event)
		}

		// Publish message
		err = t.channel.PublishWithContext(ctx,
			t.cfg.Exchange, // exchange
			routingKey,     // routing key
			false,          // mandatory
			false,          // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: t.getDeliveryMode(),
				Body:         body,
				MessageId:    event.Event.ID(),
				Timestamp:    time.Now(),
				Priority:     t.cfg.Priority,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to publish event %s: %w", event.Event.ID(), err)
		}
	}

	t.logInfo("Published %d events successfully", len(events))
	return nil
}

// Pull is not implemented for RabbitMQ (use Subscribe for async consumption).
// Consider hybrid approach: HTTP for Pull, RabbitMQ for Subscribe/Push.
func (t *Transport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, fmt.Errorf("rabbitmq transport Pull not implemented; use Subscribe for async consumption or HTTP transport for sync Pull")
}

// GetLatestVersion is not implemented for RabbitMQ.
// Use HTTP transport or store-based approach for version queries.
func (t *Transport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return nil, fmt.Errorf("rabbitmq transport GetLatestVersion not implemented; use HTTP transport or store-based version queries")
}

// Subscribe starts consuming messages from the configured queue.
func (t *Transport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.channel == nil {
		return fmt.Errorf("transport not connected")
	}

	if t.cfg.QueueName == "" {
		return fmt.Errorf("queue name required for Subscribe")
	}

	// Start consuming
	t.consumerTag = fmt.Sprintf("go-sync-kit-%d", time.Now().Unix())
	messages, err := t.channel.Consume(
		t.cfg.QueueName, // queue
		t.consumerTag,   // consumer
		false,           // auto-ack (we want manual ack)
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	t.logInfo("Started consuming from queue %s", t.cfg.QueueName)

	// Process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				t.logInfo("Context cancelled, stopping consumer")
				return
			case <-t.closeCh:
				t.logInfo("Transport closed, stopping consumer")
				return
			case msg, ok := <-messages:
				if !ok {
					t.logInfo("Message channel closed, stopping consumer")
					return
				}

				// Deserialize event
				var event synckit.EventWithVersion
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					t.logError("Failed to deserialize message: %v", err)
					msg.Nack(false, false) // Don't requeue malformed messages
					continue
				}

				// Handle event (batch of 1 for now)
				events := []synckit.EventWithVersion{event}
				if err := handler(events); err != nil {
					t.logError("Handler failed for message %s: %v", msg.MessageId, err)
					msg.Nack(false, true) // Requeue on handler error
					continue
				}

				// Acknowledge successful processing
				msg.Ack(false)
			}
		}
	}()

	return nil
}

// Close closes the RabbitMQ connection and stops consumers.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	close(t.closeCh)

	// Cancel consumer if active
	if t.channel != nil && t.consumerTag != "" {
		t.channel.Cancel(t.consumerTag, false)
	}

	// Close channel and connection
	if t.channel != nil {
		t.channel.Close()
	}
	if t.conn != nil {
		t.conn.Close()
	}

	t.logInfo("RabbitMQ transport closed")
	return nil
}

// getDeliveryMode returns the AMQP delivery mode based on config.
func (t *Transport) getDeliveryMode() uint8 {
	if t.cfg.MessagePersistent {
		return amqp.Persistent
	}
	return amqp.Transient
}

// logInfo logs info messages if logger is configured.
func (t *Transport) logInfo(format string, args ...interface{}) {
	if t.cfg.Logger != nil {
		t.cfg.Logger.Info(fmt.Sprintf("[RabbitMQ] "+format, args...))
	}
}

// logError logs error messages if logger is configured.
func (t *Transport) logError(format string, args ...interface{}) {
	if t.cfg.Logger != nil {
		t.cfg.Logger.Error(fmt.Sprintf("[RabbitMQ] "+format, args...))
	}
}
