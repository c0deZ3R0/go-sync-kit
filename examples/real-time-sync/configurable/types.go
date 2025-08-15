package main

import (
	"time"

	sync "github.com/c0deZ3R0/go-sync-kit/synckit"
)

type DocumentData struct {
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Tags      []string               `json:"tags"`
	Status    string                 `json:"status"` // draft -> review -> published
	Author    string                 `json:"author"`
	UpdatedAt time.Time              `json:"updated_at"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

type DocumentEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        DocumentData
	metadata    map[string]interface{}
}

func (e *DocumentEvent) ID() string                       { return e.id }
func (e *DocumentEvent) Type() string                     { return e.eventType }
func (e *DocumentEvent) AggregateID() string              { return e.aggregateID }
func (e *DocumentEvent) Data() interface{}                { return e.data }
func (e *DocumentEvent) Metadata() map[string]interface{} { return e.metadata }

// helper
func ewv(ev sync.Event) sync.EventWithVersion {
	return sync.EventWithVersion{Event: ev, Version: nil} // version assigned by VersionedStore
}
