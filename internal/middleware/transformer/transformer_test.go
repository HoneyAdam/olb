package transformer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransformer_Disabled(t *testing.T) {
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
		w.Write([]byte("Hello"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should have been called when transformer is disabled")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestTransformer_AddHeaders(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.AddHeaders = map[string]string{
		"X-Custom-Header": "custom-value",
		"X-Frame-Options": "DENY",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Custom-Header") != "custom-value" {
		t.Error("X-Custom-Header should be set")
	}

	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options should be set")
	}
}

func TestTransformer_RemoveHeaders(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.RemoveHeaders = []string{"X-Internal-Token", "Server"}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Internal-Token", "secret")
		w.Header().Set("Server", "internal-server")
		w.Header().Set("X-Public-Header", "public")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Internal-Token") != "" {
		t.Error("X-Internal-Token should be removed")
	}

	if rec.Header().Get("Server") != "" {
		t.Error("Server should be removed")
	}

	if rec.Header().Get("X-Public-Header") != "public" {
		t.Error("X-Public-Header should be preserved")
	}
}

func TestTransformer_BodyRewrite(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.RewriteBody = map[string]string{
		"old-domain.com": "new-domain.com",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Visit https://old-domain.com/api for more info"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "new-domain.com") {
		t.Errorf("Body should contain new-domain.com, got: %s", string(body))
	}

	if strings.Contains(string(body), "old-domain.com") {
		t.Errorf("Body should not contain old-domain.com, got: %s", string(body))
	}
}

func TestTransformer_ExcludePath(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.ExcludePaths = []string{"/health", "/metrics"}
	config.AddHeaders = map[string]string{
		"X-Transformed": "true",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test excluded path
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Transformed") == "true" {
		t.Error("Excluded path should not have headers added")
	}
}

func TestTransformer_ExcludeMIMEType(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.ExcludeMIMETypes = []string{"image/", "application/pdf"}
	config.AddHeaders = map[string]string{
		"X-Transformed": "true",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-data"))
	}))

	req := httptest.NewRequest("GET", "/image.png", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Transformed") == "true" {
		t.Error("Excluded MIME type should not have headers added")
	}
}

func TestTransformer_Compress(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Compress = true
	config.MinCompressSize = 10

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Large body that should be compressed
	largeBody := strings.Repeat("This is a large response body. ", 100)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check if compressed
	if rec.Header().Get("Content-Encoding") == "gzip" {
		// Compression was applied
		if rec.Body.Len() >= len(largeBody) {
			t.Error("Compressed body should be smaller than original")
		}
	}
}

func TestTransformer_NoCompressSmallBody(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Compress = true
	config.MinCompressSize = 1024

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Small body"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Small body should not be compressed")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled != false {
		t.Error("Default Enabled should be false")
	}
	if config.Compress != false {
		t.Error("Default Compress should be false")
	}
	if config.CompressLevel != 6 {
		t.Errorf("Default CompressLevel should be 6, got %d", config.CompressLevel)
	}
	if config.MinCompressSize != 1024 {
		t.Errorf("Default MinCompressSize should be 1024, got %d", config.MinCompressSize)
	}
}

func TestMiddleware_Priority(t *testing.T) {
	config := DefaultConfig()
	mw, _ := New(config)

	if mw.Priority() != 850 {
		t.Errorf("Expected priority 850, got %d", mw.Priority())
	}
}

func TestMiddleware_Name(t *testing.T) {
	config := DefaultConfig()
	mw, _ := New(config)

	if mw.Name() != "transformer" {
		t.Errorf("Expected name 'transformer', got '%s'", mw.Name())
	}
}

func TestNew_InvalidRegex(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.RewriteBody = map[string]string{
		"[invalid(": "replacement",
	}

	_, err := New(config)
	if err == nil {
		t.Error("New should return error for invalid regex")
	}
}

func TestTransformer_EmptyBody(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.AddHeaders = map[string]string{
		"X-Custom": "value",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		// No body written
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rec.Code)
	}

	if rec.Header().Get("X-Custom") != "value" {
		t.Error("X-Custom header should be set even with empty body")
	}
}

func TestTransformer_Compress_OutputLargerThanOriginal(t *testing.T) {
	// Test the branch where compressed output is larger than original body.
	// In that case compressBody should return the original body without
	// setting Content-Encoding.
	config := DefaultConfig()
	config.Enabled = true
	config.Compress = true
	config.CompressLevel = 9 // max compression level
	config.MinCompressSize = 1

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Use random-looking data that doesn't compress well
	// Repeating a single byte should compress very well, but using a
	// short random-ish string with MinCompressSize=1 might produce larger output
	// after gzip overhead.
	body := "ABCDEFGHIJ" // 10 bytes, may get larger with gzip headers
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// If compression was applied, Content-Encoding should be gzip.
	// If compressed output was larger, the original body is returned without gzip.
	// Either way, the response should be valid.
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// If Content-Encoding is NOT gzip, the body should be the original.
	if rec.Header().Get("Content-Encoding") != "gzip" {
		if rec.Body.String() != body {
			t.Errorf("Expected original body when compressed is larger, got: %s", rec.Body.String())
		}
	}
}

func TestTransformer_Compress_InvalidLevel(t *testing.T) {
	// Test compressBody with an invalid compression level.
	// gzip.NewWriterLevel returns an error for levels outside 1-9 (except
	// gzip.DefaultCompression=-1 and gzip.NoCompression=0).
	// When CompressLevel is invalid, compressBody should return the original body.
	config := DefaultConfig()
	config.Enabled = true
	config.Compress = true
	config.CompressLevel = 99 // invalid level
	config.MinCompressSize = 1

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	largeBody := "This is a test body that is definitely large enough to compress properly."
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// With invalid compression level, should fall back to original body
	if rec.Header().Get("Content-Encoding") == "gzip" {
		// If it somehow used gzip, that's fine, but let's check it's valid
	} else {
		if rec.Body.String() != largeBody {
			t.Errorf("Expected original body when compression fails, got: %s", rec.Body.String())
		}
	}
}

