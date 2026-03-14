package health

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultPassiveHealthConfig(t *testing.T) {
	config := DefaultPassiveHealthConfig()

	if config.ErrorRateThreshold != 0.5 {
		t.Errorf("ErrorRateThreshold = %v, want 0.5", config.ErrorRateThreshold)
	}
	if config.WindowSize != 60*time.Second {
		t.Errorf("WindowSize = %v, want 60s", config.WindowSize)
	}
	if config.MinRequests != 10 {
		t.Errorf("MinRequests = %v, want 10", config.MinRequests)
	}
	if config.CooldownPeriod != 30*time.Second {
		t.Errorf("CooldownPeriod = %v, want 30s", config.CooldownPeriod)
	}
	if config.ConsecutiveErrors != 5 {
		t.Errorf("ConsecutiveErrors = %v, want 5", config.ConsecutiveErrors)
	}
	if config.EvalInterval != 1*time.Second {
		t.Errorf("EvalInterval = %v, want 1s", config.EvalInterval)
	}
}

func TestNewPassiveChecker_DefaultConfig(t *testing.T) {
	pc := NewPassiveChecker(nil)
	if pc == nil {
		t.Fatal("NewPassiveChecker(nil) returned nil")
	}
	if pc.config.ErrorRateThreshold != 0.5 {
		t.Errorf("default config not applied: ErrorRateThreshold = %v", pc.config.ErrorRateThreshold)
	}
}

func TestNewPassiveChecker_CustomConfig(t *testing.T) {
	config := &PassiveHealthConfig{
		ErrorRateThreshold: 0.3,
		WindowSize:         30 * time.Second,
		MinRequests:        5,
		CooldownPeriod:     10 * time.Second,
		ConsecutiveErrors:  3,
		EvalInterval:       500 * time.Millisecond,
	}

	pc := NewPassiveChecker(config)
	if pc.config.ErrorRateThreshold != 0.3 {
		t.Errorf("custom config not applied: ErrorRateThreshold = %v", pc.config.ErrorRateThreshold)
	}
	if pc.config.MinRequests != 5 {
		t.Errorf("custom config not applied: MinRequests = %v", pc.config.MinRequests)
	}
}

func TestSlidingWindow_Record(t *testing.T) {
	sw := NewSlidingWindow(100)
	now := time.Now()

	sw.Record(true, now)
	sw.Record(false, now)
	sw.Record(true, now)

	total, failures, errorRate := sw.Stats(time.Minute, now.Add(time.Second))
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if failures != 1 {
		t.Errorf("failures = %d, want 1", failures)
	}
	expectedRate := 1.0 / 3.0
	if errorRate < expectedRate-0.001 || errorRate > expectedRate+0.001 {
		t.Errorf("errorRate = %f, want ~%f", errorRate, expectedRate)
	}
}

func TestSlidingWindow_Expiry(t *testing.T) {
	sw := NewSlidingWindow(100)
	now := time.Now()

	// Record old data
	oldTime := now.Add(-2 * time.Minute)
	sw.Record(false, oldTime)
	sw.Record(false, oldTime)

	// Record recent data
	sw.Record(true, now)
	sw.Record(true, now)

	// Query with a 1-minute window -- old data should be excluded
	total, failures, _ := sw.Stats(time.Minute, now.Add(time.Second))
	if total != 2 {
		t.Errorf("total = %d, want 2 (old entries should be excluded)", total)
	}
	if failures != 0 {
		t.Errorf("failures = %d, want 0 (old failures should be excluded)", failures)
	}
}

func TestSlidingWindow_WrapAround(t *testing.T) {
	// Small capacity to force wrap-around
	sw := NewSlidingWindow(4)
	now := time.Now()

	// Write 6 entries to overflow the 4-capacity ring
	for i := 0; i < 6; i++ {
		sw.Record(i%2 == 0, now) // alternating success/failure
	}

	// Should only see the latest 4 entries
	total, _, _ := sw.Stats(time.Minute, now.Add(time.Second))
	if total != 4 {
		t.Errorf("total = %d, want 4 (capacity limit)", total)
	}
}

func TestSlidingWindow_Reset(t *testing.T) {
	sw := NewSlidingWindow(100)
	now := time.Now()

	sw.Record(true, now)
	sw.Record(false, now)

	sw.Reset()

	total, _, _ := sw.Stats(time.Minute, now.Add(time.Second))
	if total != 0 {
		t.Errorf("total after reset = %d, want 0", total)
	}
}

