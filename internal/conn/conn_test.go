package conn

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestNewTrackedConn(t *testing.T) {
	// Create a test connection
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	closed := false
	tracked := NewTrackedConn(rawConn, "test-conn", func() {
		closed = true
	})

	if tracked.ID() != "test-conn" {
		t.Errorf("ID() = %v, want %v", tracked.ID(), "test-conn")
	}

	if tracked.IsClosed() {
		t.Error("New connection should not be closed")
	}

	if closed {
		t.Error("onClose should not be called yet")
	}

	// Test CreatedAt
	if time.Since(tracked.CreatedAt()) > time.Second {
		t.Error("CreatedAt() should be recent")
	}

	tracked.Close()

	if !closed {
		t.Error("onClose should be called after Close()")
	}

	if !tracked.IsClosed() {
		t.Error("Connection should be closed after Close()")
	}
}

func TestTrackedConn_ReadWrite(t *testing.T) {
	// Create a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			if n > 0 {
				conn.Write(buf[:n])
			}
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	tracked := NewTrackedConn(rawConn, "test-conn", nil)

	// Test Write
	data := []byte("hello world")
	n, err := tracked.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}

	if tracked.BytesOut() != int64(len(data)) {
		t.Errorf("BytesOut() = %d, want %d", tracked.BytesOut(), len(data))
	}

	// Test Read
	buf := make([]byte, 1024)
	n, err = tracked.Read(buf)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if string(buf[:n]) != string(data) {
		t.Errorf("Read() = %s, want %s", string(buf[:n]), string(data))
	}

	if tracked.BytesIn() != int64(n) {
		t.Errorf("BytesIn() = %d, want %d", tracked.BytesIn(), n)
	}
}

func TestTrackedConn_BackendID(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	tracked := NewTrackedConn(rawConn, "test-conn", nil)

	if tracked.BackendID() != "" {
		t.Error("BackendID() should be empty initially")
	}

	tracked.SetBackendID("backend-1")

	if tracked.BackendID() != "backend-1" {
		t.Errorf("BackendID() = %v, want %v", tracked.BackendID(), "backend-1")
	}
}

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	if mgr.ActiveCount() != 0 {
		t.Errorf("ActiveCount() initial = %v, want 0", mgr.ActiveCount())
	}
}

func TestManager_Accept(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   2,
	}
	mgr := NewManager(config)

	// Create a test connection
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
				return
			}
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	tracked, err := mgr.Accept(rawConn)
	if err != nil {
		t.Errorf("Accept() error = %v", err)
	}
	if tracked == nil {
		t.Fatal("Accept() returned nil")
	}

	if mgr.ActiveCount() != 1 {
		t.Errorf("ActiveCount() after accept = %v, want 1", mgr.ActiveCount())
	}

	// Close and verify cleanup
	tracked.Close()

	// Wait a bit for cleanup
	time.Sleep(10 * time.Millisecond)

	if mgr.ActiveCount() != 0 {
		t.Errorf("ActiveCount() after close = %v, want 0", mgr.ActiveCount())
	}
}

func TestManager_Accept_Limits(t *testing.T) {
	config := &Config{
		MaxConnections: 2,
		MaxPerSource:   1,
	}
	mgr := NewManager(config)

	// Create test connections
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for i := 0; i < 3; i++ {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}
	}()

	// First connection should succeed
	conn1, _ := net.Dial("tcp", listener.Addr().String())
	if conn1 == nil {
		t.Fatal("Failed to create first connection")
	}
	defer conn1.Close()

	tracked1, err := mgr.Accept(conn1)
	if err != nil {
		t.Errorf("First Accept() error = %v", err)
	}

	// Close first tracked connection to clean up
	tracked1.Close()
	time.Sleep(10 * time.Millisecond)
}

func TestManager_AssociateBackend(t *testing.T) {
	config := &Config{
		MaxPerBackend: 10,
	}
	mgr := NewManager(config)

	// Create a test connection
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	tracked, _ := mgr.Accept(rawConn)

	err = mgr.AssociateBackend(tracked.ID(), "backend-1")
	if err != nil {
		t.Errorf("AssociateBackend() error = %v", err)
	}

	if tracked.BackendID() != "backend-1" {
		t.Errorf("BackendID() = %v, want %v", tracked.BackendID(), "backend-1")
	}

	if mgr.BackendCount("backend-1") != 1 {
		t.Errorf("BackendCount() = %v, want 1", mgr.BackendCount("backend-1"))
	}
}

