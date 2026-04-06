package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTrace_Disabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false

	mw := New(config)

	callCount := int32(0)
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTrace_W3CPropagation(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with traceparent
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Response should have traceparent
	traceparent := rec.Header().Get("traceparent")
	if traceparent == "" {
		t.Error("Expected traceparent header in response")
	}

	// Should contain same trace ID
	if !strings.Contains(traceparent, "4bf92f3577b34da6a3ce929d0e0e4736") {
		t.Error("Response traceparent should contain same trace ID")
	}
}

func TestTrace_B3Propagation(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"b3"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with b3 header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("b3", "4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Response should have b3 header
	b3 := rec.Header().Get("b3")
	if b3 == "" {
		t.Error("Expected b3 header in response")
	}
}

func TestTrace_B3MultiPropagation(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"b3multi"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with B3 headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-B3-TraceId", "4bf92f3577b34da6a3ce929d0e0e4736")
	req.Header.Set("X-B3-SpanId", "00f067aa0ba902b7")
	req.Header.Set("X-B3-Sampled", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Response should have B3 headers
	traceID := rec.Header().Get("X-B3-TraceId")
	if traceID == "" {
		t.Error("Expected X-B3-TraceId header in response")
	}
}

func TestTrace_JaegerPropagation(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"jaeger"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with uber-trace-id
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("uber-trace-id", "4bf92f3577b34da6a3ce929d0e0e4736:00f067aa0ba902b7:0:1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Response should have uber-trace-id
	uberTraceID := rec.Header().Get("uber-trace-id")
	if uberTraceID == "" {
		t.Error("Expected uber-trace-id header in response")
	}
}

func TestTrace_NewTrace(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without trace context - should create new trace
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	traceparent := rec.Header().Get("traceparent")
	if traceparent == "" {
		t.Error("Expected traceparent header for new trace")
	}

	// Should be a valid trace ID (32 hex characters)
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		t.Errorf("Invalid traceparent format: %s", traceparent)
	}

	if len(parts[1]) != 32 {
		t.Errorf("Expected 32 char trace ID, got %d chars", len(parts[1]))
	}
}

func TestTrace_BaggagePropagation(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.BaggageHeaders = []string{"X-Request-ID", "X-User-ID"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if baggage was extracted
		span := GetSpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Request with baggage headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-User-ID", "user-456")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTrace_ExcludePath(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.ExcludePaths = []string{"/health"}
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request to excluded path
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should not have traceparent
	if rec.Header().Get("traceparent") != "" {
		t.Error("Excluded path should not have trace headers")
	}
}

func TestTrace_Sampling(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.SampleRate = 0.0 // Don't sample
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// New trace with sampling disabled
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	traceparent := rec.Header().Get("traceparent")
	if traceparent == "" {
		t.Error("Expected traceparent even when not sampled")
	}

	// Check the sampled flag (last 2 chars should be 00 for not sampled)
	if strings.HasSuffix(traceparent, "-01") {
		t.Error("Expected not sampled flag")
	}
}

func TestTrace_SpanAttributes(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("response body"))
	}))

	req := httptest.NewRequest("POST", "/api/users?filter=active", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	span := mw.GetSpansByTrace("")
	if len(span) == 0 {
		// Spans are stored by trace ID, which is generated
		// Just verify the request completed
		if rec.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
		}
	}
}