func TestSlidingWindow_EmptyStats(t *testing.T) {
	sw := NewSlidingWindow(100)
	now := time.Now()

	total, failures, errorRate := sw.Stats(time.Minute, now)
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if failures != 0 {
		t.Errorf("failures = %d, want 0", failures)
	}
	if errorRate != 0 {
		t.Errorf("errorRate = %f, want 0", errorRate)
	}
}

func TestPassiveChecker_RecordSuccess(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     time.Minute,
		ConsecutiveErrors:  5,
		EvalInterval:       time.Hour, // don't run eval loop
	})

	pc.RecordSuccess("backend1")
	pc.RecordSuccess("backend1")
	pc.RecordSuccess("backend1")

	stats := pc.GetStats("backend1")
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", stats.TotalRequests)
	}
	if stats.Failures != 0 {
		t.Errorf("Failures = %d, want 0", stats.Failures)
	}
	if stats.ErrorRate != 0 {
		t.Errorf("ErrorRate = %f, want 0", stats.ErrorRate)
	}
	if stats.Status != StatusHealthy {
		t.Errorf("Status = %v, want %v", stats.Status, StatusHealthy)
	}
}

func TestPassiveChecker_RecordFailure(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     time.Minute,
		ConsecutiveErrors:  100, // high so it doesn't trigger
		EvalInterval:       time.Hour,
	})

	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	stats := pc.GetStats("backend1")
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.TotalRequests != 2 {
		t.Errorf("TotalRequests = %d, want 2", stats.TotalRequests)
	}
	if stats.Failures != 2 {
		t.Errorf("Failures = %d, want 2", stats.Failures)
	}
	if stats.ErrorRate != 1.0 {
		t.Errorf("ErrorRate = %f, want 1.0", stats.ErrorRate)
	}
	if stats.ConsecutiveFailures != 2 {
		t.Errorf("ConsecutiveFailures = %d, want 2", stats.ConsecutiveFailures)
	}
}

func TestPassiveChecker_ErrorRateThresholdTriggersUnhealthy(t *testing.T) {
	var unhealthyCalled atomic.Int32

	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        4,
		CooldownPeriod:     time.Hour, // long so it doesn't auto-recover
		ConsecutiveErrors:  100,       // high so only rate triggers
		EvalInterval:       time.Hour,
	})
	pc.OnBackendUnhealthy = func(addr string) {
		unhealthyCalled.Add(1)
	}

	// 1 success + 3 failures = 75% error rate, above 50% threshold
	pc.RecordSuccess("backend1")
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	// Manually trigger evaluation
	pc.checkHealth("backend1")

	status := pc.GetStatus("backend1")
	if status != StatusUnhealthy {
		t.Errorf("Status = %v, want %v (error rate exceeded threshold)", status, StatusUnhealthy)
	}

	if unhealthyCalled.Load() != 1 {
		t.Errorf("OnBackendUnhealthy called %d times, want 1", unhealthyCalled.Load())
	}
}

func TestPassiveChecker_MinRequestsRequired(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        10,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  100, // high so it doesn't trigger
		EvalInterval:       time.Hour,
	})

	// Only 2 requests: 1 success + 1 failure = 50% error rate,
	// but below MinRequests so should stay healthy
	pc.RecordSuccess("backend1")
	pc.RecordFailure("backend1")

	pc.checkHealth("backend1")

	status := pc.GetStatus("backend1")
	if status != StatusHealthy {
		t.Errorf("Status = %v, want %v (below MinRequests threshold)", status, StatusHealthy)
	}
}

func TestPassiveChecker_ConsecutiveErrorsTrigger(t *testing.T) {
	var unhealthyCalled atomic.Int32

	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        100, // high so rate doesn't trigger
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  3,
		EvalInterval:       time.Hour,
	})
	pc.OnBackendUnhealthy = func(addr string) {
		unhealthyCalled.Add(1)
	}

	// 3 consecutive failures should trigger immediately
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	status := pc.GetStatus("backend1")
	if status != StatusUnhealthy {
		t.Errorf("Status = %v, want %v (consecutive errors threshold)", status, StatusUnhealthy)
	}

	if unhealthyCalled.Load() != 1 {
		t.Errorf("OnBackendUnhealthy called %d times, want 1", unhealthyCalled.Load())
	}
}

func TestPassiveChecker_ConsecutiveErrorsResetOnSuccess(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        100, // high so rate doesn't trigger
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  3,
		EvalInterval:       time.Hour,
	})

	// 2 failures, then a success, then 2 more failures
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")
	pc.RecordSuccess("backend1") // resets consecutive counter
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	// Should still be healthy because consecutive count never hit 3
	status := pc.GetStatus("backend1")
	if status != StatusHealthy {
		t.Errorf("Status = %v, want %v (consecutive reset on success)", status, StatusHealthy)
	}
}

