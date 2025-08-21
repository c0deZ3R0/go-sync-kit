package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler creates an HTTP handler for exposing Prometheus metrics.
// It returns a standard http.Handler that can be used with any HTTP server.
func (m *SyncKitMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		// Enable collection of Go runtime metrics
		EnableOpenMetrics: true,
	})
}

// HandlerWithOptions creates an HTTP handler with custom options for exposing Prometheus metrics.
func (m *SyncKitMetrics) HandlerWithOptions(opts promhttp.HandlerOpts) http.Handler {
	return promhttp.HandlerFor(m.registry, opts)
}

// DefaultHandler creates a default Prometheus metrics HTTP handler using the default registry.
// This is useful for applications that want to expose basic Go runtime metrics without
// setting up a custom SyncKitMetrics instance.
func DefaultHandler() http.Handler {
	return promhttp.Handler()
}
