//go:build !linux
// +build !linux

package l4

import (
	"io"
	"net"
)

// zeroCopyTransfer is not supported on non-Linux platforms.
// It always returns false for the second return value.
func zeroCopyTransfer(dst, src net.Conn) (int64, bool, error) {
	return 0, false, nil
}

// getTCPConn extracts the underlying TCP connection.
func getTCPConn(conn net.Conn) *net.TCPConn {
	switch c := conn.(type) {
	case *net.TCPConn:
		return c
	default:
		return nil
	}
}

// copyWithZeroCopy falls back to buffer copy on non-Linux platforms.
func copyWithZeroCopy(dst, src net.Conn) (int64, error) {
	buf := make([]byte, 32*1024)
	return io.CopyBuffer(dst, src, buf)
}
