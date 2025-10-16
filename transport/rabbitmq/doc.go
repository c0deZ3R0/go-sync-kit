// Package rabbitmq provides a RabbitMQ-based message transport.
//
// RabbitMQTransport implements the synckit.Transport interface using RabbitMQ as a
// message broker. It's suitable for distributed systems requiring reliable, decoupled
// event propagation across multiple nodes and services.
//
// See also:
//   - README: https://github.com/c0deZ3R0/go-sync-kit#readme
//   - Architecture overview: https://github.com/c0deZ3R0/go-sync-kit/blob/main/docs/overview.md
package rabbitmq
