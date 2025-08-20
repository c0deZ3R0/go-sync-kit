// Package tracing provides OpenTelemetry integration for go-sync-kit.
// It enables distributed tracing of sync operations, providing visibility
// into operation performance and debugging capabilities.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SyncKitTracer wraps OpenTelemetry tracing functionality for sync-kit operations.
// It provides a standardized way to create spans for sync operations with
// consistent attributes and naming conventions.
type SyncKitTracer struct {
	tracer      trace.Tracer
	serviceName string
	version     string
	attributes  []attribute.KeyValue
}

// TracerOption allows for functional configuration of SyncKitTracer.
type TracerOption func(*SyncKitTracer)

// WithServiceName sets the service name for the tracer.
func WithServiceName(name string) TracerOption {
	return func(t *SyncKitTracer) {
		t.serviceName = name
		t.attributes = append(t.attributes, attribute.String("service.name", name))
	}
}

// WithVersion sets the service version for the tracer.
func WithVersion(version string) TracerOption {
	return func(t *SyncKitTracer) {
		t.version = version
		t.attributes = append(t.attributes, attribute.String("service.version", version))
	}
}

// WithAttributes adds additional attributes to all spans created by this tracer.
func WithAttributes(attrs ...attribute.KeyValue) TracerOption {
	return func(t *SyncKitTracer) {
		t.attributes = append(t.attributes, attrs...)
	}
}

// NewTracer creates a new SyncKitTracer with the given service name and options.
// It uses the global OpenTelemetry tracer provider to create the underlying tracer.
func NewTracer(serviceName string, opts ...TracerOption) *SyncKitTracer {
	tracer := otel.Tracer("github.com/c0deZ3R0/go-sync-kit")
	
	t := &SyncKitTracer{
		tracer:      tracer,
		serviceName: serviceName,
		attributes: []attribute.KeyValue{
			attribute.String("service.name", serviceName),
			attribute.String("library.name", "go-sync-kit"),
		},
	}
	
	for _, opt := range opts {
		opt(t)
	}
	
	return t
}

// StartSyncOperation starts a new span for a sync operation.
// It automatically sets standard attributes and naming conventions.
func (t *SyncKitTracer) StartSyncOperation(ctx context.Context, operation string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("synckit.sync.%s", operation)
	
	attrs := append([]attribute.KeyValue{
		SyncOperationKey.String(operation),
		SyncPhaseKey.String("start"),
		ComponentKey.String("synckit"),
	}, t.attributes...)
	
	return t.tracer.Start(ctx, spanName,
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartTransportOperation starts a new span for a transport operation.
func (t *SyncKitTracer) StartTransportOperation(ctx context.Context, operation, transport string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("synckit.transport.%s", operation)
	
	attrs := append([]attribute.KeyValue{
		TransportOperationKey.String(operation),
		TransportTypeKey.String(transport),
		ComponentKey.String("transport"),
	}, t.attributes...)
	
	return t.tracer.Start(ctx, spanName,
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindClient),
	)
}

// StartStorageOperation starts a new span for a storage operation.
func (t *SyncKitTracer) StartStorageOperation(ctx context.Context, operation, storageType string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("synckit.storage.%s", operation)
	
	attrs := append([]attribute.KeyValue{
		StorageOperationKey.String(operation),
		StorageTypeKey.String(storageType),
		ComponentKey.String("storage"),
	}, t.attributes...)
	
	return t.tracer.Start(ctx, spanName,
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartConflictResolution starts a new span for conflict resolution.
func (t *SyncKitTracer) StartConflictResolution(ctx context.Context, strategy string) (context.Context, trace.Span) {
	spanName := "synckit.conflict.resolve"
	
	attrs := append([]attribute.KeyValue{
		ConflictStrategyKey.String(strategy),
		ComponentKey.String("conflict-resolver"),
	}, t.attributes...)
	
	return t.tracer.Start(ctx, spanName,
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// AddEventAttributes adds sync-specific event attributes to a span.
func (t *SyncKitTracer) AddEventAttributes(span trace.Span, eventCount int, aggregateIDs []string) {
	attrs := []attribute.KeyValue{
		EventCountKey.Int(eventCount),
	}
	
	if len(aggregateIDs) > 0 {
		// Limit the number of aggregate IDs to prevent span size issues
		maxAggregateIDs := 10
		if len(aggregateIDs) > maxAggregateIDs {
			attrs = append(attrs, AggregateIDsKey.StringSlice(aggregateIDs[:maxAggregateIDs]))
			attrs = append(attrs, attribute.Bool("aggregate_ids_truncated", true))
		} else {
			attrs = append(attrs, AggregateIDsKey.StringSlice(aggregateIDs))
		}
	}
	
	span.SetAttributes(attrs...)
}

// RecordError records an error on the span with proper error attributes.
func (t *SyncKitTracer) RecordError(span trace.Span, err error, description string) {
	if err == nil {
		return
	}
	
	span.SetStatus(codes.Error, description)
	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.description", description),
	))
}

// SetSyncResult sets attributes on the span based on sync operation results.
func (t *SyncKitTracer) SetSyncResult(span trace.Span, eventsPushed, eventsPulled, conflictsResolved int) {
	span.SetAttributes(
		EventsPushedKey.Int(eventsPushed),
		EventsPulledKey.Int(eventsPulled),
		ConflictsResolvedKey.Int(conflictsResolved),
		attribute.Bool("success", true),
	)
	
	span.SetStatus(codes.Ok, "Sync operation completed successfully")
}

// AddSpanEvent adds a structured event to the span with sync-specific context.
func (t *SyncKitTracer) AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attrs...))
}
