package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewBodyLimitMiddleware_Defaults(t *testing.T) {
	mw := NewBodyLimitMiddleware(BodyLimitConfig{})
	if mw.config.MaxSize != 10*1024*1024 {
		t.Errorf("expected default max size 10MB, got %d", mw.config.MaxSize)
	}
}

func TestBodyLimitMiddleware_Name(t *testing.T) {
	mw := NewBodyLimitMiddleware(DefaultBodyLimitConfig())
	if mw.Name() != "body_limit" {
		t.Errorf("expected name 'body_limit', got %q", mw.Name())
	}
}

func TestBodyLimitMiddleware_Priority(t *testing.T) {
	mw := NewBodyLimitMiddleware(DefaultBodyLimitConfig())
	if mw.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", mw.Priority())
	}
}

func TestBodyLimitMiddleware_SmallBody(t *testing.T) {
	mw := NewBodyLimitMiddleware(BodyLimitConfig{MaxSize: 1024})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.Repeat([]byte("a"), 100)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Length", "100")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_OversizedContentLength(t *testing.T) {
	mw := NewBodyLimitMiddleware(BodyLimitConfig{MaxSize: 1024})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.Repeat([]byte("a"), 2048)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.ContentLength = 2048
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_NoBody(t *testing.T) {
	mw := NewBodyLimitMiddleware(BodyLimitConfig{MaxSize: 1024})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET with no body, got %d", rec.Code)
	}
}

func TestBodyLimitMiddleware_MaxBytesReaderEnforcement(t *testing.T) {
	mw := NewBodyLimitMiddleware(BodyLimitConfig{MaxSize: 64})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to read more than allowed
		buf := make([]byte, 256)
		_, err := r.Body.Read(buf)
		if err != nil && strings.Contains(err.Error(), "http: request body too large") {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// ContentLength is 0 (unknown) but body is bigger than limit
	body := bytes.Repeat([]byte("x"), 256)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.ContentLength = -1 // unknown content length
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 from MaxBytesReader, got %d", rec.Code)
	}
}