func TestManager_Drain(t *testing.T) {
	config := &Config{
		DrainTimeout: 1 * time.Second,
	}
	mgr := NewManager(config)

	// Create test connections
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
				return
			}
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	tracked, _ := mgr.Accept(rawConn)

	// Drain should timeout since connection is still open
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = mgr.Drain(ctx)
	if err == nil {
		t.Error("Drain() should timeout with active connections")
	}

	// Close connection and try again
	tracked.Close()
	rawConn.Close()

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	err = mgr.Drain(ctx2)
	if err != nil {
		t.Errorf("Drain() after close error = %v", err)
	}
}

func TestManager_CloseAll(t *testing.T) {
	mgr := NewManager(nil)

	// Create test connections
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
				return
			}
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	tracked, _ := mgr.Accept(rawConn)

	if !tracked.IsClosed() {
		mgr.CloseAll()
	}

	if !tracked.IsClosed() {
		t.Error("CloseAll() should close all connections")
	}
}

func TestManager_GetConnection(t *testing.T) {
	mgr := NewManager(nil)

	// Non-existent connection
	conn := mgr.GetConnection("nonexistent")
	if conn != nil {
		t.Error("GetConnection() for non-existent should return nil")
	}
}

func BenchmarkManager_Accept(b *testing.B) {
	mgr := NewManager(nil)

	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				conn.Close()
			}
		}
	}()

	addr := listener.Addr().String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rawConn, _ := net.Dial("tcp", addr)
		if rawConn != nil {
			tracked, _ := mgr.Accept(rawConn)
			if tracked != nil {
				tracked.Close()
			}
		}
	}
}

// TestTrackedConn_ReadWriteErrors tests Read and Write error handling
func TestTrackedConn_ReadWriteErrors(t *testing.T) {
	// Create a test server that closes immediately
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close() // Close immediately
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	tracked := NewTrackedConn(rawConn, "test-conn", nil)

	// Read should return error since connection is closed
	buf := make([]byte, 1024)
	_, err = tracked.Read(buf)
	if err == nil {
		t.Error("Read() should return error on closed connection")
	}

	// BytesIn should still be 0 since no bytes were read
	if tracked.BytesIn() != 0 {
		t.Errorf("BytesIn() = %d, want 0", tracked.BytesIn())
	}
}

// TestTrackedConn_MultipleClose tests that Close is idempotent
func TestTrackedConn_MultipleClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	closeCount := 0
	tracked := NewTrackedConn(rawConn, "test-conn", func() {
		closeCount++
	})

	// First close
	err = tracked.Close()
	if err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should succeed (idempotent)
	err = tracked.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}

	// onClose should only be called once
	if closeCount != 1 {
		t.Errorf("onClose called %d times, want 1", closeCount)
	}
}

// TestTrackedConn_ConcurrentReadWrite tests concurrent access
func TestTrackedConn_ConcurrentReadWrite(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverReady := make(chan bool)
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			close(serverReady)
			buf := make([]byte, 1024)
			for {
				n, _ := conn.Read(buf)
				if n > 0 {
					conn.Write(buf[:n])
				}
				if n <= 0 {
					return
				}
			}
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	<-serverReady

	tracked := NewTrackedConn(rawConn, "test-conn", nil)

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			data := []byte("hello")
			tracked.Write(data)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 50 bytes out
	if tracked.BytesOut() != 50 {
		t.Errorf("BytesOut() = %d, want 50", tracked.BytesOut())
	}
}

// TestTrackedConn_Stats tests Stats method
func TestTrackedConn_Stats(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			buf := make([]byte, 1024)
			conn.Read(buf)
			conn.Write([]byte("response"))
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	tracked := NewTrackedConn(rawConn, "test-conn-123", nil)
	tracked.SetBackendID("backend-1")

	// Do some I/O
	tracked.Write([]byte("hello"))
	buf := make([]byte, 1024)
	tracked.Read(buf)

	stats := tracked.Stats()

	if stats.ID != "test-conn-123" {
		t.Errorf("Stats.ID = %v, want %v", stats.ID, "test-conn-123")
	}
	if stats.BackendID != "backend-1" {
		t.Errorf("Stats.BackendID = %v, want %v", stats.BackendID, "backend-1")
	}
	if stats.BytesOut != 5 {
		t.Errorf("Stats.BytesOut = %d, want 5", stats.BytesOut)
	}
	if stats.BytesIn != 8 {
		t.Errorf("Stats.BytesIn = %d, want 8", stats.BytesIn)
	}
	if stats.Duration < 0 {
		t.Error("Stats.Duration should be >= 0")
	}
}

