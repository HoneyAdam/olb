package l7

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
)

// IsGRPCRequest checks if the request is a gRPC request.
func IsGRPCRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/grpc")
}

// GRPCConfig configures gRPC proxy behavior.
type GRPCConfig struct {
	// EnableGRPC enables gRPC proxying.
	EnableGRPC bool

	// MaxMessageSize is the maximum gRPC message size.
	MaxMessageSize int

	// Timeout is the default timeout for gRPC calls.
	Timeout time.Duration

	// EnableGRPWeb enables gRPC-Web support.
	EnableGRPCWeb bool
}

// DefaultGRPCConfig returns a default gRPC configuration.
func DefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		EnableGRPC:     true,
		MaxMessageSize: 4 * 1024 * 1024, // 4MB
		Timeout:        30 * time.Second,
		EnableGRPCWeb:  true,
	}
}

// GRPCHandler handles gRPC proxying.
type GRPCHandler struct {
	config    *GRPCConfig
	transport http.RoundTripper
}

// NewGRPCHandler creates a new gRPC handler.
func NewGRPCHandler(config *GRPCConfig) *GRPCHandler {
	if config == nil {
		config = DefaultGRPCConfig()
	}

	return &GRPCHandler{
		config:    config,
		transport: createGRPCTransport(),
	}
}

// createGRPCTransport creates an HTTP/2 transport for gRPC.
func createGRPCTransport() http.RoundTripper {
	// For gRPC, we need HTTP/2 support
	// The transport should support h2c (HTTP/2 without TLS)
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// Allow HTTP/2 without TLS (h2c)
		// This is done by not setting ForceAttemptHTTP2
	}
}

// HandleGRPC handles a gRPC request.
func (gh *GRPCHandler) HandleGRPC(w http.ResponseWriter, r *http.Request, b *backend.Backend) error {
	if !gh.config.EnableGRPC {
		return fmt.Errorf("gRPC disabled")
	}

	// Acquire connection slot
	if !b.AcquireConn() {
		return fmt.Errorf("backend at max connections")
	}
	defer b.ReleaseConn()

	// Prepare outbound request
	outReq, err := gh.prepareGRPCRequest(r, b)
	if err != nil {
		return err
	}

	// Set timeout
	ctx := outReq.Context()
	if gh.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, gh.config.Timeout)
		defer cancel()
		outReq = outReq.WithContext(ctx)
	}

	// Execute request
	resp, err := gh.transport.RoundTrip(outReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Copy response headers (excluding trailers)
	copyGRPCHeaders(w.Header(), resp.Header)

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}

	// Copy trailers if present
	if trailers := resp.Trailer; len(trailers) > 0 {
		for key, values := range trailers {
			for _, value := range values {
				w.Header().Add(http.TrailerPrefix+key, value)
			}
		}
	}

	return nil
}

// prepareGRPCRequest creates the outbound gRPC request.
func (gh *GRPCHandler) prepareGRPCRequest(r *http.Request, b *backend.Backend) (*http.Request, error) {
	// Clone the request
	outReq := r.Clone(r.Context())

	// Set the URL to point to the backend
	backendURL, err := url.Parse("http://" + b.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid backend address: %w", err)
	}

	outReq.URL.Scheme = backendURL.Scheme
	outReq.URL.Host = backendURL.Host
	outReq.Host = r.Host
	outReq.RequestURI = ""

	// Set X-Forwarded-For
	clientIP := getClientIP(r)
	if prior := outReq.Header.Get("X-Forwarded-For"); prior != "" {
		outReq.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	// Set X-Real-IP
	outReq.Header.Set("X-Real-IP", clientIP)

	// Set X-Forwarded-Proto
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	outReq.Header.Set("X-Forwarded-Proto", proto)

	// Ensure HTTP/2 for gRPC
	// gRPC requires HTTP/2
	outReq.Proto = "HTTP/2.0"
	outReq.ProtoMajor = 2
	outReq.ProtoMinor = 0

	return outReq, nil
}

// copyGRPCHeaders copies headers from source to destination, excluding hop-by-hop and trailers.
func copyGRPCHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		// Skip trailer headers (they'll be handled separately)
		if strings.HasPrefix(key, "Trailer") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// IsGRPCWebRequest checks if the request is a gRPC-Web request.
func IsGRPCWebRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/grpc-web")
}

// GRPCWebHandler handles gRPC-Web proxying (converts gRPC-Web to gRPC).
type GRPCWebHandler struct {
	grpcHandler *GRPCHandler
}

// NewGRPCWebHandler creates a new gRPC-Web handler.
func NewGRPCWebHandler(grpcHandler *GRPCHandler) *GRPCWebHandler {
	return &GRPCWebHandler{
		grpcHandler: grpcHandler,
	}
}

// HandleGRPCWeb handles a gRPC-Web request.
func (gwh *GRPCWebHandler) HandleGRPCWeb(w http.ResponseWriter, r *http.Request, b *backend.Backend) error {
	if !gwh.grpcHandler.config.EnableGRPCWeb {
		return fmt.Errorf("gRPC-Web disabled")
	}

	// For now, delegate to gRPC handler
	// In a full implementation, this would handle the gRPC-Web framing
	return gwh.grpcHandler.HandleGRPC(w, r, b)
}

// GRPCStatus represents a gRPC status code.
type GRPCStatus int

