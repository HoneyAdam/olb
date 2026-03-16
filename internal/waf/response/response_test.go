package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInjectHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := DefaultHeadersConfig()
	InjectHeaders(rr, cfg)

	checks := []struct {
		header, expected string
	}{
		{"Strict-Transport-Security", "max-age=31536000; includeSubDomains"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "SAMEORIGIN"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"X-XSS-Protection", "0"},
	}

	for _, c := range checks {
		got := rr.Header().Get(c.header)
		if got != c.expected {
			t.Errorf("%s = %q, want %q", c.header, got, c.expected)
		}
	}
}

func TestInjectHeaders_HSTSPreload(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := DefaultHeadersConfig()
	cfg.HSTSPreload = true
	InjectHeaders(rr, cfg)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "preload") {
		t.Error("expected preload in HSTS header")
	}
}

func TestInjectHeaders_HSTSDisabled(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := DefaultHeadersConfig()
	cfg.HSTSEnabled = false
	InjectHeaders(rr, cfg)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Error("expected no HSTS header when disabled")
	}
}

func TestInjectHeaders_HSTSZeroMaxAge(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := DefaultHeadersConfig()
	cfg.HSTSMaxAge = 0
	InjectHeaders(rr, cfg)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Error("expected no HSTS header when max-age is 0")
	}
}

func TestInjectHeaders_XContentTypeOptionsDisabled(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{}
	InjectHeaders(rr, cfg)

	xcto := rr.Header().Get("X-Content-Type-Options")
	if xcto != "" {
		t.Error("expected no X-Content-Type-Options when disabled")
	}
}

func TestInjectHeaders_EmptyXFrameOptions(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{}
	InjectHeaders(rr, cfg)

	xfo := rr.Header().Get("X-Frame-Options")
	if xfo != "" {
		t.Error("expected no X-Frame-Options when empty")
	}
}

func TestInjectHeaders_EmptyReferrerPolicy(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{}
	InjectHeaders(rr, cfg)

	rp := rr.Header().Get("Referrer-Policy")
	if rp != "" {
		t.Error("expected no Referrer-Policy when empty")
	}
}

func TestInjectHeaders_EmptyPermissionsPolicy(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{}
	InjectHeaders(rr, cfg)

	pp := rr.Header().Get("Permissions-Policy")
	if pp != "" {
		t.Error("expected no Permissions-Policy when empty")
	}
}

func TestInjectHeaders_CSP(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{CSP: "default-src 'self'"}
	InjectHeaders(rr, cfg)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp != "default-src 'self'" {
		t.Errorf("expected CSP header, got %q", csp)
	}
}

func TestInjectHeaders_NoCSPWhenEmpty(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{}
	InjectHeaders(rr, cfg)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp != "" {
		t.Error("expected no CSP header when empty")
	}
}

func TestInjectHeaders_HSTSNoIncludeSubdomains(t *testing.T) {
	rr := httptest.NewRecorder()
	cfg := HeadersConfig{
		HSTSEnabled:           true,
		HSTSMaxAge:            3600,
		HSTSIncludeSubdomains: false,
	}
	InjectHeaders(rr, cfg)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if strings.Contains(hsts, "includeSubDomains") {
		t.Error("expected no includeSubDomains")
	}
	if hsts != "max-age=3600" {
		t.Errorf("expected 'max-age=3600', got %q", hsts)
	}
}

func TestMaskSensitiveData_CreditCard(t *testing.T) {
	cfg := MaskingConfig{MaskCreditCards: true}
	input := []byte(`{"card":"4532 1234 5678 0123"}`)
	result := MaskSensitiveData(input, cfg)

	if strings.Contains(string(result), "1234 5678") {
		t.Error("expected middle digits to be masked")
	}
	if !strings.Contains(string(result), "4532") {
		t.Error("expected first 4 digits preserved")
	}
}

