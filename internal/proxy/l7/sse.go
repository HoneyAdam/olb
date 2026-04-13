package l7

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
)

// SSEConfig configures Server-Sent Events proxy behavior.
type SSEConfig struct {
	// EnableSSE enables SSE proxying support.
	EnableSSE bool

	// MaxEventSize is the maximum size of a single SSE event.
	MaxEventSize int64

	// IdleTimeout is the maximum time to wait between events.
	IdleTimeout time.Duration

	// FlushInterval is how often to flush even if buffer not full (0 = disable).
	FlushInterval time.Duration
}

// DefaultSSEConfig returns a default SSE configuration.
func DefaultSSEConfig() *SSEConfig {
	return &SSEConfig{
		EnableSSE:     true,
		MaxEventSize:  1024 * 1024, // 1MB max per event
		IdleTimeout:   60 * time.Second,
		FlushInterval: 0, // No forced flush, rely on line-based flushing
	}
}

// IsSSERequest checks if the request is an SSE request.
func IsSSERequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/event-stream")
}

// IsSSEResponse checks if the response is an SSE response.
func IsSSEResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/event-stream")
}

// SSEHandler handles Server-Sent Events proxying.
type SSEHandler struct {
	config    *SSEConfig
	transport *http.Transport
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(config *SSEConfig) *SSEHandler {
	if config == nil {
		config = DefaultSSEConfig()
	}
	return &SSEHandler{
		config:    config,
		transport: newSSETransport(),
	}
}

// newSSETransport creates an HTTP transport optimized for SSE.
func newSSETransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
}

