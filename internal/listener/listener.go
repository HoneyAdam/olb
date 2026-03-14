// Package listener provides HTTP/HTTPS/TCP listeners for OpenLoadBalancer.
// It supports graceful shutdown, connection management, and configurable timeouts.
package listener

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Listener is the interface for all listeners (HTTP, HTTPS, TCP, etc.)
type Listener interface {
	// Start begins listening for connections
	Start() error

	// Stop gracefully shuts down the listener
	Stop(ctx context.Context) error

	// Name returns the listener name
	Name() string

	// Address returns the listener address (host:port)
	Address() string

	// IsRunning returns true if the listener is active
	IsRunning() bool
}

// Common configuration options
type Options struct {
	Name           string
	Address        string
	Handler        http.Handler
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	HeaderTimeout  time.Duration
	MaxHeaderBytes int
	ConnManager    ConnManager // Optional connection manager for tracking
}

// ConnManager is the interface for connection management.
// This is typically satisfied by *conn.Manager.
type ConnManager interface {
	// Accept wraps a connection with tracking
	Accept(conn net.Conn) (net.Conn, error)
}

// DefaultOptions returns default options for a listener.
func DefaultOptions() *Options {
	return &Options{
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		HeaderTimeout:  10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}

// mergeOptions merges user-provided options with defaults.
func mergeOptions(opts *Options) *Options {
	if opts == nil {
		return DefaultOptions()
	}

	defaults := DefaultOptions()

	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = defaults.ReadTimeout
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = defaults.WriteTimeout
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = defaults.IdleTimeout
	}
	if opts.HeaderTimeout == 0 {
		opts.HeaderTimeout = defaults.HeaderTimeout
	}
	if opts.MaxHeaderBytes == 0 {
		opts.MaxHeaderBytes = defaults.MaxHeaderBytes
	}

	return opts
}
