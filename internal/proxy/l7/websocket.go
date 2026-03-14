package l7

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
)

// WebSocketConfig configures WebSocket proxy behavior.
type WebSocketConfig struct {
	// EnableWebSocket enables WebSocket proxying.
	EnableWebSocket bool

	// IdleTimeout is the maximum time to wait for data before closing.
	IdleTimeout time.Duration

	// PingInterval is the interval between ping frames (0 = disabled).
	PingInterval time.Duration

	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize int64
}

// DefaultWebSocketConfig returns a default WebSocket configuration.
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		EnableWebSocket: true,
		IdleTimeout:     60 * time.Second,
		PingInterval:    30 * time.Second,
		MaxMessageSize:  10 * 1024 * 1024, // 10MB
	}
}

// IsWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func IsWebSocketUpgrade(r *http.Request) bool {
	// Check Connection header contains "Upgrade"
	connHeader := strings.ToLower(r.Header.Get("Connection"))
	if !strings.Contains(connHeader, "upgrade") {
		return false
	}

	// Check Upgrade header is "websocket"
	upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	return upgradeHeader == "websocket"
}

// WebSocketHandler handles WebSocket proxying.
type WebSocketHandler struct {
	config *WebSocketConfig
	dialer *net.Dialer
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(config *WebSocketConfig) *WebSocketHandler {
	if config == nil {
		config = DefaultWebSocketConfig()
	}

	return &WebSocketHandler{
		config: config,
		dialer: &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
}

// HandleWebSocket handles a WebSocket upgrade request.
// It hijacks the client connection and establishes a bidirectional tunnel to the backend.
func (wh *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request, b *backend.Backend) error {
	if !wh.config.EnableWebSocket {
		return errors.New("WebSocket disabled")
	}

	// Validate WebSocket request
	if r.Header.Get("Sec-WebSocket-Version") == "" {
		return errors.New("missing Sec-WebSocket-Version header")
	}

	// Acquire connection slot
	if !b.AcquireConn() {
		return errors.New("backend at max connections")
	}
	defer b.ReleaseConn()

	// Hijack the client connection
	clientConn, clientBuf, err := wh.hijackConnection(w)
	if err != nil {
		return fmt.Errorf("failed to hijack connection: %w", err)
	}
	defer clientConn.Close()

	// Connect to backend
	backendConn, err := wh.dialBackend(r, b)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}
	defer backendConn.Close()

	// If we have buffered data from the client, forward it first
	if clientBuf != nil && clientBuf.Reader != nil {
		if clientBuf.Reader.Buffered() > 0 {
			buffered := make([]byte, clientBuf.Reader.Buffered())
			_, err := clientBuf.Reader.Read(buffered)
			if err == nil && len(buffered) > 0 {
				if _, writeErr := backendConn.Write(buffered); writeErr != nil {
					return fmt.Errorf("failed to write buffered data: %w", writeErr)
				}
			}
		}
	}

	// Perform bidirectional copy
	return wh.proxyWebSocket(clientConn, backendConn, b)
}

// hijackConnection hijacks the HTTP connection to get raw TCP access.
func (wh *WebSocketHandler) hijackConnection(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}

	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}

	return conn, buf, nil
}

// dialBackend dials the backend server for WebSocket connection.
func (wh *WebSocketHandler) dialBackend(r *http.Request, b *backend.Backend) (net.Conn, error) {
	// Determine if we need TLS
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}

	// Build backend WebSocket URL
	backendURL := &url.URL{
		Scheme: scheme,
		Host:   b.Address,
		Path:   r.URL.Path,
	}
	if r.URL.RawQuery != "" {
		backendURL.RawQuery = r.URL.RawQuery
	}

	// Check if backend address implies TLS
	isTLS := strings.HasPrefix(b.Address, "https://") || strings.HasPrefix(b.Address, "wss://")
	address := strings.TrimPrefix(b.Address, "https://")
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimPrefix(address, "wss://")
	address = strings.TrimPrefix(address, "ws://")

	// Use appropriate dialer based on TLS
	if isTLS || scheme == "wss" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // In production, verify backend certs
		}
		return tls.DialWithDialer(wh.dialer, "tcp", address, tlsConfig)
	}

	return wh.dialer.Dial("tcp", address)
}

