package projection

import (
	"context"
	"log/slog"
	"sync"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// runner is the concrete implementation of the Runner interface.
// This is a stub implementation for Phase 1 - complete implementation in Phase 3.
type runner struct {
	store     synckit.EventStore
	offsets   OffsetStore
	projector Projector
	batchSize int
	logger    *slog.Logger
	mu        sync.Mutex
}

// ApplySince is a stub implementation - will be completed in Phase 3
func (r *runner) ApplySince(ctx context.Context) (applied int, last synckit.Version, err error) {
	// Stub implementation for Phase 1 - this will be fully implemented in Phase 3
	return 0, nil, nil
}

// ApplyBatch is a stub implementation - will be completed in Phase 3
func (r *runner) ApplyBatch(ctx context.Context, batch []synckit.EventWithVersion) error {
	// Stub implementation for Phase 1 - this will be fully implemented in Phase 3
	return nil
}
