package botdetect

import (
	"testing"
)

func TestCheckUAFPMismatch_UnknownFP(t *testing.T) {
	score := CheckUAFPMismatch("Mozilla/5.0 Chrome/120.0", FPResult{Category: FPUnknown})
	if score != 0 {
		t.Errorf("expected 0 for unknown FP category, got %d", score)
	}
}

func TestCheckUAFPMismatch_BrowserMismatch(t *testing.T) {
	// UA says Chrome but FP says Firefox
	score := CheckUAFPMismatch(
		"Mozilla/5.0 Chrome/120.0",
		FPResult{Category: FPGood, Name: "Firefox 121"},
	)
	if score != 60 {
		t.Errorf("expected 60 for browser mismatch, got %d", score)
	}
}

func TestCheckUAFPMismatch_BrowserWithBadFP(t *testing.T) {
	// UA says browser but FP is bad (known attack tool)
	score := CheckUAFPMismatch(
		"Mozilla/5.0 Chrome/120.0",
		FPResult{Category: FPBad, Name: "sqlmap"},
	)
	if score != 70 {
		t.Errorf("expected 70 for browser UA with bad FP, got %d", score)
	}
}

func TestCheckUAFPMismatch_NoBrowserInUA(t *testing.T) {
	// UA doesn't identify a browser (e.g., curl)
	score := CheckUAFPMismatch(
		"curl/7.64.1",
		FPResult{Category: FPBad, Name: "curl"},
	)
	if score != 0 {
		t.Errorf("expected 0 when UA doesn't claim a browser, got %d", score)
	}
}

func TestCheckUAFPMismatch_Matching(t *testing.T) {
	// UA says Chrome and FP says Chrome — no mismatch
	score := CheckUAFPMismatch(
		"Mozilla/5.0 Chrome/120.0",
		FPResult{Category: FPGood, Name: "Chrome 120"},
	)
	if score != 0 {
		t.Errorf("expected 0 for matching browser, got %d", score)
	}
}

func TestCheckUAFPMismatch_SafariEdgeCases(t *testing.T) {
	// Safari UA with Chrome FP (Safari check has "not chrome" condition)
	score := CheckUAFPMismatch(
		"Mozilla/5.0 (Macintosh) AppleWebKit/605.1.15 Safari/537.36",
		FPResult{Category: FPGood, Name: "Firefox 121"},
	)
	if score != 60 {
		t.Errorf("expected 60 for safari/firefox mismatch, got %d", score)
	}
}

func TestCheckUAFPMismatch_EdgeBrowser(t *testing.T) {
	// Edge UA with Firefox FP
	score := CheckUAFPMismatch(
		"Mozilla/5.0 Edge/120.0",
		FPResult{Category: FPGood, Name: "Firefox 121"},
	)
	if score != 60 {
		t.Errorf("expected 60 for edge/firefox mismatch, got %d", score)
	}
}

func TestIdentifyBrowser(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"chrome/120", "chrome"},
		{"firefox/121", "firefox"},
		{"safari/537", "safari"},    // no "chrome" present
		{"chrome safari", "chrome"}, // chrome takes precedence
		{"edge/120", "edge"},
		{"curl/7.0", ""},
		{"python-requests/2.28", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := identifyBrowser(tt.input)
		if got != tt.expected {
			t.Errorf("identifyBrowser(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAnalyzeUA_NoVersion(t *testing.T) {
	result := AnalyzeUA("CustomBot")
	if result.Rule != "no_version" {
		t.Errorf("expected 'no_version' rule for UA without digits, got %q", result.Rule)
	}
	if result.Score < 20 {
		t.Errorf("expected score >= 20 for no version, got %d", result.Score)
	}
}

func TestAnalyzeUA_ShortUA(t *testing.T) {
	result := AnalyzeUA("Bot v1.0")
	if result.Rule != "short_ua" {
		t.Errorf("expected 'short_ua' rule for short UA, got %q", result.Rule)
	}
	if result.Score < 20 {
		t.Errorf("expected score >= 20 for short UA, got %d", result.Score)
	}
}

func TestAnalyzeUA_OutdatedFirefox(t *testing.T) {
	result := AnalyzeUA("Mozilla/5.0 (Windows NT 10.0; rv:50.0) Gecko/20100101 Firefox/50.0")
	if result.Rule != "outdated_browser" {
		t.Errorf("expected 'outdated_browser' rule for Firefox/50, got %q", result.Rule)
	}
}

func TestAnalyzeUA_AllKnownScanners(t *testing.T) {
	for _, scanner := range knownScanners {
		result := AnalyzeUA("Mozilla/5.0 " + scanner + "/1.0")
		if result.Rule != "known_scanner" {
			t.Errorf("expected known_scanner for %q, got %q", scanner, result.Rule)
		}
		if result.Score < 80 {
			t.Errorf("expected score >= 80 for scanner %q, got %d", scanner, result.Score)
		}
	}
}

func TestExtractVersion_EdgeCases(t *testing.T) {
	tests := []struct {
		ua      string
		prefix  string
		version int
	}{
		{"chrome/", "chrome/", 0},      // empty version
		{"chrome/abc", "chrome/", 0},   // non-numeric
		{"chrome/999", "chrome/", 999}, // large version
		{"nothing", "chrome/", 0},      // prefix not found
		{"firefox/0", "firefox/", 0},   // zero version
	}

	for _, tt := range tests {
		got := extractVersion(tt.ua, tt.prefix)
		if got != tt.version {
			t.Errorf("extractVersion(%q, %q) = %d, want %d", tt.ua, tt.prefix, got, tt.version)
		}
	}
}
