// Package ratelimit provides distributed rate limiting with multiple backend stores.
package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Algorithm represents the rate limiting algorithm.
type Algorithm string

const (
	// TokenBucket uses the token bucket algorithm.
	TokenBucket Algorithm = "token_bucket"
	// SlidingWindow uses a sliding window counter.
	SlidingWindow Algorithm = "sliding_window"
	// FixedWindow uses a fixed time window counter.
	FixedWindow Algorithm = "fixed_window"
)

// Backend represents the storage backend for rate limiting.
type Backend string

const (
	// MemoryBackend uses in-memory storage (local to instance).
	MemoryBackend Backend = "memory"
	// RedisBackend uses Redis for distributed rate limiting.
	RedisBackend Backend = "redis"
)

// Config contains rate limiter configuration.
type Config struct {
	Algorithm   Algorithm       `json:"algorithm" yaml:"algorithm"`
	Backend     Backend         `json:"backend" yaml:"backend"`
	Rate        float64         `json:"rate" yaml:"rate"`               // Tokens per second (or requests per window)
	Burst       int             `json:"burst" yaml:"burst"`             // Maximum burst size
	Window      time.Duration   `json:"window" yaml:"window"`           // Window size for sliding/fixed window
	KeyPrefix   string          `json:"key_prefix" yaml:"key_prefix"`   // Key prefix for storage
	BackendOpts map[string]string `json:"backend_opts" yaml:"backend_opts"` // Backend-specific options
}

