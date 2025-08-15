package main

import (
	"context"
	"fmt"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	std_sync "sync"
)

// MemoryEventStore is a simple in-memory implementation of EventStore
type MemoryEventStore struct {
	mu     std_sync.RWMutex
	events []synckit.EventWithVersion
}

func NewMemoryEventStore() synckit.EventStore {
	return &MemoryEventStore{
		events: make([]synckit.EventWithVersion, 0),
	}
}

func (s *MemoryEventStore) Store(_ context.Context, event synckit.Event, version synckit.Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, synckit.EventWithVersion{
		Event:   event,
		Version: version,
	})
	return nil
}

func (s *MemoryEventStore) Load(_ context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if since == nil {
		return s.events, nil
	}

	var result []synckit.EventWithVersion
	for _, ev := range s.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (s *MemoryEventStore) LoadByAggregate(_ context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []synckit.EventWithVersion
	for _, ev := range s.events {
		if ev.Event.AggregateID() == aggregateID {
			if since == nil || ev.Version.Compare(since) > 0 {
				result = append(result, ev)
			}
		}
	}
	return result, nil
}

func (s *MemoryEventStore) LatestVersion(_ context.Context) (synckit.Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.events) == 0 {
		return nil, nil
	}

	latest := s.events[0].Version
	for i := 1; i < len(s.events); i++ {
		if s.events[i].Version.Compare(latest) > 0 {
			latest = s.events[i].Version
		}
	}
	return latest, nil
}

func (s *MemoryEventStore) ParseVersion(_ context.Context, versionStr string) (synckit.Version, error) {
	// This is just an example - implement proper version parsing based on your version type
	return nil, fmt.Errorf("version parsing not implemented in memory store")
}

func (s *MemoryEventStore) Close() error {
	return nil // Nothing to close for in-memory store
}
