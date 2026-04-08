package security

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// TLS config tests
// --------------------------------------------------------------------------

func TestDefaultTLSConfig_MinVersion(t *testing.T) {
	cfg := DefaultTLSConfig()
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d; want %d (TLS 1.2)", cfg.MinVersion, tls.VersionTLS12)
	}
}

func TestDefaultTLSConfig_CipherSuites(t *testing.T) {
	cfg := DefaultTLSConfig()

	expected := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}

	if len(cfg.CipherSuites) != len(expected) {
		t.Fatalf("CipherSuites length = %d; want %d", len(cfg.CipherSuites), len(expected))
	}

	for i, cs := range cfg.CipherSuites {
		if cs != expected[i] {
			t.Errorf("CipherSuites[%d] = 0x%04x; want 0x%04x", i, cs, expected[i])
		}
	}
}

func TestDefaultTLSConfig_CurvePreferences(t *testing.T) {
	cfg := DefaultTLSConfig()

	expected := []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
	if len(cfg.CurvePreferences) != len(expected) {
		t.Fatalf("CurvePreferences length = %d; want %d", len(cfg.CurvePreferences), len(expected))
	}
	for i, c := range cfg.CurvePreferences {
		if c != expected[i] {
			t.Errorf("CurvePreferences[%d] = %v; want %v", i, c, expected[i])
		}
	}
}

func TestDefaultTLSConfig_PreferServerCipherSuites(t *testing.T) {
	cfg := DefaultTLSConfig()
	if !cfg.PreferServerCipherSuites {
		t.Error("PreferServerCipherSuites should be true")
	}
}

func TestDefaultTLSConfig_NoWeakCiphers(t *testing.T) {
	cfg := DefaultTLSConfig()

	// These are weak / deprecated cipher suites that must NOT be present.
	weak := []uint16{
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_RC4_128_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	}

	for _, w := range weak {
		for _, cs := range cfg.CipherSuites {
			if cs == w {
				t.Errorf("CipherSuites contains weak cipher 0x%04x", w)
			}
		}
	}
}

func TestStrongCipherSuiteNames(t *testing.T) {
	names := StrongCipherSuiteNames()
	if len(names) != 6 {
		t.Errorf("StrongCipherSuiteNames returned %d names; want 6", len(names))
	}
	// Verify all names contain ECDHE.
	for _, name := range names {
		if !strings.Contains(name, "ECDHE") {
			t.Errorf("cipher suite %q does not contain ECDHE", name)
		}
	}
}

// --------------------------------------------------------------------------
// Slow loris protection tests
// --------------------------------------------------------------------------

func TestDefaultSlowLorisProtection(t *testing.T) {
	slp := DefaultSlowLorisProtection()

	if slp.MaxHeaderBytes != 1<<20 {
		t.Errorf("MaxHeaderBytes = %d; want %d", slp.MaxHeaderBytes, 1<<20)
	}
	if slp.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v; want 10s", slp.ReadHeaderTimeout)
	}
	if slp.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v; want 30s", slp.ReadTimeout)
	}
	if slp.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v; want 30s", slp.WriteTimeout)
	}
	if slp.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v; want 120s", slp.IdleTimeout)
	}
}

func TestSlowLorisProtection_ApplyToServer(t *testing.T) {
	slp := DefaultSlowLorisProtection()
	srv := &http.Server{}
	slp.ApplyToServer(srv)

	if srv.MaxHeaderBytes != slp.MaxHeaderBytes {
		t.Errorf("MaxHeaderBytes = %d; want %d", srv.MaxHeaderBytes, slp.MaxHeaderBytes)
	}
	if srv.ReadHeaderTimeout != slp.ReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v; want %v", srv.ReadHeaderTimeout, slp.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != slp.ReadTimeout {
		t.Errorf("ReadTimeout = %v; want %v", srv.ReadTimeout, slp.ReadTimeout)
	}
	if srv.WriteTimeout != slp.WriteTimeout {
		t.Errorf("WriteTimeout = %v; want %v", srv.WriteTimeout, slp.WriteTimeout)
	}
	if srv.IdleTimeout != slp.IdleTimeout {
		t.Errorf("IdleTimeout = %v; want %v", srv.IdleTimeout, slp.IdleTimeout)
	}
}

