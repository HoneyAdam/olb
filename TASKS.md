# OpenLoadBalancer — TASKS v1.0

> **Reference**: SPECIFICATION.md + IMPLEMENTATION.md
> **Methodology**: Each task is atomic, testable, and completable in 1-4 hours
> **Convention**: `[ ]` = todo, `[x]` = done, `[~]` = in progress, `[-]` = blocked

---

## Phase 1: Core L7 Proxy (v0.1.0) — MVP

### 1.1 Project Bootstrap
- [ ] Initialize Go module (`go mod init github.com/openloadbalancer/olb`)
- [ ] Create full directory structure (cmd, internal, pkg, configs, docs, test, scripts)
- [ ] Create `pkg/version/version.go` with build-time injection (Version, Commit, Date)
- [ ] Create `Makefile` with build, test, lint, bench, build-all targets
- [ ] Create `Dockerfile` (multi-stage, alpine-based)
- [ ] Create `.github/workflows/ci.yml` (test, build, lint)
- [ ] Create `llms.txt` (LLM-friendly project description)
- [ ] Create `README.md` (project overview, quick start, architecture)
- [ ] Create `.gitignore` (Go standard + bin/ + coverage files)
- [ ] Create `LICENSE` (Apache 2.0)
- [ ] Create `cmd/olb/main.go` (entry point stub)

### 1.2 Core Utilities (pkg/utils)
- [ ] Implement `BufferPool` — `sync.Pool` based buffer management with multiple size tiers
- [ ] Write unit tests for BufferPool (get, put, size matching, oversized allocation)
- [ ] Implement `RingBuffer[T]` — generic lock-free circular buffer (SPSC)
- [ ] Write unit tests for RingBuffer (push, pop, full, empty, snapshot, concurrent)
- [ ] Implement `LRU[K,V]` — thread-safe LRU cache with TTL support
- [ ] Write unit tests for LRU (get, put, evict, TTL expiry, concurrent access)
- [ ] Implement `AtomicFloat64` — atomic operations on float64
- [ ] Implement `AtomicDuration` — atomic operations on time.Duration
- [ ] Write unit tests for atomic helpers
- [ ] Implement `FastRand` — SplitMix64 pseudo-random number generator
- [ ] Write unit tests for FastRand (distribution, uniqueness)
- [ ] Implement `CIDRMatcher` — radix trie based IP/CIDR matching
- [ ] Write unit tests for CIDRMatcher (IPv4, IPv6, CIDR ranges, edge cases)
- [ ] Implement `BloomFilter` — probabilistic set membership
- [ ] Write unit tests for BloomFilter (add, contains, false positive rate)
- [ ] Implement IP utility functions (extractIP, isPrivate, parsePort)
- [ ] Implement time/duration utility functions (parseDuration, parseByteSize)

### 1.3 Error Types (pkg/errors)
- [ ] Define sentinel errors (ErrBackendNotFound, ErrPoolNotFound, ErrRouteNotFound, etc.)
- [ ] Implement error wrapping helpers with context
- [ ] Implement error code system (for API responses)
- [ ] Write unit tests for error types and wrapping

### 1.4 Structured Logger (internal/logging)
- [ ] Define `Level` type with Trace through Fatal
- [ ] Implement `Logger` struct with atomic level, inherited fields, child loggers
- [ ] Implement zero-alloc fast path (level check before allocation)
- [ ] Implement `Field` type with string, int, float, bool, error, duration variants
- [ ] Implement `JSONOutput` — manual JSON encoding (no encoding/json on hot path)
- [ ] Implement `TextOutput` — human-readable format for development
- [ ] Implement `MultiOutput` — fan-out to multiple outputs
- [ ] Implement `RotatingFileOutput` — file rotation by size, max backups, compression
- [ ] Implement SIGUSR1 handler for log file reopening
- [ ] Implement `Logger.With()` for child loggers with inherited fields
- [ ] Write unit tests for all log levels, outputs, rotation
- [ ] Write benchmark tests comparing to stdlib log

### 1.5 Config System (internal/config)

