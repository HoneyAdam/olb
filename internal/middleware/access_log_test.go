package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/logging"
)

func TestAccessLogMiddleware_Name(t *testing.T) {
	m := NewAccessLogMiddleware(AccessLogConfig{})
	if m.Name() != "access-log" {
		t.Errorf("expected name 'access-log', got %s", m.Name())
	}
}

func TestAccessLogMiddleware_Priority(t *testing.T) {
	m := NewAccessLogMiddleware(AccessLogConfig{})
	if m.Priority() != PriorityAccessLog {
		t.Errorf("expected priority %d, got %d", PriorityAccessLog, m.Priority())
	}
}

func TestAccessLogMiddleware_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	config := AccessLogConfig{
		Format: AccessLogFormatJSON,
		Output: &buf,
	}
	m := NewAccessLogMiddleware(config)

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test/path?page=1", nil)
	req.Header.Set("User-Agent", "Test-Agent")
	req.Header.Set("Referer", "https://example.com")
	req.Host = "api.example.com"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Wait for async log write
	time.Sleep(100 * time.Millisecond)

	// Parse JSON log
	logLine := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLine), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}

	// Verify fields
	if logEntry["method"] != "GET" {
		t.Errorf("expected method GET, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/test/path" {
		t.Errorf("expected path /test/path, got %v", logEntry["path"])
	}
	if logEntry["query"] != "page=1" {
		t.Errorf("expected query page=1, got %v", logEntry["query"])
	}
	if logEntry["host"] != "api.example.com" {
		t.Errorf("expected host api.example.com, got %v", logEntry["host"])
	}
	if logEntry["user_agent"] != "Test-Agent" {
		t.Errorf("expected user_agent Test-Agent, got %v", logEntry["user_agent"])
	}
	if logEntry["referer"] != "https://example.com" {
		t.Errorf("expected referer https://example.com, got %v", logEntry["referer"])
	}
	if logEntry["status"] != float64(200) {
		t.Errorf("expected status 200, got %v", logEntry["status"])
	}
	if logEntry["bytes_out"] != float64(13) { // len("Hello, World!")
		t.Errorf("expected bytes_out 13, got %v", logEntry["bytes_out"])
	}

	// Verify timestamp format
	timestamp, ok := logEntry["timestamp"].(string)
	if !ok {
		t.Error("timestamp should be a string")
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		t.Errorf("timestamp should be RFC3339 format: %v", err)
	}

	// Verify duration_ms exists and is positive
	duration, ok := logEntry["duration_ms"].(float64)
	if !ok {
		t.Error("duration_ms should be a number")
	}
	if duration < 0 {
		t.Error("duration_ms should be positive")
	}
}

func TestAccessLogMiddleware_CLFFormat(t *testing.T) {
	var buf bytes.Buffer
	config := AccessLogConfig{
		Format: AccessLogFormatCLF,
		Output: &buf,
	}
	m := NewAccessLogMiddleware(config)

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.Proto = "HTTP/1.1"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Wait for async log write
	time.Sleep(100 * time.Millisecond)

	logLine := buf.String()

	// CLF format: host ident authuser date request status bytes
	// Example: 192.0.2.1 - - [14/Mar/2026:10:30:00 +0000] "POST /api/users HTTP/1.1" 200 5
	expectedParts := []string{
		" - - [", // ident and authuser are always "-"
		"] \"POST /api/users HTTP/1.1\" 200 5",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logLine, part) {
			t.Errorf("expected log to contain %q, got: %s", part, logLine)
		}
	}
}

