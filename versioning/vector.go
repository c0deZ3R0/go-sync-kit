package versioning

import "maps"

type VectorClock struct {
	// map of nodeID -> counter
	m map[string]uint64
}

func NewVectorClock() *VectorClock { return &VectorClock{m: make(map[string]uint64)} }

func (v *VectorClock) clone() *VectorClock {
	c := NewVectorClock()
	for k, val := range v.m {
		c.m[k] = val
	}
	return c
}

func (v *VectorClock) Compare(other Clock) Relation {
	ovc, ok := other.(*VectorClock)
	if !ok || ovc == nil {
		return Con
	}
	// track if v <= other and strictly less somewhere, and vice versa
	less, greater := false, false
	seen := map[string]struct{}{}
	for k, a := range v.m {
		b := ovc.m[k]
		if a < b {
			less = true
		} else if a > b {
			greater = true
		}
		seen[k] = struct{}{}
	}
	for k, b := range ovc.m {
		if _, ok := seen[k]; ok {
			continue
		}
		a := v.m[k]
		if a < b {
			less = true
		} else if a > b {
			greater = true
		}
	}
	if less && !greater {
		return Lt
	}
	if greater && !less {
		return Gt
	}
	if !less && !greater {
		return Eq
	}
	return Con
}

func (v *VectorClock) Merge(other Clock) Clock {
	ovc, ok := other.(*VectorClock)
	if !ok || ovc == nil {
		return v.clone()
	}
	c := v.clone()
	for k, b := range ovc.m {
		if a, ok := c.m[k]; !ok || b > a {
			c.m[k] = b
		}
	}
	return c
}

func (v *VectorClock) Bump(nodeID string) Clock {
	c := v.clone()
	c.m[nodeID] = c.m[nodeID] + 1
	return c
}

// ToMap returns a copy of the internal map for storage in metadata, etc.
func (v *VectorClock) ToMap() map[string]uint64 {
	out := make(map[string]uint64, len(v.m))
	for k, val := range v.m {
		out[k] = val
	}
	return out
}

// FromMap builds a VectorClock from the provided map.
func FromMap(m map[string]uint64) *VectorClock {
	vc := NewVectorClock()
	if m == nil {
		return vc
	}
	maps.Copy(vc.m, m)
	return vc
}