// HandleSSE handles an SSE request.
// For SSE, we need to:
// 1. Disable buffering and enable immediate flush
// 2. Preserve the connection for streaming
// 3. Handle Last-Event-ID for replay/resume
func (sh *SSEHandler) HandleSSE(w http.ResponseWriter, r *http.Request, b *backend.Backend) error {
	if !sh.config.EnableSSE {
		return errors.New("sse disabled")
	}

	// Acquire connection slot
	if !b.AcquireConn() {
		return errors.New("backend at max connections")
	}
	defer b.ReleaseConn()

	// Prepare outbound request
	outReq, err := sh.prepareSSERequest(r, b)
	if err != nil {
		return err
	}

	// Execute request
	resp, err := sh.transport.RoundTrip(outReq)
	if err != nil {
		return fmt.Errorf("backend request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if response is actually SSE
	if !IsSSEResponse(resp) {
		// Not an SSE response, treat as regular HTTP response
		return sh.copyRegularResponse(w, resp)
	}

	// Handle SSE response with streaming
	return sh.streamSSEResponseWithContext(w, r, resp, b)
}

// prepareSSERequest creates the outbound SSE request.
func (sh *SSEHandler) prepareSSERequest(r *http.Request, b *backend.Backend) (*http.Request, error) {
	// Clone the request
	outReq := r.Clone(r.Context())

	// Set the URL to point to the backend
	outReq.URL.Scheme = "http"
	outReq.URL.Host = b.Address
	outReq.Host = r.Host
	outReq.RequestURI = ""

	// Set X-Forwarded headers
	clientIP := getClientIP(r)
	if prior := outReq.Header.Get("X-Forwarded-For"); prior != "" {
		outReq.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}
	outReq.Header.Set("X-Real-IP", clientIP)

	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	outReq.Header.Set("X-Forwarded-Proto", proto)

	// Ensure Accept header is set
	if outReq.Header.Get("Accept") == "" {
		outReq.Header.Set("Accept", "text/event-stream")
	}

	return outReq, nil
}

// streamSSEResponse streams an SSE response to the client without context awareness.
// This is kept for backward compatibility; prefer streamSSEResponseWithContext.
func (sh *SSEHandler) streamSSEResponse(w http.ResponseWriter, resp *http.Response, b *backend.Backend) error {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	return sh.streamSSEResponseWithContext(w, req, resp, b)
}

// streamSSEResponseWithContext streams an SSE response to the client,
// stopping when the client disconnects (via request context cancellation).
func (sh *SSEHandler) streamSSEResponseWithContext(w http.ResponseWriter, r *http.Request, resp *http.Response, b *backend.Backend) error {
	// Copy headers
	copySSEHeaders(w.Header(), resp.Header)

	// Set SSE-required cache and streaming headers per spec
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable proxy buffering (nginx compat)

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Get flusher (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		// If we can't flush, just copy the body normally (bounded)
		const maxSSEFallbackSize = 64 * 1024 * 1024 // 64MB
		_, err := io.Copy(w, io.LimitReader(resp.Body, maxSSEFallbackSize))
		return err
	}

	// Stream events line by line
	ctx := r.Context()
	maxLineSize := sh.config.MaxEventSize
	if maxLineSize <= 0 {
		maxLineSize = 1024 * 1024 // 1MB default
	}
	reader := bufio.NewReader(io.LimitReader(resp.Body, maxLineSize))
	for {
		// Read line with timeout handling
		lineCh := make(chan readLineResult, 1)
		go func() {
			line, err := sh.readLineWithTimeout(ctx, reader, sh.config.IdleTimeout, func() { resp.Body.Close() })
			lineCh <- readLineResult{line: line, err: err}
		}()

		select {
		case <-ctx.Done():
			// Client disconnected, stop streaming
			return ctx.Err()
		case res := <-lineCh:
			if res.err != nil {
				if res.err == io.EOF {
					return nil
				}
				// Check if it's a timeout (normal for SSE idle connections)
				if netErr, ok := res.err.(net.Error); ok && netErr.Timeout() {
					// Send a keepalive comment and continue
					if _, writeErr := w.Write([]byte(":keepalive\n")); writeErr != nil {
						return writeErr
					}
					flusher.Flush()
					return nil
				}
				return res.err
			}

			// Write the line
			if _, writeErr := w.Write(res.line); writeErr != nil {
				return writeErr
			}

			// Flush after each line (critical for SSE)
			flusher.Flush()
		}
	}
}

// readLineResult holds the result of an async line read.
type readLineResult struct {
	line []byte
	err  error
}

// readLineWithTimeout reads a line with a timeout.
// If a cancel function is provided, it is called on timeout so the caller
// can unblock the underlying reader (e.g. by closing resp.Body).
// The drain goroutine uses the provided context to bound its lifetime
// rather than potentially blocking forever.
func (sh *SSEHandler) readLineWithTimeout(ctx context.Context, reader *bufio.Reader, timeout time.Duration, onCancel func()) ([]byte, error) {
	if timeout > 0 {
		type result struct {
			line []byte
			err  error
		}
		ch := make(chan result, 1)

		go func() {
			line, err := reader.ReadBytes('\n')
			ch <- result{line, err}
		}()

		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case res := <-ch:
			return res.line, res.err
		case <-timer.C:
			if onCancel != nil {
				onCancel()
			}
			// Drain the goroutine, but bound its lifetime to the context
			// so it doesn't leak if the underlying reader never returns.
			go func() {
				select {
				case <-ch:
				case <-ctx.Done():
				}
			}()
			return nil, &timeoutError{}
		}
	}

	return reader.ReadBytes('\n')
}

// timeoutError represents a timeout error.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

// copySSEHeaders copies headers from source to destination, excluding hop-by-hop.
func copySSEHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// copyRegularResponse copies a non-SSE response (bounded to prevent DoS).
func (sh *SSEHandler) copyRegularResponse(w http.ResponseWriter, resp *http.Response) error {
	copySSEHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	const maxRegularResponseSize = 64 * 1024 * 1024 // 64MB
	_, err := io.Copy(w, io.LimitReader(resp.Body, maxRegularResponseSize))
	return err
}

// SSEEvent represents a parsed Server-Sent Event.
type SSEEvent struct {
	ID    string
	Event string
	Data  []byte
	Retry int
}

// ParseSSEEvent parses a single SSE event from bytes.
func ParseSSEEvent(data []byte) (*SSEEvent, error) {
	event := &SSEEvent{}
	lines := bytes.Split(data, []byte("\n"))

	var dataLines [][]byte

	for _, line := range lines {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			continue
		}

		// Check for comment
		if line[0] == ':' {
			continue
		}

		// Parse field
		fieldBytes, value, found := bytes.Cut(line, []byte(":"))
		if !found {
			// Field with no value
			field := string(line)
			switch field {
			case "event":
				event.Event = ""
			case "data":
				dataLines = append(dataLines, []byte{})
			case "id":
				event.ID = ""
			case "retry":
				event.Retry = 0
			}
			continue
		}

		field := string(fieldBytes)

		// Strip leading space if present
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		switch field {
		case "event":
			event.Event = string(value)
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			event.ID = string(value)
		case "retry":
			// Parse retry as integer
			fmt.Sscanf(string(value), "%d", &event.Retry)
		}
	}

	// Join data lines with newlines
	if len(dataLines) > 0 {
		event.Data = bytes.Join(dataLines, []byte("\n"))
	}

	return event, nil
}

