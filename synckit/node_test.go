package synckit

import (
	"context"
	"testing"
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

		// Verify it implements SyncNode interface (which is SyncManager)
		var _ SyncNode = node
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

	t.Run("works with all lifecycle methods", func(t *testing.T) {
		store := &TestEventStore{}
		transport := &TestTransport{}

		node, err := NewNode(
			WithStore(store),
			WithTransport(transport),
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

		// Verify it implements SyncNode interface
		var _ SyncNode = node
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

		// Verify it implements SyncNode interface
		var _ SyncNode = node
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

		// Verify it implements SyncNode interface
		var _ SyncNode = node
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