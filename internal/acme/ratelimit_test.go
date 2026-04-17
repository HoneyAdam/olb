package acme

import (
	"strings"
	"testing"
	"time"
)

func TestNewRateTracker_DefaultConfig(t *testing.T) {
	rt := NewRateTracker(nil)
	if rt == nil {
		t.Fatal("NewRateTracker returned nil")
	}
	if rt.config.CertsPerDomainLimit != CertsPerDomainPerWeek {
		t.Errorf("CertsPerDomainLimit = %d, want %d", rt.config.CertsPerDomainLimit, CertsPerDomainPerWeek)
	}
	if rt.config.OrdersPerAccountLimit != OrdersPerAccountPer3Hours {
		t.Errorf("OrdersPerAccountLimit = %d, want %d", rt.config.OrdersPerAccountLimit, OrdersPerAccountPer3Hours)
	}
	if rt.config.FailedValidationsLimit != FailedValidationsPerHour {
		t.Errorf("FailedValidationsLimit = %d, want %d", rt.config.FailedValidationsLimit, FailedValidationsPerHour)
	}
}

func TestNewRateTracker_CustomConfig(t *testing.T) {
	cfg := &RateLimitConfig{
		CertsPerDomainWindow:    24 * time.Hour,
		OrdersPerAccountWindow:  1 * time.Hour,
		FailedValidationsWindow: 30 * time.Minute,
		CertsPerDomainLimit:     10,
		OrdersPerAccountLimit:   50,
		FailedValidationsLimit:  3,
	}
	rt := NewRateTracker(cfg)
	if rt.config.CertsPerDomainLimit != 10 {
		t.Errorf("CertsPerDomainLimit = %d, want 10", rt.config.CertsPerDomainLimit)
	}
}

func TestRateTracker_CanOrder(t *testing.T) {
	cfg := testConfig()
	rt := NewRateTracker(cfg)

	// Should be able to order initially
	ok, warning := rt.CanOrder([]string{"example.com"})
	if !ok {
		t.Error("CanOrder should return true initially")
	}
	if warning != "" {
		t.Errorf("Expected no warning, got: %s", warning)
	}
}

func TestRateTracker_OrderLimit(t *testing.T) {
	cfg := testConfig()
	cfg.OrdersPerAccountLimit = 3
	cfg.OrdersPerAccountWindow = 1 * time.Hour
	rt := NewRateTracker(cfg)

	// Place 3 orders
	for i := 0; i < 3; i++ {
		rt.RecordOrder([]string{"example.com"})
	}

	// 4th should be blocked
	ok, reason := rt.CanOrder([]string{"example.com"})
	if ok {
		t.Error("CanOrder should return false after reaching limit")
	}
	if !strings.Contains(reason, "order limit reached") {
		t.Errorf("Expected limit reached message, got: %s", reason)
	}
}

func TestRateTracker_DomainLimit(t *testing.T) {
	cfg := testConfig()
	cfg.CertsPerDomainLimit = 2
	cfg.CertsPerDomainWindow = 1 * time.Hour
	rt := NewRateTracker(cfg)

	// 2 orders for example.com
	rt.RecordOrder([]string{"example.com"})
	rt.RecordOrder([]string{"example.com"})

	// 3rd should be blocked for that domain
	ok, reason := rt.CanOrder([]string{"example.com"})
	if ok {
		t.Error("CanOrder should return false after domain limit reached")
	}
	if !strings.Contains(reason, "cert limit reached for example.com") {
		t.Errorf("Expected domain limit message, got: %s", reason)
	}

	// Different domain should still work
	ok, _ = rt.CanOrder([]string{"other.com"})
	if !ok {
		t.Error("CanOrder should return true for different domain")
	}
}

func TestRateTracker_FailedValidationLimit(t *testing.T) {
	cfg := testConfig()
	cfg.FailedValidationsLimit = 2
	cfg.FailedValidationsWindow = 1 * time.Hour
	rt := NewRateTracker(cfg)

	// 2 failed validations
	w1 := rt.RecordFailedValidation("example.com")
	if w1 != "" && !strings.Contains(w1, "warning") {
		t.Errorf("Expected warning at 50%%: %s", w1)
	}

	w2 := rt.RecordFailedValidation("example.com")
	if !strings.Contains(w2, "limit reached") {
		t.Errorf("Expected limit reached at 2/2: %s", w2)
	}

	// Should block new orders now
	ok, reason := rt.CanOrder([]string{"example.com"})
	if ok {
		t.Error("CanOrder should return false after validation failures")
	}
	if !strings.Contains(reason, "validation failure limit") {
		t.Errorf("Expected validation failure message, got: %s", reason)
	}
}