#### 1.5.1 YAML Parser
- [ ] Implement YAML lexer/scanner (tokenization)
  - [ ] Handle indentation tracking (indent stack, INDENT/DEDENT tokens)
  - [ ] Handle scalar values (strings, numbers, booleans, null)
  - [ ] Handle quoted strings (single, double, with escape sequences)
  - [ ] Handle multi-line strings (literal `|`, folded `>`)
  - [ ] Handle comments (#)
  - [ ] Handle flow collections (`{}`, `[]`)
  - [ ] Handle anchors (`&`) and aliases (`*`)
- [ ] Implement YAML parser (recursive descent, token → AST)
  - [ ] Parse mappings (key: value)
  - [ ] Parse sequences (- item)
  - [ ] Parse nested structures (indentation-based)
  - [ ] Parse flow mappings and sequences
  - [ ] Resolve anchors and aliases
- [ ] Implement YAML decoder (AST → Go struct via reflection)
  - [ ] Handle struct field matching (case-insensitive + yaml tags)
  - [ ] Handle type conversions (string→int, string→bool, string→duration, string→byteSize)
  - [ ] Handle maps, slices, nested structs
  - [ ] Handle pointer types and interfaces
  - [ ] Handle `${ENV_VAR}` and `${ENV_VAR:-default}` substitution
- [ ] Write comprehensive unit tests for YAML parser
  - [ ] Test all scalar types (int, float, bool, null, string)
  - [ ] Test nested mappings and sequences
  - [ ] Test multi-line strings
  - [ ] Test anchors/aliases
  - [ ] Test flow collections
  - [ ] Test edge cases (empty values, special characters, unicode)
- [ ] Write fuzz tests for YAML parser
- [ ] Test against the full olb.yaml example config

#### 1.5.2 JSON Parser
- [ ] Use stdlib `encoding/json` (this IS stdlib)
- [ ] Implement JSON → generic map adapter for config loader
- [ ] Write unit tests for JSON config loading

#### 1.5.3 Config Structure
- [ ] Define `Config` struct with all sections (Global, Admin, Metrics, Listeners, Backends, Routes, Cluster, MCP)
- [ ] Define `DefaultConfig()` with sensible defaults for all fields
- [ ] Implement `Config.Validate()` — comprehensive validation
  - [ ] Validate addresses (host:port format)
  - [ ] Validate durations (parseable)
  - [ ] Validate algorithm names (known algorithms)
  - [ ] Validate backend references in routes (exist)
  - [ ] Validate TLS cert/key file accessibility
  - [ ] Detect port conflicts between listeners
  - [ ] Detect circular references
- [ ] Write unit tests for validation (valid configs, each error case)

#### 1.5.4 Config Loader
- [ ] Implement `Loader` — detect format from extension, parse, decode, validate
- [ ] Implement environment variable overlay (`OLB_` prefix, `__` separator)
- [ ] Implement config merge (file + env + CLI flags)
- [ ] Write unit tests for loader (YAML, JSON, env overlay, merge)

#### 1.5.5 Config Watcher
- [ ] Implement `FileWatcher` — polling-based file change detection
- [ ] Use SHA-256 content hash to avoid false positives
- [ ] Write unit tests for watcher (detect change, ignore no-change)

### 1.6 Metrics Engine (internal/metrics)
- [ ] Implement `Counter` — atomic int64 counter
- [ ] Implement `Gauge` — atomic float64 gauge
- [ ] Implement `Histogram` — log-linear bucketed histogram with percentiles
- [ ] Implement `TimeSeries` — ring buffer based time-bucketed data
- [ ] Implement `Engine` — metrics registry with pre-registered fast-path metrics
- [ ] Implement `CounterVec` / `GaugeVec` / `HistogramVec` — labeled metric families
- [ ] Implement Prometheus exposition format handler
- [ ] Implement JSON metrics API handler
- [ ] Write unit tests for all metric types (correctness, concurrency safety)
- [ ] Write benchmark tests (Counter.Inc, Histogram.Observe, Gauge.Set)

### 1.7 TLS Manager (internal/tls) — Basic
- [ ] Implement `Manager` — certificate storage with exact + wildcard matching
- [ ] Implement `GetCertificate` callback for `tls.Config`
- [ ] Implement `BuildTLSConfig` — create tls.Config from config
- [ ] Implement `ReloadCertificates` — hot-reload certs
- [ ] Implement `LoadCertificate` — load PEM cert + key from files
- [ ] Write unit tests (cert loading, SNI matching, wildcard matching, reload)

### 1.8 Backend Pool (internal/backend)
- [ ] Implement `Backend` struct with atomic stats (conns, requests, errors, latency)
- [ ] Implement `BackendState` state machine (Up, Down, Draining, Maintenance, Starting)
- [ ] Implement `Pool` — named collection of backends with balancer
- [ ] Implement `PoolManager` — manage multiple pools, lookup by name
- [ ] Implement `Pool.AddBackend`, `Pool.RemoveBackend`, `Pool.DrainBackend`
- [ ] Write unit tests for state transitions, stats tracking, pool operations

### 1.9 Load Balancer Algorithms (internal/balancer)
- [ ] Define `Balancer` interface (Name, Next, Add, Remove, Update, Stats)
- [ ] Define `RequestContext` struct (ClientIP, Method, Host, Path, Headers, SessionKey)
- [ ] Implement `RoundRobin` — simple atomic counter rotation
- [ ] Write unit tests for RoundRobin (distribution, empty pool, single backend)
- [ ] Implement `WeightedRoundRobin` — Nginx smooth weighted algorithm
- [ ] Write unit tests for WRR (weighted distribution, smooth spread, weight changes)
- [ ] Implement balancer registry — name → factory lookup
- [ ] Write benchmark tests for both algorithms

### 1.10 Health Checker (internal/health) — Basic
- [ ] Define health check config structs (HTTP, TCP)
- [ ] Implement `Checker` — orchestrator with per-backend check goroutines
- [ ] Implement HTTP health check (configurable path, method, expected status, timeout)
- [ ] Implement TCP health check (connection test with timeout)
- [ ] Implement state transition logic (consecutive OK/fail thresholds)
- [ ] Write unit tests (HTTP check mock, TCP check, state transitions)

### 1.11 Connection Manager (internal/conn)
- [ ] Implement `Manager` — global connection tracking, limits, per-source limits
- [ ] Implement `TrackedConn` — wrapped net.Conn with metadata
- [ ] Implement `Accept` — wrap conn with tracking and limit checks
- [ ] Implement `Release` — remove from tracking
- [ ] Implement `Drain` — wait for connections with timeout
- [ ] Implement `ActiveConnections` — snapshot of all connections
- [ ] Implement backend `Pool` — channel-based idle connection pool
- [ ] Implement `Pool.Get`, `Pool.Put`, `Pool.Close`
- [ ] Implement `PoolManager` — per-backend pool management
- [ ] Write unit tests for limits, tracking, pooling, drain

### 1.12 Router (internal/router)
- [ ] Implement `RadixTrie` — insert, match, with parameter and wildcard support
- [ ] Write unit tests for trie (exact, prefix, param, wildcard, priority)
- [ ] Implement `Router` — host-based trie selection + path matching + header/method checks
- [ ] Implement route hot-reload (atomic swap)
- [ ] Write unit tests for router (host matching, wildcard hosts, method filter, header filter)
- [ ] Write benchmark tests for route matching

### 1.13 Middleware Pipeline (internal/middleware) — Basic Set
- [ ] Implement `Chain` — middleware chain builder with priority ordering
- [ ] Implement `RequestContext` — per-request state (request, response, route, backend, metrics)
- [ ] Implement `ResponseWriter` wrapper — capture status code, byte count

#### Middleware: Request ID
- [ ] Generate UUID v4 (from crypto/rand)
- [ ] Inject X-Request-Id header (or use existing if present)
- [ ] Write unit tests

#### Middleware: Real IP
- [ ] Extract client IP from X-Forwarded-For, X-Real-IP, or RemoteAddr
- [ ] Handle trusted proxy list
- [ ] Write unit tests

#### Middleware: Rate Limiter
- [ ] Implement token bucket algorithm
- [ ] Implement per-key bucket management (client IP, header value, etc.)
- [ ] Implement rate limit response headers (X-RateLimit-Limit, Remaining, Reset, Retry-After)
- [ ] Implement cleanup goroutine for expired buckets
- [ ] Write unit tests (allow, deny, burst, refill, multiple keys)

#### Middleware: CORS
- [ ] Handle preflight OPTIONS requests
- [ ] Set Access-Control-Allow-* headers
- [ ] Configurable allowed origins, methods, headers, max-age
- [ ] Write unit tests

#### Middleware: Headers
- [ ] Add/remove/set request headers
- [ ] Add/remove/set response headers
- [ ] Security headers preset (HSTS, X-Content-Type-Options, X-Frame-Options, etc.)
- [ ] Write unit tests

#### Middleware: Compression
- [ ] Detect Accept-Encoding (gzip, deflate)
- [ ] Compress using stdlib compress/gzip, compress/flate
- [ ] Skip small responses (< configurable min size)
- [ ] Skip already-compressed content types
- [ ] Set Vary: Accept-Encoding
- [ ] Write unit tests

#### Middleware: Access Log
- [ ] Record request start time, method, path, client IP
- [ ] Record response status, body size, duration
- [ ] Support JSON and CLF format
- [ ] Write to logger (non-blocking)
- [ ] Write unit tests

#### Middleware: Metrics
- [ ] Record per-request metrics (duration, status, bytes)
- [ ] Record per-route and per-backend metrics
- [ ] Write unit tests

### 1.14 L7 HTTP Proxy (internal/proxy/l7)
- [ ] Implement `HTTPProxy` — main reverse proxy handler
- [ ] Implement `ServeHTTP` — create context, run middleware chain
- [ ] Implement `proxyRequest` — forward request to backend, stream response
- [ ] Implement `prepareOutboundRequest` — add proxy headers, rewrite path
- [ ] Implement hop-by-hop header stripping
- [ ] Implement X-Forwarded-For, X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host
- [ ] Implement request/response body streaming (chunked transfer)
- [ ] Implement error handling (backend down, timeout, connection refused)
- [ ] Write unit tests with httptest server as backend
- [ ] Write integration test (full proxy chain)
- [ ] Write benchmark tests (small payload, large payload)

### 1.15 HTTP Listener (internal/listener)
- [ ] Implement `Listener` interface (Start, Stop, Name, Address)
- [ ] Implement `HTTPListener` — wraps net/http.Server
- [ ] Implement `HTTPSListener` — HTTPListener with TLS config
- [ ] Implement graceful shutdown (http.Server.Shutdown)
- [ ] Implement ACME challenge handler integration point
- [ ] Write unit tests

### 1.16 Admin API (internal/admin) — Basic
- [ ] Implement admin HTTP server with auth middleware
- [ ] Implement `GET /api/v1/system/info` (version, uptime, state)
- [ ] Implement `GET /api/v1/system/health` (self health check)
- [ ] Implement `POST /api/v1/system/reload` (trigger config reload)
- [ ] Implement `GET /api/v1/backends` (list all pools)
- [ ] Implement `GET /api/v1/backends/:pool` (pool detail)
- [ ] Implement `POST /api/v1/backends/:pool` (add backend)
- [ ] Implement `DELETE /api/v1/backends/:pool/:backend` (remove backend)
- [ ] Implement `POST /api/v1/backends/:pool/:backend/drain` (drain)
- [ ] Implement `GET /api/v1/routes` (list routes)
- [ ] Implement `GET /api/v1/health` (all health status)
- [ ] Implement `GET /api/v1/metrics` (JSON metrics)
- [ ] Implement `GET /metrics` (Prometheus format)
- [ ] Implement basic auth middleware (username + bcrypt password)
- [ ] Implement bearer token auth middleware
- [ ] Write unit tests for each endpoint
- [ ] Write integration test for API flows

### 1.17 CLI (internal/cli) — Basic Commands
- [ ] Implement argument parser (commands, subcommands, flags)
- [ ] Implement output formatters (table, JSON)
- [ ] Implement `olb start` — load config, start engine
  - [ ] `--config` flag (default: olb.yaml)
  - [ ] `--daemon` flag (background)
  - [ ] `--pid-file` flag
- [ ] Implement `olb stop` — send SIGTERM to running process
- [ ] Implement `olb reload` — send SIGHUP or call admin API
- [ ] Implement `olb status` — query admin API for system info
- [ ] Implement `olb version` — print version info
- [ ] Implement `olb config validate` — validate config without starting
- [ ] Implement `olb backend list` — query admin API
- [ ] Implement `olb health show` — query admin API
- [ ] Write unit tests for parser, formatters

### 1.18 Engine Orchestrator (internal/engine)
- [ ] Implement `Engine` struct — central coordinator
- [ ] Implement `New(cfg)` — initialize all components in correct order
- [ ] Implement `Start()` — start all components, install signal handlers
- [ ] Implement `Shutdown(ctx)` — graceful drain + stop
- [ ] Implement `Reload()` — hot reload config
- [ ] Implement signal handler (SIGHUP→reload, SIGTERM/SIGINT→shutdown, SIGUSR1→reopen logs)
- [ ] Write integration test (start, send request, shutdown)

### 1.19 cmd/olb Entry Point
- [ ] Parse CLI args
- [ ] Route to appropriate command
- [ ] Handle `start` command: load config → create engine → start → wait
- [ ] Write end-to-end test (binary starts, accepts HTTP, proxies to backend)

### 1.20 Phase 1 Polish
- [ ] Write example configs (olb.yaml, olb.minimal.yaml)
- [ ] Write getting-started.md
- [ ] Run full test suite with race detector
- [ ] Run benchmarks, document baseline numbers
- [ ] Binary size check
- [ ] Startup time check
- [ ] Tag v0.1.0

---

## Phase 2: Advanced L7 + L4 (v0.2.0)

### 2.1 Additional Balancer Algorithms
- [ ] Implement `LeastConnections` with weighted variant
- [ ] Implement `LeastResponseTime` with sliding window
- [ ] Implement `IPHash` (FNV-1a based)
- [ ] Implement `ConsistentHash` (Ketama with virtual nodes)
- [ ] Implement `Maglev` (Google Maglev hashing, prime table 65537)
- [ ] Implement `PowerOfTwo` (P2C random choices)
- [ ] Implement `Random` and `WeightedRandom`
- [ ] Implement `RingHash` with configurable virtual nodes
- [ ] Write unit tests for each (distribution, backend add/remove stability)
- [ ] Write benchmark tests for all algorithms

### 2.2 Session Affinity
- [ ] Implement cookie-based sticky sessions (set/read OLB_SRV cookie)
- [ ] Implement header-based session affinity
- [ ] Implement URL parameter-based session affinity
- [ ] Configurable per-route
- [ ] Write unit tests

### 2.3 WebSocket Proxying
- [ ] Implement WebSocket upgrade detection
- [ ] Implement connection hijacking (client side)
- [ ] Implement upgrade forwarding to backend
- [ ] Implement bidirectional frame copy
- [ ] Implement ping/pong keepalive
- [ ] Implement idle timeout
- [ ] Write unit tests with gorilla/websocket test client (test only dep)

### 2.4 gRPC Proxying
- [ ] Implement HTTP/2 h2c (prior knowledge) support
- [ ] Implement trailer header propagation
- [ ] Implement streaming body forwarding
- [ ] Write unit tests

### 2.5 SSE Proxying
- [ ] Detect Accept: text/event-stream
- [ ] Disable response buffering for SSE routes
- [ ] Implement flush-after-each-event
- [ ] Write unit tests

### 2.6 HTTP/2 Support
- [ ] Integrate `golang.org/x/net/http2` (or implement minimal h2)
- [ ] HTTP/2 connection multiplexing to backends
- [ ] ALPN negotiation (h2, http/1.1)
- [ ] Write unit tests

### 2.7 TCP Proxy (L4)
- [ ] Implement `TCPProxy` — accept, select backend, bidirectional copy
- [ ] Implement `TCPListener` — raw TCP listener
- [ ] Implement zero-copy splice on Linux (+build linux)
- [ ] Implement fallback io.CopyBuffer on non-Linux
- [ ] Write unit tests (proxy MySQL-like protocol, large data transfer)
- [ ] Write benchmark tests (throughput, latency)

### 2.8 SNI-Based Routing
- [ ] Implement TLS ClientHello peek (extract SNI without consuming)
- [ ] Implement SNI → backend mapping
- [ ] Implement TLS passthrough mode (no termination)
- [ ] Write unit tests (SNI extraction, routing)

### 2.9 PROXY Protocol
- [ ] Implement PROXY protocol v1 parser (text format)
- [ ] Implement PROXY protocol v2 parser (binary format)
- [ ] Implement PROXY protocol v1 writer
- [ ] Implement PROXY protocol v2 writer
- [ ] Configurable per-listener (send/receive)
- [ ] Write unit tests

### 2.10 UDP Proxy
- [ ] Implement `UDPProxy` — session tracking by source addr
- [ ] Implement `UDPListener`
- [ ] Implement session timeout and cleanup
- [ ] Write unit tests (DNS proxy simulation)

### 2.11 Circuit Breaker Middleware
- [ ] Implement state machine (Closed → Open → Half-Open)
- [ ] Configurable error threshold, open duration, half-open requests
- [ ] Per-backend circuit breakers
- [ ] Write unit tests (state transitions, recovery)

### 2.12 Retry Middleware
- [ ] Implement retry with configurable max retries
- [ ] Implement exponential backoff with jitter
- [ ] Configurable retry-on status codes
- [ ] Only retry idempotent methods by default
- [ ] Write unit tests

### 2.13 Response Cache Middleware
- [ ] Implement LRU-based HTTP response cache
- [ ] Cache key generation (method + host + path + query)
- [ ] Respect Cache-Control headers
- [ ] Configurable max size, TTL
- [ ] Stale-while-revalidate support
- [ ] Write unit tests

### 2.14 Basic WAF Middleware
- [ ] Implement SQL injection pattern detection
- [ ] Implement XSS pattern detection
- [ ] Implement path traversal detection
- [ ] Implement command injection detection
- [ ] Configurable block vs log-only mode
- [ ] Write unit tests

### 2.15 IP Filter Middleware
- [ ] Implement allow/deny lists using CIDRMatcher
- [ ] Configurable per-route
- [ ] Write unit tests

### 2.16 Passive Health Checking
- [ ] Track error rates per backend from real traffic
- [ ] Sliding window counter
- [ ] Configurable error rate threshold
- [ ] Auto-disable backend on high error rate
- [ ] Auto-recovery after cooldown
- [ ] Write unit tests

### 2.17 TOML Parser
- [ ] Implement TOML v1.0 lexer
- [ ] Implement TOML parser (tables, arrays of tables, inline tables)
- [ ] Implement all value types (string, int, float, bool, datetime, array)
- [ ] Write unit tests + fuzz tests
- [ ] Test against olb.toml example config

### 2.18 HCL Parser
- [ ] Implement HCL lexer
- [ ] Implement HCL parser (blocks, attributes, expressions)
- [ ] Implement string interpolation (${...})
- [ ] Implement here-doc strings
- [ ] Write unit tests + fuzz tests
- [ ] Test against olb.hcl example config

### 2.19 Config Hot Reload
- [ ] Implement atomic config swap
- [ ] Implement config diff computation and logging
- [ ] Test hot reload of routes, backends, TLS certs, middleware
- [ ] Write integration test (change config, verify new behavior)

### 2.20 Phase 2 Polish
- [ ] Write example configs in TOML and HCL
- [ ] Update documentation
- [ ] Full test suite + benchmarks
- [ ] Tag v0.2.0

---

## Phase 3: Web UI + Advanced Features (v0.3.0)

### 3.1 Web UI — Foundation
- [ ] Create vanilla JS SPA framework (router, state, rendering)
- [ ] Create CSS design system (variables, dark/light theme, grid, components)
- [ ] Create reusable UI components (table, card, badge, button, form, modal)
- [ ] Create navigation (sidebar, breadcrumbs)
- [ ] Implement WebSocket client with auto-reconnect
- [ ] Implement `go:embed` for static assets
- [ ] Bundle and minify (or keep simple, no build step)

### 3.2 Web UI — Dashboard Page
- [ ] Live request rate sparkline
- [ ] Active connections gauge
- [ ] Error rate display
- [ ] Backend health grid
- [ ] Top routes by traffic
- [ ] Latency histogram
- [ ] System resources (CPU, memory, goroutines)
- [ ] Recent errors list
- [ ] Uptime, version display

### 3.3 Web UI — Backends Page
- [ ] Backend pool table with status, connections, RPS, latency
- [ ] Per-backend detail view
- [ ] Actions: drain, enable, disable, remove
- [ ] Add backend form
- [ ] Real-time health check results

### 3.4 Web UI — Routes Page
- [ ] Route table with match criteria
- [ ] Per-route metrics (RPS, latency p50/p95/p99, error rate)
- [ ] Route testing tool

### 3.5 Web UI — Metrics Page
- [ ] Interactive time-range charts
- [ ] Metric explorer with search/filter
- [ ] Export to JSON/CSV

### 3.6 Web UI — Logs Page
- [ ] Real-time log stream via WebSocket
- [ ] Full-text search
- [ ] Filter by level, route, backend, status
- [ ] Log entry detail view

### 3.7 Web UI — Config Page
- [ ] Current config viewer (syntax highlighted)
- [ ] Config diff view
- [ ] Reload button with confirmation

### 3.8 Web UI — Certificates Page
- [ ] Certificate inventory table
- [ ] Expiry warnings
- [ ] ACME status
- [ ] Force renewal button

### 3.9 Custom Chart Library
- [ ] Implement line chart with smooth curves
- [ ] Implement sparkline (minimal line, no axes)
- [ ] Implement bar chart (vertical, stacked)
- [ ] Implement gauge (semicircle)
- [ ] Implement histogram visualization
- [ ] Tooltip on hover
- [ ] Responsive canvas sizing

### 3.10 ACME Client (Let's Encrypt)
- [ ] Implement ACME v2 directory discovery
- [ ] Implement account registration (ECDSA P-256)
- [ ] Implement JWS signing for ACME requests
- [ ] Implement nonce management
- [ ] Implement order creation
- [ ] Implement HTTP-01 challenge solver
- [ ] Implement TLS-ALPN-01 challenge solver
- [ ] Implement CSR generation and order finalization
- [ ] Implement certificate download and parsing
- [ ] Implement certificate storage (PEM files)
- [ ] Implement auto-renewal (background goroutine, 30 days before expiry)
- [ ] Write unit tests (against Pebble or mock)
- [ ] Write integration test (full issuance flow)

### 3.11 OCSP Stapling
- [ ] Implement OCSP response fetching from CA
- [ ] Implement OCSP response caching
- [ ] Implement stapling into TLS handshake
- [ ] Implement background refresh
- [ ] Write unit tests

### 3.12 mTLS Support
- [ ] Implement client certificate validation
- [ ] Implement CA cert pool loading
- [ ] Implement upstream mTLS (OLB → backend)
- [ ] Configurable per-listener and per-backend
- [ ] Write unit tests

### 3.13 `olb top` TUI Dashboard
- [ ] Implement TUI engine (raw terminal, ANSI escape codes)
- [ ] Implement box drawing, progress bars, tables
- [ ] Implement color support
- [ ] Implement keyboard input handling
- [ ] Implement live metrics display
- [ ] Implement backend status view
- [ ] Implement route metrics view
- [ ] Implement key shortcuts ([q] quit, [b] backends, [r] routes, etc.)

### 3.14 Service Discovery
- [ ] Implement Discovery interface
- [ ] Implement static (config-based) provider
- [ ] Implement DNS SRV provider
- [ ] Implement DNS A/AAAA provider
- [ ] Implement file-based provider (watch JSON/YAML file)
- [ ] Implement Docker provider (unix socket, label-based)
- [ ] Write unit tests for each provider

### 3.15 Advanced CLI Commands
- [ ] Implement `olb backend add/remove/drain/enable/disable/stats`
- [ ] Implement `olb route add/remove/test`
- [ ] Implement `olb cert list/add/remove/renew/info`
- [ ] Implement `olb metrics show/export`
- [ ] Implement `olb log tail/search`
- [ ] Implement `olb config show/diff/generate`
- [ ] Implement shell completions (bash, zsh, fish)

### 3.16 Phase 3 Polish
- [ ] Responsive Web UI test (mobile, tablet)
- [ ] Web UI accessibility (ARIA labels, keyboard nav)
- [ ] Web UI bundle size check (< 2MB)
- [ ] Full test suite
- [ ] Tag v0.3.0

---

## Phase 4: Multi-Node Clustering (v0.4.0)

### 4.1 Gossip Protocol (SWIM)
- [ ] Implement UDP message serialization (binary format)
- [ ] Implement PING / ACK / PING-REQ message handlers
- [ ] Implement probe loop with random member selection
- [ ] Implement indirect probe (PING-REQ via random members)
- [ ] Implement SUSPECT / ALIVE / DEAD state transitions
- [ ] Implement incarnation numbers for state precedence
- [ ] Implement piggybacked broadcast queue
- [ ] Implement member join / leave handling
- [ ] Implement TCP fallback for large messages
- [ ] Write unit tests (membership, failure detection, state propagation)
- [ ] Write integration test (3-node cluster, kill one, detect failure)

### 4.2 Raft Consensus
- [ ] Implement Raft log (append, get, truncate, compact)
- [ ] Implement persistent state storage (term, votedFor, log)
- [ ] Implement RequestVote RPC (handler + sender)
- [ ] Implement AppendEntries RPC (handler + sender)
- [ ] Implement leader election with randomized timeout
- [ ] Implement log replication
- [ ] Implement commit index advancement
- [ ] Implement state machine application (apply committed entries)
- [ ] Implement snapshots (create, send, restore)
- [ ] Implement membership changes (joint consensus)
- [ ] Implement TCP transport for Raft RPCs
- [ ] Write unit tests (election, replication, commit, snapshot)
- [ ] Write integration test (3-node cluster, leader failure, re-election)

### 4.3 Config State Machine
- [ ] Implement config store as Raft state machine
- [ ] Implement config change proposal (leader only)
- [ ] Implement follower → leader forwarding
- [ ] Implement config apply on commit
- [ ] Write unit tests

### 4.4 Distributed State
- [ ] Implement health status propagation via gossip
- [ ] Implement distributed rate limiting (CRDT counters)
- [ ] Implement session affinity table propagation
- [ ] Write unit tests

### 4.5 Inter-Node Security
- [ ] Implement mTLS between cluster nodes
- [ ] Implement node authentication
- [ ] Write unit tests

### 4.6 Cluster Management
- [ ] Implement cluster join flow
- [ ] Implement cluster leave flow (graceful)
- [ ] Implement `olb cluster status/join/leave/members` CLI commands
- [ ] Implement cluster admin API endpoints
- [ ] Add cluster page to Web UI

### 4.7 Phase 4 Polish
- [ ] 3-node integration test
- [ ] 5-node integration test
- [ ] Network partition simulation
- [ ] Split-brain protection verification
- [ ] Tag v0.4.0

---

## Phase 5: AI Integration + Polish (v1.0.0)

### 5.1 MCP Server
- [ ] Implement MCP JSON-RPC protocol handler
- [ ] Implement stdio transport (stdin/stdout)
- [ ] Implement HTTP/SSE transport
- [ ] Implement `olb_query_metrics` tool
- [ ] Implement `olb_list_backends` tool
- [ ] Implement `olb_modify_backend` tool
- [ ] Implement `olb_modify_route` tool
- [ ] Implement `olb_diagnose` tool (error analysis, latency analysis, capacity)
- [ ] Implement `olb_get_logs` tool
- [ ] Implement `olb_get_config` tool
- [ ] Implement `olb_cluster_status` tool
- [ ] Implement MCP resources (metrics, config, health, logs)
- [ ] Implement MCP prompt templates (diagnose, capacity planning, canary deploy)
- [ ] Write unit tests for each tool
- [ ] Write integration test (Claude Code ↔ MCP Server)

### 5.2 Plugin System
- [ ] Implement Plugin interface
- [ ] Implement PluginAPI (register middleware, balancer, health check, discovery)
- [ ] Implement Go plugin loader (.so files)
- [ ] Implement plugin directory scanning
- [ ] Implement event system (subscribe/publish)
- [ ] Write example plugin (custom middleware)
- [ ] Write unit tests

### 5.3 Documentation
- [ ] Write comprehensive README.md
- [ ] Write getting-started.md (5-minute quick start)
- [ ] Write configuration.md (all options documented)
- [ ] Write algorithms.md (explain each algorithm with diagrams)
- [ ] Write clustering.md (setup, operation, troubleshooting)
- [ ] Write mcp.md (AI integration guide)
- [ ] Write api.md (REST API reference)
- [ ] Write llms.txt (LLM-friendly project summary)
- [ ] Write CHANGELOG.md

### 5.4 Performance Optimization Pass
- [ ] Profile CPU under load (go tool pprof)
- [ ] Profile memory under load
- [ ] Optimize hot path allocations (escape analysis)
- [ ] Verify buffer pool effectiveness
- [ ] Verify connection pool effectiveness
- [ ] Benchmark: HTTP RPS (target: >50K single core, >300K 8-core)
- [ ] Benchmark: TCP throughput (target: >10Gbps with splice)
- [ ] Benchmark: Latency overhead (target: <1ms p99 L7, <0.1ms p99 L4)
- [ ] Benchmark: Memory per connection (target: <4KB idle)
- [ ] Benchmark: Startup time (target: <500ms)
- [ ] Binary size check (target: <20MB)

### 5.5 Security Audit
- [ ] Review TLS configuration defaults
- [ ] Review admin API authentication
- [ ] Review input validation (config, API, headers)
- [ ] Review WAF rule coverage
- [ ] Test slow loris protection
- [ ] Test request smuggling prevention
- [ ] Test header injection prevention
- [ ] Review privilege dropping implementation

### 5.6 Packaging & Distribution
- [ ] Docker image (multi-arch: amd64, arm64)
- [ ] Docker Compose example
- [ ] Homebrew formula
- [ ] systemd service file
- [ ] DEB package
- [ ] RPM package
- [ ] Install script (curl | sh)
- [ ] GitHub Actions release workflow

### 5.7 v1.0.0 Release
- [ ] All tests pass with -race
- [ ] All benchmarks meet targets
- [ ] Documentation complete
- [ ] Example configs for all formats
- [ ] Docker images published
- [ ] Homebrew formula published
- [ ] GitHub release with binaries
- [ ] Blog post / announcement
- [ ] Tag v1.0.0

---

## Task Statistics

| Phase | Tasks | Estimated Hours |
|-------|-------|-----------------|
| Phase 1 (MVP) | ~120 | 200-280h |
| Phase 2 (Advanced) | ~60 | 120-160h |
| Phase 3 (Web UI) | ~55 | 140-180h |
| Phase 4 (Cluster) | ~30 | 100-140h |
| Phase 5 (AI+Polish) | ~40 | 80-120h |
| **Total** | **~305** | **640-880h** |

---

*Track progress by checking off tasks. Each phase should be tagged as a release before starting the next phase.*
