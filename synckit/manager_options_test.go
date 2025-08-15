package synckit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager_BasicUsage(t *testing.T) {
	// Create a mock store for testing
	mockStore := &mockEventStore{}
	mockTransport := &mockTransport{}

	mgr, err := NewManager(
		WithStore(mockStore),
		WithTransport(mockTransport),
		WithLWW(),
		WithBatchSize(100),
		WithTimeout(30*time.Second),
	)

	require.NoError(t, err)
	assert.NotNil(t, mgr)
}

func TestNewManager_RequiresStore(t *testing.T) {
	mockTransport := &mockTransport{}

	_, err := NewManager(
		WithTransport(mockTransport),
		WithLWW(),
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store is required")
}

func TestNewManager_WithSQLiteReturnsError(t *testing.T) {
	// WithSQLite should now return an error to avoid import cycles
	mockTransport := &mockTransport{}

	_, err := NewManager(
		WithSQLite("test.db"),
		WithTransport(mockTransport),
		WithLWW(),
	)

	// Should get an error about import cycles
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "import cycle")
}

func TestWithHTTPTransport(t *testing.T) {
	// WithHTTPTransport should now return an error to avoid import cycles
	mockStore := &mockEventStore{}

	_, err := NewManager(
		WithStore(mockStore),
		WithHTTPTransport("http://localhost:8080/sync"),
		WithLWW(),
	)

	// Should get an error about import cycles
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "import cycle")
}

func TestNewManager_AllOptions(t *testing.T) {
	mockStore := &mockEventStore{}
	mockTransport := &mockTransport{}

	mgr, err := NewManager(
		WithStore(mockStore),
		WithTransport(mockTransport),
		WithLWW(),
		WithBatchSize(50),
		WithTimeout(15*time.Second),
		WithSyncInterval(5*time.Minute),
		WithFilter(func(e Event) bool { return e.Type() == "important" }),
		WithValidation(),
		WithCompression(true),
	)

	require.NoError(t, err)
	assert.NotNil(t, mgr)
}

func TestNewManager_PushPullOptions(t *testing.T) {
	mockStore := &mockEventStore{}
	mockTransport := &mockTransport{}

	t.Run("WithPushOnly", func(t *testing.T) {
		mgr, err := NewManager(
			WithStore(mockStore),
			WithTransport(mockTransport),
			WithPushOnly(),
			WithLWW(),
		)

		require.NoError(t, err)
		assert.NotNil(t, mgr)
	})

	t.Run("WithPullOnly", func(t *testing.T) {
		mgr, err := NewManager(
			WithStore(mockStore),
			WithTransport(mockTransport),
			WithPullOnly(),
			WithLWW(),
		)

		require.NoError(t, err)
		assert.NotNil(t, mgr)
	})
}

func TestNewManager_ConflictResolvers(t *testing.T) {
	mockStore := &mockEventStore{}
	mockTransport := &mockTransport{}

	tests := []struct {
		name   string
		option ManagerOption
	}{
		{"WithLWW", WithLWW()},
		{"WithFWW", WithFWW()},
		{"WithAdditiveMerge", WithAdditiveMerge()},
		{"Custom resolver", WithConflictResolver(&LastWriteWinsResolver{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(
				WithStore(mockStore),
				WithTransport(mockTransport),
				tt.option,
			)

			require.NoError(t, err)
			assert.NotNil(t, mgr)
		})
	}
}

func TestNewManager_BuilderCompatibility(t *testing.T) {
	// Test that the new functional options work alongside the existing builder
	mockStore := &mockEventStore{}
	mockTransport := &mockTransport{}

	// Create using builder (existing way)
	builderMgr, err := NewSyncManagerBuilder().
		WithStore(mockStore).
		WithTransport(mockTransport).
		WithConflictResolver(&LastWriteWinsResolver{}).
		WithBatchSize(100).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, builderMgr)

	// Create using functional options (new way)
	optionsMgr, err := NewManager(
		WithStore(mockStore),
		WithTransport(mockTransport),
		WithLWW(),
		WithBatchSize(100),
	)

	require.NoError(t, err)
	assert.NotNil(t, optionsMgr)

	// Both should work and be non-nil
	assert.NotNil(t, builderMgr)
	assert.NotNil(t, optionsMgr)
}

func TestNewManager_WithNullTransport(t *testing.T) {
	store := &mockEventStore{}

	// Test that WithNullTransport provides a working no-op transport
	mgr, err := NewManager(
		WithStore(store),
		WithNullTransport(),
		WithLWW(),
	)
	require.NoError(t, err)
	require.NotNil(t, mgr)

	// Test that sync operations work with null transport (should be no-op)
	result, err := mgr.Sync(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, result.EventsPushed)
	require.Equal(t, 0, result.EventsPulled)
	require.Equal(t, 0, result.ConflictsResolved)
}
