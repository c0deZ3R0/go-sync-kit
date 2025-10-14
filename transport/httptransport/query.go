package httptransport

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/c0deZ3R0/go-sync-kit/synckit/types"
)

// PullQuery represents parsed query parameters for pull requests
type PullQuery struct {
	Since   types.Version  // Cursor position to start from
	Limit   int            // Max events to return (default 100, max 1000)
	Filters []types.Filter // Event filters (type, tenant, aggregate_id, etc.)
}

// ParsePullQuery extracts and validates query parameters from an HTTP request
func ParsePullQuery(ctx context.Context, r *http.Request, parser VersionParser) (*PullQuery, error) {
	query := &PullQuery{
		Limit:   100, // Default limit
		Filters: make([]types.Filter, 0),
	}

	// Parse 'since' cursor (default to "0" if not provided)
	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		sinceStr = "0"
	}
	version, err := parser(ctx, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid since cursor: %w", err)
	}
	query.Since = version

	// Parse 'limit'
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid limit: must be integer")
		}
		if limit <= 0 {
			return nil, fmt.Errorf("invalid limit: must be positive")
		}
		if limit > 1000 {
			return nil, fmt.Errorf("invalid limit: maximum is 1000")
		}
		query.Limit = limit
	}

	// Parse 'type' filter
	if eventType := r.URL.Query().Get("type"); eventType != "" {
		query.Filters = append(query.Filters, types.Filter{
			Key:   "type",
			Value: eventType,
		})
	}

	// Parse 'tenant' filter
	if tenant := r.URL.Query().Get("tenant"); tenant != "" {
		query.Filters = append(query.Filters, types.Filter{
			Key:   "tenant",
			Value: tenant,
		})
	}

	// Parse 'aggregate_id' filter
	if aggregateID := r.URL.Query().Get("aggregate_id"); aggregateID != "" {
		query.Filters = append(query.Filters, types.Filter{
			Key:   "aggregate_id",
			Value: aggregateID,
		})
	}

	return query, nil
}

// GetFilter retrieves a filter value by key
func GetFilter(filters []types.Filter, key string) (string, bool) {
	for _, f := range filters {
		if f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}
