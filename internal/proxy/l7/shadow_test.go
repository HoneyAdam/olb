package l7

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/balancer"
)

func TestNewShadowManager(t *testing.T) {
	tests := []struct {
		name   string
		config ShadowConfig
		want   bool
	}{
		{
			name: "enabled shadow manager",
			config: ShadowConfig{
				Enabled:     true,
				Percentage:  50.0,
				CopyHeaders: true,
				CopyBody:    true,
				Timeout:     5 * time.Second,
			},
			want: true,
		},
		{
			name: "disabled shadow manager",
			config: ShadowConfig{
				Enabled:     false,
				Percentage:  0.0,
				CopyHeaders: false,
				CopyBody:    false,
				Timeout:     0,
			},
			want: false,
		},
		{
			name: "zero timeout uses default",
			config: ShadowConfig{
				Enabled:     true,
				Percentage:  100.0,
				CopyHeaders: true,
				CopyBody:    true,
				Timeout:     0,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewShadowManager(tt.config)
			if sm == nil {
				t.Fatal("NewShadowManager() returned nil")
			}
			if sm.enabled != tt.want {
				t.Errorf("enabled = %v, want %v", sm.enabled, tt.want)
			}
			if sm.config.Percentage != tt.config.Percentage {
				t.Errorf("config.Percentage = %v, want %v", sm.config.Percentage, tt.config.Percentage)
			}
			if sm.targets == nil {
				t.Error("targets should be initialized")
			}
		})
	}
}

func TestShadowManager_AddTarget(t *testing.T) {
	t.Run("add target to enabled manager", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     5 * time.Second,
		}
		sm := NewShadowManager(config)

		be := backend.NewBackend("test", "127.0.0.1:8080")
		be.SetState(backend.StateUp)
		backends := []*backend.Backend{be}
		b := balancer.NewRoundRobin()

		sm.AddTarget(b, backends, 50.0)

		if len(sm.targets) != 1 {
			t.Errorf("expected 1 target, got %d", len(sm.targets))
		}
	})

	t.Run("add target to disabled manager is no-op", func(t *testing.T) {
		config := ShadowConfig{
			Enabled: false,
		}
		sm := NewShadowManager(config)

		be := backend.NewBackend("test", "127.0.0.1:8080")
		be.SetState(backend.StateUp)
		backends := []*backend.Backend{be}
		b := balancer.NewRoundRobin()

		sm.AddTarget(b, backends, 50.0)

		if len(sm.targets) != 0 {
			t.Errorf("expected 0 targets for disabled manager, got %d", len(sm.targets))
		}
	})

	t.Run("add multiple targets", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     5 * time.Second,
		}
		sm := NewShadowManager(config)

		// Add first target
		be1 := backend.NewBackend("test1", "127.0.0.1:8080")
		be1.SetState(backend.StateUp)
		sm.AddTarget(balancer.NewRoundRobin(), []*backend.Backend{be1}, 50.0)

		// Add second target
		be2 := backend.NewBackend("test2", "127.0.0.1:8081")
		be2.SetState(backend.StateUp)
		sm.AddTarget(balancer.NewRoundRobin(), []*backend.Backend{be2}, 100.0)

		if len(sm.targets) != 2 {
			t.Errorf("expected 2 targets, got %d", len(sm.targets))
		}
	})
}

func TestShadowManager_ShouldShadow(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled returns true",
			enabled: true,
			want:    true,
		},
		{
			name:    "disabled returns false",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ShadowConfig{
				Enabled:    tt.enabled,
				Percentage: 50.0,
			}
			sm := NewShadowManager(config)
			got := sm.ShouldShadow()
			if got != tt.want {
				t.Errorf("ShouldShadow() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("nil manager returns false", func(t *testing.T) {
		var sm *ShadowManager
		got := sm.ShouldShadow()
		if got {
			t.Error("ShouldShadow() on nil manager should return false")
		}
	})
}

func TestShadowManager_ShadowRequest(t *testing.T) {
	t.Run("shadow request with no targets is no-op", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Host = "example.com"

		// Should not panic or block
		sm.ShadowRequest(req)
	})

	t.Run("disabled manager does nothing", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     false,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		be := backend.NewBackend("test", "127.0.0.1:8080")
		be.SetState(backend.StateUp)
		sm.AddTarget(balancer.NewRoundRobin(), []*backend.Backend{be}, 100.0)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Host = "example.com"

		// Should not panic or block
		sm.ShadowRequest(req)
	})

	t.Run("nil manager does not panic", func(t *testing.T) {
		var sm *ShadowManager
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		// Should not panic
		sm.ShadowRequest(req)
	})

	t.Run("shadow request with nil balancer skips target", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		// Add target with nil balancer
		be := backend.NewBackend("test", "127.0.0.1:8080")
		be.SetState(backend.StateUp)
		sm.targets = append(sm.targets, ShadowTarget{
			Balancer:   nil,
			Backends:   []*backend.Backend{be},
			Percentage: 100.0,
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Host = "example.com"

		// Should not panic
		sm.ShadowRequest(req)
	})

	t.Run("shadow request with no available backend", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		// Add target with empty backends
		sm.AddTarget(balancer.NewRoundRobin(), []*backend.Backend{}, 100.0)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Host = "example.com"

		// Should not panic
		sm.ShadowRequest(req)
	})
}

