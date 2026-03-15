package conn

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     "127.0.0.1:8080",
		MaxSize:     10,
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 30 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)
	if pool == nil {
		t.Fatal("NewPool() returned nil")
	}

	stats := pool.Stats()
	if stats.BackendID != "backend-1" {
		t.Errorf("Stats.BackendID = %v, want %v", stats.BackendID, "backend-1")
	}
	if stats.Address != "127.0.0.1:8080" {
		t.Errorf("Stats.Address = %v, want %v", stats.Address, "127.0.0.1:8080")
	}
	if stats.MaxSize != 10 {
		t.Errorf("Stats.MaxSize = %v, want %v", stats.MaxSize, 10)
	}
}

func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()
	if config.MaxSize != 10 {
		t.Errorf("MaxSize = %v, want %v", config.MaxSize, 10)
	}
	if config.MaxLifetime != 1*time.Hour {
		t.Errorf("MaxLifetime = %v, want %v", config.MaxLifetime, 1*time.Hour)
	}
	if config.IdleTimeout != 30*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", config.IdleTimeout, 30*time.Minute)
	}
	if config.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout = %v, want %v", config.DialTimeout, 5*time.Second)
	}
}

func TestPool_GetAndPut(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n > 0 {
							c.Write(buf[:n])
						}
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     2,
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 30 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)

	ctx := context.Background()

	// Get a connection
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if conn1 == nil {
		t.Fatal("Get() returned nil")
	}

	stats := pool.Stats()
	if stats.Active != 1 {
		t.Errorf("Active after Get = %v, want 1", stats.Active)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses after Get = %v, want 1", stats.Misses)
	}

	// Put it back
	pool.Put(conn1)

	stats = pool.Stats()
	if stats.Idle != 1 {
		t.Errorf("Idle after Put = %v, want 1", stats.Idle)
	}

	// Get again - should reuse
	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	stats = pool.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits after reuse = %v, want 1", stats.Hits)
	}

	// Put back and close pool
	pool.Put(conn2)
	pool.Close()
}

func TestPool_Get_Closed(t *testing.T) {
	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     "127.0.0.1:8080",
		DialTimeout: 100 * time.Millisecond,
	}

	pool := NewPool(config)
	pool.Close()

	ctx := context.Background()
	_, err := pool.Get(ctx)
	if err == nil {
		t.Error("Get() on closed pool should return error")
	}
}

func TestPool_Close(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     2,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)

	ctx := context.Background()
	conn, _ := pool.Get(ctx)
	pool.Put(conn)

	// Close should succeed
	err = pool.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should succeed
	err = pool.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestPooledConn_isExpired(t *testing.T) {
	// Test max lifetime expiration
	conn1 := &PooledConn{
		createdAt: time.Now().Add(-2 * time.Hour),
		lastUsed:  time.Now(),
	}
	if !conn1.isExpired(1*time.Hour, 0) {
		t.Error("Connection should be expired by max lifetime")
	}

	// Test idle timeout expiration
	conn2 := &PooledConn{
		createdAt: time.Now(),
		lastUsed:  time.Now().Add(-2 * time.Hour),
	}
	if !conn2.isExpired(0, 1*time.Hour) {
		t.Error("Connection should be expired by idle timeout")
	}

	// Test not expired
	conn3 := &PooledConn{
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	if conn3.isExpired(1*time.Hour, 1*time.Hour) {
		t.Error("Connection should not be expired")
	}

	// Test no limits
	conn4 := &PooledConn{
		createdAt: time.Now().Add(-10 * time.Hour),
		lastUsed:  time.Now().Add(-10 * time.Hour),
	}
	if conn4.isExpired(0, 0) {
		t.Error("Connection should not expire with no limits")
	}
}

func TestPoolManager_GetPool(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)
	defer pm.Close()

	// Get pool for backend
	pool1 := pm.GetPool("backend-1", "127.0.0.1:8080")
	if pool1 == nil {
		t.Fatal("GetPool() returned nil")
	}

	// Get same pool again
	pool2 := pm.GetPool("backend-1", "127.0.0.1:8080")
	if pool2 != pool1 {
		t.Error("GetPool() should return same pool for same backend")
	}

	// Get different pool
	pool3 := pm.GetPool("backend-2", "127.0.0.1:8081")
	if pool3 == pool1 {
		t.Error("GetPool() should return different pool for different backend")
	}
}

