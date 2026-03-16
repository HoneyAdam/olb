package detection

// Detector is the interface for WAF attack detectors.
type Detector interface {
	// Name returns the detector name (e.g., "sqli", "xss").
	Name() string

	// Detect analyzes a request context and returns findings.
	Detect(ctx *RequestContext) []Finding
}
