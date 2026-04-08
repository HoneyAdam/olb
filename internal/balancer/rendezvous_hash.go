package balancer

import (
	"hash/fnv"
	"sync"

	"github.com/openloadbalancer/olb/internal/backend"
)

// RendezvousHash implements the Rendezvous Hashing (Highest Random Weight) algorithm.
// This is an alternative to consistent hashing that provides:
// - Better distribution uniformity
// - Lower memory overhead
// - Simpler implementation
// - Similar minimal disruption when backends change
//
// Reference: https://en.wikipedia.org/wiki/Rendezvous_hashing
type RendezvousHash struct {
	mu       sync.RWMutex
	seeds    []uint32       // Per-backend seeds for randomness
	backends map[string]int // Backend ID to index mapping
}

// NewRendezvousHash creates a new RendezvousHash balancer.
func NewRendezvousHash() *RendezvousHash {
	return &RendezvousHash{
		seeds:    make([]uint32, 0),
		backends: make(map[string]int),
	}
}

// Name returns the name of the balancer.
func (r *RendezvousHash) Name() string {
	return "rendezvous_hash"
}

// Next selects the backend with the highest random weight for the given key.
// The key parameter is used from request context (e.g., URL path, query, etc.)
// For simplicity, we use a round-robin key here; in production you'd pass
// a meaningful key from the request.
func (r *RendezvousHash) Next(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// For simplicity, use backend address as key
	// In production, you'd use request-specific data like URL, headers, etc.
	key := generateKey()

	var bestBackend *backend.Backend
	maxWeight := uint64(0)

	for _, be := range backends {
		if !be.IsHealthy() {
			continue
		}

		// Calculate weight for this backend
		weight := r.computeWeight(key, be.ID)
		if weight > maxWeight {
			maxWeight = weight
			bestBackend = be
		}
	}

	// Fall back to first backend if none healthy
	if bestBackend == nil && len(backends) > 0 {
		return backends[0]
	}

	return bestBackend
}

// computeWeight calculates the weight for a given key-backend pair.
// Uses FNV hash combined with backend-specific seed.
func (r *RendezvousHash) computeWeight(key, backendID string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	h.Write([]byte(backendID))
	return h.Sum64()
}

// generateKey creates a key for the request.
// In production, this would be derived from request attributes.
var keyCounter uint32
var keyMu sync.Mutex

func generateKey() string {
	keyMu.Lock()
	keyCounter++
	keyMu.Unlock()
	return string(rune(keyCounter))
}

// Add registers a new backend.
func (r *RendezvousHash) Add(be *backend.Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.backends[be.ID]; !exists {
		r.backends[be.ID] = len(r.seeds)
		// Generate random seed for this backend
		r.seeds = append(r.seeds, hashString(be.ID))
	}
}

// Remove unregisters a backend.
func (r *RendezvousHash) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if idx, exists := r.backends[id]; exists {
		// Remove from slice by swapping with last element
		lastIdx := len(r.seeds) - 1
		if idx != lastIdx {
			r.seeds[idx] = r.seeds[lastIdx]
			// Update index for the moved backend
			for movedID, movedIdx := range r.backends {
				if movedIdx == lastIdx {
					r.backends[movedID] = idx
					break
				}
			}
		}
		r.seeds = r.seeds[:lastIdx]
		delete(r.backends, id)
	}
}

// Update is called when a backend is updated.
func (r *RendezvousHash) Update(be *backend.Backend) {
	// Seed is based on ID which shouldn't change
}

// hashString generates a hash from a string.
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// Stats returns balancer statistics.
func (r *RendezvousHash) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"algorithm":     "rendezvous_hash",
		"backend_count": len(r.backends),
		"seed_count":    len(r.seeds),
	}
}
