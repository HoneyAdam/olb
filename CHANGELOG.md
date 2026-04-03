# Changelog

All notable changes to OpenLoadBalancer will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Fix Dockerfile HEALTHCHECK syntax — shell `||` operator in JSON-form CMD didn't execute correctly
- Fix admin error message sanitization to prevent internal info leakage
- Fix cluster data race in state management
- Fix health check HTTP client reuse and SSE context cancellation
- Fix OCSP verification and cluster safety
- Fix CLI tests for cross-platform compatibility (TempDir for non-existent paths)
- Fix goroutine leaks, memory exhaustion, and race conditions across proxy, cluster, and engine
- Fix CORS, proxy trust, and secrets masking security hardening
- Fix connection release to O(1) and track config reload goroutines
- Fix response body close before checking JSON decode errors (ACME PollOrder)
- Fix cluster goroutine leaks and race conditions
- Fix connection-refused tests for deterministic cross-platform behavior

### Added
- SSE transport tests: auth, CORS, broadcast, audit logging, client lifecycle, message routing
- HTTP bearer auth tests: valid, invalid, missing token scenarios
- Package-level godoc for backend, yaml, metrics, and detection packages
- Comprehensive README.md with full feature documentation
- Updated llms.txt with complete project description for AI assistants

### Changed
- MCP test coverage improved from 57.9% to 88.5%
- README test badge updated from "56 E2E + 49 unit" to "138 files + 3300 funcs"
- Documentation updates across all files

---

## [0.5.0] - 2026-03-15 — AI Integration + Packaging

### Added

#### Phase 5.1: MCP Server (Model Context Protocol)
- **MCP JSON-RPC 2.0 protocol handler** with full request/response lifecycle
- **Stdio transport** -- stdin/stdout communication for CLI tools (Claude Code, etc.)
- **HTTP transport** -- HTTP POST endpoint for remote AI agent access
- **8 MCP tools:**
  - `olb_query_metrics` -- Query RPS, latency, error rates by scope and time range
  - `olb_list_backends` -- List all pools with backend status and connections
  - `olb_modify_backend` -- Add, remove, drain, enable, disable backends
  - `olb_modify_route` -- Add or remove routes dynamically
  - `olb_diagnose` -- Automated error analysis, latency analysis, capacity planning
  - `olb_get_logs` -- Retrieve recent log entries with level filtering
  - `olb_get_config` -- Get the current running configuration
  - `olb_cluster_status` -- Get cluster membership and leader info
- **MCP resources** -- `olb://metrics`, `olb://config`, `olb://health`, `olb://logs`
- **MCP prompt templates** -- Diagnose issues, capacity planning, canary deployment setup

#### Phase 5.2: Plugin System
- **Plugin interface** with lifecycle hooks (Init, Start, Stop)
- **PluginAPI** -- Register custom middleware, balancers, health checks, discovery providers
- **Go plugin loader** -- Dynamic `.so` file loading and symbol resolution
- **Plugin directory scanning** -- Auto-discover plugins at startup
- **Event system** -- Subscribe/publish pattern for plugin communication

#### Phase 5.6: Packaging & Distribution
- **Docker image** -- Multi-arch (amd64, arm64) Alpine-based image
- **Docker Compose** example for quick deployment
- **systemd service file** -- Production service management
- **DEB package** -- Debian/Ubuntu installation
- **RPM package** -- RHEL/Fedora installation
- **Install script** -- `curl | sh` one-liner installer
- **GitHub Actions release workflow** -- Automated release pipeline

#### Phase 5.3: Documentation
- Getting started guide (5-minute quick start)
- Full configuration reference (all options)
- Load balancing algorithms guide with selection criteria
- Clustering setup and operations guide
- MCP / AI integration guide
- REST API reference

---

## [0.4.0] - 2026-03-14 — Multi-Node Clustering

### Added

#### Phase 4.1: Gossip Protocol (SWIM)
- **UDP message serialization** -- Binary format for efficient network communication
- **PING / ACK / PING-REQ** message handlers for failure detection
- **Probe loop** with random member selection and configurable interval
- **Indirect probe** (PING-REQ via random members) for reducing false positives
- **SUSPECT / ALIVE / DEAD** state transitions with incarnation numbers
- **Piggybacked broadcast queue** for efficient state dissemination
- **Member join / leave** handling with graceful departure
- **TCP fallback** for large messages exceeding UDP MTU

