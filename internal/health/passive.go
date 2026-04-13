// Package health provides health checking for OpenLoadBalancer backends.
// This file implements passive health checking based on real traffic patterns.
package health

import (
	"sync"
	"time"
)

// PassiveHealthConfig holds the configuration for passive health checking.
type PassiveHealthConfig struct {
	// ErrorRateThreshold is the error rate (0.0 to 1.0) above which a backend
	// is considered unhealthy. Default: 0.5 (50%).
	ErrorRateThreshold float64

	// WindowSize is the duration of the sliding window over which error rates
	// are calculated. Default: 60s.
	WindowSize time.Duration

	// MinRequests is the minimum number of requests in the window before
	// the error rate is evaluated. This prevents marking a backend unhealthy
	// based on very few samples. Default: 10.
	MinRequests int

	// CooldownPeriod is the duration to wait after marking a backend unhealthy
	// before automatically resetting its window and allowing traffic again.
	// Default: 30s.
	CooldownPeriod time.Duration

	// ConsecutiveErrors is the number of consecutive failures that immediately
	// triggers an unhealthy state, regardless of error rate. Default: 5.
	ConsecutiveErrors int

	// EvalInterval is the interval at which the background goroutine evaluates
	// backend health. Default: 1s.
	EvalInterval time.Duration
}

// DefaultPassiveHealthConfig returns a PassiveHealthConfig with sensible defaults.
func DefaultPassiveHealthConfig() *PassiveHealthConfig {
	return &PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         60 * time.Second,
		MinRequests:        10,
		CooldownPeriod:     30 * time.Second,
		ConsecutiveErrors:  5,
		EvalInterval:       1 * time.Second,
	}
}

// requestResult represents the outcome of a single request.
type requestResult struct {
	success   bool
	timestamp time.Time
}

// SlidingWindow tracks request results over a configurable time window
// using a ring buffer approach with timestamp-based expiry.
type SlidingWindow struct {
	mu      sync.RWMutex
	results []requestResult
	size    int
	head    int // next write position
	count   int // current number of entries
}

// NewSlidingWindow creates a new SlidingWindow with the given capacity.
// The capacity determines the maximum number of results that can be stored.
func NewSlidingWindow(capacity int) *SlidingWindow {
	if capacity <= 0 {
		capacity = 1024
	}
	return &SlidingWindow{
		results: make([]requestResult, capacity),
		size:    capacity,
	}
}

// Record adds a request result to the sliding window.
func (sw *SlidingWindow) Record(success bool, ts time.Time) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.results[sw.head] = requestResult{
		success:   success,
		timestamp: ts,
	}
	sw.head = (sw.head + 1) % sw.size
	if sw.count < sw.size {
		sw.count++
	}
}

// Stats returns the total requests, failures, and error rate within the
// given window duration from the reference time.
func (sw *SlidingWindow) Stats(windowSize time.Duration, now time.Time) (total int, failures int, errorRate float64) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	cutoff := now.Add(-windowSize)

	for i := range sw.count {
		// Walk backward from the most recent entry
		idx := (sw.head - 1 - i + sw.size) % sw.size
		r := sw.results[idx]

		if r.timestamp.Before(cutoff) {
			// Once we hit an entry before the cutoff, all remaining are older
			break
		}

		total++
		if !r.success {
			failures++
		}
	}

	if total > 0 {
		errorRate = float64(failures) / float64(total)
	}
	return
}

// Reset clears all recorded results.
func (sw *SlidingWindow) Reset() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.head = 0
	sw.count = 0
}

// backendState holds the passive health state for a single backend.
type backendState struct {
	mu               sync.RWMutex
	window           *SlidingWindow
	status           Status
	consecutiveFails int
	unhealthySince   time.Time // when the backend was marked unhealthy
}

