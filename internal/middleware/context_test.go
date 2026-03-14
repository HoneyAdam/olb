package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/router"
)

func TestNewRequestContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	ctx := NewRequestContext(req, rec)
	if ctx == nil {
		t.Fatal("NewRequestContext returned nil")
	}

	// Check initial state
	if ctx.Request != req {
		t.Error("Request not set correctly")
	}
	if ctx.Response == nil {
		t.Error("Response not set")
	}
	if ctx.Route != nil {
		t.Error("Route should be nil initially")
	}
	if ctx.Backend != nil {
		t.Error("Backend should be nil initially")
	}
	if ctx.RequestID != "" {
		t.Error("RequestID should be empty initially")
	}
	if ctx.ClientIP != "" {
		t.Error("ClientIP should be empty initially")
	}
	if ctx.BytesIn != 0 {
		t.Error("BytesIn should be 0 initially")
	}
	if ctx.BytesOut != 0 {
		t.Error("BytesOut should be 0 initially")
	}
	if ctx.StatusCode != 0 {
		t.Error("StatusCode should be 0 initially")
	}

	// Check that StartTime was set recently
	if time.Since(ctx.StartTime) > time.Second {
		t.Error("StartTime should be set to current time")
	}

	ctx.Response.(*responseWriter).Release()
	ctx.Release()
}

func TestRequestContextSetGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	// Set and get a value
	ctx.Set("key", "value")
	val, ok := ctx.Get("key")
	if !ok {
		t.Error("Get returned false for existing key")
	}
	if val != "value" {
		t.Errorf("expected 'value', got %v", val)
	}

	// Get non-existent key
	_, ok = ctx.Get("non-existent")
	if ok {
		t.Error("Get returned true for non-existent key")
	}
}

func TestRequestContextGetString(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.Set("str", "hello")
	ctx.Set("not-str", 123)

	if s := ctx.GetString("str"); s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
	if s := ctx.GetString("not-str"); s != "" {
		t.Errorf("expected empty string for non-string, got %q", s)
	}
	if s := ctx.GetString("missing"); s != "" {
		t.Errorf("expected empty string for missing key, got %q", s)
	}
}

func TestRequestContextGetInt(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.Set("int", 42)
	ctx.Set("int64", int64(64))
	ctx.Set("int32", int32(32))
	ctx.Set("not-int", "string")

	if i := ctx.GetInt("int"); i != 42 {
		t.Errorf("expected 42, got %d", i)
	}
	if i := ctx.GetInt("int64"); i != 64 {
		t.Errorf("expected 64, got %d", i)
	}
	if i := ctx.GetInt("int32"); i != 32 {
		t.Errorf("expected 32, got %d", i)
	}
	if i := ctx.GetInt("not-int"); i != 0 {
		t.Errorf("expected 0 for non-int, got %d", i)
	}
	if i := ctx.GetInt("missing"); i != 0 {
		t.Errorf("expected 0 for missing key, got %d", i)
	}
}

func TestRequestContextGetBool(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.Set("bool", true)
	ctx.Set("not-bool", "string")

	if b := ctx.GetBool("bool"); !b {
		t.Error("expected true, got false")
	}
	if b := ctx.GetBool("not-bool"); b {
		t.Error("expected false for non-bool, got true")
	}
	if b := ctx.GetBool("missing"); b {
		t.Error("expected false for missing key, got true")
	}
}

func TestRequestContextDelete(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.Set("key", "value")
	ctx.Delete("key")

	_, ok := ctx.Get("key")
	if ok {
		t.Error("key should be deleted")
	}
}

func TestRequestContextHas(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.Set("key", "value")

	if !ctx.Has("key") {
		t.Error("Has should return true for existing key")
	}
	if ctx.Has("missing") {
		t.Error("Has should return false for missing key")
	}
}

func TestRequestContextDuration(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	// Sleep briefly to ensure some duration
	time.Sleep(10 * time.Millisecond)

	duration := ctx.Duration()
	if duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", duration)
	}
}

func TestRequestContextUpdateStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.UpdateStatus(http.StatusNotFound)
	if ctx.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, ctx.StatusCode)
	}
}

func TestRequestContextUpdateBytesOut(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.UpdateBytesOut(100)
	ctx.UpdateBytesOut(50)
	if ctx.BytesOut != 150 {
		t.Errorf("expected BytesOut 150, got %d", ctx.BytesOut)
	}
}

func TestRequestContextUpdateBytesIn(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.UpdateBytesIn(200)
	ctx.UpdateBytesIn(25)
	if ctx.BytesIn != 225 {
		t.Errorf("expected BytesIn 225, got %d", ctx.BytesIn)
	}
}

func TestRequestContextAllMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	ctx.Set("key1", "value1")
	ctx.Set("key2", 42)

	all := ctx.AllMetadata()
	if len(all) != 2 {
		t.Errorf("expected 2 items, got %d", len(all))
	}
	if all["key1"] != "value1" {
		t.Error("key1 not correct")
	}
	if all["key2"] != 42 {
		t.Error("key2 not correct")
	}

	// Modifying returned map should not affect context
	all["key3"] = "value3"
	if ctx.Has("key3") {
		t.Error("modifying returned map should not affect context")
	}
}

func TestRequestContextRelease(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)

	// Set some state
	ctx.Set("key", "value")
	ctx.RequestID = "test-id"
	ctx.ClientIP = "1.2.3.4"
	ctx.StatusCode = 200
	ctx.BytesIn = 100
	ctx.BytesOut = 200

	// Release
	ctx.Release()

	// After release, fields should be cleared
	if ctx.Request != nil {
		t.Error("Request should be nil after Release")
	}
	if ctx.Response != nil {
		t.Error("Response should be nil after Release")
	}
	if ctx.RequestID != "" {
		t.Error("RequestID should be empty after Release")
	}
	if ctx.ClientIP != "" {
		t.Error("ClientIP should be empty after Release")
	}
	if ctx.StatusCode != 0 {
		t.Error("StatusCode should be 0 after Release")
	}
	if ctx.BytesIn != 0 {
		t.Error("BytesIn should be 0 after Release")
	}
	if ctx.BytesOut != 0 {
		t.Error("BytesOut should be 0 after Release")
	}
	if ctx.Has("key") {
		t.Error("metadata should be cleared after Release")
	}
}

func TestRequestContextConcurrency(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx.Set("key", n)
			ctx.Get("key")
		}(i)
	}
	wg.Wait()
}

func TestRequestContextWithRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	route := &router.Route{
		Name:        "test-route",
		Host:        "example.com",
		Path:        "/test",
		BackendPool: "test-pool",
	}

	ctx.Route = route
	if ctx.Route.Name != "test-route" {
		t.Error("Route not set correctly")
	}
}

func TestRequestContextWithBackend(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := NewRequestContext(req, rec)
	defer ctx.Release()

	b := backend.NewBackend("test-backend", "localhost:8080")

	ctx.Backend = b
	if ctx.Backend.ID != "test-backend" {
		t.Error("Backend not set correctly")
	}
}