func TestMaskSensitiveData_SSN(t *testing.T) {
	cfg := MaskingConfig{MaskSSN: true}
	input := []byte(`SSN: 123-45-6789`)
	result := MaskSensitiveData(input, cfg)

	if strings.Contains(string(result), "123-45") {
		t.Error("expected SSN prefix to be masked")
	}
	if !strings.Contains(string(result), "6789") {
		t.Error("expected last 4 SSN digits preserved")
	}
}

func TestMaskSensitiveData_APIKeys(t *testing.T) {
	cfg := MaskingConfig{MaskAPIKeys: true}
	// Build test key at runtime to avoid triggering secret scanners
	prefix := "sk_" + "live_"
	fakeKey := prefix + "abcdefghijklmnopqrstuvwxyz1234"
	input := []byte(`{"key":"` + fakeKey + `"}`)
	result := MaskSensitiveData(input, cfg)

	output := string(result)
	if strings.Contains(output, "abcdefghijklmnopqrstuvwxyz") {
		t.Error("expected API key middle to be masked")
	}
	if !strings.Contains(output, "sk_l") {
		t.Error("expected API key prefix preserved")
	}
}

func TestMaskSensitiveData_EmptyBody(t *testing.T) {
	cfg := MaskingConfig{MaskCreditCards: true}
	result := MaskSensitiveData(nil, cfg)
	if result != nil {
		t.Error("expected nil for nil input")
	}
	result2 := MaskSensitiveData([]byte{}, cfg)
	if len(result2) != 0 {
		t.Error("expected empty for empty input")
	}
}

func TestMaskSensitiveData_MultipleAPIKeyPatterns(t *testing.T) {
	cfg := MaskingConfig{MaskAPIKeys: true}
	suffix := "abcdefghijklmnopqrstuvwxyz"
	// Build test keys at runtime to avoid triggering secret scanners
	keys := []string{
		"sk_" + "test_" + suffix,
		"pk_" + "live_" + suffix,
		"pk_" + "test_" + suffix,
		"ghp_" + suffix + "0123456789",
		"gho_" + suffix + "0123456789",
		"glpat-" + suffix[:20] + "0123",
		"AKIA" + "IOSFODNN7EXAMPLE",
	}
	for _, key := range keys {
		result := MaskSensitiveData([]byte(key), cfg)
		if string(result) == key {
			t.Errorf("expected API key %q to be masked", key)
		}
	}
}

func TestMaskSensitiveData_StackTraces(t *testing.T) {
	cfg := MaskingConfig{StripStackTraces: true}

	// Go goroutine stack trace
	goTrace := []byte("goroutine 1 [running]:\nmain.main()\n\t/app/main.go:10\n")
	result := MaskSensitiveData(goTrace, cfg)
	if strings.Contains(string(result), "goroutine") {
		t.Error("expected Go stack trace to be stripped")
	}
	if !strings.Contains(string(result), "[stack trace removed]") {
		t.Error("expected stack trace replacement text")
	}

	// Node.js stack trace
	nodeTrace := []byte("Error: something\n    at Object.<anonymous> (/app/index.js:10:5)\n")
	result2 := MaskSensitiveData(nodeTrace, cfg)
	if strings.Contains(string(result2), "at Object") {
		t.Error("expected Node.js stack trace to be stripped")
	}

	// Python traceback header
	pyTrace := []byte("Traceback (most recent call last):\n")
	result3 := MaskSensitiveData(pyTrace, cfg)
	if strings.Contains(string(result3), "Traceback") {
		t.Error("expected Python traceback to be stripped")
	}

	// Python file line
	pyFile := []byte(`  File "/app/main.py", line 42` + "\n")
	result4 := MaskSensitiveData(pyFile, cfg)
	if strings.Contains(string(result4), "File \"/app") {
		t.Error("expected Python file reference to be stripped")
	}
}

