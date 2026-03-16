// Package sanitizer provides request validation and normalization for the WAF.
package sanitizer

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// Config holds sanitizer configuration.
type Config struct {
	MaxHeaderSize     int
	MaxHeaderCount    int
	MaxBodySize       int64
	MaxURLLength      int
	MaxCookieSize     int
	MaxCookieCount    int
	BlockNullBytes    bool
	NormalizeEncoding bool
	StripHopByHop     bool
	AllowedMethods    []string
	PathOverrides     []PathOverride
}

// PathOverride allows per-path config overrides.
type PathOverride struct {
	Pattern     string
	MaxBodySize int64
}

// DefaultConfig returns a default sanitizer configuration.
func DefaultConfig() Config {
	return Config{
		MaxHeaderSize:     8192,
		MaxHeaderCount:    100,
		MaxBodySize:       10 * 1024 * 1024, // 10MB
		MaxURLLength:      8192,
		MaxCookieSize:     4096,
		MaxCookieCount:    50,
		BlockNullBytes:    true,
		NormalizeEncoding: true,
		StripHopByHop:     true,
		AllowedMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
	}
}

// Result holds the sanitized and normalized request data.
type Result struct {
	Body         []byte
	DecodedPath  string
	DecodedQuery string
	DecodedBody  string
}

// Sanitizer validates and normalizes HTTP requests.
type Sanitizer struct {
	config         Config
	allowedMethods map[string]bool
}

// New creates a new Sanitizer.
func New(cfg Config) *Sanitizer {
	methods := make(map[string]bool, len(cfg.AllowedMethods))
	for _, m := range cfg.AllowedMethods {
		methods[strings.ToUpper(m)] = true
	}
	return &Sanitizer{
		config:         cfg,
		allowedMethods: methods,
	}
}

// Process validates the request and returns normalized data.
// Returns a ValidationError if the request fails validation.
func (s *Sanitizer) Process(r *http.Request) (*Result, *ValidationError) {
	// 1. Validate method
	if len(s.allowedMethods) > 0 && !s.allowedMethods[r.Method] {
		return nil, &ValidationError{Status: 405, Message: "method not allowed"}
	}

	// 2. Validate URL length
	if s.config.MaxURLLength > 0 && len(r.URL.String()) > s.config.MaxURLLength {
		return nil, &ValidationError{Status: 414, Message: "URI too long"}
	}

	// 3. Validate header count
	if s.config.MaxHeaderCount > 0 && len(r.Header) > s.config.MaxHeaderCount {
		return nil, &ValidationError{Status: 431, Message: "too many headers"}
	}

	// 4. Validate header sizes
	if s.config.MaxHeaderSize > 0 {
		for name, values := range r.Header {
			for _, v := range values {
				if len(name)+len(v) > s.config.MaxHeaderSize {
					return nil, &ValidationError{Status: 431, Message: "header too large: " + name}
				}
			}
		}
	}

	// 5. Validate cookies
	if s.config.MaxCookieCount > 0 {
		cookies := r.Cookies()
		if len(cookies) > s.config.MaxCookieCount {
			return nil, &ValidationError{Status: 400, Message: "too many cookies"}
		}
		if s.config.MaxCookieSize > 0 {
			for _, c := range cookies {
				if len(c.Value) > s.config.MaxCookieSize {
					return nil, &ValidationError{Status: 400, Message: "cookie too large: " + c.Name}
				}
			}
		}
	}

	// 6. Read and validate body size
	maxBody := s.config.MaxBodySize
	for _, override := range s.config.PathOverrides {
		if matchPath(r.URL.Path, override.Pattern) {
			maxBody = override.MaxBodySize
			break
		}
	}

	var body []byte
	if r.Body != nil && r.ContentLength != 0 {
		if r.ContentLength > 0 && maxBody > 0 && r.ContentLength > maxBody {
			return nil, &ValidationError{Status: 413, Message: "payload too large"}
		}
		var err error
		body, err = io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil {
			return nil, &ValidationError{Status: 400, Message: "failed to read body"}
		}
		if int64(len(body)) > maxBody && maxBody > 0 {
			return nil, &ValidationError{Status: 413, Message: "payload too large"}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	// 7. Check for null bytes
	if s.config.BlockNullBytes {
		if containsNullByte(r.URL.RawQuery) || containsNullByte(r.URL.Path) {
			return nil, &ValidationError{Status: 400, Message: "null byte in request"}
		}
		for _, values := range r.Header {
			for _, v := range values {
				if containsNullByte(v) {
					return nil, &ValidationError{Status: 400, Message: "null byte in header"}
				}
			}
		}
	}

	// 8. Strip hop-by-hop headers
	if s.config.StripHopByHop {
		stripHopByHop(r)
	}

	// 9. Build result with normalization
	result := &Result{Body: body}

	if s.config.NormalizeEncoding {
		result.DecodedPath = NormalizePath(r.URL.Path)
		result.DecodedQuery = DecodeMultiLevel(r.URL.RawQuery)
		if len(body) > 0 {
			result.DecodedBody = DecodeMultiLevel(string(body))
		}
	}

	return result, nil
}

// ValidationError represents a request validation failure.
type ValidationError struct {
	Status  int
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func containsNullByte(s string) bool {
	return strings.Contains(s, "\x00") || strings.Contains(s, "%00")
}

// hopByHopHeaders are headers that should be removed by proxies.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func stripHopByHop(r *http.Request) {
	for _, h := range hopByHopHeaders {
		r.Header.Del(h)
	}
}

// matchPath checks if a path matches a glob pattern (simple * matching).
func matchPath(path, pattern string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return path == pattern
	}
	prefix := strings.TrimSuffix(pattern, "*")
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}
