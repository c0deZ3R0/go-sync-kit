package synckit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ObservableResolver wraps any ConflictResolver to provide detailed metrics and logging.
// It implements the Observer pattern for monitoring conflict resolution performance.
type ObservableResolver struct {
	wrapped        ConflictResolver
	metrics        MetricsCollector
	logger         *slog.Logger
	hooks          *ResolutionHooks
	groupName      string // Optional: for group-level observability
}

// ExtendedMetricsCollector extends the basic MetricsCollector with detailed resolution metrics.
type ExtendedMetricsCollector interface {
	MetricsCollector // Embed existing interface
	
	// New methods for detailed conflict resolution metrics
	RecordResolution(ruleName, groupName, decision string, duration time.Duration, success bool)
	RecordRuleMatch(ruleName, groupName string)
	RecordFallbackUsage(groupName string)
	RecordResolutionError(ruleName, groupName, errorType string)
	RecordManualReview(reason string)
}

// ResolutionHooks provides callbacks for observing conflict resolution events.
type ResolutionHooks struct {
	OnConflict      func(ctx context.Context, conflict *Conflict)
	OnRuleMatched   func(ctx context.Context, conflict *Conflict, rule *Rule, group *RuleGroup)
	OnResolution    func(ctx context.Context, result *ResolvedConflict, duration time.Duration)
	OnFallback      func(ctx context.Context, conflict *Conflict, group *RuleGroup)
	OnError         func(ctx context.Context, conflict *Conflict, err error)
}

// ObservableOption provides configuration for ObservableResolver.
type ObservableOption interface {
	apply(*ObservableResolver)
}

type observableOptionFunc func(*ObservableResolver)

func (f observableOptionFunc) apply(or *ObservableResolver) {
	f(or)
}

// WithMetricsCollector sets the metrics collector for the resolver.
func WithMetricsCollector(mc MetricsCollector) ObservableOption {
	return observableOptionFunc(func(or *ObservableResolver) {
		or.metrics = mc
	})
}

// WithObservableLogger sets the logger for the resolver.
func WithObservableLogger(logger *slog.Logger) ObservableOption {
	return observableOptionFunc(func(or *ObservableResolver) {
		or.logger = logger
	})
}

// WithResolutionHooks sets the resolution hooks for the resolver.
func WithResolutionHooks(hooks *ResolutionHooks) ObservableOption {
	return observableOptionFunc(func(or *ObservableResolver) {
		or.hooks = hooks
	})
}

// WithGroupName sets the group name for observability.
func WithGroupName(name string) ObservableOption {
	return observableOptionFunc(func(or *ObservableResolver) {
		or.groupName = name
	})
}

// NewObservableResolver creates a new ObservableResolver that wraps an existing resolver.
func NewObservableResolver(resolver ConflictResolver, opts ...ObservableOption) *ObservableResolver {
	or := &ObservableResolver{
		wrapped: resolver,
	}
	
	for _, opt := range opts {
		opt.apply(or)
	}
	
	return or
}

// Resolve implements the ConflictResolver interface with added observability.
func (or *ObservableResolver) Resolve(ctx context.Context, conflict Conflict) (ResolvedConflict, error) {
	start := time.Now()
	
	// Trigger OnConflict hook
	if or.hooks != nil && or.hooks.OnConflict != nil {
		or.hooks.OnConflict(ctx, &conflict)
	}
	
	// Log the conflict
	if or.logger != nil {
		or.logger.Debug("Attempting to resolve conflict", "aggregate_id", conflict.AggregateID, "group", or.groupName)
	}
	
	// Perform the actual resolution
	resolved, err := or.wrapped.Resolve(ctx, conflict)
	duration := time.Since(start)
	
	// Get resolution details
	_, ruleName := getResolverDetails(or.wrapped)
	
	// Record metrics and call hooks
	if err != nil {
		if or.metrics != nil {
			// Try to use extended metrics if available, otherwise use basic metrics
			if extMetrics, ok := or.metrics.(ExtendedMetricsCollector); ok {
				extMetrics.RecordResolution(ruleName, or.groupName, "error", duration, false)
				extMetrics.RecordResolutionError(ruleName, or.groupName, classifyError(err))
			}
		}
		if or.hooks != nil && or.hooks.OnError != nil {
			or.hooks.OnError(ctx, &conflict, err)
		}
		if or.logger != nil {
			or.logger.Error("Conflict resolution failed", "error", err, "duration", duration)
		}
	} else {
		if or.metrics != nil {
			// Try to use extended metrics if available, otherwise use basic metrics
			if extMetrics, ok := or.metrics.(ExtendedMetricsCollector); ok {
				extMetrics.RecordResolution(ruleName, or.groupName, resolved.Decision, duration, true)
				if resolved.Decision == "manual_review" {
					extMetrics.RecordManualReview(getManualReviewReason(resolved))
				}
			}
		}
		if or.hooks != nil && or.hooks.OnResolution != nil {
			or.hooks.OnResolution(ctx, &resolved, duration)
		}
		if or.logger != nil {
			or.logger.Info("Conflict resolved successfully", "decision", resolved.Decision, "duration", duration)
		}
	}
	
	return resolved, err
}

// Helper to get resolver and rule name details
func getResolverDetails(resolver ConflictResolver) (string, string) {
	switch r := resolver.(type) {
	case *DynamicResolver:
		// This case is tricky, as we don't know which rule matched.
		// The DynamicResolver itself should be instrumented to provide this.
		return "DynamicResolver", ""
	default:
		return fmt.Sprintf("%T", r), ""
	}
}

// Helper to classify errors for metrics
func classifyError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "generic"
	}
}

// Helper to get manual review reason
func getManualReviewReason(resolved ResolvedConflict) string {
	if len(resolved.Reasons) > 0 {
		return resolved.Reasons[0]
	}
	return "unknown"
}

// ExtendedNoOpMetricsCollector extends NoOpMetricsCollector with extended methods.
type ExtendedNoOpMetricsCollector struct {
	NoOpMetricsCollector // Embed existing no-op collector
}

func (c *ExtendedNoOpMetricsCollector) RecordResolution(rule, group, decision string, dur time.Duration, success bool) {}
func (c *ExtendedNoOpMetricsCollector) RecordRuleMatch(rule, group string)                 {}
func (c *ExtendedNoOpMetricsCollector) RecordFallbackUsage(group string)                   {}
func (c *ExtendedNoOpMetricsCollector) RecordResolutionError(rule, group, errType string) {}
func (c *ExtendedNoOpMetricsCollector) RecordManualReview(reason string)                 {}

