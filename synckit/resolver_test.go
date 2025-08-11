package synckit

import (
	"context"
	"errors"
	"testing"
)

// mockResolver is a simple test double implementing ConflictResolver.
type mockResolver struct{
	name string
	res  ResolvedConflict
	err  error
	calls int
}

func (m *mockResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	m.calls++
	if m.err != nil { return ResolvedConflict{}, m.err }
	return m.res, nil
}

func TestDynamicResolver_FirstMatchWins(t *testing.T) {
	mr1 := &mockResolver{name:"r1", res: ResolvedConflict{Decision:"r1"}}
	mr2 := &mockResolver{name:"r2", res: ResolvedConflict{Decision:"r2"}}
	fb := &mockResolver{name:"fb", res: ResolvedConflict{Decision:"fb"}}

	dr, err := NewDynamicResolver(
		WithRule("not-matching", func(c Conflict) bool { return false }, mr1),
		WithRule("match", func(c Conflict) bool { return true }, mr2),
		WithFallback(fb),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }

	res, err := dr.Resolve(context.Background(), Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "r2" { t.Fatalf("expected r2, got %s", res.Decision) }
	if mr1.calls != 0 { t.Fatalf("expected r1 not called") }
	if mr2.calls != 1 { t.Fatalf("expected r2 called once") }
	if fb.calls != 0 { t.Fatalf("expected fallback not called") }
}

func TestDynamicResolver_Fallback(t *testing.T) {
	fb := &mockResolver{name:"fb", res: ResolvedConflict{Decision:"fb"}}
	dr, err := NewDynamicResolver(
		WithRule("nope", func(c Conflict) bool { return false }, &mockResolver{}),
		WithFallback(fb),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	res, err := dr.Resolve(context.Background(), Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "fb" { t.Fatalf("expected fb, got %s", res.Decision) }
	if fb.calls != 1 { t.Fatalf("expected fallback called once") }
}

func TestDynamicResolver_NoFallbackError(t *testing.T) {
	_, err := NewDynamicResolver(
		WithRule("nope", func(c Conflict) bool { return false }, &mockResolver{}),
	)
	if err == nil { t.Fatalf("expected constructor error without fallback and no matching rules") }
}

func TestDynamicResolver_InvalidRuleErrors(t *testing.T) {
	_, err := NewDynamicResolver(
		WithRule("bad-matcher", nil, &mockResolver{}),
		WithFallback(&mockResolver{}),
	)
	if err == nil { t.Fatalf("expected error for nil matcher") }

	_, err = NewDynamicResolver(
		WithRule("bad-resolver", func(c Conflict) bool { return true }, nil),
		WithFallback(&mockResolver{}),
	)
	if err == nil { t.Fatalf("expected error for nil resolver") }
}

func TestDynamicResolver_Hooks(t *testing.T) {
	var matched, resolved, fallbackCalled, errored bool
	mr := &mockResolver{name:"ok", res: ResolvedConflict{Decision:"ok"}}
	fb := &mockResolver{name:"fb", res: ResolvedConflict{Decision:"fb"}}

	dr, err := NewDynamicResolver(
		WithRule("match", func(c Conflict) bool { return true }, mr),
		WithFallback(fb),
		WithHooks(Hooks{
			OnRuleMatched: func(conflict Conflict, rule Rule) { matched = true },
			OnResolved:    func(conflict Conflict, result ResolvedConflict) { resolved = true },
			OnFallback:    func(conflict Conflict) { fallbackCalled = true },
			OnError:       func(conflict Conflict, err error) { errored = true },
		}),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	_, err = dr.Resolve(context.Background(), Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if !matched || !resolved { t.Fatalf("expected matched and resolved hooks to be called") }
	if fallbackCalled { t.Fatalf("did not expect fallback hook") }
	if errored { t.Fatalf("did not expect error hook") }
}

func TestDynamicResolver_ErrorPropagatesAndHookCalled(t *testing.T) {
	mr := &mockResolver{name:"bad", err: errors.New("boom")}
	var errored bool
	dr, err := NewDynamicResolver(
		WithRule("match", func(c Conflict) bool { return true }, mr),
		WithHooks(Hooks{ OnError: func(conflict Conflict, err error) { errored = true } }),
	)
	if err != nil { t.Fatalf("unexpected construct error: %v", err) }
	_, err = dr.Resolve(context.Background(), Conflict{})
	if err == nil { t.Fatalf("expected resolve error") }
	if !errored { t.Fatalf("expected error hook to be called") }
}

func TestWithEventTypeRule_Helper(t *testing.T) {
	mr := &mockResolver{name:"evt", res: ResolvedConflict{Decision:"evt"}}
	fb := &mockResolver{name:"fb", res: ResolvedConflict{Decision:"fb"}}
	dr, err := NewDynamicResolver(
		WithEventTypeRule("evt", "UserUpdated", mr),
		WithFallback(fb),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	res, err := dr.Resolve(context.Background(), Conflict{EventType:"UserUpdated"})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "evt" { t.Fatalf("expected evt, got %s", res.Decision) }
}
