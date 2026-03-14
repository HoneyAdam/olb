package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestGenerateUUID(t *testing.T) {
	// Test UUID format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx where y is 8,9,a, or b
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	for i := 0; i < 100; i++ {
		uuid := generateUUID()
		if !uuidPattern.MatchString(uuid) {
			t.Errorf("generateUUID() = %q, want valid UUID v4 format", uuid)
		}
	}
}

func TestGenerateUUID_Uniqueness(t *testing.T) {
	// Generate multiple UUIDs and ensure they're unique
	uuids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		uuid := generateUUID()
		if uuids[uuid] {
			t.Errorf("generateUUID() produced duplicate: %s", uuid)
		}
		uuids[uuid] = true
	}
}

func TestRequestIDMiddleware_Name(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{})
	if got := m.Name(); got != "request-id" {
		t.Errorf("Name() = %q, want %q", got, "request-id")
	}
}

func TestRequestIDMiddleware_Priority(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{})
	if got := m.Priority(); got != 400 {
		t.Errorf("Priority() = %d, want %d", got, 400)
	}
}

func TestRequestIDMiddleware_GeneratesNewID(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{
		Generate:      true,
		TrustIncoming: false,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Check that response has a request ID header
	requestID := rr.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Error("Expected X-Request-Id header to be set")
	}

	// Verify it's a valid UUID
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(requestID) {
		t.Errorf("X-Request-Id = %q, want valid UUID v4 format", requestID)
	}
}

func TestRequestIDMiddleware_TrustIncoming(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{
		Generate:      true,
		TrustIncoming: true,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "existing-request-id")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should preserve the existing request ID
	requestID := rr.Header().Get("X-Request-Id")
	if requestID != "existing-request-id" {
		t.Errorf("X-Request-Id = %q, want %q", requestID, "existing-request-id")
	}
}

func TestRequestIDMiddleware_TrustIncoming_EmptyHeader(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{
		Generate:      true,
		TrustIncoming: true,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Request-Id header set
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should generate a new request ID
	requestID := rr.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Error("Expected X-Request-Id header to be set when incoming is empty")
	}

	// Verify it's a valid UUID (generated)
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(requestID) {
		t.Errorf("X-Request-Id = %q, want valid UUID v4 format", requestID)
	}
}

func TestRequestIDMiddleware_DontTrustIncoming(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{
		Generate:      true,
		TrustIncoming: false,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "existing-request-id")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should generate a new request ID, ignoring the incoming one
	requestID := rr.Header().Get("X-Request-Id")
	if requestID == "existing-request-id" {
		t.Error("Expected X-Request-Id to be regenerated, not use incoming")
	}

	// Verify it's a valid UUID (generated)
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(requestID) {
		t.Errorf("X-Request-Id = %q, want valid UUID v4 format", requestID)
	}
}

func TestRequestIDMiddleware_NoGenerate(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{
		Generate:      false,
		TrustIncoming: true,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Request-Id header set
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should not set a request ID
	requestID := rr.Header().Get("X-Request-Id")
	if requestID != "" {
		t.Errorf("X-Request-Id = %q, want empty", requestID)
	}
}

func TestRequestIDMiddleware_CustomHeaderName(t *testing.T) {
	m := NewRequestIDMiddleware(RequestIDConfig{
		HeaderName:    "X-Custom-Request-ID",
		Generate:      true,
		TrustIncoming: false,
	})

	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Check that response has the custom header
	requestID := rr.Header().Get("X-Custom-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Custom-Request-ID header to be set")
	}

	// Default header should not be set
	if rr.Header().Get("X-Request-Id") != "" {
		t.Error("X-Request-Id should not be set when custom header is used")
	}
}
