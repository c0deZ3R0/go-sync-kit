package projection

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// runner is the concrete implementation of the Runner interface.
type runner struct {
	store     synckit.EventStore
	offsets   OffsetStore
	projector Projector
	batchSize int
	logger    *slog.Logger
}

// ApplySince applies all events since the last saved offset.
// Returns the number of events applied, the last processed version, and any error.
func (r *runner) ApplySince(ctx context.Context) (applied int, last synckit.Version, err error) {
	projectionName := r.projector.Name()
	
	// Get the last applied version from the offset store
	lastVersion, err := r.offsets.Get(ctx, projectionName)
	if err != nil {
		return 0, nil, errors.E(
			errors.Op("runner.ApplySince"),
			errors.Component("projection"),
			fmt.Errorf("failed to get last offset for projection %s: %w", projectionName, err),
		)
	}

	r.logger.Debug("Starting projection from last offset",
		slog.String("projection", projectionName),
		slog.Any("last_version", lastVersion),
		slog.Int("batch_size", r.batchSize),
	)

	totalApplied := 0
	var lastProcessed synckit.Version
	currentAfter := lastVersion

	for {
		// Load the next batch of events
		allEvents, err := r.store.Load(ctx, currentAfter)
		if err != nil {
			return totalApplied, lastProcessed, errors.E(
				errors.Op("runner.ApplySince"),
				errors.Component("projection"),
				fmt.Errorf("failed to load events for projection %s: %w", projectionName, err),
			)
		}

		// Limit to batch size
		batch := allEvents
		if len(allEvents) > r.batchSize {
			batch = allEvents[:r.batchSize]
		}

		// If no more events, we're done
		if len(batch) == 0 {
			break
		}

		// Apply this batch
		if err := r.ApplyBatch(ctx, batch); err != nil {
			return totalApplied, lastProcessed, err
		}

		// Update counters and position
		totalApplied += len(batch)
		lastProcessed = batch[len(batch)-1].Version
		currentAfter = lastProcessed

		r.logger.Debug("Applied batch of events",
			slog.String("projection", projectionName),
			slog.Int("batch_size", len(batch)),
			slog.Int("total_applied", totalApplied),
			slog.Any("last_processed", lastProcessed),
		)

		// If we got fewer events than requested, we're at the end
		if len(batch) < r.batchSize {
			break
		}
	}

	r.logger.Info("Projection processing complete",
		slog.String("projection", projectionName),
		slog.Int("total_applied", totalApplied),
		slog.Any("last_processed", lastProcessed),
	)

	return totalApplied, lastProcessed, nil
}

// ApplyBatch applies a specific batch of events directly.
func (r *runner) ApplyBatch(ctx context.Context, batch []synckit.EventWithVersion) error {
	if len(batch) == 0 {
		return nil // Nothing to do
	}

	projectionName := r.projector.Name()
	
	// Apply the events to the projector
	if err := r.projector.Apply(ctx, batch); err != nil {
		return errors.E(
			errors.Op("runner.ApplyBatch"),
			errors.Component("projection"),
			fmt.Errorf("failed to apply batch to projection %s: %w", projectionName, err),
		)
	}

	// Update the offset to the last event's version
	lastVersion := batch[len(batch)-1].Version
	if err := r.offsets.Set(ctx, projectionName, lastVersion); err != nil {
		return errors.E(
			errors.Op("runner.ApplyBatch"),
			errors.Component("projection"),
			fmt.Errorf("failed to update offset for projection %s: %w", projectionName, err),
		)
	}

	return nil
}
