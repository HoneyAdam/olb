package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIPMiddleware_Name(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}
	if got := m.Name(); got != "real-ip" {
		t.Errorf("Name() = %q, want %q", got, "real-ip")
	}
}

func TestRealIPMiddleware_Priority(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}
	if got := m.Priority(); got != 300 {
		t.Errorf("Priority() = %d, want %d", got, 300)
	}
}

func TestRealIPMiddleware_InvalidCIDR(t *testing.T) {
	_, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"invalid-cidr"},
	})
	if err == nil {
		t.Error("NewRealIPMiddleware() expected error for invalid CIDR")
	}
}

func TestRealIPMiddleware_NoTrustedProxies(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedIP != "192.168.1.100" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "192.168.1.100")
	}
}

func TestRealIPMiddleware_UntrustedProxy(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Request comes from untrusted IP (not in 10.0.0.0/8)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should use RemoteAddr, not X-Forwarded-For
	if capturedIP != "192.168.1.100" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "192.168.1.100")
	}
}

func TestRealIPMiddleware_TrustedProxy_XForwardedFor(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Request comes from trusted proxy
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should extract client IP from X-Forwarded-For
	if capturedIP != "1.2.3.4" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "1.2.3.4")
	}
}

func TestRealIPMiddleware_TrustedProxy_XForwardedFor_MultipleHops(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8", "172.16.0.0/12"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Request chain: 1.2.3.4 -> 10.0.0.5 -> 172.16.0.1 -> us
	// X-Forwarded-For: client, proxy1, proxy2
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.1:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.5, 172.16.0.2")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should extract the rightmost non-trusted IP (10.0.0.5 is trusted, so we want 1.2.3.4)
	// Actually, walking from right: 172.16.0.2 (trusted), 10.0.0.5 (trusted), 1.2.3.4 (not trusted)
	// So we should get 1.2.3.4
	if capturedIP != "1.2.3.4" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "1.2.3.4")
	}
}

func TestRealIPMiddleware_TrustedProxy_XForwardedFor_AllTrusted(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// All IPs in chain are trusted
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 10.0.0.6, 10.0.0.7")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should return the leftmost (original client) when all are trusted
	if capturedIP != "10.0.0.5" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "10.0.0.5")
	}
}

func TestRealIPMiddleware_TrustedProxy_XRealIp(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Request comes from trusted proxy with X-Real-Ip
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-Ip", "5.6.7.8")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should extract client IP from X-Real-Ip (no X-Forwarded-For)
	if capturedIP != "5.6.7.8" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "5.6.7.8")
	}
}

func TestRealIPMiddleware_TrustedProxy_XForwardedFor_Preferred(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Both headers present - X-Forwarded-For should be preferred
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-Ip", "5.6.7.8")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should prefer X-Forwarded-For over X-Real-Ip
	if capturedIP != "1.2.3.4" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "1.2.3.4")
	}
}

func TestRealIPMiddleware_CustomHeaders(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
		Header:         "X-Custom-Forwarded-For",
		RealIPHeader:   "X-Custom-Real-Ip",
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Custom-Forwarded-For", "9.8.7.6")
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // Should be ignored
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedIP != "9.8.7.6" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "9.8.7.6")
	}
}

func TestRealIPMiddleware_IPv6(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"::1/128"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:12345"
	req.Header.Set("X-Forwarded-For", "2001:db8::1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedIP != "2001:db8::1" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "2001:db8::1")
	}
}

func TestRealIPMiddleware_Whitespace(t *testing.T) {
	m, err := NewRealIPMiddleware(RealIPConfig{
		TrustedProxies: []string{"10.0.0.0/8", "5.6.7.0/24"},
	})
	if err != nil {
		t.Fatalf("NewRealIPMiddleware() error = %v", err)
	}

	var capturedIP string
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = ClientIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "  1.2.3.4  ,  5.6.7.8  ")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should handle whitespace correctly and find rightmost non-trusted IP
	// 5.6.7.8 is trusted (in 5.6.7.0/24), so we should get 1.2.3.4
	if capturedIP != "1.2.3.4" {
		t.Errorf("ClientIP = %q, want %q", capturedIP, "1.2.3.4")
	}
}

func TestClientIPFromContext_NotSet(t *testing.T) {
	ip := ClientIPFromContext(nil)
	if ip != "" {
		t.Errorf("ClientIPFromContext(nil) = %q, want empty string", ip)
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"fe80::1", true},
		{"", false},
		{"not-an-ip", false},
		{"256.1.1.1", false},
		{"1.2.3", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := isValidIP(tt.ip); got != tt.valid {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.ip, got, tt.valid)
			}
		})
	}
}