func TestPoolManager_RemovePool(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)

	pm.GetPool("backend-1", "127.0.0.1:8080")

	// Remove existing pool
	err := pm.RemovePool("backend-1")
	if err != nil {
		t.Errorf("RemovePool() error = %v", err)
	}

	// Remove non-existent pool
	err = pm.RemovePool("backend-1")
	if err == nil {
		t.Error("RemovePool() for non-existent should return error")
	}
}

func TestPoolManager_Stats(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)
	defer pm.Close()

	// Initially empty
	stats := pm.Stats()
	if len(stats) != 0 {
		t.Errorf("Stats() initial length = %v, want 0", len(stats))
	}

	// Add pools
	pm.GetPool("backend-1", "127.0.0.1:8080")
	pm.GetPool("backend-2", "127.0.0.1:8081")

	stats = pm.Stats()
	if len(stats) != 2 {
		t.Errorf("Stats() after pools length = %v, want 2", len(stats))
	}

	if _, ok := stats["backend-1"]; !ok {
		t.Error("Stats() missing backend-1")
	}
	if _, ok := stats["backend-2"]; !ok {
		t.Error("Stats() missing backend-2")
	}
}

func TestPool_Put_NonPooled(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     2,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)

	// Dial directly (not through pool)
	rawConn, _ := net.Dial("tcp", listener.Addr().String())
	if rawConn == nil {
		t.Fatal("Failed to dial")
	}

	// Put non-pooled connection should just close it
	pool.Put(rawConn)
}

// TestPool_GetFromClosedPool tests Get from closed pool
func TestPool_GetFromClosedPool(t *testing.T) {
	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     "127.0.0.1:8080",
		DialTimeout: 100 * time.Millisecond,
	}

	pool := NewPool(config)
	pool.Close()

	ctx := context.Background()
	conn, err := pool.Get(ctx)
	if err == nil {
		t.Error("Get() from closed pool should return error")
	}
	if conn != nil {
		t.Error("Get() from closed pool should return nil connection")
	}
	if err.Error() != "pool is closed" {
		t.Errorf("Expected 'pool is closed', got %v", err)
	}
}

// TestPool_PutExpiredConnection tests Put with expired connection
func TestPool_PutExpiredConnection(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     2,
		MaxLifetime: 1 * time.Millisecond, // Very short lifetime
		IdleTimeout: 1 * time.Millisecond,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)

	ctx := context.Background()
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Wait for connection to expire
	time.Sleep(10 * time.Millisecond)

	// Put expired connection - should be closed
	pool.Put(conn)

	stats := pool.Stats()
	if stats.Idle != 0 {
		t.Errorf("Idle after Put expired = %d, want 0", stats.Idle)
	}
}

// TestPool_PutToFullPool tests Put when pool is full
func TestPool_PutToFullPool(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     1, // Only allow 1 idle connection
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 30 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)

	ctx := context.Background()

	// Get and put first connection
	conn1, _ := pool.Get(ctx)
	pool.Put(conn1)

	// Get second connection
	conn2, _ := pool.Get(ctx)

	// Put second connection - should succeed since we took one out
	pool.Put(conn2)

	stats := pool.Stats()
	if stats.Idle != 1 {
		t.Errorf("Idle = %d, want 1", stats.Idle)
	}
}

// TestPool_ConcurrentGetPut tests concurrent Get and Put operations
func TestPool_ConcurrentGetPut(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     10,
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 30 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)
	defer pool.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := pool.Get(ctx)
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
			pool.Put(conn)
		}()
	}
	wg.Wait()

	stats := pool.Stats()
	if stats.Idle+stats.Active > config.MaxSize {
		t.Errorf("Total connections %d exceeds max size %d", stats.Idle+stats.Active, config.MaxSize)
	}
}

