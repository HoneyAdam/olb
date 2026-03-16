// Package xss provides Cross-Site Scripting detection for the WAF.
package xss

import (
	"strings"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

// Detector implements XSS detection.
type Detector struct{}

// New creates a new XSS detector.
func New() *Detector { return &Detector{} }

// Name returns the detector name.
func (d *Detector) Name() string { return "xss" }

// Detect analyzes request fields for XSS patterns.
func (d *Detector) Detect(ctx *detection.RequestContext) []detection.Finding {
	var findings []detection.Finding

	for _, field := range ctx.AllInputs() {
		if field.Value == "" {
			continue
		}
		if f := d.analyze(field.Value, field.Location); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings
}

func (d *Detector) analyze(input, location string) *detection.Finding {
	lower := strings.ToLower(input)
	var maxScore int
	var maxRule string
	var maxEvidence string

	// 1. Check for HTML tag injection: <tagname
	if score, rule, evidence := d.checkTags(lower, input); score > maxScore {
		maxScore = score
		maxRule = rule
		maxEvidence = evidence
	}

	// 2. Check for dangerous protocols: javascript:, vbscript:, data:
	if score, rule, evidence := d.checkProtocols(lower); score > maxScore {
		maxScore = score
		maxRule = rule
		maxEvidence = evidence
	}

	// 3. Check for event handlers outside tags: onclick=, onerror=
	if score, rule, evidence := d.checkEventHandlers(lower); score > maxScore {
		maxScore = score
		maxRule = rule
		maxEvidence = evidence
	}

	// 4. Check for DOM manipulation patterns
	if score, rule, evidence := d.checkDOMPatterns(lower); score > maxScore {
		maxScore = score
		maxRule = rule
		maxEvidence = evidence
	}

	// 5. Check for expression() (IE CSS)
	if strings.Contains(lower, "expression(") || strings.Contains(lower, "expression (") {
		score := 45
		if score > maxScore {
			maxScore = score
			maxRule = "css_expression"
			maxEvidence = "expression()"
		}
	}

	if maxScore == 0 {
		return nil
	}

	return &detection.Finding{
		Detector: "xss",
		Score:    maxScore,
		Location: location,
		Evidence: truncate(maxEvidence, 80),
		Rule:     maxRule,
	}
}

// checkProtocols looks for dangerous URI protocols.
func (d *Detector) checkProtocols(lower string) (int, string, string) {
	for proto, score := range dangerousProtocols {
		pattern := proto + ":"
		if idx := strings.Index(lower, pattern); idx >= 0 {
			// data:text/html is more dangerous
			if proto == "data" && strings.Contains(lower[idx:], "text/html") {
				score = 75
			}
			return score, "protocol:" + proto, pattern
		}
	}
	return 0, "", ""
}

// checkEventHandlers looks for standalone event handler attributes.
func (d *Detector) checkEventHandlers(lower string) (int, string, string) {
	for handler := range eventHandlers {
		pattern := handler + "="
		if idx := strings.Index(lower, pattern); idx >= 0 {
			return 70, "event_handler:" + handler, handler + "=..."
		}
		// Also check with space before =
		pattern = handler + " ="
		if idx := strings.Index(lower, pattern); idx >= 0 {
			return 70, "event_handler:" + handler, handler + " =..."
		}
	}
	return 0, "", ""
}

// checkDOMPatterns looks for JavaScript DOM manipulation patterns.
func (d *Detector) checkDOMPatterns(lower string) (int, string, string) {
	for _, dp := range domPatterns {
		if strings.Contains(lower, dp.pattern) {
			return dp.score, "dom:" + dp.pattern, dp.pattern
		}
	}
	return 0, "", ""
}

func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}
