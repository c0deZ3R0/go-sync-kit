package resolvers

import (
	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
	"github.com/c0deZ3R0/go-sync-kit/versioning"
)

// PolicyMerge defers to a domain-specific merge policy via a hook.
type PolicyMerge struct {
	MergeHook func(local, remote types.Event, lc, rc versioning.Clock) (*types.Event, bool)
}

func (p PolicyMerge) Decide(local, remote types.Event, lc, rc versioning.Clock) (versioning.ResolveDecision, *types.Event) {
	if p.MergeHook == nil {
		return versioning.DeferToUser, nil
	}
	if ev, ok := p.MergeHook(local, remote, lc, rc); ok {
		return versioning.Merge, ev
	}
	return versioning.DeferToUser, nil
}