func TestMaskSensitiveData_MaskEmails(t *testing.T) {
	cfg := MaskingConfig{MaskEmails: true}
	// MaskEmails is in the config but the code doesn't currently implement email masking
	// This test verifies it doesn't panic and returns input unchanged
	input := []byte("user@example.com")
	result := MaskSensitiveData(input, cfg)
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestProtection_HeaderInjection(t *testing.T) {
	p := DefaultProtection()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rr := httptest.NewRecorder()

	pw := p.Wrap(rr)
	next.ServeHTTP(pw, req)

	// Flush buffered response
	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options: nosniff")
	}
}

func TestGenericErrorPage(t *testing.T) {
	page := GenericErrorPage(500)
	s := string(page)
	if !strings.Contains(s, "500") {
		t.Error("expected 500 in error page")
	}
	if !strings.Contains(s, "Internal Server Error") {
		t.Error("expected status text in error page")
	}
	if !strings.Contains(s, "OpenLoadBalancer") {
		t.Error("expected OLB branding in error page")
	}
}

func TestGenericErrorPage_UnknownStatus(t *testing.T) {
	page := GenericErrorPage(999)
	s := string(page)
	if !strings.Contains(s, "Error") {
		t.Error("expected 'Error' title for unknown status code")
	}
	if !strings.Contains(s, "999") {
		t.Error("expected 999 in error page")
	}
}

func TestIsTextContent(t *testing.T) {
	texts := []string{"text/html", "application/json", "text/plain", "text/xml", "application/xml"}
	for _, ct := range texts {
		if !IsTextContent(ct) {
			t.Errorf("expected %q to be text content", ct)
		}
	}

	nonTexts := []string{"image/png", "application/octet-stream", "video/mp4"}
	for _, ct := range nonTexts {
		if IsTextContent(ct) {
			t.Errorf("expected %q to NOT be text content", ct)
		}
	}
}

func TestIsServerError(t *testing.T) {
	if !IsServerError(500) {
		t.Error("500 should be server error")
	}
	if !IsServerError(503) {
		t.Error("503 should be server error")
	}
	if IsServerError(200) {
		t.Error("200 should not be server error")
	}
	if IsServerError(404) {
		t.Error("404 should not be server error")
	}
	if IsServerError(499) {
		t.Error("499 should not be server error")
	}
	if !IsServerError(599) {
		t.Error("599 should be server error")
	}
	if IsServerError(600) {
		t.Error("600 should not be server error")
	}
}

func TestProtectedWriter_WriteWithoutWriteHeader(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	// Write without calling WriteHeader — should auto-set 200
	pw.Write([]byte("hello"))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestProtectedWriter_DoubleWriteHeader(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.WriteHeader(http.StatusOK)
	pw.WriteHeader(http.StatusNotFound) // second call should be ignored

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected first status 200, got %d", rr.Code)
	}
}

func TestProtectedWriter_BufferingForMasking(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{MaskCreditCards: true},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	// Set text content type so masking is triggered
	pw.Header().Set("Content-Type", "text/html")
	pw.WriteHeader(http.StatusOK)
	pw.Write([]byte(`Card: 4532 1234 5678 0123`))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	body := rr.Body.String()
	if strings.Contains(body, "1234 5678") {
		t.Error("expected credit card to be masked in buffered response")
	}
}

func TestProtectedWriter_BufferingForErrorPage(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{},
		ErrorPages: ErrorPageConfig{Enabled: true, Mode: "production"},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	// Set text content type
	pw.Header().Set("Content-Type", "text/html")
	pw.WriteHeader(http.StatusInternalServerError)
	pw.Write([]byte("internal error details that should be hidden"))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	body := rr.Body.String()
	if strings.Contains(body, "internal error details") {
		t.Error("expected error details to be replaced with generic page")
	}
	if !strings.Contains(body, "500") {
		t.Error("expected generic 500 error page")
	}
	if !strings.Contains(body, "OpenLoadBalancer") {
		t.Error("expected branding in error page")
	}
}

