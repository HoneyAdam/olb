package balancer

import (
	"sync/atomic"

	"github.com/openloadbalancer/olb/internal/backend"
)

// RoundRobin implements a simple round-robin load balancing algorithm.
// It maintains an atomic counter to select backends in rotation.
type RoundRobin struct {
	counter atomic.Uint64
}

// NewRoundRobin creates a new RoundRobin balancer.
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

// Name returns the name of the balancer.
func (rr *RoundRobin) Name() string {
	return "round_robin"
}

// Next selects the next backend using round-robin rotation.
// Returns nil if no backends are available.
func (rr *RoundRobin) Next(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	// Atomically increment and get the index
	count := rr.counter.Add(1)
	index := int((count - 1) % uint64(len(backends)))

	return backends[index]
}

// Add is a no-op for RoundRobin (stateless algorithm).
func (rr *RoundRobin) Add(backend *backend.Backend) {
	// No-op: round robin doesn't maintain state per backend
}

// Remove is a no-op for RoundRobin (stateless algorithm).
func (rr *RoundRobin) Remove(id string) {
	// No-op: round robin doesn't maintain state per backend
}

// Update is a no-op for RoundRobin (stateless algorithm).
func (rr *RoundRobin) Update(backend *backend.Backend) {
	// No-op: round robin doesn't maintain state per backend
}
