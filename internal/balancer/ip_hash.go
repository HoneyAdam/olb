package balancer

import (
	"hash/fnv"
	"net"
	"sync"

	"github.com/openloadbalancer/olb/internal/backend"
)

// IPHash implements an IP hash-based load balancing algorithm.
// It uses FNV-1a hash algorithm to consistently map client IPs to backends,
// providing session affinity (same IP always maps to same backend).
type IPHash struct {
	mu       sync.RWMutex
	backends []*backend.Backend
}

// NewIPHash creates a new IPHash balancer.
func NewIPHash() *IPHash {
	return &IPHash{
		backends: make([]*backend.Backend, 0),
	}
}

// Name returns the name of the balancer.
func (ih *IPHash) Name() string {
	return "ip_hash"
}

// Next selects the next backend based on the client IP hash.
// The client IP should be stored in the backend's metadata or extracted from context.
// For proper IP-based routing, use NextWithIP with the actual client IP.
// Returns nil if no backends are available.
func (ih *IPHash) Next(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	// For interface compatibility, use empty IP which will hash to index 0
	// In practice, the caller should use NextWithIP with the actual client IP
	hash := ih.hashIP("")
	index := hash % uint32(len(backends))
	return backends[index]
}

// NextWithIP selects the next backend based on the client IP hash.
// This is the preferred method for IPHash balancer.
func (ih *IPHash) NextWithIP(backends []*backend.Backend, clientIP string) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	hash := ih.hashIP(clientIP)
	index := hash % uint32(len(backends))
	return backends[index]
}

// Add adds a backend to the balancer.
func (ih *IPHash) Add(b *backend.Backend) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	// Check if backend already exists
	for _, existing := range ih.backends {
		if existing.ID == b.ID {
			return
		}
	}
	ih.backends = append(ih.backends, b)
}

// Remove removes a backend from the balancer by ID.
func (ih *IPHash) Remove(id string) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	for i, b := range ih.backends {
		if b.ID == id {
			// Remove by swapping with last element and truncating
			ih.backends[i] = ih.backends[len(ih.backends)-1]
			ih.backends = ih.backends[:len(ih.backends)-1]
			return
		}
	}
}

// Update updates a backend's state in the balancer.
func (ih *IPHash) Update(b *backend.Backend) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	for i, existing := range ih.backends {
		if existing.ID == b.ID {
			ih.backends[i] = b
			return
		}
	}
}

// GetBackends returns a copy of the current backend list.
func (ih *IPHash) GetBackends() []*backend.Backend {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	result := make([]*backend.Backend, len(ih.backends))
	copy(result, ih.backends)
	return result
}

// hashIP computes the FNV-1a hash of the IP address.
// Returns 0 for empty or invalid IPs.
func (ih *IPHash) hashIP(ip string) uint32 {
	if ip == "" {
		return 0
	}

	// Parse and normalize the IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		// If not a valid IP, hash the string as-is
		h := fnv.New32a()
		h.Write([]byte(ip))
		return h.Sum32()
	}

	// Use the normalized 16-byte representation for IPv6
	// or 4-byte representation for IPv4
	h := fnv.New32a()

	// For IPv4, use the 4-byte representation
	if ipv4 := parsedIP.To4(); ipv4 != nil {
		h.Write(ipv4)
	} else {
		// For IPv6, use the full 16-byte representation
		h.Write(parsedIP.To16())
	}

	return h.Sum32()
}

// extractIP extracts the IP address from a host:port string.
// If no port is present, returns the input as-is.
func extractIP(hostport string) string {
	if hostport == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		// No port present, return as-is
		return hostport
	}
	return host
}
