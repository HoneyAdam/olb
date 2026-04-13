package balancer

import (
	"sync"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/pkg/utils"
)

// PowerOfTwo implements the Power of Two Choices (P2C) load balancing algorithm.
// It randomly selects 2 backends and picks the one with fewer active connections.
// This provides a good balance between simplicity and load distribution.
type PowerOfTwo struct {
	backends []*backend.Backend
	rnd      *utils.FastRand
	mu       sync.RWMutex
}

// NewPowerOfTwo creates a new PowerOfTwo balancer.
func NewPowerOfTwo() *PowerOfTwo {
	return &PowerOfTwo{
		backends: make([]*backend.Backend, 0),
		rnd:      utils.NewFastRand(),
	}
}

// Name returns the name of the balancer.
func (p *PowerOfTwo) Name() string {
	return "power_of_two"
}

// Next selects the next backend using the Power of Two Choices algorithm.
// It randomly picks 2 backends and selects the one with fewer active connections.
// If backends have the same number of connections, picks one randomly.
// Returns nil if no backends are available.
func (p *PowerOfTwo) Next(ctx *RequestContext, backends []*backend.Backend) *backend.Backend {
	n := len(backends)
	if n == 0 {
		return nil
	}

	// If only one backend, return it immediately
	if n == 1 {
		return backends[0]
	}

	// Pick 2 random backends
	// Use different random values to ensure we might pick the same backend twice,
	// but in that case we just return that backend
	random1 := p.rnd.Int63()
	idx1 := int(random1 % int64(n))
	b1 := backends[idx1]

	random2 := p.rnd.Int63()
	idx2 := int(random2 % int64(n))
	b2 := backends[idx2]

	// If we picked the same backend twice, return it
	if idx1 == idx2 {
		return b1
	}

	// Compare active connections and return the one with fewer
	conns1 := b1.ActiveConns()
	conns2 := b2.ActiveConns()

	if conns1 < conns2 {
		return b1
	}
	if conns2 < conns1 {
		return b2
	}

	// Equal connections - pick randomly between the two
	if p.rnd.Int63()%2 == 0 {
		return b1
	}
	return b2
}

// Add adds a backend to the balancer's internal tracking.
func (p *PowerOfTwo) Add(b *backend.Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already exists
	for _, existing := range p.backends {
		if existing.ID == b.ID {
			return
		}
	}
	p.backends = append(p.backends, b)
}

// Remove removes a backend from the balancer's internal tracking.
func (p *PowerOfTwo) Remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, b := range p.backends {
		if b.ID == id {
			// Remove by swapping with last element
			p.backends[i] = p.backends[len(p.backends)-1]
			p.backends = p.backends[:len(p.backends)-1]
			return
		}
	}
}

// Update is a no-op for PowerOfTwo (stateless algorithm).
func (p *PowerOfTwo) Update(b *backend.Backend) {
	// No-op: power of two doesn't maintain per-backend state beyond the list
}
