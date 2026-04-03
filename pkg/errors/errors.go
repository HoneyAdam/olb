// Package errors provides sentinel errors and error wrapping helpers for OpenLoadBalancer.
// It follows Go best practices for error handling with context and error codes.
package errors

import (
	"fmt"
	"maps"
)

// Code is an error code for API responses and categorization.
type Code int

// Error codes organized by category.
const (
	// General errors (1-99)
	CodeUnknown      Code = 0
	CodeInternal     Code = 1
	CodeInvalidArg   Code = 2
	CodeNotFound     Code = 3
	CodeAlreadyExist Code = 4
	CodeUnavailable  Code = 5
	CodeTimeout      Code = 6
	CodeCanceled     Code = 7

	// Configuration errors (100-199)
	CodeConfigInvalid    Code = 100
	CodeConfigParseError Code = 101
	CodeConfigNotFound   Code = 102
	CodeConfigValidation Code = 103

	// Backend/Pool errors (200-299)
	CodeBackendNotFound    Code = 200
	CodePoolNotFound       Code = 201
	CodeRouteNotFound      Code = 202
	CodeBackendUnavailable Code = 203
	CodeBackendUnhealthy   Code = 204
	CodePoolEmpty          Code = 205

	// Connection errors (300-399)
	CodeConnectionRefused Code = 300
	CodeConnectionReset   Code = 301
	CodeConnectionTimeout Code = 302
	CodeConnectionClosed  Code = 303
	CodeTLSHandshake      Code = 304
	CodeCertificate       Code = 305

	// Protocol errors (400-499)
	CodeProtocolError   Code = 400
	CodeInvalidRequest  Code = 401
	CodeInvalidResponse Code = 402
	CodeHTTPError       Code = 403

	// Middleware errors (500-599)
	CodeRateLimitExceeded Code = 500
	CodeCircuitOpen       Code = 501
	CodeAuthRequired      Code = 502
	CodeAuthFailed        Code = 503
	CodeForbidden         Code = 504
	CodeWAFBlocked        Code = 505

	// Cluster errors (600-699)
	CodeClusterNotReady Code = 600
	CodeNodeNotFound    Code = 601
	CodeRaftError       Code = 602
	CodeGossipError     Code = 603
)

// String returns the string representation of the error code.
func (c Code) String() string {
	switch c {
	case CodeUnknown:
		return "UNKNOWN"
	case CodeInternal:
		return "INTERNAL"
	case CodeInvalidArg:
		return "INVALID_ARGUMENT"
	case CodeNotFound:
		return "NOT_FOUND"
	case CodeAlreadyExist:
		return "ALREADY_EXISTS"
	case CodeUnavailable:
		return "UNAVAILABLE"
	case CodeTimeout:
		return "TIMEOUT"
	case CodeCanceled:
		return "CANCELED"
	case CodeConfigInvalid:
		return "CONFIG_INVALID"
	case CodeConfigParseError:
		return "CONFIG_PARSE_ERROR"
	case CodeConfigNotFound:
		return "CONFIG_NOT_FOUND"
	case CodeConfigValidation:
		return "CONFIG_VALIDATION"
	case CodeBackendNotFound:
		return "BACKEND_NOT_FOUND"
	case CodePoolNotFound:
		return "POOL_NOT_FOUND"
	case CodeRouteNotFound:
		return "ROUTE_NOT_FOUND"
	case CodeBackendUnavailable:
		return "BACKEND_UNAVAILABLE"
	case CodeBackendUnhealthy:
		return "BACKEND_UNHEALTHY"
	case CodePoolEmpty:
		return "POOL_EMPTY"
	case CodeConnectionRefused:
		return "CONNECTION_REFUSED"
	case CodeConnectionReset:
		return "CONNECTION_RESET"
	case CodeConnectionTimeout:
		return "CONNECTION_TIMEOUT"
	case CodeConnectionClosed:
		return "CONNECTION_CLOSED"
	case CodeTLSHandshake:
		return "TLS_HANDSHAKE"
	case CodeCertificate:
		return "CERTIFICATE"
	case CodeProtocolError:
		return "PROTOCOL_ERROR"
	case CodeInvalidRequest:
		return "INVALID_REQUEST"
	case CodeInvalidResponse:
		return "INVALID_RESPONSE"
	case CodeHTTPError:
		return "HTTP_ERROR"
	case CodeRateLimitExceeded:
		return "RATE_LIMIT_EXCEEDED"
	case CodeCircuitOpen:
		return "CIRCUIT_OPEN"
	case CodeAuthRequired:
		return "AUTH_REQUIRED"
	case CodeAuthFailed:
		return "AUTH_FAILED"
	case CodeForbidden:
		return "FORBIDDEN"
	case CodeWAFBlocked:
		return "WAF_BLOCKED"
	case CodeClusterNotReady:
		return "CLUSTER_NOT_READY"
	case CodeNodeNotFound:
		return "NODE_NOT_FOUND"
	case CodeRaftError:
		return "RAFT_ERROR"
	case CodeGossipError:
		return "GOSSIP_ERROR"
	default:
		return fmt.Sprintf("CODE_%d", c)
	}
}

