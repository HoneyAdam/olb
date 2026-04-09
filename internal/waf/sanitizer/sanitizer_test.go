package sanitizer

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizer_ValidRequest(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/path?key=value", nil)

	result, err := s.Process(req)
	if err != nil {
		t.Fatalf("expected no error for valid request, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSanitizer_InvalidMethod(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("HACK", "http://example.com/", nil)

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
	if err.Status != 405 {
		t.Errorf("expected status 405, got %d", err.Status)
	}
}

func TestSanitizer_URLTooLong(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxURLLength = 50
	s := New(cfg)

	longURL := "http://example.com/" + strings.Repeat("a", 100)
	req := httptest.NewRequest("GET", longURL, nil)

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for URL too long")
	}
	if err.Status != 414 {
		t.Errorf("expected status 414, got %d", err.Status)
	}
}

func TestSanitizer_TooManyHeaders(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxHeaderCount = 5
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	for i := 0; i < 10; i++ {
		req.Header.Set("X-Custom-"+strings.Repeat("a", i+1), "value")
	}

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for too many headers")
	}
	if err.Status != 431 {
		t.Errorf("expected status 431, got %d", err.Status)
	}
}

func TestSanitizer_NullByteBlocked(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/path%00evil", nil)

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for null byte in URL")
	}
	if err.Status != 400 {
		t.Errorf("expected status 400, got %d", err.Status)
	}
}

func TestSanitizer_BodyTooLarge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBodySize = 10
	s := New(cfg)

	body := strings.NewReader(strings.Repeat("x", 20))
	req := httptest.NewRequest("POST", "http://example.com/", body)
	req.Header.Set("Content-Type", "text/plain")

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for body too large")
	}
	if err.Status != 413 {
		t.Errorf("expected status 413, got %d", err.Status)
	}
}

func TestSanitizer_HopByHopStripped(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Proxy-Connection", "keep-alive")

	_, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("Connection") != "" {
		t.Error("expected Connection header to be stripped")
	}
	if req.Header.Get("Keep-Alive") != "" {
		t.Error("expected Keep-Alive header to be stripped")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"/a/b/../c", "/a/c"},
		{"/a/./b", "/a/b"},
		{"/a//b", "/a/b"},
		{"/a/b/../../c", "/c"},
		{"/%2e%2e/etc/passwd", "/etc/passwd"}, // decoded + resolved
	}

	for _, tt := range tests {
		got := NormalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDecodeMultiLevel(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"%27", "'"},
		{"%2527", "'"},     // double encoded
		{"hello", "hello"}, // no encoding
		{"%00", ""},        // null byte removed
	}

	for _, tt := range tests {
		got := DecodeMultiLevel(tt.input)
		if got != tt.expected {
			t.Errorf("DecodeMultiLevel(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDecodeMultiLevel_Empty(t *testing.T) {
	result := DecodeMultiLevel("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Status: 400, Message: "bad request"}
	if ve.Error() != "bad request" {
		t.Errorf("expected 'bad request', got %q", ve.Error())
	}
}

func TestSanitizer_HeaderTooLarge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxHeaderSize = 50
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Big", strings.Repeat("x", 100))

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for header too large")
	}
	if err.Status != 431 {
		t.Errorf("expected status 431, got %d", err.Status)
	}
	if !strings.Contains(err.Message, "X-Big") {
		t.Error("expected header name in error message")
	}
}

func TestSanitizer_TooManyCookies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCookieCount = 2
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Add("Cookie", "a=1")
	req.Header.Add("Cookie", "b=2")
	req.Header.Add("Cookie", "c=3")

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for too many cookies")
	}
	if err.Status != 400 {
		t.Errorf("expected status 400, got %d", err.Status)
	}
	if !strings.Contains(err.Message, "too many cookies") {
		t.Errorf("expected 'too many cookies' message, got %q", err.Message)
	}
}

func TestSanitizer_CookieTooLarge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCookieCount = 10
	cfg.MaxCookieSize = 5
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Add("Cookie", "big="+strings.Repeat("x", 20))

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for cookie too large")
	}
	if err.Status != 400 {
		t.Errorf("expected status 400, got %d", err.Status)
	}
	if !strings.Contains(err.Message, "cookie too large") {
		t.Errorf("expected 'cookie too large' message, got %q", err.Message)
	}
}

