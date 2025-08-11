package dynres

import (
	"context"

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// Conflict carries the context needed to resolve a detected conflict between
// local and remote changes. Domain-agnostic by design.
type Conflict struct {
	EventType     string
	AggregateID   string
	ChangedFields []string
	Metadata      map[string]any

	Local  types.EventWithVersion
	Remote types.EventWithVersion
}

// ResolvedConflict captures the decision and any follow-up data.
type ResolvedConflict struct {
	ResolvedEvents []types.EventWithVersion
	Decision       string
	Reasons        []string
}

// ConflictResolver is the Strategy interface for conflict resolution.
type ConflictResolver interface {
	Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error)
}