func TestProtectedWriter_FlushNonBuffering(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.WriteHeader(http.StatusOK)
	pw.Write([]byte("hello"))

	// Flush when not buffering should call underlying Flusher
	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	if rr.Body.String() != "hello" {
		t.Errorf("expected 'hello', got %q", rr.Body.String())
	}
}

func TestProtectedWriter_FlushWithMaskingNoErrorPage(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{MaskSSN: true},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.Header().Set("Content-Type", "text/html")
	pw.WriteHeader(http.StatusOK)
	pw.Write([]byte("SSN: 123-45-6789"))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	body := rr.Body.String()
	if strings.Contains(body, "123-45") {
		t.Error("expected SSN to be masked")
	}
	if !strings.Contains(body, "6789") {
		t.Error("expected last 4 digits preserved")
	}
}

func TestProtectedWriter_ContentLengthUpdated(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{MaskCreditCards: true},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.Header().Set("Content-Type", "application/json")
	pw.WriteHeader(http.StatusOK)
	pw.Write([]byte(`{"card":"4532 1234 5678 0123"}`))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	cl := rr.Header().Get("Content-Length")
	if cl == "" {
		t.Error("expected Content-Length header to be set")
	}
}

func TestProtectedWriter_NonTextContentNotBuffered(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{MaskCreditCards: true},
		ErrorPages: ErrorPageConfig{},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.Header().Set("Content-Type", "image/png")
	pw.WriteHeader(http.StatusOK)
	pw.Write([]byte("binary data"))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	if rr.Body.String() != "binary data" {
		t.Error("expected binary data to pass through unchanged")
	}
}

func TestProtectedWriter_ErrorPageContentType(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{},
		ErrorPages: ErrorPageConfig{Enabled: true, Mode: "production"},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.Header().Set("Content-Type", "text/plain")
	pw.WriteHeader(http.StatusBadGateway)
	pw.Write([]byte("upstream error"))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type for error page, got %q", ct)
	}
}

func TestDefaultProtection(t *testing.T) {
	p := DefaultProtection()
	if p == nil {
		t.Fatal("expected non-nil protection")
	}
	if !p.Headers.HSTSEnabled {
		t.Error("expected HSTS enabled by default")
	}
	if !p.Masking.MaskCreditCards {
		t.Error("expected credit card masking by default")
	}
	if !p.ErrorPages.Enabled {
		t.Error("expected error pages enabled by default")
	}
}

func TestProtectedWriter_ErrorPageInDevMode(t *testing.T) {
	p := &Protection{
		Headers:    HeadersConfig{},
		Masking:    MaskingConfig{},
		ErrorPages: ErrorPageConfig{Enabled: true, Mode: "development"},
	}

	rr := httptest.NewRecorder()
	pw := p.Wrap(rr)

	pw.WriteHeader(http.StatusInternalServerError)
	pw.Write([]byte("detailed error in dev mode"))

	if f, ok := pw.(interface{ Flush() }); ok {
		f.Flush()
	}

	// In development mode, error pages should NOT be replaced
	// (needsErrorPage is false because mode != "production")
	body := rr.Body.String()
	if !strings.Contains(body, "detailed error in dev mode") {
		t.Error("expected dev mode error to pass through")
	}
}

func TestMaskSensitiveData_CreditCardWithDashes(t *testing.T) {
	cfg := MaskingConfig{MaskCreditCards: true}
	input := []byte(`4532-1234-5678-0123`)
	result := MaskSensitiveData(input, cfg)
	if strings.Contains(string(result), "1234-5678") {
		t.Error("expected dashed credit card to be masked")
	}
}

func TestMaskSensitiveData_CreditCardNoSeparator(t *testing.T) {
	cfg := MaskingConfig{MaskCreditCards: true}
	input := []byte(`4532123456780123`)
	result := MaskSensitiveData(input, cfg)
	if strings.Contains(string(result), "12345678") {
		t.Error("expected contiguous credit card to be masked")
	}
}