#### Phase 4.2: Raft Consensus (Enhanced)
- **Raft log** -- Append, get, truncate, compact operations with persistence
- **Persistent state storage** -- Term, votedFor, log entries survive restarts
- **RequestVote RPC** with log up-to-date verification
- **AppendEntries RPC** with heartbeat and log consistency checks
- **Leader election** with randomized timeout and majority voting
- **Log replication** from leader to followers with backtracking
- **Commit index advancement** and state machine application
- **Snapshots** -- Create, send, and restore for log compaction
- **Membership changes** via joint consensus

#### Phase 4.3: Config State Machine
- **Config store as Raft state machine** -- All config changes go through Raft
- **Config change proposal** -- Leader-only writes with follower forwarding
- **Config apply on commit** -- Automatic application when Raft commits

#### Phase 4.4: Distributed State
- **Health status propagation** via gossip protocol
- **Distributed rate limiting** with CRDT counters across nodes
- **Session affinity table propagation** for cluster-wide sticky sessions

#### Phase 4.5: Inter-Node Security
- **mTLS between cluster nodes** -- All cluster communication encrypted
- **Node authentication** -- Certificate-based identity verification

#### Phase 4.6: Cluster Management
- **Cluster join flow** -- Node discovery and membership addition
- **Cluster leave flow** -- Graceful departure with state handoff
- **CLI commands** -- `olb cluster status/join/leave/members`
- **Cluster admin API endpoints** -- REST API for cluster operations

---

## [0.3.0] - 2026-03-14 — Web UI + Advanced Features

### Added

#### Phase 3.1-3.9: Web UI Dashboard
- **Vanilla JS SPA framework** -- Custom router, state management, component system
- **CSS design system** -- Dark/light themes, CSS variables, responsive grid
- **Reusable UI components** -- Table, card, badge, button, form, modal, tooltip
- **WebSocket client** with auto-reconnect for real-time updates
- **`go:embed`** for embedding static assets in binary (zero build step)
- **Dashboard page** -- Live request rate sparkline, active connections gauge, error rates, backend health grid, latency histogram, system resources, recent errors
- **Backends page** -- Pool table with status/connections/RPS/latency, per-backend detail, add/drain/enable/disable actions, real-time health check results
- **Routes page** -- Route table with match criteria, per-route metrics (RPS, p50/p95/p99 latency, error rate), route testing tool
- **Metrics page** -- Interactive time-series charts, metric explorer with search/filter, JSON/CSV export
- **Logs page** -- Real-time log stream via WebSocket, full-text search, level/route/backend/status filters, log entry detail view
- **Config page** -- Syntax-highlighted config viewer, config diff view, reload with confirmation dialog
- **Certificates page** -- Certificate inventory table, expiry warnings, ACME status display, force renewal button
- **Custom chart library** -- Line charts with smooth curves, sparklines, bar charts (vertical, stacked), gauge (semicircle), histogram, responsive canvas, hover tooltips

#### Phase 3.10: ACME Client (Let's Encrypt)
- **ACME v2 protocol** -- Full RFC 8555 implementation
- **Account registration** -- ECDSA P-256 key-based account creation
- **JWS signing** -- JSON Web Signature for authenticated ACME requests
- **Nonce management** -- Automatic replay nonce handling
- **Order creation** -- Certificate orders for single and multiple domains
- **HTTP-01 challenge solver** -- Domain validation via well-known HTTP endpoint
- **TLS-ALPN-01 challenge solver** -- Domain validation via TLS extension
- **CSR generation** -- Certificate signing request with key generation
- **Certificate download and parsing** -- PEM encoding utilities
- **Auto-renewal** -- Background goroutine, 30 days before expiry
- **Staging detection** -- Let's Encrypt staging environment support

