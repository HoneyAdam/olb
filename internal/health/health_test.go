package health

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
)

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusHealthy, "healthy"},
		{StatusUnhealthy, "unhealthy"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("Status.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultCheck(t *testing.T) {
	check := DefaultCheck()

	if check.Type != "tcp" {
		t.Errorf("DefaultCheck().Type = %v, want %v", check.Type, "tcp")
	}
	if check.Interval != 10*time.Second {
		t.Errorf("DefaultCheck().Interval = %v, want %v", check.Interval, 10*time.Second)
	}
	if check.Timeout != 5*time.Second {
		t.Errorf("DefaultCheck().Timeout = %v, want %v", check.Timeout, 5*time.Second)
	}
	if check.Path != "/health" {
		t.Errorf("DefaultCheck().Path = %v, want %v", check.Path, "/health")
	}
	if check.Method != "GET" {
		t.Errorf("DefaultCheck().Method = %v, want %v", check.Method, "GET")
	}
	if check.ExpectedStatus != 200 {
		t.Errorf("DefaultCheck().ExpectedStatus = %v, want %v", check.ExpectedStatus, 200)
	}
	if check.HealthyThreshold != 2 {
		t.Errorf("DefaultCheck().HealthyThreshold = %v, want %v", check.HealthyThreshold, 2)
	}
	if check.UnhealthyThreshold != 3 {
		t.Errorf("DefaultCheck().UnhealthyThreshold = %v, want %v", check.UnhealthyThreshold, 3)
	}
}

func TestChecker_Register(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	b := backend.NewBackend("test", "127.0.0.1:18080")
	check := &Check{
		Type:     "tcp",
		Interval: 100 * time.Millisecond,
		Timeout:  50 * time.Millisecond,
	}

	err := checker.Register(b, check)
	if err != nil {
		t.Errorf("Register() error = %v", err)
	}

	// Registering duplicate should fail
	err = checker.Register(b, check)
	if err == nil {
		t.Error("Register() duplicate should return error")
	}

	// Registering nil backend should fail
	err = checker.Register(nil, check)
	if err == nil {
		t.Error("Register() nil backend should return error")
	}
}

func TestChecker_Unregister(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	b := backend.NewBackend("test", "127.0.0.1:18081")
	check := &Check{
		Type:     "tcp",
		Interval: 100 * time.Millisecond,
		Timeout:  50 * time.Millisecond,
	}

	checker.Register(b, check)
	checker.Unregister("test")

	// Should be able to register again after unregister
	err := checker.Register(b, check)
	if err != nil {
		t.Errorf("Register() after unregister error = %v", err)
	}
}

func TestChecker_GetStatus(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	b := backend.NewBackend("test", "127.0.0.1:18082")
	check := &Check{
		Type:     "tcp",
		Interval: 100 * time.Millisecond,
		Timeout:  50 * time.Millisecond,
	}

	// Status should be unknown before registration
	if status := checker.GetStatus("test"); status != StatusUnknown {
		t.Errorf("GetStatus() before register = %v, want %v", status, StatusUnknown)
	}

	checker.Register(b, check)

	// Status should be unknown initially (no check performed yet in this test)
	// In real scenario, the first check happens immediately
}

func TestChecker_ListStatuses(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	b1 := backend.NewBackend("test1", "127.0.0.1:18083")
	b2 := backend.NewBackend("test2", "127.0.0.1:18084")

	checker.Register(b1, DefaultCheck())
	checker.Register(b2, DefaultCheck())

	statuses := checker.ListStatuses()
	if len(statuses) != 2 {
		t.Errorf("ListStatuses() length = %v, want %v", len(statuses), 2)
	}
}

func TestChecker_checkTCP(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Test successful TCP check
	b := backend.NewBackend("test", listener.Addr().String())
	check := &Check{
		Timeout: 1 * time.Second,
	}

	result := checker.checkTCP(b, check)
	if !result.Healthy {
		t.Errorf("checkTCP() on available port = %v, want healthy", result.Healthy)
	}
	if result.Error != nil {
		t.Errorf("checkTCP() error = %v", result.Error)
	}

	// Test failed TCP check - use a port that's not listening
	failListener, _ := net.Listen("tcp", "127.0.0.1:0")
	failAddr := failListener.Addr().String()
	failListener.Close()
	b2 := backend.NewBackend("test2", failAddr)
	result2 := checker.checkTCP(b2, check)
	if result2.Healthy {
		t.Error("checkTCP() on unavailable port should be unhealthy")
	}
	if result2.Error == nil {
		t.Error("checkTCP() on unavailable port should return error")
	}
}

func TestChecker_checkHTTP(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	// Start a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
	})

	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	go server.Serve(listener)
	defer server.Close()

	addr := listener.Addr().String()
	time.Sleep(10 * time.Millisecond) // Wait for server to start

	// Test successful HTTP check
	b := backend.NewBackend("test", addr)
	check := &Check{
		Type:           "http",
		Path:           "/health",
		Method:         "GET",
		ExpectedStatus: 200,
		Timeout:        1 * time.Second,
	}

	result := checker.checkHTTP(b, check)
	if !result.Healthy {
		t.Errorf("checkHTTP() on healthy endpoint = %v, want healthy", result.Healthy)
	}
	if result.Error != nil {
		t.Errorf("checkHTTP() error = %v", result.Error)
	}

	// Test failed HTTP check (wrong status)
	check2 := &Check{
		Type:           "http",
		Path:           "/error",
		Method:         "GET",
		ExpectedStatus: 200,
		Timeout:        1 * time.Second,
	}

	result2 := checker.checkHTTP(b, check2)
	if result2.Healthy {
		t.Error("checkHTTP() on error endpoint should be unhealthy")
	}

	// Test with any 2xx status
	check3 := &Check{
		Type:           "http",
		Path:           "/health",
		Method:         "GET",
		ExpectedStatus: 0, // Any 2xx
		Timeout:        1 * time.Second,
	}

	result3 := checker.checkHTTP(b, check3)
	if !result3.Healthy {
		t.Error("checkHTTP() with ExpectedStatus=0 should accept 200")
	}
}

