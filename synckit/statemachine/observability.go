package statemachine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/attribute"
)

// ObservabilityHooks integrates state machines with existing observability infrastructure.
type ObservabilityHooks[T comparable] struct {
	// MetricsCollector for recording state machine metrics
	MetricsCollector StateMetricsCollector
	
	// Tracer for distributed tracing integration
	Tracer StateTracer
	
	// HealthUpdater for health check integration
	HealthUpdater StateHealthUpdater
	
	// Logger for structured logging
	Logger *slog.Logger
	
	// ComponentName identifies the component for observability
	ComponentName string
}

// StateMetricsCollector defines the interface for collecting state machine metrics.
// This integrates with the existing MetricsCollector interface in synckit.
type StateMetricsCollector interface {
	// RecordStateTransition records a successful state transition
	RecordStateTransition(component, from, to string, duration time.Duration, metadata map[string]interface{})
	
	// RecordStateTransitionError records a failed state transition
	RecordStateTransitionError(component, from, to, errorType string, metadata map[string]interface{})
	
	// RecordCurrentState updates the current state gauge
	RecordCurrentState(component, state string)
	
	// RecordStateDuration records time spent in a state
	RecordStateDuration(component, state string, duration time.Duration)
	
	// RecordStateTransitionLatency records how long a transition took
	RecordStateTransitionLatency(component, from, to string, latency time.Duration)
}

// StateTracer defines the interface for distributed tracing of state machines.
type StateTracer interface {
	// StartStateTransition starts a new span for a state transition
	StartStateTransition(ctx context.Context, component, from, to string) (context.Context, trace.Span)
	
	// RecordTransitionSuccess records successful transition attributes
	RecordTransitionSuccess(span trace.Span, component, from, to, transitionID string, duration time.Duration, metadata map[string]interface{})
	
	// RecordTransitionError records failed transition error
	RecordTransitionError(span trace.Span, component, from, to string, err error, metadata map[string]interface{})
}

// StateHealthUpdater defines the interface for updating health checks based on state.
type StateHealthUpdater interface {
	// UpdateComponentState updates the health check state for a component
	UpdateComponentState(component, state string, metadata map[string]interface{})
	
	// RecordStateError records a state-related error for health monitoring
	RecordStateError(component, state string, err error)
}

// NewObservabilityHooks creates new observability hooks for state machines.
func NewObservabilityHooks[T comparable](
	metricsCollector StateMetricsCollector,
	tracer StateTracer,
	healthUpdater StateHealthUpdater,
	logger *slog.Logger,
	componentName string,
) *ObservabilityHooks[T] {
	return &ObservabilityHooks[T]{
		MetricsCollector: metricsCollector,
		Tracer:          tracer,
		HealthUpdater:   healthUpdater,
		Logger:          logger,
		ComponentName:   componentName,
	}
}

// OnTransition handles successful state transitions with full observability integration.
func (h *ObservabilityHooks[T]) OnTransition(transition StateTransition[T]) {
	fromStr := fmt.Sprintf("%v", transition.From)
	toStr := fmt.Sprintf("%v", transition.To)
	
	// Record metrics
	if h.MetricsCollector != nil {
		h.MetricsCollector.RecordStateTransition(
			h.ComponentName, fromStr, toStr, transition.Duration, transition.Metadata,
		)
		h.MetricsCollector.RecordCurrentState(h.ComponentName, toStr)
		h.MetricsCollector.RecordStateDuration(h.ComponentName, fromStr, transition.Duration)
	}
	
	// Update health status
	if h.HealthUpdater != nil {
		h.HealthUpdater.UpdateComponentState(h.ComponentName, toStr, transition.Metadata)
	}
	
	// Structured logging
	if h.Logger != nil {
		h.Logger.Info("State transition completed",
			slog.String("component", h.ComponentName),
			slog.String("from_state", fromStr),
			slog.String("to_state", toStr),
			slog.Duration("duration", transition.Duration),
			slog.String("transition_id", transition.TransitionID),
			slog.Time("timestamp", transition.Timestamp))
	}
}

