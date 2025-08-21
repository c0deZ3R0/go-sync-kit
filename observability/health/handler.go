package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HTTPHandler provides HTTP endpoints for health checks.
type HTTPHandler struct {
	checker *HealthChecker
	timeout time.Duration
}

// NewHTTPHandler creates a new HTTP handler for health checks.
func NewHTTPHandler(checker *HealthChecker, timeout time.Duration) *HTTPHandler {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HTTPHandler{
		checker: checker,
		timeout: timeout,
	}
}

// LivenessHandler handles liveness probe requests (Kubernetes-style).
// GET /health/live
func (h *HTTPHandler) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	result := h.checker.CheckLiveness(ctx)
	h.writeHealthResponse(w, result)
}

// ReadinessHandler handles readiness probe requests (Kubernetes-style).
// GET /health/ready
func (h *HTTPHandler) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	result := h.checker.CheckReadiness(ctx)
	h.writeHealthResponse(w, result)
}

// StartupHandler handles startup probe requests (Kubernetes-style).
// GET /health/startup
func (h *HTTPHandler) StartupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	result := h.checker.CheckStartup(ctx)
	h.writeHealthResponse(w, result)
}

// HealthHandler provides a comprehensive health endpoint.
// GET /health
func (h *HTTPHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// Check query parameters for specific check types
	checkType := r.URL.Query().Get("type")
	component := r.URL.Query().Get("component")

	switch checkType {
	case "liveness":
		result := h.checker.CheckLiveness(ctx)
		h.writeHealthResponse(w, result)
		return
	case "readiness":
		result := h.checker.CheckReadiness(ctx)
		h.writeHealthResponse(w, result)
		return
	case "startup":
		result := h.checker.CheckStartup(ctx)
		h.writeHealthResponse(w, result)
		return
	}

	// Handle component-specific requests
	if component != "" {
		results := h.checker.GetComponentStatus(ctx, component)
		w.Header().Set("Content-Type", "application/json")

		// Determine overall status
		overallStatus := StatusUp
		for _, result := range results {
			if result.Status == StatusDown {
				overallStatus = StatusDown
				break
			} else if result.Status == StatusDegraded && overallStatus == StatusUp {
				overallStatus = StatusDegraded
			} else if result.Status == StatusUnknown && overallStatus == StatusUp {
				overallStatus = StatusUnknown
			}
		}

		statusCode := h.getStatusCode(overallStatus)
		w.WriteHeader(statusCode)

		response := map[string]interface{}{
			"status":    overallStatus,
			"component": component,
			"checks":    results,
			"timestamp": time.Now(),
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// Default: return all health checks
	results := h.checker.CheckAll(ctx)
	w.Header().Set("Content-Type", "application/json")

	// Determine overall status from all check types
	overallStatus := StatusUp
	for _, result := range results {
		if result.Status == StatusDown {
			overallStatus = StatusDown
			break
		} else if result.Status == StatusDegraded && overallStatus == StatusUp {
			overallStatus = StatusDegraded
		} else if result.Status == StatusUnknown && overallStatus == StatusUp {
			overallStatus = StatusUnknown
		}
	}

	statusCode := h.getStatusCode(overallStatus)
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"status":    overallStatus,
		"checks":    results,
		"timestamp": time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// ComponentsHandler lists all registered components.
// GET /health/components
func (h *HTTPHandler) ComponentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	components := h.checker.ListComponents()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"components": components,
		"count":      len(components),
		"timestamp":  time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// writeHealthResponse writes a standardized health check response.
func (h *HTTPHandler) writeHealthResponse(w http.ResponseWriter, result OverallResult) {
	w.Header().Set("Content-Type", "application/json")

	statusCode := h.getStatusCode(result.Status)
	w.WriteHeader(statusCode)

	// Add cache headers to prevent caching of health check responses
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	json.NewEncoder(w).Encode(result)
}

// getStatusCode maps health status to HTTP status codes.
func (h *HTTPHandler) getStatusCode(status Status) int {
	switch status {
	case StatusUp:
		return http.StatusOK
	case StatusDown:
		return http.StatusServiceUnavailable
	case StatusDegraded:
		return http.StatusOK // Still serving but with degraded performance
	case StatusUnknown:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// RegisterRoutes registers health check routes with a ServeMux.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/health/live", h.LivenessHandler)
	mux.HandleFunc("/health/ready", h.ReadinessHandler)
	mux.HandleFunc("/health/startup", h.StartupHandler)
	mux.HandleFunc("/health/components", h.ComponentsHandler)
}

// HealthMiddleware provides middleware for adding health check information to responses.
func (h *HTTPHandler) HealthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add health check headers to all responses
		w.Header().Set("X-Health-Status", "available")
		w.Header().Set("X-Health-Endpoint", "/health")

		// Check if service is healthy for non-health endpoints
		if r.URL.Path != "/health" &&
			r.URL.Path != "/health/live" &&
			r.URL.Path != "/health/ready" &&
			r.URL.Path != "/health/startup" {

			// Quick liveness check
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			result := h.checker.CheckLiveness(ctx)
			if result.Status == StatusDown {
				http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// HealthCheckResponse represents the standard health check response format.
type HealthCheckResponse struct {
	Status    Status                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  string                 `json:"duration"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SimpleHealthHandler provides a simple health endpoint that just returns 200 OK.
// This can be useful for basic load balancer health checks.
func SimpleHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// DetailedHealthOptions configures the detailed health response.
type DetailedHealthOptions struct {
	IncludeDetails   bool
	IncludeTimestamp bool
	IncludeDuration  bool
	Format           string // "json" or "text"
}

// DetailedHealthHandler provides a configurable health endpoint.
func (h *HTTPHandler) DetailedHealthHandler(opts DetailedHealthOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
		defer cancel()

		result := h.checker.CheckLiveness(ctx)

		// Handle text format
		if opts.Format == "text" {
			w.Header().Set("Content-Type", "text/plain")
			statusCode := h.getStatusCode(result.Status)
			w.WriteHeader(statusCode)

			response := string(result.Status)
			if opts.IncludeTimestamp {
				response += " at " + result.Timestamp.Format(time.RFC3339)
			}
			if opts.IncludeDuration {
				response += " (took " + result.Duration.String() + ")"
			}

			w.Write([]byte(response))
			return
		}

		// JSON format (default)
		w.Header().Set("Content-Type", "application/json")
		statusCode := h.getStatusCode(result.Status)
		w.WriteHeader(statusCode)

		response := HealthCheckResponse{
			Status: result.Status,
		}

		if opts.IncludeTimestamp {
			response.Timestamp = result.Timestamp
		}
		if opts.IncludeDuration {
			response.Duration = result.Duration.String()
		}
		if opts.IncludeDetails {
			response.Checks = result.Results
		}

		json.NewEncoder(w).Encode(response)
	}
}

// HealthCheckServer provides a simple HTTP server dedicated to health checks.
type HealthCheckServer struct {
	handler *HTTPHandler
	server  *http.Server
}

// NewHealthCheckServer creates a new dedicated health check server.
func NewHealthCheckServer(checker *HealthChecker, addr string) *HealthCheckServer {
	handler := NewHTTPHandler(checker, 30*time.Second)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Add a simple ping endpoint
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &HealthCheckServer{
		handler: handler,
		server:  server,
	}
}

// Start starts the health check server.
func (s *HealthCheckServer) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully stops the health check server.
func (s *HealthCheckServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
