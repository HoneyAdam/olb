package hmac

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHMAC_Disabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should have been called when HMAC is disabled")
	}
}

func TestHMAC_MissingSignature(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without signature")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestHMAC_InvalidSignature(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with invalid signature")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", "invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestHMAC_ValidSignature(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Generate valid signature
	message := "GET\n/test\n"
	sig, err := GenerateSignature("secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid signature")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestHMAC_WithBody(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.UseBody = true

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"name":"test"}`
	// Message includes method, path, newline, then body
	message := "POST\n/test\n" + body
	sig, err := GenerateSignature("secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	var receivedBody string
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if receivedBody != body {
		t.Errorf("Body should be preserved, got %s", receivedBody)
	}
}

func TestHMAC_WithQueryString(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Message includes method, path, query string, newline
	message := "GET\n/test\npage=1&limit=10\n"
	sig, err := GenerateSignature("secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test?page=1&limit=10", nil)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid signature")
	}
}

func TestHMAC_WithPrefix(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.Prefix = "sha256="

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	message := "GET\n/test\n"
	sig, err := GenerateSignature("secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", "sha256="+sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid prefixed signature")
	}
}

func TestHMAC_Base64Encoding(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.Encoding = "base64"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	message := "GET\n/test\n"
	sig, err := GenerateSignature("secret", message, "sha256", "base64")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid base64 signature")
	}
}

func TestHMAC_SHA512(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.Algorithm = "sha512"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	message := "GET\n/test\n"
	sig, err := GenerateSignature("secret", message, "sha512", "hex")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid sha512 signature")
	}
}

func TestHMAC_ExcludedPath(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.ExcludePaths = []string{"/health", "/metrics"}
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called for excluded paths")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestHMAC_CustomHeader(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.Header = "X-HMAC-Signature"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	message := "GET\n/test\n"
	sig, err := GenerateSignature("secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-HMAC-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with custom header")
	}
}

func TestHMAC_WithoutBody(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.UseBody = false

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Message without body
	message := "POST\n/test\n"
	sig, err := GenerateSignature("secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	var receivedBody string
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
	}))

	body := `{"name":"test"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if receivedBody != body {
		t.Errorf("Body should be preserved, got %s", receivedBody)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled != false {
		t.Error("Default Enabled should be false")
	}
	if config.Algorithm != "sha256" {
		t.Errorf("Default Algorithm should be 'sha256', got '%s'", config.Algorithm)
	}
	if config.Header != "X-Signature" {
		t.Errorf("Default Header should be 'X-Signature', got '%s'", config.Header)
	}
	if config.Encoding != "hex" {
		t.Errorf("Default Encoding should be 'hex', got '%s'", config.Encoding)
	}
	if config.UseBody != true {
		t.Error("Default UseBody should be true")
	}
	if config.MaxAge != "5m" {
		t.Errorf("Default MaxAge should be '5m', got '%s'", config.MaxAge)
	}
}

func TestMiddleware_Priority(t *testing.T) {
	config := DefaultConfig()
	mw, _ := New(config)

	if mw.Priority() != 213 {
		t.Errorf("Expected priority 213, got %d", mw.Priority())
	}
}

func TestMiddleware_Name(t *testing.T) {
	config := DefaultConfig()
	mw, _ := New(config)

	if mw.Name() != "hmac" {
		t.Errorf("Expected name 'hmac', got '%s'", mw.Name())
	}
}

func TestGenerateSignature(t *testing.T) {
	tests := []struct {
		algorithm string
		encoding  string
	}{
		{"sha256", "hex"},
		{"sha256", "base64"},
		{"sha512", "hex"},
		{"sha512", "base64"},
	}

	for _, tt := range tests {
		sig, err := GenerateSignature("secret", "message", tt.algorithm, tt.encoding)
		if err != nil {
			t.Errorf("GenerateSignature(%s, %s) error: %v", tt.algorithm, tt.encoding, err)
			continue
		}
		if sig == "" {
			t.Error("Generated signature should not be empty")
		}
	}
}

func TestGenerateSignature_Defaults(t *testing.T) {
	// Test with empty algorithm and encoding (should use defaults)
	sig, err := GenerateSignature("secret", "message", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if sig == "" {
		t.Error("Generated signature should not be empty")
	}
}

func TestHMAC_WrongSecret(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "correct-secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Generate signature with wrong secret
	message := "GET\n/test\n"
	sig, err := GenerateSignature("wrong-secret", message, "sha256", "hex")
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with wrong secret")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

// --------------------------------------------------------------------------
// computeSignature: body read error path
// --------------------------------------------------------------------------

type errorReader struct{}

func (e *errorReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (e *errorReader) Close() error               { return nil }

func TestHMAC_ComputeSignature_BodyReadError(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.UseBody = true

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/test", io.NopCloser(&errorReader{}))
	_, sigErr := mw.computeSignature(req)
	if sigErr == nil {
		t.Error("expected error when body read fails")
	}
}

// --------------------------------------------------------------------------
// New: default algorithm case (unknown algorithm falls back to sha256)
// --------------------------------------------------------------------------

func TestNew_UnknownAlgorithm_DefaultsToSHA256(t *testing.T) {
	config := Config{
		Enabled:   true,
		Secret:    "secret",
		Algorithm: "blake2b",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Should not be nil (default case sets sha256.New)
	if mw.hasher == nil {
		t.Error("hasher should not be nil for unknown algorithm")
	}
}

// --------------------------------------------------------------------------
// computeSignature: default encoding case (unknown encoding falls back to hex)
// --------------------------------------------------------------------------

func TestHMAC_ComputeSignature_DefaultEncoding(t *testing.T) {
	config := Config{
		Enabled:  true,
		Secret:   "secret",
		Encoding: "unknown",
		UseBody:  false,
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	sig, sigErr := mw.computeSignature(req)
	if sigErr != nil {
		t.Fatalf("computeSignature error: %v", sigErr)
	}
	if sig == "" {
		t.Error("signature should not be empty")
	}

	// The default encoding case should produce hex output
	sig2, _ := GenerateSignature("secret", "GET\n/test\n", "sha256", "hex")
	if sig != sig2 {
		t.Errorf("default encoding should fall back to hex; got %q, want %q", sig, sig2)
	}
}

// --------------------------------------------------------------------------
// simpleError and errorf coverage
// --------------------------------------------------------------------------

func TestSimpleError_Error(t *testing.T) {
	err := errorf("test error message")
	if err.Error() != "test error message" {
		t.Errorf("errorf() = %q, want %q", err.Error(), "test error message")
	}

	// Verify it's a *simpleError
	if _, ok := err.(*simpleError); !ok {
		t.Error("errorf should return *simpleError")
	}
}

// --------------------------------------------------------------------------
// GenerateSignature: default encoding case
// --------------------------------------------------------------------------

func TestGenerateSignature_DefaultEncoding(t *testing.T) {
	sig, err := GenerateSignature("secret", "msg", "sha256", "custom_encoding")
	if err != nil {
		t.Fatal(err)
	}
	if sig == "" {
		t.Error("signature should not be empty for unknown encoding")
	}
}

func BenchmarkHMAC_Verification(b *testing.B) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, _ := New(config)

	message := "GET\n/test\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Signature", sig)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkGenerateSignature(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateSignature("secret", "message", "sha256", "hex")
	}
}

func TestErrorf(t *testing.T) {
	err := errorf("test error message")
	if err.Error() != "test error message" {
		t.Errorf("Expected 'test error message', got '%s'", err.Error())
	}
}

func TestNew_InvalidMaxAge(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "test-secret"
	config.MaxAge = "invalid-duration"

	mw, err := New(config)
	if err == nil {
		t.Error("Expected error for invalid MaxAge duration")
	}
	if mw != nil {
		t.Error("Expected nil middleware for invalid MaxAge")
	}
}

func TestNew_EmptyAlgorithm(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "test-secret"
	config.Algorithm = ""

	mw, err := New(config)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mw == nil {
		t.Fatal("Expected non-nil middleware")
	}
}

func TestNew_EmptyEncoding(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "test-secret"
	config.Encoding = ""

	mw, err := New(config)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mw == nil {
		t.Fatal("Expected non-nil middleware")
	}
}

// --------------------------------------------------------------------------
// Timestamp validation / replay protection tests
// --------------------------------------------------------------------------

func TestHMAC_TimestampMissing(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called without timestamp")
	}))

	message := "GET\n/test\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	// Intentionally NOT setting X-Timestamp
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "missing timestamp") {
		t.Errorf("Expected 'missing timestamp' in body, got %q", body)
	}
}

func TestHMAC_TimestampInvalidFormat(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with invalid timestamp")
	}))

	message := "GET\n/test\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", "not-a-number-or-rfc3339")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "invalid timestamp format") {
		t.Errorf("Expected 'invalid timestamp format' in body, got %q", body)
	}
}