// TestManager_Accept_GlobalLimitExceeded tests global connection limit
func TestManager_Accept_GlobalLimitExceeded(t *testing.T) {
	config := &Config{
		MaxConnections: 1,
		MaxPerSource:   10,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
				return
			}
		}
	}()

	// First connection should succeed
	conn1, _ := net.Dial("tcp", listener.Addr().String())
	if conn1 == nil {
		t.Fatal("Failed to create first connection")
	}
	defer conn1.Close()

	tracked1, err := mgr.Accept(conn1)
	if err != nil {
		t.Errorf("First Accept() error = %v", err)
	}
	if tracked1 == nil {
		t.Fatal("First Accept() returned nil")
	}

	// Second connection should fail due to global limit
	conn2, _ := net.Dial("tcp", listener.Addr().String())
	if conn2 == nil {
		t.Fatal("Failed to create second connection")
	}

	_, err = mgr.Accept(conn2)
	if err == nil {
		t.Error("Second Accept() should fail due to global limit")
	}
	if err.Error() != "global connection limit exceeded" {
		t.Errorf("Expected 'global connection limit exceeded', got %v", err)
	}
}

// TestManager_Accept_PerSourceLimitExceeded tests per-source connection limit
func TestManager_Accept_PerSourceLimitExceeded(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   1,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for i := 0; i < 2; i++ {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}
	}()

	// First connection should succeed
	conn1, _ := net.Dial("tcp", listener.Addr().String())
	if conn1 == nil {
		t.Fatal("Failed to create first connection")
	}
	defer conn1.Close()

	tracked1, err := mgr.Accept(conn1)
	if err != nil {
		t.Errorf("First Accept() error = %v", err)
	}
	if tracked1 == nil {
		t.Fatal("First Accept() returned nil")
	}

	// Second connection from same source should fail
	conn2, _ := net.Dial("tcp", listener.Addr().String())
	if conn2 == nil {
		t.Fatal("Failed to create second connection")
	}

	_, err = mgr.Accept(conn2)
	if err == nil {
		t.Error("Second Accept() should fail due to per-source limit")
	}
	if err.Error() != "per-source connection limit exceeded" {
		t.Errorf("Expected 'per-source connection limit exceeded', got %v", err)
	}
}

// TestManager_ReleaseConnection tests connection release
func TestManager_ReleaseConnection(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	tracked, err := mgr.Accept(rawConn)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	if mgr.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", mgr.ActiveCount())
	}

	// Close should trigger release
	tracked.Close()
	rawConn.Close()

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	if mgr.ActiveCount() != 0 {
		t.Errorf("ActiveCount() after close = %d, want 0", mgr.ActiveCount())
	}
}

// TestManager_AssociateBackend_NotFound tests AssociateBackend with non-existent connection
func TestManager_AssociateBackend_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.AssociateBackend("nonexistent", "backend-1")
	if err == nil {
		t.Error("AssociateBackend() for non-existent connection should return error")
	}
	if err.Error() != "connection not found" {
		t.Errorf("Expected 'connection not found', got %v", err)
	}
}

// TestManager_AssociateBackend_PerBackendLimit tests per-backend connection limit
func TestManager_AssociateBackend_PerBackendLimit(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
		MaxPerBackend:  1,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for i := 0; i < 2; i++ {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}
	}()

	// First connection
	conn1, _ := net.Dial("tcp", listener.Addr().String())
	if conn1 == nil {
		t.Fatal("Failed to create first connection")
	}
	defer conn1.Close()

	tracked1, _ := mgr.Accept(conn1)
	err = mgr.AssociateBackend(tracked1.ID(), "backend-1")
	if err != nil {
		t.Errorf("First AssociateBackend() error = %v", err)
	}

	// Second connection
	conn2, _ := net.Dial("tcp", listener.Addr().String())
	if conn2 == nil {
		t.Fatal("Failed to create second connection")
	}
	defer conn2.Close()

	tracked2, _ := mgr.Accept(conn2)
	err = mgr.AssociateBackend(tracked2.ID(), "backend-1")
	if err == nil {
		t.Error("Second AssociateBackend() should fail due to per-backend limit")
	}
	if err.Error() != "per-backend connection limit exceeded" {
		t.Errorf("Expected 'per-backend connection limit exceeded', got %v", err)
	}
}

