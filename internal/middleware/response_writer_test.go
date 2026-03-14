package middleware

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	if rw == nil {
		t.Fatal("NewResponseWriter returned nil")
	}

	// Check initial state
	if rw.Status() != http.StatusOK {
		t.Errorf("expected default status %d, got %d", http.StatusOK, rw.Status())
	}
	if rw.BytesWritten() != 0 {
		t.Errorf("expected 0 bytes written, got %d", rw.BytesWritten())
	}
	if rw.Written() {
		t.Error("expected Written() to be false initially")
	}
}

func TestResponseWriterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusNotFound)

	if rw.Status() != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rw.Status())
	}
	if !rw.Written() {
		t.Error("expected Written() to be true after WriteHeader")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected recorder status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestResponseWriterWriteHeaderMultipleCalls(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusNotFound)
	rw.WriteHeader(http.StatusInternalServerError) // Should be ignored

	// First WriteHeader wins
	if rw.Status() != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rw.Status())
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected recorder status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestResponseWriterWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	data := []byte("Hello, World!")
	n, err := rw.Write(data)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if rw.BytesWritten() != int64(len(data)) {
		t.Errorf("expected %d bytes written, got %d", len(data), rw.BytesWritten())
	}
	if !rw.Written() {
		t.Error("expected Written() to be true after Write")
	}
	// Write without WriteHeader should default to 200
	if rw.Status() != http.StatusOK {
		t.Errorf("expected default status %d, got %d", http.StatusOK, rw.Status())
	}
	if rec.Body.String() != string(data) {
		t.Errorf("expected body %q, got %q", string(data), rec.Body.String())
	}
}

func TestResponseWriterWriteAfterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.WriteHeader(http.StatusCreated)
	data := []byte("Created!")
	rw.Write(data)

	if rw.Status() != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rw.Status())
	}
	if rw.BytesWritten() != int64(len(data)) {
		t.Errorf("expected %d bytes written, got %d", len(data), rw.BytesWritten())
	}
}

func TestResponseWriterMultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.Write([]byte("Hello, "))
	rw.Write([]byte("World!"))

	if rw.BytesWritten() != 13 {
		t.Errorf("expected 13 bytes written, got %d", rw.BytesWritten())
	}
	if rec.Body.String() != "Hello, World!" {
		t.Errorf("expected body %q, got %q", "Hello, World!", rec.Body.String())
	}
}

func TestResponseWriterHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	rw.Header().Set("X-Custom", "value")

	if rec.Header().Get("X-Custom") != "value" {
		t.Error("Header was not set correctly")
	}
}

func TestResponseWriterHijack(t *testing.T) {
	// Create a mock ResponseWriter that implements Hijacker
	hijackable := &mockHijackableResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	rw := NewResponseWriter(hijackable)

	conn, bufrw, err := rw.(http.Hijacker).Hijack()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conn != hijackable.conn {
		t.Error("Hijack did not return expected conn")
	}
	if bufrw != hijackable.bufrw {
		t.Error("Hijack did not return expected bufio.ReadWriter")
	}
}

func TestResponseWriterHijackNotSupported(t *testing.T) {
	// Standard httptest.ResponseRecorder does not support Hijack
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	_, _, err := rw.(http.Hijacker).Hijack()
	if err != http.ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

func TestResponseWriterFlush(t *testing.T) {
	// Create a mock ResponseWriter that implements Flusher
	flushable := &mockFlushableResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	rw := NewResponseWriter(flushable)

	rw.(http.Flusher).Flush()

	if !flushable.flushed {
		t.Error("Flush was not called")
	}
}

func TestResponseWriterFlushNotSupported(t *testing.T) {
	// Standard httptest.ResponseRecorder does not support Flush
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	// Should not panic
	rw.(http.Flusher).Flush()
}

func TestResponseWriterPush(t *testing.T) {
	// Create a mock ResponseWriter that implements Pusher
	pusher := &mockPusherResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	rw := NewResponseWriter(pusher)

	err := rw.(http.Pusher).Push("/style.css", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if pusher.target != "/style.css" {
		t.Errorf("expected target %q, got %q", "/style.css", pusher.target)
	}
}

func TestResponseWriterPushNotSupported(t *testing.T) {
	// Standard httptest.ResponseRecorder does not support Push
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	err := rw.(http.Pusher).Push("/style.css", nil)
	if err != http.ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

func TestResponseWriterUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec)

	unwrapped := rw.(*responseWriter).Unwrap()
	if unwrapped != rec {
		t.Error("Unwrap did not return original ResponseWriter")
	}
}

func TestResponseWriterRelease(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewResponseWriter(rec).(*responseWriter)

	// Set some state
	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte("test"))

	// Release back to pool
	rw.Release()

	// After release, ResponseWriter should be nil
	if rw.ResponseWriter != nil {
		t.Error("ResponseWriter should be nil after Release")
	}
}

func TestResponseWriterImplementsInterfaces(t *testing.T) {
	// Verify that responseWriter implements all required interfaces
	var _ ResponseWriter = (*responseWriter)(nil)
	var _ http.Hijacker = (*responseWriter)(nil)
	var _ http.Flusher = (*responseWriter)(nil)
	var _ http.Pusher = (*responseWriter)(nil)
}

// Mock implementations for testing optional interfaces

type mockHijackableResponseWriter struct {
	*httptest.ResponseRecorder
	conn   net.Conn
	bufrw  *bufio.ReadWriter
	hijacked bool
}

func (m *mockHijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	return m.conn, m.bufrw, nil
}

type mockFlushableResponseWriter struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (m *mockFlushableResponseWriter) Flush() {
	m.flushed = true
}

type mockPusherResponseWriter struct {
	*httptest.ResponseRecorder
	target string
	opts   *http.PushOptions
}

func (m *mockPusherResponseWriter) Push(target string, opts *http.PushOptions) error {
	m.target = target
	m.opts = opts
	return nil
}
