package synckit

import (
	"context"
)

// Conflict carries the context needed to resolve a detected conflict between
// local and remote changes. Domain-agnostic by design.
type Conflict struct {
	EventType     string
	AggregateID   string
	ChangedFields []string
	Metadata      map[string]any

	Local  EventWithVersion
	Remote EventWithVersion
}

// ResolvedConflict captures the decision and any follow-up data.
type ResolvedConflict struct {
	ResolvedEvents []EventWithVersion
	Decision       string
	Reasons        []string
}

// ConflictResolver is the Strategy interface for conflict resolution.
type ConflictResolver interface {
	Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error)
}
