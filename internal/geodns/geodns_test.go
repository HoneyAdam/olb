package geodns

import (
	"net"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		DefaultPool: "default",
		Rules: []GeoRule{
			{ID: "us", Country: "US", Pool: "us-pool"},
			{ID: "eu", Country: "EU", Pool: "eu-pool"},
		},
	}

	g := New(cfg)
	if g == nil {
		t.Fatal("New() returned nil")
	}

	stats := g.Stats()
	if stats.Rules != 2 {
		t.Errorf("expected 2 rules, got %d", stats.Rules)
	}
}

func TestGeoDNS_extractClientIP(t *testing.T) {
	g := New(Config{DefaultPool: "default"})

	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.168.1.100:12345",
			want:       "192.168.1.100",
		},
		{
			name:       "with X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 10.0.0.1"},
			want:       "203.0.113.1",
		},
		{
			name:       "with X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Real-IP": "198.51.100.1"},
			want:       "198.51.100.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := g.extractClientIP(req)
			if got != tt.want {
				t.Errorf("extractClientIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeoDNS_matchesRule(t *testing.T) {
	g := New(Config{DefaultPool: "default"})

	tests := []struct {
		name    string
		loc     *Location
		rule    GeoRule
		matches bool
	}{
		{
			name:    "exact country match",
			loc:     &Location{Country: "US"},
			rule:    GeoRule{Country: "US"},
			matches: true,
		},
		{
			name:    "country mismatch",
			loc:     &Location{Country: "UK"},
			rule:    GeoRule{Country: "US"},
			matches: false,
		},
		{
			name:    "wildcard country",
			loc:     &Location{Country: "XX"},
			rule:    GeoRule{Country: "*"},
			matches: true,
		},
		{
			name:    "region match",
			loc:     &Location{Country: "US", Region: "CA"},
			rule:    GeoRule{Country: "US", Region: "CA"},
			matches: true,
		},
		{
			name:    "region mismatch",
			loc:     &Location{Country: "US", Region: "NY"},
			rule:    GeoRule{Country: "US", Region: "CA"},
			matches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.matchesRule(tt.loc, &tt.rule)
			if got != tt.matches {
				t.Errorf("matchesRule() = %v, want %v", got, tt.matches)
			}
		})
	}
}

func TestGeoDNS_AddRule(t *testing.T) {
	g := New(Config{DefaultPool: "default"})

	rule := GeoRule{
		ID:      "test-rule",
		Country: "JP",
		Pool:    "jp-pool",
	}

	if err := g.AddRule(rule); err != nil {
		t.Errorf("AddRule() error = %v", err)
	}

	// Duplicate ID should fail
	if err := g.AddRule(rule); err == nil {
		t.Error("AddRule() with duplicate ID should fail")
	}

	// Missing ID should fail
	rule2 := GeoRule{Country: "CN", Pool: "cn-pool"}
	if err := g.AddRule(rule2); err == nil {
		t.Error("AddRule() without ID should fail")
	}

	// Missing Pool should fail
	rule3 := GeoRule{ID: "test3", Country: "KR"}
	if err := g.AddRule(rule3); err == nil {
		t.Error("AddRule() without Pool should fail")
	}
}

func TestGeoDNS_RemoveRule(t *testing.T) {
	g := New(Config{
		DefaultPool: "default",
		Rules: []GeoRule{
			{ID: "rule1", Country: "US", Pool: "us-pool"},
			{ID: "rule2", Country: "EU", Pool: "eu-pool"},
		},
	})

	if !g.RemoveRule("rule1") {
		t.Error("RemoveRule() should return true for existing rule")
	}

	if g.RemoveRule("rule1") {
		t.Error("RemoveRule() should return false for already removed rule")
	}

	if g.RemoveRule("nonexistent") {
		t.Error("RemoveRule() should return false for non-existent rule")
	}

	stats := g.Stats()
	if stats.Rules != 1 {
		t.Errorf("expected 1 rule after removal, got %d", stats.Rules)
	}
}

