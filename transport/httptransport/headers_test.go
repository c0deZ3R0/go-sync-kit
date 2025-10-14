package httptransport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// Test ExtractTenant from header
func TestExtractTenant_FromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/pull?since=0", nil)
	req.Header.Set(HeaderSyncKitTenant, "tenant-123")

	tenant := ExtractTenant(req)
	assert.Equal(t, "tenant-123", tenant)
}

// Test ExtractTenant from query param
func TestExtractTenant_FromQueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/pull?since=0&tenant=tenant-456", nil)

	tenant := ExtractTenant(req)
	assert.Equal(t, "tenant-456", tenant)
}

// Test ExtractTenant priority (header over query param)
func TestExtractTenant_PriorityHeaderOverQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/pull?since=0&tenant=tenant-query", nil)
	req.Header.Set(HeaderSyncKitTenant, "tenant-header")

	tenant := ExtractTenant(req)
	// Header should take priority
	assert.Equal(t, "tenant-header", tenant)
}

// Test ExtractTenant when not present
func TestExtractTenant_NotPresent(t *testing.T) {
	req := httptest.NewRequest("GET", "/pull?since=0", nil)

	tenant := ExtractTenant(req)
	assert.Equal(t, "", tenant)
}

// Test pull with tenant header filtering
func TestSyncHandler_HandlePull_WithTenantHeader(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	// Store events with different tenants in metadata
	_ = store.Store(ctx, &MockEvent{
		id:          "1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "data1",
		metadata:    map[string]interface{}{"tenant": "acme-corp"},
	}, cursor.IntegerCursor{Seq: 1})

	_ = store.Store(ctx, &MockEvent{
		id:          "2",
		eventType:   "TestEvent",
		aggregateID: "agg-2",
		data:        "data2",
		metadata:    map[string]interface{}{"tenant": "widgets-inc"},
	}, cursor.IntegerCursor{Seq: 2})

	_ = store.Store(ctx, &MockEvent{
		id:          "3",
		eventType:   "TestEvent",
		aggregateID: "agg-3",
		data:        "data3",
		metadata:    map[string]interface{}{"tenant": "acme-corp"},
	}, cursor.IntegerCursor{Seq: 3})

	handler := NewSyncHandler(store, slog.Default(), nil, DefaultServerOptions())

	// Pull with tenant header
	req := httptest.NewRequest("GET", "/pull?since=0", nil)
	req.Header.Set(HeaderSyncKitTenant, "acme-corp")
	w := httptest.NewRecorder()

	handler.handlePull(w, req)

	assert.Equal(t, 200, w.Code)

	var jsonEvents []JSONEventWithVersion
	require.NoError(t, json.NewDecoder(w.Body).Decode(&jsonEvents))

	// Should only return events for acme-corp tenant
	assert.Equal(t, 2, len(jsonEvents))
	for _, evt := range jsonEvents {
		meta := evt.Event.Metadata
		assert.NotNil(t, meta)
		assert.Equal(t, "acme-corp", meta["tenant"])
	}
}

// Test pull with tenant query param filtering
func TestSyncHandler_HandlePull_WithTenantQueryParam(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	// Store events with different tenants
_ = store.Store(ctx, &MockEvent{
		id:          "1",
		eventType:   "TestEvent",
		aggregateID: "agg-1",
		data:        "data1",
		metadata:    map[string]interface{}{"tenant": "tenant-a"},
	}, cursor.IntegerCursor{Seq: 1})

_ = store.Store(ctx, &MockEvent{
		id:          "2",
		eventType:   "TestEvent",
		aggregateID: "agg-2",
		data:        "data2",
		metadata:    map[string]interface{}{"tenant": "tenant-b"},
	}, cursor.IntegerCursor{Seq: 2})

	handler := NewSyncHandler(store, slog.Default(), nil, DefaultServerOptions())

	// Pull with tenant query param
	req := httptest.NewRequest("GET", "/pull?since=0&tenant=tenant-a", nil)
	w := httptest.NewRecorder()

	handler.handlePull(w, req)

	assert.Equal(t, 200, w.Code)

	var jsonEvents []JSONEventWithVersion
	require.NoError(t, json.NewDecoder(w.Body).Decode(&jsonEvents))

	// Should only return events for tenant-a
	assert.Equal(t, 1, len(jsonEvents))
	assert.Equal(t, "tenant-a", jsonEvents[0].Event.Metadata["tenant"])
}

// Test tenant isolation - ensure tenants can't see each other's data
func TestSyncHandler_TenantIsolation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	// Store events for multiple tenants
	for i := 1; i <= 5; i++ {
		for _, tenant := range []string{"tenant-1", "tenant-2", "tenant-3"} {
			_ = store.Store(ctx, &MockEvent{
				id:          tenant + "-event-" + string(rune(i)),
				eventType:   "TestEvent",
				aggregateID: "agg",
				data:        "data",
				metadata:    map[string]interface{}{"tenant": tenant},
			}, cursor.IntegerCursor{Seq: uint64(i)})
		}
	}

	handler := NewSyncHandler(store, slog.Default(), nil, DefaultServerOptions())

	// Each tenant should only see their own events
	for _, tenant := range []string{"tenant-1", "tenant-2", "tenant-3"} {
		req := httptest.NewRequest("GET", "/pull?since=0", nil)
		req.Header.Set(HeaderSyncKitTenant, tenant)
		w := httptest.NewRecorder()

		handler.handlePull(w, req)

		assert.Equal(t, 200, w.Code)

		var jsonEvents []JSONEventWithVersion
		require.NoError(t, json.NewDecoder(w.Body).Decode(&jsonEvents))

		// Each tenant should have 5 events
		assert.Equal(t, 5, len(jsonEvents), "Tenant %s should have 5 events", tenant)

		// Verify all events belong to this tenant
		for _, evt := range jsonEvents {
			assert.Equal(t, tenant, evt.Event.Metadata["tenant"], "Event should belong to tenant %s", tenant)
		}
	}
}
