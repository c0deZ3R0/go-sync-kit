package dynres

import (
	"context"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

var (
	_ ConflictResolver = (*LastWriteWinsResolver)(nil)
	_ ConflictResolver = (*AdditiveMergeResolver)(nil)
	_ ConflictResolver = (*ManualReviewResolver)(nil)
)

type LastWriteWinsResolver struct{}

func (r *LastWriteWinsResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	if c.Local.Version == nil && c.Remote.Version == nil {
		return ResolvedConflict{Decision: "noop", Reasons: []string{"no versions"}}, nil
	}
	if c.Local.Version == nil {
		return ResolvedConflict{ResolvedEvents: []synckit.EventWithVersion{c.Remote}, Decision: "keep_remote", Reasons: []string{"local missing"}}, nil
	}
	if c.Remote.Version == nil {
		return ResolvedConflict{ResolvedEvents: []synckit.EventWithVersion{c.Local}, Decision: "keep_local", Reasons: []string{"remote missing"}}, nil
	}
	switch c.Local.Version.Compare(c.Remote.Version) {
	case -1:
		return ResolvedConflict{ResolvedEvents: []synckit.EventWithVersion{c.Remote}, Decision: "keep_remote", Reasons: []string{"remote newer"}}, nil
	case 1:
		return ResolvedConflict{ResolvedEvents: []synckit.EventWithVersion{c.Local}, Decision: "keep_local", Reasons: []string{"local newer"}}, nil
	default:
		return ResolvedConflict{ResolvedEvents: []synckit.EventWithVersion{c.Remote}, Decision: "keep_remote", Reasons: []string{"equal versions, prefer remote"}}, nil
	}
}

type AdditiveMergeResolver struct{}

func (r *AdditiveMergeResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	out := make([]synckit.EventWithVersion, 0, 2)
	if (c.Local != synckit.EventWithVersion{}) {
		out = append(out, c.Local)
	}
	if (c.Remote != synckit.EventWithVersion{}) {
		out = append(out, c.Remote)
	}
	return ResolvedConflict{ResolvedEvents: out, Decision: "merge", Reasons: []string{"additive merge"}}, nil
}

type ManualReviewResolver struct{ Reason string }

func (r *ManualReviewResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	reasons := []string{"manual review required"}
	if r.Reason != "" { reasons = append(reasons, r.Reason) }
	return ResolvedConflict{Decision: "manual_review", Reasons: reasons}, nil
}

