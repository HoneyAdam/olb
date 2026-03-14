// Package middleware provides HTTP middleware components for OpenLoadBalancer.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// RequestIDConfig configures the RequestID middleware.
type RequestIDConfig struct {
	HeaderName    string // default: X-Request-Id
	Generate      bool   // generate if not present (default: true)
	TrustIncoming bool   // trust existing header (default: false)
}

// RequestIDMiddleware adds or generates request IDs for tracing.
type RequestIDMiddleware struct {
	config RequestIDConfig
}

// NewRequestIDMiddleware creates a new RequestID middleware.
func NewRequestIDMiddleware(config RequestIDConfig) *RequestIDMiddleware {
	// Apply defaults
	if config.HeaderName == "" {
		config.HeaderName = "X-Request-Id"
	}

	return &RequestIDMiddleware{
		config: config,
	}
}

// Name returns the middleware name.
func (m *RequestIDMiddleware) Name() string {
	return "request-id"
}

// Priority returns the middleware priority (higher runs first).
func (m *RequestIDMiddleware) Priority() int {
	return 400
}

// Wrap wraps the next handler with request ID functionality.
func (m *RequestIDMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := ""

		// Check for existing header
		if m.config.TrustIncoming {
			requestID = r.Header.Get(m.config.HeaderName)
		}

		// Generate new ID if needed
		if requestID == "" && m.config.Generate {
			requestID = generateUUID()
		}

		// Set the request ID on the response header
		if requestID != "" {
			w.Header().Set(m.config.HeaderName, requestID)
		}

		next.ServeHTTP(w, r)
	})
}

// generateUUID generates a UUID v4 using crypto/rand.
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx where y is 8, 9, a, or b.
func generateUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to zeros (should never happen with crypto/rand)
		return "00000000-0000-4000-8000-000000000000"
	}

	// Set version (4) and variant (10xxxxxx = 0x80-0xBF)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10

	// Convert to string format
	var sb strings.Builder
	sb.Grow(36)

	hex := hex.EncodeToString(b[:])
	sb.WriteString(hex[0:8])
	sb.WriteByte('-')
	sb.WriteString(hex[8:12])
	sb.WriteByte('-')
	sb.WriteString(hex[12:16])
	sb.WriteByte('-')
	sb.WriteString(hex[16:20])
	sb.WriteByte('-')
	sb.WriteString(hex[20:32])

	return sb.String()
}