// FormatSSEEvent formats an SSE event for transmission.
func FormatSSEEvent(event *SSEEvent) []byte {
	var buf bytes.Buffer

	if event.ID != "" {
		fmt.Fprintf(&buf, "id: %s\n", event.ID)
	}

	if event.Event != "" {
		fmt.Fprintf(&buf, "event: %s\n", event.Event)
	}

	if event.Retry > 0 {
		fmt.Fprintf(&buf, "retry: %d\n", event.Retry)
	}

	// Write data lines
	if len(event.Data) > 0 {
		lines := bytes.Split(event.Data, []byte("\n"))
		for _, line := range lines {
			fmt.Fprintf(&buf, "data: %s\n", line)
		}
	}

	// Empty line to terminate event
	buf.WriteByte('\n')

	return buf.Bytes()
}

// SSEProxy wraps an HTTPProxy with SSE support.
type SSEProxy struct {
	httpProxy  *HTTPProxy
	sseHandler *SSEHandler
}

// NewSSEProxy creates a new proxy with SSE support.
func NewSSEProxy(httpProxy *HTTPProxy, sseConfig *SSEConfig) *SSEProxy {
	return &SSEProxy{
		httpProxy:  httpProxy,
		sseHandler: NewSSEHandler(sseConfig),
	}
}

// ServeHTTP implements http.Handler with SSE support.
func (sp *SSEProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is an SSE request
	if IsSSERequest(r) {
		// Get route match
		routeMatch, ok := sp.httpProxy.router.Match(r)
		if !ok {
			sp.httpProxy.getErrorHandler()(w, r, errors.New("route not found"))
			return
		}

		// Get backend pool
		pool := sp.httpProxy.poolManager.GetPool(routeMatch.Route.BackendPool)
		if pool == nil {
			sp.httpProxy.getErrorHandler()(w, r, errors.New("pool not found"))
			return
		}

		// Select backend
		backends := pool.GetHealthyBackends()
		if len(backends) == 0 {
			sp.httpProxy.getErrorHandler()(w, r, errors.New("no healthy backends"))
			return
		}

		selected := pool.GetBalancer().Next(nil, backends)
		backend.ReleaseHealthyBackends(backends)
		if selected == nil {
			sp.httpProxy.getErrorHandler()(w, r, errors.New("no backend available"))
			return
		}

		// Handle SSE
		if err := sp.sseHandler.HandleSSE(w, r, selected); err != nil {
			sp.httpProxy.getErrorHandler()(w, r, err)
		}
		return
	}

	// Not an SSE request, use regular HTTP proxy
	sp.httpProxy.ServeHTTP(w, r)
}
