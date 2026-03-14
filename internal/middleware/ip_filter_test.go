package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPFilterMiddleware_Name(t *testing.T) {
	m, err := NewIPFilterMiddleware(DefaultIPFilterConfig())
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}
	if got := m.Name(); got != "ip-filter" {
		t.Errorf("Name() = %q, want %q", got, "ip-filter")
	}
}

func TestIPFilterMiddleware_Priority(t *testing.T) {
	m, err := NewIPFilterMiddleware(DefaultIPFilterConfig())
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}
	if got := m.Priority(); got != PrioritySecurity {
		t.Errorf("Priority() = %d, want %d", got, PrioritySecurity)
	}
}

func TestIPFilterMiddleware_DefaultConfig(t *testing.T) {
	cfg := DefaultIPFilterConfig()
	if cfg.DefaultAction != "allow" {
		t.Errorf("DefaultAction = %q, want %q", cfg.DefaultAction, "allow")
	}
	if cfg.TrustXForwardedFor {
		t.Error("TrustXForwardedFor should be false by default")
	}
	if len(cfg.AllowList) != 0 {
		t.Errorf("AllowList should be empty, got %d entries", len(cfg.AllowList))
	}
	if len(cfg.DenyList) != 0 {
		t.Errorf("DenyList should be empty, got %d entries", len(cfg.DenyList))
	}
}

func TestIPFilterMiddleware_InvalidDefaultAction(t *testing.T) {
	_, err := NewIPFilterMiddleware(IPFilterConfig{
		DefaultAction: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid default action")
	}
}

func TestIPFilterMiddleware_InvalidAllowCIDR(t *testing.T) {
	_, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList: []string{"invalid-cidr/99"},
	})
	if err == nil {
		t.Error("expected error for invalid allow CIDR")
	}
}

func TestIPFilterMiddleware_InvalidDenyCIDR(t *testing.T) {
	_, err := NewIPFilterMiddleware(IPFilterConfig{
		DenyList: []string{"invalid-cidr/99"},
	})
	if err == nil {
		t.Error("expected error for invalid deny CIDR")
	}
}

func TestIPFilterMiddleware_InvalidAllowIP(t *testing.T) {
	_, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList: []string{"not.an.ip.address"},
	})
	if err == nil {
		t.Error("expected error for invalid allow IP")
	}
}

func TestIPFilterMiddleware_InvalidDenyIP(t *testing.T) {
	_, err := NewIPFilterMiddleware(IPFilterConfig{
		DenyList: []string{"not.an.ip.address"},
	})
	if err == nil {
		t.Error("expected error for invalid deny IP")
	}
}

