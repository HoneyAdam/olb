package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/config"
	"github.com/openloadbalancer/olb/internal/engine"
)

// ========== LOAD TEST FRAMEWORK ==========

type LoadTestResult struct {
	Name          string
	TotalRequests int
	Concurrency   int
	Duration      time.Duration
	SuccessCount  int64
	ErrorCount    int64
	StatusCodes   map[int]int64
	RPS           float64
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	MaxLatency    time.Duration
	MinLatency    time.Duration
	BytesReceived int64
	Throughput    float64 // MB/s
	BackendHits   map[string]int64
	Latencies     []time.Duration
}

func (r *LoadTestResult) Calculate() {
	if r.SuccessCount+r.ErrorCount == 0 {
		return
	}
	total := r.SuccessCount + r.ErrorCount
	r.RPS = float64(total) / r.Duration.Seconds()
	r.Throughput = float64(r.BytesReceived) / r.Duration.Seconds() / 1024 / 1024

	if len(r.Latencies) == 0 {
		return
	}
	sort.Slice(r.Latencies, func(i, j int) bool { return r.Latencies[i] < r.Latencies[j] })

	var sum time.Duration
	for _, l := range r.Latencies {
		sum += l
	}
	r.AvgLatency = sum / time.Duration(len(r.Latencies))
	r.MinLatency = r.Latencies[0]
	r.MaxLatency = r.Latencies[len(r.Latencies)-1]
	r.P50Latency = r.Latencies[len(r.Latencies)*50/100]
	r.P95Latency = r.Latencies[len(r.Latencies)*95/100]
	r.P99Latency = r.Latencies[len(r.Latencies)*99/100]
}

func runLoadTest(t *testing.T, name string, proxyAddr string, totalReqs, concurrency int) *LoadTestResult {
	t.Helper()
	result := &LoadTestResult{
		Name:          name,
		TotalRequests: totalReqs,
		Concurrency:   concurrency,
		StatusCodes:   make(map[int]int64),
		BackendHits:   make(map[string]int64),
		Latencies:     make([]time.Duration, 0, totalReqs),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 10 * time.Second}
			for j := 0; j < totalReqs/concurrency; j++ {
				reqStart := time.Now()
				resp, err := client.Get(fmt.Sprintf("http://%s/", proxyAddr))
				lat := time.Since(reqStart)

				mu.Lock()
				result.Latencies = append(result.Latencies, lat)
				mu.Unlock()

				if err != nil {
					atomic.AddInt64(&result.ErrorCount, 1)
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				atomic.AddInt64(&result.SuccessCount, 1)
				atomic.AddInt64(&result.BytesReceived, int64(len(body)))

				mu.Lock()
				result.StatusCodes[resp.StatusCode]++
				if b := resp.Header.Get("X-Backend"); b != "" {
					result.BackendHits[b]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	result.Duration = time.Since(start)
	result.Calculate()
	return result
}

// ========== BACKEND HELPERS ==========

type testBackend struct {
	name    string
	addr    string
	hits    atomic.Int64
	latency time.Duration
	server  *http.Server
}

func startTestBackends(t *testing.T, count int, baseLatency time.Duration) []*testBackend {
	t.Helper()
	backends := make([]*testBackend, count)
	for i := 0; i < count; i++ {
		b := &testBackend{
			name:    fmt.Sprintf("backend-%d", i+1),
			latency: baseLatency * time.Duration(i+1),
		}
		localB := b
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		b.addr = listener.Addr().String()
		b.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(200)
				return
			}
			if localB.latency > 0 {
				time.Sleep(localB.latency)
			}
			localB.hits.Add(1)
			w.Header().Set("X-Backend", localB.name)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"backend": localB.name,
				"time":    time.Now().UnixMicro(),
			})
		})}
		go b.server.Serve(listener)
		t.Cleanup(func() { localB.server.Close() })
		backends[i] = b
	}
	return backends
}

