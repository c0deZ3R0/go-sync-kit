# RABBITMQ_ROADMAP.md

Purpose
- Introduce a RabbitMQ transport to complement existing HTTP (request/response) and SSE (real-time subscribe) transports.
- Provide durable queuing, reliable delivery (ack/confirm), flexible routing, and enterprise messaging patterns.

Status
- Branch: feature/transport-rabbitmq
- Phase: Planning and scaffolding
- Owner: Agent Mode (assisting via Warp)
- Commit policy: Do not commit or push without explicit instruction

Links
- Planned transport note in WARP.md
- Transport interface: synckit/sync.go (type Transport, type CursorTransport)
- HTTP transport reference: transport/httptransport/
- SSE transport reference: transport/sse/

High-level Design
- Transport maps
  - Push: publish events to an exchange using persistent messages; optional publisher confirms
  - Subscribe: consume from queues with manual acks; deliver batches to handler
  - Pull: for RabbitMQ, pull is emulated via short-lived consumer or Get; may be optional if using subscribe; consider hybrid approach (HTTP for Pull, RabbitMQ for Subscribe/Push)
- Version handling: event Version carried in message body; remote GetLatestVersion may be handled via store-backed endpoint or a lightweight control queue; recommended hybrid: GetLatestVersion via HTTP until a broker-backed pattern is added
- Routing: topic exchange by default, configurable strategy function to derive routing key from event

Milestones & Phases

Phase 1: MVP RabbitMQ Transport
- [ ] Package scaffolding: transport/rabbitmq/
  - [ ] README.md with usage and design notes
  - [ ] types.go: Config, internal state, basic DTOs
  - [ ] transport.go: implements synckit.Transport (Push, Subscribe, Close); Pull may be stubbed or delegated
- [ ] Config structure
  - [ ] URL, ConnectionName, Heartbeat
  - [ ] Exchange, ExchangeType (topic|direct|fanout|headers)
  - [ ] QueueName (consumer), QueueDurable, QueueAutoDelete, QueueExclusive
  - [ ] RoutingKey func(Event) string
  - [ ] MessagePersistent, MessageTTL, Priority
  - [ ] ConfirmMode (publisher confirms), PrefetchCount (QoS)
  - [ ] Logger, Metrics, Tracer hooks
- [ ] Connection management
  - [ ] Establish connection/channel
  - [ ] Heartbeat and connection name
  - [ ] Reconnect loop with backoff
- [ ] Topology
  - [ ] Declare exchange (idempotent)
  - [ ] Declare queue (consumer side)
  - [ ] Bindings from BindingKeys
- [ ] Publishing (Push)
  - [ ] JSON serialization of EventWithVersion
  - [ ] Persistent mode; content-type application/json
  - [ ] Optional publisher confirms and error propagation
- [ ] Consuming (Subscribe)
  - [ ] Consume with manual acks
  - [ ] Batch assembly and handler invocation
  - [ ] Nack on error with requeue=true
  - [ ] Prefetch to control load
- [ ] Observability
  - [ ] Trace spans around publish/consume
  - [ ] Metrics counters and histograms
- [ ] Docs
  - [ ] Usage doc in transport/rabbitmq/README.md with example Manager setup

Acceptance for Phase 1
- [ ] Unit tests for config validation, basic publish path, and consumer handler invocation
- [ ] Local integration test using Docker RabbitMQ (documented compose and make target)
- [ ] Graceful Close releases channel/connection and stops consumers

Phase 2: Reliability & Enterprise Features
- [ ] Dead letter queues (DLQ)
  - [ ] x-dead-letter-exchange/queue topology
  - [ ] Route failed messages to DLQ on Nack/retries exceeded
- [ ] Retry policy
  - [ ] Configurable MaxRetries with exponential backoff
  - [ ] Delayed retries via per-message TTL and DLX cycling
- [ ] Priority queues
  - [ ] Enable x-max-priority and map event metadata to priority header
- [ ] Message TTL
  - [ ] Queue- or message-level TTL
- [ ] Confirm mode improvements
  - [ ] Track confirms and map to errors with correlation IDs
- [ ] Return channel (mandatory publishes)
- [ ] Back-pressure & load shedding via prefetch and worker pools

Acceptance for Phase 2
- [ ] Integration tests demonstrating DLQ, retries with backoff, priority ordering
- [ ] Metrics for retries, DLQ routed messages, consumer lag

Phase 3: go-sync-kit Patterns & Specialization
- [ ] Multi-tenant routing
  - [ ] RoutingKey func that incorporates tenant, type, and priority (e.g., tenant.<id>.<type>.<prio>)
  - [ ] Declarative BindingKeys for selective consumption
- [ ] Conflict-resolution routing
  - [ ] Dedicated exchanges/queues for conflict classes
- [ ] Event replay & audit trail
  - [ ] Durable queues for time-windowed replay
  - [ ] Tooling to requeue from DLQ with safety controls
- [ ] Hybrid transport guidance
  - [ ] Recommend HTTP for Pull and GetLatestVersion; RabbitMQ for Push/Subscribe
  - [ ] Example setups and docs

Acceptance for Phase 3
- [ ] Example applications: multi-tenant consumer setup; conflict queue workflow; replay demo
- [ ] Documentation covering operational patterns and tradeoffs

Design Notes & Decisions
- Push/Pull vs Subscribe semantics
  - RabbitMQ excels at async pub/sub; HTTP remains best for ad-hoc Pull and GetLatestVersion
  - Cursor-based semantics are not native to brokers; treat version as payload-only and rely on store for version queries
- Idempotency
  - Consumers should handle at-least-once delivery; include idempotent processing guidance
- Observability
  - Reuse existing metrics/tracing conventions; label transport_type="rabbitmq"

Development Setup
- Requirements
  - Go 1.24.4+
  - Docker (for local RabbitMQ)
- Local broker
  - docker run -it --rm -p 5672:5672 -p 15672:15672 rabbitmq:3-management
- Make targets (planned)
  - make rabbitmq-up / rabbitmq-down
  - make rabbitmq-test (integration)

Risks & Mitigations
- Broker outages: implement reconnect + backoff
- Message loss on publish: use persistent messages and publisher confirms
- Poison messages: DLQ and limited retry with backoff
- Consumer overload: prefetch, worker pool, and back-pressure metrics

Open Questions
- Should Pull be fully implemented on RabbitMQ, or documented as "Subscribe-only" with HTTP Pull fallback?
- Should we add a small control topic/queue to query latest version, or keep it out-of-band?

Changelog
- 2025-08-21: Roadmap created and branch feature/transport-rabbitmq started

