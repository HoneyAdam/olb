package botdetect

import (
	"net/http/httptest"
	"testing"
)

func TestAnalyzeUA_Empty(t *testing.T) {
	result := AnalyzeUA("")
	if result.Score < 30 {
		t.Errorf("expected score >= 30 for empty UA, got %d", result.Score)
	}
}

func TestAnalyzeUA_KnownScanner(t *testing.T) {
	scanners := []string{
		"sqlmap/1.0",
		"Nikto/2.1.6",
		"Mozilla/5.0 (compatible; Nmap Scripting Engine)",
		"gobuster/3.0",
	}
	for _, ua := range scanners {
		result := AnalyzeUA(ua)
		if result.Score < 80 {
			t.Errorf("expected score >= 80 for scanner %q, got %d", ua, result.Score)
		}
	}
}

func TestAnalyzeUA_LegitBrowser(t *testing.T) {
	legit := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Firefox/121.0",
	}
	for _, ua := range legit {
		result := AnalyzeUA(ua)
		if result.Score >= 30 {
			t.Errorf("expected score < 30 for legit browser %q, got %d (rule: %s)", ua, result.Score, result.Rule)
		}
	}
}

func TestAnalyzeUA_OutdatedBrowser(t *testing.T) {
	result := AnalyzeUA("Mozilla/5.0 Chrome/40.0.2214.111 Safari/537.36")
	if result.Score < 20 {
		t.Errorf("expected score >= 20 for outdated Chrome 40, got %d", result.Score)
	}
}

func TestBotDetector_Analyze(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		BehaviorEnabled: false,
	})
	defer bd.Stop()

	// Scanner UA should be blocked
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "sqlmap/1.7")
	result := bd.Analyze(req)
	if !result.Blocked {
		t.Error("expected sqlmap to be blocked")
	}

	// Normal UA should pass
	req = httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36")
	result = bd.Analyze(req)
	if result.Blocked {
		t.Error("expected normal browser to pass")
	}
}

func TestBotDetector_EmptyUA(t *testing.T) {
	bd := New(Config{
		UAEnabled:       true,
		UABlockEmpty:    true,
		BehaviorEnabled: false,
	})
	defer bd.Stop()

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Del("User-Agent")
	result := bd.Analyze(req)
	if result.Score < 30 {
		t.Errorf("expected score >= 30 for empty UA, got %d", result.Score)
	}
}

func TestClassifyFingerprint_Known(t *testing.T) {
	// Known bad
	result := ClassifyFingerprint("36f7277af969a6947a61ae0b815907a1")
	if result.Category != FPBad {
		t.Errorf("expected FPBad for sqlmap fingerprint, got %s", result.Category)
	}

	// Unknown
	result = ClassifyFingerprint("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if result.Category != FPUnknown {
		t.Errorf("expected FPUnknown for random hash, got %s", result.Category)
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		ua      string
		prefix  string
		version int
	}{
		{"chrome/120.0.0.0", "chrome/", 120},
		{"firefox/121.0", "firefox/", 121},
		{"chrome/40.0", "chrome/", 40},
		{"noversion", "chrome/", 0},
	}

	for _, tt := range tests {
		got := extractVersion(tt.ua, tt.prefix)
		if got != tt.version {
			t.Errorf("extractVersion(%q, %q) = %d, want %d", tt.ua, tt.prefix, got, tt.version)
		}
	}
}
