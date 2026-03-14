package l7

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
)

func TestIsGRPCRequest(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "gRPC request",
			contentType: "application/grpc",
			expected:    true,
		},
		{
			name:        "gRPC with encoding",
			contentType: "application/grpc+proto",
			expected:    true,
		},
		{
			name:        "regular HTTP request",
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "empty content type",
			contentType: "",
			expected:    false,
		},
		{
			name:        "gRPC-Web request",
			contentType: "application/grpc-web",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/service/method", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			got := IsGRPCRequest(req)
			if got != tt.expected {
				t.Errorf("IsGRPCRequest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultGRPCConfig(t *testing.T) {
	config := DefaultGRPCConfig()

	if !config.EnableGRPC {
		t.Error("EnableGRPC should be true by default")
	}
	if config.MaxMessageSize != 4*1024*1024 {
		t.Errorf("MaxMessageSize = %v, want 4MB", config.MaxMessageSize)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
	if !config.EnableGRPCWeb {
		t.Error("EnableGRPCWeb should be true by default")
	}
}

func TestNewGRPCHandler(t *testing.T) {
	config := DefaultGRPCConfig()
	handler := NewGRPCHandler(config)

	if handler == nil {
		t.Fatal("NewGRPCHandler() returned nil")
	}
	if handler.config != config {
		t.Error("Handler config mismatch")
	}
	if handler.transport == nil {
		t.Error("Handler transport should not be nil")
	}
}

func TestNewGRPCHandler_NilConfig(t *testing.T) {
	handler := NewGRPCHandler(nil)

	if handler == nil {
		t.Fatal("NewGRPCHandler(nil) returned nil")
	}
	if handler.config == nil {
		t.Error("Handler config should use defaults when nil")
	}
}

func TestGRPCHandler_Disabled(t *testing.T) {
	config := &GRPCConfig{EnableGRPC: false}
	handler := NewGRPCHandler(config)

	be := backend.NewBackend("backend-1", "127.0.0.1:8080")
	req := httptest.NewRequest("POST", "/service/method", nil)
	req.Header.Set("Content-Type", "application/grpc")

	rec := httptest.NewRecorder()
	err := handler.HandleGRPC(rec, req, be)

	if err == nil || err.Error() != "gRPC disabled" {
		t.Errorf("Expected 'gRPC disabled' error, got: %v", err)
	}
}

func TestGRPCHandler_BackendMaxConnections(t *testing.T) {
	handler := NewGRPCHandler(nil)

	be := backend.NewBackend("backend-1", "127.0.0.1:8080")
	be.SetState(backend.StateUp)
	be.MaxConns = 1

	// First connection should acquire
	if !be.AcquireConn() {
		t.Fatal("Failed to acquire first connection")
	}

	req := httptest.NewRequest("POST", "/service/method", nil)
	req.Header.Set("Content-Type", "application/grpc")

	rec := httptest.NewRecorder()
	err := handler.HandleGRPC(rec, req, be)

	if err == nil || err.Error() != "backend at max connections" {
		t.Errorf("Expected 'backend at max connections' error, got: %v", err)
	}

	be.ReleaseConn()
}

func TestIsGRPCWebRequest(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "gRPC-Web request",
			contentType: "application/grpc-web",
			expected:    true,
		},
		{
			name:        "gRPC-Web with text",
			contentType: "application/grpc-web-text",
			expected:    true,
		},
		{
			name:        "regular gRPC request",
			contentType: "application/grpc",
			expected:    false,
		},
		{
			name:        "regular HTTP request",
			contentType: "application/json",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/service/method", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			got := IsGRPCWebRequest(req)
			if got != tt.expected {
				t.Errorf("IsGRPCWebRequest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewGRPCWebHandler(t *testing.T) {
	grpcHandler := NewGRPCHandler(nil)
	webHandler := NewGRPCWebHandler(grpcHandler)

	if webHandler == nil {
		t.Fatal("NewGRPCWebHandler() returned nil")
	}
	if webHandler.grpcHandler != grpcHandler {
		t.Error("gRPC-Web handler should reference gRPC handler")
	}
}

func TestGRPCWebHandler_Disabled(t *testing.T) {
	config := &GRPCConfig{EnableGRPC: true, EnableGRPCWeb: false}
	grpcHandler := NewGRPCHandler(config)
	webHandler := NewGRPCWebHandler(grpcHandler)

	be := backend.NewBackend("backend-1", "127.0.0.1:8080")
	req := httptest.NewRequest("POST", "/service/method", nil)
	req.Header.Set("Content-Type", "application/grpc-web")

	rec := httptest.NewRecorder()
	err := webHandler.HandleGRPCWeb(rec, req, be)

	if err == nil || err.Error() != "gRPC-Web disabled" {
		t.Errorf("Expected 'gRPC-Web disabled' error, got: %v", err)
	}
}

func TestHTTPStatusToGRPCStatus(t *testing.T) {
	tests := []struct {
		httpStatus int
		expected   GRPCStatus
	}{
		{http.StatusOK, GRPCStatusOK},
		{http.StatusBadRequest, GRPCStatusInvalidArgument},
		{http.StatusUnauthorized, GRPCStatusUnauthenticated},
		{http.StatusForbidden, GRPCStatusPermissionDenied},
		{http.StatusNotFound, GRPCStatusNotFound},
		{http.StatusTooManyRequests, GRPCStatusResourceExhausted},
		{http.StatusInternalServerError, GRPCStatusInternal},
		{http.StatusNotImplemented, GRPCStatusUnimplemented},
		{http.StatusBadGateway, GRPCStatusUnavailable},
		{http.StatusServiceUnavailable, GRPCStatusUnavailable},
		{http.StatusGatewayTimeout, GRPCStatusDeadlineExceeded},
		{999, GRPCStatusUnknown}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.httpStatus), func(t *testing.T) {
			got := HTTPStatusToGRPCStatus(tt.httpStatus)
			if got != tt.expected {
				t.Errorf("HTTPStatusToGRPCStatus(%d) = %v, want %v", tt.httpStatus, got, tt.expected)
			}
		})
	}
}

func TestGRPCStatusToHTTPStatus(t *testing.T) {
	tests := []struct {
		grpcStatus GRPCStatus
		expected   int
	}{
		{GRPCStatusOK, http.StatusOK},
		{GRPCStatusCancelled, 499},
		{GRPCStatusUnknown, http.StatusInternalServerError},
		{GRPCStatusInvalidArgument, http.StatusBadRequest},
		{GRPCStatusDeadlineExceeded, http.StatusGatewayTimeout},
		{GRPCStatusNotFound, http.StatusNotFound},
		{GRPCStatusAlreadyExists, http.StatusConflict},
		{GRPCStatusPermissionDenied, http.StatusForbidden},
		{GRPCStatusResourceExhausted, http.StatusTooManyRequests},
		{GRPCStatusFailedPrecondition, http.StatusPreconditionFailed},
		{GRPCStatusAborted, 409},
		{GRPCStatusOutOfRange, http.StatusBadRequest},
		{GRPCStatusUnimplemented, http.StatusNotImplemented},
		{GRPCStatusInternal, http.StatusInternalServerError},
		{GRPCStatusUnavailable, http.StatusServiceUnavailable},
		{GRPCStatusDataLoss, http.StatusInternalServerError},
		{GRPCStatusUnauthenticated, http.StatusUnauthorized},
		{99, http.StatusInternalServerError}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.grpcStatus)), func(t *testing.T) {
			got := GRPCStatusToHTTPStatus(tt.grpcStatus)
			if got != tt.expected {
				t.Errorf("GRPCStatusToHTTPStatus(%d) = %d, want %d", tt.grpcStatus, got, tt.expected)
			}
		})
	}
}

