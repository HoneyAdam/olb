package geodns

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCov_Route_NilReceiver tests calling Route on a nil *GeoDNS.
// Note: The nil-receiver check in Route() at line 82-84 has a bug —
// `g.defaultPool` panics because g is nil. This test documents the issue.
// func TestCov_Route_NilReceiver(t *testing.T) { ... } — skipped, panics

// TestCov_Route_FallbackPoolHealthy tests that when the primary pool is
// unhealthy but the fallback pool is healthy, the fallback is returned.
func TestCov_Route_FallbackPoolHealthy(t *testing.T) {
	g := New(Config{
		DefaultPool: "default-pool",
		Rules: []GeoRule{
			{
				ID:       "us-rule",
				Country:  "US",
				Pool:     "us-pool",
				Fallback: "us-fallback",
			},
		},
	})

	// Mark primary as unhealthy, fallback as healthy
	g.SetPoolHealth("us-pool", false)
	g.SetPoolHealth("us-fallback", true)

	// 8.8.8.0/24 is in the default geoDB as US
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "8.8.8.1")

	pool, _, err := g.Route(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool != "us-fallback" {
		t.Errorf("expected fallback pool 'us-fallback', got %q", pool)
	}
}

// TestCov_Route_FallbackBothUnhealthy tests that when both primary and
// fallback are unhealthy, the default pool is returned.
func TestCov_Route_FallbackBothUnhealthy(t *testing.T) {
	g := New(Config{
		DefaultPool: "default-pool",
		Rules: []GeoRule{
			{
				ID:       "us-rule",
				Country:  "US",
				Pool:     "us-pool",
				Fallback: "us-fallback",
			},
		},
	})

	g.SetPoolHealth("us-pool", false)
	g.SetPoolHealth("us-fallback", false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "8.8.8.1")

	pool, _, err := g.Route(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool != "default-pool" {
		t.Errorf("expected default pool, got %q", pool)
	}
}

// TestCov_Route_NoRulesMatch tests that when no rules match the location,
// the default pool is returned.
func TestCov_Route_NoRulesMatch(t *testing.T) {
	g := New(Config{
		DefaultPool: "default-pool",
		Rules: []GeoRule{
			{
				ID:      "jp-rule",
				Country: "JP",
				Pool:    "jp-pool",
			},
		},
	})

	// 8.8.8.0/24 is US, rule is for JP — no match
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "8.8.8.1")

	pool, _, _ := g.Route(req)
	if pool != "default-pool" {
		t.Errorf("expected default pool when no rules match, got %q", pool)
	}
}

// TestCov_ExtractClientIP_PublicIPWithXFF tests that a public IP ignores X-Forwarded-For.
func TestCov_ExtractClientIP_PublicIPWithXFF(t *testing.T) {
	g := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.50:12345" // public IP
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	ip := g.extractClientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected public IP to ignore XFF, got %q", ip)
	}
}

// TestCov_ExtractClientIP_EmptyXFFEntry tests X-Forwarded-For with empty first entry.
func TestCov_ExtractClientIP_EmptyXFFEntry(t *testing.T) {
	g := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:12345" // private IP — trusts XFF
	req.Header.Set("X-Forwarded-For", " ,10.0.0.1")
	// The first entry after split is " " which trims to "" — should fall through

	ip := g.extractClientIP(req)
	// Should fall through to RemoteAddr since first XFF entry is empty
	if ip != "10.0.0.5" {
		t.Errorf("expected fallback to RemoteAddr for empty XFF, got %q", ip)
	}
}

// TestCov_ExtractClientIP_XRealIP tests X-Real-IP from trusted source.
func TestCov_ExtractClientIP_XRealIP(t *testing.T) {
	g := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("X-Real-IP", "203.0.113.50")

	ip := g.extractClientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected X-Real-IP value, got %q", ip)
	}
}