func startEngine(t *testing.T, backends []*testBackend, algorithm string, middleware *config.MiddlewareConfig, wafCfg *config.WAFConfig) (string, string) {
	t.Helper()
	proxyPort := getFreePort(t)
	adminPort := getFreePort(t)

	backendYAML := ""
	for _, b := range backends {
		backendYAML += fmt.Sprintf("      - address: \"%s\"\n        weight: 100\n", b.addr)
	}

	yamlCfg := fmt.Sprintf(`admin:
  address: "127.0.0.1:%d"
listeners:
  - name: http
    address: "127.0.0.1:%d"
    protocol: http
    routes:
      - path: /
        pool: bench-pool
pools:
  - name: bench-pool
    algorithm: %s
    backends:
%s    health_check:
      type: http
      interval: 1s
      timeout: 1s
      path: /health
`, adminPort, proxyPort, algorithm, backendYAML)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "olb.yaml")
	os.WriteFile(cfgPath, []byte(yamlCfg), 0644)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if middleware != nil {
		cfg.Middleware = middleware
	}
	if wafCfg != nil {
		cfg.WAF = wafCfg
	}

	eng, err := engine.New(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		eng.Shutdown(ctx)
	})

	proxyAddr := fmt.Sprintf("127.0.0.1:%d", proxyPort)
	waitForReady(t, proxyAddr, 5*time.Second)
	time.Sleep(2 * time.Second) // health checks
	return proxyAddr, fmt.Sprintf("127.0.0.1:%d", adminPort)
}

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func waitForReady(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for %s", addr)
}

// ========== REPORT GENERATION ==========

func printReport(t *testing.T, results []*LoadTestResult) {
	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════════╗")
	t.Log("║              OPENLOADBALANCER BENCHMARK REPORT                      ║")
	t.Log("╠══════════════════════════════════════════════════════════════════════╣")
	t.Log("")

	for _, r := range results {
		successRate := float64(r.SuccessCount) / float64(r.SuccessCount+r.ErrorCount) * 100
		t.Logf("  %-40s", r.Name)
		t.Logf("  ├─ Requests:    %d total, %d concurrent", r.TotalRequests, r.Concurrency)
		t.Logf("  ├─ Duration:    %v", r.Duration.Round(time.Millisecond))
		t.Logf("  ├─ RPS:         %.0f req/sec", r.RPS)
		t.Logf("  ├─ Success:     %.1f%% (%d ok, %d err)", successRate, r.SuccessCount, r.ErrorCount)
		t.Logf("  ├─ Throughput:  %.2f MB/s", r.Throughput)
		t.Logf("  ├─ Latency:")
		t.Logf("  │  ├─ avg: %v", r.AvgLatency.Round(time.Microsecond))
		t.Logf("  │  ├─ p50: %v", r.P50Latency.Round(time.Microsecond))
		t.Logf("  │  ├─ p95: %v", r.P95Latency.Round(time.Microsecond))
		t.Logf("  │  ├─ p99: %v", r.P99Latency.Round(time.Microsecond))
		t.Logf("  │  ├─ min: %v", r.MinLatency.Round(time.Microsecond))
		t.Logf("  │  └─ max: %v", r.MaxLatency.Round(time.Microsecond))
		if len(r.BackendHits) > 0 {
			t.Logf("  ├─ Backend Distribution:")
			for name, hits := range r.BackendHits {
				pct := float64(hits) / float64(r.SuccessCount) * 100
				bar := strings.Repeat("█", int(pct/5))
				t.Logf("  │  ├─ %-12s %4d hits (%5.1f%%) %s", name, hits, pct, bar)
			}
		}
		if len(r.StatusCodes) > 0 {
			t.Logf("  └─ Status Codes:")
			for code, count := range r.StatusCodes {
				t.Logf("       %d: %d", code, count)
			}
		}
		t.Log("")
	}

	t.Log("╚══════════════════════════════════════════════════════════════════════╝")
}

