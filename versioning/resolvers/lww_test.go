package resolvers

import (
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/versioning"
)

type simpleEvent struct{ id string }

func (e simpleEvent) ID() string                       { return e.id }
func (e simpleEvent) Type() string                     { return "t" }
func (e simpleEvent) AggregateID() string              { return "agg" }
func (e simpleEvent) Data() interface{}                { return nil }
func (e simpleEvent) Metadata() map[string]interface{} { return nil }

func TestLWWDecideTable(t *testing.T) {
	l := LWW{}
	local := simpleEvent{"L"}
	remote := simpleEvent{"R"}
	vc0 := versioning.NewVectorClock()
	vc1 := vc0.Bump("n1").(*versioning.VectorClock)
	vc2 := vc1.Bump("n1").(*versioning.VectorClock)

	tcs := []struct {
		lc, rc versioning.Clock
		want  versioning.ResolveDecision
	}{
		{lc: vc1, rc: vc2, want: versioning.KeepRemote},
		{lc: vc2, rc: vc1, want: versioning.KeepLocal},
		{lc: vc1, rc: vc1, want: versioning.KeepRemote},
		{lc: versioning.FromMap(map[string]uint64{"a": 1}), rc: versioning.FromMap(map[string]uint64{"b": 1}), want: versioning.KeepRemote},
	}

	for i, tc := range tcs {
		got, _ := l.Decide(local, remote, tc.lc, tc.rc)
		if got != tc.want {
			t.Fatalf("case %d: want %v got %v", i, tc.want, got)
		}
	}
}