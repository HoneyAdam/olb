package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	mw := NewRecoveryMiddleware(RecoveryConfig{})
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRecoveryMiddleware_PanicRecovered(t *testing.T) {
	var loggedPanic interface{}
	var loggedStack string

	mw := NewRecoveryMiddleware(RecoveryConfig{
		LogFunc: func(panicVal interface{}, stack string) {
			loggedPanic = panicVal
			loggedStack = stack
		},
	})

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()

	// Should NOT panic — middleware catches it
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Errorf("expected error body, got %q", rec.Body.String())
	}
	if loggedPanic != "test panic" {
		t.Errorf("expected panic value 'test panic', got %v", loggedPanic)
	}
	if loggedStack == "" {
		t.Error("expected non-empty stack trace")
	}
}

func TestRecoveryMiddleware_Name(t *testing.T) {
	mw := NewRecoveryMiddleware(RecoveryConfig{})
	if mw.Name() != "recovery" {
		t.Errorf("expected name 'recovery', got %q", mw.Name())
	}
}

func TestRecoveryMiddleware_Priority(t *testing.T) {
	mw := NewRecoveryMiddleware(RecoveryConfig{})
	if mw.Priority() != 1 {
		t.Errorf("expected priority 1, got %d", mw.Priority())
	}
}
