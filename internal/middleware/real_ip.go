// Package middleware provides HTTP middleware components for OpenLoadBalancer.
package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/openloadbalancer/olb/pkg/utils"
)

// RealIPConfig configures the RealIP middleware.
type RealIPConfig struct {
	TrustedProxies []string // CIDR ranges of trusted proxies
	Header         string   // default: X-Forwarded-For
	RealIPHeader   string   // default: X-Real-Ip
}

// RealIPMiddleware extracts the real client IP from proxy headers.
type RealIPMiddleware struct {
	config  RealIPConfig
	trusted *utils.CIDRMatcher
}

// NewRealIPMiddleware creates a new RealIP middleware.
// Returns an error if any of the trusted proxy CIDRs are invalid.
func NewRealIPMiddleware(config RealIPConfig) (*RealIPMiddleware, error) {
	// Apply defaults
	if config.Header == "" {
		config.Header = "X-Forwarded-For"
	}
	if config.RealIPHeader == "" {
		config.RealIPHeader = "X-Real-Ip"
	}

	// Parse trusted proxies
	matcher := utils.NewCIDRMatcher()
	for _, cidr := range config.TrustedProxies {
		if err := matcher.Add(cidr); err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", cidr, err)
		}
	}

	return &RealIPMiddleware{
		config:  config,
		trusted: matcher,
	}, nil
}

// Name returns the middleware name.
func (m *RealIPMiddleware) Name() string {
	return "real-ip"
}

// Priority returns the middleware priority (higher runs first).
func (m *RealIPMiddleware) Priority() int {
	return 300
}

// Wrap wraps the next handler with real IP extraction.
func (m *RealIPMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := m.extractClientIP(r)

		// Store the client IP in the request context for downstream use
		ctx := WithClientIP(r.Context(), clientIP)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// extractClientIP extracts the real client IP from the request.
// If the remote address is from a trusted proxy, it extracts the IP from
// X-Forwarded-For (rightmost non-trusted) or falls back to X-Real-Ip.
// If not trusted, it uses the RemoteAddr directly.
func (m *RealIPMiddleware) extractClientIP(r *http.Request) string {
	// Get the direct remote address
	remoteIP := utils.ExtractIP(r.RemoteAddr)

	// If no trusted proxies configured or remote is not trusted, use remote addr
	if m.trusted.Len() == 0 || !m.trusted.Contains(remoteIP) {
		return remoteIP
	}

	// Remote is trusted, try to extract real client IP from headers

	// First, check X-Forwarded-For
	forwardedFor := r.Header.Get(m.config.Header)
	if forwardedFor != "" {
		// Parse the X-Forwarded-For chain
		// Format: client, proxy1, proxy2, ..., proxyN
		// We want the rightmost non-trusted IP (closest to us that we trust)
		ips := parseXForwardedFor(forwardedFor)

		// Walk from right to left to find the first trusted proxy,
		// then return the one to its left (the last non-trusted)
		for i := len(ips) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(ips[i])
			if !m.trusted.Contains(ip) {
				// This is the rightmost non-trusted IP
				return ip
			}
		}

		// All IPs in the chain are trusted, return the leftmost (original client)
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to X-Real-Ip
	realIP := r.Header.Get(m.config.RealIPHeader)
	if realIP != "" {
		// X-Real-Ip should contain a single IP
		return strings.TrimSpace(realIP)
	}

	// No headers found, use remote address
	return remoteIP
}

// parseXForwardedFor parses the X-Forwarded-For header value into a slice of IPs.
func parseXForwardedFor(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		ip := strings.TrimSpace(part)
		if ip != "" {
			result = append(result, ip)
		}
	}

	return result
}

// isValidIP checks if the string is a valid IP address.
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