func TestCopyGRPCHeaders(t *testing.T) {
	src := http.Header{
		"Content-Type": []string{"application/grpc"},
		"X-Custom":     []string{"value"},
		"Connection":   []string{"keep-alive"}, // hop-by-hop, should be skipped
		"Trailer":      []string{"grpc-status"}, // trailer header, should be skipped
	}

	dst := make(http.Header)
	copyGRPCHeaders(dst, src)

	// Should have Content-Type and X-Custom
	if dst.Get("Content-Type") != "application/grpc" {
		t.Error("Content-Type header not copied")
	}
	if dst.Get("X-Custom") != "value" {
		t.Error("X-Custom header not copied")
	}

	// Should not have hop-by-hop headers
	if dst.Get("Connection") != "" {
		t.Error("Connection header should not be copied")
	}

	// Should not have trailer headers
	if dst.Get("Trailer") != "" {
		t.Error("Trailer header should not be copied")
	}
}

func TestParseGRPCFrame(t *testing.T) {
	// Create a gRPC frame
	data := []byte("hello world")
	frame := &gRPCFrame{
		Compressed: false,
		Length:     uint32(len(data)),
		Data:       data,
	}

	// Write to buffer
	var buf bytes.Buffer
	err := writeGRPCFrame(&buf, frame)
	if err != nil {
		t.Fatalf("writeGRPCFrame error: %v", err)
	}

	// Parse back
	parsed, err := parseGRPCFrame(&buf)
	if err != nil {
		t.Fatalf("parseGRPCFrame error: %v", err)
	}

	if parsed.Compressed != frame.Compressed {
		t.Errorf("Compressed = %v, want %v", parsed.Compressed, frame.Compressed)
	}
	if parsed.Length != frame.Length {
		t.Errorf("Length = %v, want %v", parsed.Length, frame.Length)
	}
	if !bytes.Equal(parsed.Data, frame.Data) {
		t.Errorf("Data = %v, want %v", parsed.Data, frame.Data)
	}
}

