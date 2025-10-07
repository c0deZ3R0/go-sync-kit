package tracing

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware provides OpenTelemetry tracing for HTTP requests.
// It automatically creates spans for incoming HTTP requests with proper
// context propagation and standard HTTP semantic conventions.
type HTTPMiddleware struct {
	tracer      trace.Tracer
	propagator  propagation.TextMapPropagator
	serviceName string
}

// NewHTTPMiddleware creates a new HTTP tracing middleware.
func NewHTTPMiddleware(serviceName string) *HTTPMiddleware {
	return &HTTPMiddleware{
		tracer:      otel.Tracer("github.com/c0deZ3R0/go-sync-kit/http"),
		propagator:  otel.GetTextMapPropagator(),
		serviceName: serviceName,
	}
}

// Handler wraps an HTTP handler with distributed tracing.
// It extracts trace context from incoming requests, creates new spans,
// and properly propagates context to downstream operations.
func (m *HTTPMiddleware) Handler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming request headers
		ctx := m.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Create span name based on HTTP method and route
		spanName := r.Method + " " + r.URL.Path
		if spanName == "" {
			spanName = "HTTP " + r.Method
		}

		// Start new span with extracted context
		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithAttributes(
				// Standard HTTP semantic conventions
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.target", r.URL.Path),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.route", r.URL.Path),

				// Sync-kit specific attributes
				attribute.String("service.name", m.serviceName),
				ComponentKey.String(ComponentTransport),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Wrap response writer to capture status code and response size
		ww := &wrappedResponseWriter{
			ResponseWriter: w,
			statusCode:     200, // Default to 200 if WriteHeader is not called
		}

		// Record request start time
		start := time.Now()

		// Execute the handler with the traced context
		handler.ServeHTTP(ww, r.WithContext(ctx))

		// Record final span attributes after request completion
		duration := time.Since(start)
		span.SetAttributes(
			attribute.Int("http.status_code", ww.statusCode),
			attribute.Int64("http.response_size", ww.bytesWritten),
			attribute.Float64("http.duration_ms", float64(duration.Nanoseconds())/1e6),
		)

		// Set span status based on HTTP status code
		if ww.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(ww.statusCode))
			span.SetAttributes(attribute.Bool("error", true))
		} else {
			span.SetStatus(codes.Ok, "Request completed successfully")
		}

		// Add events for significant status codes
		if ww.statusCode >= 500 {
			span.AddEvent("server_error", trace.WithAttributes(
				attribute.String("error.message", http.StatusText(ww.statusCode)),
			))
		} else if ww.statusCode >= 400 {
			span.AddEvent("client_error", trace.WithAttributes(
				attribute.String("error.message", http.StatusText(ww.statusCode)),
			))
		}
	})
}

// HandlerFunc wraps an HTTP handler function with distributed tracing.
func (m *HTTPMiddleware) HandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.Handler(handler).ServeHTTP
}

// wrappedResponseWriter captures response data for tracing
type wrappedResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
	headerWritten bool
}

func (w *wrappedResponseWriter) WriteHeader(statusCode int) {
	if !w.headerWritten {
		w.statusCode = statusCode
		w.headerWritten = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *wrappedResponseWriter) Write(data []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(200)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytesWritten += int64(n)
	return n, err
}

// ClientTransport wraps an HTTP transport with distributed tracing for outgoing requests.
type ClientTransport struct {
	base       http.RoundTripper
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// NewClientTransport creates a new tracing HTTP client transport.
func NewClientTransport(base http.RoundTripper) *ClientTransport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &ClientTransport{
		base:       base,
		tracer:     otel.Tracer("github.com/c0deZ3R0/go-sync-kit/http-client"),
		propagator: otel.GetTextMapPropagator(),
	}
}

// RoundTrip implements http.RoundTripper interface with tracing.
func (t *ClientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create span name based on HTTP method and host
	spanName := req.Method + " " + req.URL.Host

	// Start new span for outgoing request
	ctx, span := t.tracer.Start(req.Context(), spanName,
		trace.WithAttributes(
			// Standard HTTP client semantic conventions
			attribute.String("http.method", req.Method),
			attribute.String("http.url", req.URL.String()),
			attribute.String("http.scheme", req.URL.Scheme),
			attribute.String("http.host", req.URL.Host),
			attribute.String("http.target", req.URL.Path),

			// Sync-kit specific attributes
			ComponentKey.String(ComponentTransport),
			TransportTypeKey.String(TransportTypeHTTP),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	// Inject trace context into request headers
	t.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Execute the request
	start := time.Now()
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	duration := time.Since(start)

	// Record response attributes
	if err != nil {
		span.SetStatus(codes.Error, "Request failed")
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.type", "http_request_failed"),
		))
		span.SetAttributes(attribute.Bool("error", true))
	} else {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
			attribute.Float64("http.duration_ms", float64(duration.Nanoseconds())/1e6),
		)

		// Set status based on response code
		if resp.StatusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(resp.StatusCode))
			span.SetAttributes(attribute.Bool("error", true))
		} else {
			span.SetStatus(codes.Ok, "Request completed successfully")
		}

		// Add response size if available
		if resp.ContentLength > 0 {
			span.SetAttributes(attribute.Int64("http.response_size", resp.ContentLength))
		}
	}

	return resp, err
}

// TraceConfig provides configuration for HTTP tracing middleware
type TraceConfig struct {
	ServiceName     string
	SkipHealthCheck bool
	SkipUserAgent   []string
	AttributeFilter func(*http.Request) []attribute.KeyValue
}

// NewHTTPMiddlewareWithConfig creates HTTP middleware with custom configuration
func NewHTTPMiddlewareWithConfig(config TraceConfig) *HTTPMiddleware {
	if config.ServiceName == "" {
		config.ServiceName = "sync-kit-service"
	}

	return &HTTPMiddleware{
		tracer:      otel.Tracer("github.com/c0deZ3R0/go-sync-kit/http"),
		propagator:  otel.GetTextMapPropagator(),
		serviceName: config.ServiceName,
	}
}