// TestIPFilterMiddleware_EmptyLists_DefaultAllow tests that with empty lists
// and default action "allow", all traffic passes through.
func TestIPFilterMiddleware_EmptyLists_DefaultAllow(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"allows any IPv4", "192.168.1.1:1234", http.StatusOK},
		{"allows loopback", "127.0.0.1:1234", http.StatusOK},
		{"allows any IPv6", "[2001:db8::1]:1234", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_EmptyLists_DefaultDeny tests that with empty lists
// and default action "deny", all traffic is blocked.
func TestIPFilterMiddleware_EmptyLists_DefaultDeny(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"denies any IPv4", "192.168.1.1:1234", http.StatusForbidden},
		{"denies loopback", "127.0.0.1:1234", http.StatusForbidden},
		{"denies any IPv6", "[2001:db8::1]:1234", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_AllowListOnly tests that with only an allow list,
// only listed IPs are allowed and all others are denied.
func TestIPFilterMiddleware_AllowListOnly(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"192.168.1.100", "10.0.0.5"},
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"allowed IP passes", "192.168.1.100:1234", http.StatusOK},
		{"second allowed IP passes", "10.0.0.5:1234", http.StatusOK},
		{"unlisted IP denied", "172.16.0.1:1234", http.StatusForbidden},
		{"random IP denied", "8.8.8.8:1234", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_DenyListOnly tests that with only a deny list,
// listed IPs are denied and all others use the default action.
func TestIPFilterMiddleware_DenyListOnly(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		DenyList:      []string{"10.0.0.1", "192.168.1.50"},
		DefaultAction: "allow",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"denied IP blocked", "10.0.0.1:1234", http.StatusForbidden},
		{"second denied IP blocked", "192.168.1.50:1234", http.StatusForbidden},
		{"unlisted IP allowed", "172.16.0.1:1234", http.StatusOK},
		{"random IP allowed", "8.8.8.8:1234", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_DenyPrecedenceOverAllow tests that when an IP appears
// in both deny and allow lists, the deny list takes precedence.
func TestIPFilterMiddleware_DenyPrecedenceOverAllow(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"10.0.0.0/8"},
		DenyList:      []string{"10.0.0.1"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"denied IP blocked even though in allow CIDR", "10.0.0.1:1234", http.StatusForbidden},
		{"other IP in allow CIDR passes", "10.0.0.2:1234", http.StatusOK},
		{"IP outside both lists denied", "192.168.1.1:1234", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_CIDRRangeMatching tests CIDR range matching for both
// allow and deny lists.
func TestIPFilterMiddleware_CIDRRangeMatching(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"192.168.1.0/24", "10.0.0.0/8"},
		DenyList:      []string{"172.16.0.0/12"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"IP in allowed /24", "192.168.1.50:1234", http.StatusOK},
		{"IP at boundary of /24", "192.168.1.254:1234", http.StatusOK},
		{"IP outside /24", "192.168.2.1:1234", http.StatusForbidden},
		{"IP in allowed /8", "10.255.255.255:1234", http.StatusOK},
		{"IP in denied /12", "172.16.5.1:1234", http.StatusForbidden},
		{"IP in denied /12 upper boundary", "172.31.255.255:1234", http.StatusForbidden},
		{"IP outside denied /12", "172.32.0.1:1234", http.StatusForbidden}, // Not in allow either
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_IndividualIPMatching tests matching of individual IPs
// (not CIDR ranges) in both allow and deny lists.
func TestIPFilterMiddleware_IndividualIPMatching(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"1.2.3.4", "5.6.7.8"},
		DenyList:      []string{"9.8.7.6"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"exact match allowed", "1.2.3.4:1234", http.StatusOK},
		{"second exact match allowed", "5.6.7.8:1234", http.StatusOK},
		{"exact match denied", "9.8.7.6:1234", http.StatusForbidden},
		{"non-matching denied by default", "11.22.33.44:1234", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_IPv6Support tests that IPv6 addresses work correctly
// for both individual IPs and CIDR ranges.
func TestIPFilterMiddleware_IPv6Support(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"2001:db8::/32", "::1"},
		DenyList:      []string{"2001:db8:1::1"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"IPv6 in allowed CIDR", "[2001:db8::100]:1234", http.StatusOK},
		{"IPv6 loopback allowed", "[::1]:1234", http.StatusOK},
		{"IPv6 in denied list (also in allowed CIDR)", "[2001:db8:1::1]:1234", http.StatusForbidden},
		{"IPv6 outside allowed CIDR", "[2001:db9::1]:1234", http.StatusForbidden},
		{"IPv6 in allowed range", "[2001:db8:0:1::5]:1234", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_XForwardedFor_Trusted tests that X-Forwarded-For
// is used when TrustXForwardedFor is enabled.
func TestIPFilterMiddleware_XForwardedFor_Trusted(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:          []string{"1.2.3.4"},
		DenyList:           []string{"9.9.9.9"},
		DefaultAction:      "deny",
		TrustXForwardedFor: true,
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		xff      string
		wantCode int
	}{
		{
			name:     "XFF allowed IP passes even though RemoteAddr is denied",
			remoteIP: "9.9.9.9:1234",
			xff:      "1.2.3.4",
			wantCode: http.StatusOK,
		},
		{
			name:     "XFF denied IP blocked even though RemoteAddr would be allowed",
			remoteIP: "1.2.3.4:1234",
			xff:      "9.9.9.9",
			wantCode: http.StatusForbidden,
		},
		{
			name:     "XFF with multiple IPs uses leftmost",
			remoteIP: "10.0.0.1:1234",
			xff:      "1.2.3.4, 10.0.0.1",
			wantCode: http.StatusOK,
		},
		{
			name:     "no XFF falls back to RemoteAddr",
			remoteIP: "1.2.3.4:1234",
			xff:      "",
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.xff != "" {
				headers["X-Forwarded-For"] = tt.xff
			}
			code := serveIPFilter(t, m, tt.remoteIP, headers)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_XForwardedFor_NotTrusted tests that X-Forwarded-For
// is ignored when TrustXForwardedFor is disabled.
func TestIPFilterMiddleware_XForwardedFor_NotTrusted(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:          []string{"10.0.0.1"},
		DefaultAction:      "deny",
		TrustXForwardedFor: false,
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	// RemoteAddr is 10.0.0.1 (allowed), XFF says 9.9.9.9 but should be ignored
	headers := map[string]string{"X-Forwarded-For": "9.9.9.9"}
	code := serveIPFilter(t, m, "10.0.0.1:1234", headers)
	if code != http.StatusOK {
		t.Errorf("status = %d, want %d (XFF should be ignored)", code, http.StatusOK)
	}

	// RemoteAddr is 9.9.9.9 (not allowed), XFF says 10.0.0.1 but should be ignored
	headers = map[string]string{"X-Forwarded-For": "10.0.0.1"}
	code = serveIPFilter(t, m, "9.9.9.9:1234", headers)
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (XFF should be ignored)", code, http.StatusForbidden)
	}
}

// TestIPFilterMiddleware_RuntimeUpdate tests that allow/deny lists can be
// updated at runtime in a thread-safe manner.
func TestIPFilterMiddleware_RuntimeUpdate(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"1.2.3.4"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	// Initially, 1.2.3.4 is allowed and 5.6.7.8 is denied
	code := serveIPFilter(t, m, "1.2.3.4:1234", nil)
	if code != http.StatusOK {
		t.Errorf("before update: 1.2.3.4 status = %d, want %d", code, http.StatusOK)
	}
	code = serveIPFilter(t, m, "5.6.7.8:1234", nil)
	if code != http.StatusForbidden {
		t.Errorf("before update: 5.6.7.8 status = %d, want %d", code, http.StatusForbidden)
	}

	// Update the lists: now 5.6.7.8 is allowed and 1.2.3.4 is denied
	err = m.UpdateLists(
		[]string{"5.6.7.8"},
		[]string{"1.2.3.4"},
	)
	if err != nil {
		t.Fatalf("UpdateLists() error = %v", err)
	}

	// After update, 5.6.7.8 should be allowed and 1.2.3.4 should be denied
	code = serveIPFilter(t, m, "5.6.7.8:1234", nil)
	if code != http.StatusOK {
		t.Errorf("after update: 5.6.7.8 status = %d, want %d", code, http.StatusOK)
	}
	code = serveIPFilter(t, m, "1.2.3.4:1234", nil)
	if code != http.StatusForbidden {
		t.Errorf("after update: 1.2.3.4 status = %d, want %d", code, http.StatusForbidden)
	}
}

// TestIPFilterMiddleware_RuntimeUpdateInvalid tests that invalid entries
// in UpdateLists are rejected without changing the current state.
func TestIPFilterMiddleware_RuntimeUpdateInvalid(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"1.2.3.4"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	// Try to update with invalid entries
	err = m.UpdateLists([]string{"invalid-ip"}, nil)
	if err == nil {
		t.Error("UpdateLists() expected error for invalid IP")
	}

	// Original config should still be in effect
	code := serveIPFilter(t, m, "1.2.3.4:1234", nil)
	if code != http.StatusOK {
		t.Errorf("after failed update: 1.2.3.4 status = %d, want %d", code, http.StatusOK)
	}
}

// TestIPFilterMiddleware_Config tests that Config() returns a copy of the configuration.
func TestIPFilterMiddleware_Config(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:          []string{"1.2.3.4"},
		DenyList:           []string{"5.6.7.8"},
		DefaultAction:      "deny",
		TrustXForwardedFor: true,
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	cfg := m.Config()
	if cfg.DefaultAction != "deny" {
		t.Errorf("Config().DefaultAction = %q, want %q", cfg.DefaultAction, "deny")
	}
	if !cfg.TrustXForwardedFor {
		t.Error("Config().TrustXForwardedFor should be true")
	}
	if len(cfg.AllowList) != 1 || cfg.AllowList[0] != "1.2.3.4" {
		t.Errorf("Config().AllowList = %v, want [1.2.3.4]", cfg.AllowList)
	}
	if len(cfg.DenyList) != 1 || cfg.DenyList[0] != "5.6.7.8" {
		t.Errorf("Config().DenyList = %v, want [5.6.7.8]", cfg.DenyList)
	}

	// Verify it's a copy (modifying returned config doesn't affect middleware)
	cfg.AllowList[0] = "modified"
	cfg2 := m.Config()
	if cfg2.AllowList[0] != "1.2.3.4" {
		t.Errorf("Config() returned a reference instead of a copy")
	}
}

// TestIPFilterMiddleware_MixedIPAndCIDR tests mixing individual IPs and
// CIDR ranges in the same list.
func TestIPFilterMiddleware_MixedIPAndCIDR(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"1.2.3.4", "10.0.0.0/8"},
		DenyList:      []string{"10.0.0.1", "172.16.0.0/12"},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"individual allow", "1.2.3.4:1234", http.StatusOK},
		{"CIDR allow", "10.0.0.50:1234", http.StatusOK},
		{"individual deny overrides CIDR allow", "10.0.0.1:1234", http.StatusForbidden},
		{"CIDR deny", "172.16.5.5:1234", http.StatusForbidden},
		{"neither list, default deny", "8.8.8.8:1234", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := serveIPFilter(t, m, tt.remoteIP, nil)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

// TestIPFilterMiddleware_ImplementsMiddleware verifies that IPFilterMiddleware
// satisfies the Middleware interface.
func TestIPFilterMiddleware_ImplementsMiddleware(t *testing.T) {
	m, err := NewIPFilterMiddleware(DefaultIPFilterConfig())
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	// Compile-time check that IPFilterMiddleware implements Middleware
	var _ Middleware = m
}

// TestIPFilterMiddleware_WhitespaceEntries tests that whitespace in list entries
// is handled correctly.
func TestIPFilterMiddleware_WhitespaceEntries(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"  1.2.3.4  ", " 10.0.0.0/8 "},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	code := serveIPFilter(t, m, "1.2.3.4:1234", nil)
	if code != http.StatusOK {
		t.Errorf("whitespace-trimmed IP: status = %d, want %d", code, http.StatusOK)
	}

	code = serveIPFilter(t, m, "10.5.5.5:1234", nil)
	if code != http.StatusOK {
		t.Errorf("whitespace-trimmed CIDR: status = %d, want %d", code, http.StatusOK)
	}
}

// TestIPFilterMiddleware_EmptyEntries tests that empty strings in lists
// are gracefully ignored.
func TestIPFilterMiddleware_EmptyEntries(t *testing.T) {
	m, err := NewIPFilterMiddleware(IPFilterConfig{
		AllowList:     []string{"", "1.2.3.4", ""},
		DefaultAction: "deny",
	})
	if err != nil {
		t.Fatalf("NewIPFilterMiddleware() error = %v", err)
	}

	code := serveIPFilter(t, m, "1.2.3.4:1234", nil)
	if code != http.StatusOK {
		t.Errorf("status = %d, want %d", code, http.StatusOK)
	}
}

// serveIPFilter is a test helper that creates a request with the given remote
// address and optional headers, runs it through the IP filter, and returns
// the response status code.
func serveIPFilter(t *testing.T, m *IPFilterMiddleware, remoteAddr string, headers map[string]string) int {
	t.Helper()

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return rr.Code
}