func TestParseGRPCFrame_Compressed(t *testing.T) {
	// Create a compressed gRPC frame
	data := []byte("compressed data")
	frame := &gRPCFrame{
		Compressed: true,
		Length:     uint32(len(data)),
		Data:       data,
	}

	// Write to buffer
	var buf bytes.Buffer
	err := writeGRPCFrame(&buf, frame)
	if err != nil {
		t.Fatalf("writeGRPCFrame error: %v", err)
	}

	// Parse back
	parsed, err := parseGRPCFrame(&buf)
	if err != nil {
		t.Fatalf("parseGRPCFrame error: %v", err)
	}

	if !parsed.Compressed {
		t.Error("Compressed flag should be true")
	}
}

func TestParseGRPCFrame_Empty(t *testing.T) {
	// Create an empty gRPC frame
	frame := &gRPCFrame{
		Compressed: false,
		Length:     0,
		Data:       []byte{},
	}

	// Write to buffer
	var buf bytes.Buffer
	err := writeGRPCFrame(&buf, frame)
	if err != nil {
		t.Fatalf("writeGRPCFrame error: %v", err)
	}

	// Parse back
	parsed, err := parseGRPCFrame(&buf)
	if err != nil {
		t.Fatalf("parseGRPCFrame error: %v", err)
	}

	if parsed.Length != 0 {
		t.Errorf("Length = %v, want 0", parsed.Length)
	}
	if len(parsed.Data) != 0 {
		t.Errorf("Data length = %v, want 0", len(parsed.Data))
	}
}

func TestParseGRPCFrame_Invalid(t *testing.T) {
	// Test with incomplete data
	buf := bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}) // Missing length byte
	_, err := parseGRPCFrame(buf)
	if err == nil {
		t.Error("Expected error for incomplete frame header")
	}
}

func TestPrepareGRPCRequest(t *testing.T) {
	handler := NewGRPCHandler(nil)

	be := backend.NewBackend("backend-1", "10.0.0.1:8080")
	req := httptest.NewRequest("POST", "/my.service/method", bytes.NewReader([]byte("request body")))
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Host = "example.com"

	outReq, err := handler.prepareGRPCRequest(req, be)
	if err != nil {
		t.Fatalf("prepareGRPCRequest error: %v", err)
	}

	// Check URL
	if outReq.URL.Host != "10.0.0.1:8080" {
		t.Errorf("URL.Host = %v, want 10.0.0.1:8080", outReq.URL.Host)
	}
	if outReq.URL.Scheme != "http" {
		t.Errorf("URL.Scheme = %v, want http", outReq.URL.Scheme)
	}

	// Check Host is preserved
	if outReq.Host != "example.com" {
		t.Errorf("Host = %v, want example.com", outReq.Host)
	}

	// Check HTTP/2
	if outReq.ProtoMajor != 2 {
		t.Errorf("ProtoMajor = %v, want 2", outReq.ProtoMajor)
	}

	// Check X-Forwarded headers
	if outReq.Header.Get("X-Forwarded-For") == "" {
		t.Error("X-Forwarded-For header not set")
	}
	if outReq.Header.Get("X-Forwarded-Proto") != "http" {
		t.Errorf("X-Forwarded-Proto = %v, want http", outReq.Header.Get("X-Forwarded-Proto"))
	}

	// Check custom header is preserved
	if outReq.Header.Get("X-Custom-Header") != "custom-value" {
		t.Error("Custom header not preserved")
	}
}

func TestGRPCFrame_LargeData(t *testing.T) {
	// Test with larger data
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	frame := &gRPCFrame{
		Compressed: false,
		Length:     uint32(len(data)),
		Data:       data,
	}

	var buf bytes.Buffer
	err := writeGRPCFrame(&buf, frame)
	if err != nil {
		t.Fatalf("writeGRPCFrame error: %v", err)
	}

	parsed, err := parseGRPCFrame(&buf)
	if err != nil {
		t.Fatalf("parseGRPCFrame error: %v", err)
	}

	if parsed.Length != 10000 {
		t.Errorf("Length = %v, want 10000", parsed.Length)
	}
	if !bytes.Equal(parsed.Data, data) {
		t.Error("Data mismatch")
	}
}

func TestGRPCStatusConstants(t *testing.T) {
	// Verify gRPC status code values match spec
	if GRPCStatusOK != 0 {
		t.Error("GRPCStatusOK should be 0")
	}
	if GRPCStatusCancelled != 1 {
		t.Error("GRPCStatusCancelled should be 1")
	}
	if GRPCStatusUnknown != 2 {
		t.Error("GRPCStatusUnknown should be 2")
	}
	if GRPCStatusInternal != 13 {
		t.Error("GRPCStatusInternal should be 13")
	}
	if GRPCStatusUnavailable != 14 {
		t.Error("GRPCStatusUnavailable should be 14")
	}
	if GRPCStatusUnauthenticated != 16 {
		t.Error("GRPCStatusUnauthenticated should be 16")
	}
}