func TestHMAC_TimestampExpired(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with expired timestamp")
	}))

	message := "GET\n/test\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	// Timestamp 10 minutes ago — beyond the 5m MaxAge
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "request timestamp expired") {
		t.Errorf("Expected 'request timestamp expired' in body, got %q", body)
	}
}

func TestHMAC_TimestampFutureExpired(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with far-future timestamp")
	}))

	message := "GET\n/test\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	// Timestamp 10 minutes in the future — beyond the 5m MaxAge (abs diff)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "request timestamp expired") {
		t.Errorf("Expected 'request timestamp expired' in body, got %q", body)
	}
}

func TestHMAC_TimestampValidUnix(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	ts := fmt.Sprintf("%d", time.Now().Add(-30*time.Second).Unix())

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Message must include the timestamp value
	message := "GET\n/test\n" + ts + "\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", ts)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid timestamp")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestHMAC_TimestampValidRFC3339(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	ts := time.Now().Add(-1 * time.Minute).Format(time.RFC3339)

	called := false
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Message must include the timestamp value
	message := "GET\n/test\n" + ts + "\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", ts)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called with valid RFC3339 timestamp")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestHMAC_TimestampInvalidMaxAge(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "not-a-duration"

	// Invalid MaxAge should fail at construction time, not request time
	mw, err := New(config)
	if err == nil {
		t.Error("Expected error for invalid MaxAge")
	}
	if mw != nil {
		t.Error("Expected nil middleware for invalid MaxAge")
	}
}

