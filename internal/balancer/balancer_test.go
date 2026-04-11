package balancer

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/backend"
)

func TestRegister(t *testing.T) {
	// Register should work for new names
	Register("test_balancer", func() Balancer { return NewRoundRobin() })

	// Registering duplicate should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register() with duplicate name should panic")
		}
	}()
	Register("test_balancer", func() Balancer { return NewRoundRobin() })
}

func TestGet(t *testing.T) {
	// Register a test balancer
	Register("test_get", func() Balancer {
		return &mockBalancer{name: "test_get"}
	})

	factory := Get("test_get")
	if factory == nil {
		t.Fatal("Get() returned nil for registered balancer")
	}

	balancer := factory()
	if balancer.Name() != "test_get" {
		t.Errorf("Created balancer name = %v, want %v", balancer.Name(), "test_get")
	}

	// Get non-existent
	factory = Get("nonexistent")
	if factory != nil {
		t.Error("Get() for non-existent balancer should return nil")
	}
}

func TestNew(t *testing.T) {
	// Test creating built-in balancers
	rr := New("round_robin")
	if rr == nil {
		t.Error("New(round_robin) should return a balancer")
	}
	if rr.Name() != "round_robin" {
		t.Errorf("New(round_robin).Name() = %v, want %v", rr.Name(), "round_robin")
	}

	wrr := New("weighted_round_robin")
	if wrr == nil {
		t.Error("New(weighted_round_robin) should return a balancer")
	}
	if wrr.Name() != "weighted_round_robin" {
		t.Errorf("New(weighted_round_robin).Name() = %v, want %v", wrr.Name(), "weighted_round_robin")
	}

	// Non-existent
	nilBalancer := New("nonexistent")
	if nilBalancer != nil {
		t.Error("New(nonexistent) should return nil")
	}
}

func TestNames(t *testing.T) {
	names := Names()

	// Should contain built-in balancers
	hasRR := false
	hasWRR := false
	for _, name := range names {
		if name == "round_robin" {
			hasRR = true
		}
		if name == "weighted_round_robin" {
			hasWRR = true
		}
	}

	if !hasRR {
		t.Error("Names() should include 'round_robin'")
	}
	if !hasWRR {
		t.Error("Names() should include 'weighted_round_robin'")
	}
}

func TestRoundRobinBuiltIn(t *testing.T) {
	// Verify round_robin is registered
	factory := Get("round_robin")
	if factory == nil {
		t.Fatal("round_robin should be registered")
	}

	balancer := factory()
	if balancer.Name() != "round_robin" {
		t.Errorf("round_robin balancer name = %v", balancer.Name())
	}
}

func TestWeightedRoundRobinBuiltIn(t *testing.T) {
	// Verify weighted_round_robin is registered
	factory := Get("weighted_round_robin")
	if factory == nil {
		t.Fatal("weighted_round_robin should be registered")
	}

	balancer := factory()
	if balancer.Name() != "weighted_round_robin" {
		t.Errorf("weighted_round_robin balancer name = %v", balancer.Name())
	}
}

func TestConsistentHashBuiltIn(t *testing.T) {
	// Verify consistent_hash is registered
	factory := Get("consistent_hash")
	if factory == nil {
		t.Fatal("consistent_hash should be registered")
	}

	balancer := factory()
	if balancer.Name() != "consistent_hash" {
		t.Errorf("consistent_hash balancer name = %v", balancer.Name())
	}

	// Verify aliases
	chAlias := Get("ch")
	if chAlias == nil {
		t.Error("ch alias should be registered")
	}

	ketamaAlias := Get("ketama")
	if ketamaAlias == nil {
		t.Error("ketama alias should be registered")
	}
}

// mockBalancer is a simple mock for testing
type mockBalancer struct {
	name string
}

func (m *mockBalancer) Name() string {
	return m.name
}

func (m *mockBalancer) Next(ctx *RequestContext, backends []*backend.Backend) *backend.Backend {
	if len(backends) > 0 {
		return backends[0]
	}
	return nil
}

func (m *mockBalancer) Add(backend *backend.Backend) {}

func (m *mockBalancer) Remove(id string) {}

func (m *mockBalancer) Update(backend *backend.Backend) {}

func TestInit_AllBalancersRegistered(t *testing.T) {
	// Verify all built-in balancers and their aliases are registered
	testCases := []struct {
		name         string
		expectedName string
	}{
		{"round_robin", "round_robin"},
		{"weighted_round_robin", "weighted_round_robin"},
		{"ip_hash", "ip_hash"},
		{"iphash", "ip_hash"},
		{"least_connections", "least_connections"},
		{"lc", "least_connections"},
		{"weighted_least_connections", "weighted_least_connections"},
		{"wlc", "weighted_least_connections"},
		{"random", "random"},
		{"weighted_random", "weighted_random"},
		{"wrandom", "weighted_random"},
		{"consistent_hash", "consistent_hash"},
		{"ch", "consistent_hash"},
		{"ketama", "consistent_hash"},
		{"power_of_two", "power_of_two"},
		{"p2c", "power_of_two"},
		{"least_response_time", "least_response_time"},
		{"lrt", "least_response_time"},
		{"weighted_least_response_time", "weighted_least_response_time"},
		{"wlrt", "weighted_least_response_time"},
		{"maglev", "maglev"},
		{"ring_hash", "ring_hash"},
		{"ringhash", "ring_hash"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := New(tc.name)
			if b == nil {
				t.Fatalf("New(%q) returned nil", tc.name)
			}
			if b.Name() != tc.expectedName {
				t.Errorf("New(%q).Name() = %q, want %q", tc.name, b.Name(), tc.expectedName)
			}
		})
	}
}