// PassiveChecker monitors backend health based on real traffic error rates.
// Unlike the active Checker which probes backends, the PassiveChecker
// observes actual request outcomes to determine health.
type PassiveChecker struct {
	config *PassiveHealthConfig

	mu       sync.RWMutex
	backends map[string]*backendState

	// Callbacks invoked on state transitions.
	OnBackendUnhealthy func(addr string)
	OnBackendRecovered func(addr string)

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewPassiveChecker creates a new PassiveChecker with the given configuration.
// If config is nil, DefaultPassiveHealthConfig is used.
func NewPassiveChecker(config *PassiveHealthConfig) *PassiveChecker {
	if config == nil {
		config = DefaultPassiveHealthConfig()
	}
	return &PassiveChecker{
		config:   config,
		backends: make(map[string]*backendState),
		stopCh:   make(chan struct{}),
	}
}

// getOrCreateBackend returns the backendState for the given address,
// creating one if it does not exist.
func (pc *PassiveChecker) getOrCreateBackend(addr string) *backendState {
	pc.mu.RLock()
	bs, ok := pc.backends[addr]
	pc.mu.RUnlock()
	if ok {
		return bs
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Double-check after acquiring write lock.
	if bs, ok = pc.backends[addr]; ok {
		return bs
	}

	bs = &backendState{
		window: NewSlidingWindow(1024),
		status: StatusHealthy,
	}
	pc.backends[addr] = bs
	return bs
}

// RecordSuccess records a successful request for the given backend address.
func (pc *PassiveChecker) RecordSuccess(backendAddr string) {
	bs := pc.getOrCreateBackend(backendAddr)

	bs.mu.Lock()
	bs.window.Record(true, time.Now())
	bs.consecutiveFails = 0
	bs.mu.Unlock()
}

// RecordFailure records a failed request for the given backend address
// and immediately evaluates whether the backend should be marked unhealthy.
func (pc *PassiveChecker) RecordFailure(backendAddr string) {
	bs := pc.getOrCreateBackend(backendAddr)

	now := time.Now()
	bs.mu.Lock()
	bs.window.Record(false, now)
	bs.consecutiveFails++
	consec := bs.consecutiveFails
	bs.mu.Unlock()

	// Check for consecutive error threshold immediately.
	if consec >= pc.config.ConsecutiveErrors {
		pc.markUnhealthy(backendAddr, bs)
	}
}

// RecordResult auto-classifies a request result and records it.
// Status codes >= 500 and connection errors (non-nil err) are treated as failures.
// Status codes < 500 with no error are treated as successes.
func (pc *PassiveChecker) RecordResult(backendAddr string, statusCode int, err error) {
	if err != nil || statusCode >= 500 {
		pc.RecordFailure(backendAddr)
	} else {
		pc.RecordSuccess(backendAddr)
	}
}

// markUnhealthy transitions a backend to unhealthy state.
func (pc *PassiveChecker) markUnhealthy(addr string, bs *backendState) {
	bs.mu.Lock()
	if bs.status == StatusUnhealthy {
		bs.mu.Unlock()
		return
	}
	bs.status = StatusUnhealthy
	bs.unhealthySince = time.Now()
	bs.mu.Unlock()

	// Read callback under the checker's mutex for safe concurrent access.
	pc.mu.RLock()
	cb := pc.OnBackendUnhealthy
	pc.mu.RUnlock()
	if cb != nil {
		cb(addr)
	}
}

// markRecovered transitions a backend back to healthy state.
func (pc *PassiveChecker) markRecovered(addr string, bs *backendState) {
	bs.mu.Lock()
	if bs.status != StatusUnhealthy {
		bs.mu.Unlock()
		return
	}
	bs.status = StatusHealthy
	bs.consecutiveFails = 0
	bs.window.Reset()
	bs.unhealthySince = time.Time{}
	bs.mu.Unlock()

	// Read callback under the checker's mutex for safe concurrent access.
	pc.mu.RLock()
	cb := pc.OnBackendRecovered
	pc.mu.RUnlock()
	if cb != nil {
		cb(addr)
	}
}

// checkHealth evaluates the current health of a backend based on its
// sliding window error rate and the configured thresholds.
func (pc *PassiveChecker) checkHealth(backendAddr string) {
	pc.mu.RLock()
	bs, ok := pc.backends[backendAddr]
	pc.mu.RUnlock()
	if !ok {
		return
	}

	bs.mu.RLock()
	status := bs.status
	unhealthySince := bs.unhealthySince
	bs.mu.RUnlock()

	now := time.Now()

	// Check auto-recovery for unhealthy backends.
	if status == StatusUnhealthy {
		if !unhealthySince.IsZero() && now.Sub(unhealthySince) >= pc.config.CooldownPeriod {
			pc.markRecovered(backendAddr, bs)
		}
		return
	}

	// Evaluate error rate for healthy backends.
	total, _, errorRate := bs.window.Stats(pc.config.WindowSize, now)

	if total >= pc.config.MinRequests && errorRate > pc.config.ErrorRateThreshold {
		pc.markUnhealthy(backendAddr, bs)
	}
}

// Start begins the background evaluation goroutine that periodically
// checks all tracked backends.
func (pc *PassiveChecker) Start() {
	pc.wg.Add(1)
	go pc.evalLoop()
}

// Stop stops the background evaluation goroutine and waits for it to exit.
func (pc *PassiveChecker) Stop() {
	close(pc.stopCh)
	pc.wg.Wait()
}

// evalLoop runs periodically to evaluate all backends.
func (pc *PassiveChecker) evalLoop() {
	defer pc.wg.Done()

	interval := pc.config.EvalInterval
	if interval <= 0 {
		interval = 1 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pc.evaluateAll()
		case <-pc.stopCh:
			return
		}
	}
}

// evaluateAll checks health for all tracked backends.
func (pc *PassiveChecker) evaluateAll() {
	pc.mu.RLock()
	addrs := make([]string, 0, len(pc.backends))
	for addr := range pc.backends {
		addrs = append(addrs, addr)
	}
	pc.mu.RUnlock()

	for _, addr := range addrs {
		pc.checkHealth(addr)
	}
}

// BackendPassiveStats contains the current passive health statistics
// for a single backend.
type BackendPassiveStats struct {
	// Status is the current health status.
	Status Status

	// TotalRequests is the number of requests in the current window.
	TotalRequests int

	// Failures is the number of failed requests in the current window.
	Failures int

	// ErrorRate is the current error rate (0.0 to 1.0).
	ErrorRate float64

	// ConsecutiveFailures is the number of consecutive failures.
	ConsecutiveFailures int

	// UnhealthySince is the time the backend was marked unhealthy.
	// Zero value if the backend is healthy.
	UnhealthySince time.Time
}

// GetStats returns the current passive health statistics for the given
// backend address. Returns nil if the backend is not tracked.
func (pc *PassiveChecker) GetStats(backendAddr string) *BackendPassiveStats {
	pc.mu.RLock()
	bs, ok := pc.backends[backendAddr]
	pc.mu.RUnlock()
	if !ok {
		return nil
	}

	now := time.Now()
	total, failures, errorRate := bs.window.Stats(pc.config.WindowSize, now)

	bs.mu.RLock()
	defer bs.mu.RUnlock()

	return &BackendPassiveStats{
		Status:              bs.status,
		TotalRequests:       total,
		Failures:            failures,
		ErrorRate:           errorRate,
		ConsecutiveFailures: bs.consecutiveFails,
		UnhealthySince:      bs.unhealthySince,
	}
}

// GetStatus returns the current health status for the given backend address.
// Returns StatusUnknown if the backend is not tracked.
func (pc *PassiveChecker) GetStatus(backendAddr string) Status {
	pc.mu.RLock()
	bs, ok := pc.backends[backendAddr]
	pc.mu.RUnlock()
	if !ok {
		return StatusUnknown
	}

	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.status
}

// Remove removes a backend from passive health tracking.
func (pc *PassiveChecker) Remove(backendAddr string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.backends, backendAddr)
}

// ListBackends returns the addresses of all tracked backends.
func (pc *PassiveChecker) ListBackends() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	addrs := make([]string, 0, len(pc.backends))
	for addr := range pc.backends {
		addrs = append(addrs, addr)
	}
	return addrs
}