func TestPassiveChecker_AutoRecoveryAfterCooldown(t *testing.T) {
	var recoveredCalled atomic.Int32

	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     50 * time.Millisecond, // very short cooldown
		ConsecutiveErrors:  2,
		EvalInterval:       time.Hour,
	})
	pc.OnBackendRecovered = func(addr string) {
		recoveredCalled.Add(1)
	}

	// Trigger unhealthy via consecutive errors
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	status := pc.GetStatus("backend1")
	if status != StatusUnhealthy {
		t.Fatalf("Status = %v, want %v", status, StatusUnhealthy)
	}

	// Wait for cooldown
	time.Sleep(100 * time.Millisecond)

	// Trigger evaluation
	pc.checkHealth("backend1")

	status = pc.GetStatus("backend1")
	if status != StatusHealthy {
		t.Errorf("Status = %v, want %v (should recover after cooldown)", status, StatusHealthy)
	}

	if recoveredCalled.Load() != 1 {
		t.Errorf("OnBackendRecovered called %d times, want 1", recoveredCalled.Load())
	}
}

func TestPassiveChecker_MultipleBackendsIndependence(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  2,
		EvalInterval:       time.Hour,
	})

	// backend1 gets failures
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	// backend2 gets successes
	pc.RecordSuccess("backend2")
	pc.RecordSuccess("backend2")

	// backend1 should be unhealthy (consecutive errors)
	status1 := pc.GetStatus("backend1")
	if status1 != StatusUnhealthy {
		t.Errorf("backend1 Status = %v, want %v", status1, StatusUnhealthy)
	}

	// backend2 should be healthy
	status2 := pc.GetStatus("backend2")
	if status2 != StatusHealthy {
		t.Errorf("backend2 Status = %v, want %v", status2, StatusHealthy)
	}

	// backend2 stats should not be affected by backend1
	stats2 := pc.GetStats("backend2")
	if stats2.ConsecutiveFailures != 0 {
		t.Errorf("backend2 ConsecutiveFailures = %d, want 0", stats2.ConsecutiveFailures)
	}
}

func TestPassiveChecker_RecordResult_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		err        error
		wantFail   bool
	}{
		{"200 OK", 200, nil, false},
		{"201 Created", 201, nil, false},
		{"301 Redirect", 301, nil, false},
		{"404 Not Found", 404, nil, false},
		{"499 Client Error", 499, nil, false},
		{"500 Internal Server Error", 500, nil, true},
		{"502 Bad Gateway", 502, nil, true},
		{"503 Service Unavailable", 503, nil, true},
		{"connection error", 0, fmt.Errorf("connection refused"), true},
		{"error with 200", 200, fmt.Errorf("timeout"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := NewPassiveChecker(&PassiveHealthConfig{
				ErrorRateThreshold: 0.5,
				WindowSize:         time.Minute,
				MinRequests:        100,
				CooldownPeriod:     time.Hour,
				ConsecutiveErrors:  100,
				EvalInterval:       time.Hour,
			})

			pc.RecordResult("backend1", tt.statusCode, tt.err)

			stats := pc.GetStats("backend1")
			if stats == nil {
				t.Fatal("GetStats returned nil")
			}
			if tt.wantFail {
				if stats.Failures != 1 {
					t.Errorf("Failures = %d, want 1 for %s", stats.Failures, tt.name)
				}
			} else {
				if stats.Failures != 0 {
					t.Errorf("Failures = %d, want 0 for %s", stats.Failures, tt.name)
				}
			}
		})
	}
}

func TestPassiveChecker_GetStats_UnknownBackend(t *testing.T) {
	pc := NewPassiveChecker(nil)

	stats := pc.GetStats("nonexistent")
	if stats != nil {
		t.Errorf("GetStats for unknown backend = %v, want nil", stats)
	}
}

func TestPassiveChecker_GetStatus_UnknownBackend(t *testing.T) {
	pc := NewPassiveChecker(nil)

	status := pc.GetStatus("nonexistent")
	if status != StatusUnknown {
		t.Errorf("GetStatus for unknown backend = %v, want %v", status, StatusUnknown)
	}
}

func TestPassiveChecker_Remove(t *testing.T) {
	pc := NewPassiveChecker(nil)

	pc.RecordSuccess("backend1")
	pc.Remove("backend1")

	stats := pc.GetStats("backend1")
	if stats != nil {
		t.Errorf("GetStats after Remove = %v, want nil", stats)
	}
}

