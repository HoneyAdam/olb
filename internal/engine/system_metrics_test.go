package engine

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/conn"
	"github.com/openloadbalancer/olb/internal/metrics"
)

func TestRegisterSystemMetrics(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	if sm == nil {
		t.Fatal("registerSystemMetrics returned nil")
	}

	// Verify all gauges and counters are non-nil
	checks := []struct {
		name string
		got  interface{}
	}{
		{"backendTotal", sm.backendTotal},
		{"backendHealthy", sm.backendHealthy},
		{"backendUnhealthy", sm.backendUnhealthy},
		{"activeConnections", sm.activeConnections},
		{"poolIdleConns", sm.poolIdleConns},
		{"poolActiveConns", sm.poolActiveConns},
		{"poolHitsTotal", sm.poolHitsTotal},
		{"poolMissesTotal", sm.poolMissesTotal},
		{"poolEvictionsTotal", sm.poolEvictionsTotal},
		{"healthChecksTotal", sm.healthChecksTotal},
	}
	for _, c := range checks {
		if c.got == nil {
			t.Errorf("%s is nil", c.name)
		}
	}
}

func TestUpdateSystemMetrics_NilPoolMgr(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	// Should not panic with nil poolMgr
	sm.updateSystemMetrics(nil, nil, nil)
}

func TestUpdateSystemMetrics_BackendCounts(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	poolMgr := backend.NewPoolManager()
	pool := backend.NewPool("test-pool", "round_robin")
	b1 := backend.NewBackend("b1", "localhost:3001")
	b2 := backend.NewBackend("b2", "localhost:3002")
	b1.SetState(backend.StateUp)
	b2.SetState(backend.StateUp)
	pool.AddBackend(b1)
	pool.AddBackend(b2)
	poolMgr.AddPool(pool)

	sm.updateSystemMetrics(poolMgr, nil, nil)

	if got := sm.backendTotal.Get(); got != 2 {
		t.Errorf("backendTotal = %v, want 2", got)
	}
	if got := sm.backendHealthy.Get(); got != 2 {
		t.Errorf("backendHealthy = %v, want 2", got)
	}
	if got := sm.backendUnhealthy.Get(); got != 0 {
		t.Errorf("backendUnhealthy = %v, want 0", got)
	}
}

func TestUpdateSystemMetrics_ConnectionPoolGauges(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	poolMgr := backend.NewPoolManager()
	pool := backend.NewPool("test-pool", "round_robin")
	pool.AddBackend(backend.NewBackend("b1", "localhost:3001"))
	poolMgr.AddPool(pool)

	connPoolMgr := conn.NewPoolManager(nil)

	sm.updateSystemMetrics(poolMgr, nil, connPoolMgr)

	// No pools created in connPoolMgr, so gauges should be 0
	if got := sm.poolIdleConns.Get(); got != 0 {
		t.Errorf("poolIdleConns = %v, want 0", got)
	}
	if got := sm.poolActiveConns.Get(); got != 0 {
		t.Errorf("poolActiveConns = %v, want 0", got)
	}
}

func TestUpdateSystemMetrics_NilConnPoolMgr(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	poolMgr := backend.NewPoolManager()
	pool := backend.NewPool("test-pool", "round_robin")
	pool.AddBackend(backend.NewBackend("b1", "localhost:3001"))
	poolMgr.AddPool(pool)

	// Should not panic with nil connPoolMgr
	sm.updateSystemMetrics(poolMgr, nil, nil)

	if got := sm.backendTotal.Get(); got != 1 {
		t.Errorf("backendTotal = %v, want 1", got)
	}
}

func TestUpdateSystemMetrics_UnhealthyBackends(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	poolMgr := backend.NewPoolManager()
	pool := backend.NewPool("test-pool", "round_robin")
	b1 := backend.NewBackend("b1", "localhost:3001")
	b2 := backend.NewBackend("b2", "localhost:3002")
	b1.SetState(backend.StateUp)
	b2.SetState(backend.StateDown)
	pool.AddBackend(b1)
	pool.AddBackend(b2)
	poolMgr.AddPool(pool)

	sm.updateSystemMetrics(poolMgr, nil, nil)

	if got := sm.backendTotal.Get(); got != 2 {
		t.Errorf("backendTotal = %v, want 2", got)
	}
	if got := sm.backendHealthy.Get(); got != 1 {
		t.Errorf("backendHealthy = %v, want 1", got)
	}
	if got := sm.backendUnhealthy.Get(); got != 1 {
		t.Errorf("backendUnhealthy = %v, want 1", got)
	}
}

func TestUpdateSystemMetrics_MultipleRefreshes(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	poolMgr := backend.NewPoolManager()
	pool := backend.NewPool("test-pool", "round_robin")
	pool.AddBackend(backend.NewBackend("b1", "localhost:3001"))
	poolMgr.AddPool(pool)

	// First refresh
	sm.updateSystemMetrics(poolMgr, nil, nil)
	if got := sm.backendTotal.Get(); got != 1 {
		t.Errorf("backendTotal after first refresh = %v, want 1", got)
	}

	// Second refresh should work fine
	sm.updateSystemMetrics(poolMgr, nil, nil)
	if got := sm.backendTotal.Get(); got != 1 {
		t.Errorf("backendTotal after second refresh = %v, want 1", got)
	}
}
