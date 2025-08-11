package dynres

import (
	"context"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Conflict carries the context needed to resolve a detected conflict between
// local and remote changes. Domain-agnostic by design.
type Conflict struct {
	EventType     string
	AggregateID   string
	ChangedFields []string
	Metadata      map[string]any

	Local  synckit.EventWithVersion
	Remote synckit.EventWithVersion
}

// ResolvedConflict captures the decision and any follow-up data.
type ResolvedConflict struct {
	ResolvedEvents []synckit.EventWithVersion
	Decision       string
	Reasons        []string
}

// ConflictResolver is the Strategy interface for conflict resolution.
type ConflictResolver interface {
	Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error)
}

