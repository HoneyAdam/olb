package ipacl

import (
	"testing"
	"time"
)

func TestIPAccessList_WhitelistBypass(t *testing.T) {
	acl, err := New(Config{
		Whitelist: []EntryConfig{
			{CIDR: "10.0.0.0/8", Reason: "internal"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if action := acl.Check("10.1.2.3"); action != ActionBypass {
		t.Errorf("expected ActionBypass for whitelisted IP, got %d", action)
	}
	if action := acl.Check("192.168.1.1"); action != ActionAllow {
		t.Errorf("expected ActionAllow for non-listed IP, got %d", action)
	}
}

func TestIPAccessList_BlacklistBlock(t *testing.T) {
	acl, err := New(Config{
		Blacklist: []EntryConfig{
			{CIDR: "203.0.113.0/24", Reason: "bad actor"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if action := acl.Check("203.0.113.50"); action != ActionBlock {
		t.Errorf("expected ActionBlock for blacklisted IP, got %d", action)
	}
	if action := acl.Check("8.8.8.8"); action != ActionAllow {
		t.Errorf("expected ActionAllow for non-listed IP, got %d", action)
	}
}

func TestIPAccessList_WhitelistPrecedence(t *testing.T) {
	acl, err := New(Config{
		Whitelist: []EntryConfig{{CIDR: "10.0.0.1/32", Reason: "trusted"}},
		Blacklist: []EntryConfig{{CIDR: "10.0.0.0/8", Reason: "blocked range"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	// Whitelist is checked first
	if action := acl.Check("10.0.0.1"); action != ActionBypass {
		t.Errorf("expected ActionBypass (whitelist precedence), got %d", action)
	}
	if action := acl.Check("10.0.0.2"); action != ActionBlock {
		t.Errorf("expected ActionBlock for non-whitelisted IP in blacklisted range, got %d", action)
	}
}

func TestIPAccessList_AutoBan(t *testing.T) {
	acl, err := New(Config{
		AutoBan: AutoBanConfig{
			Enabled:    true,
			DefaultTTL: time.Minute,
			MaxTTL:     time.Hour,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if err := acl.Ban("1.2.3.4", 0, "rate limit"); err != nil {
		t.Fatal(err)
	}

	if action := acl.Check("1.2.3.4"); action != ActionBlock {
		t.Errorf("expected ActionBlock for auto-banned IP, got %d", action)
	}
}

func TestIPAccessList_AddRemove(t *testing.T) {
	acl, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if err := acl.AddBlacklist("192.168.1.0/24", "test", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if action := acl.Check("192.168.1.100"); action != ActionBlock {
		t.Errorf("expected ActionBlock after AddBlacklist, got %d", action)
	}

	if !acl.RemoveBlacklist("192.168.1.0/24") {
		t.Error("expected RemoveBlacklist to return true")
	}
	if action := acl.Check("192.168.1.100"); action != ActionAllow {
		t.Errorf("expected ActionAllow after RemoveBlacklist, got %d", action)
	}
}

func TestIPAccessList_IPv6(t *testing.T) {
	acl, err := New(Config{
		Blacklist: []EntryConfig{{CIDR: "2001:db8::/32", Reason: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if action := acl.Check("2001:db8::1"); action != ActionBlock {
		t.Errorf("expected ActionBlock for IPv6 in blacklisted range, got %d", action)
	}
	if action := acl.Check("2001:db9::1"); action != ActionAllow {
		t.Errorf("expected ActionAllow for IPv6 not in range, got %d", action)
	}
}

func TestIPAccessList_ListRules(t *testing.T) {
	acl, err := New(Config{
		Whitelist: []EntryConfig{{CIDR: "10.0.0.0/8", Reason: "internal"}},
		Blacklist: []EntryConfig{{CIDR: "1.2.3.0/24", Reason: "bad"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	all := acl.ListRules("all")
	if len(all) != 2 {
		t.Errorf("expected 2 rules, got %d", len(all))
	}

	wl := acl.ListRules("whitelist")
	if len(wl) != 1 {
		t.Errorf("expected 1 whitelist rule, got %d", len(wl))
	}
}

func TestIPAccessList_InvalidCIDR(t *testing.T) {
	_, err := New(Config{
		Whitelist: []EntryConfig{{CIDR: "not-a-cidr", Reason: "test"}},
	})
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestIPAccessList_ExpiryCleanup(t *testing.T) {
	acl, err := New(Config{
		AutoBan: AutoBanConfig{
			Enabled:    true,
			DefaultTTL: 50 * time.Millisecond,
			MaxTTL:     time.Hour,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	// Ban with very short TTL
	if err := acl.Ban("1.2.3.4", 50*time.Millisecond, "test ban"); err != nil {
		t.Fatal(err)
	}

	// Should be blocked immediately
	if action := acl.Check("1.2.3.4"); action != ActionBlock {
		t.Errorf("expected ActionBlock immediately after ban, got %d", action)
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup
	acl.cleanupExpired()

	// Should be allowed after expiry + cleanup
	if action := acl.Check("1.2.3.4"); action != ActionAllow {
		t.Errorf("expected ActionAllow after expiry cleanup, got %d", action)
	}
}

func TestIPAccessList_WhitelistExpiryCleanup(t *testing.T) {
	acl, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	// Add whitelist with very short expiry
	expires := time.Now().Add(50 * time.Millisecond)
	if err := acl.AddWhitelist("10.0.0.0/8", "temp trust", expires); err != nil {
		t.Fatal(err)
	}

	// Should bypass immediately
	if action := acl.Check("10.0.0.1"); action != ActionBypass {
		t.Errorf("expected ActionBypass immediately after adding whitelist, got %d", action)
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup
	acl.cleanupExpired()

	// Should be allow (not bypass) after expiry
	if action := acl.Check("10.0.0.1"); action != ActionAllow {
		t.Errorf("expected ActionAllow after whitelist expiry, got %d", action)
	}
}

func TestFormatID(t *testing.T) {
	tests := []struct {
		prefix   string
		counter  int
		expected string
	}{
		{"wl", 0, "wl-0"},
		{"wl", 1, "wl-1"},
		{"bl", 10, "bl-10"},
		{"ab", 123, "ab-123"},
		{"wl", 9999, "wl-9999"},
	}

	for _, tt := range tests {
		got := formatID(tt.prefix, tt.counter)
		if got != tt.expected {
			t.Errorf("formatID(%q, %d) = %q, want %q", tt.prefix, tt.counter, got, tt.expected)
		}
	}
}

func TestIPAccessList_ListRules_Detail(t *testing.T) {
	acl, err := New(Config{
		Whitelist: []EntryConfig{
			{CIDR: "10.0.0.0/8", Reason: "internal"},
			{CIDR: "172.16.0.0/12", Reason: "vpn"},
		},
		Blacklist: []EntryConfig{
			{CIDR: "1.2.3.0/24", Reason: "bad actor"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	// Test "all" filter
	all := acl.ListRules("")
	if len(all) != 3 {
		t.Errorf("expected 3 total rules, got %d", len(all))
	}

	// Test whitelist filter
	wl := acl.ListRules("whitelist")
	if len(wl) != 2 {
		t.Errorf("expected 2 whitelist rules, got %d", len(wl))
	}

	// Test blacklist filter
	bl := acl.ListRules("blacklist")
	if len(bl) != 1 {
		t.Errorf("expected 1 blacklist rule, got %d", len(bl))
	}

	// Verify rule metadata
	for _, r := range wl {
		if r.ID == "" {
			t.Error("expected non-empty rule ID")
		}
		if r.CIDR == "" {
			t.Error("expected non-empty CIDR")
		}
		if r.Source != "manual" {
			t.Errorf("expected source 'manual', got %q", r.Source)
		}
		if r.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}
	}
}

func TestIPAccessList_Ban_DisabledAutoBan(t *testing.T) {
	acl, err := New(Config{
		AutoBan: AutoBanConfig{Enabled: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	err = acl.Ban("1.2.3.4", time.Hour, "test")
	if err != nil {
		t.Errorf("expected no error when auto-ban is disabled, got %v", err)
	}

	// Should NOT be blocked since auto-ban is disabled
	if action := acl.Check("1.2.3.4"); action != ActionAllow {
		t.Errorf("expected ActionAllow when auto-ban is disabled, got %d", action)
	}
}

func TestIPAccessList_Ban_MaxTTL(t *testing.T) {
	acl, err := New(Config{
		AutoBan: AutoBanConfig{
			Enabled:    true,
			DefaultTTL: time.Minute,
			MaxTTL:     time.Hour,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	// Request a 48h TTL, should be capped to MaxTTL (1h)
	if err := acl.Ban("1.2.3.4", 48*time.Hour, "test"); err != nil {
		t.Fatal(err)
	}

	// Verify the ban exists
	if action := acl.Check("1.2.3.4"); action != ActionBlock {
		t.Errorf("expected ActionBlock for banned IP, got %d", action)
	}

	// Check metadata for TTL cap
	rules := acl.ListRules("blacklist")
	found := false
	for _, r := range rules {
		if r.CIDR == "1.2.3.4/32" {
			found = true
			if r.Source != "auto-ban" {
				t.Errorf("expected source 'auto-ban', got %q", r.Source)
			}
		}
	}
	if !found {
		t.Error("expected to find banned IP in blacklist rules")
	}
}

func TestIPAccessList_RemoveWhitelist_NotFound(t *testing.T) {
	acl, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if acl.RemoveWhitelist("1.2.3.4/32") {
		t.Error("expected RemoveWhitelist to return false for nonexistent entry")
	}
}

func TestIPAccessList_RemoveBlacklist_NotFound(t *testing.T) {
	acl, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if acl.RemoveBlacklist("1.2.3.4/32") {
		t.Error("expected RemoveBlacklist to return false for nonexistent entry")
	}
}

func TestIPAccessList_AddRemoveWhitelist(t *testing.T) {
	acl, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer acl.Stop()

	if err := acl.AddWhitelist("10.0.0.0/8", "test", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if action := acl.Check("10.1.2.3"); action != ActionBypass {
		t.Errorf("expected ActionBypass, got %d", action)
	}

	if !acl.RemoveWhitelist("10.0.0.0/8") {
		t.Error("expected RemoveWhitelist to return true")
	}
	if action := acl.Check("10.1.2.3"); action != ActionAllow {
		t.Errorf("expected ActionAllow after remove, got %d", action)
	}
}

func TestRuleMetadata_IsExpired(t *testing.T) {
	// Non-expiring rule
	r1 := &RuleMetadata{ExpiresAt: time.Time{}}
	if r1.IsExpired() {
		t.Error("expected non-expiring rule to not be expired")
	}

	// Future expiry
	r2 := &RuleMetadata{ExpiresAt: time.Now().Add(time.Hour)}
	if r2.IsExpired() {
		t.Error("expected future-expiring rule to not be expired")
	}

	// Past expiry
	r3 := &RuleMetadata{ExpiresAt: time.Now().Add(-time.Hour)}
	if !r3.IsExpired() {
		t.Error("expected past-expiring rule to be expired")
	}
}

func TestIPAccessList_InvalidBlacklistCIDR(t *testing.T) {
	_, err := New(Config{
		Blacklist: []EntryConfig{{CIDR: "invalid", Reason: "test"}},
	})
	if err == nil {
		t.Error("expected error for invalid blacklist CIDR")
	}
}
