package detection

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRequestContext_BasicGET(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/path?key=value&foo=bar", nil)
	req.Header.Set("X-Custom", "test-header")
	req.RemoteAddr = "192.168.1.1:12345"

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	if ctx.Method != "GET" {
		t.Errorf("expected method GET, got %q", ctx.Method)
	}
	if ctx.Path != "/path" {
		t.Errorf("expected path /path, got %q", ctx.Path)
	}
	if ctx.RemoteIP != "192.168.1.1" {
		t.Errorf("expected remote IP 192.168.1.1, got %q", ctx.RemoteIP)
	}
	if ctx.DecodedPath != "/path" {
		t.Errorf("expected decoded path /path, got %q", ctx.DecodedPath)
	}
	if ctx.Request != req {
		t.Error("expected Request to be set")
	}
}

func TestNewRequestContext_WithCookies(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "pref", Value: "dark"})

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	if ctx.Cookies["session"] != "abc123" {
		t.Errorf("expected cookie 'session'='abc123', got %q", ctx.Cookies["session"])
	}
	if ctx.Cookies["pref"] != "dark" {
		t.Errorf("expected cookie 'pref'='dark', got %q", ctx.Cookies["pref"])
	}
}

func TestNewRequestContext_WithFormBody(t *testing.T) {
	body := "username=admin&password=secret"
	req := httptest.NewRequest("POST", "http://example.com/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	if ctx.BodyParams["username"] != "admin" {
		t.Errorf("expected param 'username'='admin', got %q", ctx.BodyParams["username"])
	}
	if ctx.BodyParams["password"] != "secret" {
		t.Errorf("expected param 'password'='secret', got %q", ctx.BodyParams["password"])
	}
	if ctx.DecodedBody != body {
		t.Errorf("expected decoded body %q, got %q", body, ctx.DecodedBody)
	}
}

func TestNewRequestContext_WithJSONBody(t *testing.T) {
	body := `{"user":"admin","nested":{"key":"val"},"list":["a","b"]}`
	req := httptest.NewRequest("POST", "http://example.com/api", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	if ctx.BodyParams["user"] != "admin" {
		t.Errorf("expected param 'user'='admin', got %q", ctx.BodyParams["user"])
	}
	if ctx.BodyParams["nested.key"] != "val" {
		t.Errorf("expected param 'nested.key'='val', got %q", ctx.BodyParams["nested.key"])
	}
}

func TestNewRequestContext_NilBody(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Body = nil

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	if len(ctx.Body) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(ctx.Body))
	}
}

func TestNewRequestContext_EncodedPath(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/foo%20bar?q=%3Cscript%3E", nil)

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	// Decoded query should have the unescaped version
	if !strings.Contains(ctx.DecodedQuery, "<script>") {
		t.Errorf("expected decoded query to contain '<script>', got %q", ctx.DecodedQuery)
	}
}

func TestAllInputs(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/path?q=test", nil)
	req.Header.Set("X-Test", "header-val")
	req.AddCookie(&http.Cookie{Name: "sid", Value: "cookie-val"})

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	fields := ctx.AllInputs()
	if len(fields) == 0 {
		t.Fatal("expected non-empty AllInputs")
	}

	// Check that path is included
	foundPath := false
	foundQuery := false
	foundHeader := false
	foundCookie := false
	for _, f := range fields {
		if f.Location == "path" {
			foundPath = true
		}
		if f.Location == "query" {
			foundQuery = true
		}
		if strings.HasPrefix(f.Location, "header:") {
			foundHeader = true
		}
		if strings.HasPrefix(f.Location, "cookie:") {
			foundCookie = true
		}
	}

	if !foundPath {
		t.Error("expected path in AllInputs")
	}
	if !foundQuery {
		t.Error("expected query in AllInputs")
	}
	if !foundHeader {
		t.Error("expected header in AllInputs")
	}
	if !foundCookie {
		t.Error("expected cookie in AllInputs")
	}
}