// TestManager_AssociateBackend_SwitchBackend tests switching backend
func TestManager_AssociateBackend_SwitchBackend(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
		MaxPerBackend:  10,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, _ := net.Dial("tcp", listener.Addr().String())
	if rawConn == nil {
		t.Fatal("Failed to dial")
	}
	defer rawConn.Close()

	tracked, _ := mgr.Accept(rawConn)

	// Associate with backend-1
	err = mgr.AssociateBackend(tracked.ID(), "backend-1")
	if err != nil {
		t.Errorf("AssociateBackend(backend-1) error = %v", err)
	}
	if mgr.BackendCount("backend-1") != 1 {
		t.Errorf("BackendCount(backend-1) = %d, want 1", mgr.BackendCount("backend-1"))
	}

	// Switch to backend-2
	err = mgr.AssociateBackend(tracked.ID(), "backend-2")
	if err != nil {
		t.Errorf("AssociateBackend(backend-2) error = %v", err)
	}
	if mgr.BackendCount("backend-1") != 0 {
		t.Errorf("BackendCount(backend-1) after switch = %d, want 0", mgr.BackendCount("backend-1"))
	}
	if mgr.BackendCount("backend-2") != 1 {
		t.Errorf("BackendCount(backend-2) = %d, want 1", mgr.BackendCount("backend-2"))
	}
}

// TestManager_SourceCount tests SourceCount method
func TestManager_SourceCount(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	// Initially should be 0
	if mgr.SourceCount("127.0.0.1") != 0 {
		t.Errorf("SourceCount() initial = %d, want 0", mgr.SourceCount("127.0.0.1"))
	}

	rawConn, _ := net.Dial("tcp", listener.Addr().String())
	if rawConn == nil {
		t.Fatal("Failed to dial")
	}
	defer rawConn.Close()

	tracked, _ := mgr.Accept(rawConn)

	// Should be 1 after accept
	if mgr.SourceCount("127.0.0.1") != 1 {
		t.Errorf("SourceCount() after accept = %d, want 1", mgr.SourceCount("127.0.0.1"))
	}

	tracked.Close()
	time.Sleep(50 * time.Millisecond)

	// Should be 0 after close
	if mgr.SourceCount("127.0.0.1") != 0 {
		t.Errorf("SourceCount() after close = %d, want 0", mgr.SourceCount("127.0.0.1"))
	}
}

// TestManager_TotalCount tests TotalCount method
func TestManager_TotalCount(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverConns := make(chan net.Conn, 3)
	go func() {
		for i := 0; i < 3; i++ {
			conn, _ := listener.Accept()
			if conn != nil {
				serverConns <- conn
			}
		}
	}()

	// TotalCount tracks active connections (incremented on Accept, decremented on release)
	var trackedConns []*TrackedConn
	for i := 0; i < 3; i++ {
		conn, _ := net.Dial("tcp", listener.Addr().String())
		if conn == nil {
			t.Fatal("Failed to dial")
		}

		tracked, _ := mgr.Accept(conn)
		trackedConns = append(trackedConns, tracked)

		// TotalCount should reflect active connections
		if mgr.TotalCount() != int64(i+1) {
			t.Errorf("TotalCount() after accept %d = %d, want %d", i+1, mgr.TotalCount(), i+1)
		}
	}

	// Close all connections
	for _, tracked := range trackedConns {
		tracked.Close()
	}
	for i := 0; i < 3; i++ {
		conn := <-serverConns
		conn.Close()
	}

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// After all closed, TotalCount should be 0
	if mgr.TotalCount() != 0 {
		t.Errorf("TotalCount() after close = %d, want 0", mgr.TotalCount())
	}
}

// TestManager_ActiveConnections tests ActiveConnections method
func TestManager_ActiveConnections(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for i := 0; i < 2; i++ {
			conn, _ := listener.Accept()
			if conn != nil {
				defer conn.Close()
			}
		}
	}()

	// Initially empty
	conns := mgr.ActiveConnections()
	if len(conns) != 0 {
		t.Errorf("ActiveConnections() initial length = %d, want 0", len(conns))
	}

	// Add two connections
	conn1, _ := net.Dial("tcp", listener.Addr().String())
	if conn1 == nil {
		t.Fatal("Failed to dial")
	}
	defer conn1.Close()

	tracked1, _ := mgr.Accept(conn1)
	mgr.AssociateBackend(tracked1.ID(), "backend-1")

	conn2, _ := net.Dial("tcp", listener.Addr().String())
	if conn2 == nil {
		t.Fatal("Failed to dial")
	}
	defer conn2.Close()

	tracked2, _ := mgr.Accept(conn2)
	mgr.AssociateBackend(tracked2.ID(), "backend-2")

	conns = mgr.ActiveConnections()
	if len(conns) != 2 {
		t.Errorf("ActiveConnections() length = %d, want 2", len(conns))
	}

	// Verify stats content
	backendIDs := make(map[string]bool)
	for _, stats := range conns {
		backendIDs[stats.BackendID] = true
	}
	if !backendIDs["backend-1"] || !backendIDs["backend-2"] {
		t.Error("ActiveConnections() missing expected backend IDs")
	}
}

