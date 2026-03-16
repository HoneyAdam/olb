// Package response provides response protection for the WAF including
// security headers, data masking, and error page standardization.
package response

import (
	"fmt"
	"net/http"
	"strings"
)

// HeadersConfig configures security header injection.
type HeadersConfig struct {
	HSTSEnabled           bool
	HSTSMaxAge            int
	HSTSIncludeSubdomains bool
	HSTSPreload           bool
	XContentTypeOptions   bool
	XFrameOptions         string
	ReferrerPolicy        string
	PermissionsPolicy     string
	CSP                   string
}

// DefaultHeadersConfig returns default security headers configuration.
func DefaultHeadersConfig() HeadersConfig {
	return HeadersConfig{
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		XContentTypeOptions:   true,
		XFrameOptions:         "SAMEORIGIN",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "camera=(), microphone=(), geolocation=()",
	}
}

// InjectHeaders adds security headers to the response.
func InjectHeaders(w http.ResponseWriter, cfg HeadersConfig) {
	h := w.Header()

	// HSTS
	if cfg.HSTSEnabled && cfg.HSTSMaxAge > 0 {
		value := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
		if cfg.HSTSIncludeSubdomains {
			value += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			value += "; preload"
		}
		h.Set("Strict-Transport-Security", value)
	}

	// X-Content-Type-Options
	if cfg.XContentTypeOptions {
		h.Set("X-Content-Type-Options", "nosniff")
	}

	// X-Frame-Options
	if cfg.XFrameOptions != "" {
		h.Set("X-Frame-Options", cfg.XFrameOptions)
	}

	// Referrer-Policy
	if cfg.ReferrerPolicy != "" {
		h.Set("Referrer-Policy", cfg.ReferrerPolicy)
	}

	// Permissions-Policy
	if cfg.PermissionsPolicy != "" {
		h.Set("Permissions-Policy", cfg.PermissionsPolicy)
	}

	// Content-Security-Policy
	if cfg.CSP != "" {
		h.Set("Content-Security-Policy", cfg.CSP)
	}

	// X-XSS-Protection — disable browser XSS filter (CSP is preferred)
	h.Set("X-XSS-Protection", "0")
}

// IsTextContent checks if a content type is text-based (suitable for body scanning).
func IsTextContent(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") ||
		strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "text/plain") ||
		strings.Contains(ct, "text/xml") ||
		strings.Contains(ct, "application/xml")
}