#### Phase 3.11: OCSP Stapling
- **OCSP response caching** -- In-memory cache with configurable TTL
- **Background refresh** -- Automatic periodic refresh before expiry
- **POST and GET request modes** -- RFC 6960 compliant methods
- **Must-Staple detection** -- Check for OCSP Must-Staple extension in certificates
- **Cache statistics** -- Total, valid, expired response counts
- **Fallback support** -- Return cached response even if expired
- **TLS handshake integration** -- Staple OCSP response into TLS config

#### Phase 3.12: mTLS Support
- **Client certificate validation** -- Full chain verification with intermediates
- **CA certificate loading** -- Load from individual files or directories
- **Upstream mTLS** -- OLB-to-backend mutual TLS authentication
- **Per-listener/backend configuration** -- Flexible mTLS policies per endpoint
- **5 client auth policies** -- NoClientCert through RequireAndVerifyClientCert
- **CRL and OCSP checking** -- Certificate revocation verification
- **Certificate info extraction** -- Client cert subject, issuer, serial for logging/headers

#### Phase 3.13: `olb top` TUI Dashboard
- **Terminal UI engine** -- Raw terminal mode with ANSI escape codes
- **Box drawing** -- Unicode box characters for borders and layout
- **Progress bars and gauges** -- Visual metric display with color support
- **Table component** -- Backend and route listings with alignment
- **Keyboard input** -- `q` quit, `b` backends, `r` routes, `m` metrics, `o` overview
- **Live metrics** -- 1-second refresh cycle from admin API
- **Multiple views** -- Overview, backends, routes, metrics detail
- **Double buffering** -- Flicker-free screen updates
- **Cross-platform** -- Unix (termios) and Windows terminal support

#### Phase 3.14: Service Discovery
- **Discovery Manager** -- Multi-provider orchestration with event system
- **Static provider** -- Config-based address list or JSON file watching
- **DNS SRV provider** -- SRV record-based service discovery with TTL respect
- **DNS A/AAAA provider** -- Simple DNS resolution for service endpoints
- **Consul provider** -- HashiCorp Consul service catalog integration
- **File-based provider** -- Watch JSON/YAML files for backend changes
- **Service filtering** -- Tag, health status, and metadata-based filtering
- **Provider registry** -- Factory pattern for custom provider registration

#### Phase 3.15: Advanced CLI Commands
- **Backend commands** -- `add`, `remove`, `drain`, `enable`, `disable`, `stats`
- **Route commands** -- `add`, `remove`, `test`
- **Certificate commands** -- `list`, `add`, `remove`, `renew`, `info`
- **Metrics commands** -- `show`, `export` (Prometheus/JSON format)
- **Config commands** -- `show`, `diff`, `validate`
- **Log commands** -- `tail`, `search`
- **Shell completions** -- bash, zsh, fish auto-completion scripts

---

## [0.2.0] - 2026-03-14 — Advanced L7 + L4

### Added

#### Phase 2.1: Additional Load Balancer Algorithms
- **LeastConnections** (`lc`) -- Selects backend with fewest active connections
- **WeightedLeastConnections** (`wlc`) -- Least connections adjusted by weight
- **LeastResponseTime** (`lrt`) -- Sliding window response time tracking
- **WeightedLeastResponseTime** (`wlrt`) -- Response time adjusted by weight
- **IPHash** (`iphash`) -- FNV-1a hash of client IP for session affinity
- **ConsistentHash** (`ch`, `ketama`) -- Ketama algorithm with virtual nodes
- **Maglev** (`maglev`) -- Google Maglev consistent hashing (65537 lookup table)
- **PowerOfTwo** (`p2c`) -- Power of Two Choices random selection
- **RingHash** (`ringhash`) -- Consistent hash ring with configurable virtual nodes
- **Random** (`random`) -- Uniform random backend selection
- **WeightedRandom** (`wrandom`) -- Random with probability proportional to weight
- Total: 12 load balancing algorithms with 16 name aliases

#### Phase 2.2: Session Affinity (Sticky Sessions)
- **Sticky wrapper** -- Session affinity wrapper for any base balancer
- **Cookie-based** -- Configurable cookie name, path, max-age, secure, httponly, samesite
- **Header-based** -- `X-Backend-ID` header for session tracking
- **URL parameter** -- `?backend=xxx` query parameter support
- **Automatic fallback** -- Falls back to base balancer when session backend unavailable
- **Session cleanup** -- Automatic cleanup when backends removed