func TestSanitizer_NullByteInQuery(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/path?q=hello%00world", nil)

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for null byte in query")
	}
	if err.Status != 400 {
		t.Errorf("expected status 400, got %d", err.Status)
	}
}

func TestSanitizer_NullByteInHeader(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Test", "hello\x00world")

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for null byte in header")
	}
	if err.Status != 400 {
		t.Errorf("expected status 400, got %d", err.Status)
	}
	if !strings.Contains(err.Message, "null byte in header") {
		t.Errorf("expected 'null byte in header' message, got %q", err.Message)
	}
}

func TestSanitizer_BodyContentLengthTooLarge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBodySize = 10
	s := New(cfg)

	body := strings.NewReader("short")
	req := httptest.NewRequest("POST", "http://example.com/", body)
	req.ContentLength = 100 // claims large content-length

	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for content-length > max body size")
	}
	if err.Status != 413 {
		t.Errorf("expected status 413, got %d", err.Status)
	}
}

func TestSanitizer_ValidBody(t *testing.T) {
	s := New(DefaultConfig())
	body := strings.NewReader("hello world")
	req := httptest.NewRequest("POST", "http://example.com/", body)
	req.Header.Set("Content-Type", "text/plain")

	result, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Body) != "hello world" {
		t.Errorf("expected body 'hello world', got %q", string(result.Body))
	}
}

func TestSanitizer_NormalizeEncoding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NormalizeEncoding = true
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/a/b/../c?q=%27", nil)

	result, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DecodedPath == "" {
		t.Error("expected decoded path to be populated")
	}
	if result.DecodedQuery == "" {
		t.Error("expected decoded query to be populated")
	}
}

func TestSanitizer_NormalizeEncodingDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NormalizeEncoding = false
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/test?q=hello", nil)

	result, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DecodedPath != "" {
		t.Error("expected no decoded path when normalization disabled")
	}
}

func TestSanitizer_BodyDecodedWhenNormalize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NormalizeEncoding = true
	s := New(cfg)

	body := strings.NewReader("key=%27value%27")
	req := httptest.NewRequest("POST", "http://example.com/", body)
	req.Header.Set("Content-Type", "text/plain")

	result, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DecodedBody == "" {
		t.Error("expected decoded body to be populated")
	}
}

func TestSanitizer_PathOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBodySize = 10
	cfg.PathOverrides = []PathOverride{
		{Pattern: "/upload/*", MaxBodySize: 1024},
	}
	s := New(cfg)

	// Normal path should reject large body
	body := strings.NewReader(strings.Repeat("x", 20))
	req := httptest.NewRequest("POST", "http://example.com/api", body)
	_, err := s.Process(req)
	if err == nil {
		t.Fatal("expected error for body too large on normal path")
	}

	// Override path should allow larger body
	body2 := strings.NewReader(strings.Repeat("x", 20))
	req2 := httptest.NewRequest("POST", "http://example.com/upload/file", body2)
	_, err2 := s.Process(req2)
	if err2 != nil {
		t.Fatalf("expected no error on override path, got %v", err2)
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		path, pattern string
		want          bool
	}{
		{"/api/upload", "/api/upload", true},
		{"/api/upload", "/api/other", false},
		{"/upload/file.txt", "/upload/*", true},
		{"/upload/dir/file.txt", "/upload/*", true},
		{"/other/file.txt", "/upload/*", false},
		{"/any", "", false},
		{"/exact", "/exact", true},
	}
	for _, tt := range tests {
		got := matchPath(tt.path, tt.pattern)
		if got != tt.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestSanitizer_StripHopByHopDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StripHopByHop = false
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("Connection", "keep-alive")

	_, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("Connection") != "keep-alive" {
		t.Error("expected Connection header to be preserved when StripHopByHop is disabled")
	}
}

func TestSanitizer_BlockNullBytesDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BlockNullBytes = false
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/path%00evil", nil)

	_, err := s.Process(req)
	if err != nil {
		t.Fatal("expected no error when null byte blocking is disabled")
	}
}

