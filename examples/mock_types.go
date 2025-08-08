package main

import (
	"strconv"
	"time"

sync_pkg "github.com/c0deZ3R0/go-sync-kit/sync"
)

// MockEvent implements the Event interface for testing
type MockEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        interface{}
	metadata    map[string]interface{}
}

func NewMockEvent(id, eventType, aggregateID string, data interface{}) *MockEvent {
	return &MockEvent{
		id:          id,
		eventType:   eventType,
		aggregateID: aggregateID,
		data:        data,
		metadata:    make(map[string]interface{}),
	}
}

func (e *MockEvent) ID() string                           { return e.id }
func (e *MockEvent) Type() string                        { return e.eventType }
func (e *MockEvent) AggregateID() string                 { return e.aggregateID }
func (e *MockEvent) Data() interface{}                   { return e.data }
func (e *MockEvent) Metadata() map[string]interface{}    { return e.metadata }

// MockVersion implements the Version interface for testing
type MockVersion struct {
	timestamp time.Time
}

func NewMockVersion(t time.Time) *MockVersion {
	return &MockVersion{timestamp: t}
}

func (v *MockVersion) Compare(other sync_pkg.Version) int {
	if other == nil {
		return 1 // non-nil is greater than nil
	}
	otherV, ok := other.(*MockVersion)
	if !ok {
		return 0 // incomparable across different version types
	}
	if v.timestamp.Equal(otherV.timestamp) {
		return 0
	}
	if v.timestamp.Before(otherV.timestamp) {
		return -1
	}
	return 1
}

func (v *MockVersion) String() string {
	return strconv.FormatInt(v.timestamp.UnixNano(), 10)
}

func (v *MockVersion) IsZero() bool {
	return v.timestamp.IsZero()
}