#### Phase 2.3: WebSocket Proxying
- **Upgrade detection** -- `Connection: Upgrade` + `Upgrade: websocket` header matching
- **Connection hijacking** -- Raw TCP access for bidirectional communication
- **Bidirectional frame copy** -- Concurrent goroutines for client-to-backend and backend-to-client
- **Idle timeout** -- Configurable timeout for inactive WebSocket connections
- **Ping/pong keepalive** -- WebSocket keepalive handling
- **WSS support** -- WebSocket Secure over TLS
- **Clean close handling** -- Proper WebSocket connection termination

#### Phase 2.4: gRPC Proxying
- **gRPC detection** -- Content-Type `application/grpc` and `application/grpc+proto`
- **HTTP/2 h2c support** -- HTTP/2 without TLS (cleartext) for backend connections
- **gRPC frame parsing** -- 5-byte frame header with flags and length
- **Trailer propagation** -- gRPC trailer header forwarding for status codes
- **Status code mapping** -- Bidirectional gRPC (0-16) to HTTP status conversion
- **gRPC-Web support** -- Browser-compatible gRPC-Web protocol
- **17 gRPC status codes** -- Full implementation (OK through Unauthenticated)

#### Phase 2.5: SSE Proxying
- **SSE detection** -- `Accept: text/event-stream` header matching
- **Response streaming** -- Line-by-line with immediate flush after each event
- **Event parsing** -- Full SSE format (id, event, data, retry fields)
- **Last-Event-ID support** -- Header preservation for event replay/resume
- **Idle timeout** -- Configurable timeout with keepalive comment injection

#### Phase 2.6: HTTP/2 Support
- **HTTP/2 listener** -- Full HTTP/2 server support
- **h2c support** -- HTTP/2 cleartext via upgrade mechanism
- **ALPN negotiation** -- Automatic h2/http/1.1 protocol selection via TLS
- **HTTP/2 transport** -- Backend connections with multiplexing
- **Protocol detection** -- `IsHTTP2Request()` and `IsH2CRequest()` helpers
- **Configurable streams** -- MaxConcurrentStreams and MaxFrameSize settings

#### Phase 2.7: TCP Proxy (L4)
- **TCPProxy** -- Layer 4 TCP proxy with bidirectional data copying
- **TCPListener** -- Raw TCP listener with connection acceptance
- **Zero-copy splice** -- Linux-specific `splice()` syscall for kernel-space data transfer
- **Fallback copy** -- `io.CopyBuffer` for non-Linux platforms
- **Connection limits** -- MaxConnections tracking and enforcement
- **TCP keepalive** -- Configurable keepalive with period
- **Idle timeout** -- Connection idle timeout with deadline management

#### Phase 2.8: SNI-Based Routing
- **SNI router** -- Route TCP connections based on TLS SNI hostname
- **ClientHello peeking** -- Extract SNI without consuming connection data
- **SNI parsing** -- Full TLS ClientHello parser with SNI extension extraction
- **Wildcard matching** -- `*.example.com` wildcard patterns
- **TLS passthrough** -- Route encrypted traffic without termination
- **Buffered connection** -- Peeked data preserved for backend proxying

#### Phase 2.9: PROXY Protocol
- **PROXY Protocol v1** -- Human-readable text format parser and writer
- **PROXY Protocol v2** -- Binary format parser and writer with TLV support
- **PROXYConn** -- Connection wrapper preserving original source/dest addresses
- **PROXYListener** -- Auto-detects and parses PROXY headers on accept
- **Protocol detection** -- `IsPROXYProtocol()` for v1/v2 signature matching
- **Address families** -- IPv4, IPv6, and UNIX socket support
- **Transport protocols** -- TCP (stream) and UDP (dgram) handling

#### Phase 2.10: UDP Proxy
- **UDPProxy** -- Session tracking by source address
- **UDPListener** -- UDP packet listener
- **Session management** -- Timeout and automatic cleanup

