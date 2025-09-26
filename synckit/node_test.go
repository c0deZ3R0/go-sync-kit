package synckit

import (
	"context"
	"testing"
	"time"
)

func TestNewNode(t *testing.T) {
	t.Run("creates node with basic options", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{}

		node, err := NewNode(
			WithStore(store),
			WithTransport(transport),
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if node == nil {
			t.Fatal("Expected node to be non-nil")
		}

		// Verify it implements SyncNode interface (ensured by compile-time check in node.go)
	})

	t.Run("fails without store", func(t *testing.T) {
		transport := &TestTransport{}

		_, err := NewNode(
			WithTransport(transport),
		)

		if err == nil {
			t.Fatal("Expected error when store is missing")
		}
	})

	t.Run("fails without transport", func(t *testing.T) {
		store := &TestEventStore{}

		_, err := NewNode(
			WithStore(store),
		)

		if err == nil {
			t.Fatal("Expected error when transport is missing")
		}
	})

	t.Run("works with all lifecycle methods", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{}

		node, err := NewNode(
			WithStore(store),
			WithTransport(transport),
			WithSyncInterval(time.Second), // Required for StartAutoSync
		)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		ctx := context.Background()

		// Test sync operations
		result, err := node.Sync(ctx)
		if err != nil {
			t.Errorf("Sync failed: %v", err)
		}
		if result == nil {
			t.Error("Expected sync result to be non-nil")
		}

		// Test push operation
		pushResult, err := node.Push(ctx)
		if err != nil {
			t.Errorf("Push failed: %v", err)
		}
		if pushResult == nil {
			t.Error("Expected push result to be non-nil")
		}

		// Test pull operation
		pullResult, err := node.Pull(ctx)
		if err != nil {
			t.Errorf("Pull failed: %v", err)
		}
		if pullResult == nil {
			t.Error("Expected pull result to be non-nil")
		}

		// Test auto sync lifecycle
		if err := node.StartAutoSync(ctx); err != nil {
			t.Errorf("StartAutoSync failed: %v", err)
		}

		if err := node.StopAutoSync(); err != nil {
			t.Errorf("StopAutoSync failed: %v", err)
		}

		// Test subscription
		subscribeErr := node.Subscribe(func(result *SyncResult) {
			// Simple callback for testing
		})
		if subscribeErr != nil {
			t.Errorf("Subscribe failed: %v", subscribeErr)
		}

		// Test close
		if err := node.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}

func TestNewInMemoryNode(t *testing.T) {
	t.Run("creates node with valid parameters", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{}

		node, err := NewInMemoryNode(store, transport)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if node == nil {
			t.Fatal("Expected node to be non-nil")
		}

		// Verify it implements SyncNode interface (ensured by compile-time check in node.go)
	})

	t.Run("fails with nil store", func(t *testing.T) {
		transport := &TestTransport{}

		_, err := NewInMemoryNode(nil, transport)

		if err == nil {
			t.Fatal("Expected error when store is nil")
		}

		if err.Error() != "store cannot be nil" {
			t.Errorf("Expected specific error message, got %v", err)
		}
	})

	t.Run("fails with nil transport", func(t *testing.T) {
		store := &TestEventStore{}

		_, err := NewInMemoryNode(store, nil)

		if err == nil {
			t.Fatal("Expected error when transport is nil")
		}

		if err.Error() != "transport cannot be nil" {
			t.Errorf("Expected specific error message, got %v", err)
		}
	})

	t.Run("fails with both nil parameters", func(t *testing.T) {
		_, err := NewInMemoryNode(nil, nil)

		if err == nil {
			t.Fatal("Expected error when both parameters are nil")
		}

		// Should fail on the first check (store)
		if err.Error() != "store cannot be nil" {
			t.Errorf("Expected store error first, got %v", err)
		}
	})

	t.Run("works with sync operations", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{}

		node, err := NewInMemoryNode(store, transport)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		ctx := context.Background()
		result, err := node.Sync(ctx)
		if err != nil {
			t.Errorf("Sync failed: %v", err)
		}
		if result == nil {
			t.Error("Expected sync result to be non-nil")
		}

		// Clean up
		if err := node.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}

func TestNewHTTPServerNode(t *testing.T) {
	t.Run("creates node with valid parameters", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{} // Using test transport instead of HTTP

		node, err := NewHTTPServerNode(store, transport)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if node == nil {
			t.Fatal("Expected node to be non-nil")
		}

		// Verify it implements SyncNode interface (ensured by compile-time check in node.go)
	})

	t.Run("fails with nil store", func(t *testing.T) {
		transport := &TestTransport{}

		_, err := NewHTTPServerNode(nil, transport)

		if err == nil {
			t.Fatal("Expected error when store is nil")
		}

		if err.Error() != "store cannot be nil" {
			t.Errorf("Expected specific error message, got %v", err)
		}
	})

	t.Run("fails with nil transport", func(t *testing.T) {
		store := &TestEventStore{}

		_, err := NewHTTPServerNode(store, nil)

		if err == nil {
			t.Fatal("Expected error when transport is nil")
		}

		if err.Error() != "transport cannot be nil" {
			t.Errorf("Expected specific error message, got %v", err)
		}
	})
}

