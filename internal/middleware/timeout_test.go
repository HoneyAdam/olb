package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTimeoutMiddleware_Defaults(t *testing.T) {
	mw := NewTimeoutMiddleware(TimeoutConfig{})
	if mw.config.Timeout != 60*time.Second {
		t.Errorf("expected default timeout 60s, got %v", mw.config.Timeout)
	}
	if mw.config.Message != "request timeout" {
		t.Errorf("expected default message 'request timeout', got %q", mw.config.Message)
	}
}

func TestTimeoutMiddleware_Name(t *testing.T) {
	mw := NewTimeoutMiddleware(DefaultTimeoutConfig())
	if mw.Name() != "timeout" {
		t.Errorf("expected name 'timeout', got %q", mw.Name())
	}
}

func TestTimeoutMiddleware_Priority(t *testing.T) {
	mw := NewTimeoutMiddleware(DefaultTimeoutConfig())
	if mw.Priority() != 450 {
		t.Errorf("expected priority 450, got %d", mw.Priority())
	}
}

func TestTimeoutMiddleware_NormalRequest(t *testing.T) {
	mw := NewTimeoutMiddleware(TimeoutConfig{Timeout: 5 * time.Second})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got %q", rec.Body.String())
	}
}

func TestTimeoutMiddleware_SlowRequest(t *testing.T) {
	mw := NewTimeoutMiddleware(TimeoutConfig{
		Timeout: 50 * time.Millisecond,
		Message: "too slow",
	})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow handler
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))

	req := httptest.NewRequest("GET", "/slow", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "too slow") {
		t.Errorf("expected body to contain 'too slow', got %q", rec.Body.String())
	}
}

func TestTimeoutMiddleware_HandlerWritesBeforeTimeout(t *testing.T) {
	mw := NewTimeoutMiddleware(TimeoutConfig{
		Timeout: 50 * time.Millisecond,
	})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
		// Simulate slow cleanup after writing
		time.Sleep(100 * time.Millisecond)
	}))

	req := httptest.NewRequest("POST", "/create", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Handler wrote first, so we should see its response
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}
