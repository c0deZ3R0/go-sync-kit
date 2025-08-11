package synckit

import (
	"context"
)

// Compile-time checks to ensure resolvers implement ConflictResolver.
var (
	_ ConflictResolver = (*LastWriteWinsResolver)(nil)
	_ ConflictResolver = (*AdditiveMergeResolver)(nil)
	_ ConflictResolver = (*ManualReviewResolver)(nil)
)

// LastWriteWinsResolver selects the event with the higher Version according to
// Version.Compare semantics. If equal, defaults to remote.
type LastWriteWinsResolver struct{}

func (r *LastWriteWinsResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	if c.Local.Version == nil && c.Remote.Version == nil {
		return ResolvedConflict{ResolvedEvents: nil, Decision: "noop", Reasons: []string{"no versions"}}, nil
	}
	if c.Local.Version == nil {
		return ResolvedConflict{ResolvedEvents: []EventWithVersion{c.Remote}, Decision: "keep_remote", Reasons: []string{"local missing"}}, nil
	}
	if c.Remote.Version == nil {
		return ResolvedConflict{ResolvedEvents: []EventWithVersion{c.Local}, Decision: "keep_local", Reasons: []string{"remote missing"}}, nil
	}
	switch c.Local.Version.Compare(c.Remote.Version) {
	case -1:
		return ResolvedConflict{ResolvedEvents: []EventWithVersion{c.Remote}, Decision: "keep_remote", Reasons: []string{"remote newer"}}, nil
	case 1:
		return ResolvedConflict{ResolvedEvents: []EventWithVersion{c.Local}, Decision: "keep_local", Reasons: []string{"local newer"}}, nil
	default:
		// equal – prefer remote by default
		return ResolvedConflict{ResolvedEvents: []EventWithVersion{c.Remote}, Decision: "keep_remote", Reasons: []string{"equal versions, prefer remote"}}, nil
	}
}

// AdditiveMergeResolver performs a simple union-like merge by returning both
// local and remote events (caller must define idempotency downstream).
type AdditiveMergeResolver struct{}

func (r *AdditiveMergeResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	out := make([]EventWithVersion, 0, 2)
	if (c.Local != EventWithVersion{}) {
		out = append(out, c.Local)
	}
	if (c.Remote != EventWithVersion{}) {
		out = append(out, c.Remote)
	}
	return ResolvedConflict{ResolvedEvents: out, Decision: "merge", Reasons: []string{"additive merge"}}, nil
}

// ManualReviewResolver flags a manual review requirement without choosing a side.
type ManualReviewResolver struct{
	Reason string
}

func (r *ManualReviewResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	reasons := []string{"manual review required"}
	if r.Reason != "" {
		reasons = append(reasons, r.Reason)
	}
	return ResolvedConflict{ResolvedEvents: nil, Decision: "manual_review", Reasons: reasons}, nil
}
