package versioning

import "testing"

func TestVectorClockCompareMergeBump(t *testing.T) {
	vc1 := NewVectorClock()
	vc2 := NewVectorClock()

	vc1 = FromMap(map[string]uint64{"a": 1})
	vc2 = FromMap(map[string]uint64{"a": 2})

	if got := vc1.Compare(vc2); got != Lt {
		t.Fatalf("expected vc1 < vc2, got %v", got)
	}
	m := vc1.Merge(vc2).(*VectorClock)
	if m.m["a"] != 2 {
		t.Fatalf("merge should take max; got %v", m.m["a"])
	}
	b := m.Bump("a").(*VectorClock)
	if b.m["a"] != 3 {
		t.Fatalf("bump expected 3 got %v", b.m["a"])
	}
}