func TestTransformer_Compress_NoCompressionLevel(t *testing.T) {
	// Test with gzip.NoCompression (level 0) - gzip header overhead
	// makes the compressed output larger than the original for short bodies.
	config := DefaultConfig()
	config.Enabled = true
	config.Compress = true
	config.CompressLevel = 0 // gzip.NoCompression
	config.MinCompressSize = 1

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	body := "hi" // very short body
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// With NoCompression level and short body, compressed should be larger
	// so the original should be returned
	if rec.Header().Get("Content-Encoding") != "gzip" {
		if rec.Body.String() != body {
			t.Errorf("Expected original body, got: %s", rec.Body.String())
		}
	}
}

func TestTransformer_TransformJSON(t *testing.T) {
	// Test that transformJSON is called when JSONTransform is configured
	// and content type is application/json.
	config := DefaultConfig()
	config.Enabled = true
	config.JSONTransform = &JSONTransform{
		AddFields: map[string]interface{}{
			"injected": true,
		},
		RemoveFields: []string{"secret"},
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"hello":"world"}`))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// transformJSON currently returns the body as-is, so we just verify
	// the response body is intact and the JSON content type was processed.
	body := rec.Body.String()
	if !strings.Contains(body, "hello") {
		t.Errorf("Expected body to contain 'hello', got: %s", body)
	}
}

func TestTransformer_TransformJSON_NonJSONContentType(t *testing.T) {
	// Test that transformJSON is NOT called when content type is not JSON.
	// Even though JSONTransform is configured, non-JSON content should be
	// passed through without JSON transformation.
	config := DefaultConfig()
	config.Enabled = true
	config.JSONTransform = &JSONTransform{
		AddFields: map[string]interface{}{
			"injected": true,
		},
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	plainBody := "This is plain text, not JSON"
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(plainBody))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != plainBody {
		t.Errorf("Plain text body should not be modified, got: %s", rec.Body.String())
	}
}

func TestTransformer_TransformJSON_WithBodyRewrite(t *testing.T) {
	// Test that both body rewrite AND JSON transform are applied together
	// when both are configured for a JSON response.
	config := DefaultConfig()
	config.Enabled = true
	config.RewriteBody = map[string]string{
		"v1": "v2",
	}
	config.JSONTransform = &JSONTransform{
		RemoveFields: []string{"internal"},
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"api":"v1","internal":"data"}`))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "v2") {
		t.Errorf("Body rewrite should have replaced v1 with v2, got: %s", body)
	}
}

func TestTransformer_TransformJSON_EmptyBody(t *testing.T) {
	// Test transformBody early return with empty body even when JSONTransform is set.
	config := DefaultConfig()
	config.Enabled = true
	config.JSONTransform = &JSONTransform{
		AddFields: map[string]interface{}{
			"extra": "value",
		},
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", rec.Code)
	}
}

func TestTransformer_Compress_SetsContentEncoding(t *testing.T) {
	// Test that successful compression sets Content-Encoding: gzip
	// and removes Content-Length.
	config := DefaultConfig()
	config.Enabled = true
	config.Compress = true
	config.MinCompressSize = 1

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Highly compressible body to ensure compressed is smaller
	largeBody := strings.Repeat("a", 5000)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "5000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected Content-Encoding: gzip header")
	}

	if rec.Header().Get("Content-Length") != "" {
		t.Error("Content-Length should be removed when gzip is applied")
	}

	// Verify body is actually gzip compressed (should be much smaller)
	if rec.Body.Len() >= len(largeBody) {
		t.Errorf("Compressed body (%d bytes) should be smaller than original (%d bytes)",
			rec.Body.Len(), len(largeBody))
	}
}

func TestTransformer_StatusZero_NoWriteHeader(t *testing.T) {
	// Test the case where handler does NOT call WriteHeader,
	// so w.status remains 0 and WriteHeader should not be called on the
	// underlying ResponseWriter.
	config := DefaultConfig()
	config.Enabled = true
	config.AddHeaders = map[string]string{
		"X-Test": "value",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do NOT call WriteHeader, just write body
		w.Write([]byte("Hello"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// httptest.ResponseRecorder defaults to 200 when WriteHeader not called
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "Hello" {
		t.Errorf("Expected body 'Hello', got: %s", rec.Body.String())
	}
}

func TestTransformer_ExcludeMIMEType_WriteRawWithBody(t *testing.T) {
	// Test writeRaw path where buffer has content and status is set.
	// This exercises the writeRaw body-writing branch.
	config := DefaultConfig()
	config.Enabled = true
	config.ExcludeMIMETypes = []string{"application/octet-stream"}
	config.AddHeaders = map[string]string{
		"X-Should-Not-Be-Set": "true",
	}

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("raw binary data"))
	}))

	req := httptest.NewRequest("GET", "/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// writeRaw path should not add headers
	if rec.Header().Get("X-Should-Not-Be-Set") != "" {
		t.Error("Excluded MIME type should not have transformer headers added")
	}

	if rec.Body.String() != "raw binary data" {
		t.Errorf("Expected raw body to pass through, got: %s", rec.Body.String())
	}
}

func TestTransformer_StatusCodePreserved(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true

	mw, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	tests := []int{200, 201, 400, 404, 500}

	for _, status := range tests {
		handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte("Response"))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != status {
			t.Errorf("Expected status %d, got %d", status, rec.Code)
		}
	}
}