// OnTransitionFailed handles failed state transitions with full observability integration.
func (h *ObservabilityHooks[T]) OnTransitionFailed(from, to T, err error, metadata map[string]interface{}) {
	fromStr := fmt.Sprintf("%v", from)
	toStr := fmt.Sprintf("%v", to)
	errorType := "transition_failed"
	
	if err != nil {
		errorType = err.Error()
	}
	
	// Record error metrics
	if h.MetricsCollector != nil {
		h.MetricsCollector.RecordStateTransitionError(
			h.ComponentName, fromStr, toStr, errorType, metadata,
		)
	}
	
	// Record health error
	if h.HealthUpdater != nil {
		h.HealthUpdater.RecordStateError(h.ComponentName, fromStr, err)
	}
	
	// Error logging
	if h.Logger != nil {
		h.Logger.Error("State transition failed",
			slog.String("component", h.ComponentName),
			slog.String("from_state", fromStr),
			slog.String("to_state", toStr),
			slog.String("error", err.Error()))
	}
}

// SyncKitMetricsAdapter adapts the existing synckit MetricsCollector to work with state machines.
type SyncKitMetricsAdapter struct {
	collector interface {
		// Methods from the existing MetricsCollector interface
		RecordSyncDuration(operation string, duration time.Duration)
		RecordSyncEvents(pushed, pulled int)
		RecordSyncErrors(operation, errorType string)
		RecordConflicts(count int)
	}
}

// NewSyncKitMetricsAdapter creates an adapter for the existing synckit metrics collector.
func NewSyncKitMetricsAdapter(collector interface {
	RecordSyncDuration(operation string, duration time.Duration)
	RecordSyncEvents(pushed, pulled int)
	RecordSyncErrors(operation, errorType string)
	RecordConflicts(count int)
}) *SyncKitMetricsAdapter {
	return &SyncKitMetricsAdapter{collector: collector}
}

// RecordStateTransition implements StateMetricsCollector interface.
func (a *SyncKitMetricsAdapter) RecordStateTransition(component, from, to string, duration time.Duration, metadata map[string]interface{}) {
	// Map state transitions to existing sync operation metrics
	operation := fmt.Sprintf("%s_%s_to_%s", component, from, to)
	a.collector.RecordSyncDuration(operation, duration)
}

// RecordStateTransitionError implements StateMetricsCollector interface.
func (a *SyncKitMetricsAdapter) RecordStateTransitionError(component, from, to, errorType string, metadata map[string]interface{}) {
	operation := fmt.Sprintf("%s_transition", component)
	a.collector.RecordSyncErrors(operation, errorType)
}

// RecordCurrentState implements StateMetricsCollector interface.
func (a *SyncKitMetricsAdapter) RecordCurrentState(component, state string) {
	// Current state is tracked via the duration metrics
	// This could be extended to use gauge metrics if available
}

// RecordStateDuration implements StateMetricsCollector interface.
func (a *SyncKitMetricsAdapter) RecordStateDuration(component, state string, duration time.Duration) {
	operation := fmt.Sprintf("%s_state_%s", component, state)
	a.collector.RecordSyncDuration(operation, duration)
}

// RecordStateTransitionLatency implements StateMetricsCollector interface.
func (a *SyncKitMetricsAdapter) RecordStateTransitionLatency(component, from, to string, latency time.Duration) {
	operation := fmt.Sprintf("%s_transition_latency", component)
	a.collector.RecordSyncDuration(operation, latency)
}

// SyncKitTracerAdapter adapts the existing synckit Tracer to work with state machines.
type SyncKitTracerAdapter struct {
	tracer interface {
		// Methods from existing Tracer interface
		StartSyncOperation(ctx context.Context, operation string) (context.Context, trace.Span)
		RecordError(span trace.Span, err error, description string)
		SetSyncResult(span trace.Span, eventsPushed, eventsPulled, conflictsResolved int)
	}
}

// NewSyncKitTracerAdapter creates an adapter for the existing synckit tracer.
func NewSyncKitTracerAdapter(tracer interface {
	StartSyncOperation(ctx context.Context, operation string) (context.Context, trace.Span)
	RecordError(span trace.Span, err error, description string)
	SetSyncResult(span trace.Span, eventsPushed, eventsPulled, conflictsResolved int)
}) *SyncKitTracerAdapter {
	return &SyncKitTracerAdapter{tracer: tracer}
}

// StartStateTransition implements StateTracer interface.
func (a *SyncKitTracerAdapter) StartStateTransition(ctx context.Context, component, from, to string) (context.Context, trace.Span) {
	operation := fmt.Sprintf("%s.state.%s.to.%s", component, from, to)
	return a.tracer.StartSyncOperation(ctx, operation)
}

