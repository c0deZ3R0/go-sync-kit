package synckit

import (
	"context"
	"testing"
)

func BenchmarkDynamicResolver_Dispatch_FirstMatch(b *testing.B) {
	match := func(Conflict) bool { return true }
	nomatch := func(Conflict) bool { return false }
	mr := &mockResolver{res: ResolvedConflict{Decision:"ok"}}
	dr, _ := NewDynamicResolver(
		WithRule("rule0", match, mr),
		WithRule("rule1", nomatch, &mockResolver{}),
		WithFallback(&mockResolver{}),
	)
	c := Conflict{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dr.Resolve(context.Background(), c)
	}
}

func BenchmarkDynamicResolver_Dispatch_MiddleMatch(b *testing.B) {
	nomatch := func(Conflict) bool { return false }
	match := func(Conflict) bool { return true }
	mr := &mockResolver{res: ResolvedConflict{Decision:"ok"}}
	// Position K=10 in a 20-rule chain
	opts := make([]Option, 0, 22)
	for i := 0; i < 10; i++ { opts = append(opts, WithRule("nm", nomatch, &mockResolver{})) }
	opts = append(opts, WithRule("m", match, mr))
	for i := 0; i < 9; i++ { opts = append(opts, WithRule("nm2", nomatch, &mockResolver{})) }
	opts = append(opts, WithFallback(&mockResolver{}))
	dr, _ := NewDynamicResolver(opts...)
	c := Conflict{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dr.Resolve(context.Background(), c)
	}
}

func BenchmarkDynamicResolver_Dispatch_Fallback(b *testing.B) {
	nomatch := func(Conflict) bool { return false }
	fb := &mockResolver{res: ResolvedConflict{Decision:"fb"}}
	dr, _ := NewDynamicResolver(
		WithRule("nm1", nomatch, &mockResolver{}),
		WithRule("nm2", nomatch, &mockResolver{}),
		WithRule("nm3", nomatch, &mockResolver{}),
		WithFallback(fb),
	)
	c := Conflict{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dr.Resolve(context.Background(), c)
	}
}
