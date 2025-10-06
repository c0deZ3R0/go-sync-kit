# Transport Layer (Architecture)

What it is:
- The network boundary. Transport moves events between nodes.
- Implements push/pull request/response and optional real-time subscribe.

How it fits with SyncNode:
- Push: SyncNode hands Transport local events to send to remote.
- Pull: SyncNode asks Transport for remote events since a Version.
- Subscribe (optional): Transport streams remote events to SyncNode in real time.

Contract (simplified):
- Push(ctx, []EventWithVersion) → error
- Pull(ctx, since Version) → []EventWithVersion, error
- Subscribe(ctx, handler func([]EventWithVersion) error) → error
- Close() → error

Swap the backend, keep the contract:
- In-memory (memchan), HTTP (request/response), SSE (real-time), RabbitMQ (durable messaging).
- Your app code talks to the interface, not the concrete transport.

Minimal usage:
```go
node, _ := synckit.NewNode(
    synckit.WithStore(store),          // any EventStore
    synckit.WithTransport(transport),  // any Transport
    synckit.WithLWW(),
)
```

Notes:
- Modes: HTTP for Push/Pull, SSE for Subscribe; combine for hybrid real-time.
- Backpressure: implementations should handle batching and rate limiting.
- Idempotency: server side should accept duplicate deliveries safely.
- Choice guide: dev = memchan, client/server = httptransport (+ sse), durable bus = rabbitmq.
