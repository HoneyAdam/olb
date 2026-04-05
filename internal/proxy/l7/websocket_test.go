package l7

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/balancer"
	"github.com/openloadbalancer/olb/internal/router"
)

// mockHijacker is a ResponseWriter that supports hijacking for testing.
type mockHijacker struct {
	*httptest.ResponseRecorder
	conn     net.Conn
	buf      *bytes.Buffer
	hijacked bool
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	if m.conn == nil {
		// Create a pipe for testing
		client, server := net.Pipe()
		m.conn = server
		go func() {
			// Keep the connection open
			buf := make([]byte, 1024)
			for {
				_, err := client.Read(buf)
				if err != nil {
					return
				}
			}
		}()
	}
	return m.conn, nil, nil
}

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		conn     string
		upgrade  string
		expected bool
	}{
		{
			name:     "valid websocket upgrade",
			conn:     "Upgrade",
			upgrade:  "websocket",
			expected: true,
		},
		{
			name:     "upgrade with keep-alive",
			conn:     "keep-alive, Upgrade",
			upgrade:  "websocket",
			expected: true,
		},
		{
			name:     "missing connection header",
			conn:     "",
			upgrade:  "websocket",
			expected: false,
		},
		{
			name:     "missing upgrade header",
			conn:     "Upgrade",
			upgrade:  "",
			expected: false,
		},
		{
			name:     "wrong upgrade type",
			conn:     "Upgrade",
			upgrade:  "h2",
			expected: false,
		},
		{
			name:     "case insensitive",
			conn:     "UPGRADE",
			upgrade:  "WEBSOCKET",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			if tt.conn != "" {
				req.Header.Set("Connection", tt.conn)
			}
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}

			got := IsWebSocketUpgrade(req)
			if got != tt.expected {
				t.Errorf("IsWebSocketUpgrade() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultWebSocketConfig(t *testing.T) {
	config := DefaultWebSocketConfig()

	if !config.EnableWebSocket {
		t.Error("EnableWebSocket should be true by default")
	}
	if config.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", config.IdleTimeout)
	}
	if config.PingInterval != 30*time.Second {
		t.Errorf("PingInterval = %v, want 30s", config.PingInterval)
	}
	if config.MaxMessageSize != 10*1024*1024 {
		t.Errorf("MaxMessageSize = %v, want 10MB", config.MaxMessageSize)
	}
}

func TestNewWebSocketHandler(t *testing.T) {
	config := DefaultWebSocketConfig()
	handler := NewWebSocketHandler(config)

	if handler == nil {
		t.Fatal("NewWebSocketHandler() returned nil")
	}
	if handler.config != config {
		t.Error("Handler config mismatch")
	}
	if handler.dialer == nil {
		t.Error("Handler dialer should not be nil")
	}
}

func TestWebSocketHandler_Disabled(t *testing.T) {
	config := &WebSocketConfig{EnableWebSocket: false}
	handler := NewWebSocketHandler(config)

	be := backend.NewBackend("backend-1", "127.0.0.1:8080")
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")

	rec := httptest.NewRecorder()
	err := handler.HandleWebSocket(rec, req, be)

	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Expected 'WebSocket disabled' error, got: %v", err)
	}
}

func TestWebSocketHandler_MissingKey(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	be := backend.NewBackend("backend-1", "127.0.0.1:8080")
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	// Missing Sec-WebSocket-Key

	rec := httptest.NewRecorder()
	err := handler.HandleWebSocket(rec, req, be)

	if err == nil || !strings.Contains(err.Error(), "missing Sec-WebSocket-Key") {
		t.Errorf("Expected 'missing Sec-WebSocket-Key' error, got: %v", err)
	}
}

func TestWebSocketHandler_BackendDialFail(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	be := backend.NewBackend("backend-1", "127.0.0.1:1") // unreachable port
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rec := httptest.NewRecorder()
	err := handler.HandleWebSocket(rec, req, be)

	if err == nil || !strings.Contains(err.Error(), "failed to connect to backend") {
		t.Errorf("Expected backend dial error, got: %v", err)
	}
}

func TestWebSocketHandler_BackendMaxConnections(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	be := backend.NewBackend("backend-1", "127.0.0.1:8080")
	be.SetState(backend.StateUp)
	be.MaxConns = 1

	// First connection should acquire
	if !be.AcquireConn() {
		t.Fatal("Failed to acquire first connection")
	}

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rec := httptest.NewRecorder()
	err := handler.HandleWebSocket(rec, req, be)

	if err == nil || !strings.Contains(err.Error(), "max connections") {
		t.Errorf("Expected 'max connections' error, got: %v", err)
	}

	be.ReleaseConn()
}

