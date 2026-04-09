// Package middleware provides HTTP middleware infrastructure for OpenLoadBalancer.
package middleware

import (
	"net/http"
	"sort"
	"sync"
)

// Priority constants for standard middleware ordering.
// Lower numbers execute earlier in the chain.
const (
	PrioritySecurity  = 100  // Security headers, IP filtering, WAF
	PriorityAuth      = 200  // Authentication, JWT validation
	PriorityRealIP    = 300  // Real IP extraction from headers
	PriorityRequestID = 400  // Request ID generation
	PriorityRateLimit = 500  // Rate limiting
	PriorityCORS      = 600  // CORS handling
	PriorityHeaders   = 700  // Header manipulation
	PriorityCompress  = 800  // Compression (gzip, brotli)
	PriorityMetrics   = 900  // Metrics collection
	PriorityAccessLog = 1000 // Access logging
)

// Middleware is the interface for HTTP middleware components.
type Middleware interface {
	// Name returns the unique name of this middleware.
	Name() string

	// Priority returns the execution priority (lower = earlier).
	Priority() int

	// Wrap wraps the next handler with this middleware.
	Wrap(next http.Handler) http.Handler
}

// Chain is a builder for middleware chains.
type Chain struct {
	mu          sync.RWMutex
	middlewares []Middleware
	sorted      []Middleware // cached sorted middleware, invalidated on Use/Remove/Clear
}

// NewChain creates a new empty middleware chain.
func NewChain() *Chain {
	return &Chain{
		middlewares: make([]Middleware, 0),
	}
}

// Use adds a middleware to the chain.
// Middleware are sorted by priority (lower first) when the chain is built.
func (c *Chain) Use(mw Middleware) *Chain {
	if mw == nil {
		return c
	}
	c.mu.Lock()
	c.middlewares = append(c.middlewares, mw)
	c.sorted = nil // invalidate cache
	c.mu.Unlock()
	return c
}

// sortedMiddlewares returns the cached sorted middleware list, building it if needed.
func (c *Chain) sortedMiddlewares() []Middleware {
	c.mu.RLock()
	if c.sorted != nil {
		s := c.sorted
		c.mu.RUnlock()
		return s
	}
	c.mu.RUnlock()

	// Build sorted list
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.sorted != nil {
		return c.sorted
	}

	sorted := make([]Middleware, len(c.middlewares))
	copy(sorted, c.middlewares)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})
	c.sorted = sorted
	return sorted
}

// Then builds the middleware chain and returns the final handler.
// The handler is wrapped by all middleware in priority order.
// The sorted middleware list is cached after the first call.
func (c *Chain) Then(handler http.Handler) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}

	sorted := c.sortedMiddlewares()

	// Wrap handler in reverse order (last middleware wraps the handler,
	// then second-to-last wraps that, etc.)
	result := handler
	for i := len(sorted) - 1; i >= 0; i-- {
		result = sorted[i].Wrap(result)
	}

	return result
}

// ThenFunc is a convenience method that wraps an http.HandlerFunc.
func (c *Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}

// Clone creates a copy of the chain that can be modified independently.
// This is useful for creating per-route middleware chains.
func (c *Chain) Clone() *Chain {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cloned := &Chain{
		middlewares: make([]Middleware, len(c.middlewares)),
	}
	copy(cloned.middlewares, c.middlewares)
	return cloned
}

// Len returns the number of middleware in the chain.
func (c *Chain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.middlewares)
}

// Middlewares returns a copy of the middleware slice (unsorted).
func (c *Chain) Middlewares() []Middleware {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Middleware, len(c.middlewares))
	copy(result, c.middlewares)
	return result
}

// Remove removes a middleware by name.
func (c *Chain) Remove(name string) *Chain {
	c.mu.Lock()
	defer c.mu.Unlock()
	filtered := make([]Middleware, 0, len(c.middlewares))
	for _, mw := range c.middlewares {
		if mw.Name() != name {
			filtered = append(filtered, mw)
		}
	}
	c.middlewares = filtered
	c.sorted = nil // invalidate cache
	return c
}

// Get returns a middleware by name, or nil if not found.
func (c *Chain) Get(name string) Middleware {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, mw := range c.middlewares {
		if mw.Name() == name {
			return mw
		}
	}
	return nil
}

// Clear removes all middleware from the chain.
func (c *Chain) Clear() *Chain {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = c.middlewares[:0]
	c.sorted = nil // invalidate cache
	return c
}
