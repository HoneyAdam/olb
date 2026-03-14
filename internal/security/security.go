// Package security provides security hardening utilities for OpenLoadBalancer.
//
// It includes secure TLS defaults, slow loris protection, request smuggling
// detection, header injection prevention, input validation, and privilege
// dropping on supported platforms.
package security

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// --------------------------------------------------------------------------
// Secure TLS defaults
// --------------------------------------------------------------------------

// DefaultTLSConfig returns a hardened tls.Config with secure defaults.
//
// Highlights:
//   - MinVersion: TLS 1.2 (TLS 1.0/1.1 are deprecated per RFC 8996)
//   - Only strong cipher suites: ECDHE + AESGCM / CHACHA20-POLY1305
//   - Curve preferences: X25519, P-256, P-384
//   - Server selects the cipher suite
func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,

		CipherSuites: []uint16{
			// TLS 1.2 cipher suites ordered by preference.
			// TLS 1.3 suites are always enabled by Go and cannot be configured.
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},

		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},

		PreferServerCipherSuites: true,
	}
}

// StrongCipherSuiteNames returns the list of cipher suite names that
// DefaultTLSConfig enables. Useful for documentation and logging.
func StrongCipherSuiteNames() []string {
	return []string{
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
	}
}

// --------------------------------------------------------------------------
// Slow Loris protection
// --------------------------------------------------------------------------

// SlowLorisProtection holds configuration to defend against slow loris and
// similar slow-rate HTTP attacks.
type SlowLorisProtection struct {
	// MaxHeaderBytes limits the size of request headers (default 1 MB).
	MaxHeaderBytes int

	// ReadHeaderTimeout is the maximum duration allowed for reading request
	// headers (default 10 s).
	ReadHeaderTimeout time.Duration

	// ReadTimeout is the maximum duration for reading the entire request
	// including body (default 30 s).
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the
	// response (default 30 s).
	WriteTimeout time.Duration

	// IdleTimeout is the maximum time to wait for the next request when
	// keep-alives are enabled (default 120 s).
	IdleTimeout time.Duration
}

// DefaultSlowLorisProtection returns sensible defaults.
func DefaultSlowLorisProtection() *SlowLorisProtection {
	return &SlowLorisProtection{
		MaxHeaderBytes:    1 << 20, // 1 MB
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// ApplyToServer applies the protection settings to an http.Server.
func (s *SlowLorisProtection) ApplyToServer(srv *http.Server) {
	if s.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = s.MaxHeaderBytes
	}
	if s.ReadHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = s.ReadHeaderTimeout
	}
	if s.ReadTimeout > 0 {
		srv.ReadTimeout = s.ReadTimeout
	}
	if s.WriteTimeout > 0 {
		srv.WriteTimeout = s.WriteTimeout
	}
	if s.IdleTimeout > 0 {
		srv.IdleTimeout = s.IdleTimeout
	}
}

// --------------------------------------------------------------------------
// Request smuggling protection
// --------------------------------------------------------------------------

// Request smuggling errors.
var (
	ErrConflictingHeaders   = errors.New("security: conflicting Content-Length and Transfer-Encoding headers")
	ErrDuplicateContentLen  = errors.New("security: duplicate Content-Length headers with different values")
	ErrMalformedContentLen  = errors.New("security: malformed Content-Length value")
	ErrNegativeContentLen   = errors.New("security: negative Content-Length value")
	ErrHTTP10TransferEnc    = errors.New("security: Transfer-Encoding is not allowed in HTTP/1.0")
	ErrChunkedNotFinal      = errors.New("security: chunked is not the final Transfer-Encoding")
	ErrInvalidTransferEnc   = errors.New("security: invalid Transfer-Encoding value")
)

