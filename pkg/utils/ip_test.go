package utils

import (
	"net"
	"testing"
)

func TestCIDRMatcher_Basic(t *testing.T) {
	cm := NewCIDRMatcher()

	// Add CIDR ranges
	if err := cm.Add("10.0.0.0/8"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := cm.Add("192.168.0.0/16"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Should match
	if !cm.Contains("10.1.2.3") {
		t.Error("Should match 10.1.2.3")
	}
	if !cm.Contains("192.168.1.1") {
		t.Error("Should match 192.168.1.1")
	}

	// Should not match
	if cm.Contains("172.16.0.1") {
		t.Error("Should not match 172.16.0.1")
	}
	if cm.Contains("8.8.8.8") {
		t.Error("Should not match 8.8.8.8")
	}
}

func TestCIDRMatcher_IPv6(t *testing.T) {
	cm := NewCIDRMatcher()

	if err := cm.Add("2001:db8::/32"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Should match
	if !cm.Contains("2001:db8::1") {
		t.Error("Should match 2001:db8::1")
	}

	// Should not match
	if cm.Contains("2001:db9::1") {
		t.Error("Should not match 2001:db9::1")
	}
}

func TestCIDRMatcher_InvalidCIDR(t *testing.T) {
	cm := NewCIDRMatcher()

	err := cm.Add("invalid")
	if err == nil {
		t.Error("Should fail for invalid CIDR")
	}
}

func TestCIDRMatcher_Clear(t *testing.T) {
	cm := NewCIDRMatcher()

	cm.Add("10.0.0.0/8")
	if !cm.Contains("10.1.2.3") {
		t.Error("Should match before Clear")
	}

	cm.Clear()

	if cm.Contains("10.1.2.3") {
		t.Error("Should not match after Clear")
	}
}

func TestCIDRMatcher_ContainsIP(t *testing.T) {
	cm := NewCIDRMatcher()
	cm.Add("10.0.0.0/8")

	ip := net.ParseIP("10.1.2.3")
	if !cm.ContainsIP(ip) {
		t.Error("Should match IP")
	}

	if cm.ContainsIP(nil) {
		t.Error("Should not match nil")
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"169.254.0.1", true},
		{"169.254.255.255", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
		{"192.169.0.1", false},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"2001:db8::1", false},
	}

	for _, tt := range tests {
		if got := IsPrivateIP(tt.ip); got != tt.private {
			t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func TestIsPrivateIP_Invalid(t *testing.T) {
	if IsPrivateIP("invalid") {
		t.Error("Should return false for invalid IP")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1", "192.168.1.1"},
		{"192.168.1.1:8080", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"10.0.0.1", "10.0.0.1"},
	}

	for _, tt := range tests {
		if got := ExtractIP(tt.input); got != tt.expected {
			t.Errorf("ExtractIP(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"80", 80},
		{"8080", 8080},
		{"443", 443},
		{"65535", 65535},
		{"0", 0},
		{"65536", 0},
		{"abc", 0},
		{"", 0},
	}

	for _, tt := range tests {
		if got := ParsePort(tt.input); got != tt.expected {
			t.Errorf("ParsePort(%s) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"invalid", false},
		{"", false},
		{"256.1.1.1", false},
	}

	for _, tt := range tests {
		if got := IsValidIP(tt.ip); got != tt.valid {
			t.Errorf("IsValidIP(%s) = %v, want %v", tt.ip, got, tt.valid)
		}
	}
}

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		ip     string
		isIPv4 bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"::1", false},
		{"2001:db8::1", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		if got := IsIPv4(tt.ip); got != tt.isIPv4 {
			t.Errorf("IsIPv4(%s) = %v, want %v", tt.ip, got, tt.isIPv4)
		}
	}
}

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		ip     string
		isIPv6 bool
	}{
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"::1", true},
		{"2001:db8::1", true},
		{"::", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		if got := IsIPv6(tt.ip); got != tt.isIPv6 {
			t.Errorf("IsIPv6(%s) = %v, want %v", tt.ip, got, tt.isIPv6)
		}
	}
}

func TestIPToUint32(t *testing.T) {
	tests := []struct {
		ip       string
		expected uint32
	}{
		{"0.0.0.0", 0},
		{"0.0.0.1", 1},
		{"1.0.0.0", 1 << 24},
		{"192.168.1.1", 0xC0A80101},
		{"255.255.255.255", 0xFFFFFFFF},
		{"::1", 0}, // IPv6 returns 0
		{"invalid", 0},
	}

	for _, tt := range tests {
		if got := IPToUint32(tt.ip); got != tt.expected {
			t.Errorf("IPToUint32(%s) = %d (0x%08x), want %d (0x%08x)",
				tt.ip, got, got, tt.expected, tt.expected)
		}
	}
}

func TestUint32ToIP(t *testing.T) {
	tests := []struct {
		n        uint32
		expected string
	}{
		{0, "0.0.0.0"},
		{1, "0.0.0.1"},
		{1 << 24, "1.0.0.0"},
		{0xC0A80101, "192.168.1.1"},
		{0xFFFFFFFF, "255.255.255.255"},
	}

	for _, tt := range tests {
		if got := Uint32ToIP(tt.n); got != tt.expected {
			t.Errorf("Uint32ToIP(%d) = %s, want %s", tt.n, got, tt.expected)
		}
	}
}