// proxyWebSocket performs bidirectional data copying between client and backend.
func (wh *WebSocketHandler) proxyWebSocket(clientConn, backendConn net.Conn, b *backend.Backend) error {
	// Set up error channels
	errChan := make(chan error, 2)

	// Create wait group to wait for both directions to complete
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Backend
	go func() {
		defer wg.Done()
		err := wh.copyWithIdleTimeout(backendConn, clientConn, wh.config.IdleTimeout)
		if err != nil && !isWebSocketCloseError(err) {
			errChan <- fmt.Errorf("client to backend: %w", err)
		}
		// Signal other direction to close
		backendConn.Close()
	}()

	// Backend -> Client
	go func() {
		defer wg.Done()
		err := wh.copyWithIdleTimeout(clientConn, backendConn, wh.config.IdleTimeout)
		if err != nil && !isWebSocketCloseError(err) {
			errChan <- fmt.Errorf("backend to client: %w", err)
		}
		// Signal other direction to close
		clientConn.Close()
	}()

	// Wait for both directions to complete
	wg.Wait()

	// Check if there were any errors
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// copyWithIdleTimeout copies data with an idle timeout.
func (wh *WebSocketHandler) copyWithIdleTimeout(dst, src net.Conn, timeout time.Duration) error {
	buf := make([]byte, 32*1024)

	for {
		// Set read deadline
		if timeout > 0 {
			src.SetReadDeadline(time.Now().Add(timeout))
		}

		nr, err := src.Read(buf)
		if nr > 0 {
			// Clear deadline for write
			if timeout > 0 {
				src.SetReadDeadline(time.Time{})
			}

			// Write to destination
			nw, writeErr := dst.Write(buf[:nr])
			if writeErr != nil {
				return writeErr
			}
			if nw != nr {
				return io.ErrShortWrite
			}
		}

		if err != nil {
			if isWebSocketCloseError(err) {
				return nil // Normal close
			}
			return err
		}
	}
}

// isWebSocketCloseError checks if an error is a normal WebSocket close condition.
func isWebSocketCloseError(err error) bool {
	if err == nil {
		return false
	}

	// Check for EOF (normal close)
	if errors.Is(err, io.EOF) {
		return true
	}

	// Check for net.Error
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Connection closed is normal for WebSockets
		if netErr.Timeout() {
			return true
		}
	}

	// Check for syscall errors
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, syscall.ECONNABORTED) {
		return true
	}

	// Check error string for common close conditions
	errStr := strings.ToLower(err.Error())
	closeConditions := []string{
		"use of closed network connection",
		"broken pipe",
		"connection reset",
		"websocket close",
		"eof",
	}

	for _, cond := range closeConditions {
		if strings.Contains(errStr, cond) {
			return true
		}
	}

	return false
}

// WebSocketProxy wraps an HTTPProxy with WebSocket support.
type WebSocketProxy struct {
	httpProxy *HTTPProxy
	wsHandler *WebSocketHandler
}

// NewWebSocketProxy creates a new proxy with WebSocket support.
func NewWebSocketProxy(httpProxy *HTTPProxy, wsConfig *WebSocketConfig) *WebSocketProxy {
	return &WebSocketProxy{
		httpProxy: httpProxy,
		wsHandler: NewWebSocketHandler(wsConfig),
	}
}

// ServeHTTP implements http.Handler with WebSocket support.
func (wp *WebSocketProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket upgrade request
	if IsWebSocketUpgrade(r) {
		// Get route match (reuse HTTP proxy's router)
		routeMatch, ok := wp.httpProxy.router.Match(r)
		if !ok {
			wp.httpProxy.errorHandler(w, r, errors.New("route not found"))
			return
		}

		// Get backend pool
		pool := wp.httpProxy.poolManager.GetPool(routeMatch.Route.BackendPool)
		if pool == nil {
			wp.httpProxy.errorHandler(w, r, errors.New("pool not found"))
			return
		}

		// Select backend
		backends := pool.GetHealthyBackends()
		if len(backends) == 0 {
			wp.httpProxy.errorHandler(w, r, errors.New("no healthy backends"))
			return
		}

		selected := pool.GetBalancer().Next(backends)
		if selected == nil {
			wp.httpProxy.errorHandler(w, r, errors.New("no backend available"))
			return
		}

		// Handle WebSocket
		if err := wp.wsHandler.HandleWebSocket(w, r, selected); err != nil {
			wp.httpProxy.errorHandler(w, r, err)
		}
		return
	}

	// Not a WebSocket request, use regular HTTP proxy
	wp.httpProxy.ServeHTTP(w, r)
}
