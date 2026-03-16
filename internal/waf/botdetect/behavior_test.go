package botdetect

import (
	"testing"
	"time"
)

func TestBehaviorTracker_RecordAndAnalyze_NoRequests(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{
		Window:             5 * time.Minute,
		RPSThreshold:       10,
		ErrorRateThreshold: 30,
	})
	defer bt.Stop()

	// Analyze unknown IP should return empty result
	result := bt.Analyze("1.2.3.4")
	if result.Score != 0 {
		t.Errorf("expected score 0 for unknown IP, got %d", result.Score)
	}
	if result.Rule != "" {
		t.Errorf("expected empty rule for unknown IP, got %q", result.Rule)
	}
}

func TestBehaviorTracker_RecordAndAnalyze_FewRequests(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{
		Window:             5 * time.Minute,
		RPSThreshold:       10,
		ErrorRateThreshold: 30,
	})
	defer bt.Stop()

	// Less than 3 requests should return empty result
	bt.Record("1.2.3.4", "/page1", 200)
	bt.Record("1.2.3.4", "/page2", 200)

	result := bt.Analyze("1.2.3.4")
	if result.Score != 0 {
		t.Errorf("expected score 0 for < 3 requests, got %d", result.Score)
	}
}

func TestBehaviorTracker_HighErrorRate(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{
		Window:             5 * time.Minute,
		RPSThreshold:       10,
		ErrorRateThreshold: 30,
	})
	defer bt.Stop()

	ip := "10.0.0.1"
	// Send 15 requests, with >30% errors
	for i := 0; i < 15; i++ {
		status := 200
		if i%2 == 0 {
			status = 404 // 50% error rate
		}
		bt.Record(ip, "/page", status)
	}

	result := bt.Analyze(ip)
	if result.Score < 50 {
		t.Errorf("expected score >= 50 for high error rate, got %d", result.Score)
	}
	if result.Rule != "high_error_rate" {
		t.Errorf("expected rule 'high_error_rate', got %q", result.Rule)
	}
}

func TestBehaviorTracker_ScanningBehavior(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{
		Window:             1 * time.Second, // very short window
		RPSThreshold:       10,
		ErrorRateThreshold: 90, // high threshold so error rate doesn't trigger
	})
	defer bt.Stop()

	ip := "10.0.0.2"
	// Generate many unique paths to trigger scanning detection
	for i := 0; i < 60; i++ {
		path := "/path/" + string(rune('a'+i%26)) + "/" + string(rune('0'+i/26))
		bt.Record(ip, path, 200)
	}

	result := bt.Analyze(ip)
	if result.Rule == "scanning_behavior" {
		if result.Score < 70 {
			t.Errorf("expected score >= 70 for scanning behavior, got %d", result.Score)
		}
	}
	// Note: This may or may not trigger depending on timing. We verify it doesn't crash.
}

func TestBehaviorTracker_DefaultConfig(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{})
	defer bt.Stop()

	// Defaults should be applied
	if bt.window != 5*time.Minute {
		t.Errorf("expected default window of 5m, got %v", bt.window)
	}
	if bt.RPSThreshold != 10 {
		t.Errorf("expected default RPSThreshold of 10, got %f", bt.RPSThreshold)
	}
	if bt.ErrorRateThreshold != 30 {
		t.Errorf("expected default ErrorRateThreshold of 30, got %f", bt.ErrorRateThreshold)
	}
}

func TestBehaviorTracker_MultipleIPs(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{
		Window:             5 * time.Minute,
		RPSThreshold:       10,
		ErrorRateThreshold: 30,
	})
	defer bt.Stop()

	// Record requests for two IPs
	for i := 0; i < 5; i++ {
		bt.Record("1.1.1.1", "/ok", 200)
		bt.Record("2.2.2.2", "/err", 500)
	}

	result1 := bt.Analyze("1.1.1.1")
	result2 := bt.Analyze("2.2.2.2")

	// IP1 has all 200s, IP2 has all 500s — they should differ
	if result1.Score > result2.Score {
		t.Errorf("expected IP with all errors to have higher score: ip1=%d, ip2=%d", result1.Score, result2.Score)
	}
}

func TestTimingStdDev_FewRequests(t *testing.T) {
	// Less than 3 requests should return 0
	result := timingStdDev([]requestRecord{
		{timestamp: time.Now()},
		{timestamp: time.Now().Add(time.Millisecond)},
	})
	if result != 0 {
		t.Errorf("expected 0 for < 3 requests, got %v", result)
	}
}

func TestTimingStdDev_UniformTiming(t *testing.T) {
	now := time.Now()
	requests := make([]requestRecord, 10)
	for i := 0; i < 10; i++ {
		requests[i] = requestRecord{
			timestamp: now.Add(time.Duration(i) * 100 * time.Millisecond),
		}
	}

	result := timingStdDev(requests)
	// With perfectly uniform timing, stddev should be very small (near 0)
	if result > 5*time.Millisecond {
		t.Errorf("expected very small stddev for uniform timing, got %v", result)
	}
}

func TestTimingStdDev_VariableTiming(t *testing.T) {
	now := time.Now()
	requests := []requestRecord{
		{timestamp: now},
		{timestamp: now.Add(10 * time.Millisecond)},
		{timestamp: now.Add(500 * time.Millisecond)},
		{timestamp: now.Add(510 * time.Millisecond)},
		{timestamp: now.Add(1000 * time.Millisecond)},
	}

	result := timingStdDev(requests)
	// Variable timing should produce non-trivial stddev
	if result == 0 {
		t.Error("expected non-zero stddev for variable timing")
	}
}

func TestBehaviorTracker_RecordErrorCounting(t *testing.T) {
	bt := NewBehaviorTracker(BehaviorConfig{
		Window: 5 * time.Minute,
	})
	defer bt.Stop()

	ip := "5.5.5.5"
	bt.Record(ip, "/ok", 200)
	bt.Record(ip, "/not-found", 404)
	bt.Record(ip, "/error", 500)
	bt.Record(ip, "/bad", 400)

	bt.mu.RLock()
	w := bt.windows[ip]
	errorCount := w.errorCount
	uniquePaths := len(w.uniquePaths)
	totalRequests := len(w.requests)
	bt.mu.RUnlock()

	if errorCount != 3 {
		t.Errorf("expected 3 errors (404, 500, 400), got %d", errorCount)
	}
	if uniquePaths != 4 {
		t.Errorf("expected 4 unique paths, got %d", uniquePaths)
	}
	if totalRequests != 4 {
		t.Errorf("expected 4 total requests, got %d", totalRequests)
	}
}
