package balancer

import (
	"testing"
)

func TestRegistry_IPHash(t *testing.T) {
	// Test ip_hash registration
	b := New("ip_hash")
	if b == nil {
		t.Fatal("ip_hash not registered")
	}
	if b.Name() != "ip_hash" {
		t.Errorf("wrong name: got %s, want ip_hash", b.Name())
	}
}

func TestRegistry_IPHashAlias(t *testing.T) {
	// Test iphash alias
	b := New("iphash")
	if b == nil {
		t.Fatal("iphash alias not registered")
	}
	if b.Name() != "ip_hash" {
		t.Errorf("wrong name: got %s, want ip_hash", b.Name())
	}
}

func TestRegistry_AllBalancers(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Error("No balancers registered")
	}

	// Check that ip_hash is in the list
	found := false
	for _, name := range names {
		if name == "ip_hash" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ip_hash not found in registered balancers")
	}
}
