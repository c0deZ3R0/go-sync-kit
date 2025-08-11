package dynres

import (
	"context"
	"testing"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// testVersion implements Version for ordering in tests.
type testVersion struct{ v int }

func (t testVersion) Compare(other synckit.Version) int {
	if other == nil {
		return 1
	}
	o, ok := other.(testVersion)
	if !ok {
		return 0
	}
	if t.v < o.v {
		return -1
	}
	if t.v > o.v {
		return 1
	}
	return 0
}
func (t testVersion) String() string { return "" }
func (t testVersion) IsZero() bool  { return t.v == 0 }

// testEvent implements Event for tests.
type testEvent struct{
	id string
	typ string
	agg string
	data any
	meta map[string]any
}

func (e testEvent) ID() string               { return e.id }
func (e testEvent) Type() string             { return e.typ }
func (e testEvent) AggregateID() string      { return e.agg }
func (e testEvent) Data() any                { return e.data }
func (e testEvent) Metadata() map[string]any { return e.meta }

func ev(id string, v int) synckit.EventWithVersion {
	return synckit.EventWithVersion{Event: testEvent{id: id}, Version: testVersion{v: v}}
}

func TestLastWriteWinsResolver(t *testing.T) {
	r := &LastWriteWinsResolver{}
	ctx := context.Background()
	tests := []struct{
		name string
		c    Conflict
		wantDecision string
		wantIDs []string
	}{
		{
			name: "remote newer wins",
			c: Conflict{Local: ev("L", 1), Remote: ev("R", 2)},
			wantDecision: "keep_remote",
			wantIDs: []string{"R"},
		},
		{
			name: "local newer wins",
			c: Conflict{Local: ev("L", 3), Remote: ev("R", 2)},
			wantDecision: "keep_local",
			wantIDs: []string{"L"},
		},
		{
			name: "equal prefer remote",
			c: Conflict{Local: ev("L", 2), Remote: ev("R", 2)},
			wantDecision: "keep_remote",
			wantIDs: []string{"R"},
		},
		{
			name: "local nil uses remote",
			c: Conflict{Local: synckit.EventWithVersion{}, Remote: ev("R", 1)},
			wantDecision: "keep_remote",
			wantIDs: []string{"R"},
		},
		{
			name: "remote nil uses local",
			c: Conflict{Local: ev("L", 1), Remote: synckit.EventWithVersion{}},
			wantDecision: "keep_local",
			wantIDs: []string{"L"},
		},
		{
			name: "both nil noop",
			c: Conflict{Local: synckit.EventWithVersion{}, Remote: synckit.EventWithVersion{}},
			wantDecision: "noop",
			wantIDs: []string{},
		},
	}
for _, tt := range tests {
		tt := tt
		res, err := r.Resolve(ctx, tt.c)
		if err != nil { t.Fatalf("unexpected err: %v", err) }
		if res.Decision != tt.wantDecision {
			t.Fatalf("decision: got %s want %s", res.Decision, tt.wantDecision)
		}
		if len(res.ResolvedEvents) != len(tt.wantIDs) {
			t.Fatalf("len events: got %d want %d", len(res.ResolvedEvents), len(tt.wantIDs))
		}
		for i, e := range res.ResolvedEvents {
			if e.Event.ID() != tt.wantIDs[i] {
				t.Fatalf("id[%d]: got %s want %s", i, e.Event.ID(), tt.wantIDs[i])
			}
		}
	}
}

func TestAdditiveMergeResolver(t *testing.T) {
	r := &AdditiveMergeResolver{}
	ctx := context.Background()
	tests := []struct{
		name string
		c    Conflict
		wantIDs []string
	}{
		{"local only", Conflict{Local: ev("L", 1)}, []string{"L"}},
		{"remote only", Conflict{Remote: ev("R", 1)}, []string{"R"}},
		{"both", Conflict{Local: ev("L", 1), Remote: ev("R", 2)}, []string{"L","R"}},
		{"none", Conflict{}, []string{}},
	}
for _, tt := range tests {
		res, err := r.Resolve(ctx, tt.c)
		if err != nil { t.Fatalf("unexpected err: %v", err) }
		if len(res.ResolvedEvents) != len(tt.wantIDs) {
			t.Fatalf("len events: got %d want %d", len(res.ResolvedEvents), len(tt.wantIDs))
		}
		for i, e := range res.ResolvedEvents {
			if e.Event.ID() != tt.wantIDs[i] {
				t.Fatalf("id[%d]: got %s want %s", i, e.Event.ID(), tt.wantIDs[i])
			}
		}
	}
}

func TestManualReviewResolver(t *testing.T) {
	r := &ManualReviewResolver{Reason: "policy"}
	res, err := r.Resolve(context.Background(), Conflict{})
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if res.Decision != "manual_review" { t.Fatalf("decision: %s", res.Decision) }
	if len(res.ResolvedEvents) != 0 { t.Fatalf("expected no events") }
	found := false
	for _, s := range res.Reasons { if s == "policy" { found = true; break } }
	if !found { t.Fatalf("expected reason 'policy' in %v", res.Reasons) }
}

