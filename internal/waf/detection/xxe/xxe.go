// Package xxe provides XML External Entity detection for the WAF.
package xxe

import (
	"strings"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

// Detector implements XXE detection.
type Detector struct{}

// New creates a new XXE detector.
func New() *Detector { return &Detector{} }

// Name returns the detector name.
func (d *Detector) Name() string { return "xxe" }

// Detect analyzes request for XXE patterns.
// Only active when Content-Type contains "xml".
func (d *Detector) Detect(ctx *detection.RequestContext) []detection.Finding {
	// Only check XML content
	ct := strings.ToLower(ctx.ContentType)
	if !strings.Contains(ct, "xml") {
		return nil
	}

	if len(ctx.Body) == 0 {
		return nil
	}

	return d.analyzeXML(string(ctx.Body))
}

func (d *Detector) analyzeXML(body string) []detection.Finding {
	lower := strings.ToLower(body)
	var findings []detection.Finding
	var maxScore int
	var maxRule, maxEvidence string

	// <!DOCTYPE
	if strings.Contains(lower, "<!doctype") {
		maxScore = 30
		maxRule = "doctype"
		maxEvidence = "<!DOCTYPE"
	}

	// <!ENTITY
	if strings.Contains(lower, "<!entity") {
		score := 70
		if score > maxScore {
			maxScore = score
			maxRule = "entity_declaration"
			maxEvidence = "<!ENTITY"
		}
	}

	// Parameter entity: <!ENTITY %
	if strings.Contains(lower, "<!entity %") || strings.Contains(lower, "<!entity%") {
		score := 85
		if score > maxScore {
			maxScore = score
			maxRule = "parameter_entity"
			maxEvidence = "<!ENTITY %"
		}
	}

	// SYSTEM with file://
	if strings.Contains(lower, "system") {
		if strings.Contains(lower, "file://") || strings.Contains(lower, "file:") {
			score := 95
			if score > maxScore {
				maxScore = score
				maxRule = "system_file"
				maxEvidence = "SYSTEM file://"
			}
		}
		// SYSTEM with http:// (SSRF via XXE)
		if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
			score := 80
			if score > maxScore {
				maxScore = score
				maxRule = "system_http"
				maxEvidence = "SYSTEM http://"
			}
		}
		// SYSTEM with expect://
		if strings.Contains(lower, "expect://") {
			score := 95
			if score > maxScore {
				maxScore = score
				maxRule = "system_expect"
				maxEvidence = "SYSTEM expect://"
			}
		}
	}

	// SSI injection: <!--#include
	if strings.Contains(lower, "<!--#include") || strings.Contains(lower, "<!--#exec") {
		score := 70
		if score > maxScore {
			maxScore = score
			maxRule = "ssi_injection"
			maxEvidence = "<!--#include"
		}
	}

	if maxScore > 0 {
		findings = append(findings, detection.Finding{
			Detector: "xxe",
			Score:    maxScore,
			Location: "body",
			Evidence: maxEvidence,
			Rule:     maxRule,
		})
	}

	return findings
}
