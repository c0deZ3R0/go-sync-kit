package sync

import (
	"context"
	"testing"
	"time"
)

func TestMetricsCollection(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func() (*TestEventStore, *TestTransport)
		operation  func(SyncManager) error
		assertions func(*testing.T, *MockMetricsCollector)
	}{
		{
			name: "successful sync records duration and events",
			setupMocks: func() (*TestEventStore, *TestTransport) {
				return &TestEventStore{}, &TestTransport{}
			},
			operation: func(sm SyncManager) error {
				_, err := sm.Sync(context.Background())
				return err
			},
			assertions: func(t *testing.T, mc *MockMetricsCollector) {
				if len(mc.DurationCalls) == 0 {
					t.Error("Expected duration metric to be recorded")
				}

				foundFullSync := false
				for _, call := range mc.DurationCalls {
					if call.Operation == "full_sync" {
						foundFullSync = true
						break
					}
				}
				if !foundFullSync {
					t.Error("Expected full_sync operation to be recorded")
				}
			},
		},
		{
			name: "failed sync records error metrics",
			setupMocks: func() (*TestEventStore, *TestTransport) {
				return &TestEventStore{}, &TestTransport{shouldError: true}
			},
			operation: func(sm SyncManager) error {
				_, err := sm.Sync(context.Background())
				return err
			},
			assertions: func(t *testing.T, mc *MockMetricsCollector) {
				foundError := false
				for _, call := range mc.ErrorCalls {
					if call.Operation == "pull" && call.ErrorType == "pull_failure" {
						foundError = true
						break
					}
				}
				if !foundError {
					t.Error("Expected error metric to be recorded")
				}
			},
		},
		{
			name: "successful push records push metrics",
			setupMocks: func() (*TestEventStore, *TestTransport) {
				store := &TestEventStore{}
				store.events = []EventWithVersion{
					{Event: &TestEvent{}, Version: &TestVersion{}},
				}
				return store, &TestTransport{}
			},
			operation: func(sm SyncManager) error {
				_, err := sm.Push(context.Background())
				return err
			},
			assertions: func(t *testing.T, mc *MockMetricsCollector) {
				foundPush := false
				for _, call := range mc.DurationCalls {
					if call.Operation == "push" {
						foundPush = true
						break
					}
				}
				if !foundPush {
					t.Error("Expected push operation metrics to be recorded")
				}

				foundEvents := false
				for _, call := range mc.EventCalls {
					if call.Pushed > 0 {
						foundEvents = true
						break
					}
				}
				if !foundEvents {
					t.Error("Expected push events to be recorded")
				}
			},
		},
		{
			name: "successful pull records pull metrics",
			setupMocks: func() (*TestEventStore, *TestTransport) {
				store := &TestEventStore{}
				transport := &TestTransport{}
				return store, transport
			},
			operation: func(sm SyncManager) error {
				_, err := sm.Pull(context.Background())
				return err
			},
			assertions: func(t *testing.T, mc *MockMetricsCollector) {
				foundPull := false
				for _, call := range mc.DurationCalls {
					if call.Operation == "pull" {
						foundPull = true
						break
					}
				}
				if !foundPull {
					t.Error("Expected pull operation metrics to be recorded")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, transport := tt.setupMocks()
			metrics := &MockMetricsCollector{}

			sm := NewSyncManager(store, transport, &SyncOptions{
				MetricsCollector: metrics,
				BatchSize:        100,
			})

			err := tt.operation(sm)
			if err != nil && tt.name != "failed sync records error metrics" {
				t.Errorf("Unexpected error: %v", err)
			}

			tt.assertions(t, metrics)
		})
	}
}

func TestNoOpMetricsCollector(t *testing.T) {
	noOp := &NoOpMetricsCollector{}

	// Verify that none of these operations panic
	noOp.RecordSyncDuration("test", time.Second)
	noOp.RecordSyncEvents(1, 2)
	noOp.RecordSyncErrors("test", "error")
	noOp.RecordConflicts(3)
}
