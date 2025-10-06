// Package synckit provides a single import surface for event-driven synchronization.
// Applications can import this package and access all core types and interfaces.
//
// # Single Import Surface
//
// All core types and interfaces are available by importing synckit:
//
//	import "github.com/c0deZ3R0/go-sync-kit/synckit"
//
// Core types (aliased from synckit/types):
//   - Event: Represents a syncable event
//   - Version: Point-in-time snapshot for ordering and conflict detection
//   - EventWithVersion: Pairs an event with its version
//   - Conflict: Context for resolving detected conflicts
//   - ResolvedConflict: Resolution decision and follow-up data
//   - ConflictResolver: Strategy interface for conflict resolution
//
// Core interfaces:
//   - EventStore: Persistence for events (see sync.go)
//   - Transport: Network communication layer (see sync.go)
//   - CursorTransport: Transport with cursor-based pagination (see sync.go)
//
// # Implementor Guidance
//
// EventStore implementations should provide:
//   - Store: Persist events with version
//   - Load: Retrieve events since a given version
//   - LoadByAggregate: Retrieve events for a specific aggregate
//   - LatestVersion: Get the latest version from store
//   - ParseVersion: Convert string to Version
//   - Close: Release resources
//
// Transport implementations should provide:
//   - Push: Send events to remote endpoint
//   - Pull: Retrieve events from remote since a version
//   - GetLatestVersion: Efficiently get latest remote version
//   - Subscribe: Listen for real-time updates (optional for polling)
//   - Close: Release connection resources
//
// See subpackages for concrete implementations:
//   - storage/memstore: In-memory store for testing
//   - storage/sqlite: SQLite-based persistent store
//   - storage/postgres: PostgreSQL-based persistent store
//   - transport/httptransport: HTTP-based transport
//   - transport/sse: Server-Sent Events transport
//   - transport/rabbitmq: RabbitMQ-based transport
package synckit

// Note: Type aliases for Event, Version, EventWithVersion are defined in sync.go.
// Conflict-related type aliases are defined in conflict.go.
// This file serves as API documentation and import surface guidance.