func TestSlowLorisProtection_ApplyToServer_ZeroValues(t *testing.T) {
	slp := &SlowLorisProtection{} // all zero
	srv := &http.Server{
		MaxHeaderBytes:    999,
		ReadHeaderTimeout: 5 * time.Second,
	}
	slp.ApplyToServer(srv)

	// Zero values should not overwrite.
	if srv.MaxHeaderBytes != 999 {
		t.Errorf("MaxHeaderBytes was overwritten to %d; should remain 999", srv.MaxHeaderBytes)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout was overwritten; should remain 5s")
	}
}

// --------------------------------------------------------------------------
// Request smuggling tests
// --------------------------------------------------------------------------

func TestValidateRequest_Clean(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := ValidateRequest(r); err != nil {
		t.Errorf("clean request should pass; got %v", err)
	}
}

func TestValidateRequest_ConflictingCLAndTE(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Content-Length", "4")
	r.Header.Set("Transfer-Encoding", "chunked")
	err := ValidateRequest(r)
	if err != ErrConflictingHeaders {
		t.Errorf("expected ErrConflictingHeaders; got %v", err)
	}
}

func TestValidateRequest_DuplicateContentLength_DifferentValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header["Content-Length"] = []string{"4", "5"}
	err := ValidateRequest(r)
	if err != ErrDuplicateContentLen {
		t.Errorf("expected ErrDuplicateContentLen; got %v", err)
	}
}

func TestValidateRequest_DuplicateContentLength_SameValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header["Content-Length"] = []string{"4", "4"}
	err := ValidateRequest(r)
	if err != nil {
		t.Errorf("duplicate CL with same values should be ok; got %v", err)
	}
}

func TestValidateRequest_MalformedContentLength(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Content-Length", "abc")
	err := ValidateRequest(r)
	if err != ErrMalformedContentLen {
		t.Errorf("expected ErrMalformedContentLen; got %v", err)
	}
}

func TestValidateRequest_NegativeContentLength(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Content-Length", "-1")
	err := ValidateRequest(r)
	if err != ErrNegativeContentLen {
		t.Errorf("expected ErrNegativeContentLen for negative CL; got %v", err)
	}
}

func TestValidateRequest_CommaSeparatedContentLength_Different(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Content-Length", "1, 2")
	err := ValidateRequest(r)
	if err != ErrDuplicateContentLen {
		t.Errorf("expected ErrDuplicateContentLen for comma-separated different CL; got %v", err)
	}
}

func TestValidateRequest_HTTP10WithTransferEncoding(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Proto = "HTTP/1.0"
	r.ProtoMajor = 1
	r.ProtoMinor = 0
	r.Header.Set("Transfer-Encoding", "chunked")
	err := ValidateRequest(r)
	if err != ErrHTTP10TransferEnc {
		t.Errorf("expected ErrHTTP10TransferEnc; got %v", err)
	}
}

func TestValidateRequest_ChunkedNotFinal(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Transfer-Encoding", "chunked, gzip")
	err := ValidateRequest(r)
	if err != ErrChunkedNotFinal {
		t.Errorf("expected ErrChunkedNotFinal; got %v", err)
	}
}

func TestValidateRequest_InvalidTransferEncoding(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Transfer-Encoding", "bogus")
	err := ValidateRequest(r)
	if err != ErrInvalidTransferEnc {
		t.Errorf("expected ErrInvalidTransferEnc; got %v", err)
	}
}

func TestValidateRequest_ValidChunked(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Transfer-Encoding", "gzip, chunked")
	err := ValidateRequest(r)
	if err != nil {
		t.Errorf("valid TE (gzip, chunked) should pass; got %v", err)
	}
}

func TestValidateRequest_ValidContentLength(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Content-Length", "4")
	err := ValidateRequest(r)
	if err != nil {
		t.Errorf("valid CL should pass; got %v", err)
	}
}

