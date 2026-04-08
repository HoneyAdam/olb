# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project
High-performance zero-dependency L4/L7 load balancer written in Go.
- Website: https://openloadbalancer.dev
- Repo: github.com/openloadbalancer/olb
- Go 1.25+, single binary, ~380 Go files
- External deps: `golang.org/x/crypto` (bcrypt/OCSP), `golang.org/x/net` (http2), `golang.org/x/text` (indirect) — no other external deps allowed

## Build & Test
```bash
go build ./cmd/olb/                       # Build binary
go test ./...                              # Run all tests
go test -race ./...                        # Race detector (needs GCC/Linux)
go test -cover ./...                       # Coverage (~87%, must stay ≥85%)
go test ./internal/balancer/...            # Run single package tests
go test -run TestRoundRobin ./internal/balancer/  # Run single test
go test -bench=. -benchmem ./...          # Benchmarks
gofmt -w .                                 # Format all
go vet ./...                               # Lint
make build                                 # Build via Makefile
make test                                  # Test with coverage output
make check                                 # fmt + vet + lint + test
make build-all                             # Cross-platform (linux/darwin/windows/freebsd)
```
E2E tests live in `test/e2e/`. Example configs in `configs/` (YAML, TOML, HCL).

## Architecture
```
cmd/olb/main.go          → Entry point, delegates to internal/cli
internal/engine/          → Central orchestrator — engine.go + config.go + adapters.go + signals_{unix,windows}.go
internal/proxy/l7/        → HTTP reverse proxy + WebSocket/gRPC/SSE detection + shadow manager
internal/proxy/l4/        → TCP/UDP proxy, SNI routing, PROXY protocol
internal/balancer/        → 16 load balancing algorithms
internal/middleware/      → ~30 middleware components (config-gated)
internal/router/          → Radix trie router (router.go + radix.go)
internal/config/          → YAML/TOML/HCL/JSON config + hot reload + env var substitution
internal/admin/           → REST API + Web UI serving
internal/cluster/         → Raft consensus + SWIM gossip
internal/mcp/             → MCP server for AI integration (SSE + HTTP + stdio)
internal/tls/             → TLS, mTLS, OCSP stapling
internal/acme/            → ACME/Let's Encrypt client
internal/health/          → Active + passive health checking (HTTP, TCP, gRPC)
internal/waf/             → WAF: 6-layer security pipeline
internal/security/        → Request smuggling, header injection protection
internal/webui/           → Embedded SPA (vanilla JS/CSS)
internal/plugin/          → Plugin system with event bus
internal/discovery/       → Service discovery (static/DNS/file/Docker/Consul)
internal/geodns/          → Geographic DNS routing
internal/conn/            → Connection management + pooling
internal/logging/         → Structured JSON logging with rotating file output
internal/metrics/         → Prometheus metrics registry
internal/profiling/       → Go runtime profiling (pprof)
pkg/version/              → Version info injected via ldflags at build time
pkg/utils/                → Buffer pool, LRU, ring buffer, CIDR matcher
pkg/errors/               → Sentinel errors with context
```

### Request Flow
```
Client → Listener → Middleware Chain → Router (radix trie path match) → Pool (balancer selects backend) → Backend
                                                                                                      ↓
                                                                                               Health checker updates backend state
```

### Engine Lifecycle
`engine.New()` creates all components. `Start()` starts listeners, health checks, admin, cluster, config watcher. `Shutdown()` stops everything gracefully. `Reload()` re-reads config and reinitializes pools/routes/listeners.

## Key Rules
1. **No new external deps** — only stdlib + existing x/crypto, x/net, x/text
2. **All features must be wired in engine.go** — `New()` creates the component, `Start()` runs it
3. **Config-gated middleware** — each middleware has `enabled: true/false`
4. **Port 0 in tests** — never hardcode ports, use `net.Listen(":0")`
5. **gofmt clean** — CI enforces formatting
6. **Coverage ≥ 85%** — enforced by `make coverage-check`
7. **Only openloadbalancer.dev** — no other domains in any file
8. **Conventional commits** — `feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `perf:`
9. **Version via ldflags** — `pkg/version` fields set by Makefile at build time

## Balancer Algorithms
16 algorithms, all wired in `engine.initializePools()`. Config accepts full names and aliases:
| Algorithm | Config Names |
|-----------|-------------|
| Round Robin | `round_robin`, `rr` |
| Weighted RR | `weighted_round_robin`, `wrr` |
| Least Connections | `least_connections`, `lc` |
| Weighted LC | `weighted_least_connections`, `wlc` |
| Least Response Time | `least_response_time`, `lrt` |
| Weighted LRT | `weighted_least_response_time`, `wlrt` |
| IP Hash | `ip_hash`, `iphash` |
| Consistent Hash | `consistent_hash`, `ch`, `ketama` |
| Maglev | `maglev` |
| Ring Hash | `ring_hash`, `ringhash` |
| Power of Two | `power_of_two`, `p2c` |
| Random | `random` |
| Weighted Random | `weighted_random`, `wrandom` |
| Rendezvous Hash | `rendezvous`, `rendezvous_hash` |
| Peak EWMA | `peak_ewma`, `pewma` |
| Sticky Sessions | `sticky` |

## Config Format
```yaml
listeners:
  - name: http
    address: ":8080"
    protocol: http          # http, https, tcp, udp
    routes:
      - path: /
        pool: my-pool

pools:
  - name: my-pool
    algorithm: round_robin
    backends:
      - address: "localhost:3001"
      - address: "localhost:3002"
    health_check:
      type: http
      path: /health
      interval: 10s

middleware:
  rate_limit:
    enabled: true
    requests_per_second: 100
  cors:
    enabled: true
admin:
  address: ":9090"

waf:
  enabled: true
  mode: enforce               # enforce, monitor, disabled
  ip_acl:
    enabled: true
    whitelist: [{cidr: "10.0.0.0/8", reason: "internal"}]
    auto_ban: {enabled: true, default_ttl: "1h"}
  rate_limit:
    enabled: true
    rules:
      - {id: "per-ip", scope: "ip", limit: 1000, window: "1m"}
  sanitizer: {enabled: true}
  detection:
    enabled: true
    threshold: {block: 50, log: 25}
    detectors:
      sqli: {enabled: true}
      xss: {enabled: true}
  bot_detection: {enabled: true, mode: monitor}
  response:
    security_headers: {enabled: true}
    data_masking: {enabled: true, mask_credit_cards: true}
```
