package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitMiddleware_AllowUnderLimit(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         10,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make 5 requests (under burst limit of 10)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimitMiddleware_BlockOverLimit(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make 5 requests (at burst limit)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// 6th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	// Check Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header on rate limited response")
	}
}

func TestRateLimitMiddleware_BurstCapacity(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1.0, // Slow rate
		BurstSize:         10,  // But large burst
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should be able to make 10 requests immediately (burst)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Burst request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// 11th request should be blocked
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRateLimitMiddleware_TokenRefill(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0, // 100 tokens per second
		BurstSize:         2,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Next request should be blocked
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected to be rate limited, got status %d", rec.Code)
	}

	// Wait for tokens to refill (at 100 RPS, should take ~10ms per token)
	time.Sleep(20 * time.Millisecond)

	// Now request should succeed
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected request to succeed after refill, got status %d", rec.Code)
	}
}

func TestRateLimitMiddleware_PerKeyIsolation(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         3,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use different IPs
	ips := []string{"192.168.1.1:1234", "192.168.1.2:1234", "192.168.1.3:1234"}

	// Each IP should have its own burst limit
	for _, ip := range ips {
		for j := 0; j < 3; j++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = ip
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("IP %s request %d: expected status %d, got %d", ip, j+1, http.StatusOK, rec.Code)
			}
		}
	}

	// Each IP should now be rate limited
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("IP %s: expected to be rate limited, got status %d", ip, rec.Code)
		}
	}
}

func TestRateLimitMiddleware_Cleanup(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         10,
		CleanupInterval:   50 * time.Millisecond,
		CleanupTimeout:    100 * time.Millisecond,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make a request to create a bucket
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check bucket exists
	if _, ok := mw.buckets.Load("192.168.1.1"); !ok {
		t.Fatal("Bucket should exist after request")
	}

	// Wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Bucket should be cleaned up
	if _, ok := mw.buckets.Load("192.168.1.1"); ok {
		t.Error("Bucket should have been cleaned up")
	}
}

func TestRateLimitMiddleware_HeadersOnAllowed(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check X-RateLimit-Limit
	limit := rec.Header().Get("X-RateLimit-Limit")
	if limit != "10" {
		t.Errorf("Expected X-RateLimit-Limit to be '10', got '%s'", limit)
	}

	// Check X-RateLimit-Remaining
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	if remaining != "4" {
		t.Errorf("Expected X-RateLimit-Remaining to be '4', got '%s'", remaining)
	}

	// Check X-RateLimit-Reset exists and is in the future
	reset := rec.Header().Get("X-RateLimit-Reset")
	if reset == "" {
		t.Error("Expected X-RateLimit-Reset header to be set")
	} else {
		resetTime, err := strconv.ParseInt(reset, 10, 64)
		if err != nil {
			t.Errorf("X-RateLimit-Reset should be a valid timestamp: %v", err)
		} else if resetTime <= time.Now().Unix() {
			t.Error("X-RateLimit-Reset should be in the future")
		}
	}
}

func TestRateLimitMiddleware_HeadersOnBlocked(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request succeeds
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Second request is blocked
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	// Check X-RateLimit-Remaining is 0
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	if remaining != "0" {
		t.Errorf("Expected X-RateLimit-Remaining to be '0' on blocked request, got '%s'", remaining)
	}

	// Check Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header on rate limited response")
	} else {
		retrySeconds, err := strconv.ParseInt(retryAfter, 10, 64)
		if err != nil {
			t.Errorf("Retry-After should be a valid number: %v", err)
		} else if retrySeconds <= 0 {
			t.Error("Retry-After should be positive")
		}
	}
}

func TestRateLimitMiddleware_Name(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         5,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	if mw.Name() != "rate-limiter" {
		t.Errorf("Expected name 'rate-limiter', got '%s'", mw.Name())
	}
}

func TestRateLimitMiddleware_Priority(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         5,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	if mw.Priority() != PriorityRateLimit {
		t.Errorf("Expected priority %d, got %d", PriorityRateLimit, mw.Priority())
	}
}

func TestRateLimitMiddleware_CustomKeyFunc(t *testing.T) {
	// Limit by custom header instead of IP
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-API-Key")
		},
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Same IP, different API keys
	apiKeys := []string{"key1", "key2", "key3"}

	for _, key := range apiKeys {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-API-Key", key)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("API key %s request %d: expected status %d, got %d", key, i+1, http.StatusOK, rec.Code)
			}
		}
	}

	// Each API key should now be rate limited
	for _, key := range apiKeys {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("API key %s: expected to be rate limited, got status %d", key, rec.Code)
		}
	}
}

func TestRateLimitMiddleware_ConcurrentAccess(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 10000.0, // High rate to avoid most rejections
		BurstSize:         1000,
		CleanupInterval:   time.Minute,
		CleanupTimeout:    time.Minute,
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Run concurrent requests
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			done <- rec.Code == http.StatusOK
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < 100; i++ {
		if <-done {
			successCount++
		}
	}

	// Most requests should succeed (some may fail due to burst limit)
	if successCount < 90 {
		t.Errorf("Expected at least 90 successful requests, got %d", successCount)
	}
}

func TestRateLimitMiddleware_DefaultKeyFunc(t *testing.T) {
	// Test the default key function directly
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	key := defaultKeyFunc(req)
	if key != "192.168.1.100" {
		t.Errorf("Expected key '192.168.1.100', got '%s'", key)
	}

	// Test with invalid RemoteAddr (no port)
	req.RemoteAddr = "192.168.1.100"
	key = defaultKeyFunc(req)
	if key != "192.168.1.100" {
		t.Errorf("Expected key '192.168.1.100' for invalid addr, got '%s'", key)
	}
}

func TestRateLimitMiddleware_Defaults(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         5,
		// Leave other fields as zero values
	}

	mw, err := NewRateLimitMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer mw.Stop()

	// Check defaults were applied
	if mw.config.CleanupInterval != time.Minute {
		t.Errorf("Expected default CleanupInterval of 1m, got %v", mw.config.CleanupInterval)
	}
	if mw.config.CleanupTimeout != 10*time.Minute {
		t.Errorf("Expected default CleanupTimeout of 10m, got %v", mw.config.CleanupTimeout)
	}
	if mw.config.KeyFunc == nil {
		t.Error("Expected default KeyFunc to be set")
	}
}
