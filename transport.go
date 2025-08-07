package sync

import (
    "context"

    "github.com/c0deZ3R0/go-sync-kit/cursor"
)

// CursorTransport extends Transport with cursor-based operations
type CursorTransport interface {
    PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]EventWithVersion, cursor.Cursor, error)
}
