package httptransport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// mockVersionParser is a simple version parser for testing
func mockVersionParser(ctx context.Context, s string) (types.Version, error) {
	if s == "" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}
	// Simple integer parsing for tests
	var seq uint64
	if _, err := fmt.Sscanf(s, "%d", &seq); err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}
	return cursor.IntegerCursor{Seq: seq}, nil
}

func TestParsePullQuery_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/pull", nil)

	query, err := ParsePullQuery(context.Background(), req, mockVersionParser)
	if err != nil {
		t.Fatalf("ParsePullQuery() error = %v", err)
	}

	// Check default limit
	if query.Limit != 100 {
		t.Errorf("ParsePullQuery() limit = %v, want 100", query.Limit)
	}

	// Check empty filters
	if len(query.Filters) != 0 {
		t.Errorf("ParsePullQuery() filters = %v, want empty", query.Filters)
	}

	// Check nil/zero since
	if query.Since != nil && !query.Since.IsZero() {
		t.Errorf("ParsePullQuery() since should be nil or zero")
	}
}

func TestParsePullQuery_WithSince(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/pull?since=42", nil)

	query, err := ParsePullQuery(context.Background(), req, mockVersionParser)
	if err != nil {
		t.Fatalf("ParsePullQuery() error = %v", err)
	}

	// Check since cursor
	if query.Since == nil {
		t.Fatal("ParsePullQuery() since is nil")
	}

	cursor, ok := query.Since.(cursor.IntegerCursor)
	if !ok {
		t.Fatal("ParsePullQuery() since is not IntegerCursor")
	}

	if cursor.Seq != 42 {
		t.Errorf("ParsePullQuery() since = %v, want 42", cursor.Seq)
	}
}

func TestParsePullQuery_WithLimit(t *testing.T) {
	tests := []struct {
		name      string
		limit     string
		want      int
		expectErr bool
	}{
		{
			name:      "valid limit 50",
			limit:     "50",
			want:      50,
			expectErr: false,
		},
		{
			name:      "valid limit 1000",
			limit:     "1000",
			want:      1000,
			expectErr: false,
		},
		{
			name:      "invalid - zero",
			limit:     "0",
			expectErr: true,
		},
		{
			name:      "invalid - negative",
			limit:     "-10",
			expectErr: true,
		},
		{
			name:      "invalid - exceeds max",
			limit:     "1001",
			expectErr: true,
		},
		{
			name:      "invalid - not integer",
			limit:     "abc",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/pull?limit="+tt.limit, nil)

			query, err := ParsePullQuery(context.Background(), req, mockVersionParser)

			if tt.expectErr {
				if err == nil {
					t.Errorf("ParsePullQuery() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParsePullQuery() error = %v", err)
			}

			if query.Limit != tt.want {
				t.Errorf("ParsePullQuery() limit = %v, want %v", query.Limit, tt.want)
			}
		})
	}
}