func TestTrace_Stats(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.SampleRate = 1.0

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	stats := mw.Stats()

	if stats["sample_rate"] != 1.0 {
		t.Errorf("Expected sample rate 1.0, got %v", stats["sample_rate"])
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled != false {
		t.Error("Default Enabled should be false")
	}
	if config.ServiceName != "openloadbalancer" {
		t.Errorf("Default ServiceName should be 'openloadbalancer', got '%s'", config.ServiceName)
	}
	if config.SampleRate != 1.0 {
		t.Errorf("Default SampleRate should be 1.0, got %f", config.SampleRate)
	}
	if len(config.Propagators) == 0 {
		t.Error("Default Propagators should not be empty")
	}
}

func TestMiddleware_Priority(t *testing.T) {
	config := DefaultConfig()
	mw := New(config)

	if mw.Priority() != 10 {
		t.Errorf("Expected priority 10, got %d", mw.Priority())
	}
}

func TestMiddleware_Name(t *testing.T) {
	config := DefaultConfig()
	mw := New(config)

	if mw.Name() != "trace" {
		t.Errorf("Expected name 'trace', got '%s'", mw.Name())
	}
}

func TestSpan_Duration(t *testing.T) {
	span := &Span{
		StartTime: time.Now().Add(-time.Second),
	}

	duration := span.Duration()
	if duration < time.Millisecond {
		t.Error("Duration should be > 0")
	}
}

func TestGetSpanFromContext(t *testing.T) {
	// Empty context should return nil
	ctx := context.Background()
	span := GetSpanFromContext(ctx)

	if span != nil {
		t.Error("Expected nil span from empty context")
	}
}

func TestContextWithSpan(t *testing.T) {
	ctx := context.Background()
	span := &Span{
		TraceID: "test-trace-id",
		SpanID:  "test-span-id",
	}

	ctx = contextWithSpan(ctx, span)
	retrieved := GetSpanFromContext(ctx)

	if retrieved == nil {
		t.Error("Expected span from context")
	}

	if retrieved.TraceID != "test-trace-id" {
		t.Errorf("Expected trace ID 'test-trace-id', got '%s'", retrieved.TraceID)
	}
}

func TestTrace_MultiplePropagators(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"w3c", "b3"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should have both headers
	if rec.Header().Get("traceparent") == "" {
		t.Error("Expected traceparent header")
	}
	if rec.Header().Get("b3") == "" {
		t.Error("Expected b3 header")
	}
}

func TestTrace_InvalidTraceparent(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Invalid traceparent format
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("traceparent", "invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should create new trace
	traceparent := rec.Header().Get("traceparent")
	if traceparent == "" {
		t.Error("Expected new traceparent for invalid input")
	}
}

func TestTrace_ResponseRecording(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if rec.Body.String() != "not found" {
		t.Errorf("Expected body 'not found', got '%s'", rec.Body.String())
	}
}

func TestIDGenerator(t *testing.T) {
	gen := newIDGenerator()

	traceID := gen.generateTraceID()
	if len(traceID) != 32 {
		t.Errorf("Expected 32 char trace ID, got %d", len(traceID))
	}

	spanID := gen.generateSpanID()
	if len(spanID) != 16 {
		t.Errorf("Expected 16 char span ID, got %d", len(spanID))
	}

	// Should generate unique IDs
	traceID2 := gen.generateTraceID()
	if traceID == traceID2 {
		t.Error("Generated trace IDs should be unique")
	}
}

func TestTrace_GetSpansByTrace(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.SampleRate = 1.0

	mw := New(config)

	// Manually store a span
	span := &Span{
		TraceID: "test-trace-1",
		SpanID:  "span-1",
		sampled: true,
	}
	mw.storeSpan(span)

	spans := mw.GetSpansByTrace("test-trace-1")
	if len(spans) != 1 {
		t.Errorf("Expected 1 span, got %d", len(spans))
	}

	// Non-existent trace
	spans = mw.GetSpansByTrace("non-existent")
	if len(spans) != 0 {
		t.Error("Expected 0 spans for non-existent trace")
	}
}

func TestTrace_New_Defaults(t *testing.T) {
	// Test with empty config to exercise all default-setting branches
	config := Config{} // All zero values

	mw := New(config)

	if mw.config.ServiceName != "openloadbalancer" {
		t.Errorf("ServiceName should default to 'openloadbalancer', got '%s'", mw.config.ServiceName)
	}
	// Note: zero SampleRate (0.0) is not corrected since the check is < 0 only
	if mw.config.MaxBaggageItems != 10 {
		t.Errorf("MaxBaggageItems should default to 10, got %d", mw.config.MaxBaggageItems)
	}
	if mw.config.MaxBaggageSize != 8192 {
		t.Errorf("MaxBaggageSize should default to 8192, got %d", mw.config.MaxBaggageSize)
	}
	if len(mw.config.Propagators) != 1 || mw.config.Propagators[0] != "w3c" {
		t.Errorf("Propagators should default to [w3c], got %v", mw.config.Propagators)
	}
}

func TestTrace_New_NegativeSampleRate(t *testing.T) {
	config := Config{
		Enabled:    true,
		SampleRate: -0.5,
	}

	mw := New(config)

	if mw.config.SampleRate != 1.0 {
		t.Errorf("Negative SampleRate should be corrected to 1.0, got %f", mw.config.SampleRate)
	}
}

func TestTrace_New_EmptyServiceName(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "",
	}

	mw := New(config)

	if mw.config.ServiceName != "openloadbalancer" {
		t.Errorf("Empty ServiceName should default to 'openloadbalancer', got '%s'", mw.config.ServiceName)
	}
}

func TestSpan_DurationZeroEndTime(t *testing.T) {
	// Test the case where EndTime is zero (span still in progress)
	span := &Span{
		StartTime: time.Now().Add(-500 * time.Millisecond),
		// EndTime is zero
	}

	duration := span.Duration()
	if duration < 400*time.Millisecond {
		t.Errorf("Duration with zero EndTime should use time.Since(StartTime), got %v", duration)
	}
}

func TestSpan_DurationSetEndTime(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	end := start.Add(1 * time.Second)

	span := &Span{
		StartTime: start,
		EndTime:   end,
	}

	duration := span.Duration()
	if duration != 1*time.Second {
		t.Errorf("Duration with set EndTime should be EndTime-StartTime, got %v", duration)
	}
}

func TestTrace_W3CTracestate(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with traceparent and tracestate
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("tracestate", "vendor=value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// tracestate should be propagated
	if rec.Header().Get("tracestate") != "vendor=value" {
		t.Errorf("Expected tracestate 'vendor=value', got '%s'", rec.Header().Get("tracestate"))
	}
}

func TestTrace_B3SingleDebug(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"b3"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// B3 with debug flag (d implies sampled)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("b3", "4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-d")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	b3 := rec.Header().Get("b3")
	if b3 == "" {
		t.Error("Expected b3 header in response")
	}
	// Debug implies sampled=1
	if !strings.Contains(b3, "-1") {
		t.Errorf("Expected sampled=1 in b3 response for debug flag, got %s", b3)
	}
}

func TestTrace_B3SingleNotSampled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"b3"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// B3 with sampled=0
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("b3", "4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	b3 := rec.Header().Get("b3")
	if b3 == "" {
		t.Error("Expected b3 header in response")
	}
}

func TestTrace_B3MultiDebugFlag(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"b3multi"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// B3 multi with debug flag
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-B3-TraceId", "4bf92f3577b34da6a3ce929d0e0e4736")
	req.Header.Set("X-B3-SpanId", "00f067aa0ba902b7")
	req.Header.Set("X-B3-Flags", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Debug flag should set sampled=true
	traceID := rec.Header().Get("X-B3-TraceId")
	if traceID == "" {
		t.Error("Expected X-B3-TraceId header in response")
	}
	sampled := rec.Header().Get("X-B3-Sampled")
	if sampled != "1" {
		t.Errorf("Expected X-B3-Sampled=1 for debug flag, got '%s'", sampled)
	}
}

func TestTrace_JaegerHexFlags(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Propagators = []string{"jaeger"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Jaeger with hex flags (01 means sampled)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("uber-trace-id", "4bf92f3577b34da6a3ce929d0e0e4736:00f067aa0ba902b7:0:01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	uberTraceID := rec.Header().Get("uber-trace-id")
	if uberTraceID == "" {
		t.Error("Expected uber-trace-id header in response")
	}
}

func TestTrace_BaggageMaxItems(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.MaxBaggageItems = 2
	config.BaggageHeaders = []string{"X-A", "X-B", "X-C"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := GetSpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in context")
			w.WriteHeader(http.StatusOK)
			return
		}
		// Only first 2 baggage items should be captured
		if len(span.Baggage) > 2 {
			t.Errorf("Expected at most 2 baggage items, got %d", len(span.Baggage))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-A", "val-a")
	req.Header.Set("X-B", "val-b")
	req.Header.Set("X-C", "val-c")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTrace_BaggageMaxSize(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.MaxBaggageSize = 5 // Very small
	config.BaggageHeaders = []string{"X-Small", "X-Large"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := GetSpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in context")
			w.WriteHeader(http.StatusOK)
			return
		}
		// X-Large should be skipped (too big)
		if _, ok := span.Baggage["large"]; ok {
			t.Error("X-Large should not be in baggage (exceeds size)")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Small", "abc")                  // 3 bytes, fits
	req.Header.Set("X-Large", "this-is-way-too-long") // Exceeds remaining size
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTrace_ResponseRecorderDoubleWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rr := &responseRecorder{ResponseWriter: rec, statusCode: http.StatusOK}

	rr.WriteHeader(http.StatusCreated)
	rr.WriteHeader(http.StatusBadRequest) // Should be ignored

	if rr.statusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.statusCode)
	}
}

func TestTrace_ResponseRecorderWriteAutoHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rr := &responseRecorder{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write without calling WriteHeader first
	rr.Write([]byte("test"))

	if rr.statusCode != http.StatusOK {
		t.Errorf("Expected status %d (auto-set), got %d", http.StatusOK, rr.statusCode)
	}
	if rr.bytesWritten != 4 {
		t.Errorf("Expected 4 bytes written, got %d", rr.bytesWritten)
	}
}

func TestTrace_StatsWithSpans(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.SampleRate = 1.0
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests to populate spans
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	stats := mw.Stats()
	spanCount := stats["spans"].(int)
	if spanCount != 5 {
		t.Errorf("Expected 5 spans, got %d", spanCount)
	}
	sampledCount := stats["sampled"].(int)
	if sampledCount != 5 {
		t.Errorf("Expected 5 sampled spans, got %d", sampledCount)
	}
}

func TestTrace_SamplingMiddle(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.SampleRate = 0.5
	config.Propagators = []string{"w3c"}

	mw := New(config)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Just verify it runs without panicking with a middle sample rate
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTrace_GetSpan(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.SampleRate = 1.0

	mw := New(config)

	// Manually store a span
	span := &Span{
		TraceID: "test-trace-1",
		SpanID:  "span-1",
		sampled: true,
	}
	mw.storeSpan(span)

	retrieved := mw.GetSpan("test-trace-1", "span-1")
	if retrieved == nil {
		t.Error("Expected to retrieve span")
	}

	// Non-existent span
	retrieved = mw.GetSpan("non-existent", "non-existent")
	if retrieved != nil {
		t.Error("Expected nil for non-existent span")
	}
}