const (
	// GRPCStatusOK indicates success.
	GRPCStatusOK GRPCStatus = 0
	// GRPCStatusCancelled indicates the operation was cancelled.
	GRPCStatusCancelled GRPCStatus = 1
	// GRPCStatusUnknown indicates an unknown error.
	GRPCStatusUnknown GRPCStatus = 2
	// GRPCStatusInvalidArgument indicates an invalid argument.
	GRPCStatusInvalidArgument GRPCStatus = 3
	// GRPCStatusDeadlineExceeded indicates the deadline was exceeded.
	GRPCStatusDeadlineExceeded GRPCStatus = 4
	// GRPCStatusNotFound indicates the requested entity was not found.
	GRPCStatusNotFound GRPCStatus = 5
	// GRPCStatusAlreadyExists indicates the entity already exists.
	GRPCStatusAlreadyExists GRPCStatus = 6
	// GRPCStatusPermissionDenied indicates permission denied.
	GRPCStatusPermissionDenied GRPCStatus = 7
	// GRPCStatusResourceExhausted indicates resource exhaustion.
	GRPCStatusResourceExhausted GRPCStatus = 8
	// GRPCStatusFailedPrecondition indicates a failed precondition.
	GRPCStatusFailedPrecondition GRPCStatus = 9
	// GRPCStatusAborted indicates the operation was aborted.
	GRPCStatusAborted GRPCStatus = 10
	// GRPCStatusOutOfRange indicates the value is out of range.
	GRPCStatusOutOfRange GRPCStatus = 11
	// GRPCStatusUnimplemented indicates the operation is unimplemented.
	GRPCStatusUnimplemented GRPCStatus = 12
	// GRPCStatusInternal indicates an internal error.
	GRPCStatusInternal GRPCStatus = 13
	// GRPCStatusUnavailable indicates the service is unavailable.
	GRPCStatusUnavailable GRPCStatus = 14
	// GRPCStatusDataLoss indicates data loss.
	GRPCStatusDataLoss GRPCStatus = 15
	// GRPCStatusUnauthenticated indicates the caller is unauthenticated.
	GRPCStatusUnauthenticated GRPCStatus = 16
)

// HTTPStatusToGRPCStatus converts an HTTP status code to a gRPC status code.
func HTTPStatusToGRPCStatus(httpStatus int) GRPCStatus {
	switch httpStatus {
	case http.StatusOK:
		return GRPCStatusOK
	case http.StatusBadRequest:
		return GRPCStatusInvalidArgument
	case http.StatusUnauthorized:
		return GRPCStatusUnauthenticated
	case http.StatusForbidden:
		return GRPCStatusPermissionDenied
	case http.StatusNotFound:
		return GRPCStatusNotFound
	case http.StatusTooManyRequests:
		return GRPCStatusResourceExhausted
	case http.StatusInternalServerError:
		return GRPCStatusInternal
	case http.StatusNotImplemented:
		return GRPCStatusUnimplemented
	case http.StatusBadGateway:
		return GRPCStatusUnavailable
	case http.StatusServiceUnavailable:
		return GRPCStatusUnavailable
	case http.StatusGatewayTimeout:
		return GRPCStatusDeadlineExceeded
	default:
		return GRPCStatusUnknown
	}
}

// GRPCStatusToHTTPStatus converts a gRPC status code to an HTTP status code.
func GRPCStatusToHTTPStatus(grpcStatus GRPCStatus) int {
	switch grpcStatus {
	case GRPCStatusOK:
		return http.StatusOK
	case GRPCStatusCancelled:
		return 499 // Client Closed Request (nginx convention)
	case GRPCStatusUnknown:
		return http.StatusInternalServerError
	case GRPCStatusInvalidArgument:
		return http.StatusBadRequest
	case GRPCStatusDeadlineExceeded:
		return http.StatusGatewayTimeout
	case GRPCStatusNotFound:
		return http.StatusNotFound
	case GRPCStatusAlreadyExists:
		return http.StatusConflict
	case GRPCStatusPermissionDenied:
		return http.StatusForbidden
	case GRPCStatusResourceExhausted:
		return http.StatusTooManyRequests
	case GRPCStatusFailedPrecondition:
		return http.StatusPreconditionFailed
	case GRPCStatusAborted:
		return 409 // Conflict
	case GRPCStatusOutOfRange:
		return http.StatusBadRequest
	case GRPCStatusUnimplemented:
		return http.StatusNotImplemented
	case GRPCStatusInternal:
		return http.StatusInternalServerError
	case GRPCStatusUnavailable:
		return http.StatusServiceUnavailable
	case GRPCStatusDataLoss:
		return http.StatusInternalServerError
	case GRPCStatusUnauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// gRPCFrame represents a gRPC frame header.
type gRPCFrame struct {
	Compressed bool
	Length     uint32
	Data       []byte
}

// parseGRPCFrame parses a gRPC frame from the reader.
func parseGRPCFrame(r io.Reader) (*gRPCFrame, error) {
	// gRPC frame format:
	// 1 byte: flags (compressed flag)
	// 4 bytes: message length (big-endian)
	// N bytes: message data

	buf := make([]byte, 5)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	compressed := buf[0] == 1
	length := uint32(buf[1])<<24 | uint32(buf[2])<<16 | uint32(buf[3])<<8 | uint32(buf[4])

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	return &gRPCFrame{
		Compressed: compressed,
		Length:     length,
		Data:       data,
	}, nil
}

// writeGRPCFrame writes a gRPC frame to the writer.
func writeGRPCFrame(w io.Writer, frame *gRPCFrame) error {
	buf := make([]byte, 5+len(frame.Data))
	if frame.Compressed {
		buf[0] = 1
	} else {
		buf[0] = 0
	}
	buf[1] = byte(frame.Length >> 24)
	buf[2] = byte(frame.Length >> 16)
	buf[3] = byte(frame.Length >> 8)
	buf[4] = byte(frame.Length)
	copy(buf[5:], frame.Data)

	_, err := w.Write(buf)
	return err
}
