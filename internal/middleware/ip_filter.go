// Package middleware provides HTTP middleware components for OpenLoadBalancer.
package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/openloadbalancer/olb/pkg/utils"
)

// IPFilterConfig configures the IP filter middleware.
type IPFilterConfig struct {
	// AllowList is a list of IPs and/or CIDR ranges to allow.
	// If non-empty, only these IPs are allowed (after deny check).
	AllowList []string

	// DenyList is a list of IPs and/or CIDR ranges to deny.
	// Checked before the allow list; deny always takes precedence.
	DenyList []string

	// DefaultAction is the action when an IP matches neither list.
	// Valid values: "allow" (default) or "deny".
	DefaultAction string

	// TrustXForwardedFor controls whether the X-Forwarded-For header
	// is used to extract the client IP. When false, only RemoteAddr is used.
	TrustXForwardedFor bool
}

// DefaultIPFilterConfig returns a sensible default IP filter configuration.
// Default action is "allow" with empty allow/deny lists (all traffic passes).
func DefaultIPFilterConfig() IPFilterConfig {
	return IPFilterConfig{
		AllowList:          nil,
		DenyList:           nil,
		DefaultAction:      "allow",
		TrustXForwardedFor: false,
	}
}

// IPFilterMiddleware filters requests based on client IP address.
// It supports individual IPs, CIDR ranges, and both IPv4 and IPv6.
// Processing order: DenyList first, then AllowList, then DefaultAction.
type IPFilterMiddleware struct {
	mu           sync.RWMutex
	config       IPFilterConfig
	allowMatcher *utils.CIDRMatcher
	denyMatcher  *utils.CIDRMatcher
	// allowIPs holds individual IPs (not CIDRs) from the allow list
	allowIPs map[string]bool
	// denyIPs holds individual IPs (not CIDRs) from the deny list
	denyIPs map[string]bool
	// hasAllow indicates if any allow rules are configured
	hasAllow bool
	// hasDeny indicates if any deny rules are configured
	hasDeny bool
}

// NewIPFilterMiddleware creates a new IP filter middleware.
// Returns an error if any of the IPs or CIDRs are invalid.
func NewIPFilterMiddleware(config IPFilterConfig) (*IPFilterMiddleware, error) {
	// Normalize default action
	if config.DefaultAction == "" {
		config.DefaultAction = "allow"
	}
	config.DefaultAction = strings.ToLower(config.DefaultAction)
	if config.DefaultAction != "allow" && config.DefaultAction != "deny" {
		return nil, fmt.Errorf("invalid default action %q: must be \"allow\" or \"deny\"", config.DefaultAction)
	}

	m := &IPFilterMiddleware{
		config: config,
	}

	if err := m.buildMatchers(config.AllowList, config.DenyList); err != nil {
		return nil, err
	}

	return m, nil
}

// buildMatchers builds the CIDR matchers and IP sets from the allow/deny lists.
func (m *IPFilterMiddleware) buildMatchers(allowList, denyList []string) error {
	allowMatcher := utils.NewCIDRMatcher()
	denyMatcher := utils.NewCIDRMatcher()
	allowIPs := make(map[string]bool)
	denyIPs := make(map[string]bool)

	// Parse allow list
	for _, entry := range allowList {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if isCIDR(entry) {
			if err := allowMatcher.Add(entry); err != nil {
				return fmt.Errorf("invalid allow CIDR %q: %w", entry, err)
			}
		} else {
			// Individual IP - validate it
			ip := net.ParseIP(entry)
			if ip == nil {
				return fmt.Errorf("invalid allow IP %q", entry)
			}
			allowIPs[ip.String()] = true
		}
	}

	// Parse deny list
	for _, entry := range denyList {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if isCIDR(entry) {
			if err := denyMatcher.Add(entry); err != nil {
				return fmt.Errorf("invalid deny CIDR %q: %w", entry, err)
			}
		} else {
			// Individual IP - validate it
			ip := net.ParseIP(entry)
			if ip == nil {
				return fmt.Errorf("invalid deny IP %q", entry)
			}
			denyIPs[ip.String()] = true
		}
	}

	m.allowMatcher = allowMatcher
	m.denyMatcher = denyMatcher
	m.allowIPs = allowIPs
	m.denyIPs = denyIPs
	m.hasAllow = len(allowList) > 0
	m.hasDeny = len(denyList) > 0

	return nil
}

// isCIDR checks if the string contains a CIDR notation (has a slash).
func isCIDR(s string) bool {
	return strings.Contains(s, "/")
}

// Name returns the middleware name.
func (m *IPFilterMiddleware) Name() string {
	return "ip-filter"
}

