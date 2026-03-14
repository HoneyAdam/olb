# Changelog

All notable changes to OpenLoadBalancer will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Phase 2.1: Additional Load Balancer Algorithms
- **LeastConnections** (`lc`) - Selects backend with fewest active connections
- **WeightedLeastConnections** (`wlc`) - Least connections with weight adjustment
- **LeastResponseTime** (`lrt`) - Tracks response times with sliding window
- **WeightedLeastResponseTime** (`wlrt`) - Response time with weight adjustment
- **IPHash** (`iphash`) - Session affinity using FNV-1a hash of client IP
- **ConsistentHash** (`ch`, `ketama`) - Ketama algorithm with virtual nodes
- **PowerOfTwo** (`p2c`) - Power of Two Choices random selection
- **Maglev** - Google's Maglev consistent hashing (65537 lookup table)
- **RingHash** (`ringhash`) - Consistent hash ring with virtual nodes
- **Random** - Uniform random backend selection
- **WeightedRandom** (`wrandom`) - Random with probability proportional to weight

Total: 12 load balancing algorithms with 16 name aliases

#### Phase 2.2: Session Affinity (Sticky Sessions)
- **Sticky** - Session affinity wrapper for any base balancer
- **Cookie-based** - Configurable cookie name, path, max-age, secure, httponly, samesite
- **Header-based** - `X-Backend-ID` header for session tracking
- **URL Parameter** - `?backend=xxx` parameter support
- **Automatic fallback** - Falls back to base balancer when session backend unavailable
- **Session cleanup** - Automatic cleanup when backends removed
- **Per-route configuration** - Different sticky settings per route

#### Phase 2.3: WebSocket Proxying
- **WebSocket Upgrade Detection** - Detects `Connection: Upgrade` and `Upgrade: websocket` headers
- **Connection Hijacking** - Hijacks HTTP connection for raw TCP access
- **Bidirectional Frame Copy** - Concurrent copying in both directions
- **Idle Timeout** - Configurable idle timeout for inactive connections
- **Ping/Pong Support** - Keepalive handling
- **TLS Support** - WSS (WebSocket Secure) support
- **WebSocketProxy** - Wrapper that routes WebSocket vs HTTP requests
- **Proper Close Handling** - Clean connection termination

#### Phase 2.4: gRPC Proxying
- **gRPC Detection** - Content-Type header checks (`application/grpc`, `application/grpc+proto`)
- **HTTP/2 Support** - h2c (HTTP/2 without TLS) transport configuration
- **gRPC Frame Parsing** - 5-byte frame header (flags + length) with data extraction
- **Trailer Propagation** - Proper gRPC trailer header forwarding for status codes
- **Status Code Mapping** - Bidirectional conversion between gRPC (0-16) and HTTP status codes
- **Timeout Support** - Configurable timeouts with context cancellation
- **Connection Limits** - Respects backend max connection settings
- **gRPC-Web Support** - Browser-compatible gRPC-Web protocol support
- **17 gRPC Status Codes** - Full implementation per gRPC spec (OK through Unauthenticated)

#### Phase 2.5: SSE Proxying
- **SSE Detection** - `Accept: text/event-stream` header detection
- **Response Streaming** - Line-by-line streaming with immediate flush after each event
- **Event Parsing** - Full SSE event parsing (id, event, data, retry fields)
- **Event Formatting** - SSE event serialization with proper formatting
- **Last-Event-ID Support** - Header preserved for event replay/resume
- **Idle Timeout** - Configurable timeout between events with keepalive comments
- **Connection Limits** - Respects backend max connection settings
- **Round-trip Support** - Parse and format SSE events bidirectionally

#### Phase 2.6: HTTP/2 Support
- **HTTP/2 Listener** - Full HTTP/2 server with golang.org/x/net/http2
- **h2c Support** - HTTP/2 without TLS (cleartext) via h2c upgrade
- **ALPN Negotiation** - Automatic protocol selection (h2 vs http/1.1) via TLS ALPN
- **HTTP/2 Transport** - Backend connections using HTTP/2 with multiplexing
- **Protocol Detection** - `IsHTTP2Request()` for HTTP/2 detection
- **h2c Detection** - `IsH2CRequest()` for upgrade detection (HTTP2-Settings header)
- **HTTP2Proxy** - Wrapper that routes HTTP/2 vs HTTP/1.x requests
- **Configurable Streams** - MaxConcurrentStreams and MaxFrameSize settings
- **Idle Timeout** - Connection idle timeout with PING keepalive