func TestNewHTTPClientNode(t *testing.T) {
	t.Run("creates node with valid parameters", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{} // Using test transport instead of HTTP

		node, err := NewHTTPClientNode(store, transport)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if node == nil {
			t.Fatal("Expected node to be non-nil")
		}

		// Verify it implements SyncNode interface (ensured by compile-time check in node.go)
	})

	t.Run("fails with nil store", func(t *testing.T) {
		transport := &TestTransport{}

		_, err := NewHTTPClientNode(nil, transport)

		if err == nil {
			t.Fatal("Expected error when store is nil")
		}

		if err.Error() != "store cannot be nil" {
			t.Errorf("Expected specific error message, got %v", err)
		}
	})

	t.Run("fails with nil transport", func(t *testing.T) {
		store := &TestEventStore{}

		_, err := NewHTTPClientNode(store, nil)

		if err == nil {
			t.Fatal("Expected error when transport is nil")
		}

		if err.Error() != "transport cannot be nil" {
			t.Errorf("Expected specific error message, got %v", err)
		}
	})
}

// TestSyncNodeLifecycle ensures that SyncNode correctly exposes lifecycle methods
// (StartAutoSync, StopAutoSync, Close, etc.) identical to SyncManager.
// This test locks in behavior to prevent regressions if we ever switch from type alias to wrapper struct.
func TestSyncNodeLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a simple in-memory node with sync interval for StartAutoSync
	node, err := NewNode(
		WithStore(&TestEventStore{}),
		WithTransport(&TestTransport{}),
		WithSyncInterval(100*time.Millisecond), // Required for StartAutoSync
	)
	if err != nil {
		t.Fatalf("failed to create SyncNode: %v", err)
	}


	// Start auto sync
	if err := node.StartAutoSync(ctx); err != nil {
		t.Errorf("StartAutoSync failed: %v", err)
	}

	// Give it a brief moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop auto sync
	if err := node.StopAutoSync(); err != nil {
		t.Errorf("StopAutoSync failed: %v", err)
	}

	// Subscribe should work (testing another lifecycle method)
	if err := node.Subscribe(func(result *SyncResult) {
		// Simple callback for testing
	}); err != nil {
		t.Errorf("Subscribe failed: %v", err)
	}

	// Close should work without panic
	if err := node.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// This test ensures that if SyncNode is ever changed from a type alias
	// to a wrapper struct, all lifecycle methods must be properly forwarded.
}

// TestSyncNodeManagerIdenticalBehavior verifies that SyncNode behaves identically to SyncManager.
// This test will catch any behavioral differences if SyncNode is ever changed from type alias to wrapper.
func TestSyncNodeManagerIdenticalBehavior(t *testing.T) {
	ctx := context.Background()

	// Create identical configurations
	store1, store2 := &TestEventStore{}, &TestEventStore{}
	transport1, transport2 := &TestTransport{}, &TestTransport{}

	// Create SyncManager (current implementation)
	manager, err := NewManager(
		WithStore(store1),
		WithTransport(transport1),
	)
	if err != nil {
		t.Fatalf("failed to create SyncManager: %v", err)
	}
	defer manager.Close()

	// Create SyncNode (façade)
	node, err := NewNode(
		WithStore(store2),
		WithTransport(transport2),
	)
	if err != nil {
		t.Fatalf("failed to create SyncNode: %v", err)
	}
	defer node.Close()

	// Test Sync operation - should behave identically
	managerResult, managerErr := manager.Sync(ctx)
	nodeResult, nodeErr := node.Sync(ctx)

	// Both should succeed or fail in the same way
	if (managerErr == nil) != (nodeErr == nil) {
		t.Errorf("Sync behavior differs: manager error=%v, node error=%v", managerErr, nodeErr)
	}

	if managerErr == nil && nodeErr == nil {
		// Both succeeded - results should be comparable
		if managerResult.EventsPushed != nodeResult.EventsPushed {
			t.Errorf("EventsPushed differs: manager=%d, node=%d", 
				managerResult.EventsPushed, nodeResult.EventsPushed)
		}
		if managerResult.EventsPulled != nodeResult.EventsPulled {
			t.Errorf("EventsPulled differs: manager=%d, node=%d", 
				managerResult.EventsPulled, nodeResult.EventsPulled)
		}
	}

	// Test Push operation - should behave identically
	managerPush, managerPushErr := manager.Push(ctx)
	nodePush, nodePushErr := node.Push(ctx)

	if (managerPushErr == nil) != (nodePushErr == nil) {
		t.Errorf("Push behavior differs: manager error=%v, node error=%v", managerPushErr, nodePushErr)
	}

	if managerPushErr == nil && nodePushErr == nil {
		if managerPush.EventsPushed != nodePush.EventsPushed {
			t.Errorf("Push EventsPushed differs: manager=%d, node=%d", 
				managerPush.EventsPushed, nodePush.EventsPushed)
		}
	}

	// Test Pull operation - should behave identically
	managerPull, managerPullErr := manager.Pull(ctx)
	nodePull, nodePullErr := node.Pull(ctx)

	if (managerPullErr == nil) != (nodePullErr == nil) {
		t.Errorf("Pull behavior differs: manager error=%v, node error=%v", managerPullErr, nodePullErr)
	}

	if managerPullErr == nil && nodePullErr == nil {
		if managerPull.EventsPulled != nodePull.EventsPulled {
			t.Errorf("Pull EventsPulled differs: manager=%d, node=%d", 
				managerPull.EventsPulled, nodePull.EventsPulled)
		}
	}

	// This test ensures SyncNode and SyncManager are behaviorally identical.
	// If SyncNode becomes a wrapper struct, this test will verify all methods
	// are properly forwarded with identical behavior.
}
