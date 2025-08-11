package synckit

import (
	"context"
	"errors"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/synckit/dynres"
)

// mockResolver is a simple test double implementing ConflictResolver.
type mockResolver struct{
	name string
	res  dynres.ResolvedConflict
	err  error
	calls int
}

func (m *mockResolver) Resolve(ctx context.Context, c dynres.Conflict) (dynres.ResolvedConflict, error) {
	m.calls++
	if m.err != nil { return dynres.ResolvedConflict{}, m.err }
	return m.res, nil
}

func TestDynamicResolver_FirstMatchWins(t *testing.T) {
	mr1 := &mockResolver{name:"r1", res: dynres.ResolvedConflict{Decision:"r1"}}
	mr2 := &mockResolver{name:"r2", res: dynres.ResolvedConflict{Decision:"r2"}}
	fb := &mockResolver{name:"fb", res: dynres.ResolvedConflict{Decision:"fb"}}

	dr, err := NewDynamicResolver(
		WithRule("not-matching", func(c dynres.Conflict) bool { return false }, mr1),
		WithRule("match", func(c dynres.Conflict) bool { return true }, mr2),
		WithFallback(fb),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }

	res, err := dr.Resolve(context.Background(), dynres.Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "r2" { t.Fatalf("expected r2, got %s", res.Decision) }
	if mr1.calls != 0 { t.Fatalf("expected r1 not called") }
	if mr2.calls != 1 { t.Fatalf("expected r2 called once") }
	if fb.calls != 0 { t.Fatalf("expected fallback not called") }
}

func TestDynamicResolver_Fallback(t *testing.T) {
	fb := &mockResolver{name:"fb", res: dynres.ResolvedConflict{Decision:"fb"}}
	dr, err := NewDynamicResolver(
		WithRule("nope", func(c dynres.Conflict) bool { return false }, &mockResolver{}),
		WithFallback(fb),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	res, err := dr.Resolve(context.Background(), dynres.Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "fb" { t.Fatalf("expected fb, got %s", res.Decision) }
	if fb.calls != 1 { t.Fatalf("expected fallback called once") }
}

func TestDynamicResolver_NoFallbackError(t *testing.T) {
	_, err := NewDynamicResolver(
		WithRule("nope", func(c dynres.Conflict) bool { return false }, &mockResolver{}),
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
		WithRule("bad-resolver", func(c dynres.Conflict) bool { return true }, nil),
		WithFallback(&mockResolver{}),
	)
	if err == nil { t.Fatalf("expected error for nil resolver") }
}

func TestDynamicResolver_Hooks(t *testing.T) {
	var matched, resolved, fallbackCalled, errored bool
	mr := &mockResolver{name:"ok", res: dynres.ResolvedConflict{Decision:"ok"}}
	fb := &mockResolver{name:"fb", res: dynres.ResolvedConflict{Decision:"fb"}}

	dr, err := NewDynamicResolver(
		WithRule("match", func(c dynres.Conflict) bool { return true }, mr),
		WithFallback(fb),
		WithHooks(Hooks{
			OnRuleMatched: func(conflict dynres.Conflict, rule Rule) { matched = true },
			OnResolved:    func(conflict dynres.Conflict, result dynres.ResolvedConflict) { resolved = true },
			OnFallback:    func(conflict dynres.Conflict) { fallbackCalled = true },
			OnError:       func(conflict dynres.Conflict, err error) { errored = true },
		}),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	_, err = dr.Resolve(context.Background(), dynres.Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if !matched || !resolved { t.Fatalf("expected matched and resolved hooks to be called") }
	if fallbackCalled { t.Fatalf("did not expect fallback hook") }
	if errored { t.Fatalf("did not expect error hook") }
}

func TestDynamicResolver_ErrorPropagatesAndHookCalled(t *testing.T) {
	mr := &mockResolver{name:"bad", err: errors.New("boom")}
	var errored bool
	dr, err := NewDynamicResolver(
		WithRule("match", func(c dynres.Conflict) bool { return true }, mr),
		WithHooks(Hooks{ OnError: func(conflict dynres.Conflict, err error) { errored = true } }),
	)
	if err != nil { t.Fatalf("unexpected construct error: %v", err) }
	_, err = dr.Resolve(context.Background(), dynres.Conflict{})
	if err == nil { t.Fatalf("expected resolve error") }
	if !errored { t.Fatalf("expected error hook to be called") }
}

func TestWithEventTypeRule_Helper(t *testing.T) {
	mr := &mockResolver{name:"evt", res: dynres.ResolvedConflict{Decision:"evt"}}
	fb := &mockResolver{name:"fb", res: dynres.ResolvedConflict{Decision:"fb"}}
	dr, err := NewDynamicResolver(
		WithEventTypeRule("evt", "UserUpdated", mr),
		WithFallback(fb),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	res, err := dr.Resolve(context.Background(), dynres.Conflict{EventType:"UserUpdated"})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "evt" { t.Fatalf("expected evt, got %s", res.Decision) }
}

func TestDynamicResolver_OverlappingRules_FirstWins(t *testing.T) {
	mr1 := &mockResolver{name:"r1", res: dynres.ResolvedConflict{Decision:"r1"}}
	mr2 := &mockResolver{name:"r2", res: dynres.ResolvedConflict{Decision:"r2"}}
	// Both rules match any conflict
	dr, err := NewDynamicResolver(
		WithRule("rule1", func(c dynres.Conflict) bool { return true }, mr1),
		WithRule("rule2", func(c dynres.Conflict) bool { return true }, mr2),
	)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	res, err := dr.Resolve(context.Background(), dynres.Conflict{})
	if err != nil { t.Fatalf("unexpected resolve error: %v", err) }
	if res.Decision != "r1" { t.Fatalf("expected first rule (r1) to win, got %s", res.Decision) }
	if mr1.calls != 1 || mr2.calls != 0 { t.Fatalf("expected r1 called once, r2 not called") }
}

// testValidator errors if any rule has name "bad"; used to verify validator hook.
type testValidator struct{}

func (testValidator) Validate(o *resolverOptions) error {
	for _, r := range o.rules {
		if r.Name == "bad" { return errors.New("bad rule not allowed") }
	}
	return nil
}

func TestDynamicResolver_ValidatorCalled(t *testing.T) {
	_, err := NewDynamicResolver(
		WithRule("ok", func(dynres.Conflict) bool { return true }, &mockResolver{}),
		WithValidator(testValidator{}),
	)
	if err != nil { t.Fatalf("did not expect error for ok rule: %v", err) }

	_, err = NewDynamicResolver(
		WithRule("bad", func(dynres.Conflict) bool { return true }, &mockResolver{}),
		WithValidator(testValidator{}),
	)
	if err == nil { t.Fatalf("expected validator error for bad rule") }
}

func TestDynamicResolver_DeterministicOrder(t *testing.T) {
	mr1 := &mockResolver{name:"r1", res: dynres.ResolvedConflict{Decision:"r1"}}
	mr2 := &mockResolver{name:"r2", res: dynres.ResolvedConflict{Decision:"r2"}}
	build := func() *DynamicResolver {
		d, err := NewDynamicResolver(
			WithRule("a", func(dynres.Conflict) bool { return true }, mr1),
			WithRule("b", func(dynres.Conflict) bool { return true }, mr2),
		)
		if err != nil { t.Fatalf("construct: %v", err) }
		return d
	}
	// Run multiple times to ensure stable selection
	for i := 0; i < 5; i++ {
		mr1.calls, mr2.calls = 0, 0
		dr := build()
		res, err := dr.Resolve(context.Background(), dynres.Conflict{})
		if err != nil { t.Fatalf("resolve: %v", err) }
		if res.Decision != "r1" { t.Fatalf("iteration %d: expected r1, got %s", i, res.Decision) }
	}
}