// --------------------------------------------------------------------------
// Header injection tests
// --------------------------------------------------------------------------

func TestSanitizeHeaderValue_NoInjection(t *testing.T) {
	val := "application/json"
	out := SanitizeHeaderValue(val)
	if out != val {
		t.Errorf("SanitizeHeaderValue(%q) = %q; want %q", val, out, val)
	}
}

func TestSanitizeHeaderValue_RemovesCR(t *testing.T) {
	val := "value\rinjected"
	out := SanitizeHeaderValue(val)
	if strings.Contains(out, "\r") {
		t.Errorf("SanitizeHeaderValue should remove CR; got %q", out)
	}
	if out != "valueinjected" {
		t.Errorf("SanitizeHeaderValue(%q) = %q; want %q", val, out, "valueinjected")
	}
}

func TestSanitizeHeaderValue_RemovesLF(t *testing.T) {
	val := "value\ninjected"
	out := SanitizeHeaderValue(val)
	if strings.Contains(out, "\n") {
		t.Errorf("SanitizeHeaderValue should remove LF; got %q", out)
	}
	if out != "valueinjected" {
		t.Errorf("SanitizeHeaderValue(%q) = %q; want %q", val, out, "valueinjected")
	}
}

func TestSanitizeHeaderValue_RemovesCRLF(t *testing.T) {
	val := "value\r\nX-Injected: evil"
	out := SanitizeHeaderValue(val)
	expected := "valueX-Injected: evil"
	if out != expected {
		t.Errorf("SanitizeHeaderValue(%q) = %q; want %q", val, out, expected)
	}
}

func TestSanitizeHeaderValue_RemovesNUL(t *testing.T) {
	val := "value\x00injected"
	out := SanitizeHeaderValue(val)
	if out != "valueinjected" {
		t.Errorf("SanitizeHeaderValue should remove NUL; got %q", out)
	}
}

func TestSanitizeHeaderValue_ResponseSplitting(t *testing.T) {
	// Classic HTTP response splitting payload.
	val := "text/html\r\n\r\n<script>alert(1)</script>"
	out := SanitizeHeaderValue(val)
	if strings.Contains(out, "\r") || strings.Contains(out, "\n") {
		t.Errorf("response splitting payload not sanitized: %q", out)
	}
}

func TestValidateHeaderName_Valid(t *testing.T) {
	valid := []string{
		"Content-Type",
		"X-Custom-Header",
		"Accept",
		"x-lower",
		"X-123",
		"X-a.b",
		"X-a~b",
	}
	for _, name := range valid {
		if !ValidateHeaderName(name) {
			t.Errorf("ValidateHeaderName(%q) = false; want true", name)
		}
	}
}

func TestValidateHeaderName_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"Content Type",  // space
		"Content\tType", // tab
		"Content:Type",  // colon
		"Content\rType", // CR
		"Content\nType", // LF
		"(invalid)",     // parens
		"header@name",   // @
	}
	for _, name := range invalid {
		if ValidateHeaderName(name) {
			t.Errorf("ValidateHeaderName(%q) = true; want false", name)
		}
	}
}

// --------------------------------------------------------------------------
// Host header validation tests
// --------------------------------------------------------------------------

func TestValidateHostHeader_Valid(t *testing.T) {
	valid := []string{
		"example.com",
		"example.com:8080",
		"sub.example.com",
		"example.com:443",
		"localhost",
		"localhost:3000",
		"192.168.1.1",
		"192.168.1.1:80",
		"my-host.example.com",
	}
	for _, host := range valid {
		if !ValidateHostHeader(host) {
			t.Errorf("ValidateHostHeader(%q) = false; want true", host)
		}
	}
}

func TestValidateHostHeader_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"example.com:0",
		"example.com:99999",
		"example.com:abc",
		"user@example.com",
		"example .com",
		"example\t.com",
		"example\r.com",
		"example\n.com",
		".example.com",
		"example.com.",
		"-example.com",
		"example.com-",
		"example.com\\path",
	}
	for _, host := range invalid {
		if ValidateHostHeader(host) {
			t.Errorf("ValidateHostHeader(%q) = true; want false", host)
		}
	}
}