func TestChecker_checkHTTP_WithHeaders(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	var receivedHeader string

	// Start a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	go server.Serve(listener)
	defer server.Close()

	addr := listener.Addr().String()
	time.Sleep(10 * time.Millisecond)

	b := backend.NewBackend("test", addr)
	check := &Check{
		Type:           "http",
		Path:           "/health",
		Method:         "GET",
		ExpectedStatus: 200,
		Timeout:        1 * time.Second,
		Headers: map[string]string{
			"X-Custom-Header": "test-value",
		},
	}

	checker.checkHTTP(b, check)

	if receivedHeader != "test-value" {
		t.Errorf("Header not received correctly: got %v, want %v", receivedHeader, "test-value")
	}
}

func TestChecker_StateTransitions(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	b := backend.NewBackend("test", listener.Addr().String())
	b.SetState(backend.StateStarting)

	check := &Check{
		Type:               "tcp",
		Interval:           50 * time.Millisecond,
		Timeout:            100 * time.Millisecond,
		HealthyThreshold:   2,
		UnhealthyThreshold: 2,
	}

	checker.Register(b, check)

	// Wait for health checks to run and transition to healthy
	time.Sleep(200 * time.Millisecond)

	status := checker.GetStatus("test")
	if status != StatusHealthy {
		t.Errorf("Status after successful checks = %v, want %v", status, StatusHealthy)
	}

	if b.State() != backend.StateUp {
		t.Errorf("Backend state = %v, want %v", b.State(), backend.StateUp)
	}
}

func TestChecker_CountHealthyUnhealthy(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	// Register some backends
	checker.Register(backend.NewBackend("b1", "127.0.0.1:18085"), DefaultCheck())
	checker.Register(backend.NewBackend("b2", "127.0.0.1:18086"), DefaultCheck())
	checker.Register(backend.NewBackend("b3", "127.0.0.1:18087"), DefaultCheck())

	// Initially all unknown
	if healthy := checker.CountHealthy(); healthy != 0 {
		t.Errorf("CountHealthy() initial = %v, want 0", healthy)
	}
	if unhealthy := checker.CountUnhealthy(); unhealthy != 0 {
		t.Errorf("CountUnhealthy() initial = %v, want 0", unhealthy)
	}
}

func TestChecker_GetResult(t *testing.T) {
	checker := NewChecker()
	defer checker.Stop()

	// Non-existent backend
	if result := checker.GetResult("nonexistent"); result != nil {
		t.Error("GetResult() for non-existent backend should return nil")
	}

	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	b := backend.NewBackend("test", listener.Addr().String())
	check := &Check{
		Type:     "tcp",
		Interval: 50 * time.Millisecond,
		Timeout:  100 * time.Millisecond,
	}

	checker.Register(b, check)

	// Wait for a check to complete
	time.Sleep(100 * time.Millisecond)

	result := checker.GetResult("test")
	if result == nil {
		t.Fatal("GetResult() should return a result after check")
	}

	if result.Timestamp.IsZero() {
		t.Error("Result.Timestamp should be set")
	}
}

func BenchmarkChecker_checkTCP(b *testing.B) {
	checker := NewChecker()
	defer checker.Stop()

	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	be := backend.NewBackend("test", listener.Addr().String())
	check := &Check{
		Timeout: 1 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.checkTCP(be, check)
	}
}

func BenchmarkChecker_checkHTTP(b *testing.B) {
	checker := NewChecker()
	defer checker.Stop()

	// Start a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}

	go server.Serve(listener)
	defer server.Close()

	addr := listener.Addr().String()
	time.Sleep(10 * time.Millisecond)

	be := backend.NewBackend("test", addr)
	check := &Check{
		Type:           "http",
		Path:           "/health",
		Method:         "GET",
		ExpectedStatus: 200,
		Timeout:        1 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.checkHTTP(be, check)
	}
}

// mockBalancer is a simple mock balancer for integration tests
type mockBalancer struct {
	name string
}

func (m *mockBalancer) Name() string {
	return m.name
}

func (m *mockBalancer) Next(backends []*backend.Backend) *backend.Backend {
	if len(backends) > 0 {
		return backends[0]
	}
	return nil
}

func (m *mockBalancer) Add(b *backend.Backend) {}

func (m *mockBalancer) Remove(id string) {}

func (m *mockBalancer) Update(b *backend.Backend) {}

func ExampleChecker_Register() {
	checker := NewChecker()
	defer checker.Stop()

	b := backend.NewBackend("web1", "10.0.0.1:8080")
	check := &Check{
		Type:     "http",
		Path:     "/health",
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
	}

	err := checker.Register(b, check)
	if err != nil {
		fmt.Printf("Failed to register: %v\n", err)
		return
	}

	fmt.Println("Backend registered for health checks")
	// Output: Backend registered for health checks
}
