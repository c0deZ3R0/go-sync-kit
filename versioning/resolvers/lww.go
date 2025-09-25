package resolvers

import (
	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
	"github.com/c0deZ3R0/go-sync-kit/versioning"
)

// LWW implements a simple last-write-wins decision based on vector or any clock.
type LWW struct{}

func (LWW) Decide(local, remote types.Event, lc, rc versioning.Clock) (versioning.ResolveDecision, *types.Event) {
	switch lc.Compare(rc) {
	case versioning.Lt:
		return versioning.KeepRemote, &remote
	case versioning.Gt:
		return versioning.KeepLocal, &local
	case versioning.Eq:
		// prefer remote on tie for determinism
		return versioning.KeepRemote, &remote
	default:
		// concurrent -> keep remote by default to converge
		return versioning.KeepRemote, &remote
	}
}