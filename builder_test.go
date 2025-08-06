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

	t.Run("fails with negative batch size", func(t *testing.T) {
		_, err := NewSyncManagerBuilder().
			WithStore(store).
			WithTransport(transport).
			WithBatchSize(-1).
			Build()

		if err == nil {
			t.Error("expected error when batch size is negative")
		}
	})

	t.Run("configures validation and compression options", func(t *testing.T) {
		timeout := 30 * time.Second

		manager, err := NewSyncManagerBuilder().
			WithStore(store).
			WithTransport(transport).
			WithValidation().
			WithTimeout(timeout).
			WithCompression(true).
			Build()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if manager == nil {
			t.Error("expected manager to be created")
		}

		// Type assert to access internal fields
		sm := manager.(*syncManager)
		if !sm.options.EnableValidation {
			t.Error("validation not enabled")
		}
		if sm.options.Timeout != timeout {
			t.Error("timeout not set correctly")
		}
		if !sm.options.EnableCompression {
			t.Error("compression not enabled")
		}
	})

	t.Run("resets builder state", func(t *testing.T) {
		builder := NewSyncManagerBuilder().
			WithStore(store).
			WithTransport(transport).
			WithValidation().
			WithTimeout(30 * time.Second).
			WithCompression(true).
			WithBatchSize(200).
			WithPushOnly()

		builder.Reset()

		// After reset, building should fail due to missing required components
		_, err := builder.Build()
		if err == nil {
			t.Error("expected error after reset due to missing components")
		}

		// Verify options are reset to defaults
		if builder.options.BatchSize != 100 {
			t.Error("batch size not reset to default")
		}
		if builder.options.EnableValidation {
			t.Error("validation not reset to false")
		}
		if builder.options.Timeout != 0 {
			t.Error("timeout not reset to 0")
		}
		if builder.options.EnableCompression {
			t.Error("compression not reset to false")
		}
		if builder.pushOnlySet {
			t.Error("pushOnlySet not reset to false")
		}
	})
}