func TestParsePullQuery_WithFilters(t *testing.T) {
	tests := []struct {
		name          string
		queryString   string
		expectedCount int
		checkFilters  func(t *testing.T, filters []types.Filter)
	}{
		{
			name:          "type filter",
			queryString:   "/pull?type=OrderCreated",
			expectedCount: 1,
			checkFilters: func(t *testing.T, filters []types.Filter) {
				if filters[0].Key != "type" || filters[0].Value != "OrderCreated" {
					t.Errorf("Expected type=OrderCreated filter")
				}
			},
		},
		{
			name:          "tenant filter",
			queryString:   "/pull?tenant=acme-corp",
			expectedCount: 1,
			checkFilters: func(t *testing.T, filters []types.Filter) {
				if filters[0].Key != "tenant" || filters[0].Value != "acme-corp" {
					t.Errorf("Expected tenant=acme-corp filter")
				}
			},
		},
		{
			name:          "aggregate_id filter",
			queryString:   "/pull?aggregate_id=order-123",
			expectedCount: 1,
			checkFilters: func(t *testing.T, filters []types.Filter) {
				if filters[0].Key != "aggregate_id" || filters[0].Value != "order-123" {
					t.Errorf("Expected aggregate_id=order-123 filter")
				}
			},
		},
		{
			name:          "multiple filters",
			queryString:   "/pull?type=OrderCreated&tenant=acme-corp&aggregate_id=order-123",
			expectedCount: 3,
			checkFilters: func(t *testing.T, filters []types.Filter) {
				found := make(map[string]string)
				for _, f := range filters {
					found[f.Key] = f.Value
				}

				if found["type"] != "OrderCreated" {
					t.Errorf("Expected type=OrderCreated")
				}
				if found["tenant"] != "acme-corp" {
					t.Errorf("Expected tenant=acme-corp")
				}
				if found["aggregate_id"] != "order-123" {
					t.Errorf("Expected aggregate_id=order-123")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.queryString, nil)

			query, err := ParsePullQuery(context.Background(), req, mockVersionParser)
			if err != nil {
				t.Fatalf("ParsePullQuery() error = %v", err)
			}

			if len(query.Filters) != tt.expectedCount {
				t.Errorf("ParsePullQuery() filters count = %v, want %v", len(query.Filters), tt.expectedCount)
			}

			if tt.checkFilters != nil {
				tt.checkFilters(t, query.Filters)
			}
		})
	}
}

func TestParsePullQuery_CompleteQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/pull?since=100&limit=50&type=OrderCreated&tenant=acme-corp", nil)

	query, err := ParsePullQuery(context.Background(), req, mockVersionParser)
	if err != nil {
		t.Fatalf("ParsePullQuery() error = %v", err)
	}

	// Check since
	if query.Since == nil {
		t.Fatal("ParsePullQuery() since is nil")
	}
	if c, ok := query.Since.(cursor.IntegerCursor); !ok || c.Seq != 100 {
		t.Errorf("ParsePullQuery() since = %v, want 100", query.Since)
	}

	// Check limit
	if query.Limit != 50 {
		t.Errorf("ParsePullQuery() limit = %v, want 50", query.Limit)
	}

	// Check filters
	if len(query.Filters) != 2 {
		t.Errorf("ParsePullQuery() filters count = %v, want 2", len(query.Filters))
	}

	// Verify specific filters
	found := make(map[string]string)
	for _, f := range query.Filters {
		found[f.Key] = f.Value
	}

	if found["type"] != "OrderCreated" {
		t.Errorf("Expected type=OrderCreated filter")
	}
	if found["tenant"] != "acme-corp" {
		t.Errorf("Expected tenant=acme-corp filter")
	}
}

func TestParsePullQuery_InvalidSince(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/pull?since=invalid", nil)

	_, err := ParsePullQuery(context.Background(), req, mockVersionParser)
	if err == nil {
		t.Error("ParsePullQuery() expected error for invalid since")
	}
}

func TestGetFilter(t *testing.T) {
	filters := []types.Filter{
		{Key: "type", Value: "OrderCreated"},
		{Key: "tenant", Value: "acme-corp"},
		{Key: "aggregate_id", Value: "order-123"},
	}

	tests := []struct {
		name      string
		key       string
		wantValue string
		wantFound bool
	}{
		{
			name:      "find type",
			key:       "type",
			wantValue: "OrderCreated",
			wantFound: true,
		},
		{
			name:      "find tenant",
			key:       "tenant",
			wantValue: "acme-corp",
			wantFound: true,
		},
		{
			name:      "find aggregate_id",
			key:       "aggregate_id",
			wantValue: "order-123",
			wantFound: true,
		},
		{
			name:      "not found",
			key:       "nonexistent",
			wantValue: "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := GetFilter(filters, tt.key)

			if found != tt.wantFound {
				t.Errorf("GetFilter() found = %v, want %v", found, tt.wantFound)
			}

			if value != tt.wantValue {
				t.Errorf("GetFilter() value = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestGetFilter_EmptyFilters(t *testing.T) {
	var filters []types.Filter

	value, found := GetFilter(filters, "type")

	if found {
		t.Error("GetFilter() should not find in empty filters")
	}

	if value != "" {
		t.Errorf("GetFilter() value = %v, want empty string", value)
	}
}