// ValidateRequest inspects an HTTP request for indicators of request
// smuggling attacks and returns a descriptive error when one is detected.
//
// Checks performed:
//  1. Conflicting Content-Length and Transfer-Encoding headers
//  2. Duplicate Content-Length headers with differing values
//  3. Malformed or negative Content-Length values
//  4. Transfer-Encoding in HTTP/1.0 requests
//  5. Chunked must be the final Transfer-Encoding
//  6. Invalid Transfer-Encoding values
func ValidateRequest(r *http.Request) error {
	// Collect raw Content-Length and Transfer-Encoding values.
	clValues := r.Header.Values("Content-Length")
	teValues := r.Header.Values("Transfer-Encoding")

	// --- Check 1: Conflicting CL + TE ---
	if len(clValues) > 0 && len(teValues) > 0 {
		return ErrConflictingHeaders
	}

	// --- Check 2 & 3: Content-Length validation ---
	if len(clValues) > 0 {
		// Validate each CL value individually and check for duplicates.
		var firstVal int64 = -1
		for _, raw := range clValues {
			// A single header value may contain a comma-separated list in
			// malicious requests (e.g., "Content-Length: 1, 2").
			parts := strings.Split(raw, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				n, err := strconv.ParseInt(part, 10, 64)
				if err != nil {
					return ErrMalformedContentLen
				}
				if n < 0 {
					return ErrNegativeContentLen
				}
				if firstVal == -1 {
					firstVal = n
				} else if n != firstVal {
					return ErrDuplicateContentLen
				}
			}
		}
	}

	// --- Check 4: HTTP/1.0 must not use Transfer-Encoding ---
	if len(teValues) > 0 && !r.ProtoAtLeast(1, 1) {
		return ErrHTTP10TransferEnc
	}

	// --- Check 5 & 6: Transfer-Encoding validation ---
	if len(teValues) > 0 {
		// Flatten all TE values.
		var encodings []string
		for _, te := range teValues {
			for _, part := range strings.Split(te, ",") {
				enc := strings.TrimSpace(strings.ToLower(part))
				if enc == "" {
					continue
				}
				encodings = append(encodings, enc)
			}
		}

		allowedEncodings := map[string]bool{
			"chunked":  true,
			"identity": true,
			"gzip":     true,
			"compress": true,
			"deflate":  true,
		}

		for _, enc := range encodings {
			if !allowedEncodings[enc] {
				return ErrInvalidTransferEnc
			}
		}

		// If "chunked" appears it must be the last encoding.
		for i, enc := range encodings {
			if enc == "chunked" && i != len(encodings)-1 {
				return ErrChunkedNotFinal
			}
		}
	}

	return nil
}

// --------------------------------------------------------------------------
// Header injection protection
// --------------------------------------------------------------------------

// SanitizeHeaderValue removes CR (\r), LF (\n), and NUL (\x00) characters
// from a header value to prevent header injection / response splitting.
func SanitizeHeaderValue(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c != '\r' && c != '\n' && c != 0x00 {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ValidateHeaderName checks whether name is a valid HTTP header field name
// per RFC 7230 section 3.2.6 (token characters only).
//
//	token = 1*tchar
//	tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//	        "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
func ValidateHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !isTokenChar(c) {
			return false
		}
	}
	return true
}

// isTokenChar returns whether c is a valid token character per RFC 7230.
func isTokenChar(c byte) bool {
	// ALPHA
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		return true
	}
	// DIGIT
	if c >= '0' && c <= '9' {
		return true
	}
	// Special token chars
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

// --------------------------------------------------------------------------
// Input validation
// --------------------------------------------------------------------------