func TestGeoDNS_SetPoolHealth(t *testing.T) {
	g := New(Config{
		DefaultPool: "default",
		Rules: []GeoRule{
			{ID: "rule1", Country: "US", Pool: "us-pool", Fallback: "default"},
		},
	})

	// Mark pool as unhealthy
	g.SetPoolHealth("us-pool", false)

	// Route should return fallback
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "3.0.0.1:12345" // US IP

	pool, _, err := g.Route(req)
	if err != nil {
		t.Errorf("Route() error = %v", err)
	}
	if pool != "default" {
		t.Errorf("expected fallback pool 'default', got '%s'", pool)
	}

	// Mark pool as healthy
	g.SetPoolHealth("us-pool", true)

	pool, _, err = g.Route(req)
	if err != nil {
		t.Errorf("Route() error = %v", err)
	}
	if pool != "us-pool" {
		t.Errorf("expected pool 'us-pool', got '%s'", pool)
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"10.x.x.x", "10.0.0.1", true},
		{"172.16.x.x", "172.16.0.1", true},
		{"192.168.x.x", "192.168.1.1", true},
		{"127.x.x.x", "127.0.0.1", true},
		{"public", "8.8.8.8", false},
		{"public", "1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := isPrivateIP(ip); got != tt.want {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIpToInt(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want uint32
	}{
		{"8.8.8.8", "8.8.8.8", 0x08080808},
		{"0.0.0.0", "0.0.0.0", 0},
		{"255.255.255.255", "255.255.255.255", 0xFFFFFFFF},
		{"192.168.1.1", "192.168.1.1", 0xC0A80101},
		{"1.2.3.4", "1.2.3.4", 0x01020304},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			got := ipToInt(ip)
			if got != tt.want {
				t.Errorf("ipToInt(%s) = 0x%08X, want 0x%08X", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIpToInt_IPv6(t *testing.T) {
	// IPv6 should return 0 since To4() returns nil
	ip := net.ParseIP("::1")
	if ip == nil {
		t.Fatal("failed to parse IPv6")
	}
	got := ipToInt(ip)
	if got != 0 {
		t.Errorf("ipToInt(::1) = %d, want 0", got)
	}
}

func TestGuessLocationFromIP_PublicUS(t *testing.T) {
	g := New(Config{})
	loc := g.guessLocationFromIP("3.100.50.25")
	if loc == nil {
		t.Fatal("expected non-nil location")
	}
	if loc.Country != "US" {
		t.Errorf("Country = %q, want US", loc.Country)
	}
}

func TestGuessLocationFromIP_PublicEU(t *testing.T) {
	g := New(Config{})
	loc := g.guessLocationFromIP("5.100.50.25")
	if loc == nil {
		t.Fatal("expected non-nil location")
	}
	if loc.Country != "EU" {
		t.Errorf("Country = %q, want EU", loc.Country)
	}
}

func TestGuessLocationFromIP_Loopback(t *testing.T) {
	g := New(Config{})
	loc := g.guessLocationFromIP("127.0.0.1")
	if loc == nil {
		t.Fatal("expected non-nil location")
	}
	// 127.0.0.1 is classified as PRIVATE by isPrivateIP (which checks 127.0.0.0/8)
	// before the loopback check runs. This is the current behavior.
	if loc.Country != "PRIVATE" {
		t.Errorf("Country = %q, want PRIVATE (isPrivateIP catches loopback range first)", loc.Country)
	}
}

func TestGuessLocationFromIP_Private(t *testing.T) {
	g := New(Config{})
	loc := g.guessLocationFromIP("192.168.1.1")
	if loc == nil {
		t.Fatal("expected non-nil location")
	}
	if loc.Country != "PRIVATE" {
		t.Errorf("Country = %q, want PRIVATE", loc.Country)
	}
}

func TestGuessLocationFromIP_Unknown(t *testing.T) {
	g := New(Config{})
	// IP in a range not mapped to any known region
	loc := g.guessLocationFromIP("100.100.100.100")
	if loc == nil {
		t.Fatal("expected non-nil location (should return UNKNOWN)")
	}
	if loc.Country != "UNKNOWN" {
		t.Errorf("Country = %q, want UNKNOWN", loc.Country)
	}
}

func TestGuessLocationFromIP_Invalid(t *testing.T) {
	g := New(Config{})
	loc := g.guessLocationFromIP("not-an-ip")
	if loc != nil {
		t.Errorf("expected nil for invalid IP, got %v", loc)
	}
}

func TestLookupLocation(t *testing.T) {
	g := New(Config{})
	// Load default geo data includes 8.8.8.0/24
	loc := g.lookupLocation("8.8.8.8")
	if loc == nil {
		t.Fatal("expected non-nil location for 8.8.8.8")
	}
	if loc.Country != "US" {
		t.Errorf("Country = %q, want US", loc.Country)
	}
}

func TestLookupLocation_InvalidIP(t *testing.T) {
	g := New(Config{})
	loc := g.lookupLocation("invalid")
	if loc != nil {
		t.Errorf("expected nil for invalid IP, got %v", loc)
	}
}

func TestMatchesRule_NilLocation_Wildcard(t *testing.T) {
	g := New(Config{})
	rule := &GeoRule{Country: "*"}
	if !g.matchesRule(nil, rule) {
		t.Error("wildcard should match nil location")
	}
}

func TestMatchesRule_NilLocation_NonWildcard(t *testing.T) {
	g := New(Config{})
	rule := &GeoRule{Country: "US"}
	if g.matchesRule(nil, rule) {
		t.Error("non-wildcard should not match nil location")
	}
}

func TestMatchesRule_CountryMismatch(t *testing.T) {
	g := New(Config{})
	loc := &Location{Country: "EU"}
	rule := &GeoRule{Country: "US"}
	if g.matchesRule(loc, rule) {
		t.Error("EU location should not match US rule")
	}
}

func TestMatchesRule_RegionMismatch(t *testing.T) {
	g := New(Config{})
	loc := &Location{Country: "US", Region: "CA"}
	rule := &GeoRule{Country: "US", Region: "NY"}
	if g.matchesRule(loc, rule) {
		t.Error("CA region should not match NY rule")
	}
}

func TestMatchesRule_RegionMatch(t *testing.T) {
	g := New(Config{})
	loc := &Location{Country: "US", Region: "CA"}
	rule := &GeoRule{Country: "US", Region: "CA"}
	if !g.matchesRule(loc, rule) {
		t.Error("CA region should match CA rule")
	}
}