func TestRateTracker_WarningThreshold(t *testing.T) {
	cfg := testConfig()
	cfg.OrdersPerAccountLimit = 10
	cfg.OrdersPerAccountWindow = 1 * time.Hour
	rt := NewRateTracker(cfg)

	// Place 7 orders (70% of 10, below 80% threshold)
	for i := 0; i < 7; i++ {
		w := rt.RecordOrder([]string{"example.com"})
		if w != "" {
			t.Errorf("Expected no warning at %d/10, got: %s", i+1, w)
		}
	}

	// 8th order (80% threshold)
	w := rt.RecordOrder([]string{"example.com"})
	if !strings.Contains(w, "warning") {
		t.Errorf("Expected warning at 80%%, got: %s", w)
	}
}

func TestRateTracker_Stats(t *testing.T) {
	cfg := testConfig()
	rt := NewRateTracker(cfg)

	rt.RecordOrder([]string{"example.com"})
	rt.RecordOrder([]string{"example.com"})
	rt.RecordOrder([]string{"other.com"})
	rt.RecordFailedValidation("example.com")

	stats := rt.Stats()
	if stats.AccountOrders != 3 {
		t.Errorf("AccountOrders = %d, want 3", stats.AccountOrders)
	}
	if stats.DomainOrders != 2 {
		t.Errorf("DomainOrders = %d, want 2 (max domain)", stats.DomainOrders)
	}
	if stats.DomainOrdersName != "example.com" {
		t.Errorf("DomainOrdersName = %q, want example.com", stats.DomainOrdersName)
	}
	if stats.FailedValidations != 1 {
		t.Errorf("FailedValidations = %d, want 1", stats.FailedValidations)
	}
}

func TestRateTracker_WindowExpiry(t *testing.T) {
	cfg := testConfig()
	cfg.OrdersPerAccountLimit = 2
	cfg.OrdersPerAccountWindow = 100 * time.Millisecond
	rt := NewRateTracker(cfg)

	rt.RecordOrder([]string{"example.com"})
	rt.RecordOrder([]string{"example.com"})

	// Should be blocked
	ok, _ := rt.CanOrder([]string{"example.com"})
	if ok {
		t.Error("Should be blocked at limit")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	ok, _ = rt.CanOrder([]string{"example.com"})
	if !ok {
		t.Error("Should be allowed after window expires")
	}
}

func TestRateTracker_MultipleDomains(t *testing.T) {
	cfg := testConfig()
	cfg.CertsPerDomainLimit = 2
	cfg.CertsPerDomainWindow = 1 * time.Hour
	rt := NewRateTracker(cfg)

	// Order with multiple domains
	rt.RecordOrder([]string{"a.example.com", "b.example.com"})

	// Each domain should have 1 order
	stats := rt.Stats()
	if stats.AccountOrders != 1 {
		t.Errorf("AccountOrders = %d, want 1", stats.AccountOrders)
	}

	// Fill up a.example.com
	rt.RecordOrder([]string{"a.example.com"})

	// a.example.com should be at limit
	ok, reason := rt.CanOrder([]string{"a.example.com"})
	if ok {
		t.Error("a.example.com should be at limit")
	}
	if !strings.Contains(reason, "a.example.com") {
		t.Errorf("Expected a.example.com in reason, got: %s", reason)
	}

	// b.example.com should still be available
	ok, _ = rt.CanOrder([]string{"b.example.com"})
	if !ok {
		t.Error("b.example.com should still be available")
	}
}

func TestRateTracker_RecordOrderWarning(t *testing.T) {
	cfg := testConfig()
	cfg.OrdersPerAccountLimit = 5
	cfg.OrdersPerAccountWindow = 1 * time.Hour
	rt := NewRateTracker(cfg)

	// Orders 1-3: no warning
	for i := 0; i < 3; i++ {
		w := rt.RecordOrder([]string{"example.com"})
		if w != "" {
			t.Errorf("No warning expected at %d/5, got: %s", i+1, w)
		}
	}

	// Order 4 (80% of 5): warning
	w := rt.RecordOrder([]string{"example.com"})
	if !strings.Contains(w, "order limit warning") {
		t.Errorf("Expected warning at 4/5, got: %s", w)
	}

	// Order 5: limit reached
	w = rt.RecordOrder([]string{"example.com"})
	if !strings.Contains(w, "order limit reached") {
		t.Errorf("Expected limit reached at 5/5, got: %s", w)
	}
}

func testConfig() *RateLimitConfig {
	return &RateLimitConfig{
		CertsPerDomainWindow:    1 * time.Hour,
		OrdersPerAccountWindow:  1 * time.Hour,
		FailedValidationsWindow: 1 * time.Hour,
		CertsPerDomainLimit:     CertsPerDomainPerWeek,
		OrdersPerAccountLimit:   OrdersPerAccountPer3Hours,
		FailedValidationsLimit:  FailedValidationsPerHour,
	}
}