// ValidateHostHeader validates a Host header value.
//
// The function ensures the host:
//   - Is not empty
//   - Contains no whitespace, CR, LF, or NUL characters
//   - Does not contain user-info ("@")
//   - When a port is present, it is a valid number (1-65535)
//   - The hostname portion contains only allowed characters
func ValidateHostHeader(host string) bool {
	if host == "" {
		return false
	}

	// Reject control characters, whitespace, and NUL.
	for i := 0; i < len(host); i++ {
		c := host[i]
		if c <= ' ' || c == 0x7f {
			return false
		}
		if c == '\r' || c == '\n' || c == 0x00 {
			return false
		}
	}

	// Reject user-info component.
	if strings.Contains(host, "@") {
		return false
	}

	// Reject backslashes (path traversal in host).
	if strings.Contains(host, `\`) {
		return false
	}

	// Split host and port.
	hostname, portStr, err := net.SplitHostPort(host)
	if err != nil {
		// No port – treat entire value as hostname.
		hostname = host
		portStr = ""
	}

	// Validate port if present.
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}

	// Validate hostname characters.
	if hostname == "" {
		return false
	}

	// Allow IPv6 addresses wrapped in brackets.
	if strings.HasPrefix(hostname, "[") && strings.HasSuffix(hostname, "]") {
		ip := net.ParseIP(hostname[1 : len(hostname)-1])
		return ip != nil
	}

	// Check for valid hostname characters: a-z, A-Z, 0-9, '-', '.', and
	// allow IPv4 dotted addresses.
	for i := 0; i < len(hostname); i++ {
		c := hostname[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '.') {
			return false
		}
	}

	// Reject leading/trailing dots or hyphens in hostname.
	if hostname[0] == '.' || hostname[0] == '-' ||
		hostname[len(hostname)-1] == '.' || hostname[len(hostname)-1] == '-' {
		return false
	}

	return true
}

// ValidateContentLength validates a Content-Length header value and returns
// the parsed length. It rejects non-numeric, negative, and overflow values.
func ValidateContentLength(cl string) (int64, error) {
	cl = strings.TrimSpace(cl)
	if cl == "" {
		return 0, fmt.Errorf("empty Content-Length value")
	}

	// Reject leading zeros (except "0" itself) to prevent ambiguous parsing.
	if len(cl) > 1 && cl[0] == '0' {
		return 0, fmt.Errorf("Content-Length has leading zeros: %q", cl)
	}

	// Reject any non-digit character.
	for i := 0; i < len(cl); i++ {
		if cl[i] < '0' || cl[i] > '9' {
			return 0, fmt.Errorf("Content-Length contains non-digit character: %q", cl)
		}
	}

	n, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Content-Length %q: %w", cl, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("negative Content-Length: %d", n)
	}

	return n, nil
}

// SanitizePath cleans a URL path to prevent directory traversal attacks.
//
// It removes ".." segments, normalises slashes, and ensures the result always
// starts with "/".
func SanitizePath(p string) string {
	if p == "" {
		return "/"
	}

	// Decode percent-encoded dots that could bypass traversal checks.
	cleaned := decodeTraversalSequences(p)

	// Use path.Clean which resolves ".." and "." segments.
	cleaned = path.Clean(cleaned)

	// Ensure leading slash.
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	return cleaned
}

// decodeTraversalSequences decodes common percent-encoded sequences used to
// bypass path traversal filters and normalises backslashes to forward slashes.
func decodeTraversalSequences(s string) string {
	// Replace common encodings of ".." and "/" used in traversal attacks.
	replacer := strings.NewReplacer(
		"%2e", ".",
		"%2E", ".",
		"%2f", "/",
		"%2F", "/",
		"%5c", "/",
		"%5C", "/",
	)
	s = replacer.Replace(s)

	// Normalise backslashes to forward slashes (Windows-style traversal).
	s = strings.ReplaceAll(s, `\`, "/")

	return s
}

// --------------------------------------------------------------------------
// Privilege dropping (platform-specific, see security_priv_*.go)
// --------------------------------------------------------------------------

// PrivilegeDropper can drop OS-level privileges after binding to privileged
// ports. The actual implementation is platform-specific.
type PrivilegeDropper struct{}

// NewPrivilegeDropper returns a new PrivilegeDropper.
func NewPrivilegeDropper() *PrivilegeDropper {
	return &PrivilegeDropper{}
}
