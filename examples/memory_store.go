package main

import (
	"context"
	"fmt"
	std_sync "sync"
	sync_pkg "github.com/c0deZ3R0/go-sync-kit-agent2/sync"
)

// MemoryEventStore is a simple in-memory implementation of EventStore
type MemoryEventStore struct {
	mu     std_sync.RWMutex
	events []sync_pkg.EventWithVersion
}

func NewMemoryEventStore() sync_pkg.EventStore {
	return &MemoryEventStore{
		events: make([]sync_pkg.EventWithVersion, 0),
	}
}

func (s *MemoryEventStore) Store(_ context.Context, event sync_pkg.Event, version sync_pkg.Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, sync_pkg.EventWithVersion{
		Event:   event,
		Version: version,
	})
	return nil
}

func (s *MemoryEventStore) Load(_ context.Context, since sync_pkg.Version) ([]sync_pkg.EventWithVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if since == nil {
		return s.events, nil
	}

var result []sync_pkg.EventWithVersion
	for _, ev := range s.events {
		if ev.Version.Compare(since) > 0 {
			result = append(result, ev)
		}
	}
	return result, nil
}

func (s *MemoryEventStore) LoadByAggregate(_ context.Context, aggregateID string, since sync_pkg.Version) ([]sync_pkg.EventWithVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

var result []sync_pkg.EventWithVersion
	for _, ev := range s.events {
		if ev.Event.AggregateID() == aggregateID {
			if since == nil || ev.Version.Compare(since) > 0 {
				result = append(result, ev)
			}
		}
	}
	return result, nil
}

func (s *MemoryEventStore) LatestVersion(_ context.Context) (sync_pkg.Version, error) {
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

func (s *MemoryEventStore) ParseVersion(_ context.Context, versionStr string) (sync_pkg.Version, error) {
	// This is just an example - implement proper version parsing based on your version type
	return nil, fmt.Errorf("version parsing not implemented in memory store")
}

func (s *MemoryEventStore) Close() error {
	return nil // Nothing to close for in-memory store
}
