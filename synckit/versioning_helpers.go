package synckit

import (
	"github.com/c0deZ3R0/go-sync-kit/versioning"
)

const metadataClockKey = "synckit.clock"

// ExtractVectorClockFromMetadata reads a vector clock stored in Event.Metadata under a reserved key.
// It expects a map[string]uint64 or returns an empty clock if missing/wrong type.
func ExtractVectorClockFromMetadata(e Event) *versioning.VectorClock {
	m := e.Metadata()
	if m == nil {
		return versioning.FromMap(nil)
	}
	if raw, ok := m[metadataClockKey]; ok {
		if mm, ok := raw.(map[string]uint64); ok {
			return versioning.FromMap(mm)
		}
	}
	return versioning.FromMap(nil)
}

// WithVectorClock returns a shallow event wrapper that injects the provided clock map into Metadata().
// If the event's Metadata is nil, a new map is created.
type eventWithClock struct{ Event }

func (e eventWithClock) Metadata() map[string]interface{} {
	base := e.Event.Metadata()
	if base == nil {
		base = map[string]interface{}{}
	}
	return base
}

// InjectVectorClock mutates the provided metadata map to include the clock under the reserved key.
func InjectVectorClock(meta map[string]interface{}, vc *versioning.VectorClock) {
	if meta == nil || vc == nil {
		return
	}
	meta[metadataClockKey] = vc.ToMap()
}