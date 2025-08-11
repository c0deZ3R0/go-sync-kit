package client

import (
	"context"
	"fmt"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/version"
)

// ImprovedCounterConflictResolver resolves conflicts between counter events using vector clocks
type ImprovedCounterConflictResolver struct{}

// NewImprovedCounterConflictResolver creates a new counter conflict resolver
func NewImprovedCounterConflictResolver() *ImprovedCounterConflictResolver {
	return &ImprovedCounterConflictResolver{}
}

// Resolve implements synckit.ConflictResolver using the new interface
func (r *ImprovedCounterConflictResolver) Resolve(ctx context.Context, conflict synckit.Conflict) (synckit.ResolvedConflict, error) {
	// Extract counter events from the conflict
	localEvent, localOk := conflict.Local.Event.(*CounterEvent)
	remoteEvent, remoteOk := conflict.Remote.Event.(*CounterEvent)
	
	if !localOk || !remoteOk {
		return synckit.ResolvedConflict{
			Decision: "error",
			Reasons:  []string{"conflict contains non-counter events"},
		}, fmt.Errorf("expected counter events, got local: %T, remote: %T", conflict.Local.Event, conflict.Remote.Event)
	}
	
	// Combine the conflicting events for resolution
	events := []*CounterEvent{localEvent, remoteEvent}
	
	// Resolve the counter events using our existing logic
	resolvedEvents, err := r.resolveCounterEvents(events)
	if err != nil {
		return synckit.ResolvedConflict{
			Decision: "error",
			Reasons:  []string{fmt.Sprintf("resolution failed: %v", err)},
		}, err
	}
	
	// Convert resolved events back to EventWithVersion format
	var resultEvents []synckit.EventWithVersion
	for _, ev := range resolvedEvents {
		resultEvents = append(resultEvents, synckit.EventWithVersion{
			Event:   ev,
			Version: ev.Version(),
		})
	}
	
	// Determine the resolution decision
	decision := "merge"
	reasons := []string{"counter events merged using vector clock causality"}
	
	if len(resolvedEvents) == 1 {
		if resolvedEvents[0].clientID == "resolver" {
			decision = "combined"
			reasons = []string{"concurrent counter operations combined"}
		} else {
			decision = "keep_winner"
			reasons = []string{fmt.Sprintf("kept event from client %s based on resolution rules", resolvedEvents[0].clientID)}
		}
	}
	
	return synckit.ResolvedConflict{
		ResolvedEvents: resultEvents,
		Decision:       decision,
		Reasons:        reasons,
	}, nil
}

// resolveCounterEvents resolves conflicts for a single counter's events
func (r *ImprovedCounterConflictResolver) resolveCounterEvents(events []*CounterEvent) ([]*CounterEvent, error) {
	if len(events) <= 1 {
		return events, nil
	}

	// Check vector clock causality
	var concurrent, ordered []*CounterEvent
	for i, e1 := range events {
		isConcurrent := false
		for j, e2 := range events {
			if i != j {
				rel := e1.Version().Compare(e2.Version())
				if rel == 0 && !e1.Version().IsEqual(e2.Version()) {
					isConcurrent = true
					break
				}
			}
		}
		if isConcurrent {
			concurrent = append(concurrent, e1)
		} else {
			ordered = append(ordered, e1)
		}
	}

	// If there are no concurrent events, return ordered events
	if len(concurrent) == 0 {
		return ordered, nil
	}

	// Handle concurrent events based on event type
	switch concurrent[0].Type() {
	case EventTypeCounterCreated:
		// Keep the event with the highest client ID as a tiebreaker
		resolved := concurrent[0]
		for _, event := range concurrent[1:] {
			if event.clientID > resolved.clientID {
				resolved = event
			}
		}
		return []*CounterEvent{resolved}, nil

	case EventTypeCounterIncremented, EventTypeCounterDecremented:
		// For increments/decrements, combine all concurrent changes
		total := 0
		for _, event := range concurrent {
			if event.Type() == EventTypeCounterIncremented {
				total += event.value
			} else {
				total -= event.value
			}
		}

		// Create a new event that combines the concurrent changes
		mergedVC := version.NewVectorClock()
		for _, event := range concurrent {
			if err := mergedVC.Merge(event.Version()); err != nil {
				return nil, fmt.Errorf("failed to merge vector clocks: %w", err)
			}
		}

		combined := &CounterEvent{
			id:        fmt.Sprintf("combined-%s", concurrent[0].id),
			eventType: EventTypeCounterIncremented,
			counterID: concurrent[0].counterID,
			value:     total,
			timestamp: concurrent[0].timestamp, // Use earliest timestamp
			clientID:  "resolver",              // Mark as resolved
			version:   mergedVC,
			metadata: map[string]interface{}{
				"resolved_from": len(concurrent),
			},
		}

		return []*CounterEvent{combined}, nil

	case EventTypeCounterReset:
		// For resets, take the latest reset value based on timestamp
		resolved := concurrent[0]
		for _, event := range concurrent[1:] {
			if event.timestamp.After(resolved.timestamp) {
				resolved = event
			}
		}
		return []*CounterEvent{resolved}, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", concurrent[0].Type())
	}
}
