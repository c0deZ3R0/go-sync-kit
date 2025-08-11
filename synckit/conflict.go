package synckit

import (
	"context"
)

// Conflict carries the context needed to resolve a detected conflict between
// local and remote changes. Keep this generic and domain-agnostic.
type Conflict struct {
	EventType    string                 // logical event type
	AggregateID  string                 // aggregate identifier
	ChangedFields []string              // optional: fields implicated in conflict
	Metadata     map[string]any         // arbitrary metadata (origin node, tags, etc.)

	Local  EventWithVersion            // local candidate
	Remote EventWithVersion            // remote candidate
}

// ResolvedConflict captures the decision and any follow-up data.
type ResolvedConflict struct {
	ResolvedEvents []EventWithVersion   // events to apply (may be 0, 1, or many)
	Decision       string               // optional: e.g., "keep_local", "keep_remote", "merge", "manual_review"
	Reasons        []string             // human-readable reasons/annotations for audit/telemetry
}

// ConflictResolver is the Strategy interface for conflict resolution.
type ConflictResolver interface {
	Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error)
}
