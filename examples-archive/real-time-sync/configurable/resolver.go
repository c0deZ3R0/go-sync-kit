package main

import (
	"context"
	"log/slog"

	sync "github.com/c0deZ3R0/go-sync-kit/synckit"
)

func buildResolverCode(logger *slog.Logger) (*sync.DynamicResolver, error) {
	// Hooks for visibility in demo
	hooks := sync.Hooks{
		OnRuleMatched: func(c sync.Conflict, r sync.Rule) {
			logger.Info("rule matched", "rule", r.Name, "event_type", c.EventType, "agg", c.AggregateID, "fields", c.ChangedFields)
		},
		OnResolved: func(c sync.Conflict, res sync.ResolvedConflict) {
			logger.Info("resolved", "decision", res.Decision, "reasons", res.Reasons, "events", len(res.ResolvedEvents))
		},
		OnFallback: func(c sync.Conflict) {
			logger.Warn("fallback used", "event_type", c.EventType, "agg", c.AggregateID)
		},
	}

	return sync.NewDynamicResolver(
		// Title changes -> prefer remote (simple for demo)
		sync.WithRule("title-lww",
			sync.And(sync.EventTypeIs("document_updated"), sync.FieldChanged("title")),
			&sync.LastWriteWinsResolver{},
		),

		// Tags -> additive merge
		sync.WithRule("tags-additive",
			sync.And(sync.EventTypeIs("document_updated"), sync.FieldChanged("tags")),
			&sync.AdditiveMergeResolver{},
		),

		// Status change -> manual review to prove Strategy selection in demo
		sync.WithRule("status-manual",
			sync.And(sync.EventTypeIs("document_updated"), sync.FieldChanged("status")),
			&sync.ManualReviewResolver{Reason: "status progression requires review"},
		),

		// Fallback
		sync.WithFallback(&sync.LastWriteWinsResolver{}),
		sync.WithHooks(hooks),
		sync.WithLogger(logger),
	)
}

func buildResolverFromConfig(logger *slog.Logger, yamlBytes []byte) (*sync.DynamicResolver, error) {
	loader := sync.NewConfigLoader(
		sync.WithConfigValidator(&sync.BasicValidator{}), // available in branch
	)
	if err := loader.LoadFromBytes(yamlBytes, "yaml"); err != nil {
		return nil, err
	}
	res, err := loader.BuildDynamicResolver()
	if err != nil {
		return nil, err
	}
	// Optional: wrap in ObservableResolver if you want extra metrics/logging
	_ = context.Background()
	return res, nil
}
