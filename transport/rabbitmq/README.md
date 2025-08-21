# RabbitMQ Transport (Planned)

This package will provide a RabbitMQ-based transport for go-sync-kit, complementing HTTP (Push/Pull) and SSE (Subscribe) transports.

Capabilities (planned):
- Publish events with persistent delivery and optional publisher confirms
- Topic/direct/fanout/header exchanges with configurable routing keys
- Manual-ack consumers with prefetch control and worker pool patterns
- DLQ, retry/backoff, priority queues, and TTL (Phase 2)
- Multi-tenant routing, conflict queues, replay patterns (Phase 3)

See RABBITMQ_ROADMAP.md for phases, milestones, and acceptance criteria.