// --------------------------------------------------------------------------
// Content-Length validation tests
// --------------------------------------------------------------------------

func TestValidateContentLength_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0", 0},
		{"1", 1},
		{"100", 100},
		{"999999", 999999},
		{" 42 ", 42}, // whitespace trimmed
	}
	for _, tt := range tests {
		n, err := ValidateContentLength(tt.input)
		if err != nil {
			t.Errorf("ValidateContentLength(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if n != tt.expected {
			t.Errorf("ValidateContentLength(%q) = %d; want %d", tt.input, n, tt.expected)
		}
	}
}

func TestValidateContentLength_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"abc",
		"-1",
		"12.5",
		"1e3",
		"01",  // leading zero
		"007", // leading zeros
		"1 2", // space in middle
	}
	for _, cl := range invalid {
		_, err := ValidateContentLength(cl)
		if err == nil {
			t.Errorf("ValidateContentLength(%q) expected error; got nil", cl)
		}
	}
}

// --------------------------------------------------------------------------
// Path sanitization tests
// --------------------------------------------------------------------------

func TestSanitizePath_Clean(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/foo/bar", "/foo/bar"},
		{"/", "/"},
		{"", "/"},
	}
	for _, tt := range tests {
		out := SanitizePath(tt.input)
		if out != tt.expected {
			t.Errorf("SanitizePath(%q) = %q; want %q", tt.input, out, tt.expected)
		}
	}
}

func TestSanitizePath_Traversal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/../etc/passwd", "/etc/passwd"},
		{"/foo/../../etc/shadow", "/etc/shadow"},
		{"/foo/../bar", "/bar"},
		{"/foo/./bar", "/foo/bar"},
		{"/../../../etc/passwd", "/etc/passwd"},
	}
	for _, tt := range tests {
		out := SanitizePath(tt.input)
		if out != tt.expected {
			t.Errorf("SanitizePath(%q) = %q; want %q", tt.input, out, tt.expected)
		}
	}
}

func TestSanitizePath_EncodedTraversal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/%2e%2e/etc/passwd", "/etc/passwd"},
		{"/%2E%2E/etc/passwd", "/etc/passwd"},
		{"/%2e%2e/%2e%2e/etc/passwd", "/etc/passwd"},
		{"/foo/%2e%2e/bar", "/bar"},
	}
	for _, tt := range tests {
		out := SanitizePath(tt.input)
		if out != tt.expected {
			t.Errorf("SanitizePath(%q) = %q; want %q", tt.input, out, tt.expected)
		}
	}
}

func TestSanitizePath_BackslashTraversal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/..\\etc\\passwd", "/etc/passwd"},
		{"/foo/%5c..%5cbar", "/bar"},
	}
	for _, tt := range tests {
		out := SanitizePath(tt.input)
		if out != tt.expected {
			t.Errorf("SanitizePath(%q) = %q; want %q", tt.input, out, tt.expected)
		}
	}
}

// --------------------------------------------------------------------------
// Privilege dropper tests
// --------------------------------------------------------------------------

func TestNewPrivilegeDropper(t *testing.T) {
	pd := NewPrivilegeDropper()
	if pd == nil {
		t.Fatal("NewPrivilegeDropper returned nil")
	}
}

func TestPrivilegeDropper_NoOp(t *testing.T) {
	pd := NewPrivilegeDropper()
	// On non-Linux this is a no-op, on Linux it would fail without root.
	// Either way it should not panic.
	_ = pd.DropPrivileges(65534, 65534)
}

// --------------------------------------------------------------------------
// Attack vector tests — end-to-end payloads
// --------------------------------------------------------------------------

func TestAttackVector_HTTPRequestSmuggling_CLTE(t *testing.T) {
	// CL-TE smuggling: attacker sends both CL and TE.
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0\r\n\r\nGET /admin HTTP/1.1\r\nHost: evil\r\n\r\n"))
	r.Header.Set("Content-Length", "0")
	r.Header.Set("Transfer-Encoding", "chunked")
	err := ValidateRequest(r)
	if err == nil {
		t.Error("CL-TE smuggling should be detected")
	}
}