// TestPool_StatsAccuracy tests that stats are accurate
func TestPool_StatsAccuracy(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     5,
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 30 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Initial stats
	stats := pool.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Active != 0 || stats.Idle != 0 {
		t.Error("Initial stats should be zero")
	}

	// Get first connection (miss)
	conn1, _ := pool.Get(ctx)
	stats = pool.Stats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Active != 1 {
		t.Errorf("Active = %d, want 1", stats.Active)
	}

	// Put back
	pool.Put(conn1)
	stats = pool.Stats()
	if stats.Idle != 1 {
		t.Errorf("Idle = %d, want 1", stats.Idle)
	}
	if stats.Active != 0 {
		t.Errorf("Active = %d, want 0", stats.Active)
	}

	// Get again (hit)
	conn2, _ := pool.Get(ctx)
	stats = pool.Stats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}

	pool.Put(conn2)
}

// TestPooledConn_Close tests PooledConn Close method
func TestPooledConn_Close(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     5,
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 30 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Get connection and close it
	conn, _ := pool.Get(ctx)
	if conn == nil {
		t.Fatal("Get() returned nil")
	}

	// Close should return to pool
	err = conn.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	stats := pool.Stats()
	if stats.Idle != 1 {
		t.Errorf("Idle after Close = %d, want 1", stats.Idle)
	}
}

// TestPooledConn_CloseNoPool tests PooledConn Close when pool is nil
func TestPooledConn_CloseNoPool(t *testing.T) {
	// Start a test server
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

	// Create PooledConn with nil pool
	pc := &PooledConn{
		Conn:      rawConn,
		pool:      nil,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}

	// Close should close underlying connection
	err = pc.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestPool_GetExpiredFromIdle tests Get with expired idle connections
func TestPool_GetExpiredFromIdle(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, _ := listener.Accept()
			if conn != nil {
				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 1024)
					for {
						n, _ := c.Read(buf)
						if n <= 0 {
							return
						}
					}
				}(conn)
			}
		}
	}()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     listener.Addr().String(),
		MaxSize:     5,
		MaxLifetime: 1 * time.Hour,
		IdleTimeout: 1 * time.Millisecond, // Very short idle timeout
		DialTimeout: 5 * time.Second,
	}

	pool := NewPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Get and put connection
	conn1, _ := pool.Get(ctx)
	pool.Put(conn1)

	// Wait for connection to expire
	time.Sleep(10 * time.Millisecond)

	// Get again - should detect expired and create new
	conn2, _ := pool.Get(ctx)
	if conn2 == nil {
		t.Fatal("Get() returned nil")
	}

	stats := pool.Stats()
	// Should have 2 misses (one for first get, one for expired get)
	if stats.Misses != 2 {
		t.Errorf("Misses = %d, want 2", stats.Misses)
	}

	pool.Put(conn2)
}

// TestPoolManager_GetPoolConcurrent tests concurrent GetPool access
func TestPoolManager_GetPoolConcurrent(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)
	defer pm.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			backendID := fmt.Sprintf("backend-%d", id%5) // 5 unique backends
			pool := pm.GetPool(backendID, "127.0.0.1:8080")
			if pool == nil {
				t.Error("GetPool() returned nil")
			}
		}(i)
	}
	wg.Wait()

	stats := pm.Stats()
	if len(stats) != 5 {
		t.Errorf("Expected 5 pools, got %d", len(stats))
	}
}

// TestPoolManager_CloseAllPools tests closing all pools
func TestPoolManager_CloseAllPools(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)

	// Create some pools
	pm.GetPool("backend-1", "127.0.0.1:8081")
	pm.GetPool("backend-2", "127.0.0.1:8082")
	pm.GetPool("backend-3", "127.0.0.1:8083")

	stats := pm.Stats()
	if len(stats) != 3 {
		t.Errorf("Expected 3 pools, got %d", len(stats))
	}

	// Close all
	pm.Close()

	// Stats should be empty after close
	stats = pm.Stats()
	if len(stats) != 0 {
		t.Errorf("Expected 0 pools after close, got %d", len(stats))
	}
}