func TestPassiveChecker_ListBackends(t *testing.T) {
	pc := NewPassiveChecker(nil)

	pc.RecordSuccess("backend1")
	pc.RecordSuccess("backend2")
	pc.RecordSuccess("backend3")

	backends := pc.ListBackends()
	if len(backends) != 3 {
		t.Errorf("ListBackends length = %d, want 3", len(backends))
	}

	found := make(map[string]bool)
	for _, b := range backends {
		found[b] = true
	}
	for _, expected := range []string{"backend1", "backend2", "backend3"} {
		if !found[expected] {
			t.Errorf("ListBackends missing %s", expected)
		}
	}
}

func TestPassiveChecker_ConcurrentAccess(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        100,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  100,
		EvalInterval:       time.Hour,
	})

	var wg sync.WaitGroup
	backends := []string{"b1", "b2", "b3", "b4"}

	// Concurrent writers
	for _, addr := range backends {
		addr := addr
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				pc.RecordSuccess(addr)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				pc.RecordFailure(addr)
			}
		}()
	}

	// Concurrent readers
	for _, addr := range backends {
		addr := addr
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				pc.GetStats(addr)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				pc.GetStatus(addr)
			}
		}()
	}

	// Concurrent evaluations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			pc.evaluateAll()
		}
	}()

	wg.Wait()

	// Should not panic or deadlock. Verify data integrity.
	for _, addr := range backends {
		stats := pc.GetStats(addr)
		if stats == nil {
			t.Errorf("GetStats(%s) returned nil after concurrent access", addr)
			continue
		}
		if stats.TotalRequests <= 0 {
			t.Errorf("GetStats(%s).TotalRequests = %d, want > 0", addr, stats.TotalRequests)
		}
	}
}

func TestPassiveChecker_CallbackInvocation(t *testing.T) {
	var (
		unhealthyAddrs []string
		recoveredAddrs []string
		mu             sync.Mutex
	)

	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     50 * time.Millisecond,
		ConsecutiveErrors:  2,
		EvalInterval:       time.Hour,
	})
	pc.OnBackendUnhealthy = func(addr string) {
		mu.Lock()
		unhealthyAddrs = append(unhealthyAddrs, addr)
		mu.Unlock()
	}
	pc.OnBackendRecovered = func(addr string) {
		mu.Lock()
		recoveredAddrs = append(recoveredAddrs, addr)
		mu.Unlock()
	}

	// Trigger unhealthy
	pc.RecordFailure("b1")
	pc.RecordFailure("b1")

	mu.Lock()
	if len(unhealthyAddrs) != 1 || unhealthyAddrs[0] != "b1" {
		t.Errorf("unhealthyAddrs = %v, want [b1]", unhealthyAddrs)
	}
	mu.Unlock()

	// Duplicate unhealthy calls should NOT trigger callback again
	pc.RecordFailure("b1")
	pc.RecordFailure("b1")

	mu.Lock()
	if len(unhealthyAddrs) != 1 {
		t.Errorf("unhealthyAddrs = %v, want length 1 (no duplicate callback)", unhealthyAddrs)
	}
	mu.Unlock()

	// Wait for cooldown and recover
	time.Sleep(100 * time.Millisecond)
	pc.checkHealth("b1")

	mu.Lock()
	if len(recoveredAddrs) != 1 || recoveredAddrs[0] != "b1" {
		t.Errorf("recoveredAddrs = %v, want [b1]", recoveredAddrs)
	}
	mu.Unlock()
}

func TestPassiveChecker_StartStop(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     50 * time.Millisecond,
		ConsecutiveErrors:  2,
		EvalInterval:       20 * time.Millisecond,
	})

	// Record failures to trigger unhealthy
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	// Start the eval loop
	pc.Start()

	// Wait long enough for cooldown + eval to fire
	time.Sleep(200 * time.Millisecond)

	// Should have auto-recovered
	status := pc.GetStatus("backend1")
	if status != StatusHealthy {
		t.Errorf("Status = %v, want %v (auto-recovery via eval loop)", status, StatusHealthy)
	}

	// Stop should not hang
	done := make(chan struct{})
	go func() {
		pc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete within timeout")
	}
}

func TestPassiveChecker_UnhealthyStatsPersist(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  2,
		EvalInterval:       time.Hour,
	})

	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	stats := pc.GetStats("backend1")
	if stats.Status != StatusUnhealthy {
		t.Errorf("Status = %v, want %v", stats.Status, StatusUnhealthy)
	}
	if stats.UnhealthySince.IsZero() {
		t.Error("UnhealthySince should be set")
	}
}