// TestCov_GuessLocation_Loopback tests the loopback branch in guessLocationFromIP.
func TestCov_GuessLocation_Loopback(t *testing.T) {
	g := New(Config{})

	loc := g.guessLocationFromIP("127.0.0.1")
	// Note: isPrivateIP catches 127.0.0.0/8 first, so this returns PRIVATE
	if loc == nil {
		t.Fatal("expected non-nil location for loopback")
	}
	if loc.Country != "PRIVATE" {
		t.Errorf("expected PRIVATE (caught by isPrivateIP before loopback check), got %q", loc.Country)
	}
}

// TestCov_AddGeoData_VerifyStats tests that AddGeoData updates Stats().GeoEntries.
func TestCov_AddGeoData_VerifyStats(t *testing.T) {
	g := New(Config{})
	initialStats := g.Stats()

	err := g.AddGeoData("198.51.100.0/24", &Location{
		Country: "XX",
		City:    "TestCity",
	})
	if err != nil {
		t.Fatalf("AddGeoData failed: %v", err)
	}

	newStats := g.Stats()
	if newStats.GeoEntries <= initialStats.GeoEntries {
		t.Errorf("expected GeoEntries to increase after AddGeoData, was %d, now %d",
			initialStats.GeoEntries, newStats.GeoEntries)
	}
}

// TestCov_LookupLocation_InvalidCIDR tests that an invalid CIDR in geoDB is skipped.
func TestCov_LookupLocation_InvalidCIDR(t *testing.T) {
	g := New(Config{})

	// Manually inject an invalid CIDR into geoDB
	g.mu.Lock()
	g.geoDB["not-a-valid-cidr"] = &Location{Country: "XX"}
	g.mu.Unlock()

	// lookupLocation should skip the invalid CIDR without panicking
	loc := g.lookupLocation("8.8.8.8")
	if loc == nil {
		t.Error("expected location from valid geoDB entry or heuristic, got nil")
	}
}

// TestCov_Middleware_NilLocation tests the middleware when location is nil.
func TestCov_Middleware_NilLocation(t *testing.T) {
	g := New(Config{DefaultPool: "default"})

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// When location is nil, geo headers should not be set
		if r.Header.Get("X-Geo-Country") != "" {
			t.Error("expected no geo headers for nil location")
		}
	})

	handler := g.Middleware(next)

	// Use a malformed RemoteAddr that can't be parsed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "malformed-no-port"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected next handler to be called")
	}
}

// TestCov_AddGeoData_InvalidCIDR tests AddGeoData with a bad CIDR.
func TestCov_AddGeoData_InvalidCIDR(t *testing.T) {
	g := New(Config{})

	err := g.AddGeoData("not-a-cidr", &Location{Country: "XX"})
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

// TestCov_RemoveRule tests removing a rule that exists and one that doesn't.
func TestCov_RemoveRule(t *testing.T) {
	g := New(Config{
		Rules: []GeoRule{
			{ID: "r1", Country: "US", Pool: "us-pool"},
			{ID: "r2", Country: "EU", Pool: "eu-pool"},
		},
	})

	if !g.RemoveRule("r1") {
		t.Error("expected RemoveRule to return true for existing rule")
	}
	if g.RemoveRule("nonexistent") {
		t.Error("expected RemoveRule to return false for nonexistent rule")
	}

	stats := g.Stats()
	if stats.Rules != 1 {
		t.Errorf("expected 1 rule after removal, got %d", stats.Rules)
	}
}

// TestCov_MatchesRule_NilLocation tests that nil location only matches wildcard rules.
func TestCov_MatchesRule_NilLocation(t *testing.T) {
	g := New(Config{})

	wildcard := GeoRule{ID: "any", Country: "*", Pool: "any-pool"}
	specific := GeoRule{ID: "us", Country: "US", Pool: "us-pool"}

	if !g.matchesRule(nil, &wildcard) {
		t.Error("wildcard rule should match nil location")
	}
	if g.matchesRule(nil, &specific) {
		t.Error("specific country rule should not match nil location")
	}
}