func TestAttackVector_HTTPRequestSmuggling_TECL(t *testing.T) {
	// TE-CL smuggling: same idea, reversed priority.
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header.Set("Transfer-Encoding", "chunked")
	r.Header.Set("Content-Length", "4")
	err := ValidateRequest(r)
	if err == nil {
		t.Error("TE-CL smuggling should be detected")
	}
}

func TestAttackVector_HeaderInjection_ResponseSplitting(t *testing.T) {
	payload := "text/html\r\nSet-Cookie: session=hijacked"
	sanitized := SanitizeHeaderValue(payload)
	if strings.Contains(sanitized, "\r") || strings.Contains(sanitized, "\n") {
		t.Errorf("response splitting not prevented: %q", sanitized)
	}
}

func TestAttackVector_PathTraversal_EtcPasswd(t *testing.T) {
	result := SanitizePath("/static/../../../etc/passwd")
	if strings.Contains(result, "..") {
		t.Errorf("path traversal not sanitized: %q", result)
	}
}

func TestAttackVector_PathTraversal_WindowsPath(t *testing.T) {
	result := SanitizePath("/..\\..\\windows\\system32\\config\\sam")
	if strings.Contains(result, "..") {
		t.Errorf("windows path traversal not sanitized: %q", result)
	}
}

func TestAttackVector_HostHeaderInjection(t *testing.T) {
	if ValidateHostHeader("example.com\r\nX-Injected: value") {
		t.Error("host header with CRLF injection should be rejected")
	}
}

func TestAttackVector_HostHeaderUserInfo(t *testing.T) {
	if ValidateHostHeader("admin:password@example.com") {
		t.Error("host header with user info should be rejected")
	}
}

func TestAttackVector_ContentLengthOverflow(t *testing.T) {
	// Extremely large CL that could cause integer overflow.
	_, err := ValidateContentLength("99999999999999999999999999999999")
	if err == nil {
		t.Error("overflowing Content-Length should be rejected")
	}
}

func TestAttackVector_DuplicateCLSmuggling(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	r.Header["Content-Length"] = []string{"4", "100"}
	err := ValidateRequest(r)
	if err == nil {
		t.Error("duplicate CL with different values should be detected")
	}
}

func TestValidateHostHeader_EdgeCases(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"", false},
		{"valid.com", true},
		{"valid.com:443", true},
		{"192.168.1.1", true},
		{"192.168.1.1:8080", true},
		{"[::1]", true},
		{"[::1]:8080", false}, // brackets stripped by SplitHostPort, ::1 has colons
		{"host with space", false},
		{"host\r\nInjection", false},
		{"user@host.com", false},
		{`host\evil.com`, false},
		{"host:0", false},
		{"host:99999", false},
		{"host:abc", false},
		{".dot-start.com", false},
		{"-hyphen-start.com", false},
		{"trailing-dot.", false},
		{"trailing-hyphen-", false},
		{"under_score.com", false},
		{"valid-host.example.com", true},
		{"a.b.c.d.e", true},
		{"[invalid:ip", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := ValidateHostHeader(tt.host)
			if got != tt.want {
				t.Errorf("ValidateHostHeader(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestSanitizePath_EdgeCases(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"", "/"},
		{"/", "/"},
		{"/valid/path", "/valid/path"},
		{"/../etc/passwd", "/etc/passwd"},
		{"/..%2f..%2fetc/passwd", "/etc/passwd"},
		{"/path/..%5C..%5Csecret", "/secret"},
		{"/normal/../file", "/file"},
		{"no-leading-slash", "/no-leading-slash"},
		{"/a/b/../../../c", "/c"},
		{"/%2e%2e/%2e%2e/secret", "/secret"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizePath(tt.input)
			if got != tt.output {
				t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestValidateContentLength_EdgeCases(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"", 0, true},
		{"0", 0, false},
		{"100", 100, false},
		{"01", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"99999999999999999999999999", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateContentLength(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got %d, want %d", got, tt.want)
				}
			}
		})
	}
}