func TestAccessLogMiddleware_SkipPaths(t *testing.T) {
	var buf bytes.Buffer
	config := AccessLogConfig{
		Format:    AccessLogFormatCLF,
		Output:    &buf,
		SkipPaths: []string{"/health", "/metrics"},
	}
	m := NewAccessLogMiddleware(config)

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test skipped path
	req1 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	time.Sleep(50 * time.Millisecond)

	if buf.Len() != 0 {
		t.Errorf("expected no log for /health, got: %s", buf.String())
	}

	// Test skipped path with subpath
	req2 := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	time.Sleep(50 * time.Millisecond)

	if buf.Len() != 0 {
		t.Errorf("expected no log for /health/live, got: %s", buf.String())
	}

	// Test non-skipped path
	req3 := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	time.Sleep(50 * time.Millisecond)

	if buf.Len() == 0 {
		t.Error("expected log for /api/users, got none")
	}
}

func TestAccessLogMiddleware_CustomOutput(t *testing.T) {
	var buf bytes.Buffer
	config := AccessLogConfig{
		Format: AccessLogFormatCLF,
		Output: &buf,
	}
	m := NewAccessLogMiddleware(config)

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	time.Sleep(50 * time.Millisecond)

	if buf.Len() == 0 {
		t.Error("expected log output, got none")
	}
}

func TestAccessLogMiddleware_WithLogger(t *testing.T) {
	var logBuf bytes.Buffer
	output := logging.NewJSONOutput(&logBuf)
	logger := logging.New(output)

	config := AccessLogConfig{
		Format: AccessLogFormatJSON,
		Logger: logger,
	}
	m := NewAccessLogMiddleware(config)

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	time.Sleep(100 * time.Millisecond)

	logOutput := logBuf.String()
	if logOutput == "" {
		t.Error("expected log output via logger, got none")
	}

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logOutput), &logEntry); err != nil {
		t.Errorf("log output should be valid JSON: %v", err)
	}
}

func TestAccessLogMiddleware_Defaults(t *testing.T) {
	// Test default format is CLF
	m := NewAccessLogMiddleware(AccessLogConfig{})
	if m.config.Format != AccessLogFormatCLF {
		t.Errorf("expected default format CLF, got %s", m.config.Format)
	}

	// Test default output is os.Stdout
	if m.config.Output == nil {
		t.Error("expected default output to be set")
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with"quotes"`, `with\"quotes\"`},
		{`with\backslash`, `with\\backslash`},
		{"with\nnewline", "with\\nnewline"},
		{"with\ttab", "with\\ttab"},
		{"with\rreturn", "with\\rreturn"},
	}

	for _, tt := range tests {
		result := escapeJSON(tt.input)
		if result != tt.expected {
			t.Errorf("escapeJSON(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestAccessLogMiddleware_StatusCodes(t *testing.T) {
	var buf bytes.Buffer
	config := AccessLogConfig{
		Format: AccessLogFormatJSON,
		Output: &buf,
	}
	m := NewAccessLogMiddleware(config)

	testCases := []struct {
		status   int
		expected int
	}{
		{http.StatusOK, 200},
		{http.StatusCreated, 201},
		{http.StatusBadRequest, 400},
		{http.StatusNotFound, 404},
		{http.StatusInternalServerError, 500},
	}

	for _, tc := range testCases {
		buf.Reset()

		handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		time.Sleep(50 * time.Millisecond)

		logLine := strings.TrimSpace(buf.String())
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLine), &logEntry); err != nil {
			t.Fatalf("failed to parse JSON log for status %d: %v", tc.status, err)
		}

		if logEntry["status"] != float64(tc.expected) {
			t.Errorf("status %d: expected status %v, got %v", tc.status, tc.expected, logEntry["status"])
		}
	}
}

func TestAccessLogMiddleware_BytesTracking(t *testing.T) {
	var buf bytes.Buffer
	config := AccessLogConfig{
		Format: AccessLogFormatJSON,
		Output: &buf,
	}
	m := NewAccessLogMiddleware(config)

	responseBody := "Hello, World! This is a test response."
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	time.Sleep(50 * time.Millisecond)

	logLine := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLine), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}

	expectedBytes := float64(len(responseBody))
	if logEntry["bytes_out"] != expectedBytes {
		t.Errorf("expected bytes_out %v, got %v", expectedBytes, logEntry["bytes_out"])
	}
}
