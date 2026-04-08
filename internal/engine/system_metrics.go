package engine

import (
	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/health"
	"github.com/openloadbalancer/olb/internal/metrics"
)

// systemMetrics holds gauges and counters that reflect system-level state.
// They are updated periodically by the engine's metrics refresh goroutine.
type systemMetrics struct {
	// Backend pool gauges
	backendTotal     *metrics.Gauge
	backendHealthy   *metrics.Gauge
	backendUnhealthy *metrics.Gauge

	// Connection gauges
	activeConnections *metrics.Gauge

	// Health check counters
	healthChecksTotal *metrics.Counter

	// Track last health check counter values to compute deltas
	lastHealthyCount   int
	lastUnhealthyCount int
}

// registerSystemMetrics creates and registers system-level metrics in the
// shared registry so they appear in the Prometheus /metrics endpoint.
func registerSystemMetrics(registry *metrics.Registry) *systemMetrics {
	sm := &systemMetrics{
		backendTotal: metrics.NewGauge(
			"olb_backends_total",
			"Total number of configured backends",
		),
		backendHealthy: metrics.NewGauge(
			"olb_backends_healthy",
			"Number of healthy backends",
		),
		backendUnhealthy: metrics.NewGauge(
			"olb_backends_unhealthy",
			"Number of unhealthy backends",
		),
		activeConnections: metrics.NewGauge(
			"olb_active_connections",
			"Number of active connections across all backends",
		),
		healthChecksTotal: metrics.NewCounter(
			"olb_health_checks_total",
			"Total number of health check results",
		),
	}

	registry.RegisterGauge(sm.backendTotal)
	registry.RegisterGauge(sm.backendHealthy)
	registry.RegisterGauge(sm.backendUnhealthy)
	registry.RegisterGauge(sm.activeConnections)
	registry.RegisterCounter(sm.healthChecksTotal)

	return sm
}

// updateSystemMetrics refreshes system gauges from the current state of
// backend pools and health checkers.
func (sm *systemMetrics) updateSystemMetrics(
	poolMgr *backend.PoolManager,
	healthChecker *health.Checker,
) {
	if poolMgr == nil {
		return
	}

	var total, healthy, unhealthy float64
	for _, pool := range poolMgr.GetAllPools() {
		for _, b := range pool.GetAllBackends() {
			total++
			if b.State() == backend.StateUp {
				healthy++
			} else {
				unhealthy++
			}
		}
	}

	sm.backendTotal.Set(total)
	sm.backendHealthy.Set(healthy)
	sm.backendUnhealthy.Set(unhealthy)

	// Health check counts — increment counter by delta since last refresh
	if healthChecker != nil {
		hc := healthChecker.CountHealthy()
		hu := healthChecker.CountUnhealthy()
		delta := (hc + hu) - (sm.lastHealthyCount + sm.lastUnhealthyCount)
		if delta > 0 {
			sm.healthChecksTotal.Add(int64(delta))
		}
		sm.lastHealthyCount = hc
		sm.lastUnhealthyCount = hu
	}
}
