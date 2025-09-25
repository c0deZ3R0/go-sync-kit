package versioning

import "github.com/c0deZ3R0/go-sync-kit/synckit/types"

type ResolveDecision int

const (
	KeepLocal ResolveDecision = iota
	KeepRemote
	Merge
	DeferToUser
)

type VersionedResolver interface {
	Decide(local, remote types.Event, lc, rc Clock) (ResolveDecision, *types.Event)
}