#### Phase 2.7: TCP Proxy (L4)
- **TCPProxy** - Layer 4 TCP proxy with bidirectional data copying
- **TCPListener** - Raw TCP listener with connection acceptance
- **Zero-Copy Splice** - Linux-specific zero-copy using splice() syscall (+build linux)
- **Fallback Copy** - io.CopyBuffer for non-Linux platforms
- **Connection Limits** - MaxConnections tracking and enforcement
- **TCP Keepalive** - Configurable TCP keepalive with period
- **Idle Timeout** - Connection idle timeout with deadline management
- **SimpleBalancer** - Round-robin balancer for backend selection
- **Connection Lifecycle** - Proper accept, dial, proxy, close handling

#### Phase 2.8: SNI-Based Routing
- **SNI Router** - Routes TCP connections based on TLS SNI hostname
- **ClientHello Peeking** - Extract SNI without consuming the connection
- **SNI Parsing** - Full TLS ClientHello parser with SNI extraction
- **Wildcard Matching** - Supports `*.example.com` wildcard patterns
- **TLS Passthrough** - Routes encrypted traffic without termination
- **Buffered Connection** - Peeked data preserved for backend proxying
- **SNIMatcher** - Pattern matching for exact and wildcard SNIs
- **TLS Record Info** - Parse TLS record headers and version info

#### Phase 2.9: PROXY Protocol
- **PROXY Protocol v1** - Human-readable text format parser and writer
- **PROXY Protocol v2** - Binary format parser and writer with TLV support
- **PROXYConn** - Connection wrapper preserving original source/dest addresses
- **PROXYListener** - Listener that auto-detects and parses PROXY headers
- **Protocol Detection** - `IsPROXYProtocol()` for v1/v2 signature detection
- **Address Family Support** - IPv4, IPv6, and UNIX socket addresses
- **Transport Support** - TCP and UDP protocol handling
- **Command Types** - PROXY and LOCAL command support
- **TLV Extensions** - Parse Type-Length-Value entries in v2 headers
- **Configurable** - Accept/reject v1/v2, allow/block LOCAL commands

#### Phase 3.1: Service Discovery
- **Discovery Manager** - Multi-provider service discovery orchestration
- **Static Provider** - Static configuration from addresses or JSON file
- **DNS Provider** - SRV record-based service discovery
- **Consul Provider** - HashiCorp Consul service catalog integration
- **Event System** - Add/Remove/Update events for service changes
- **Service Filtering** - Tag, health, and metadata-based filtering
- **Provider Registry** - Factory pattern for custom providers

#### Phase 3.2: Distributed Rate Limiting
- **Token Bucket Algorithm** - Classic token bucket with configurable rate and burst
- **In-Memory Backend** - High-performance local rate limiting
- **Multi-Zone Support** - Multiple rate limit zones with different rules
- **Per-Client Tracking** - Individual rate limit buckets per client
- **Key Functions** - Rate limit by IP, header, or cookie
- **Concurrent Safe** - Lock-free operations for high throughput
- **Automatic Cleanup** - Expired bucket cleanup to prevent memory leaks

#### Phase 3.3: Web Application Firewall (WAF)
- **Security Rules** - Pre-configured rules for SQL injection, XSS, path traversal
- **Rule Engine** - Regex-based pattern matching with multiple targets
- **Actions** - Block, log, or challenge mode per rule
- **Severity Levels** - Critical, high, medium, low severity classification
- **Anomaly Scoring** - Cumulative scoring for multi-rule detection
- **Detection Mode** - Monitor without blocking for testing
- **JSON Logging** - Structured logging of security events
- **Custom Rules** - Add/remove rules at runtime

#### Phase 3.4: Clustering & Raft Consensus
- **Raft State Machine** - Follower, Candidate, Leader states
- **Election Process** - Randomized timeouts, majority voting
- **Log Replication** - AppendEntries with log consistency checks
- **Term Management** - Atomic term tracking with vote persistence
- **RequestVote RPC** - Vote requests with log up-to-date verification
- **AppendEntries RPC** - Heartbeat and log entry replication
- **Callbacks** - OnStateChange and OnLeaderElected hooks
- **State Machine Interface** - Pluggable state machine for applying commands

#### Phase 3.10: ACME Client (Let's Encrypt)
- **ACME v2 Protocol** - Full RFC 8555 implementation
- **Account Registration** - ECDSA P-256 key-based account setup
- **JWS Signing** - JSON Web Signature for authenticated requests
- **Order Creation** - Certificate orders for multiple domains
- **HTTP-01 Challenge** - Domain validation via HTTP challenge
- **Certificate Issuance** - CSR submission and certificate download
- **PEM Encoding** - Certificate and key encoding utilities
- **Staging Detection** - Let's Encrypt staging environment support