// RecordTransitionSuccess implements StateTracer interface.
func (a *SyncKitTracerAdapter) RecordTransitionSuccess(span trace.Span, component, from, to, transitionID string, duration time.Duration, metadata map[string]interface{}) {
	// Add state machine specific attributes
	span.SetAttributes(
		attribute.String("state.component", component),
		attribute.String("state.from", from),
		attribute.String("state.to", to),
		attribute.String("state.transition_id", transitionID),
		attribute.Int64("state.duration_ms", duration.Milliseconds()),
	)
	
	// Add metadata as span attributes
	if metadata != nil {
		for key, value := range metadata {
			span.SetAttributes(attribute.String(fmt.Sprintf("state.meta.%s", key), fmt.Sprintf("%v", value)))
		}
	}
}

// RecordTransitionError implements StateTracer interface.
func (a *SyncKitTracerAdapter) RecordTransitionError(span trace.Span, component, from, to string, err error, metadata map[string]interface{}) {
	description := fmt.Sprintf("State transition failed: %s -> %s", from, to)
	a.tracer.RecordError(span, err, description)
	
	// Add error-specific attributes
	span.SetAttributes(
		attribute.String("state.component", component),
		attribute.String("state.from", from),
		attribute.String("state.to", to),
		attribute.Bool("state.transition_failed", true),
	)
}

// StateHealthStatus represents the health status based on state.
type StateHealthStatus string

const (
	StateHealthHealthy    StateHealthStatus = "healthy"
	StateHealthDegraded   StateHealthStatus = "degraded"
	StateHealthUnhealthy  StateHealthStatus = "unhealthy"
	StateHealthCritical   StateHealthStatus = "critical"
)

// DefaultStateHealthUpdater provides a simple health updater implementation.
type DefaultStateHealthUpdater struct {
	// healthChecker would be the existing health check system
	healthChecker interface {
		UpdateStatus(component string, status string, metadata map[string]interface{})
		RecordError(component string, err error)
	}
	
	// stateHealthMap maps states to health statuses
	stateHealthMap map[string]StateHealthStatus
}

// NewDefaultStateHealthUpdater creates a new default health updater.
func NewDefaultStateHealthUpdater(
	healthChecker interface {
		UpdateStatus(component string, status string, metadata map[string]interface{})
		RecordError(component string, err error)
	},
) *DefaultStateHealthUpdater {
	return &DefaultStateHealthUpdater{
		healthChecker: healthChecker,
		stateHealthMap: map[string]StateHealthStatus{
			// Sync states
			"idle":                 StateHealthHealthy,
			"initializing":         StateHealthHealthy,
			"pushing":              StateHealthHealthy,
			"pulling":              StateHealthHealthy,
			"resolving_conflicts":  StateHealthDegraded,
			"completed":            StateHealthHealthy,
			"failed":               StateHealthCritical,
			"cancelled":            StateHealthDegraded,
			
			// Transport states (for future use)
			"connected":            StateHealthHealthy,
			"connecting":           StateHealthDegraded,
			"disconnected":         StateHealthUnhealthy,
			"reconnecting":         StateHealthDegraded,
			"transport_failed":     StateHealthCritical,
		},
	}
}

// UpdateComponentState implements StateHealthUpdater interface.
func (h *DefaultStateHealthUpdater) UpdateComponentState(component, state string, metadata map[string]interface{}) {
	if h.healthChecker == nil {
		return
	}
	
	// Map state to health status
	healthStatus := StateHealthHealthy // default
	if status, exists := h.stateHealthMap[state]; exists {
		healthStatus = status
	}
	
	// Update health status
	healthMetadata := map[string]interface{}{
		"state": state,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	// Include original metadata
	if metadata != nil {
		for key, value := range metadata {
			healthMetadata[key] = value
		}
	}
	
	h.healthChecker.UpdateStatus(component, string(healthStatus), healthMetadata)
}

// RecordStateError implements StateHealthUpdater interface.
func (h *DefaultStateHealthUpdater) RecordStateError(component, state string, err error) {
	if h.healthChecker == nil {
		return
	}
	
	h.healthChecker.RecordError(component, err)
}

// CreateObservableStateMachine creates a state machine with full observability integration.
func CreateObservableStateMachine[T comparable](
	config StateMachineConfig[T],
	metricsCollector StateMetricsCollector,
	tracer StateTracer,
	healthUpdater StateHealthUpdater,
	logger *slog.Logger,
	componentName string,
) (StateMachine[T], error) {
	// Create observability hooks
	hooks := NewObservabilityHooks[T](
		metricsCollector,
		tracer,
		healthUpdater,
		logger,
		componentName,
	)
	
	// Create the state machine
	stateMachine, err := New(config)
	if err != nil {
		return nil, err
	}
	
	// Subscribe to state changes
	stateMachine.Subscribe(hooks)
	
	return stateMachine, nil
}