func TestPassiveChecker_ErrorRateCalculation(t *testing.T) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        1,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  100,
		EvalInterval:       time.Hour,
	})

	// 3 successes + 1 failure = 25% error rate
	pc.RecordSuccess("backend1")
	pc.RecordSuccess("backend1")
	pc.RecordSuccess("backend1")
	pc.RecordFailure("backend1")

	stats := pc.GetStats("backend1")
	expectedRate := 0.25
	if stats.ErrorRate < expectedRate-0.01 || stats.ErrorRate > expectedRate+0.01 {
		t.Errorf("ErrorRate = %f, want ~%f", stats.ErrorRate, expectedRate)
	}

	// Eval should keep it healthy (25% < 50%)
	pc.checkHealth("backend1")
	if pc.GetStatus("backend1") != StatusHealthy {
		t.Error("Should remain healthy at 25% error rate")
	}

	// Add more failures: 3 success + 4 failures = 57% error rate
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	pc.checkHealth("backend1")
	if pc.GetStatus("backend1") != StatusUnhealthy {
		t.Error("Should be unhealthy at ~57% error rate")
	}
}

func TestPassiveChecker_NoEvalWhenUnhealthy(t *testing.T) {
	// Verify that checkHealth doesn't re-evaluate error rate when already unhealthy
	// (it only checks cooldown for recovery)
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        2,
		CooldownPeriod:     time.Hour, // long cooldown
		ConsecutiveErrors:  2,
		EvalInterval:       time.Hour,
	})

	// Mark unhealthy
	pc.RecordFailure("backend1")
	pc.RecordFailure("backend1")

	if pc.GetStatus("backend1") != StatusUnhealthy {
		t.Fatal("Should be unhealthy")
	}

	// Record many successes while unhealthy
	for i := 0; i < 20; i++ {
		pc.RecordSuccess("backend1")
	}

	// checkHealth should not recover because cooldown hasn't elapsed
	pc.checkHealth("backend1")
	if pc.GetStatus("backend1") != StatusHealthy {
		// Actually, since status is unhealthy, checkHealth only checks cooldown.
		// It should still be unhealthy because cooldown is 1 hour.
	}
	status := pc.GetStatus("backend1")
	if status != StatusUnhealthy {
		t.Errorf("Status = %v, want %v (cooldown not elapsed)", status, StatusUnhealthy)
	}
}

func TestNewSlidingWindow_InvalidCapacity(t *testing.T) {
	sw := NewSlidingWindow(0)
	if sw == nil {
		t.Fatal("NewSlidingWindow(0) returned nil")
	}
	// Should default to 1024
	if sw.size != 1024 {
		t.Errorf("size = %d, want 1024 (default for invalid capacity)", sw.size)
	}

	sw2 := NewSlidingWindow(-5)
	if sw2 == nil {
		t.Fatal("NewSlidingWindow(-5) returned nil")
	}
	if sw2.size != 1024 {
		t.Errorf("size = %d, want 1024", sw2.size)
	}
}

func BenchmarkPassiveChecker_RecordSuccess(b *testing.B) {
	pc := NewPassiveChecker(DefaultPassiveHealthConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pc.RecordSuccess("backend1")
	}
}

func BenchmarkPassiveChecker_RecordFailure(b *testing.B) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        100,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  1000000, // very high to avoid triggering
		EvalInterval:       time.Hour,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pc.RecordFailure("backend1")
	}
}

func BenchmarkPassiveChecker_RecordResult(b *testing.B) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        100,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  1000000,
		EvalInterval:       time.Hour,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%3 == 0 {
			pc.RecordResult("backend1", 500, nil)
		} else {
			pc.RecordResult("backend1", 200, nil)
		}
	}
}

func BenchmarkPassiveChecker_GetStats(b *testing.B) {
	pc := NewPassiveChecker(DefaultPassiveHealthConfig())

	// Pre-populate
	for i := 0; i < 100; i++ {
		pc.RecordSuccess("backend1")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pc.GetStats("backend1")
	}
}

func BenchmarkPassiveChecker_ConcurrentRecordAndRead(b *testing.B) {
	pc := NewPassiveChecker(&PassiveHealthConfig{
		ErrorRateThreshold: 0.5,
		WindowSize:         time.Minute,
		MinRequests:        100,
		CooldownPeriod:     time.Hour,
		ConsecutiveErrors:  1000000,
		EvalInterval:       time.Hour,
	})

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				pc.RecordSuccess("backend1")
			} else {
				pc.GetStats("backend1")
			}
			i++
		}
	})
}