// Sentinel errors for common conditions.
// Use errors.Is() to check for these errors.
var (
	// General errors
	ErrUnknown      = New(CodeUnknown, "unknown error")
	ErrInternal     = New(CodeInternal, "internal error")
	ErrInvalidArg   = New(CodeInvalidArg, "invalid argument")
	ErrNotFound     = New(CodeNotFound, "not found")
	ErrAlreadyExist = New(CodeAlreadyExist, "already exists")
	ErrUnavailable  = New(CodeUnavailable, "unavailable")
	ErrTimeout      = New(CodeTimeout, "timeout")
	ErrCanceled     = New(CodeCanceled, "canceled")

	// Configuration errors
	ErrConfigInvalid    = New(CodeConfigInvalid, "invalid configuration")
	ErrConfigParseError = New(CodeConfigParseError, "configuration parse error")
	ErrConfigNotFound   = New(CodeConfigNotFound, "configuration not found")
	ErrConfigValidation = New(CodeConfigValidation, "configuration validation failed")

	// Backend/Pool errors
	ErrBackendNotFound    = New(CodeBackendNotFound, "backend not found")
	ErrPoolNotFound       = New(CodePoolNotFound, "pool not found")
	ErrRouteNotFound      = New(CodeRouteNotFound, "route not found")
	ErrBackendUnavailable = New(CodeBackendUnavailable, "backend unavailable")
	ErrBackendUnhealthy   = New(CodeBackendUnhealthy, "backend unhealthy")
	ErrPoolEmpty          = New(CodePoolEmpty, "pool is empty")

	// Connection errors
	ErrConnectionRefused = New(CodeConnectionRefused, "connection refused")
	ErrConnectionReset   = New(CodeConnectionReset, "connection reset")
	ErrConnectionTimeout = New(CodeConnectionTimeout, "connection timeout")
	ErrConnectionClosed  = New(CodeConnectionClosed, "connection closed")
	ErrTLSHandshake      = New(CodeTLSHandshake, "TLS handshake failed")
	ErrCertificate       = New(CodeCertificate, "certificate error")

	// Protocol errors
	ErrProtocolError   = New(CodeProtocolError, "protocol error")
	ErrInvalidRequest  = New(CodeInvalidRequest, "invalid request")
	ErrInvalidResponse = New(CodeInvalidResponse, "invalid response")
	ErrHTTPError       = New(CodeHTTPError, "HTTP error")

	// Middleware errors
	ErrRateLimitExceeded = New(CodeRateLimitExceeded, "rate limit exceeded")
	ErrCircuitOpen       = New(CodeCircuitOpen, "circuit breaker open")
	ErrAuthRequired      = New(CodeAuthRequired, "authentication required")
	ErrAuthFailed        = New(CodeAuthFailed, "authentication failed")
	ErrForbidden         = New(CodeForbidden, "forbidden")
	ErrWAFBlocked        = New(CodeWAFBlocked, "blocked by WAF")

	// Cluster errors
	ErrClusterNotReady = New(CodeClusterNotReady, "cluster not ready")
	ErrNodeNotFound    = New(CodeNodeNotFound, "node not found")
	ErrRaftError       = New(CodeRaftError, "raft error")
	ErrGossipError     = New(CodeGossipError, "gossip error")
)

// Error is a structured error with code, message, and context.
type Error struct {
	Code    Code
	Message string
	Cause   error
	Context map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code.String(), e.Message)
}

// Unwrap returns the cause of the error for errors.Is/As support.
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithContext adds context fields to the error.
// Returns the same error for chaining.
func (e *Error) WithContext(key string, value any) *Error {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}

// WithContextMap adds multiple context fields at once.
func (e *Error) WithContextMap(ctx map[string]any) *Error {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	maps.Copy(e.Context, ctx)
	return e
}

// Is reports whether this error matches the target.
// Matches if codes are equal, or if this error wraps the target.
func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is an *Error - match by code
	if te, ok := target.(*Error); ok {
		if e.Code == te.Code {
			return true
		}
	}

	// Direct comparison
	if e == target {
		return true
	}

	// Check the cause chain
	if e.Cause != nil {
		if e.Cause == target {
			return true
		}
		// Check if cause is *Error and matches by code
		if ce, ok := e.Cause.(*Error); ok {
			if ce.Is(target) {
				return true
			}
		}
	}

	return false
}

// New creates a new error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Newf creates a new error with a formatted message.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an existing error with a code and message.
// If err is nil, returns nil.
func Wrap(err error, code Code, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// Wrapf wraps an existing error with a formatted message.
func Wrapf(err error, code Code, format string, args ...any) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
	}
}

// Is reports whether err matches the target.
// Supports *Error code matching and standard error equality.
func Is(err, target error) bool {
	// Both nil means no error to match - return false
	if err == nil && target == nil {
		return false
	}

	if target == nil {
		return err == nil
	}

	if err == nil {
		return false
	}

	// Direct comparison
	if err == target {
		return true
	}

	// Check if err has Is method
	if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
		return true
	}

	// Unwrap if possible
	if x, ok := err.(interface{ Unwrap() error }); ok {
		return Is(x.Unwrap(), target)
	}

	return false
}

// As attempts to convert err to target type.
func As(err error, target any) bool {
	if target == nil {
		panic("errors.As: target cannot be nil")
	}

	// Try direct assign
	if x, ok := target.(**Error); ok {
		if e, ok := err.(*Error); ok {
			*x = e
			return true
		}
	}

	// Check if err has As method
	if x, ok := err.(interface{ As(any) bool }); ok {
		return x.As(target)
	}

	// Unwrap and try again
	if x, ok := err.(interface{ Unwrap() error }); ok {
		return As(x.Unwrap(), target)
	}

	return false
}

// CodeOf returns the error code for the given error.
// Returns CodeUnknown if the error is nil or not an *Error.
func CodeOf(err error) Code {
	if err == nil {
		return CodeUnknown
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return CodeUnknown
}

// IsCode reports whether the error has the given code.
func IsCode(err error, code Code) bool {
	return CodeOf(err) == code
}

// WithContext wraps an error with context fields.
// If err is nil or not an *Error, returns as-is.
func WithContext(err error, key string, value any) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e.WithContext(key, value)
	}
	return err
}
