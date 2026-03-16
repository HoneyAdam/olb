package botdetect

import (
	"net/http/httptest"
	"testing"
)

func TestBotDetector_WithBehavior(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		BehaviorEnabled: true,
		BehaviorConfig: BehaviorConfig{
			RPSThreshold:       10,
			ErrorRateThreshold: 30,
		},
	})
	defer bd.Stop()

	// Normal request with behavior enabled — should pass
	req := httptest.NewRequest("GET", "http://example.com/page1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36")
	result := bd.Analyze(req)
	if result.Blocked {
		t.Error("expected normal request to pass with behavior enabled")
	}
}

func TestBotDetector_BehaviorDisabled(t *testing.T) {
	bd := New(Config{
		UAEnabled:       false,
		BehaviorEnabled: false,
	})
	defer bd.Stop()

	// With everything disabled, should always pass
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "sqlmap/1.7")
	result := bd.Analyze(req)
	if result.Blocked {
		t.Error("expected all checks to pass when disabled")
	}
	if result.Score != 0 {
		t.Errorf("expected score 0 when all checks disabled, got %d", result.Score)
	}
}

func TestBotDetector_UABlockKnownScanners(t *testing.T) {
	bd := New(Config{
		UAEnabled:            true,
		UABlockKnownScanners: true,
		BehaviorEnabled:      false,
	})
	defer bd.Stop()

	scanners := []string{"sqlmap/1.7", "nikto/2.1.6", "Nmap Scripting Engine"}
	for _, ua := range scanners {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("User-Agent", ua)
		result := bd.Analyze(req)
		if !result.Blocked {
			t.Errorf("expected scanner %q to be blocked", ua)
		}
	}
}

func TestBotDetector_UABlockEmpty(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		UABlockEmpty:    true,
		BehaviorEnabled: false,
	})
	defer bd.Stop()

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Del("User-Agent")
	result := bd.Analyze(req)
	// Empty UA gets score 40, which is below the 70 block threshold
	if result.Score < 30 {
		t.Errorf("expected score >= 30 for empty UA, got %d", result.Score)
	}
}

func TestBotDetector_Stop_NilBehavior(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		BehaviorEnabled: false,
	})
	// Should not panic
	bd.Stop()
}

func TestBotDetector_ExtractBotIP(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"10.0.0.1:12345", "10.0.0.1"},
		{"plain-addr", "plain-addr"},
		{"[::1]:8080", "::1"},
	}

	for _, tt := range tests {
		got := extractBotIP(tt.addr)
		if got != tt.expected {
			t.Errorf("extractBotIP(%q) = %q, want %q", tt.addr, got, tt.expected)
		}
	}
}

func TestBotDetector_HighScoreBlocked(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		BehaviorEnabled: false,
	})
	defer bd.Stop()

	// Score >= 70 should be blocked
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "sqlmap/1.7") // score 90
	result := bd.Analyze(req)
	if !result.Blocked {
		t.Error("expected blocked for score >= 70")
	}
	if result.Score < 70 {
		t.Errorf("expected score >= 70, got %d", result.Score)
	}
}

func TestBotDetector_LowScoreNotBlocked(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		BehaviorEnabled: false,
	})
	defer bd.Stop()

	// Normal browser — score < 70, should not be blocked
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	result := bd.Analyze(req)
	if result.Blocked {
		t.Error("expected not blocked for normal browser")
	}
}