// Priority returns the middleware priority.
// IP filtering runs at security priority (early in the chain).
func (m *IPFilterMiddleware) Priority() int {
	return PrioritySecurity
}

// Wrap wraps the next handler with IP filtering.
func (m *IPFilterMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := m.extractClientIP(r)

		if !m.isAllowed(clientIP) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractClientIP extracts the client IP from the request.
// If TrustXForwardedFor is true, the leftmost IP from X-Forwarded-For is used,
// but only when the direct peer is a private/loopback address (trusted proxy).
// Otherwise, the RemoteAddr is used directly.
func (m *IPFilterMiddleware) extractClientIP(r *http.Request) string {
	m.mu.RLock()
	trustXFF := m.config.TrustXForwardedFor
	m.mu.RUnlock()

	if trustXFF {
		peerIP := utils.ExtractIP(r.RemoteAddr)
		parsedPeer := net.ParseIP(peerIP)
		// Only trust XFF from private/loopback addresses (trusted proxies)
		if parsedPeer != nil && (parsedPeer.IsPrivate() || parsedPeer.IsLoopback()) {
			xff := r.Header.Get("X-Forwarded-For")
			if xff != "" {
				// Use the leftmost (original client) IP
				parts := strings.SplitN(xff, ",", 2)
				ip := strings.TrimSpace(parts[0])
				if ip != "" {
					return ip
				}
			}
		}
	}

	return utils.ExtractIP(r.RemoteAddr)
}

// isAllowed checks if the given IP is allowed by the filter rules.
// Processing order: DenyList first, then AllowList, then DefaultAction.
func (m *IPFilterMiddleware) isAllowed(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Normalize the IP for consistent matching
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		// If we can't parse the IP, fall back to default action
		return m.config.DefaultAction == "allow"
	}
	normalizedIP := parsedIP.String()

	// Step 1: Check deny list first (deny takes precedence)
	if m.hasDeny {
		if m.denyIPs[normalizedIP] || m.denyMatcher.Contains(normalizedIP) {
			return false
		}
	}

	// Step 2: Check allow list
	if m.hasAllow {
		if m.allowIPs[normalizedIP] || m.allowMatcher.Contains(normalizedIP) {
			return true
		}
		// IP not in allow list - deny it (allow list acts as whitelist)
		return false
	}

	// Step 3: No allow list configured, use default action
	return m.config.DefaultAction == "allow"
}

// UpdateLists atomically updates the allow and deny lists at runtime.
// Returns an error if any of the new entries are invalid.
func (m *IPFilterMiddleware) UpdateLists(allowList, denyList []string) error {
	// Build new matchers before acquiring the write lock
	// to minimize lock hold time and validate before applying.
	newAllow := utils.NewCIDRMatcher()
	newDeny := utils.NewCIDRMatcher()
	newAllowIPs := make(map[string]bool)
	newDenyIPs := make(map[string]bool)

	for _, entry := range allowList {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if isCIDR(entry) {
			if err := newAllow.Add(entry); err != nil {
				return fmt.Errorf("invalid allow CIDR %q: %w", entry, err)
			}
		} else {
			ip := net.ParseIP(entry)
			if ip == nil {
				return fmt.Errorf("invalid allow IP %q", entry)
			}
			newAllowIPs[ip.String()] = true
		}
	}

	for _, entry := range denyList {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if isCIDR(entry) {
			if err := newDeny.Add(entry); err != nil {
				return fmt.Errorf("invalid deny CIDR %q: %w", entry, err)
			}
		} else {
			ip := net.ParseIP(entry)
			if ip == nil {
				return fmt.Errorf("invalid deny IP %q", entry)
			}
			newDenyIPs[ip.String()] = true
		}
	}

	// Atomically swap under write lock
	m.mu.Lock()
	m.allowMatcher = newAllow
	m.denyMatcher = newDeny
	m.allowIPs = newAllowIPs
	m.denyIPs = newDenyIPs
	m.hasAllow = len(allowList) > 0
	m.hasDeny = len(denyList) > 0
	m.config.AllowList = allowList
	m.config.DenyList = denyList
	m.mu.Unlock()

	return nil
}

// Config returns a copy of the current configuration.
func (m *IPFilterMiddleware) Config() IPFilterConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := m.config
	cfg.AllowList = make([]string, len(m.config.AllowList))
	copy(cfg.AllowList, m.config.AllowList)
	cfg.DenyList = make([]string, len(m.config.DenyList))
	copy(cfg.DenyList, m.config.DenyList)

	return cfg
}