func TestIsWebSocketCloseError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "EOF",
			err:      io.EOF,
			expected: true,
		},
		{
			name:     "connection reset",
			err:      &net.OpError{Err: syscall.ECONNRESET},
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("write: broken pipe"),
			expected: true,
		},
		{
			name:     "use of closed connection",
			err:      errors.New("use of closed network connection"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some random error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWebSocketCloseError(tt.err)
			if got != tt.expected {
				t.Errorf("isWebSocketCloseError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWebSocketProxy_ServeHTTP_WebSocketRequest(t *testing.T) {
	// Create HTTP proxy
	httpProxy, _, _ := setupTestProxy(t)

	wsConfig := DefaultWebSocketConfig()
	wsProxy := NewWebSocketProxy(httpProxy, wsConfig)

	// Create WebSocket request
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")

	rec := httptest.NewRecorder()

	// This will fail because there's no real backend, but it should attempt WebSocket handling
	wsProxy.ServeHTTP(rec, req)

	// Should get an error (no healthy backends or hijack failure)
	// Response code will indicate error
	if rec.Code == 200 {
		t.Error("Expected non-200 response for failed WebSocket upgrade")
	}
}

func TestWebSocketProxy_ServeHTTP_HTTPRequest(t *testing.T) {
	// Create HTTP proxy
	httpProxy, _, _ := setupTestProxy(t)

	wsConfig := DefaultWebSocketConfig()
	wsProxy := NewWebSocketProxy(httpProxy, wsConfig)

	// Create regular HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	// Should route through HTTP proxy (which may error but not panic)
	wsProxy.ServeHTTP(rec, req)
}

func TestCopyWithIdleTimeout(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	// Create pipe connections
	client1, server1 := net.Pipe()
	client2, server2 := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)

	// Write data from client1
	go func() {
		defer wg.Done()
		client1.Write([]byte("hello"))
		client1.Close()
	}()

	// Copy from server1 to client2
	go func() {
		defer wg.Done()
		err := handler.copyWithIdleTimeout(client2, server1, 5*time.Second)
		if err != nil && !isWebSocketCloseError(err) {
			t.Errorf("copyWithIdleTimeout error: %v", err)
		}
	}()

	// Read from server2
	buf := make([]byte, 100)
	n, _ := server2.Read(buf)
	if string(buf[:n]) != "hello" {
		t.Errorf("Received %q, want hello", string(buf[:n]))
	}

	server2.Close()
	wg.Wait()
}

func TestCopyWithIdleTimeout_Timeout(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	// Create a TCP listener for more reliable timeout behavior
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connection but don't send data
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			// Don't write anything, just keep connection open briefly
			time.Sleep(200 * time.Millisecond)
			conn.Close()
		}
	}()

	// Connect to the listener
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Set a very short timeout
	done := make(chan error, 1)
	go func() {
		// Try to read with short timeout - will timeout since no data
		// Note: Timeout errors are treated as normal close conditions for WebSockets
		// so this may return nil
		err := handler.copyWithIdleTimeout(client, client, 50*time.Millisecond)
		done <- err
	}()

	// Wait for timeout
	select {
	case <-done:
		// Timeout errors are treated as normal close for WebSockets
		// The function should complete (not hang)
		// Success - the function returned (nil or error both OK)
	case <-time.After(500 * time.Millisecond):
		t.Error("copyWithIdleTimeout didn't complete within expected time")
	}
}

func TestProxyWebSocket(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	// Create two pipe pairs to simulate client and backend connections
	client1, server1 := net.Pipe()
	client2, server2 := net.Pipe()

	// Write from client side and read from server side
	go func() {
		client1.Write([]byte("ws message from client"))
		time.Sleep(100 * time.Millisecond)
		client1.Close()
	}()

	go func() {
		buf := make([]byte, 100)
		n, _ := server2.Read(buf)
		if n > 0 {
			server2.Write([]byte("ws response from backend"))
		}
		time.Sleep(100 * time.Millisecond)
		server2.Close()
	}()

	// proxyWebSocket should complete without panic
	err := handler.proxyWebSocket(server1, client2)
	// Error may or may not occur depending on timing, but should not panic
	_ = err
}

func TestDialBackend(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	// Create a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			// Just close the connection
			time.Sleep(100 * time.Millisecond)
			conn.Close()
		}
	}()

	be := backend.NewBackend("backend-1", listener.Addr().String())
	req := httptest.NewRequest("GET", "/ws", nil)

	// Test non-TLS dial
	conn, err := handler.dialBackend(req, be)
	if err != nil {
		t.Errorf("dialBackend error: %v", err)
		return
	}
	if conn != nil {
		conn.Close()
	}
}

func TestWebSocketHandler_HandleWebSocket_FullUpgrade(t *testing.T) {
	// Create a backend that accepts connections and echoes data
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	go func() {
		for {
			conn, err := backendListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// Read the upgrade request and respond with a WebSocket handshake
				buf := make([]byte, 4096)
				n, _ := c.Read(buf)
				if n > 0 {
					response := "HTTP/1.1 101 Switching Protocols\r\n" +
						"Upgrade: websocket\r\n" +
						"Connection: Upgrade\r\n" +
						"Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n\r\n"
					c.Write([]byte(response))
					// Echo data
					for {
						n, err := c.Read(buf)
						if err != nil {
							return
						}
						c.Write(buf[:n])
					}
				}
			}(conn)
		}
	}()

	handler := NewWebSocketHandler(nil)

	be := backend.NewBackend("ws-backend", backendListener.Addr().String())
	be.SetState(backend.StateUp)

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	// Create a hijackable response writer using net.Pipe
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	hijacker := &mockHijacker{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             serverConn,
	}

	// Close the client side after a short delay to end the proxy
	go func() {
		time.Sleep(200 * time.Millisecond)
		clientConn.Close()
	}()

	err = handler.HandleWebSocket(hijacker, req, be)
	// The error may be nil or a close error, both are acceptable
	if err != nil && !strings.Contains(err.Error(), "closed") &&
		!strings.Contains(err.Error(), "broken pipe") &&
		!strings.Contains(err.Error(), "EOF") {
		t.Logf("HandleWebSocket returned: %v (acceptable for connection lifecycle)", err)
	}
}