// TestPoolManager_StatsMultiplePools tests Stats with multiple pools
func TestPoolManager_StatsMultiplePools(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)
	defer pm.Close()

	// Create pools
	pool1 := pm.GetPool("backend-1", "127.0.0.1:8081")
	pool2 := pm.GetPool("backend-2", "127.0.0.1:8082")

	// Verify pool addresses
	stats := pm.Stats()
	if stats["backend-1"].Address != "127.0.0.1:8081" {
		t.Errorf("backend-1 address = %v, want 127.0.0.1:8081", stats["backend-1"].Address)
	}
	if stats["backend-2"].Address != "127.0.0.1:8082" {
		t.Errorf("backend-2 address = %v, want 127.0.0.1:8082", stats["backend-2"].Address)
	}

	// Verify pool objects are correct
	if pool1 != pm.GetPool("backend-1", "127.0.0.1:8081") {
		t.Error("GetPool should return same pool for backend-1")
	}
	if pool2 != pm.GetPool("backend-2", "127.0.0.1:8082") {
		t.Error("GetPool should return same pool for backend-2")
	}
}

// TestNewPool_NilConfig tests NewPool with nil config
func TestNewPool_NilConfig(t *testing.T) {
	pool := NewPool(nil)
	if pool == nil {
		t.Fatal("NewPool(nil) returned nil")
	}

	stats := pool.Stats()
	if stats.MaxSize != 10 {
		t.Errorf("MaxSize = %d, want 10", stats.MaxSize)
	}
}

// TestPoolManager_GetPoolNilConfig tests GetPool with nil config in manager
func TestPoolManager_GetPoolNilConfig(t *testing.T) {
	pm := NewPoolManager(nil)
	defer pm.Close()

	pool := pm.GetPool("backend-1", "127.0.0.1:8080")
	if pool == nil {
		t.Fatal("GetPool() returned nil")
	}

	stats := pool.Stats()
	if stats.MaxSize != 10 {
		t.Errorf("MaxSize = %d, want 10", stats.MaxSize)
	}
}

// TestPool_Get_DialError tests Get when dial fails
func TestPool_Get_DialError(t *testing.T) {
	// Find a port that is not listening by binding and immediately closing
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	unusedAddr := l.Addr().String()
	l.Close()

	config := &PoolConfig{
		BackendID:   "backend-1",
		Address:     unusedAddr, // Port is now closed, dial will fail
		DialTimeout: 100 * time.Millisecond,
	}

	pool := NewPool(config)
	defer pool.Close()

	ctx := context.Background()
	conn, err := pool.Get(ctx)
	if err == nil {
		t.Error("Get() should return error when dial fails")
	}
	if conn != nil {
		t.Error("Get() should return nil connection when dial fails")
	}

	// Stats should still track the miss
	stats := pool.Stats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
}

// TestPoolManager_GetPool_DoubleCheck tests the double-check path in GetPool
func TestPoolManager_GetPool_DoubleCheck(t *testing.T) {
	config := DefaultPoolConfig()
	pm := NewPoolManager(config)
	defer pm.Close()

	var wg sync.WaitGroup
	var pools []*Pool
	var mu sync.Mutex

	// Launch multiple goroutines to create the same pool
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool := pm.GetPool("backend-1", "127.0.0.1:8080")
			mu.Lock()
			pools = append(pools, pool)
			mu.Unlock()
		}()
	}
	wg.Wait()

	// All should return the same pool
	if len(pools) != 10 {
		t.Fatalf("Expected 10 pools, got %d", len(pools))
	}
	for i := 1; i < len(pools); i++ {
		if pools[i] != pools[0] {
			t.Error("GetPool should return the same pool instance")
		}
	}
}