// DefaultConfig returns a default rate limiter configuration.
func DefaultConfig() *Config {
	return &Config{
		Algorithm: TokenBucket,
		Backend:   MemoryBackend,
		Rate:      100,
		Burst:     150,
		Window:    time.Minute,
		KeyPrefix: "ratelimit:",
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Rate <= 0 {
		return errors.New("rate must be positive")
	}
	if c.Burst <= 0 {
		return errors.New("burst must be positive")
	}
	if c.Window <= 0 {
		c.Window = time.Minute
	}
	return nil
}

// Result represents the result of a rate limit check.
type Result struct {
	Allowed       bool          `json:"allowed"`
	Remaining     int           `json:"remaining"`
	Limit         int           `json:"limit"`
	ResetTime     time.Time     `json:"reset_time"`
	RetryAfter    time.Duration `json:"retry_after"`
}

// Store is the interface for rate limit storage backends.
type Store interface {
	// Name returns the store name.
	Name() string
	// Allow checks if a request is allowed for the given key.
	Allow(key string, tokens int) (*Result, error)
	// AllowN checks if n requests are allowed for the given key.
	AllowN(key string, n int) (*Result, error)
	// Get returns the current state for a key.
	Get(key string) (*State, error)
	// Close closes the store.
	Close() error
}

// State represents the current rate limit state.
type State struct {
	Tokens     float64   `json:"tokens"`
	LastUpdate time.Time `json:"last_update"`
	Requests   int       `json:"requests"`
	WindowStart time.Time `json:"window_start"`
}

// memoryStore implements an in-memory token bucket store.
type memoryStore struct {
	config  *Config
	buckets map[string]*tokenBucket
	mu      sync.RWMutex
	closed  atomic.Bool
}

// tokenBucket represents a token bucket for rate limiting.
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// newMemoryStore creates a new in-memory store.
func newMemoryStore(config *Config) *memoryStore {
	return &memoryStore{
		config:  config,
		buckets: make(map[string]*tokenBucket),
	}
}

// Name returns the store name.
func (s *memoryStore) Name() string {
	return "memory"
}

// Allow checks if a request is allowed for the given key.
func (s *memoryStore) Allow(key string, tokens int) (*Result, error) {
	return s.AllowN(key, 1)
}

// AllowN checks if n requests are allowed for the given key.
func (s *memoryStore) AllowN(key string, n int) (*Result, error) {
	if s.closed.Load() {
		return nil, errors.New("store is closed")
	}

	s.mu.RLock()
	bucket, ok := s.buckets[key]
	if !ok {
		s.mu.RUnlock()
		s.mu.Lock()
		// Double-check after acquiring write lock
		bucket, ok = s.buckets[key]
		if !ok {
			bucket = &tokenBucket{
				tokens:     float64(s.config.Burst),
				lastUpdate: time.Now(),
			}
			s.buckets[key] = bucket
		}
		s.mu.Unlock()
	} else {
		s.mu.RUnlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.lastUpdate = now

	// Add tokens based on elapsed time
	bucket.tokens += elapsed * s.config.Rate
	if bucket.tokens > float64(s.config.Burst) {
		bucket.tokens = float64(s.config.Burst)
	}

	// Check if request can be satisfied
	allowed := false
	if bucket.tokens >= float64(n) {
		bucket.tokens -= float64(n)
		allowed = true
	}

	// Calculate reset time (when bucket will be full)
	tokensNeeded := float64(s.config.Burst) - bucket.tokens
	secondsToReset := tokensNeeded / s.config.Rate
	resetTime := now.Add(time.Duration(secondsToReset * float64(time.Second)))

	// Calculate retry after if not allowed
	var retryAfter time.Duration
	if !allowed {
		tokensNeeded := float64(n) - bucket.tokens
		retryAfter = time.Duration(tokensNeeded / s.config.Rate * float64(time.Second))
	}

	return &Result{
		Allowed:    allowed,
		Remaining:  int(bucket.tokens),
		Limit:      s.config.Burst,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
	}, nil
}

// Get returns the current state for a key.
func (s *memoryStore) Get(key string) (*State, error) {
	s.mu.RLock()
	bucket, ok := s.buckets[key]
	s.mu.RUnlock()

	if !ok {
		return &State{
			Tokens:     float64(s.config.Burst),
			LastUpdate: time.Now(),
		}, nil
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Calculate current tokens
	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	tokens := bucket.tokens + elapsed*s.config.Rate
	if tokens > float64(s.config.Burst) {
		tokens = float64(s.config.Burst)
	}

	return &State{
		Tokens:     tokens,
		LastUpdate: bucket.lastUpdate,
	}, nil
}

// Close closes the store.
func (s *memoryStore) Close() error {
	s.closed.Store(true)
	return nil
}

// cleanup removes expired buckets periodically.
func (s *memoryStore) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if s.closed.Load() {
			return
		}

		s.mu.Lock()
		now := time.Now()
		for key, bucket := range s.buckets {
			bucket.mu.Lock()
			// Remove buckets idle for more than 5 minutes
			if now.Sub(bucket.lastUpdate) > 5*time.Minute {
				delete(s.buckets, key)
			}
			bucket.mu.Unlock()
		}
		s.mu.Unlock()
	}
}

// Limiter is the main rate limiter interface.
type Limiter struct {
	config *Config
	store  Store
	mu     sync.RWMutex
}

// NewLimiter creates a new rate limiter.
func NewLimiter(config *Config) (*Limiter, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	var store Store
	switch config.Backend {
	case MemoryBackend:
		store = newMemoryStore(config)
		go store.(*memoryStore).cleanup(time.Minute)
	default:
		return nil, fmt.Errorf("unknown backend: %q", config.Backend)
	}

	return &Limiter{
		config: config,
		store:  store,
	}, nil
}

// Allow checks if a request is allowed for the given key.
func (l *Limiter) Allow(key string) (*Result, error) {
	return l.store.Allow(l.config.KeyPrefix+key, 1)
}

// AllowN checks if n requests are allowed for the given key.
func (l *Limiter) AllowN(key string, n int) (*Result, error) {
	return l.store.AllowN(l.config.KeyPrefix+key, n)
}

// Get returns the current state for a key.
func (l *Limiter) Get(key string) (*State, error) {
	return l.store.Get(l.config.KeyPrefix + key)
}

// Close closes the limiter.
func (l *Limiter) Close() error {
	return l.store.Close()
}

// KeyFunc generates rate limit keys from requests.
type KeyFunc func(r interface{}) string

// Common key functions.
var (
	// KeyByIP uses the client IP address.
	KeyByIP KeyFunc = func(r interface{}) string {
		// This would be implemented with actual request parsing
		return "ip:unknown"
	}

	// KeyByHeader uses a specific header value.
	KeyByHeader = func(header string) KeyFunc {
		return func(r interface{}) string {
			return "header:" + header
		}
	}

	// KeyByCookie uses a cookie value.
	KeyByCookie = func(name string) KeyFunc {
		return func(r interface{}) string {
			return "cookie:" + name
		}
	}
)

// Zone represents a rate limit zone with specific rules.
type Zone struct {
	Name      string  `json:"name" yaml:"name"`
	Key       string  `json:"key" yaml:"key"`             // Variable to key by: $ip, $header_X, $cookie_X
	Rate      float64 `json:"rate" yaml:"rate"`           // Requests per second
	Burst     int     `json:"burst" yaml:"burst"`         // Burst size
	PerIP     bool    `json:"per_ip" yaml:"per_ip"`       // Apply per IP
	PerUser   bool    `json:"per_user" yaml:"per_user"`   // Apply per authenticated user
}

// MultiZoneLimiter manages multiple rate limit zones.
type MultiZoneLimiter struct {
	config *Config
	zones  map[string]*Limiter
	mu     sync.RWMutex
}

// NewMultiZoneLimiter creates a new multi-zone limiter.
func NewMultiZoneLimiter() *MultiZoneLimiter {
	return &MultiZoneLimiter{
		zones: make(map[string]*Limiter),
	}
}

// AddZone adds a rate limit zone.
func (m *MultiZoneLimiter) AddZone(zone *Zone) error {
	config := DefaultConfig()
	config.Rate = zone.Rate
	config.Burst = zone.Burst

	limiter, err := NewLimiter(config)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.zones[zone.Name] = limiter
	return nil
}

// GetZone returns a zone limiter by name.
func (m *MultiZoneLimiter) GetZone(name string) (*Limiter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limiter, ok := m.zones[name]
	return limiter, ok
}

// RemoveZone removes a rate limit zone.
func (m *MultiZoneLimiter) RemoveZone(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limiter, ok := m.zones[name]; ok {
		limiter.Close()
		delete(m.zones, name)
	}
}

// Check checks all zones for a given key.
func (m *MultiZoneLimiter) Check(key string) (allowed bool, results map[string]*Result) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results = make(map[string]*Result)
	allowed = true

	for name, limiter := range m.zones {
		result, err := limiter.Allow(key)
		if err != nil {
			continue
		}
		results[name] = result
		if !result.Allowed {
			allowed = false
		}
	}

	return allowed, results
}

// Close closes all zone limiters.
func (m *MultiZoneLimiter) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, limiter := range m.zones {
		limiter.Close()
	}
}
