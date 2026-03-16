package waf

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openloadbalancer/olb/internal/config"
	"github.com/openloadbalancer/olb/internal/waf/detection"
	"github.com/openloadbalancer/olb/internal/waf/detection/sqli"
	"github.com/openloadbalancer/olb/internal/waf/detection/xss"
	"github.com/openloadbalancer/olb/internal/waf/ipacl"
	"github.com/openloadbalancer/olb/internal/waf/sanitizer"
)

func BenchmarkIPACLLookup(b *testing.B) {
	acl, _ := ipacl.New(ipacl.Config{
		Whitelist: []ipacl.EntryConfig{{CIDR: "10.0.0.0/8", Reason: "test"}},
		Blacklist: []ipacl.EntryConfig{
			{CIDR: "203.0.113.0/24", Reason: "test"},
			{CIDR: "198.51.100.0/24", Reason: "test"},
		},
	})
	defer acl.Stop()

	b.ResetTimer()
	for b.Loop() {
		acl.Check("192.168.1.1")
	}
}

func BenchmarkSanitizerProcess(b *testing.B) {
	s := sanitizer.New(sanitizer.DefaultConfig())
	req := httptest.NewRequest("GET", "http://example.com/api/users?page=1&sort=name&q=hello+world", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	req.Header.Set("Accept", "application/json")

	b.ResetTimer()
	for b.Loop() {
		s.Process(req)
	}
}

func BenchmarkSQLiDetector(b *testing.B) {
	d := sqli.New()
	ctx := &detection.RequestContext{
		DecodedQuery: "id=1&name=John+Smith&page=1&sort=created_at&filter=active",
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}

	b.ResetTimer()
	for b.Loop() {
		d.Detect(ctx)
	}
}

func BenchmarkSQLiDetector_Attack(b *testing.B) {
	d := sqli.New()
	ctx := &detection.RequestContext{
		DecodedQuery: "id=1' UNION SELECT username, password FROM users --",
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}

	b.ResetTimer()
	for b.Loop() {
		d.Detect(ctx)
	}
}

func BenchmarkXSSDetector(b *testing.B) {
	d := xss.New()
	ctx := &detection.RequestContext{
		DecodedQuery: "q=normal+search+query&page=1&lang=en",
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}

	b.ResetTimer()
	for b.Loop() {
		d.Detect(ctx)
	}
}

func BenchmarkFullPipeline_CleanRequest(b *testing.B) {
	mw, _ := NewWAFMiddleware(WAFMiddlewareConfig{
		Config: &config.WAFConfig{Enabled: true, Mode: "enforce"},
	})
	defer mw.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.Wrap(next)

	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequest("GET", "http://example.com/api/users?page=1", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkFullPipeline_WithIPACL(b *testing.B) {
	mw, _ := NewWAFMiddleware(WAFMiddlewareConfig{
		Config: &config.WAFConfig{
			Enabled: true,
			Mode:    "enforce",
			IPACL: &config.WAFIPACLConfig{
				Enabled:   true,
				Whitelist: []config.WAFIPACLEntry{{CIDR: "10.0.0.0/8", Reason: "test"}},
				Blacklist: []config.WAFIPACLEntry{{CIDR: "203.0.113.0/24", Reason: "test"}},
			},
		},
	})
	defer mw.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.Wrap(next)

	b.ResetTimer()
	for b.Loop() {
		req := httptest.NewRequest("GET", "http://example.com/api/users?page=1", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkFullPipeline_Parallel(b *testing.B) {
	mw, _ := NewWAFMiddleware(WAFMiddlewareConfig{
		Config: &config.WAFConfig{Enabled: true, Mode: "enforce"},
	})
	defer mw.Stop()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.Wrap(next)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "http://example.com/api/users?page=1", nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}
	})
}