#### Phase 3.11: OCSP Stapling
- **OCSP Response Caching** - In-memory cache with configurable TTL
- **Background Refresh** - Automatic periodic refresh of OCSP responses
- **POST and GET Support** - RFC 6960 compliant request methods
- **Must-Staple Detection** - Check for OCSP Must-Staple extension
- **Cache Statistics** - Total, valid, and expired response counts
- **Fallback Support** - Return cached response even if expired
- **PEM Encoding** - OCSP request/response PEM utilities

#### Phase 3.12: mTLS Support
- **Client Certificate Validation** - Full chain verification with intermediates
- **CA Certificate Loading** - Load from files or directories
- **Upstream mTLS** - OLB → backend mutual TLS authentication
- **Per-Listener/Backend Config** - Flexible mTLS policies per endpoint
- **Client Auth Policies** - 5 levels from optional to required+verified
- **CRL and OCSP Support** - Certificate revocation checking
- **Certificate Info Extraction** - Client cert details for logging

#### Phase 3.13: `olb top` TUI Dashboard
- **Terminal UI Engine** - Raw terminal mode with ANSI escape codes
- **Box Drawing** - Unicode box characters for borders
- **Progress Bars/Gauges** - Visual metrics with color support
- **Table Component** - Backend and route listings
- **Keyboard Input** - q/b/r/m/o shortcuts for navigation
- **Live Metrics** - 1-second refresh from admin API
- **Multiple Views** - Overview, backends, routes, metrics
- **Double Buffering** - Efficient flicker-free screen updates
- **Cross-Platform** - Unix (termios) and Windows support

#### Phase 3.15: Advanced CLI Commands
- **Backend Commands** - add, remove, drain, enable, disable, stats
- **Route Commands** - add, remove, test
- **Certificate Commands** - list, add, remove, renew, info
- **Metrics Commands** - show, export
- **Config Commands** - show, diff, validate
- **Shell Completions** - bash, zsh, fish support

#### Phase 3.1-3.9: Web UI
- **SPA Framework** - Vanilla JS router, state management, components
- **CSS Design System** - Dark/light theme, CSS variables, responsive grid
- **Dashboard Page** - Live metrics, sparklines, gauges, backend health
- **Backends Page** - Pool management, health checks, statistics
- **Routes Page** - Route table, testing tool, metrics
- **Metrics Page** - Time-series charts, explorer, export
- **Logs Page** - Real-time stream, search, filters
- **Config Page** - YAML viewer, diff, validation
- **Certificates Page** - Inventory, expiry warnings, ACME status
- **WebSocket** - Real-time updates for all pages

## [0.1.0] - 2026-03-14

### Added - Phase 1: Core L7 Proxy MVP

#### Core Infrastructure
- **Project Bootstrap** - Go module, directory structure, Makefile, Dockerfile
- **Core Utilities** - BufferPool, RingBuffer, LRU cache, Atomic helpers, CIDRMatcher, BloomFilter
- **Error Types** - Sentinel errors with error wrapping and codes
- **Structured Logger** - JSON/Text output, log rotation, platform signals
- **Config System** - YAML lexer/parser with environment variable expansion
- **Metrics Engine** - Counter, Gauge, Histogram with Prometheus/JSON export

#### Network & Security
- **TLS Manager** - Certificate storage with SNI and wildcard matching
- **Connection Manager** - TrackedConn, connection limits, pooling
- **Health Checker** - HTTP/TCP health checks with state transitions

#### Load Balancing
- **Backend Pool** - Backend management with state machine and atomic stats
- **Load Balancer Algorithms** - RoundRobin and WeightedRoundRobin
- **Router** - RadixTrie-based HTTP routing with path parameters

#### Middleware & Proxy
- **Middleware Pipeline** - Chain with 8 middleware implementations:
  - RequestID (UUID generation)
  - RealIP (client IP extraction)
  - RateLimiter (token bucket)
  - CORS
  - Headers (security presets)
  - Compression (gzip/deflate)
  - AccessLog
  - Metrics
- **L7 HTTP Proxy** - Reverse proxy with streaming and retries

#### Operations
- **HTTP Listener** - HTTP/HTTPS with graceful shutdown
- **Admin API** - REST API with authentication (12 endpoints)
- **CLI** - 8 commands (start, stop, reload, status, version, config, backend, health)
- **Engine Orchestrator** - Central coordinator with hot reload

### Performance
- RoundRobin: 3.5 ns/op (0 allocations)
- WeightedRoundRobin: 37 ns/op (0 allocations)
- Router Match (Static): 109 ns/op
- Router Match (Param): 193 ns/op
- Binary Size: ~9MB

### Testing
- 89.7% test coverage
- 800+ test functions
- 60+ test files
- All tests passing

[Unreleased]: https://github.com/openloadbalancer/olb/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/openloadbalancer/olb/releases/tag/v0.1.0
