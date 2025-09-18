package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewTracer(t *testing.T) {
	serviceName := "test-service"

	tracer := NewTracer(serviceName)

	if tracer == nil {
		t.Fatal("Expected tracer to be created, got nil")
	}

	if tracer.serviceName != serviceName {
		t.Errorf("Expected service name %s, got %s", serviceName, tracer.serviceName)
	}
}

func TestTracerWithOptions(t *testing.T) {
	serviceName := "test-service"
	version := "v1.0.0"

	tracer := NewTracer(serviceName,
		WithVersion(version),
		WithAttributes(attribute.String("custom", "value")),
	)

	if tracer.serviceName != serviceName {
		t.Errorf("Expected service name %s, got %s", serviceName, tracer.serviceName)
	}

	if tracer.version != version {
		t.Errorf("Expected version %s, got %s", version, tracer.version)
	}

	// Check that custom attributes were added
	found := false
	for _, attr := range tracer.attributes {
		if attr.Key == "custom" && attr.Value.AsString() == "value" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected custom attribute to be added")
	}
}

func TestStartSyncOperation(t *testing.T) {
	// Set up a test tracer provider
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()
	operation := "full_sync"

	ctx, span := tracer.StartSyncOperation(ctx, operation)

	if span == nil {
		t.Fatal("Expected span to be created, got nil")
	}

	// Clean up
	span.End()
}

func TestRecordError(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartSyncOperation(ctx, "test")
	defer span.End()

	testError := &testError{"test error"}
	description := "Test error occurred"

	// This should not panic
	tracer.RecordError(span, testError, description)
}

func TestSetSyncResult(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartSyncOperation(ctx, "test")
	defer span.End()

	// This should not panic
	tracer.SetSyncResult(span, 10, 5, 2)
}

func TestAddEventAttributes(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartSyncOperation(ctx, "test")
	defer span.End()

	eventCount := 15
	aggregateIDs := []string{"agg1", "agg2", "agg3"}

	// This should not panic
	tracer.AddEventAttributes(span, eventCount, aggregateIDs)
}

func TestAddEventAttributesTooManyAggregates(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartSyncOperation(ctx, "test")
	defer span.End()

	eventCount := 15
	// Create more than maxAggregateIDs (10)
	aggregateIDs := make([]string, 15)
	for i := 0; i < 15; i++ {
		aggregateIDs[i] = "agg" + string(rune('0'+i))
	}

	// This should not panic and should truncate the list
	tracer.AddEventAttributes(span, eventCount, aggregateIDs)
}

func TestStartTransportOperation(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartTransportOperation(ctx, "push", "http")

	if span == nil {
		t.Fatal("Expected span to be created, got nil")
	}

	span.End()
}

func TestStartStorageOperation(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartStorageOperation(ctx, "store", "sqlite")

	if span == nil {
		t.Fatal("Expected span to be created, got nil")
	}

	span.End()
}

func TestStartConflictResolution(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartConflictResolution(ctx, "last_write_wins")

	if span == nil {
		t.Fatal("Expected span to be created, got nil")
	}

	span.End()
}

func TestAddSpanEvent(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartSyncOperation(ctx, "test")
	defer span.End()

	// This should not panic
	tracer.AddSpanEvent(span, "test.event",
		attribute.String("test", "value"),
		attribute.Int("count", 42),
	)
}

// Helper types for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