func TestAllInputs_WithBody(t *testing.T) {
	body := "username=admin"
	req := httptest.NewRequest("POST", "http://example.com/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	fields := ctx.AllInputs()
	foundBody := false
	foundParam := false
	for _, f := range fields {
		if f.Location == "body" {
			foundBody = true
		}
		if strings.HasPrefix(f.Location, "param:") {
			foundParam = true
		}
	}

	if !foundBody {
		t.Error("expected body in AllInputs")
	}
	if !foundParam {
		t.Error("expected param in AllInputs")
	}
}

func TestFlattenJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]string
	}{
		{
			name:     "simple string",
			input:    map[string]any{"key": "value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "nested object",
			input:    map[string]any{"outer": map[string]any{"inner": "val"}},
			expected: map[string]string{"outer.inner": "val"},
		},
		{
			name:     "numeric skipped",
			input:    map[string]any{"num": float64(42), "str": "hello"},
			expected: map[string]string{"str": "hello"},
		},
		{
			name:     "array of strings",
			input:    map[string]any{"list": []any{"a", "b"}},
			expected: map[string]string{"list.": "a", "list.0": "b"},
		},
		{
			name:     "deeply nested",
			input:    map[string]any{"a": map[string]any{"b": map[string]any{"c": "deep"}}},
			expected: map[string]string{"a.b.c": "deep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := make(map[string]string)
			flattenJSON("", tt.input, out)
			for k, v := range tt.expected {
				if out[k] != v {
					t.Errorf("expected out[%q] = %q, got %q", k, v, out[k])
				}
			}
		})
	}
}

func TestFlattenJSON_WithPrefix(t *testing.T) {
	out := make(map[string]string)
	flattenJSON("root", map[string]any{"key": "val"}, out)
	if out["root.key"] != "val" {
		t.Errorf("expected out['root.key'] = 'val', got %q", out["root.key"])
	}
}

func TestSetOnRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	req2 := ctx.SetOnRequest(req)
	retrieved := GetRequestContext(req2)
	if retrieved == nil {
		t.Fatal("expected non-nil RequestContext from GetRequestContext")
	}
	if retrieved != ctx {
		t.Error("expected same RequestContext back from GetRequestContext")
	}
}

func TestGetRequestContext_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	ctx := GetRequestContext(req)
	if ctx != nil {
		t.Error("expected nil for request without context")
	}
}

func TestReleaseRequestContext_Nil(t *testing.T) {
	// Should not panic
	ReleaseRequestContext(nil)
}

func TestReleaseRequestContext_ClearsFields(t *testing.T) {
	req := httptest.NewRequest("POST", "http://example.com/", strings.NewReader("body"))
	ctx := NewRequestContext(req)

	// Verify fields are set
	if ctx.Request == nil {
		t.Error("expected Request to be set before release")
	}

	ReleaseRequestContext(ctx)

	// After release, fields should be cleared
	if ctx.Request != nil {
		t.Error("expected Request to be nil after release")
	}
	if ctx.Body != nil {
		t.Error("expected Body to be nil after release")
	}
	if ctx.Headers != nil {
		t.Error("expected Headers to be nil after release")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"10.0.0.1:0", "10.0.0.1"},
		{"[::1]:443", "::1"},
		{"plain-addr", "plain-addr"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ExtractIP(tt.addr)
		if got != tt.expected {
			t.Errorf("ExtractIP(%q) = %q, want %q", tt.addr, got, tt.expected)
		}
	}
}

func TestNewRequestContext_BodyRestored(t *testing.T) {
	original := "test body content"
	req := httptest.NewRequest("POST", "http://example.com/", strings.NewReader(original))

	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	// Body should be read into ctx
	if string(ctx.Body) != original {
		t.Errorf("expected body %q, got %q", original, string(ctx.Body))
	}

	// Body should be restored on the request for downstream
	var buf bytes.Buffer
	buf.ReadFrom(req.Body)
	if buf.String() != original {
		t.Errorf("expected restored body %q, got %q", original, buf.String())
	}
}

func TestNewRequestContext_Whitelisted(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	ctx := NewRequestContext(req)
	defer ReleaseRequestContext(ctx)

	if ctx.IsWhitelisted {
		t.Error("expected IsWhitelisted to be false initially")
	}
	if ctx.JA3Hash != "" {
		t.Error("expected JA3Hash to be empty initially")
	}
}