// TestManager_Drain_WithActiveConnections tests drain with active connections
func TestManager_Drain_WithActiveConnections(t *testing.T) {
	config := &Config{
		DrainTimeout: 100 * time.Millisecond,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, _ := net.Dial("tcp", listener.Addr().String())
	if rawConn == nil {
		t.Fatal("Failed to dial")
	}

	tracked, _ := mgr.Accept(rawConn)

	// Drain should timeout since connection is active
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = mgr.Drain(ctx)
	if err == nil {
		t.Error("Drain() should timeout with active connections")
	}

	tracked.Close()
	rawConn.Close()
}

// TestManager_Drain_ContextCancelled tests drain with cancelled context
func TestManager_Drain_ContextCancelled(t *testing.T) {
	config := &Config{
		DrainTimeout: 30 * time.Second,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, _ := net.Dial("tcp", listener.Addr().String())
	if rawConn == nil {
		t.Fatal("Failed to dial")
	}
	defer rawConn.Close()

	mgr.Accept(rawConn)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = mgr.Drain(ctx)
	if err != context.Canceled {
		t.Errorf("Drain() error = %v, want context.Canceled", err)
	}
}

// TestManager_ConcurrentAcceptRelease tests concurrent Accept and Release
func TestManager_ConcurrentAcceptRelease(t *testing.T) {
	config := &Config{
		MaxConnections: 100,
		MaxPerSource:   100,
	}
	mgr := NewManager(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections
	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					time.Sleep(100 * time.Millisecond)
				}(conn)
			}
		}
	}()

	// Concurrent accepts
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, _ := net.Dial("tcp", listener.Addr().String())
			if conn != nil {
				tracked, _ := mgr.Accept(conn)
				if tracked != nil {
					time.Sleep(10 * time.Millisecond)
					tracked.Close()
				}
				conn.Close()
			}
		}()
	}
	wg.Wait()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	if mgr.ActiveCount() != 0 {
		t.Errorf("ActiveCount() after concurrent ops = %d, want 0", mgr.ActiveCount())
	}
}

// TestManager_GetConnection_Existing tests GetConnection with existing connection
func TestManager_GetConnection_Existing(t *testing.T) {
	mgr := NewManager(nil)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
		}
	}()

	rawConn, _ := net.Dial("tcp", listener.Addr().String())
	if rawConn == nil {
		t.Fatal("Failed to dial")
	}
	defer rawConn.Close()

	tracked, _ := mgr.Accept(rawConn)

	// Get existing connection
	retrieved := mgr.GetConnection(tracked.ID())
	if retrieved == nil {
		t.Error("GetConnection() should return existing connection")
	}
	if retrieved.ID() != tracked.ID() {
		t.Errorf("GetConnection() returned wrong connection: %v", retrieved.ID())
	}
}

// mockAddr is a mock net.Addr for testing
type mockAddr struct {
	network string
	str     string
}

func (m mockAddr) Network() string { return m.network }
func (m mockAddr) String() string  { return m.str }

// mockConn is a mock net.Conn for testing error paths
type mockConn struct {
	net.Conn
	remoteAddr net.Addr
	localAddr  net.Addr
	closed     bool
}

func (m *mockConn) RemoteAddr() net.Addr { return m.remoteAddr }
func (m *mockConn) LocalAddr() net.Addr  { return m.localAddr }
func (m *mockConn) Close() error {
	m.closed = true
	return nil
}
func (m *mockConn) Read(p []byte) (n int, err error)  { return 0, nil }
func (m *mockConn) Write(p []byte) (n int, err error) { return len(p), nil }

// TestManager_Accept_InvalidRemoteAddr tests Accept with invalid remote address
func TestManager_Accept_InvalidRemoteAddr(t *testing.T) {
	config := &Config{
		MaxConnections: 10,
		MaxPerSource:   10,
	}
	mgr := NewManager(config)

	// Create a mock connection with an address that doesn't have a port
	mock := &mockConn{
		remoteAddr: mockAddr{network: "tcp", str: "invalid-address-no-port"},
		localAddr:  mockAddr{network: "tcp", str: "127.0.0.1:8080"},
	}

	// This should work - Accept should handle the error from SplitHostPort
	// by using the full address as the host
	tracked, err := mgr.Accept(mock)
	if err != nil {
		t.Errorf("Accept() with invalid address error = %v", err)
	}
	if tracked == nil {
		t.Fatal("Accept() returned nil")
	}

	// The connection should be tracked with the full address as the source
	if mgr.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", mgr.ActiveCount())
	}

	tracked.Close()
}