func TestHMAC_Wrap_ComputeSignatureError(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.UseBody = true

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when computeSignature fails")
	}))

	// Use a body reader that will error on Read
	req := httptest.NewRequest("POST", "/test", io.NopCloser(&errorReader{}))
	req.Header.Set("X-Signature", "any-signature-will-do")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "failed to compute signature") {
		t.Errorf("Expected 'failed to compute signature' in body, got %q", body)
	}
}

func TestHMAC_ZeroSecrets(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Verify middleware is functional before zeroing
	if mw.config.Secret != "secret" {
		t.Fatal("expected secret to be set")
	}
	if !mw.config.Enabled {
		t.Fatal("expected enabled to be true")
	}

	mw.ZeroSecrets()

	if mw.config.Secret != "" {
		t.Errorf("expected secret to be cleared, got %q", mw.config.Secret)
	}
	if mw.config.Enabled {
		t.Error("expected enabled to be false after zeroing")
	}
}

func TestHMAC_ZeroSecrets_Idempotent(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, _ := New(config)
	mw.ZeroSecrets()
	mw.ZeroSecrets() // Should not panic
	if mw.config.Secret != "" {
		t.Error("expected empty secret after double zero")
	}
}

// --------------------------------------------------------------------------
// Security regression tests
// --------------------------------------------------------------------------

func TestHMAC_EmptySecretRejected(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = ""

	mw, err := New(config)
	if err == nil {
		t.Error("Expected error when secret is empty and enabled")
	}
	if mw != nil {
		t.Error("Expected nil middleware when secret is empty")
	}
}

func TestHMAC_TimestampInSignature_ReplayProtection(t *testing.T) {
	// Verify that changing the timestamp invalidates the signature
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.TimestampHeader = "X-Timestamp"
	config.MaxAge = "5m"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with altered timestamp")
	}))

	// Sign with timestamp T1
	ts1 := fmt.Sprintf("%d", time.Now().Add(-30*time.Second).Unix())
	message := "GET\n/test\n" + ts1 + "\n"
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	// Replay with a different timestamp T2 — signature should fail
	ts2 := fmt.Sprintf("%d", time.Now().Unix())
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", ts2)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 when timestamp is changed, got %d", rec.Code)
	}
}

func TestHMAC_BodyOverLimitRejected(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.UseBody = true

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when body exceeds limit")
	}))

	// Create a body just over 10MB
	largeBody := strings.NewReader(strings.Repeat("x", 10*1024*1024+1))
	req := httptest.NewRequest("POST", "/test", largeBody)
	req.Header.Set("X-Signature", "any-signature")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 when body exceeds limit, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "failed to compute signature") {
		t.Errorf("Expected 'failed to compute signature' in body, got %q", body)
	}
}

func TestHMAC_ModifiedBodyDetected(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"
	config.UseBody = true

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called with modified body")
	}))

	// Sign original body
	originalBody := `{"amount":100}`
	message := "POST\n/test\n" + originalBody
	sig, _ := GenerateSignature("secret", message, "sha256", "hex")

	// Send modified body
	modifiedBody := `{"amount":99999}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(modifiedBody))
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 when body is modified, got %d", rec.Code)
	}
}

func TestHMAC_ErrorResponseJSON(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Secret = "secret"

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check Content-Type is application/json
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", ct)
	}

	// Check body is valid JSON
	body := rec.Body.String()
	if !strings.Contains(body, `"error"`) || !strings.Contains(body, `"message"`) {
		t.Errorf("Expected JSON error body, got %q", body)
	}
}