#### Phase 2.11: Circuit Breaker Middleware
- **State machine** -- Closed, Open, Half-Open states with configurable transitions
- **Error threshold** -- Configurable consecutive failure count to trip
- **Recovery** -- Half-open state allows limited requests for recovery testing
- **Per-backend** -- Independent circuit breaker per backend

#### Phase 2.12: Retry Middleware
- **Configurable retries** -- Max retry count per request
- **Exponential backoff** -- With jitter for thundering herd prevention
- **Status code filter** -- Configurable retry-on status codes
- **Idempotent-only default** -- Only retry GET, HEAD, OPTIONS, PUT, DELETE by default

#### Phase 2.13: Response Cache Middleware
- **LRU cache** -- Size-limited HTTP response caching
- **Cache key generation** -- Method + host + path + query string
- **Cache-Control respect** -- Honor no-cache, no-store, max-age, s-maxage
- **Stale-while-revalidate** -- Serve stale content while refreshing in background

#### Phase 2.14: Basic WAF Middleware
- **SQL injection detection** -- Pattern-based SQLi detection
- **XSS detection** -- Cross-site scripting pattern matching
- **Path traversal detection** -- `../` and encoded variant detection
- **Command injection detection** -- Shell command injection patterns
- **Configurable mode** -- Block or log-only for testing

#### Phase 2.15: IP Filter Middleware
- **Allow/deny lists** -- CIDR-based IP filtering
- **CIDRMatcher integration** -- Efficient radix trie IP matching
- **Per-route configuration** -- Different IP filters per route

#### Phase 2.16: Passive Health Checking
- **Error rate tracking** -- Sliding window counter from real traffic
- **Configurable threshold** -- Error rate percentage to disable backend
- **Auto-disable** -- Backend marked down on high error rate
- **Auto-recovery** -- Backend re-enabled after cooldown period

#### Phase 2.17: TOML Parser
- **TOML v1.0 lexer** -- Full tokenization of TOML syntax
- **TOML parser** -- Tables, arrays of tables, inline tables
- **All value types** -- String, integer, float, boolean, datetime, array
- **Fuzz tested** -- Crash-resistance verified

#### Phase 2.18: HCL Parser
- **HCL lexer** -- Full tokenization of HCL syntax
- **HCL parser** -- Blocks, attributes, expressions
- **String interpolation** -- `${...}` expression evaluation
- **Here-doc strings** -- Multi-line string literals
- **Fuzz tested** -- Crash-resistance verified

#### Phase 2.19: Config Hot Reload
- **Atomic config swap** -- Thread-safe configuration replacement
- **Config diff computation** -- Log what changed between old and new config
- **Full hot reload** -- Routes, backends, TLS certs, middleware all reloadable

---

## [0.1.0] - 2026-03-14 — Core L7 Proxy (MVP)

### Added

#### Core Infrastructure
- **Project bootstrap** -- Go module, directory structure, Makefile, Dockerfile, CI workflow
- **Core utilities** -- BufferPool (sync.Pool with size tiers), RingBuffer (generic lock-free), LRU cache (thread-safe with TTL), AtomicFloat64, AtomicDuration, FastRand (SplitMix64), CIDRMatcher (radix trie), BloomFilter
- **Error types** -- Sentinel errors with error codes and wrapping helpers
- **Structured logger** -- JSON/Text output, log rotation by size, max backups, SIGUSR1 reopen, child loggers with inherited fields, zero-alloc fast path
- **Config system** -- Custom YAML lexer/parser (indentation, flow collections, anchors/aliases, multi-line strings), JSON adapter, config loader with format detection, environment variable overlay (`OLB_` prefix), file watcher (polling + SHA-256), comprehensive validation
- **Metrics engine** -- Counter (atomic int64), Gauge (atomic float64), Histogram (log-linear buckets with percentiles), TimeSeries (ring buffer), CounterVec/GaugeVec/HistogramVec (labeled families), Prometheus exposition format, JSON API

