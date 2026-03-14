//go:build linux
// +build linux

package l4

import (
	"io"
	"net"
	"os"
	"syscall"
)

// maxSpliceSize is the maximum number of bytes to splice at once.
const maxSpliceSize = 1 << 20 // 1MB

// spliceMove is the SPLICE_F_MOVE flag value (not exported in Go's syscall package).
const spliceMove = 1

// zeroCopyTransfer attempts to use splice for zero-copy transfer on Linux.
// It returns the number of bytes transferred and whether splice was used.
func zeroCopyTransfer(dst, src net.Conn) (int64, bool, error) {
	// Get underlying TCP connections
	tcpSrc := getTCPConn(src)
	tcpDst := getTCPConn(dst)

	if tcpSrc == nil || tcpDst == nil {
		return 0, false, nil
	}

	// Get file descriptors
	srcFile, err := tcpSrc.File()
	if err != nil {
		return 0, false, nil
	}
	defer srcFile.Close()

	dstFile, err := tcpDst.File()
	if err != nil {
		return 0, false, nil
	}
	defer dstFile.Close()

	// Create a pipe for splicing
	pr, pw, err := os.Pipe()
	if err != nil {
		return 0, false, nil
	}
	defer pr.Close()
	defer pw.Close()

	prFd := int(pr.Fd())
	pwFd := int(pw.Fd())

	// Set non-blocking
	syscall.SetNonblock(prFd, true)
	syscall.SetNonblock(pwFd, true)

	srcFd := int(srcFile.Fd())
	dstFd := int(dstFile.Fd())

	var total int64

	for {
		// Splice from source to pipe
		n, err := syscall.Splice(srcFd, nil, pwFd, nil, maxSpliceSize, spliceMove)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				continue
			}
			return total, total > 0, err
		}
		if n == 0 {
			break
		}

		// Splice from pipe to destination
		for n > 0 {
			n2, err := syscall.Splice(prFd, nil, dstFd, nil, int(n), spliceMove)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					continue
				}
				return total, total > 0, err
			}
			n -= n2
			total += int64(n2)
		}
	}

	return total, true, nil
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

// copyWithZeroCopy attempts zero-copy transfer, falls back to buffer copy.
func copyWithZeroCopy(dst, src net.Conn) (int64, error) {
	// Try zero-copy first
	n, used, err := zeroCopyTransfer(dst, src)
	if used {
		return n, err
	}

	// Fall back to regular copy
	buf := make([]byte, 32*1024)
	return io.CopyBuffer(dst, src, buf)
}
