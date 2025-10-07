package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	start := time.Now()
	var span trace.Span

	// Start tracing if configured
	if t.cfg.Tracer != nil {
		_, span = t.cfg.Tracer.StartTransportOperation(ctx, "connect", "rabbitmq")
		defer span.End()
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		err := fmt.Errorf("transport is closed")
		t.recordError(span, err, "connect")
		return err
	}

	// Connect to RabbitMQ
	conn, err := amqp.Dial(t.cfg.URL)
	if err != nil {
		err = fmt.Errorf("failed to connect to RabbitMQ: %w", err)
		t.recordError(span, err, "connect")
		return err
	}
	t.conn = conn

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		t.conn.Close()
		err = fmt.Errorf("failed to create channel: %w", err)
		t.recordError(span, err, "connect")
		return err
	}
	t.channel = ch

	// Set up topology
	if err := t.setupTopology(); err != nil {
		t.channel.Close()
		t.conn.Close()
		err = fmt.Errorf("failed to setup topology: %w", err)
		t.recordError(span, err, "connect")
		return err
	}

	// Record successful connection metrics
	if t.cfg.Metrics != nil {
		t.cfg.Metrics.RecordSyncDuration("rabbitmq_connect", time.Since(start))
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
	start := time.Now()
	var span trace.Span

	// Start tracing if configured
	if t.cfg.Tracer != nil {
		ctx, span = t.cfg.Tracer.StartTransportOperation(ctx, "push", "rabbitmq")
		defer span.End()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.channel == nil {
		err := fmt.Errorf("transport not connected")
		t.recordError(span, err, "push")
		return err
	}

	bytesTransferred := int64(0)
	for _, event := range events {
		// Convert to JSON structure for serialization
		jsonEvent := toJSONEventWithVersion(event)
		body, err := json.Marshal(jsonEvent)
		if err != nil {
			err = fmt.Errorf("failed to serialize event %s: %w", event.Event.ID(), err)
			t.recordError(span, err, "push")
			return err
		}
		bytesTransferred += int64(len(body))

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
			err = fmt.Errorf("failed to publish event %s: %w", event.Event.ID(), err)
			t.recordError(span, err, "push")
			return err
		}
	}

	// Record successful metrics
	duration := time.Since(start)
	if t.cfg.Metrics != nil {
		t.cfg.Metrics.RecordSyncDuration("rabbitmq_push", duration)
	}

	// Add span attributes for successful operation
	if span != nil {
		span.SetAttributes(
			attribute.Int("events.count", len(events)),
			attribute.Int64("bytes.transferred", bytesTransferred),
			attribute.String("exchange", t.cfg.Exchange),
			attribute.String("exchange.type", t.cfg.ExchangeType),
		)
	}

	t.logInfo("Published %d events successfully (%d bytes)", len(events), bytesTransferred)
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
	start := time.Now()
	var span trace.Span

	// Start tracing if configured
	if t.cfg.Tracer != nil {
		ctx, span = t.cfg.Tracer.StartTransportOperation(ctx, "subscribe", "rabbitmq")
		defer span.End()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.channel == nil {
		err := fmt.Errorf("transport not connected")
		t.recordError(span, err, "subscribe")
		return err
	}

	if t.cfg.QueueName == "" {
		err := fmt.Errorf("queue name required for Subscribe")
		t.recordError(span, err, "subscribe")
		return err
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
		err = fmt.Errorf("failed to register consumer: %w", err)
		t.recordError(span, err, "subscribe")
		return err
	}

	// Record successful subscribe setup metrics
	if t.cfg.Metrics != nil {
		t.cfg.Metrics.RecordSyncDuration("rabbitmq_subscribe", time.Since(start))
	}

	// Add span attributes
	if span != nil {
		span.SetAttributes(
			attribute.String("queue.name", t.cfg.QueueName),
			attribute.String("consumer.tag", t.consumerTag),
			attribute.Int("prefetch.count", t.cfg.PrefetchCount),
		)
	}

	t.logInfo("Started consuming from queue %s with consumer tag %s", t.cfg.QueueName, t.consumerTag)

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

				// Process individual message with observability
				t.processMessage(ctx, msg, handler)
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
		_ = t.channel.Cancel(t.consumerTag, false)
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

// recordError records an error in both tracing and metrics if configured.
func (t *Transport) recordError(span trace.Span, err error, operation string) {
	if err == nil {
		return
	}

	// Record in tracing
	if t.cfg.Tracer != nil && span != nil {
		t.cfg.Tracer.RecordError(span, err, fmt.Sprintf("RabbitMQ %s operation failed", operation))
	}

	// Record in metrics
	if t.cfg.Metrics != nil {
		t.cfg.Metrics.RecordSyncErrors(fmt.Sprintf("rabbitmq_%s", operation), "transport_error")
	}
}

// JSONEventWithVersion is a JSON-serializable representation of EventWithVersion
type JSONEventWithVersion struct {
	Event   JSONEvent `json:"event"`
	Version string    `json:"version"`
}

// JSONEvent is a JSON-serializable representation of an Event
type JSONEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	AggregateID string                 `json:"aggregate_id"`
	Data        interface{}            `json:"data"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SimpleEvent is a simple implementation of synckit.Event for RabbitMQ transport
type SimpleEvent struct {
	IDValue          string                 `json:"id"`
	TypeValue        string                 `json:"type"`
	AggregateIDValue string                 `json:"aggregate_id"`
	DataValue        interface{}            `json:"data"`
	MetadataValue    map[string]interface{} `json:"metadata"`
}

func (e *SimpleEvent) ID() string                       { return e.IDValue }
func (e *SimpleEvent) Type() string                     { return e.TypeValue }
func (e *SimpleEvent) AggregateID() string              { return e.AggregateIDValue }
func (e *SimpleEvent) Data() interface{}                { return e.DataValue }
func (e *SimpleEvent) Metadata() map[string]interface{} { return e.MetadataValue }

// toJSONEventWithVersion converts synckit.EventWithVersion to JSONEventWithVersion
func toJSONEventWithVersion(ev synckit.EventWithVersion) JSONEventWithVersion {
	var version string
	if ev.Version != nil {
		version = ev.Version.String()
	}
	return JSONEventWithVersion{
		Event: JSONEvent{
			ID:          ev.Event.ID(),
			Type:        ev.Event.Type(),
			AggregateID: ev.Event.AggregateID(),
			Data:        ev.Event.Data(),
			Metadata:    ev.Event.Metadata(),
		},
		Version: version,
	}
}

// SimpleVersion is a simple implementation of synckit.Version for transport use
type SimpleVersion struct {
	value string
}

func (v SimpleVersion) String() string { return v.value }
func (v SimpleVersion) Compare(other synckit.Version) int {
	if otherSimple, ok := other.(SimpleVersion); ok {
		if v.value < otherSimple.value {
			return -1
		} else if v.value > otherSimple.value {
			return 1
		}
		return 0
	}
	return -1
}
func (v SimpleVersion) IsZero() bool { return v.value == "" || v.value == "0" }

// fromJSONEventWithVersion converts JSONEventWithVersion back to synckit.EventWithVersion
func fromJSONEventWithVersion(jev JSONEventWithVersion) (synckit.EventWithVersion, error) {
	event := &SimpleEvent{
		IDValue:          jev.Event.ID,
		TypeValue:        jev.Event.Type,
		AggregateIDValue: jev.Event.AggregateID,
		DataValue:        jev.Event.Data,
		MetadataValue:    jev.Event.Metadata,
	}

	// Use version string as-is
	version := SimpleVersion{value: jev.Version}

	return synckit.EventWithVersion{
		Event:   event,
		Version: version,
	}, nil
}

// processMessage processes an individual AMQP message with observability.
func (t *Transport) processMessage(ctx context.Context, msg amqp.Delivery, handler func([]synckit.EventWithVersion) error) {
	start := time.Now()
	msgSize := int64(len(msg.Body))

	// Deserialize JSON event structure
	var jsonEvent JSONEventWithVersion
	if err := json.Unmarshal(msg.Body, &jsonEvent); err != nil {
		t.logError("Failed to deserialize message: %v", err)
		// Record deserialization error
		if t.cfg.Metrics != nil {
			t.cfg.Metrics.RecordSyncErrors("rabbitmq_consume", "deserialization_error")
		}
		_ = msg.Nack(false, false) // Don't requeue malformed messages
		return
	}

	// Convert to synckit.EventWithVersion
	event, err := fromJSONEventWithVersion(jsonEvent)
	if err != nil {
		t.logError("Failed to convert JSON event: %v", err)
		if t.cfg.Metrics != nil {
			t.cfg.Metrics.RecordSyncErrors("rabbitmq_consume", "conversion_error")
		}
		_ = msg.Nack(false, false) // Don't requeue malformed messages
		return
	}

	// Handle event (batch of 1 for now)
	events := []synckit.EventWithVersion{event}
	if err := handler(events); err != nil {
		t.logError("Handler failed for message %s: %v", msg.MessageId, err)
		// Record handler error
		if t.cfg.Metrics != nil {
			t.cfg.Metrics.RecordSyncErrors("rabbitmq_consume", "handler_error")
		}
		_ = msg.Nack(false, true) // Requeue on handler error
		return
	}

	// Acknowledge successful processing
	_ = msg.Ack(false)

	// Record successful message processing metrics
	if t.cfg.Metrics != nil {
		t.cfg.Metrics.RecordSyncDuration("rabbitmq_message_process", time.Since(start))
	}

	t.logInfo("Processed message %s (%d bytes) in %v", msg.MessageId, msgSize, time.Since(start))
}