#### Network & Security
- **TLS manager** -- Certificate storage with exact and wildcard matching, `GetCertificate` callback for `tls.Config`, hot reload, PEM loading
- **Connection manager** -- TrackedConn with metadata, per-source and global limits, drain with timeout, idle connection pool (channel-based), pool manager
- **Health checker** -- HTTP health checks (configurable path, method, expected status), TCP health checks (connection test), configurable consecutive OK/fail thresholds for state transitions

#### Load Balancing & Routing
- **Backend pool** -- Backend struct with atomic stats (connections, requests, errors, latency), state machine (Up/Down/Draining/Maintenance/Starting), Pool with balancer, PoolManager for multi-pool management
- **Load balancer algorithms** -- RoundRobin (atomic counter), WeightedRoundRobin (Nginx smooth weighted), balancer registry with factory pattern
- **Router** -- RadixTrie with path parameters (`:id`) and wildcards (`*path`), host-based trie selection, method and header matching, route hot-reload via atomic swap

#### Middleware Pipeline
- **Chain** -- Middleware chain builder with priority ordering
- **RequestContext** -- Per-request state carrying request, response, route, backend, metrics
- **ResponseWriter wrapper** -- Captures status code and byte count
- **8 middleware implementations:**
  - RequestID -- UUID v4 generation, X-Request-Id injection
  - RealIP -- Client IP extraction from X-Forwarded-For, X-Real-IP, RemoteAddr with trusted proxy list
  - RateLimiter -- Token bucket with per-key buckets, rate limit response headers, cleanup goroutine
  - CORS -- Preflight OPTIONS handling, Access-Control-Allow-* headers, configurable origins/methods/headers
  - Headers -- Add/remove/set request and response headers, security header presets (HSTS, X-Content-Type-Options, etc.)
  - Compression -- gzip/deflate with Accept-Encoding detection, min size threshold, content-type skip list
  - AccessLog -- JSON and CLF format, non-blocking write to logger
  - Metrics -- Per-request duration, status, bytes recording

#### L7 Proxy
- **HTTPProxy** -- Full reverse proxy with ServeHTTP handler
- **Request preparation** -- Proxy headers (X-Forwarded-For, X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host), hop-by-hop header stripping
- **Response streaming** -- Chunked transfer encoding, streaming body copy
- **Error handling** -- Backend down, timeout, connection refused with appropriate status codes

#### Operations
- **HTTP listener** -- HTTP and HTTPS listeners wrapping net/http.Server, graceful shutdown
- **Admin API** -- REST API with 12+ endpoints, Basic auth (bcrypt) and Bearer token auth
- **CLI** -- Argument parser, table/JSON formatters, 8 commands (start, stop, reload, status, version, config validate, backend list, health show)
- **Engine orchestrator** -- Central coordinator, component initialization, Start/Shutdown/Reload lifecycle, signal handling (SIGHUP, SIGTERM, SIGINT, SIGUSR1)
- **Entry point** -- cmd/olb main.go routing CLI args to commands

### Performance
- RoundRobin: 3.5 ns/op (0 allocations)
- WeightedRoundRobin: 37 ns/op (0 allocations)
- Router Match (Static): 109 ns/op (0 allocations)
- Router Match (Param): 193 ns/op (0 allocations)
- Auth Middleware: 1.5 us/op (0 allocations)
- Binary size: ~9 MB (stripped)

### Testing
- 89.7% test coverage
- 800+ test functions
- 60+ test files
- All tests passing

---

[Unreleased]: https://github.com/openloadbalancer/olb/compare/v1.0.7...HEAD
[1.0.7]: https://github.com/openloadbalancer/olb/compare/v1.0.6...v1.0.7
[1.0.6]: https://github.com/openloadbalancer/olb/compare/v1.0.5...v1.0.6
[1.0.5]: https://github.com/openloadbalancer/olb/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/openloadbalancer/olb/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/openloadbalancer/olb/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/openloadbalancer/olb/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/openloadbalancer/olb/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/openloadbalancer/olb/compare/v0.5.0...v1.0.0
[0.5.0]: https://github.com/openloadbalancer/olb/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/openloadbalancer/olb/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/openloadbalancer/olb/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/openloadbalancer/olb/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openloadbalancer/olb/releases/tag/v0.1.0