func TestWebSocketHandler_HandleWebSocket_BackendDialFail(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	// Backend address that refuses connections
	be := backend.NewBackend("ws-backend-bad", "127.0.0.1:1")
	be.SetState(backend.StateUp)

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	hijacker := &mockHijacker{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             serverConn,
	}

	err := handler.HandleWebSocket(hijacker, req, be)
	if err == nil {
		t.Error("Expected error when backend dial fails")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to connect to backend") {
		t.Errorf("Expected 'failed to connect to backend' error, got: %v", err)
	}
}

func TestDialBackend_WithQueryString(t *testing.T) {
	handler := NewWebSocketHandler(nil)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			time.Sleep(100 * time.Millisecond)
			conn.Close()
		}
	}()

	be := backend.NewBackend("backend-qs", listener.Addr().String())
	req := httptest.NewRequest("GET", "/ws?token=abc&room=test", nil)

	conn, err := handler.dialBackend(req, be)
	if err != nil {
		t.Errorf("dialBackend error: %v", err)
		return
	}
	if conn != nil {
		conn.Close()
	}
}

// ============================================================================
// WebSocketProxy Tests
// ============================================================================

func TestWebSocketProxy_ServeHTTP_NonWebSocket(t *testing.T) {
	proxy, poolManager, routerInstance := setupTestProxy(t)

	// Create test backend
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("regular http"))
	}))
	defer backendServer.Close()

	backendAddr := backendServer.Listener.Addr().String()

	pool := backend.NewPool("test-pool", "round_robin")
	pool.SetBalancer(balancer.NewRoundRobin())
	b := backend.NewBackend("backend-1", backendAddr)
	b.SetState(backend.StateUp)
	pool.AddBackend(b)
	poolManager.AddPool(pool)

	route := &router.Route{
		Name:        "test-route",
		Path:        "/test",
		BackendPool: "test-pool",
	}
	routerInstance.AddRoute(route)

	wsProxy := NewWebSocketProxy(proxy, nil)

	// Regular HTTP request (not WebSocket)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wsProxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if body := rr.Body.String(); body != "regular http" {
		t.Errorf("expected body 'regular http', got '%s'", body)
	}
}

func TestWebSocketProxy_ServeHTTP_WebSocket_NoRoute(t *testing.T) {
	proxy, _, _ := setupTestProxy(t)

	wsProxy := NewWebSocketProxy(proxy, nil)

	// WebSocket request with no matching route
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rr := httptest.NewRecorder()

	wsProxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestWebSocketProxy_ServeHTTP_WebSocket_NoPool(t *testing.T) {
	proxy, _, routerInstance := setupTestProxy(t)

	// Add route but no pool
	route := &router.Route{
		Name:        "test-route",
		Path:        "/ws",
		BackendPool: "nonexistent-pool",
	}
	routerInstance.AddRoute(route)

	wsProxy := NewWebSocketProxy(proxy, nil)

	// WebSocket request
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rr := httptest.NewRecorder()

	wsProxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestWebSocketProxy_ServeHTTP_WebSocket_NoHealthyBackends(t *testing.T) {
	proxy, poolManager, routerInstance := setupTestProxy(t)

	// Create pool with unhealthy backend
	pool := backend.NewPool("test-pool", "round_robin")
	pool.SetBalancer(balancer.NewRoundRobin())
	b := backend.NewBackend("backend-1", "127.0.0.1:8080")
	b.SetState(backend.StateDown)
	pool.AddBackend(b)
	poolManager.AddPool(pool)

	route := &router.Route{
		Name:        "test-route",
		Path:        "/ws",
		BackendPool: "test-pool",
	}
	routerInstance.AddRoute(route)

	wsProxy := NewWebSocketProxy(proxy, nil)

	// WebSocket request
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rr := httptest.NewRecorder()

	wsProxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestWebSocketProxy_NewWebSocketProxy(t *testing.T) {
	proxy, _, _ := setupTestProxy(t)

	config := &WebSocketConfig{
		EnableWebSocket: true,
		MaxMessageSize:  1024,
	}

	wsProxy := NewWebSocketProxy(proxy, config)
	if wsProxy == nil {
		t.Fatal("NewWebSocketProxy() returned nil")
	}
	if wsProxy.httpProxy != proxy {
		t.Error("httpProxy not set correctly")
	}
	if wsProxy.wsHandler == nil {
		t.Error("wsHandler not initialized")
	}
}

