// Package detection defines the scoring engine and detector interface for the WAF
// subsystem. Individual attack detectors (SQLi, XSS, command injection, etc.)
// implement the Detector interface and are orchestrated by the Engine to produce
// scored findings that drive block or log decisions.
package detection

// Detector is the interface for WAF attack detectors.
type Detector interface {
	// Name returns the detector name (e.g., "sqli", "xss").
	Name() string

	// Detect analyzes a request context and returns findings.
	Detect(ctx *RequestContext) []Finding
}
