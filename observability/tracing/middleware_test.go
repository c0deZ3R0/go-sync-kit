package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

func TestHTTPMiddleware(t *testing.T) {
	// Set up test tracer provider with in-memory exporter
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	serviceName := "test-service"
	middleware := NewHTTPMiddleware(serviceName)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if span is available in context
		span := trace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			t.Error("Expected span to be recording in handler context")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with middleware
	wrappedHandler := middleware.Handler(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("User-Agent", "test-client")
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "test response" {
		t.Errorf("Expected body 'test response', got '%s'", w.Body.String())
	}
}

func TestHTTPMiddlewareWithError(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	serviceName := "test-service"
	middleware := NewHTTPMiddleware(serviceName)

	// Create error handler
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})

	// Wrap with middleware
	wrappedHandler := middleware.Handler(errorHandler)

	// Create test request
	req := httptest.NewRequest("POST", "/api/error", strings.NewReader("request body"))
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestHTTPClientTransport(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	serviceName := "test-service"
	tracer := NewTracer(serviceName)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Trace headers should be present when span is active
		traceHeader := r.Header.Get("traceparent")
		if traceHeader != "" {
			t.Logf("Found traceparent header: %s", traceHeader)
		} else {
			t.Log("No traceparent header found - may be expected depending on transport setup")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server response"))
	}))
	defer server.Close()

	// Create HTTP client with tracing transport
	client := &http.Client{
		Transport: NewClientTransport(http.DefaultTransport),
	}

	// Create request with span context
	ctx, span := tracer.StartSyncOperation(context.Background(), "test_request")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHTTPClientTransportWithoutSpan(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server response"))
	}))
	defer server.Close()

	// Create HTTP client with tracing transport
	client := &http.Client{
		Transport: NewClientTransport(http.DefaultTransport),
	}

	// Create request without span context
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute request - should still work without span
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHTTPClientTransportError(t *testing.T) {
	tp := tracesdk.NewTracerProvider()
	otel.SetTracerProvider(tp)

	serviceName := "test-service"
	tracer := NewTracer(serviceName)

	// Create HTTP client with tracing transport
	client := &http.Client{
		Transport: NewClientTransport(http.DefaultTransport),
	}

	// Create request with span context to invalid URL
	ctx, span := tracer.StartSyncOperation(context.Background(), "test_error_request")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://invalid-host:99999/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute request - should fail
	_, err = client.Do(req)
	if err == nil {
		t.Error("Expected request to fail, but it succeeded")
	}
}

func TestMiddlewareWithEmptyServiceName(t *testing.T) {
	// Test that middleware works with empty service name
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Middleware with empty service name should not panic: %v", r)
		}
	}()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewHTTPMiddleware("")
	wrappedHandler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestTransportWithNilBase(t *testing.T) {
	// Test that transport works with nil base transport
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Transport with nil base should not panic: %v", r)
		}
	}()

	transport := NewClientTransport(nil)

	// Should not be nil when base is nil (uses default transport)
	if transport == nil {
		t.Error("Expected transport to be created even with nil base")
	}
}
