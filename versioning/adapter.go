package versioning

import (
	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// Deprecated: ConflictResolverAdapter bridges the old ConflictResolver to the new VersionedResolver.
// This adapter will be removed in a future release.
type ConflictResolverAdapter struct{ Old types.ConflictResolver }

func (a ConflictResolverAdapter) Decide(local, remote types.Event, lc, rc Clock) (ResolveDecision, *types.Event) {
	if a.Old == nil {
		return DeferToUser, nil
	}
	// We cannot perfectly reconstruct EventWithVersion here without storage versions.
	// Fallback to last-write-wins semantics using clocks to choose which event to keep,
	// then return the chosen event.
	switch lc.Compare(rc) {
	case Lt:
		return KeepRemote, &remote
	case Gt:
		return KeepLocal, &local
	default:
		return KeepRemote, &remote
	}
}

// ToOldResolved produces a minimal ResolvedConflict output for integration points still expecting the old API.
func ToOldResolved(decision ResolveDecision, chosen *types.Event) types.ResolvedConflict {
	out := types.ResolvedConflict{Decision: "versioned"}
	if chosen != nil {
		out.ResolvedEvents = []types.EventWithVersion{{Event: *chosen}}
	}
	return out
}