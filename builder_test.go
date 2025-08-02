package sync

import (
	"testing"
	"time"
)

func TestSyncManagerBuilder(t *testing.T) {
	store := &mockEventStore{}
	transport := &mockTransport{}

	t.Run("builds with required components", func(t *testing.T) {
		manager, err := NewSyncManagerBuilder().
			WithStore(store).
			WithTransport(transport).
			Build()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if manager == nil {
			t.Error("expected manager to be created")
		}
	})

	t.Run("prevents invalid push/pull configuration", func(t *testing.T) {
		_, err := NewSyncManagerBuilder().
			WithStore(store).
			WithTransport(transport).
			WithPushOnly().
			WithPullOnly().
			Build()

		if err == nil {
			t.Error("expected error when both push only and pull only are set")
		}
	})

	t.Run("fails without store", func(t *testing.T) {
		_, err := NewSyncManagerBuilder().
			WithTransport(transport).
			Build()

		if err == nil {
			t.Error("expected error when store is missing")
		}
	})

	t.Run("fails without transport", func(t *testing.T) {
		_, err := NewSyncManagerBuilder().
			WithStore(store).
			Build()

		if err == nil {
			t.Error("expected error when transport is missing")
		}
	})

	t.Run("configures all options", func(t *testing.T) {
		interval := 5 * time.Second
		resolver := &mockConflictResolver{}
		filter := func(e Event) bool { return true }

		manager, err := NewSyncManagerBuilder().
			WithStore(store).
			WithTransport(transport).
			WithBatchSize(200).
			WithPushOnly().
			WithConflictResolver(resolver).
			WithFilter(filter).
			WithSyncInterval(interval).
			Build()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if manager == nil {
			t.Error("expected manager to be created")
		}

		// Type assert to access internal fields
		sm := manager.(*syncManager)
		if sm.options.BatchSize != 200 {
			t.Error("batch size not set correctly")
		}
		if !sm.options.PushOnly {
			t.Error("push only not set correctly")
		}
		if sm.options.ConflictResolver != resolver {
			t.Error("conflict resolver not set correctly")
		}
		if sm.options.Filter == nil {
			t.Error("filter not set correctly")
		}
		if sm.options.SyncInterval != interval {
			t.Error("sync interval not set correctly")
		}
	})
}
