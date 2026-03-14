// Package admin provides the Admin API server for OpenLoadBalancer.
package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/health"
	"github.com/openloadbalancer/olb/internal/metrics"
	"github.com/openloadbalancer/olb/internal/router"
)

// Server provides the Admin API HTTP server.
type Server struct {
	addr      string
	server    *http.Server
	config    *AuthConfig
	startTime time.Time

	// Component references (interfaces)
	poolManager   PoolManager
	router        Router
	healthChecker HealthChecker
	metrics       Metrics

	// Callbacks
	onReload func() error

	// State
	mu    sync.RWMutex
	state string
}

// Config holds the server configuration.
type Config struct {
	Address       string
	Auth          *AuthConfig
	PoolManager   PoolManager
	Router        Router
	HealthChecker HealthChecker
	Metrics       Metrics
	OnReload      func() error
}

// PoolManager interface for backend pool operations.
type PoolManager interface {
	GetAllPools() []*backend.Pool
	GetPool(name string) *backend.Pool
}

// Router interface for route operations.
type Router interface {
	Routes() []*router.Route
}

// HealthChecker interface for health check operations.
type HealthChecker interface {
	ListStatuses() map[string]health.Status
	GetResult(backendID string) *health.Result
}

// Metrics interface for metrics operations.
type Metrics interface {
	GetAllMetrics() map[string]interface{}
	PrometheusFormat() string
}

// NewServer creates a new Admin API server.
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	s := &Server{
		addr:          config.Address,
		config:        config.Auth,
		poolManager:   config.PoolManager,
		router:        config.Router,
		healthChecker: config.HealthChecker,
		metrics:       config.Metrics,
		onReload:      config.OnReload,
		startTime:     time.Now(),
		state:         "running",
	}

	s.setupRoutes()
	return s, nil
}

// setupRoutes configures the HTTP routes.
func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	// System endpoints
	mux.HandleFunc("/api/v1/system/info", s.getSystemInfo)
	mux.HandleFunc("/api/v1/system/health", s.getSystemHealth)
	mux.HandleFunc("/api/v1/system/reload", s.reloadConfig)

	// Backend endpoints
	mux.HandleFunc("/api/v1/backends", s.listBackends)
	mux.HandleFunc("/api/v1/backends/", s.handleBackendDetail)

	// Route endpoints
	mux.HandleFunc("/api/v1/routes", s.listRoutes)

	// Health endpoint
	mux.HandleFunc("/api/v1/health", s.getHealthStatus)

	// Metrics endpoints
	mux.HandleFunc("/api/v1/metrics", s.getMetricsJSON)
	mux.HandleFunc("/metrics", s.getMetricsPrometheus)

	// Apply auth middleware if configured
	var handler http.Handler = mux
	if s.config != nil {
		handler = AuthMiddleware(s.config)(mux)
	}

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// Start starts the Admin API server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully stops the Admin API server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.state = "stopping"
	s.mu.Unlock()

	return s.server.Shutdown(ctx)
}

// GetState returns the current server state.
func (s *Server) GetState() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// handleBackendDetail handles requests to /api/v1/backends/...
func (s *Server) handleBackendDetail(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Check if this is a drain request
	if strings.HasSuffix(path, "/drain") {
		if r.Method == http.MethodPost {
			s.drainBackend(w, r)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		}
		return
	}

	// Count path segments to determine if it's a pool or backend request
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")

	// /api/v1/backends/:pool (4 parts: api, v1, backends, :pool)
	// /api/v1/backends/:pool/:backend (5 parts)
	if len(parts) == 4 {
		// Pool-level request
		switch r.Method {
		case http.MethodGet:
			s.getPool(w, r)
		case http.MethodPost:
			s.addBackend(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		}
	} else if len(parts) >= 5 {
		// Backend-level request
		switch r.Method {
		case http.MethodGet:
			// Could implement get single backend here
			writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "get single backend not implemented")
		case http.MethodDelete:
			s.removeBackend(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		}
	} else {
		writeError(w, http.StatusBadRequest, "INVALID_PATH", "invalid path")
	}
}

// defaultMetrics implements the Metrics interface using the default registry.
type defaultMetrics struct {
	registry *metrics.Registry
}

// NewDefaultMetrics creates a new default metrics provider.
func NewDefaultMetrics(registry *metrics.Registry) Metrics {
	if registry == nil {
		registry = metrics.DefaultRegistry
	}
	return &defaultMetrics{registry: registry}
}

// GetAllMetrics returns all metrics in JSON-compatible format.
func (m *defaultMetrics) GetAllMetrics() map[string]interface{} {
	result := make(map[string]interface{})

	var buf bytes.Buffer
	handler := metrics.NewJSONHandler(m.registry)
	if err := handler.WriteTo(&buf); err == nil {
		// Parse the JSON output
		var metrics map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &metrics); err == nil {
			return metrics
		}
	}

	return result
}

// PrometheusFormat returns metrics in Prometheus exposition format.
func (m *defaultMetrics) PrometheusFormat() string {
	var buf bytes.Buffer
	handler := metrics.NewPrometheusHandler(m.registry)
	handler.WriteTo(&buf)
	return buf.String()
}
