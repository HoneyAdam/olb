package botdetect

import (
	"math"
	"sync"
	"time"
)

// BehaviorResult holds the result of behavioral analysis.
type BehaviorResult struct {
	Score   int
	Rule    string
	Details string
}

// BehaviorTracker tracks per-IP request behavior over a sliding window.
type BehaviorTracker struct {
	mu      sync.RWMutex
	windows map[string]*ipWindow
	window  time.Duration
	stopCh  chan struct{}

	// Thresholds
	RPSThreshold       float64
	ErrorRateThreshold float64
}

type ipWindow struct {
	requests    []requestRecord
	errorCount  int
	uniquePaths map[string]bool
	lastSeen    time.Time
}

type requestRecord struct {
	timestamp time.Time
	path      string
	status    int
}

// BehaviorConfig configures behavioral analysis.
type BehaviorConfig struct {
	Window             time.Duration
	RPSThreshold       float64
	ErrorRateThreshold float64
}

// NewBehaviorTracker creates a new behavioral analysis tracker.
func NewBehaviorTracker(cfg BehaviorConfig) *BehaviorTracker {
	if cfg.Window == 0 {
		cfg.Window = 5 * time.Minute
	}
	if cfg.RPSThreshold == 0 {
		cfg.RPSThreshold = 10
	}
	if cfg.ErrorRateThreshold == 0 {
		cfg.ErrorRateThreshold = 30
	}

	bt := &BehaviorTracker{
		windows:            make(map[string]*ipWindow),
		window:             cfg.Window,
		stopCh:             make(chan struct{}),
		RPSThreshold:       cfg.RPSThreshold,
		ErrorRateThreshold: cfg.ErrorRateThreshold,
	}

	go bt.cleanupLoop()
	return bt
}

// Record records a request for behavioral analysis.
func (bt *BehaviorTracker) Record(ip, path string, status int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	w, ok := bt.windows[ip]
	if !ok {
		w = &ipWindow{
			uniquePaths: make(map[string]bool),
		}
		bt.windows[ip] = w
	}

	now := time.Now()
	w.requests = append(w.requests, requestRecord{
		timestamp: now,
		path:      path,
		status:    status,
	})
	w.uniquePaths[path] = true
	w.lastSeen = now

	if status >= 400 {
		w.errorCount++
	}
}

// Analyze analyzes the behavioral pattern of an IP.
func (bt *BehaviorTracker) Analyze(ip string) BehaviorResult {
	bt.mu.RLock()
	w, ok := bt.windows[ip]
	if !ok {
		bt.mu.RUnlock()
		return BehaviorResult{}
	}

	// Copy data under read lock
	requests := make([]requestRecord, len(w.requests))
	copy(requests, w.requests)
	uniquePaths := len(w.uniquePaths)
	errorCount := w.errorCount
	bt.mu.RUnlock()

	// Trim to window
	cutoff := time.Now().Add(-bt.window)
	var inWindow []requestRecord
	for _, r := range requests {
		if r.timestamp.After(cutoff) {
			inWindow = append(inWindow, r)
		}
	}

	if len(inWindow) < 3 {
		return BehaviorResult{}
	}

	windowSecs := bt.window.Seconds()
	rps := float64(len(inWindow)) / windowSecs

	// High RPS + many unique paths = scanning
	if rps > bt.RPSThreshold/60 && uniquePaths > 50 {
		return BehaviorResult{
			Score:   70,
			Rule:    "scanning_behavior",
			Details: "high request rate with many unique paths",
		}
	}

	// High error rate
	if len(inWindow) > 10 {
		errorRate := float64(errorCount) / float64(len(inWindow)) * 100
		if errorRate > bt.ErrorRateThreshold {
			return BehaviorResult{
				Score:   50,
				Rule:    "high_error_rate",
				Details: "error rate exceeds threshold",
			}
		}
	}

	// Machine-like timing precision
	if len(inWindow) > 5 {
		stddev := timingStdDev(inWindow)
		if stddev < 10*time.Millisecond && stddev > 0 {
			return BehaviorResult{
				Score:   60,
				Rule:    "machine_timing",
				Details: "request timing has machine-like precision",
			}
		}
	}

	return BehaviorResult{}
}

// Stop stops the cleanup goroutine.
func (bt *BehaviorTracker) Stop() {
	close(bt.stopCh)
}

func (bt *BehaviorTracker) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bt.cleanup()
		case <-bt.stopCh:
			return
		}
	}
}

func (bt *BehaviorTracker) cleanup() {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	cutoff := time.Now().Add(-bt.window * 2)
	for ip, w := range bt.windows {
		if w.lastSeen.Before(cutoff) {
			delete(bt.windows, ip)
		}
	}
}

// timingStdDev calculates the standard deviation of inter-request timing.
func timingStdDev(requests []requestRecord) time.Duration {
	if len(requests) < 3 {
		return 0
	}

	var intervals []float64
	for i := 1; i < len(requests); i++ {
		interval := requests[i].timestamp.Sub(requests[i-1].timestamp).Seconds()
		if interval > 0 {
			intervals = append(intervals, interval)
		}
	}

	if len(intervals) < 2 {
		return 0
	}

	// Calculate mean
	var sum float64
	for _, v := range intervals {
		sum += v
	}
	mean := sum / float64(len(intervals))

	// Calculate variance
	var variance float64
	for _, v := range intervals {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(intervals))

	stddev := math.Sqrt(variance)
	return time.Duration(stddev * float64(time.Second))
}
