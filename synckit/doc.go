/*
Package synckit provides a generic event-driven synchronization system for distributed applications.

# Overview

Synckit enables offline-first architectures with conflict resolution and pluggable storage backends.
Applications import this single package to access all core types and interfaces:

	import "github.com/c0deZ3R0/go-sync-kit/synckit"

# Core Concepts

The system is built around four key abstractions:

1. Event: Domain events representing state changes
2. Version: Point-in-time snapshots for ordering and conflict detection
3. Store: Local event persistence (EventStore interface)
4. Transport: Network communication layer (Transport interface)

# Usage Example

	// Define your event type
	type UserEvent struct {
		id        string
		eventType string
		aggID     string
		data      interface{}
		meta      map[string]interface{}
	}

	func (e *UserEvent) ID() string                       { return e.id }
	func (e *UserEvent) Type() string                     { return e.eventType }
	func (e *UserEvent) AggregateID() string              { return e.aggID }
	func (e *UserEvent) Data() interface{}                { return e.data }
	func (e *UserEvent) Metadata() map[string]interface{} { return e.meta }

	// Use a storage backend
	store, _ := memstore.New()

	// Configure a transport
	transport := httptransport.NewClient("https://api.example.com")

	// Create a sync node
	node := synckit.NewNode(store, transport)

	// Perform synchronization
	result, err := node.Sync(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Synced: %d pushed, %d pulled", result.EventsPushed, result.EventsPulled)

# Conflict Resolution

When local and remote changes collide, implement a ConflictResolver:

	type LastWriteWinsResolver struct{}

	func (r *LastWriteWinsResolver) Resolve(ctx context.Context, c synckit.Conflict) (synckit.ResolvedConflict, error) {
		// Compare versions and pick the latest
		if c.Remote.Version.Compare(c.Local.Version) > 0 {
			return synckit.ResolvedConflict{
				ResolvedEvents: []synckit.EventWithVersion{c.Remote},
				Decision:       "remote-wins",
			}, nil
		}
		return synckit.ResolvedConflict{
			ResolvedEvents: []synckit.EventWithVersion{c.Local},
			Decision:       "local-wins",
		}, nil
	}

# Architecture

Synckit supports:
- Offline-first: local persistence with eventual consistency
- Pluggable stores: in-memory, SQLite, PostgreSQL, BadgerDB
- Pluggable transports: HTTP, WebSockets, SSE, RabbitMQ
- Conflict resolution: custom strategies per domain
- Observability: structured logging, metrics, tracing

# API surface

Import synckit to access all core types and interfaces:

	import "github.com/c0deZ3R0/go-sync-kit/synckit"

Core types (aliased from synckit/types):
- Event: Represents a syncable event
- Version: Point-in-time snapshot for ordering and conflict detection
- EventWithVersion: Pairs an event with its version
- Conflict: Context for resolving detected conflicts
- ResolvedConflict: Resolution decision and follow-up data
- ConflictResolver: Strategy interface for conflict resolution

Core interfaces:
- EventStore: Persistence for events (see sync.go)
- Transport: Network communication layer (see sync.go)
- CursorTransport: Transport with cursor-based pagination (see sync.go)

Implementor guidance
- EventStore: implement Store, Load, LoadByAggregate, LatestVersion, ParseVersion, Close
- Transport: implement Push, Pull, GetLatestVersion, Subscribe, Close

See subpackages for concrete implementations:
- storage/memstore: In-memory store for testing
- storage/sqlite: SQLite-based persistent store
- storage/postgres: PostgreSQL-based persistent store
- transport/httptransport: HTTP-based transport
- transport/sse: Server-Sent Events transport
- transport/rabbitmq: RabbitMQ-based transport

*/
package synckit
