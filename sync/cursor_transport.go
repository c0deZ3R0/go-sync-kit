// Deprecated: This package is deprecated. Use github.com/c0deZ3R0/go-sync-kit/synckit instead.
package sync

import (
	"context"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// CursorTransport extends Transport with cursor-based capabilities.
type CursorTransport interface {
	Transport

	// PullWithCursor retrieves events using a cursor-based pagination strategy.
	// Returns events, next cursor, and any error.
	PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]EventWithVersion, cursor.Cursor, error)
}
