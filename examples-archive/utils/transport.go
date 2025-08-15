package sync

import (
	"context"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// CursorTransport extends Transport with cursor-based pagination support.
type CursorTransport interface {
	Transport

	// PullWithCursor retrieves events using a cursor-based strategy.
	// The cursor represents the last seen position in the event stream.
	// Returns the events, the next cursor position, and any error.
	PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]EventWithVersion, cursor.Cursor, error)
}
