package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// CounterConflictResolver implements conflict resolution for counter events
type CounterConflictResolver struct{}

// Resolve implements the synckit.ConflictResolver interface
func (r *CounterConflictResolver) Resolve(ctx context.Context, local, remote []synckit.EventWithVersion) ([]synckit.EventWithVersion, error) {
	// Combine all events and sort by timestamp
	allEvents := make([]synckit.EventWithVersion, 0, len(local)+len(remote))
	allEvents = append(allEvents, local...)
	allEvents = append(allEvents, remote...)

	// Sort events by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		ei, ok := allEvents[i].Event.(*CounterEvent)
		if !ok {
			return false
		}
		ej, ok := allEvents[j].Event.(*CounterEvent)
		if !ok {
			return true
		}
		return ei.timestamp.Before(ej.timestamp)
	})

	// Track the final state
	counterState := make(map[string]int) // counterID -> value

	// Process events in order
	resolvedEvents := make([]synckit.EventWithVersion, 0, len(allEvents))
	for _, ev := range allEvents {
		event, ok := ev.Event.(*CounterEvent)
		if !ok {
			continue
		}

		switch event.Type() {
		case EventTypeCounterCreated:
			// Only include the first creation event
			if _, exists := counterState[event.counterID]; !exists {
				counterState[event.counterID] = event.value
				resolvedEvents = append(resolvedEvents, ev)
			}

		case EventTypeCounterIncremented:
			if _, exists := counterState[event.counterID]; exists {
				counterState[event.counterID] += event.value
				resolvedEvents = append(resolvedEvents, ev)
			}

		case EventTypeCounterDecremented:
			if _, exists := counterState[event.counterID]; exists {
				counterState[event.counterID] -= event.value
				resolvedEvents = append(resolvedEvents, ev)
			}

		case EventTypeCounterReset:
			// Reset events override all previous operations
			counterState[event.counterID] = event.value
			resolvedEvents = append(resolvedEvents, ev)
		}
	}

	// Create a final state event if needed
	for counterID, value := range counterState {
		finalEvent := &CounterEvent{
			id:        fmt.Sprintf("resolved-%s-%d", counterID, time.Now().UnixNano()),
			eventType: EventTypeCounterReset,
			counterID: counterID,
			value:     value,
			timestamp: time.Now(),
			metadata: map[string]interface{}{
				"resolved": true,
				"source":   "conflict_resolver",
			},
		}
		resolvedEvents = append(resolvedEvents, synckit.EventWithVersion{
			Event: finalEvent,
			// Version will be assigned by the store
		})
	}

	return resolvedEvents, nil
}

// NewCounterConflictResolver creates a new CounterConflictResolver
func NewCounterConflictResolver() *CounterConflictResolver {
	return &CounterConflictResolver{}
}
