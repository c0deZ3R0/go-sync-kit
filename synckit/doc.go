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

See subpackages for specific store and transport implementations.
*/
package synckit