func generateMarkdownReport(t *testing.T, results []*LoadTestResult) string {
	var sb strings.Builder
	sb.WriteString("# OpenLoadBalancer Benchmark Report\n\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Platform:** %s\n\n", "Go "+fmt.Sprintf("%d.%d", 1, 25)))

	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Test | Requests | Concurrency | RPS | Avg Latency | P99 Latency | Success |\n")
	sb.WriteString("|------|----------|-------------|-----|-------------|-------------|--------|\n")
	for _, r := range results {
		successRate := float64(r.SuccessCount) / float64(r.SuccessCount+r.ErrorCount) * 100
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f | %v | %v | %.1f%% |\n",
			r.Name, r.TotalRequests, r.Concurrency, r.RPS,
			r.AvgLatency.Round(time.Microsecond), r.P99Latency.Round(time.Microsecond), successRate))
	}

	sb.WriteString("\n## Details\n\n")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("### %s\n\n", r.Name))
		sb.WriteString(fmt.Sprintf("- **RPS:** %.0f\n", r.RPS))
		sb.WriteString(fmt.Sprintf("- **Throughput:** %.2f MB/s\n", r.Throughput))
		sb.WriteString(fmt.Sprintf("- **Latency:** avg=%v p50=%v p95=%v p99=%v max=%v\n",
			r.AvgLatency.Round(time.Microsecond), r.P50Latency.Round(time.Microsecond),
			r.P95Latency.Round(time.Microsecond), r.P99Latency.Round(time.Microsecond),
			r.MaxLatency.Round(time.Microsecond)))
		if len(r.BackendHits) > 0 {
			sb.WriteString("\n| Backend | Hits | % |\n|---------|------|---|\n")
			for name, hits := range r.BackendHits {
				pct := float64(hits) / float64(r.SuccessCount) * 100
				sb.WriteString(fmt.Sprintf("| %s | %d | %.1f%% |\n", name, hits, pct))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ========== BENCHMARK TESTS ==========

func TestBenchmark_FullReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	var results []*LoadTestResult

	// --- Test 1: Algorithm comparison (1000 req, 50 concurrent) ---
	algorithms := []string{
		"round_robin", "weighted_round_robin", "least_connections",
		"ip_hash", "consistent_hash", "maglev", "power_of_two", "random",
	}

	for _, algo := range algorithms {
		backends := startTestBackends(t, 3, 0)
		proxyAddr, _ := startEngine(t, backends, algo, nil, nil)
		result := runLoadTest(t, fmt.Sprintf("Algorithm: %s", algo), proxyAddr, 1000, 50)
		results = append(results, result)
		for _, b := range backends {
			result.BackendHits[b.name] = b.hits.Load()
		}
	}

	// --- Test 2: Concurrency scaling ---
	for _, conc := range []int{1, 10, 50, 100} {
		backends := startTestBackends(t, 3, 0)
		proxyAddr, _ := startEngine(t, backends, "round_robin", nil, nil)
		result := runLoadTest(t, fmt.Sprintf("Concurrency: %d", conc), proxyAddr, 1000, conc)
		results = append(results, result)
	}

	// --- Test 3: Backend latency impact ---
	for _, lat := range []time.Duration{0, time.Millisecond, 5 * time.Millisecond, 10 * time.Millisecond} {
		backends := startTestBackends(t, 3, lat)
		proxyAddr, _ := startEngine(t, backends, "round_robin", nil, nil)
		name := fmt.Sprintf("Backend latency: %v", lat)
		if lat == 0 {
			name = "Backend latency: 0 (instant)"
		}
		result := runLoadTest(t, name, proxyAddr, 500, 50)
		results = append(results, result)
	}

	// --- Test 4: Middleware overhead ---
	// No middleware
	backends := startTestBackends(t, 3, 0)
	proxyAddr, _ := startEngine(t, backends, "round_robin", nil, nil)
	noMW := runLoadTest(t, "No middleware", proxyAddr, 1000, 50)
	results = append(results, noMW)

	// Full middleware stack
	backends2 := startTestBackends(t, 3, 0)
	mwCfg := &config.MiddlewareConfig{
		RateLimit:   &config.RateLimitConfig{Enabled: true, RequestsPerSecond: 50000, BurstSize: 50000},
		CORS:        &config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}},
		Compression: &config.CompressionConfig{Enabled: true, MinSize: 256},
	}
	proxyAddr2, _ := startEngine(t, backends2, "round_robin", mwCfg, nil)
	fullMW := runLoadTest(t, "Full middleware (rate+cors+gzip)", proxyAddr2, 1000, 50)
	results = append(results, fullMW)

	// WAF enabled
	backends3 := startTestBackends(t, 3, 0)
	wafCfg := &config.WAFConfig{Enabled: true, Mode: "block"}
	proxyAddr3, _ := startEngine(t, backends3, "round_robin", nil, wafCfg)
	wafResult := runLoadTest(t, "WAF enabled", proxyAddr3, 1000, 50)
	results = append(results, wafResult)

	// --- Test 5: Proxy overhead measurement ---
	backends4 := startTestBackends(t, 1, 0)
	proxyAddr4, _ := startEngine(t, backends4, "round_robin", nil, nil)

	// Direct to backend
	directClient := &http.Client{Timeout: 5 * time.Second}
	var directLatencies []time.Duration
	for i := 0; i < 200; i++ {
		start := time.Now()
		resp, err := directClient.Get(fmt.Sprintf("http://%s/", backends4[0].addr))
		if err == nil {
			io.ReadAll(resp.Body)
			resp.Body.Close()
			directLatencies = append(directLatencies, time.Since(start))
		}
	}
	sort.Slice(directLatencies, func(i, j int) bool { return directLatencies[i] < directLatencies[j] })
	var directSum time.Duration
	for _, l := range directLatencies {
		directSum += l
	}
	directAvg := directSum / time.Duration(len(directLatencies))

	// Through proxy
	var proxyLatencies []time.Duration
	for i := 0; i < 200; i++ {
		start := time.Now()
		resp, err := directClient.Get(fmt.Sprintf("http://%s/", proxyAddr4))
		if err == nil {
			io.ReadAll(resp.Body)
			resp.Body.Close()
			proxyLatencies = append(proxyLatencies, time.Since(start))
		}
	}
	sort.Slice(proxyLatencies, func(i, j int) bool { return proxyLatencies[i] < proxyLatencies[j] })
	var proxySum time.Duration
	for _, l := range proxyLatencies {
		proxySum += l
	}
	proxyAvg := proxySum / time.Duration(len(proxyLatencies))
	overhead := proxyAvg - directAvg

	overheadResult := &LoadTestResult{
		Name:          fmt.Sprintf("Proxy overhead: %v (direct=%v, proxy=%v)", overhead.Round(time.Microsecond), directAvg.Round(time.Microsecond), proxyAvg.Round(time.Microsecond)),
		TotalRequests: 200,
		Concurrency:   1,
		Duration:      proxySum,
		SuccessCount:  int64(len(proxyLatencies)),
		Latencies:     proxyLatencies,
		StatusCodes:   map[int]int64{200: int64(len(proxyLatencies))},
	}
	overheadResult.Calculate()
	results = append(results, overheadResult)

	// --- Print report ---
	printReport(t, results)

	// --- Generate markdown report ---
	report := generateMarkdownReport(t, results)
	reportPath := filepath.Join(os.TempDir(), "olb-benchmark-report.md")
	os.WriteFile(reportPath, []byte(report), 0644)
	t.Logf("Markdown report saved to: %s", reportPath)

	// --- Assertions ---
	for _, r := range results {
		if r.SuccessCount == 0 {
			t.Errorf("FAIL: %s had 0 successes", r.Name)
		}
		errorRate := float64(r.ErrorCount) / float64(r.SuccessCount+r.ErrorCount)
		if errorRate > 0.05 {
			t.Errorf("FAIL: %s error rate %.1f%% > 5%%", r.Name, errorRate*100)
		}
	}

	if overhead > 5*time.Millisecond {
		t.Errorf("FAIL: Proxy overhead %v > 5ms", overhead)
	}

	if noMW.RPS > 0 && fullMW.RPS > 0 {
		mwOverheadPct := (1 - fullMW.RPS/noMW.RPS) * 100
		t.Logf("Middleware RPS overhead: %.1f%% (no-mw: %.0f, full-mw: %.0f)", mwOverheadPct, noMW.RPS, fullMW.RPS)
	}

	_ = math.Max // ensure import
}