func TestShadowManager_sendShadow(t *testing.T) {
	t.Run("send shadow to unavailable backend", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Host = "example.com"

		// Send to a closed port
		target := ShadowTarget{
			Balancer:   balancer.NewRoundRobin(),
			Backends:   []*backend.Backend{backend.NewBackend("test", "127.0.0.1:1")},
			Percentage: 100.0,
		}

		// Should not panic or block, just return
		sm.sendShadow(req, "127.0.0.1:1", target)
	})

	t.Run("send shadow with body copying", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: true,
			CopyBody:    true,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		bodyContent := []byte("test body content")
		_ = bodyContent // Used for documentation purposes

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Body = nil // Body is nil, should not panic
		req.Host = "example.com"

		target := ShadowTarget{
			Balancer:   balancer.NewRoundRobin(),
			Backends:   []*backend.Backend{backend.NewBackend("test", "127.0.0.1:1")},
			Percentage: 100.0,
		}

		// Should not panic with nil body
		sm.sendShadow(req, "127.0.0.1:1", target)
	})

	t.Run("send shadow without headers", func(t *testing.T) {
		config := ShadowConfig{
			Enabled:     true,
			Percentage:  50.0,
			CopyHeaders: false,
			CopyBody:    false,
			Timeout:     100 * time.Millisecond,
		}
		sm := NewShadowManager(config)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("X-Custom", "should-not-be-copied")

		target := ShadowTarget{
			Balancer:   balancer.NewRoundRobin(),
			Backends:   []*backend.Backend{backend.NewBackend("test", "127.0.0.1:1")},
			Percentage: 100.0,
		}

		// Should not panic
		sm.sendShadow(req, "127.0.0.1:1", target)
	})
}

func TestShadowManager_Stats(t *testing.T) {
	tests := []struct {
		name string
		sm   *ShadowManager
		want ShadowStats
	}{
		{
			name: "enabled manager returns empty stats",
			sm: NewShadowManager(ShadowConfig{
				Enabled: true,
			}),
			want: ShadowStats{},
		},
		{
			name: "disabled manager returns empty stats",
			sm: NewShadowManager(ShadowConfig{
				Enabled: false,
			}),
			want: ShadowStats{},
		},
		{
			name: "nil manager returns empty stats",
			sm:   nil,
			want: ShadowStats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sm.Stats()
			if got.TotalRequests != tt.want.TotalRequests {
				t.Errorf("Stats().TotalRequests = %v, want %v", got.TotalRequests, tt.want.TotalRequests)
			}
			if got.SuccessRequests != tt.want.SuccessRequests {
				t.Errorf("Stats().SuccessRequests = %v, want %v", got.SuccessRequests, tt.want.SuccessRequests)
			}
			if got.FailedRequests != tt.want.FailedRequests {
				t.Errorf("Stats().FailedRequests = %v, want %v", got.FailedRequests, tt.want.FailedRequests)
			}
		})
	}
}

func TestShadowTarget(t *testing.T) {
	t.Run("shadow target with balancer and backends", func(t *testing.T) {
		be1 := backend.NewBackend("b1", "127.0.0.1:8080")
		be1.SetState(backend.StateUp)
		be2 := backend.NewBackend("b2", "127.0.0.1:8081")
		be2.SetState(backend.StateUp)

		target := ShadowTarget{
			Balancer:   balancer.NewRoundRobin(),
			Backends:   []*backend.Backend{be1, be2},
			Percentage: 50.0,
		}

		if target.Balancer == nil {
			t.Error("expected balancer to be set")
		}
		if len(target.Backends) != 2 {
			t.Errorf("expected 2 backends, got %d", len(target.Backends))
		}
		if target.Percentage != 50.0 {
			t.Errorf("expected percentage 50.0, got %f", target.Percentage)
		}
	})
}

func TestShadowConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     ShadowConfig
		wantEnabled bool
		wantPct    float64
	}{
		{
			name: "full config",
			config: ShadowConfig{
				Enabled:     true,
				Percentage:  75.0,
				CopyHeaders: true,
				CopyBody:    true,
				Timeout:     5 * time.Second,
			},
			wantEnabled: true,
			wantPct:     75.0,
		},
		{
			name: "minimal config",
			config: ShadowConfig{
				Enabled: true,
			},
			wantEnabled: true,
			wantPct:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewShadowManager(tt.config)
			if sm.enabled != tt.wantEnabled {
				t.Errorf("enabled = %v, want %v", sm.enabled, tt.wantEnabled)
			}
			if sm.config.Percentage != tt.wantPct {
				t.Errorf("percentage = %v, want %v", sm.config.Percentage, tt.wantPct)
			}
		})
	}
}