func TestSanitizer_NoBody(t *testing.T) {
	s := New(DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Body = nil
	req.ContentLength = 0

	result, err := s.Process(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Body) != 0 {
		t.Error("expected empty body")
	}
}

func TestSanitizer_EmptyAllowedMethods(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowedMethods = nil
	s := New(cfg)

	req := httptest.NewRequest("CUSTOM", "http://example.com/", nil)
	_, err := s.Process(req)
	// With empty allowed methods map, all methods should be allowed
	if err != nil {
		t.Fatalf("expected no error with empty allowed methods, got %v", err)
	}
}

func TestCanonicalizePath_Backslash(t *testing.T) {
	result := canonicalizePath("/a\\b\\c")
	if result != "/a/b/c" {
		t.Errorf("expected /a/b/c, got %q", result)
	}
}

func TestCanonicalizePath_TrailingDotSlash(t *testing.T) {
	result := canonicalizePath("/path/.")
	if result != "/path/" {
		t.Errorf("expected /path/, got %q", result)
	}
}

func TestCanonicalizePath_TrailingDots(t *testing.T) {
	result := canonicalizePath("/path...")
	if result != "/path" {
		t.Errorf("expected /path, got %q", result)
	}
}

func TestCanonicalizePath_Empty(t *testing.T) {
	result := canonicalizePath("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestResolveTraversals(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"/a/b/../c", "/a/c"},
		{"/a/b/../../c", "/c"},
		{"/../../a", "/a"},
		{"/../", "/"},
		{"/a/./b", "/a/b"},
		{"", "/"},
	}
	for _, tt := range tests {
		got := resolveTraversals(tt.input)
		if got != tt.expected {
			t.Errorf("resolveTraversals(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizer_MaxURLLengthZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxURLLength = 0 // disabled
	s := New(cfg)

	longURL := "http://example.com/" + strings.Repeat("a", 10000)
	req := httptest.NewRequest("GET", longURL, nil)
	_, err := s.Process(req)
	if err != nil {
		t.Fatal("expected no error when MaxURLLength is 0")
	}
}

func TestSanitizer_MaxHeaderCountZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxHeaderCount = 0 // disabled
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	for i := 0; i < 200; i++ {
		req.Header.Add("X-Custom", strings.Repeat("a", i+1))
	}
	_, err := s.Process(req)
	if err != nil {
		t.Fatal("expected no error when MaxHeaderCount is 0")
	}
}

func TestSanitizer_MaxHeaderSizeZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxHeaderSize = 0 // disabled
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Huge", strings.Repeat("x", 100000))
	_, err := s.Process(req)
	if err != nil {
		t.Fatal("expected no error when MaxHeaderSize is 0")
	}
}

func TestSanitizer_MaxCookieCountZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCookieCount = 0 // disabled
	s := New(cfg)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	for i := 0; i < 100; i++ {
		req.Header.Add("Cookie", "c"+strings.Repeat("a", i+1)+"=v")
	}
	_, err := s.Process(req)
	if err != nil {
		t.Fatal("expected no error when MaxCookieCount is 0")
	}
}

func TestMatchPath_WildcardNotAtEnd(t *testing.T) {
	// Pattern with * not at the end, e.g. "/api/*/data"
	// According to the code, this falls through to `path == pattern`
	got := matchPath("/api/v1/data", "/api/*/data")
	if got {
		t.Error("expected false for wildcard not at end of pattern (literal comparison)")
	}
}

func TestSanitizer_PathOverrideExactMatch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBodySize = 10
	cfg.PathOverrides = []PathOverride{
		{Pattern: "/exact/path", MaxBodySize: 1024},
	}
	s := New(cfg)

	body := strings.NewReader(strings.Repeat("x", 20))
	req := httptest.NewRequest("POST", "http://example.com/exact/path", body)
	_, err := s.Process(req)
	if err != nil {
		t.Fatalf("expected no error on exact override path, got %v", err)
	}
}

func TestDecodeMultiLevel_QuadrupleEncoding(t *testing.T) {
	// Quadruple-encoded single quote: %25252527 → %252527 → %2527 → %27 → '
	// With limit=3 this would stop at %27; with limit=5 it reaches '
	input := "%25252527"
	got := DecodeMultiLevel(input)
	if got != "'" {
		t.Errorf("DecodeMultiLevel(%q) = %q, want %q (quadruple encoding not fully decoded)", input, got, "'")
	}
}

func TestDecodeMultiLevel_FiveLevel(t *testing.T) {
	// 5-level encoding should decode fully
	input := "%2525252527"
	got := DecodeMultiLevel(input)
	if got != "'" {
		t.Errorf("DecodeMultiLevel(%q) = %q, want %q", input, got, "'")
	}
}
