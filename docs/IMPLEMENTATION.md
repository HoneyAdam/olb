# OpenLoadBalancer — IMPLEMENTATION GUIDE v1.0

> **Reference**: SPECIFICATION.md v1.0
> **Language**: Go 1.23+ (stdlib only, `golang.org/x/net` and `golang.org/x/crypto` allowed)
> **Build**: `CGO_ENABLED=0` for full static binary
> **Date**: 2026-03-13

---

## Table of Contents

1. [Development Setup & Conventions](#1-development-setup--conventions)
2. [Core Primitives & Utilities](#2-core-primitives--utilities)
3. [Config Parser Implementation](#3-config-parser-implementation)
4. [Structured Logger](#4-structured-logger)
5. [Metrics Engine](#5-metrics-engine)
6. [TLS Engine](#6-tls-engine)
7. [Connection Manager](#7-connection-manager)
8. [L7 HTTP Proxy](#8-l7-http-proxy)
9. [L4 TCP/UDP Proxy](#9-l4-tcpudp-proxy)
10. [Router Implementation](#10-router-implementation)
11. [Load Balancer Algorithms](#11-load-balancer-algorithms)
12. [Backend Pool & State Machine](#12-backend-pool--state-machine)
13. [Health Checker](#13-health-checker)
14. [Middleware Pipeline](#14-middleware-pipeline)
15. [Admin API Server](#15-admin-api-server)
16. [Web UI Implementation](#16-web-ui-implementation)
17. [CLI Implementation](#17-cli-implementation)
18. [Cluster: Gossip Protocol](#18-cluster-gossip-protocol)
19. [Cluster: Raft Consensus](#19-cluster-raft-consensus)
20. [MCP Server](#20-mcp-server)
21. [Plugin System](#21-plugin-system)
22. [Engine Orchestrator](#22-engine-orchestrator)
23. [Build, Test & Release](#23-build-test--release)
24. [Performance Optimization Playbook](#24-performance-optimization-playbook)

---

## 1. Development Setup & Conventions

### 1.1 Go Module

```bash
mkdir openloadbalancer && cd openloadbalancer
go mod init github.com/openloadbalancer/olb
```

`go.mod` will have at most:
```
module github.com/openloadbalancer/olb

go 1.23

require (
    golang.org/x/net v0.30.0    // HTTP/2, QUIC (optional)
    golang.org/x/crypto v0.29.0 // ACME, OCSP, bcrypt
)
```

### 1.2 Coding Conventions

```go
// Package-level documentation: every package starts with doc.go
// File naming: snake_case.go
// Interfaces: defined in the package that USES them (not the implementor)
// Errors: sentinel errors + wrapped errors with context
// Context: first parameter of every public function
// Options: functional options pattern for complex constructors

// Error pattern
var (
    ErrBackendNotFound   = errors.New("olb: backend not found")
    ErrPoolNotFound      = errors.New("olb: pool not found")
    ErrRouteNotFound     = errors.New("olb: route not found")
    ErrConfigInvalid     = errors.New("olb: config invalid")
    ErrConnectionRefused = errors.New("olb: connection refused")
)

// Wrap errors with context
func (p *Pool) GetBackend(id string) (*Backend, error) {
    b, ok := p.backends[id]
    if !ok {
        return nil, fmt.Errorf("%w: %s in pool %s", ErrBackendNotFound, id, p.name)
    }
    return b, nil
}
```

### 1.3 Directory Bootstrap

```bash
# Create all directories
dirs=(
    cmd/olb
    internal/{engine,listener,conn,proxy/l7,proxy/l4,router,balancer,backend}
    internal/{health,middleware/{ratelimit,circuit,retry,timeout,compress}}
    internal/{middleware/{cors,auth,headers,rewrite,cache,waf,ipfilter,logging,metrics}}
    internal/{tls/acme,config/parser,discovery,metrics,logging}
    internal/{admin/handlers,webui/assets/{components,lib},cli/{commands,tui}}
    internal/{cluster/{raft,gossip,state},mcp,plugin}
    pkg/{types,errors,version,utils}
    configs docs test/{integration,e2e,benchmark,fixtures}
    scripts web/src
)
for d in "${dirs[@]}"; do mkdir -p "$d"; done
```

### 1.4 Code Generation & Build

```makefile
# Makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
    -X github.com/openloadbalancer/olb/pkg/version.Version=$(VERSION) \
    -X github.com/openloadbalancer/olb/pkg/version.Commit=$(COMMIT) \
    -X github.com/openloadbalancer/olb/pkg/version.Date=$(DATE)

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/olb ./cmd/olb

.PHONY: build-all
build-all:
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/olb-linux-amd64 ./cmd/olb
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/olb-linux-arm64 ./cmd/olb
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/olb-darwin-amd64 ./cmd/olb
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/olb-darwin-arm64 ./cmd/olb
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/olb-windows-amd64.exe ./cmd/olb

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: bench
bench:
	go test -bench=. -benchmem ./...

.PHONY: lint
lint:
	go vet ./...
	staticcheck ./...

.PHONY: fuzz
fuzz:
	go test -fuzz=FuzzYAMLParser -fuzztime=30s ./internal/config/parser/
	go test -fuzz=FuzzTOMLParser -fuzztime=30s ./internal/config/parser/
```

---

## 2. Core Primitives & Utilities

### 2.1 Lock-Free Ring Buffer

Used for metrics time series, log buffering, and connection tracking.

```go
// pkg/utils/ring.go

// RingBuffer is a lock-free, fixed-size circular buffer.
// Uses atomic operations for single-producer/single-consumer (SPSC) pattern.
// For multi-producer: use a mutex or per-goroutine buffers that merge.
type RingBuffer[T any] struct {
    items []T
    mask  uint64 // size - 1 (size must be power of 2)
    head  atomic.Uint64
    tail  atomic.Uint64
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
    // Round up to power of 2
    n := uint64(1)
    for n < uint64(size) {
        n <<= 1
    }
    return &RingBuffer[T]{
        items: make([]T, n),
        mask:  n - 1,
    }
}

func (r *RingBuffer[T]) Push(item T) bool {
    head := r.head.Load()
    tail := r.tail.Load()
    if head-tail >= r.mask+1 {
        return false // full
    }
    r.items[head&r.mask] = item
    r.head.Add(1)
    return true
}

func (r *RingBuffer[T]) Pop() (T, bool) {
    tail := r.tail.Load()
    head := r.head.Load()
    if tail >= head {
        var zero T
        return zero, false // empty
    }
    item := r.items[tail&r.mask]
    r.tail.Add(1)
    return item, true
}

// Snapshot returns a copy of all items for iteration
func (r *RingBuffer[T]) Snapshot() []T {
    tail := r.tail.Load()
    head := r.head.Load()
    n := head - tail
    result := make([]T, 0, n)
    for i := tail; i < head; i++ {
        result = append(result, r.items[i&r.mask])
    }
    return result
}
```

### 2.2 LRU Cache

Used for response caching, DNS caching, and session tables.

```go
// pkg/utils/lru.go

// LRU is a thread-safe Least Recently Used cache.
// Uses a doubly-linked list + hash map for O(1) get/put/eviction.
type LRU[K comparable, V any] struct {
    capacity int
    mu       sync.RWMutex
    items    map[K]*lruNode[K, V]
    head     *lruNode[K, V] // most recent
    tail     *lruNode[K, V] // least recent
    size     int
    
    // Callbacks
    onEvict func(K, V)
}

type lruNode[K comparable, V any] struct {
    key   K
    value V
    prev  *lruNode[K, V]
    next  *lruNode[K, V]
    
    // For TTL support
    expiresAt time.Time
}

// Implementation approach:
// - Get: O(1) lookup in map, move to front of list
// - Put: O(1) insert at front, evict from tail if over capacity
// - Delete: O(1) remove from map and list
// - Expired items are cleaned lazily on access + periodic sweep goroutine
```

### 2.3 Buffer Pool

Critical for zero-allocation proxying.

```go
// pkg/utils/buffer.go

// BufferPool manages reusable byte buffers using sync.Pool.
// Multiple pools for different sizes to avoid waste.
type BufferPool struct {
    pools []*sync.Pool
    sizes []int
}

// Standard pool sizes: 1KB, 4KB, 8KB, 16KB, 32KB, 64KB
func NewBufferPool() *BufferPool {
    sizes := []int{1024, 4096, 8192, 16384, 32768, 65536}
    bp := &BufferPool{
        pools: make([]*sync.Pool, len(sizes)),
        sizes: sizes,
    }
    for i, size := range sizes {
        s := size // capture
        bp.pools[i] = &sync.Pool{
            New: func() interface{} {
                buf := make([]byte, s)
                return &buf
            },
        }
    }
    return bp
}

// Get returns a buffer of at least the requested size.
// Always returns a pointer to a slice for proper pool return.
func (bp *BufferPool) Get(size int) *[]byte {
    for i, s := range bp.sizes {
        if s >= size {
            return bp.pools[i].Get().(*[]byte)
        }
    }
    // Requested size exceeds all pools — allocate directly
    buf := make([]byte, size)
    return &buf
}

// Put returns a buffer to the pool.
func (bp *BufferPool) Put(buf *[]byte) {
    size := cap(*buf)
    for i, s := range bp.sizes {
        if s == size {
            bp.pools[i].Put(buf)
            return
        }
    }
    // Non-standard size, let GC handle it
}
```

### 2.4 Atomic Helpers

```go
// pkg/utils/atomic.go

// AtomicFloat64 provides atomic operations on float64.
// Uses math.Float64bits/Float64frombits for atomic storage.
type AtomicFloat64 struct {
    bits atomic.Uint64
}

func (f *AtomicFloat64) Load() float64 {
    return math.Float64frombits(f.bits.Load())
}

func (f *AtomicFloat64) Store(val float64) {
    f.bits.Store(math.Float64bits(val))
}

func (f *AtomicFloat64) Add(delta float64) float64 {
    for {
        old := f.bits.Load()
        newVal := math.Float64frombits(old) + delta
        if f.bits.CompareAndSwap(old, math.Float64bits(newVal)) {
            return newVal
        }
    }
}

// AtomicDuration wraps atomic.Int64 for time.Duration.
type AtomicDuration struct {
    ns atomic.Int64
}

func (d *AtomicDuration) Load() time.Duration {
    return time.Duration(d.ns.Load())
}

func (d *AtomicDuration) Store(dur time.Duration) {
    d.ns.Store(int64(dur))
}
```

### 2.5 Fast Random (SplitMix64)

For load balancer algorithms that need fast random without crypto overhead.

```go
// pkg/utils/rand.go

// FastRand is a per-goroutine fast pseudo-random number generator.
// Uses SplitMix64 — much faster than math/rand for non-crypto uses.
type FastRand struct {
    state uint64
}

func NewFastRand() *FastRand {
    // Seed from runtime.fastrand or time
    seed := uint64(time.Now().UnixNano())
    return &FastRand{state: seed}
}

func (r *FastRand) Uint64() uint64 {
    r.state += 0x9e3779b97f4a7c15
    z := r.state
    z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
    z = (z ^ (z >> 27)) * 0x94d049bb133111eb
    return z ^ (z >> 31)
}

func (r *FastRand) Intn(n int) int {
    return int(r.Uint64() % uint64(n))
}
```

### 2.6 IP/CIDR Utilities

```go
// pkg/utils/ip.go

// CIDRMatcher provides efficient IP matching against CIDR ranges.
// Uses a radix trie for O(32) or O(128) lookup (IPv4/IPv6).
type CIDRMatcher struct {
    root4 *cidrNode // IPv4 trie
    root6 *cidrNode // IPv6 trie
}

type cidrNode struct {
    children [2]*cidrNode // 0 and 1 bit branches
    isLeaf   bool
    data     interface{} // associated data (allow/deny action, etc.)
}

// Add inserts a CIDR range into the trie.
func (m *CIDRMatcher) Add(cidr string, data interface{}) error {
    _, network, err := net.ParseCIDR(cidr)
    if err != nil {
        return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
    }
    // Walk bits of network address up to prefix length
    // Insert into appropriate trie (v4 or v6)
    // ...
    return nil
}

// Match checks if an IP matches any added CIDR range.
// Returns the associated data and whether a match was found.
func (m *CIDRMatcher) Match(ip net.IP) (interface{}, bool) {
    // Walk the trie bit by bit
    // Return first matching prefix (longest prefix match)
    // ...
    return nil, false
}
```

### 2.7 Bloom Filter

For WAF pattern pre-screening (fast negative lookups).

```go
// pkg/utils/bloom.go

// BloomFilter provides probabilistic set membership testing.
// Uses k hash functions (double hashing technique) over m bits.
type BloomFilter struct {
    bits    []uint64
    m       uint64 // total bits
    k       uint   // number of hash functions
    count   uint64
}

// NewBloomFilter creates a filter for n expected items with fp false positive rate.
func NewBloomFilter(n uint, fp float64) *BloomFilter {
    m := optimalM(n, fp)
    k := optimalK(m, n)
    return &BloomFilter{
        bits: make([]uint64, (m+63)/64),
        m:    uint64(m),
        k:    k,
    }
}

// Add inserts an item.
func (bf *BloomFilter) Add(data []byte) {
    h1, h2 := bf.hash(data)
    for i := uint(0); i < bf.k; i++ {
        pos := (uint64(h1) + uint64(i)*uint64(h2)) % bf.m
        bf.bits[pos/64] |= 1 << (pos % 64)
    }
    bf.count++
}

// Contains tests if an item might be in the set.
// False positives possible, false negatives impossible.
func (bf *BloomFilter) Contains(data []byte) bool {
    h1, h2 := bf.hash(data)
    for i := uint(0); i < bf.k; i++ {
        pos := (uint64(h1) + uint64(i)*uint64(h2)) % bf.m
        if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
            return false
        }
    }
    return true
}

// hash uses FNV-1a split into two 32-bit hashes for double hashing.
func (bf *BloomFilter) hash(data []byte) (uint32, uint32) {
    h := fnv.New64a()
    h.Write(data)
    sum := h.Sum64()
    return uint32(sum), uint32(sum >> 32)
}
```

---

## 3. Config Parser Implementation

### 3.1 YAML Parser (From Scratch)

This is one of the most complex components. We implement a subset of YAML 1.2 sufficient for config files.

```go
// internal/config/parser/yaml.go

// Architecture:
// 1. Lexer (Scanner) → produces tokens
// 2. Parser → consumes tokens, produces AST
// 3. Decoder → converts AST to Go structs

// Token types
type TokenType int

const (
    TokenEOF TokenType = iota
    TokenNewline
    TokenIndent
    TokenDedent
    TokenKey          // "key" before ':'
    TokenColon        // ':'
    TokenValue        // scalar value
    TokenDash         // '-' (sequence item)
    TokenPipe         // '|' (literal block)
    TokenGT           // '>' (folded block)
    TokenAnchor       // '&anchor'
    TokenAlias        // '*alias'
    TokenFlowMapStart // '{'
    TokenFlowMapEnd   // '}'
    TokenFlowSeqStart // '['
    TokenFlowSeqEnd   // ']'
    TokenComma        // ','
    TokenComment      // '# comment'
    TokenTag          // '!!str', '!!int', etc.
    TokenDocStart     // '---'
    TokenDocEnd       // '...'
    TokenQuotedString // 'single' or "double"
)

type Token struct {
    Type    TokenType
    Value   string
    Line    int
    Column  int
    Indent  int
}

// Lexer implementation approach:
// 1. Track indentation levels with a stack
// 2. Emit INDENT/DEDENT tokens on indentation changes
// 3. Handle multi-line strings (| and >)
// 4. Handle quoted strings with escape sequences
// 5. Handle flow collections ({ } [ ])
// 6. Handle anchors (&) and aliases (*)

type YAMLLexer struct {
    input       []byte
    pos         int
    line        int
    col         int
    indentStack []int
    tokens      []Token
}

func (l *YAMLLexer) Scan() []Token {
    l.indentStack = []int{0}
    for l.pos < len(l.input) {
        l.scanLine()
    }
    // Emit remaining DEDENTs
    for len(l.indentStack) > 1 {
        l.emit(TokenDedent, "")
        l.indentStack = l.indentStack[:len(l.indentStack)-1]
    }
    l.emit(TokenEOF, "")
    return l.tokens
}

// Key scanning rules:
// - Unquoted key: everything before ': ' (colon + space)
// - Quoted key: 'key' or "key" before ':'
// - Keys can contain dots, hyphens, underscores, alphanumeric

// Value scanning rules:
// - Unquoted: everything after ': ' until newline or comment
// - Auto-type: "true"/"false" → bool, numbers → int/float, "null"/"~" → nil
// - Quoted: preserve as string regardless of content
// - Multi-line literal (|): preserve newlines
// - Multi-line folded (>): replace newlines with spaces

// AST node types
type YAMLNode struct {
    Kind     YAMLNodeKind
    Value    string          // for scalars
    Tag      string          // explicit tag
    Anchor   string          // anchor name
    Children []*YAMLNode     // for sequences
    Mapping  []*YAMLMapping  // for mappings (ordered)
    Line     int
}

type YAMLNodeKind int

const (
    YAMLScalar YAMLNodeKind = iota
    YAMLMapping
    YAMLSequence
    YAMLNull
)

type YAMLMapping struct {
    Key   *YAMLNode
    Value *YAMLNode
}

// Parser: recursive descent
type YAMLParser struct {
    tokens  []Token
    pos     int
    anchors map[string]*YAMLNode
}

func (p *YAMLParser) Parse() (*YAMLNode, error) {
    // Entry point: parse document
    // Handle --- document start marker
    // Parse root node (typically a mapping)
    return p.parseNode()
}

func (p *YAMLParser) parseNode() (*YAMLNode, error) {
    tok := p.peek()
    switch {
    case tok.Type == TokenDash:
        return p.parseSequence()
    case tok.Type == TokenKey:
        return p.parseMapping()
    case tok.Type == TokenPipe || tok.Type == TokenGT:
        return p.parseBlockScalar()
    case tok.Type == TokenFlowMapStart:
        return p.parseFlowMapping()
    case tok.Type == TokenFlowSeqStart:
        return p.parseFlowSequence()
    case tok.Type == TokenAlias:
        return p.resolveAlias()
    default:
        return p.parseScalar()
    }
}

// Decoder: uses reflection to map AST → Go structs
// Similar to encoding/json's approach:
// 1. Walk AST and target struct fields simultaneously
// 2. Match YAML keys to struct field names (case-insensitive) or `yaml:"name"` tags
// 3. Handle type conversions (string→int, string→duration, string→bool)
// 4. Handle nested structs, slices, maps
// 5. Handle env var substitution: ${ENV_VAR} and ${ENV_VAR:-default}

type YAMLDecoder struct {
    root *YAMLNode
}

func (d *YAMLDecoder) Decode(v interface{}) error {
    rv := reflect.ValueOf(v)
    if rv.Kind() != reflect.Ptr || rv.IsNil() {
        return fmt.Errorf("yaml: decode requires non-nil pointer")
    }
    return d.decodeNode(d.root, rv.Elem())
}

func (d *YAMLDecoder) decodeNode(node *YAMLNode, rv reflect.Value) error {
    // Handle pointer types
    // Handle interface{} (store raw)
    // Handle map types
    // Handle slice types
    // Handle struct types (field matching)
    // Handle scalar types (string, int, float, bool, time.Duration)
    // Handle custom UnmarshalYAML interface
    return nil
}

// Special handling for time.Duration:
// Accept: "5s", "100ms", "1m30s", "1h", "30m"
// Reject: plain numbers without unit

// Special handling for byte sizes:
// Accept: "100MB", "1GB", "512KB"
// Convert to int64 bytes
```

### 3.2 TOML Parser (From Scratch)

```go
// internal/config/parser/toml.go

// TOML is simpler than YAML — no indentation sensitivity.
// Key structures:
// - Key/value pairs: key = value
// - Tables: [table.name]
// - Array of tables: [[array.name]]
// - Inline tables: key = { a = 1, b = 2 }

// Lexer tokens
// TokenBareKey, TokenQuotedKey, TokenEquals, TokenString,
// TokenInteger, TokenFloat, TokenBool, TokenDatetime,
// TokenTableStart ([), TokenTableEnd (]),
// TokenArrayTableStart ([[), TokenArrayTableEnd (]]),
// TokenInlineTableStart ({), TokenInlineTableEnd (}),
// TokenArrayStart ([), TokenArrayEnd (]),
// TokenComma, TokenDot, TokenNewline, TokenComment

// Parser approach:
// 1. Line-oriented parsing (TOML is line-based)
// 2. Track current table context (dotted path)
// 3. Build nested map[string]interface{} tree
// 4. Decode to Go structs using reflection (same decoder as YAML)

// Key rules:
// - Bare keys: [A-Za-z0-9_-]+
// - Quoted keys: "key with spaces" or 'literal key'
// - Dotted keys: table.subtable.key

// Value types:
// - String: "basic\n" or 'literal' or """multi-line""" or '''multi-line'''
// - Integer: 42, 0xFF, 0o755, 0b1010 (with _ separators)
// - Float: 3.14, inf, nan
// - Bool: true, false
// - Datetime: 2024-01-01T00:00:00Z
// - Array: [1, 2, 3]
// - Inline table: {key = "value"}

type TOMLParser struct {
    input    []byte
    pos      int
    line     int
    root     map[string]interface{}
    current  map[string]interface{} // current table context
    path     []string               // current dotted path
}

func (p *TOMLParser) Parse() (map[string]interface{}, error) {
    p.root = make(map[string]interface{})
    p.current = p.root
    
    for p.pos < len(p.input) {
        p.skipWhitespace()
        if p.pos >= len(p.input) {
            break
        }
        
        switch {
        case p.input[p.pos] == '#':
            p.skipComment()
        case p.input[p.pos] == '[':
            if p.pos+1 < len(p.input) && p.input[p.pos+1] == '[' {
                p.parseArrayTable()
            } else {
                p.parseTable()
            }
        case p.input[p.pos] == '\n' || p.input[p.pos] == '\r':
            p.advance()
        default:
            p.parseKeyValue()
        }
    }
    
    return p.root, nil
}

// parseTable handles [table.name]
// 1. Parse dotted key between [ and ]
// 2. Navigate/create nested maps
// 3. Set current context

// parseArrayTable handles [[array.name]]
// 1. Parse dotted key between [[ and ]]
// 2. Navigate to parent, append new map to array

// parseKeyValue handles key = value
// 1. Parse key (bare, quoted, or dotted)
// 2. Expect '='
// 3. Parse value (detect type from first character)
```

### 3.3 HCL Parser (From Scratch)

```go
// internal/config/parser/hcl.go

// HCL is block-oriented with terraform-like syntax:
//
// block_type "label1" "label2" {
//   attribute = value
//   nested_block {
//     ...
//   }
// }

// HCL grammar (simplified):
// file      = (block | attribute | comment)*
// block     = IDENT (STRING)* "{" body "}"
// body      = (block | attribute | comment)*
// attribute = IDENT "=" expression
// expression = STRING | NUMBER | BOOL | list | map | reference | heredoc
// list      = "[" (expression ("," expression)*)? "]"
// map       = "{" (IDENT "=" expression)* "}"
// reference = IDENT ("." IDENT)*
// heredoc   = "<<" IDENT "\n" ... "\n" IDENT
// comment   = "//" ... "\n" | "/*" ... "*/"

// HCL AST
type HCLFile struct {
    Blocks     []*HCLBlock
    Attributes []*HCLAttribute
}

type HCLBlock struct {
    Type       string
    Labels     []string
    Attributes []*HCLAttribute
    Blocks     []*HCLBlock
}

type HCLAttribute struct {
    Name  string
    Value HCLValue
}

type HCLValue interface {
    hclValue()
}

type HCLString struct{ Value string }
type HCLNumber struct{ Value float64 }
type HCLBool struct{ Value bool }
type HCLList struct{ Items []HCLValue }
type HCLMap struct{ Items map[string]HCLValue }
type HCLReference struct{ Path []string }

// String interpolation: "${var.name}"
// Parsed by scanning for ${ and matching }
// References resolved against the file scope

// Conversion to config:
// HCL blocks map to YAML-style config via naming convention:
//   listener "http" { address = ":80" }
//   → listeners: [{ name: "http", address: ":80" }]
//
//   backend "app" { server { address = "..." } }
//   → backends: [{ name: "app", backends: [{ address: "..." }] }]
```

### 3.4 Config Loader & Merger

```go
// internal/config/loader.go

type Loader struct {
    parsers map[string]Parser // ".yaml" → YAMLParser, ".toml" → TOMLParser, etc.
}

func (l *Loader) Load(path string) (*Config, error) {
    // 1. Detect format from extension
    ext := filepath.Ext(path)
    parser, ok := l.parsers[ext]
    if !ok {
        return nil, fmt.Errorf("unsupported config format: %s", ext)
    }
    
    // 2. Read file
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }
    
    // 3. Parse to generic map
    raw, err := parser.Parse(data)
    if err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    
    // 4. Apply environment variable overlay
    raw = l.applyEnvOverlay(raw)
    
    // 5. Decode to Config struct
    cfg := DefaultConfig()
    if err := decode(raw, cfg); err != nil {
        return nil, fmt.Errorf("decode config: %w", err)
    }
    
    // 6. Validate
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }
    
    return cfg, nil
}

// Environment overlay:
// OLB_GLOBAL__LOG__LEVEL=debug → config.Global.Log.Level = "debug"
func (l *Loader) applyEnvOverlay(raw map[string]interface{}) map[string]interface{} {
    for _, env := range os.Environ() {
        if !strings.HasPrefix(env, "OLB_") {
            continue
        }
        parts := strings.SplitN(env, "=", 2)
        key := strings.TrimPrefix(parts[0], "OLB_")
        value := parts[1]
        
        // Convert OLB_GLOBAL__LOG__LEVEL → ["global", "log", "level"]
        path := strings.Split(strings.ToLower(key), "__")
        setNestedValue(raw, path, value)
    }
    return raw
}
```

### 3.5 Config Watcher (Hot Reload)

```go
// internal/config/watcher.go

// FileWatcher monitors config file for changes using polling.
// We don't use inotify/kqueue directly because:
// 1. It adds OS-specific code complexity
// 2. Some editors (vim) create new files on save, breaking inotify watches
// 3. Polling at 1-2s interval is sufficient for config files

type FileWatcher struct {
    path     string
    interval time.Duration
    lastMod  time.Time
    lastHash [32]byte // SHA-256 of file content
    onChange func()
    ctx      context.Context
    cancel   context.CancelFunc
}

func (w *FileWatcher) Start() {
    w.ctx, w.cancel = context.WithCancel(context.Background())
    go w.poll()
}

func (w *FileWatcher) poll() {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-w.ctx.Done():
            return
        case <-ticker.C:
            info, err := os.Stat(w.path)
            if err != nil {
                continue
            }
            // Check modification time first (cheap)
            if info.ModTime().Equal(w.lastMod) {
                continue
            }
            // Verify with content hash (avoid false positives)
            data, err := os.ReadFile(w.path)
            if err != nil {
                continue
            }
            hash := sha256.Sum256(data)
            if hash == w.lastHash {
                w.lastMod = info.ModTime()
                continue
            }
            // File genuinely changed
            w.lastMod = info.ModTime()
            w.lastHash = hash
            w.onChange()
        }
    }
}
```

---

## 4. Structured Logger

### 4.1 Logger Design

```go
// internal/logging/logger.go

// Logger is a structured, leveled logger with zero allocations on disabled levels.
// Design:
// - Level check is a simple atomic int comparison (no allocation if disabled)
// - Fields are stored in a pre-allocated slice
// - Output is buffered and flushed periodically or on high-severity
// - Multiple outputs supported simultaneously (stdout + file)

type Level int32

const (
    LevelTrace Level = iota
    LevelDebug
    LevelInfo
    LevelWarn
    LevelError
    LevelFatal
)

type Logger struct {
    level    atomic.Int32
    outputs  []Output
    fields   []Field // inherited fields (e.g., component name)
    bufPool  sync.Pool
}

type Field struct {
    Key   string
    Value interface{}
}

type Output interface {
    Write(entry *Entry) error
    Flush() error
    Close() error
}

type Entry struct {
    Time      time.Time
    Level     Level
    Message   string
    Fields    []Field
    Caller    string // file:line (optional, debug only)
}

// Zero-alloc fast path for disabled levels:
func (l *Logger) Debug(msg string, fields ...Field) {
    if Level(l.level.Load()) > LevelDebug {
        return // no allocation
    }
    l.log(LevelDebug, msg, fields)
}

func (l *Logger) Info(msg string, fields ...Field) {
    if Level(l.level.Load()) > LevelInfo {
        return
    }
    l.log(LevelInfo, msg, fields)
}

// Child logger with inherited fields:
func (l *Logger) With(fields ...Field) *Logger {
    child := &Logger{
        outputs: l.outputs,
        fields:  make([]Field, 0, len(l.fields)+len(fields)),
    }
    child.level.Store(l.level.Load())
    child.fields = append(child.fields, l.fields...)
    child.fields = append(child.fields, fields...)
    return child
}
```

### 4.2 JSON Output

```go
// internal/logging/output.go

type JSONOutput struct {
    writer  *bufio.Writer
    mu      sync.Mutex
    buf     []byte // reusable buffer
}

// Write encodes an entry as JSON without encoding/json (for performance).
// Manual JSON building is 3-5x faster than encoding/json for structured logs.
func (o *JSONOutput) Write(entry *Entry) error {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    o.buf = o.buf[:0]
    o.buf = append(o.buf, '{')
    
    // Timestamp: "ts":"2024-01-01T00:00:00.000Z"
    o.buf = append(o.buf, `"ts":"`...)
    o.buf = entry.Time.AppendFormat(o.buf, time.RFC3339Nano)
    o.buf = append(o.buf, `",`...)
    
    // Level: "level":"info"
    o.buf = append(o.buf, `"level":"`...)
    o.buf = append(o.buf, levelString(entry.Level)...)
    o.buf = append(o.buf, `",`...)
    
    // Message: "msg":"..."
    o.buf = append(o.buf, `"msg":`...)
    o.buf = appendJSONString(o.buf, entry.Message)
    
    // Fields
    for _, f := range entry.Fields {
        o.buf = append(o.buf, ',')
        o.buf = appendJSONString(o.buf, f.Key)
        o.buf = append(o.buf, ':')
        o.buf = appendJSONValue(o.buf, f.Value)
    }
    
    o.buf = append(o.buf, "}\n"...)
    _, err := o.writer.Write(o.buf)
    return err
}
```

### 4.3 Log Rotation

```go
// internal/logging/rotation.go

type RotatingFileOutput struct {
    path       string
    maxSize    int64
    maxBackups int
    maxAge     time.Duration
    compress   bool
    
    mu         sync.Mutex
    file       *os.File
    size       int64
}

// Rotation triggered when file size exceeds maxSize.
// Algorithm:
// 1. Close current file
// 2. Rename: olb.log → olb.log.1
// 3. Shift: olb.log.1 → olb.log.2, etc.
// 4. Delete oldest if > maxBackups
// 5. Compress old files with gzip (in background goroutine)
// 6. Open new olb.log

// Also supports SIGUSR1 to reopen (for external rotation tools like logrotate)
```

---

## 5. Metrics Engine

### 5.1 Counter (Atomic)

```go
// internal/metrics/counter.go

type Counter struct {
    name   string
    labels map[string]string
    value  atomic.Int64
}

func (c *Counter) Inc() {
    c.value.Add(1)
}

func (c *Counter) Add(n int64) {
    c.value.Add(n)
}

func (c *Counter) Value() int64 {
    return c.value.Load()
}
```

### 5.2 Gauge (Atomic)

```go
// internal/metrics/gauge.go

type Gauge struct {
    name   string
    labels map[string]string
    value  atomic.Int64 // stored as int64, can represent float via Float64bits
}

func (g *Gauge) Set(v float64)  { g.value.Store(int64(math.Float64bits(v))) }
func (g *Gauge) Inc()           { /* atomic add on float */ }
func (g *Gauge) Dec()           { /* atomic sub on float */ }
func (g *Gauge) Value() float64 { return math.Float64frombits(uint64(g.value.Load())) }
```

### 5.3 Histogram (HDR-Style)

```go
// internal/metrics/histogram.go

// Histogram tracks value distributions using a log-linear bucketing scheme.
// Inspired by HDR Histogram but simplified.
// Provides: count, sum, min, max, percentiles (p50, p90, p95, p99)

type Histogram struct {
    name    string
    labels  map[string]string
    mu      sync.Mutex
    
    // Buckets: [0-1ms), [1-2ms), [2-5ms), [5-10ms), [10-20ms), ...
    // Log-linear: each decade has 3 sub-buckets
    buckets []atomic.Int64
    
    count   atomic.Int64
    sum     AtomicFloat64
    min     AtomicFloat64
    max     AtomicFloat64
}

// Pre-defined bucket boundaries (in seconds for latency):
var defaultBuckets = []float64{
    0.001, 0.002, 0.005, // 1ms, 2ms, 5ms
    0.01, 0.02, 0.05,    // 10ms, 20ms, 50ms
    0.1, 0.2, 0.5,       // 100ms, 200ms, 500ms
    1.0, 2.0, 5.0,       // 1s, 2s, 5s
    10.0, 30.0, 60.0,    // 10s, 30s, 60s
}

func (h *Histogram) Observe(value float64) {
    // Find bucket index via binary search
    idx := sort.SearchFloat64s(defaultBuckets, value)
    if idx < len(h.buckets) {
        h.buckets[idx].Add(1)
    }
    h.count.Add(1)
    h.sum.Add(value)
    
    // Update min/max with CAS loop
    for {
        old := h.min.Load()
        if value >= old && old != 0 {
            break
        }
        if h.min.CompareAndSwap(old, value) {
            break
        }
    }
    // Similar for max
}

// Percentile calculates the p-th percentile (0-100).
func (h *Histogram) Percentile(p float64) float64 {
    count := h.count.Load()
    if count == 0 {
        return 0
    }
    target := int64(float64(count) * p / 100.0)
    var cumulative int64
    for i, bucket := range h.buckets {
        cumulative += bucket.Load()
        if cumulative >= target {
            if i == 0 {
                return defaultBuckets[0]
            }
            // Linear interpolation within bucket
            return defaultBuckets[i]
        }
    }
    return defaultBuckets[len(defaultBuckets)-1]
}
```

### 5.4 Time Series Ring Buffer

```go
// internal/metrics/timeseries.go

// TimeSeries stores time-bucketed metric values for graphing.
// Ring buffer with configurable resolution and retention.
// Example: 1h retention at 10s resolution = 360 data points.

type TimeSeries struct {
    mu         sync.RWMutex
    resolution time.Duration
    retention  time.Duration
    points     []TimePoint
    head       int
    size       int
}

type TimePoint struct {
    Timestamp time.Time
    Value     float64
    Count     int64
}

func (ts *TimeSeries) Record(t time.Time, value float64) {
    ts.mu.Lock()
    defer ts.mu.Unlock()
    
    bucket := t.Truncate(ts.resolution)
    
    // If same bucket as head, aggregate
    if ts.size > 0 && ts.points[ts.head].Timestamp.Equal(bucket) {
        ts.points[ts.head].Value += value
        ts.points[ts.head].Count++
        return
    }
    
    // New bucket
    ts.head = (ts.head + 1) % len(ts.points)
    ts.points[ts.head] = TimePoint{
        Timestamp: bucket,
        Value:     value,
        Count:     1,
    }
    if ts.size < len(ts.points) {
        ts.size++
    }
}

// Range returns points within the time range.
func (ts *TimeSeries) Range(from, to time.Time) []TimePoint {
    ts.mu.RLock()
    defer ts.mu.RUnlock()
    
    result := make([]TimePoint, 0, ts.size)
    for i := 0; i < ts.size; i++ {
        idx := (ts.head - ts.size + 1 + i + len(ts.points)) % len(ts.points)
        p := ts.points[idx]
        if !p.Timestamp.Before(from) && !p.Timestamp.After(to) {
            result = append(result, p)
        }
    }
    return result
}
```

### 5.5 Prometheus Exposition

```go
// internal/metrics/prometheus.go

// Export metrics in Prometheus text format:
// # HELP olb_requests_total Total requests
// # TYPE olb_requests_total counter
// olb_requests_total{route="api",method="GET",status="200"} 12345

func (e *Engine) PrometheusHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
    
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)
    
    // Iterate all registered metrics
    e.mu.RLock()
    defer e.mu.RUnlock()
    
    for _, metric := range e.counters {
        writePrometheusCounter(buf, metric)
    }
    for _, metric := range e.gauges {
        writePrometheusGauge(buf, metric)
    }
    for _, metric := range e.histograms {
        writePrometheusHistogram(buf, metric)
    }
    
    w.Write(buf.Bytes())
}

func writePrometheusCounter(buf *bytes.Buffer, c *Counter) {
    buf.WriteString("# HELP ")
    buf.WriteString(c.name)
    buf.WriteString(" Counter\n# TYPE ")
    buf.WriteString(c.name)
    buf.WriteString(" counter\n")
    buf.WriteString(c.name)
    writeLabels(buf, c.labels)
    buf.WriteByte(' ')
    buf.WriteString(strconv.FormatInt(c.Value(), 10))
    buf.WriteByte('\n')
}
```

### 5.6 Metrics Registry

```go
// internal/metrics/engine.go

type Engine struct {
    mu         sync.RWMutex
    counters   map[string]*Counter
    gauges     map[string]*Gauge
    histograms map[string]*Histogram
    timeseries map[string]*TimeSeries
    
    // Fast-path metrics (pre-registered, no map lookup)
    requestsTotal    *Counter
    activeConns      *Gauge
    requestDuration  *Histogram
    bytesIn          *Counter
    bytesOut         *Counter
}

// NewEngine pre-registers all known metrics for fast access.
func NewEngine() *Engine {
    e := &Engine{
        counters:   make(map[string]*Counter),
        gauges:     make(map[string]*Gauge),
        histograms: make(map[string]*Histogram),
        timeseries: make(map[string]*TimeSeries),
    }
    
    // Pre-register fast-path metrics
    e.requestsTotal = e.NewCounter("olb_requests_total", nil)
    e.activeConns = e.NewGauge("olb_active_connections", nil)
    e.requestDuration = e.NewHistogram("olb_request_duration_seconds", nil)
    e.bytesIn = e.NewCounter("olb_bytes_received_total", nil)
    e.bytesOut = e.NewCounter("olb_bytes_sent_total", nil)
    
    return e
}

// With labels (dynamic metrics):
func (e *Engine) CounterVec(name string, labels map[string]string) *Counter {
    key := metricKey(name, labels)
    e.mu.RLock()
    if c, ok := e.counters[key]; ok {
        e.mu.RUnlock()
        return c
    }
    e.mu.RUnlock()
    
    e.mu.Lock()
    defer e.mu.Unlock()
    // Double-check
    if c, ok := e.counters[key]; ok {
        return c
    }
    c := &Counter{name: name, labels: labels}
    e.counters[key] = c
    return c
}
```

---

## 6. TLS Engine

### 6.1 TLS Manager

```go
// internal/tls/manager.go

type Manager struct {
    mu          sync.RWMutex
    certs       map[string]*tls.Certificate  // domain → cert
    wildcards   map[string]*tls.Certificate  // *.domain → cert
    defaultCert *tls.Certificate
    acmeClient  *acme.Client
    ocspCache   map[string]*OCSPResponse
    
    logger      *logging.Logger
    metrics     *metrics.Engine
}

// GetCertificate is the tls.Config.GetCertificate callback.
// Called by Go's TLS library during handshake.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
    m.mu.RLock()
    
    // 1. Exact match
    if cert, ok := m.certs[hello.ServerName]; ok {
        m.mu.RUnlock()
        return cert, nil
    }
    
    // 2. Wildcard match: "foo.example.com" → "*.example.com"
    if dotIdx := strings.IndexByte(hello.ServerName, '.'); dotIdx > 0 {
        wildcard := "*" + hello.ServerName[dotIdx:]
        if cert, ok := m.wildcards[wildcard]; ok {
            m.mu.RUnlock()
            return cert, nil
        }
    }
    
    m.mu.RUnlock()
    
    // 3. ACME on-demand (if enabled)
    if m.acmeClient != nil {
        cert, err := m.acmeClient.ObtainCertificate(hello.ServerName)
        if err == nil {
            m.mu.Lock()
            m.certs[hello.ServerName] = cert
            m.mu.Unlock()
            return cert, nil
        }
        m.logger.Error("ACME obtain failed",
            logging.String("domain", hello.ServerName),
            logging.Error(err))
    }
    
    // 4. Default cert
    if m.defaultCert != nil {
        return m.defaultCert, nil
    }
    
    return nil, fmt.Errorf("no certificate for %s", hello.ServerName)
}

// BuildTLSConfig creates a tls.Config for a listener.
func (m *Manager) BuildTLSConfig(cfg *TLSListenerConfig) *tls.Config {
    return &tls.Config{
        GetCertificate: m.GetCertificate,
        MinVersion:     parseTLSVersion(cfg.MinVersion),
        MaxVersion:     parseTLSVersion(cfg.MaxVersion),
        CipherSuites:   parseCipherSuites(cfg.CipherSuites),
        CurvePreferences: parseCurves(cfg.CurvePreferences),
        NextProtos:     []string{"h2", "http/1.1"}, // ALPN
        
        // Client auth (mTLS)
        ClientAuth: parseClientAuth(cfg.ClientAuth),
        ClientCAs:  loadCACertPool(cfg.ClientCAs),
    }
}

// Hot reload: swap certificates atomically
func (m *Manager) ReloadCertificates(certs []CertificateSource) error {
    newCerts := make(map[string]*tls.Certificate)
    newWildcards := make(map[string]*tls.Certificate)
    
    for _, src := range certs {
        cert, err := tls.LoadX509KeyPair(src.CertFile, src.KeyFile)
        if err != nil {
            return fmt.Errorf("load cert %s: %w", src.CertFile, err)
        }
        for _, domain := range src.Domains {
            if strings.HasPrefix(domain, "*.") {
                newWildcards[domain] = &cert
            } else {
                newCerts[domain] = &cert
            }
        }
    }
    
    m.mu.Lock()
    m.certs = newCerts
    m.wildcards = newWildcards
    m.mu.Unlock()
    
    m.logger.Info("TLS certificates reloaded",
        logging.Int("count", len(newCerts)+len(newWildcards)))
    return nil
}
```

### 6.2 ACME Client (Let's Encrypt)

```go
// internal/tls/acme/client.go

// Full ACME v2 client (RFC 8555) implementation.
// Steps:
// 1. Register account (or load existing)
// 2. Create order for domain(s)
// 3. Solve challenge (HTTP-01 or TLS-ALPN-01)
// 4. Finalize order with CSR
// 5. Download certificate chain

type Client struct {
    directoryURL string           // https://acme-v02.api.letsencrypt.org/directory
    accountKey   crypto.PrivateKey // ECDSA P-256
    accountURL   string
    email        string
    storage      *Store
    
    httpSolver   *HTTP01Solver
    tlsSolver    *TLSALPN01Solver
    
    // ACME directory endpoints (discovered)
    newNonce   string
    newAccount string
    newOrder   string
    revokeCert string
    
    httpClient *http.Client
    mu         sync.Mutex
}

// ObtainCertificate gets a certificate for the given domain.
func (c *Client) ObtainCertificate(domain string) (*tls.Certificate, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // 1. Check storage cache
    if cert, err := c.storage.Load(domain); err == nil {
        if time.Until(cert.Leaf.NotAfter) > 30*24*time.Hour {
            return cert, nil
        }
    }
    
    // 2. Create order
    order, err := c.createOrder([]string{domain})
    if err != nil {
        return nil, fmt.Errorf("create order: %w", err)
    }
    
    // 3. Solve challenges
    for _, authzURL := range order.Authorizations {
        authz, err := c.getAuthorization(authzURL)
        if err != nil {
            return nil, fmt.Errorf("get authz: %w", err)
        }
        if err := c.solveChallenge(authz); err != nil {
            return nil, fmt.Errorf("solve challenge: %w", err)
        }
    }
    
    // 4. Generate CSR
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
    if err != nil {
        return nil, fmt.Errorf("generate key: %w", err)
    }
    csr, err := x509.CreateCertificateRequest(cryptorand.Reader,
        &x509.CertificateRequest{
            DNSNames: []string{domain},
        }, privKey)
    if err != nil {
        return nil, fmt.Errorf("create CSR: %w", err)
    }
    
    // 5. Finalize order
    certPEM, err := c.finalizeOrder(order.FinalizeURL, csr)
    if err != nil {
        return nil, fmt.Errorf("finalize order: %w", err)
    }
    
    // 6. Parse and store
    cert, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(
        &pem.Block{Type: "EC PRIVATE KEY", Bytes: marshalECPrivateKey(privKey)}))
    if err != nil {
        return nil, fmt.Errorf("parse cert: %w", err)
    }
    
    c.storage.Save(domain, certPEM, privKey)
    return &cert, nil
}

// ACME HTTP communication:
// - All requests use JWS (JSON Web Signature) with account key
// - Nonce management: get fresh nonce from newNonce endpoint
// - JWS header: {"alg":"ES256","nonce":"...","url":"...","kid":"..."}
// - JWS payload: request body as JSON
// - POST-as-GET: empty payload with POST method for reading resources

func (c *Client) signedRequest(url string, payload interface{}) (*http.Response, error) {
    // 1. Get nonce
    nonce, err := c.getNonce()
    if err != nil {
        return nil, err
    }
    
    // 2. Build JWS header
    header := map[string]interface{}{
        "alg":   "ES256",
        "nonce": nonce,
        "url":   url,
    }
    if c.accountURL != "" {
        header["kid"] = c.accountURL
    } else {
        header["jwk"] = jwkFromKey(c.accountKey)
    }
    
    // 3. Marshal payload
    var payloadBytes []byte
    if payload != nil {
        payloadBytes, _ = json.Marshal(payload)
    }
    
    // 4. Create JWS
    headerB64 := base64url(jsonMarshal(header))
    payloadB64 := base64url(payloadBytes)
    sigInput := headerB64 + "." + payloadB64
    signature := signES256(c.accountKey.(*ecdsa.PrivateKey), []byte(sigInput))
    
    body := map[string]string{
        "protected": headerB64,
        "payload":   payloadB64,
        "signature": base64url(signature),
    }
    
    // 5. Send request
    reqBody, _ := json.Marshal(body)
    req, _ := http.NewRequest("POST", url, bytes.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/jose+json")
    
    return c.httpClient.Do(req)
}
```

### 6.3 HTTP-01 Challenge Solver

```go
// internal/tls/acme/challenge.go

// HTTP-01 solver serves challenge tokens at:
// http://<domain>/.well-known/acme-challenge/<token>
// Response body: <token>.<thumbprint>

type HTTP01Solver struct {
    mu     sync.RWMutex
    tokens map[string]string // token → key authorization
}

func (s *HTTP01Solver) Present(domain, token, keyAuth string) error {
    s.mu.Lock()
    s.tokens[token] = keyAuth
    s.mu.Unlock()
    return nil
}

func (s *HTTP01Solver) Cleanup(token string) {
    s.mu.Lock()
    delete(s.tokens, token)
    s.mu.Unlock()
}

// ServeHTTP handles ACME challenge requests.
// This is injected into the HTTP listener's handler chain.
func (s *HTTP01Solver) ServeHTTP(w http.ResponseWriter, r *http.Request) bool {
    if !strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
        return false // not an ACME request
    }
    token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
    s.mu.RLock()
    keyAuth, ok := s.tokens[token]
    s.mu.RUnlock()
    if !ok {
        http.NotFound(w, r)
        return true
    }
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte(keyAuth))
    return true
}
```

---

## 7. Connection Manager

```go
// internal/conn/manager.go

type Manager struct {
    maxConns      int64
    activeConns   atomic.Int64
    perSourceLimit int64
    
    // Per-source tracking
    mu          sync.RWMutex
    sourceCounts map[string]*atomic.Int64
    
    // Connection tracking for graceful drain
    tracked     sync.Map // connID → *TrackedConn
    nextID      atomic.Uint64
    
    metrics     *metrics.Engine
    logger      *logging.Logger
}

type TrackedConn struct {
    ID        uint64
    Conn      net.Conn
    Source    string
    StartTime time.Time
    BytesIn   atomic.Int64
    BytesOut  atomic.Int64
    Listener  string
    Route     string
    Backend   string
}

// Accept wraps a net.Conn with tracking and limits.
func (m *Manager) Accept(conn net.Conn, listener string) (*TrackedConn, error) {
    // 1. Check global limit
    current := m.activeConns.Add(1)
    if current > m.maxConns {
        m.activeConns.Add(-1)
        conn.Close()
        return nil, ErrMaxConnsReached
    }
    
    // 2. Check per-source limit
    source := extractIP(conn.RemoteAddr())
    if m.perSourceLimit > 0 {
        count := m.getSourceCount(source)
        if count.Add(1) > m.perSourceLimit {
            count.Add(-1)
            m.activeConns.Add(-1)
            conn.Close()
            return nil, ErrSourceLimitReached
        }
    }
    
    // 3. Track connection
    tc := &TrackedConn{
        ID:        m.nextID.Add(1),
        Conn:      conn,
        Source:    source,
        StartTime: time.Now(),
        Listener:  listener,
    }
    m.tracked.Store(tc.ID, tc)
    
    // 4. Update metrics
    m.metrics.activeConns.Set(float64(m.activeConns.Load()))
    
    return tc, nil
}

// Release removes a connection from tracking.
func (m *Manager) Release(tc *TrackedConn) {
    m.tracked.Delete(tc.ID)
    m.activeConns.Add(-1)
    
    if m.perSourceLimit > 0 {
        if count := m.getSourceCount(tc.Source); count != nil {
            count.Add(-1)
        }
    }
    
    m.metrics.activeConns.Set(float64(m.activeConns.Load()))
}

// Drain waits for all tracked connections to finish or timeout.
func (m *Manager) Drain(timeout time.Duration) error {
    deadline := time.After(timeout)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-deadline:
            // Force close remaining
            remaining := 0
            m.tracked.Range(func(_, v interface{}) bool {
                tc := v.(*TrackedConn)
                tc.Conn.Close()
                remaining++
                return true
            })
            return fmt.Errorf("drain timeout: %d connections force-closed", remaining)
        case <-ticker.C:
            if m.activeConns.Load() == 0 {
                return nil
            }
        }
    }
}

// ActiveConnections returns a snapshot of all active connections.
func (m *Manager) ActiveConnections() []*TrackedConn {
    var conns []*TrackedConn
    m.tracked.Range(func(_, v interface{}) bool {
        conns = append(conns, v.(*TrackedConn))
        return true
    })
    return conns
}
```

### 7.1 Backend Connection Pool

```go
// internal/conn/pool.go

// Pool manages reusable connections to a single backend address.
// Uses a channel-based idle pool with max size.

type Pool struct {
    address     string
    maxIdle     int
    maxActive   int
    idleTimeout time.Duration
    connTimeout time.Duration
    
    idle        chan *poolConn
    active      atomic.Int32
    mu          sync.Mutex
    closed      bool
    
    dialFunc    func(ctx context.Context, address string) (net.Conn, error)
}

type poolConn struct {
    conn      net.Conn
    createdAt time.Time
    usedAt    time.Time
}

// Get retrieves a connection from the pool or dials a new one.
func (p *Pool) Get(ctx context.Context) (net.Conn, error) {
    // 1. Try to get idle connection
    for {
        select {
        case pc := <-p.idle:
            // Check if connection is still alive and not expired
            if time.Since(pc.usedAt) > p.idleTimeout {
                pc.conn.Close()
                continue
            }
            // Quick liveness check: set short read deadline, check for EOF
            pc.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
            buf := make([]byte, 1)
            if _, err := pc.conn.Read(buf); err != nil && !isTimeout(err) {
                pc.conn.Close()
                continue
            }
            pc.conn.SetReadDeadline(time.Time{}) // clear deadline
            pc.usedAt = time.Now()
            p.active.Add(1)
            return pc.conn, nil
        default:
            // No idle connections
            goto dial
        }
    }

dial:
    // 2. Check active limit
    if int(p.active.Add(1)) > p.maxActive {
        p.active.Add(-1)
        return nil, ErrPoolExhausted
    }
    
    // 3. Dial new connection
    dialCtx, cancel := context.WithTimeout(ctx, p.connTimeout)
    defer cancel()
    
    conn, err := p.dialFunc(dialCtx, p.address)
    if err != nil {
        p.active.Add(-1)
        return nil, fmt.Errorf("dial %s: %w", p.address, err)
    }
    return conn, nil
}

// Put returns a connection to the pool.
func (p *Pool) Put(conn net.Conn) {
    p.active.Add(-1)
    
    pc := &poolConn{conn: conn, usedAt: time.Now()}
    select {
    case p.idle <- pc:
        // Returned to pool
    default:
        // Pool full, close connection
        conn.Close()
    }
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
    p.mu.Lock()
    p.closed = true
    p.mu.Unlock()
    
    close(p.idle)
    for pc := range p.idle {
        pc.conn.Close()
    }
    return nil
}
```

---

## 8. L7 HTTP Proxy

### 8.1 Reverse Proxy Core

```go
// internal/proxy/l7/proxy.go

type HTTPProxy struct {
    transport   *http.Transport
    bufferPool  *utils.BufferPool
    connPool    *conn.PoolManager
    middleware  *middleware.Chain
    router      *router.Router
    metrics     *metrics.Engine
    logger      *logging.Logger
    
    // Settings
    flushInterval    time.Duration
    bufferRequests   bool
    maxBodySize      int64
    readTimeout      time.Duration
    writeTimeout     time.Duration
    idleTimeout      time.Duration
}

// ServeHTTP is the main HTTP handler.
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Create request context
    ctx := p.newRequestContext(w, r)
    defer p.recycleContext(ctx)
    
    // 2. Run middleware chain (includes routing + proxying)
    err := p.middleware.Process(ctx)
    if err != nil {
        p.handleError(ctx, err)
    }
}

// proxyRequest is the core proxy handler (called by middleware chain).
func (p *HTTPProxy) proxyRequest(ctx *RequestContext) error {
    backend := ctx.Backend
    if backend == nil {
        return ErrNoBackend
    }
    
    // 1. Get connection to backend
    backendConn, err := p.connPool.Get(ctx.Request.Context(), backend.Address)
    if err != nil {
        return fmt.Errorf("backend conn: %w", err)
    }
    
    // 2. Prepare outbound request
    outReq := p.prepareOutboundRequest(ctx)
    
    // 3. Send request to backend
    resp, err := p.roundTrip(backendConn, outReq)
    if err != nil {
        p.connPool.Discard(backendConn)
        return fmt.Errorf("backend roundtrip: %w", err)
    }
    
    // 4. Copy response headers
    p.copyResponseHeaders(ctx.Response, resp)
    
    // 5. Write status code
    ctx.Response.WriteHeader(resp.StatusCode)
    
    // 6. Copy response body (streaming)
    buf := p.bufferPool.Get(32768)
    defer p.bufferPool.Put(buf)
    
    written, err := io.CopyBuffer(ctx.Response, resp.Body, *buf)
    resp.Body.Close()
    
    // 7. Return connection to pool (only if response was fully read)
    if err == nil {
        p.connPool.Put(backendConn)
    } else {
        p.connPool.Discard(backendConn)
    }
    
    // 8. Record metrics
    ctx.Metrics.BytesOut = written
    ctx.Metrics.StatusCode = resp.StatusCode
    ctx.Metrics.BackendLatency = time.Since(ctx.Metrics.BackendStart)
    
    return err
}

// prepareOutboundRequest modifies the request for the backend.
func (p *HTTPProxy) prepareOutboundRequest(ctx *RequestContext) *http.Request {
    outReq := ctx.Request.Clone(ctx.Request.Context())
    
    // Set backend address
    outReq.URL.Scheme = ctx.Backend.Scheme
    outReq.URL.Host = ctx.Backend.Address
    
    // Path rewriting (if configured)
    if ctx.Route.Rewrite != "" {
        outReq.URL.Path = ctx.Route.ApplyRewrite(outReq.URL.Path)
    }
    
    // Strip hop-by-hop headers
    removeHopByHopHeaders(outReq.Header)
    
    // Add proxy headers
    clientIP := ctx.ClientIP.String()
    if prior := outReq.Header.Get("X-Forwarded-For"); prior != "" {
        outReq.Header.Set("X-Forwarded-For", prior+", "+clientIP)
    } else {
        outReq.Header.Set("X-Forwarded-For", clientIP)
    }
    outReq.Header.Set("X-Real-IP", clientIP)
    outReq.Header.Set("X-Forwarded-Proto", ctx.Scheme)
    outReq.Header.Set("X-Forwarded-Host", ctx.Request.Host)
    outReq.Header.Set("X-Request-ID", ctx.RequestID)
    
    return outReq
}

// Hop-by-hop headers that MUST NOT be forwarded
var hopByHopHeaders = []string{
    "Connection", "Keep-Alive", "Proxy-Authenticate",
    "Proxy-Authorization", "TE", "Trailers",
    "Transfer-Encoding", "Upgrade",
}
```

### 8.2 WebSocket Proxy

```go
// internal/proxy/l7/websocket.go

func (p *HTTPProxy) handleWebSocket(ctx *RequestContext) error {
    // 1. Hijack client connection
    hijacker, ok := ctx.Response.Unwrap().(http.Hijacker)
    if !ok {
        return fmt.Errorf("websocket: response not hijackable")
    }
    clientConn, clientBuf, err := hijacker.Hijack()
    if err != nil {
        return fmt.Errorf("websocket: hijack failed: %w", err)
    }
    defer clientConn.Close()
    
    // 2. Dial backend
    backendConn, err := net.DialTimeout("tcp", ctx.Backend.Address, 5*time.Second)
    if err != nil {
        return fmt.Errorf("websocket: backend dial: %w", err)
    }
    defer backendConn.Close()
    
    // 3. Forward the upgrade request to backend
    outReq := p.prepareOutboundRequest(ctx)
    outReq.Write(backendConn)
    
    // 4. Read backend's upgrade response
    backendBuf := bufio.NewReader(backendConn)
    resp, err := http.ReadResponse(backendBuf, outReq)
    if err != nil || resp.StatusCode != 101 {
        return fmt.Errorf("websocket: backend rejected upgrade")
    }
    
    // 5. Forward upgrade response to client
    resp.Write(clientConn)
    
    // 6. Bidirectional copy
    errc := make(chan error, 2)
    
    // Client → Backend
    go func() {
        _, err := io.Copy(backendConn, clientBuf)
        errc <- err
    }()
    
    // Backend → Client
    go func() {
        _, err := io.Copy(clientConn, backendBuf)
        errc <- err
    }()
    
    // Wait for either direction to finish
    <-errc
    return nil
}
```

---

## 9. L4 TCP/UDP Proxy

### 9.1 TCP Proxy

```go
// internal/proxy/l4/proxy.go

type TCPProxy struct {
    balancer    balancer.Balancer
    connPool    *conn.PoolManager
    metrics     *metrics.Engine
    logger      *logging.Logger
    bufferPool  *utils.BufferPool
    
    proxyProtocol string // "", "v1", "v2"
}

func (p *TCPProxy) HandleConn(tc *conn.TrackedConn) {
    defer tc.Conn.Close()
    
    // 1. SNI extraction (optional, for TLS passthrough)
    var peekedData []byte
    var sniHost string
    
    if p.sniRouting {
        var err error
        sniHost, peekedData, err = extractSNI(tc.Conn)
        if err != nil {
            p.logger.Debug("SNI extraction failed", logging.Error(err))
        }
    }
    
    // 2. Select backend
    reqCtx := &balancer.RequestContext{
        ClientIP: tc.Conn.RemoteAddr().(*net.TCPAddr).IP,
        Host:     sniHost,
    }
    backend := p.balancer.Next(reqCtx)
    if backend == nil {
        p.logger.Error("no available backend")
        return
    }
    tc.Backend = backend.Address
    
    // 3. Dial backend
    backendConn, err := net.DialTimeout("tcp", backend.Address, 5*time.Second)
    if err != nil {
        p.logger.Error("backend dial failed",
            logging.String("backend", backend.Address),
            logging.Error(err))
        p.metrics.CounterVec("olb_backend_errors_total",
            map[string]string{"backend": backend.Address}).Inc()
        return
    }
    defer backendConn.Close()
    
    // 4. Send PROXY protocol header (if configured)
    if p.proxyProtocol != "" {
        if err := p.sendProxyProtocol(backendConn, tc.Conn); err != nil {
            p.logger.Error("proxy protocol failed", logging.Error(err))
            return
        }
    }
    
    // 5. Replay peeked data (from SNI extraction)
    if len(peekedData) > 0 {
        if _, err := backendConn.Write(peekedData); err != nil {
            return
        }
    }
    
    // 6. Bidirectional copy
    p.bidirectionalCopy(tc, backendConn)
}

func (p *TCPProxy) bidirectionalCopy(tc *conn.TrackedConn, backend net.Conn) {
    errc := make(chan error, 2)
    
    // Client → Backend
    go func() {
        buf := p.bufferPool.Get(32768)
        defer p.bufferPool.Put(buf)
        n, err := io.CopyBuffer(backend, tc.Conn, *buf)
        tc.BytesIn.Add(n)
        // Half-close: signal backend that client is done writing
        if tcpConn, ok := backend.(*net.TCPConn); ok {
            tcpConn.CloseWrite()
        }
        errc <- err
    }()
    
    // Backend → Client
    go func() {
        buf := p.bufferPool.Get(32768)
        defer p.bufferPool.Put(buf)
        n, err := io.CopyBuffer(tc.Conn, backend, *buf)
        tc.BytesOut.Add(n)
        if tcpConn, ok := tc.Conn.(*net.TCPConn); ok {
            tcpConn.CloseWrite()
        }
        errc <- err
    }()
    
    // Wait for both directions
    <-errc
    <-errc
}
```

### 9.2 Zero-Copy Splice (Linux)

```go
// internal/proxy/l4/splice.go
// +build linux

// splice() copies data between two file descriptors in kernel space,
// avoiding user-space copies entirely. ~2x throughput for large transfers.

func spliceConn(dst, src *net.TCPConn) (int64, error) {
    // Get raw file descriptors
    srcFile, _ := src.File()
    dstFile, _ := dst.File()
    srcFd := int(srcFile.Fd())
    dstFd := int(dstFile.Fd())
    defer srcFile.Close()
    defer dstFile.Close()
    
    // Create pipe for splice
    var pipeFds [2]int
    if err := syscall.Pipe2(pipeFds[:], syscall.O_CLOEXEC|syscall.O_NONBLOCK); err != nil {
        // Fallback to io.Copy
        return io.Copy(dst, src)
    }
    defer syscall.Close(pipeFds[0])
    defer syscall.Close(pipeFds[1])
    
    var total int64
    for {
        // splice: src fd → pipe (read end)
        n, err := syscall.Splice(srcFd, nil, pipeFds[1], nil, 65536,
            syscall.SPLICE_F_MOVE|syscall.SPLICE_F_NONBLOCK)
        if err != nil || n == 0 {
            return total, err
        }
        
        // splice: pipe → dst fd (write end)
        _, err = syscall.Splice(pipeFds[0], nil, dstFd, nil, int(n),
            syscall.SPLICE_F_MOVE|syscall.SPLICE_F_NONBLOCK)
        if err != nil {
            return total, err
        }
        
        total += n
    }
}
```

### 9.3 SNI Extraction

```go
// internal/proxy/l4/sni.go

// extractSNI peeks at the TLS ClientHello to extract the SNI hostname.
// Does NOT consume the bytes — returns them for replay.

func extractSNI(conn net.Conn) (string, []byte, error) {
    // Set read deadline for peek
    conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    defer conn.SetReadDeadline(time.Time{})
    
    // Read enough for ClientHello (usually < 512 bytes, max 16KB)
    buf := make([]byte, 16384)
    n, err := conn.Read(buf)
    if err != nil {
        return "", nil, err
    }
    data := buf[:n]
    
    // Parse TLS record header
    if len(data) < 5 || data[0] != 0x16 { // ContentType: Handshake
        return "", data, fmt.Errorf("not TLS")
    }
    
    // Parse Handshake header
    recordLen := int(data[3])<<8 | int(data[4])
    if len(data) < 5+recordLen {
        return "", data, fmt.Errorf("incomplete record")
    }
    
    handshake := data[5:]
    if handshake[0] != 0x01 { // HandshakeType: ClientHello
        return "", data, fmt.Errorf("not ClientHello")
    }
    
    // Skip: handshake length (3), client version (2), random (32)
    pos := 4 + 2 + 32
    
    // Session ID
    if pos >= len(handshake) {
        return "", data, fmt.Errorf("truncated")
    }
    sessionIDLen := int(handshake[pos])
    pos += 1 + sessionIDLen
    
    // Cipher suites
    if pos+2 > len(handshake) {
        return "", data, fmt.Errorf("truncated")
    }
    cipherLen := int(handshake[pos])<<8 | int(handshake[pos+1])
    pos += 2 + cipherLen
    
    // Compression methods
    if pos >= len(handshake) {
        return "", data, fmt.Errorf("truncated")
    }
    compLen := int(handshake[pos])
    pos += 1 + compLen
    
    // Extensions
    if pos+2 > len(handshake) {
        return "", data, nil // no extensions, no SNI
    }
    extLen := int(handshake[pos])<<8 | int(handshake[pos+1])
    pos += 2
    
    extEnd := pos + extLen
    for pos+4 <= extEnd {
        extType := int(handshake[pos])<<8 | int(handshake[pos+1])
        extDataLen := int(handshake[pos+2])<<8 | int(handshake[pos+3])
        pos += 4
        
        if extType == 0x0000 { // SNI extension
            // Parse SNI list
            if pos+2 > extEnd {
                break
            }
            sniListLen := int(handshake[pos])<<8 | int(handshake[pos+1])
            _ = sniListLen
            pos += 2
            
            if pos+3 > extEnd {
                break
            }
            nameType := handshake[pos]
            nameLen := int(handshake[pos+1])<<8 | int(handshake[pos+2])
            pos += 3
            
            if nameType == 0 && pos+nameLen <= extEnd { // host_name
                sni := string(handshake[pos : pos+nameLen])
                return sni, data, nil
            }
        }
        
        pos += extDataLen
    }
    
    return "", data, nil // no SNI found
}
```

---

## 10. Router Implementation

### 10.1 Radix Trie for Path Matching

```go
// internal/router/trie.go

// RadixTrie provides fast path-based routing.
// Supports: exact match, prefix match, parametric (:param), wildcard (*).
// O(k) lookup where k = number of path segments.

type RadixTrie struct {
    root *trieNode
}

type trieNode struct {
    // Static children: segment → node
    children map[string]*trieNode
    
    // Parameter child (:param)
    paramChild *trieNode
    paramName  string
    
    // Wildcard child (*)
    wildcardChild *trieNode
    wildcardName  string
    
    // Route at this node (nil if not a terminal)
    route *Route
    
    // Priority for sorting matches
    priority int
}

// Insert adds a route pattern to the trie.
func (t *RadixTrie) Insert(pattern string, route *Route) {
    segments := splitPath(pattern)
    node := t.root
    
    for _, seg := range segments {
        switch {
        case seg == "*" || strings.HasPrefix(seg, "*"):
            // Wildcard: matches everything after this point
            if node.wildcardChild == nil {
                node.wildcardChild = &trieNode{}
                if len(seg) > 1 {
                    node.wildcardName = seg[1:]
                }
            }
            node = node.wildcardChild
            node.route = route
            return
            
        case strings.HasPrefix(seg, ":"):
            // Parameter: matches one segment
            if node.paramChild == nil {
                node.paramChild = &trieNode{
                    children: make(map[string]*trieNode),
                }
                node.paramName = seg[1:]
            }
            node = node.paramChild
            
        default:
            // Static segment
            child, ok := node.children[seg]
            if !ok {
                child = &trieNode{
                    children: make(map[string]*trieNode),
                }
                node.children[seg] = child
            }
            node = child
        }
    }
    
    node.route = route
}

// Match finds the best matching route for a path.
// Priority: exact > param > wildcard
func (t *RadixTrie) Match(path string) (*Route, map[string]string) {
    segments := splitPath(path)
    params := make(map[string]string)
    route := t.match(t.root, segments, 0, params)
    return route, params
}

func (t *RadixTrie) match(node *trieNode, segments []string, idx int, params map[string]string) *Route {
    if idx == len(segments) {
        return node.route
    }
    
    seg := segments[idx]
    
    // 1. Try static match (highest priority)
    if child, ok := node.children[seg]; ok {
        if route := t.match(child, segments, idx+1, params); route != nil {
            return route
        }
    }
    
    // 2. Try parameter match
    if node.paramChild != nil {
        params[node.paramName] = seg
        if route := t.match(node.paramChild, segments, idx+1, params); route != nil {
            return route
        }
        delete(params, node.paramName)
    }
    
    // 3. Try wildcard match (lowest priority)
    if node.wildcardChild != nil {
        // Wildcard captures all remaining segments
        remaining := strings.Join(segments[idx:], "/")
        if node.wildcardName != "" {
            params[node.wildcardName] = remaining
        }
        return node.wildcardChild.route
    }
    
    return nil
}
```

### 10.2 Route Matcher

```go
// internal/router/router.go

type Router struct {
    mu     sync.RWMutex
    
    // Per-host tries for path matching
    hostTries    map[string]*RadixTrie // "example.com" → trie
    wildcardTries map[string]*RadixTrie // "*.example.com" → trie
    defaultTrie  *RadixTrie             // fallback (no host match)
    
    // Routes by name (for API access)
    routes map[string]*Route
}

type Route struct {
    Name        string
    Listener    string           // Bind to specific listener (optional)
    
    // Match criteria
    Hosts       []string         // Host header matching
    PathPrefix  string           // Path prefix
    PathExact   string           // Exact path
    PathRegex   string           // Regex path (compiled)
    Methods     []string         // HTTP methods
    Headers     map[string]string // Required headers
    
    // Action
    Backend     string           // Backend pool name
    Rewrite     string           // Path rewrite rule
    Redirect    *RedirectAction  // Redirect instead of proxy
    Split       []WeightedBackend // Traffic splitting
    
    // Middleware (per-route)
    Middleware  []middleware.Config
    
    // Proxy settings
    ProxyConfig *ProxyConfig
    
    // Metadata
    Priority   int
    Enabled    bool
}

// Match finds the route for an incoming request.
func (r *Router) Match(req *http.Request) (*Route, map[string]string) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    host := stripPort(req.Host)
    
    // 1. Try exact host match
    if trie, ok := r.hostTries[host]; ok {
        if route, params := trie.Match(req.URL.Path); route != nil {
            if r.matchExtra(route, req) {
                return route, params
            }
        }
    }
    
    // 2. Try wildcard host match
    // "foo.example.com" → try "*.example.com"
    if dotIdx := strings.IndexByte(host, '.'); dotIdx > 0 {
        wildcard := "*" + host[dotIdx:]
        if trie, ok := r.wildcardTries[wildcard]; ok {
            if route, params := trie.Match(req.URL.Path); route != nil {
                if r.matchExtra(route, req) {
                    return route, params
                }
            }
        }
    }
    
    // 3. Try default trie (no host constraint)
    if route, params := r.defaultTrie.Match(req.URL.Path); route != nil {
        if r.matchExtra(route, req) {
            return route, params
        }
    }
    
    return nil, nil
}

// matchExtra checks method and header constraints.
func (r *Router) matchExtra(route *Route, req *http.Request) bool {
    // Method check
    if len(route.Methods) > 0 {
        found := false
        for _, m := range route.Methods {
            if m == req.Method {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }
    
    // Header check
    for key, expected := range route.Headers {
        actual := req.Header.Get(key)
        if !strings.EqualFold(actual, expected) {
            return false
        }
    }
    
    return true
}
```

---

## 11. Load Balancer Algorithms

### 11.1 Round Robin

```go
// internal/balancer/roundrobin.go

type RoundRobin struct {
    mu       sync.RWMutex
    backends []*backend.Backend
    next     atomic.Uint64
}

func (rr *RoundRobin) Next(ctx *RequestContext) *backend.Backend {
    rr.mu.RLock()
    backends := rr.backends
    rr.mu.RUnlock()
    
    if len(backends) == 0 {
        return nil
    }
    
    n := rr.next.Add(1)
    return backends[n%uint64(len(backends))]
}
```

### 11.2 Weighted Round Robin (Smooth)

```go
// internal/balancer/weighted_roundrobin.go

// Nginx-style smooth weighted round-robin.
// Avoids burst traffic to high-weight backends.
// Algorithm:
// 1. For each backend: currentWeight += effectiveWeight
// 2. Select backend with highest currentWeight
// 3. Selected backend: currentWeight -= totalWeight

type WeightedRoundRobin struct {
    mu       sync.Mutex
    backends []*wrr_entry
    total    int
}

type wrr_entry struct {
    backend         *backend.Backend
    weight          int // configured weight
    effectiveWeight int // adjusted weight (reduced on errors)
    currentWeight   int // running weight
}

func (w *WeightedRoundRobin) Next(ctx *RequestContext) *backend.Backend {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    if len(w.backends) == 0 {
        return nil
    }
    
    var best *wrr_entry
    for _, entry := range w.backends {
        entry.currentWeight += entry.effectiveWeight
        if best == nil || entry.currentWeight > best.currentWeight {
            best = entry
        }
    }
    
    if best == nil {
        return nil
    }
    
    best.currentWeight -= w.total
    return best.backend
}
```

### 11.3 Least Connections

```go
// internal/balancer/least_conn.go

type LeastConnections struct {
    mu       sync.RWMutex
    backends []*backend.Backend
}

func (lc *LeastConnections) Next(ctx *RequestContext) *backend.Backend {
    lc.mu.RLock()
    defer lc.mu.RUnlock()
    
    var best *backend.Backend
    var bestConns int64 = math.MaxInt64
    
    for _, b := range lc.backends {
        conns := b.ActiveConnections()
        // Weighted: connections / weight
        weighted := conns * 100 / int64(max(b.Weight, 1))
        if weighted < bestConns {
            bestConns = weighted
            best = b
        }
    }
    
    return best
}
```

### 11.4 Consistent Hashing (Ketama)

```go
// internal/balancer/consistent_hash.go

type ConsistentHash struct {
    mu           sync.RWMutex
    ring         []hashPoint
    vnodes       int // virtual nodes per backend (default: 150)
    backends     map[string]*backend.Backend
}

type hashPoint struct {
    hash    uint32
    backend string // backend ID
}

func (ch *ConsistentHash) Next(ctx *RequestContext) *backend.Backend {
    key := ch.getKey(ctx) // usually client IP or session key
    hash := ch.hash(key)
    
    ch.mu.RLock()
    defer ch.mu.RUnlock()
    
    if len(ch.ring) == 0 {
        return nil
    }
    
    // Binary search for the first point >= hash
    idx := sort.Search(len(ch.ring), func(i int) bool {
        return ch.ring[i].hash >= hash
    })
    if idx == len(ch.ring) {
        idx = 0 // wrap around
    }
    
    return ch.backends[ch.ring[idx].backend]
}

// hash uses FNV-1a for consistent hashing (good distribution)
func (ch *ConsistentHash) hash(key string) uint32 {
    h := fnv.New32a()
    h.Write([]byte(key))
    return h.Sum32()
}

// Add rebuilds the ring with the new backend.
func (ch *ConsistentHash) Add(b *backend.Backend) {
    ch.mu.Lock()
    defer ch.mu.Unlock()
    
    ch.backends[b.ID] = b
    for i := 0; i < ch.vnodes; i++ {
        key := fmt.Sprintf("%s#%d", b.ID, i)
        ch.ring = append(ch.ring, hashPoint{
            hash:    ch.hash(key),
            backend: b.ID,
        })
    }
    sort.Slice(ch.ring, func(i, j int) bool {
        return ch.ring[i].hash < ch.ring[j].hash
    })
}
```

### 11.5 Power of Two Random Choices (P2C)

```go
// internal/balancer/power_of_two.go

// P2C: Pick 2 random backends, choose the one with fewer active connections.
// Near-optimal (exponential improvement over random) with O(1) selection.

type PowerOfTwo struct {
    mu       sync.RWMutex
    backends []*backend.Backend
    rng      *utils.FastRand
}

func (p *PowerOfTwo) Next(ctx *RequestContext) *backend.Backend {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    n := len(p.backends)
    if n == 0 {
        return nil
    }
    if n == 1 {
        return p.backends[0]
    }
    
    // Pick two random backends
    i := p.rng.Intn(n)
    j := p.rng.Intn(n - 1)
    if j >= i {
        j++ // ensure i != j
    }
    
    a := p.backends[i]
    b := p.backends[j]
    
    // Choose the one with fewer active connections
    if a.ActiveConnections() <= b.ActiveConnections() {
        return a
    }
    return b
}
```

### 11.6 Maglev Hashing

```go
// internal/balancer/maglev.go

// Google's Maglev hashing algorithm.
// Properties:
// - O(1) lookup after O(M*N) initialization
// - Minimal disruption when backends change
// - Good load distribution even with heterogeneous backends
// - M must be prime (default: 65537)

const maglevTableSize = 65537 // prime

type Maglev struct {
    mu       sync.RWMutex
    table    [maglevTableSize]int // lookup table: hash → backend index
    backends []*backend.Backend
}

func (m *Maglev) populate() {
    n := len(m.backends)
    if n == 0 {
        return
    }
    
    // Calculate offset and skip for each backend
    type perm struct {
        offset uint64
        skip   uint64
    }
    perms := make([]perm, n)
    for i, b := range m.backends {
        h1 := hash1(b.ID)
        h2 := hash2(b.ID)
        perms[i] = perm{
            offset: h1 % maglevTableSize,
            skip:   (h2 % (maglevTableSize - 1)) + 1,
        }
    }
    
    // Fill table using round-robin permutation
    next := make([]uint64, n)
    for i := range next {
        next[i] = perms[i].offset
    }
    
    for i := range m.table {
        m.table[i] = -1 // empty
    }
    
    filled := 0
    for filled < maglevTableSize {
        for i := 0; i < n; i++ {
            c := next[i]
            for m.table[c] != -1 {
                c = (c + perms[i].skip) % maglevTableSize
            }
            m.table[c] = i
            next[i] = (c + perms[i].skip) % maglevTableSize
            filled++
            if filled >= maglevTableSize {
                break
            }
        }
    }
}

func (m *Maglev) Next(ctx *RequestContext) *backend.Backend {
    key := ctx.ClientIP.String() // or session key
    hash := fnvHash(key)
    
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    idx := m.table[hash%maglevTableSize]
    if idx < 0 || idx >= len(m.backends) {
        return nil
    }
    return m.backends[idx]
}
```

---

## 12. Backend Pool & State Machine

```go
// internal/backend/pool.go

type Pool struct {
    name     string
    mu       sync.RWMutex
    backends map[string]*Backend
    balancer balancer.Balancer
    health   *health.Checker
    
    connPools map[string]*conn.Pool // per-backend connection pool
}

type Backend struct {
    ID               string
    Address          string
    Weight           int
    MaxConnections   int
    Scheme           string // "http", "https"
    Metadata         map[string]string
    
    // State
    state            atomic.Int32 // BackendState
    
    // Stats (atomic, lock-free)
    activeConns      atomic.Int64
    totalConns       atomic.Int64
    totalRequests    atomic.Int64
    totalErrors      atomic.Int64
    totalBytes       atomic.Int64
    lastResponseTime AtomicDuration
    
    // Health
    healthState      atomic.Int32
    lastHealthCheck  atomic.Value // time.Time
    consecutiveOK    atomic.Int32
    consecutiveFail  atomic.Int32
}

type BackendState int32

const (
    BackendUp          BackendState = 0
    BackendDown        BackendState = 1
    BackendDraining    BackendState = 2
    BackendMaintenance BackendState = 3
    BackendStarting    BackendState = 4
)

// IsAvailable returns true if the backend can accept new connections.
func (b *Backend) IsAvailable() bool {
    state := BackendState(b.state.Load())
    return state == BackendUp || state == BackendStarting
}

// ActiveConnections returns current active connection count.
func (b *Backend) ActiveConnections() int64 {
    return b.activeConns.Load()
}
```

---

## 13. Health Checker

```go
// internal/health/checker.go

type Checker struct {
    mu       sync.RWMutex
    checks   map[string]*checkEntry // backend ID → entry
    logger   *logging.Logger
    metrics  *metrics.Engine
    
    ctx      context.Context
    cancel   context.CancelFunc
}

type checkEntry struct {
    backend  *backend.Backend
    config   HealthCheckConfig
    ticker   *time.Ticker
    stopCh   chan struct{}
}

func (c *Checker) Start() {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    for _, entry := range c.checks {
        go c.runActiveCheck(entry)
    }
}

func (c *Checker) runActiveCheck(entry *checkEntry) {
    for {
        select {
        case <-entry.ticker.C:
            result := c.performCheck(entry)
            c.processResult(entry, result)
        case <-entry.stopCh:
            return
        case <-c.ctx.Done():
            return
        }
    }
}

func (c *Checker) performCheck(entry *checkEntry) CheckResult {
    switch entry.config.Type {
    case "http":
        return c.httpCheck(entry)
    case "tcp":
        return c.tcpCheck(entry)
    case "grpc":
        return c.grpcCheck(entry)
    case "exec":
        return c.execCheck(entry)
    default:
        return CheckResult{Healthy: false, Error: "unknown check type"}
    }
}

func (c *Checker) httpCheck(entry *checkEntry) CheckResult {
    cfg := entry.config
    
    ctx, cancel := context.WithTimeout(c.ctx, cfg.Timeout)
    defer cancel()
    
    req, err := http.NewRequestWithContext(ctx, cfg.Method, 
        fmt.Sprintf("http://%s%s", entry.backend.Address, cfg.Path), nil)
    if err != nil {
        return CheckResult{Healthy: false, Error: err.Error()}
    }
    
    for k, v := range cfg.Headers {
        req.Header.Set(k, v)
    }
    
    start := time.Now()
    resp, err := http.DefaultClient.Do(req)
    duration := time.Since(start)
    
    if err != nil {
        return CheckResult{Healthy: false, Duration: duration, Error: err.Error()}
    }
    defer resp.Body.Close()
    
    // Check status code
    statusOK := false
    for _, expected := range cfg.ExpectedStatus {
        if resp.StatusCode == expected {
            statusOK = true
            break
        }
    }
    
    // Check body (if configured)
    if statusOK && cfg.ExpectedBody != "" {
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        if !strings.Contains(string(body), cfg.ExpectedBody) {
            return CheckResult{Healthy: false, Duration: duration, 
                Error: "body mismatch"}
        }
    }
    
    return CheckResult{Healthy: statusOK, Duration: duration, 
        StatusCode: resp.StatusCode}
}

func (c *Checker) processResult(entry *checkEntry, result CheckResult) {
    b := entry.backend
    cfg := entry.config
    
    if result.Healthy {
        b.ConsecutiveFail.Store(0)
        ok := b.ConsecutiveOK.Add(1)
        
        if int(ok) >= cfg.SuccessThreshold {
            if BackendState(b.State.Load()) != BackendUp {
                b.State.Store(int32(BackendUp))
                c.logger.Info("backend healthy",
                    logging.String("backend", b.Address))
                c.metrics.CounterVec("olb_health_checks_total",
                    map[string]string{"backend": b.Address, "result": "up"}).Inc()
            }
        }
    } else {
        b.ConsecutiveOK.Store(0)
        fail := b.ConsecutiveFail.Add(1)
        
        if int(fail) >= cfg.FailureThreshold {
            if BackendState(b.State.Load()) == BackendUp {
                b.State.Store(int32(BackendDown))
                c.logger.Warn("backend unhealthy",
                    logging.String("backend", b.Address),
                    logging.String("error", result.Error))
                c.metrics.CounterVec("olb_health_checks_total",
                    map[string]string{"backend": b.Address, "result": "down"}).Inc()
            }
        }
    }
}
```

---

## 14. Middleware Pipeline

### 14.1 Chain Builder

```go
// internal/middleware/chain.go

type Chain struct {
    middlewares []Middleware
    handler     Handler // final handler (proxy)
}

func NewChain(handler Handler, mws ...Middleware) *Chain {
    // Sort by priority
    sort.Slice(mws, func(i, j int) bool {
        return mws[i].Priority() < mws[j].Priority()
    })
    return &Chain{middlewares: mws, handler: handler}
}

func (c *Chain) Process(ctx *RequestContext) error {
    // Build handler chain (inside out)
    h := c.handler
    for i := len(c.middlewares) - 1; i >= 0; i-- {
        mw := c.middlewares[i]
        next := h
        h = HandlerFunc(func(ctx *RequestContext) error {
            return mw.Process(ctx, next)
        })
    }
    return h.Process(ctx)
}
```

### 14.2 Rate Limiter — Token Bucket

```go
// internal/middleware/ratelimit/token_bucket.go

type TokenBucket struct {
    mu       sync.Mutex
    rate     float64       // tokens per second
    burst    float64       // max tokens
    tokens   float64       // current tokens
    lastTime time.Time
}

func (tb *TokenBucket) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    
    now := time.Now()
    elapsed := now.Sub(tb.lastTime).Seconds()
    tb.lastTime = now
    
    // Refill tokens
    tb.tokens += elapsed * tb.rate
    if tb.tokens > tb.burst {
        tb.tokens = tb.burst
    }
    
    // Try to consume one token
    if tb.tokens >= 1.0 {
        tb.tokens -= 1.0
        return true
    }
    
    return false
}

// RateLimitMiddleware with per-key buckets
type RateLimitMiddleware struct {
    mu      sync.RWMutex
    buckets map[string]*TokenBucket
    config  RateLimitConfig
    
    // Cleanup old buckets periodically
    cleanupTicker *time.Ticker
}

func (rl *RateLimitMiddleware) Process(ctx *RequestContext, next Handler) error {
    key := rl.extractKey(ctx)
    
    bucket := rl.getBucket(key)
    if !bucket.Allow() {
        // Rate limited
        ctx.Response.Header().Set("X-RateLimit-Limit", 
            strconv.Itoa(int(rl.config.Rate)))
        ctx.Response.Header().Set("X-RateLimit-Remaining", "0")
        ctx.Response.Header().Set("Retry-After", 
            strconv.Itoa(int(1.0/rl.config.Rate)+1))
        ctx.Response.WriteHeader(http.StatusTooManyRequests)
        ctx.Response.Write([]byte(`{"error":"rate_limit_exceeded"}`))
        return nil // short-circuit, don't call next
    }
    
    return next.Process(ctx)
}
```

### 14.3 Circuit Breaker

```go
// internal/middleware/circuit/breaker.go

type State int32

const (
    StateClosed   State = 0 // normal operation
    StateOpen     State = 1 // failing, reject requests
    StateHalfOpen State = 2 // testing recovery
)

type CircuitBreaker struct {
    state        atomic.Int32
    failures     atomic.Int32
    successes    atomic.Int32 // in half-open state
    lastFailure  atomic.Value // time.Time
    
    config       CircuitBreakerConfig
    
    mu           sync.Mutex
    resetTimer   *time.Timer
}

func (cb *CircuitBreaker) Process(ctx *RequestContext, next Handler) error {
    state := State(cb.state.Load())
    
    switch state {
    case StateOpen:
        // Check if enough time has passed to try half-open
        lastFail := cb.lastFailure.Load().(time.Time)
        if time.Since(lastFail) < cb.config.OpenDuration {
            return ErrCircuitOpen
        }
        // Transition to half-open
        cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen))
        cb.successes.Store(0)
        fallthrough
        
    case StateHalfOpen:
        // Allow limited requests through
        err := next.Process(ctx)
        if err != nil || ctx.Metrics.StatusCode >= 500 {
            // Back to open
            cb.state.Store(int32(StateOpen))
            cb.lastFailure.Store(time.Now())
            return err
        }
        // Success in half-open
        if cb.successes.Add(1) >= int32(cb.config.HalfOpenRequests) {
            // Fully recovered
            cb.state.Store(int32(StateClosed))
            cb.failures.Store(0)
        }
        return nil
        
    case StateClosed:
        err := next.Process(ctx)
        if err != nil || ctx.Metrics.StatusCode >= 500 {
            fails := cb.failures.Add(1)
            if fails >= int32(cb.config.ErrorThreshold) {
                cb.state.Store(int32(StateOpen))
                cb.lastFailure.Store(time.Now())
            }
        } else {
            cb.failures.Store(0) // reset on success
        }
        return err
    }
    
    return next.Process(ctx)
}
```

---

## 15. Admin API Server

```go
// internal/admin/server.go

type Server struct {
    engine     *engine.Engine
    httpServer *http.Server
    mux        *http.ServeMux
    auth       Authenticator
    logger     *logging.Logger
    
    // WebSocket hub for real-time updates
    wsHub      *WebSocketHub
}

func NewServer(engine *engine.Engine, cfg *config.AdminConfig) *Server {
    s := &Server{
        engine: engine,
        mux:    http.NewServeMux(),
        wsHub:  NewWebSocketHub(),
    }
    
    // Register routes
    // System
    s.mux.HandleFunc("GET /api/v1/system/info", s.auth.Wrap(s.handleSystemInfo))
    s.mux.HandleFunc("GET /api/v1/system/health", s.handleSystemHealth) // no auth
    s.mux.HandleFunc("POST /api/v1/system/reload", s.auth.Wrap(s.handleReload))
    
    // Backends
    s.mux.HandleFunc("GET /api/v1/backends", s.auth.Wrap(s.handleListBackends))
    s.mux.HandleFunc("GET /api/v1/backends/{pool}", s.auth.Wrap(s.handleGetPool))
    s.mux.HandleFunc("POST /api/v1/backends/{pool}", s.auth.Wrap(s.handleAddBackend))
    s.mux.HandleFunc("DELETE /api/v1/backends/{pool}/{backend}", s.auth.Wrap(s.handleRemoveBackend))
    s.mux.HandleFunc("PATCH /api/v1/backends/{pool}/{backend}", s.auth.Wrap(s.handleUpdateBackend))
    s.mux.HandleFunc("POST /api/v1/backends/{pool}/{backend}/drain", s.auth.Wrap(s.handleDrainBackend))
    
    // Routes
    s.mux.HandleFunc("GET /api/v1/routes", s.auth.Wrap(s.handleListRoutes))
    s.mux.HandleFunc("POST /api/v1/routes", s.auth.Wrap(s.handleAddRoute))
    s.mux.HandleFunc("PUT /api/v1/routes/{name}", s.auth.Wrap(s.handleUpdateRoute))
    s.mux.HandleFunc("DELETE /api/v1/routes/{name}", s.auth.Wrap(s.handleDeleteRoute))
    
    // Metrics
    s.mux.HandleFunc("GET /api/v1/metrics", s.auth.Wrap(s.handleMetrics))
    s.mux.HandleFunc("GET /api/v1/metrics/summary", s.auth.Wrap(s.handleMetricsSummary))
    s.mux.HandleFunc("GET /api/v1/metrics/timeseries", s.auth.Wrap(s.handleTimeseries))
    s.mux.HandleFunc("GET /metrics", s.handlePrometheus) // no auth for Prometheus scrape
    
    // Health
    s.mux.HandleFunc("GET /api/v1/health", s.auth.Wrap(s.handleHealth))
    
    // Certificates
    s.mux.HandleFunc("GET /api/v1/certs", s.auth.Wrap(s.handleListCerts))
    s.mux.HandleFunc("POST /api/v1/certs/{domain}/renew", s.auth.Wrap(s.handleRenewCert))
    
    // WebSocket
    s.mux.HandleFunc("GET /api/v1/ws/metrics", s.auth.WrapWS(s.handleWSMetrics))
    s.mux.HandleFunc("GET /api/v1/ws/logs", s.auth.WrapWS(s.handleWSLogs))
    s.mux.HandleFunc("GET /api/v1/ws/events", s.auth.WrapWS(s.handleWSEvents))
    
    // Web UI (embedded)
    s.mux.Handle("GET /ui/", http.StripPrefix("/ui/", 
        http.FileServer(http.FS(webui.Assets))))
    
    return s
}

// WebSocket Hub for broadcasting real-time updates
type WebSocketHub struct {
    mu          sync.RWMutex
    connections map[*websocket.Conn]map[string]bool // conn → subscribed channels
}

func (h *WebSocketHub) Broadcast(channel string, data interface{}) {
    msg, _ := json.Marshal(map[string]interface{}{
        "channel": channel,
        "data":    data,
        "ts":      time.Now().Unix(),
    })
    
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    for conn, subs := range h.connections {
        if subs[channel] || subs["*"] {
            conn.Write(msg) // fire and forget, remove on error
        }
    }
}
```

---

## 16. Web UI Implementation

### 16.1 Embedding

```go
// internal/webui/embed.go

import "embed"

//go:embed assets/*
var Assets embed.FS
```

### 16.2 SPA Architecture (Vanilla JS)

```javascript
// web/src/app.js

// Minimal SPA framework — no React, no Vue, just vanilla JS.
// ~5KB total framework code.

class App {
    constructor() {
        this.routes = {};
        this.currentRoute = null;
        this.ws = null;
        this.state = {};
    }
    
    // Client-side router (hash-based)
    route(path, component) {
        this.routes[path] = component;
    }
    
    navigate(path) {
        window.location.hash = '#' + path;
    }
    
    start() {
        window.addEventListener('hashchange', () => this.render());
        this.connectWebSocket();
        this.render();
    }
    
    render() {
        const path = window.location.hash.slice(1) || '/';
        const component = this.routes[path] || this.routes['/'];
        const container = document.getElementById('app');
        if (component) {
            component.render(container, this);
        }
    }
    
    // WebSocket connection with auto-reconnect
    connectWebSocket() {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${protocol}//${location.host}/api/v1/ws/events`);
        
        this.ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            this.handleWSMessage(msg);
        };
        
        this.ws.onclose = () => {
            setTimeout(() => this.connectWebSocket(), 3000);
        };
    }
}

// Usage:
const app = new App();
app.route('/', new DashboardComponent());
app.route('/backends', new BackendsComponent());
app.route('/routes', new RoutesComponent());
app.route('/metrics', new MetricsComponent());
app.route('/logs', new LogsComponent());
app.route('/config', new ConfigComponent());
app.route('/certs', new CertsComponent());
app.route('/cluster', new ClusterComponent());
app.start();
```

### 16.3 Chart Library (From Scratch)

```javascript
// web/src/lib/chart.js

// Minimal canvas-based chart library.
// Supports: line, bar, area, gauge, sparkline.
// ~3KB minified.

class Chart {
    constructor(canvas, options = {}) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.options = {
            padding: { top: 20, right: 20, bottom: 30, left: 50 },
            colors: ['#10b981', '#3b82f6', '#f59e0b', '#ef4444'],
            animate: true,
            gridLines: true,
            ...options
        };
    }
    
    // Line chart with smooth curves
    line(data, labels) {
        // Implementation: Canvas moveTo/lineTo with bezier curves
        // Grid lines, Y-axis labels, X-axis labels
        // Tooltip on hover
    }
    
    // Sparkline (minimal line, no axes)
    sparkline(data, color) {
        const { width, height } = this.canvas;
        const max = Math.max(...data);
        const min = Math.min(...data);
        const range = max - min || 1;
        const step = width / (data.length - 1);
        
        this.ctx.beginPath();
        this.ctx.strokeStyle = color || this.options.colors[0];
        this.ctx.lineWidth = 2;
        
        data.forEach((val, i) => {
            const x = i * step;
            const y = height - ((val - min) / range) * (height * 0.8) - height * 0.1;
            if (i === 0) this.ctx.moveTo(x, y);
            else this.ctx.lineTo(x, y);
        });
        
        this.ctx.stroke();
    }
    
    // Gauge (semicircle)
    gauge(value, max, label) {
        // Arc from π to 2π, filled to value/max percentage
        // Color gradient: green → yellow → red
    }
    
    // Bar chart
    bar(data, labels) {
        // Vertical bars with labels
        // Support stacked bars
    }
}
```

---

## 17. CLI Implementation

### 17.1 Argument Parser

```go
// internal/cli/parser.go

// Custom argument parser — no cobra, no urfave.
// Supports:
// - Commands and subcommands: olb backend list
// - Long flags: --format json
// - Short flags: -f json
// - Boolean flags: --json, --verbose, -v
// - Flag values: --timeout 30s, --timeout=30s

type Command struct {
    Name        string
    Description string
    Usage       string
    Flags       []*Flag
    SubCommands []*Command
    Run         func(args []string, flags FlagSet) error
}

type Flag struct {
    Name     string
    Short    string
    Usage    string
    Default  string
    Required bool
    IsBool   bool
}

type FlagSet map[string]string

func (fs FlagSet) String(name string) string     { return fs[name] }
func (fs FlagSet) Int(name string) int            { n, _ := strconv.Atoi(fs[name]); return n }
func (fs FlagSet) Bool(name string) bool          { return fs[name] == "true" }
func (fs FlagSet) Duration(name string) time.Duration {
    d, _ := time.ParseDuration(fs[name]); return d
}

func Parse(root *Command, args []string) error {
    // 1. Find command/subcommand by walking args
    // 2. Parse flags from remaining args
    // 3. Validate required flags
    // 4. Apply defaults
    // 5. Call Run function
    return nil
}
```

### 17.2 TUI Engine for `olb top`

```go
// internal/cli/tui/tui.go

// Terminal UI engine using ANSI escape codes.
// No external TUI library — raw terminal manipulation.

type TUI struct {
    width    int
    height   int
    buf      bytes.Buffer
    oldState *term.State
}

func (t *TUI) Init() error {
    // Put terminal in raw mode
    oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
    if err != nil {
        return err
    }
    t.oldState = oldState
    
    // Get terminal size
    t.width, t.height, _ = term.GetSize(int(os.Stdin.Fd()))
    
    // Hide cursor
    fmt.Print("\033[?25l")
    // Clear screen
    fmt.Print("\033[2J")
    // Move to top-left
    fmt.Print("\033[H")
    
    return nil
}

func (t *TUI) Cleanup() {
    // Show cursor
    fmt.Print("\033[?25h")
    // Reset terminal
    if t.oldState != nil {
        term.Restore(int(os.Stdin.Fd()), t.oldState)
    }
}

// Drawing primitives
func (t *TUI) MoveTo(x, y int)           { fmt.Fprintf(&t.buf, "\033[%d;%dH", y, x) }
func (t *TUI) SetColor(fg, bg int)       { fmt.Fprintf(&t.buf, "\033[%d;%dm", fg, bg) }
func (t *TUI) Reset()                    { t.buf.WriteString("\033[0m") }
func (t *TUI) Bold()                     { t.buf.WriteString("\033[1m") }

func (t *TUI) DrawBox(x, y, w, h int, title string) {
    // Unicode box drawing: ┌─┐│└─┘
    t.MoveTo(x, y)
    t.buf.WriteString("┌")
    if title != "" {
        t.buf.WriteString("─ ")
        t.Bold()
        t.buf.WriteString(title)
        t.Reset()
        t.buf.WriteString(" ")
        for i := 0; i < w-len(title)-5; i++ { t.buf.WriteString("─") }
    } else {
        for i := 0; i < w-2; i++ { t.buf.WriteString("─") }
    }
    t.buf.WriteString("┐")
    // ... sides and bottom
}

func (t *TUI) DrawProgressBar(x, y, width int, value, max float64, color int) {
    filled := int(value / max * float64(width))
    t.MoveTo(x, y)
    t.SetColor(color, 40) // foreground color, black bg
    for i := 0; i < filled; i++ { t.buf.WriteString("█") }
    t.SetColor(37, 40) // gray
    for i := filled; i < width; i++ { t.buf.WriteString("░") }
    t.Reset()
}

func (t *TUI) Flush() {
    os.Stdout.Write(t.buf.Bytes())
    t.buf.Reset()
}
```

---

## 18. Cluster: Gossip Protocol

```go
// internal/cluster/gossip/gossip.go

// SWIM (Scalable Weakly-consistent Infection-style Membership) protocol.
// Used for: membership, health propagation, eventually-consistent state.

type Gossip struct {
    mu          sync.RWMutex
    self        *Member
    members     map[string]*Member
    broadcasts  *BroadcastQueue
    
    udpListener *net.UDPConn
    tcpListener net.Listener
    
    probeInterval time.Duration // Default: 1s
    probeTimeout  time.Duration // Default: 500ms
    suspectTimeout time.Duration // Default: 5s
    
    config      GossipConfig
    logger      *logging.Logger
}

type Member struct {
    Name     string
    Address  string
    State    MemberState // Alive, Suspect, Dead
    Metadata map[string]string
    
    incarnation uint32 // Lamport-like timestamp for state
    lastContact time.Time
}

type MemberState int

const (
    MemberAlive   MemberState = 0
    MemberSuspect MemberState = 1
    MemberDead    MemberState = 2
)

// Protocol loop:
// Every probeInterval:
// 1. Pick a random member to probe
// 2. Send PING via UDP
// 3. If no ACK within probeTimeout:
//    a. Pick K random members, send PING-REQ (indirect probe)
//    b. If no ACK from indirect: mark SUSPECT
// 4. Piggyback broadcast messages on PING/ACK/PING-REQ

type MessageType byte

const (
    MsgPing    MessageType = iota
    MsgAck
    MsgPingReq
    MsgSuspect
    MsgAlive
    MsgDead
    MsgUser    // Application-level broadcasts
)

type Message struct {
    Type        MessageType
    SeqNo       uint32
    Source       string
    Target       string
    Incarnation uint32
    Payload     []byte // For user messages
}

func (g *Gossip) probeLoop() {
    ticker := time.NewTicker(g.probeInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            target := g.randomMember()
            if target == nil {
                continue
            }
            g.probe(target)
        case <-g.ctx.Done():
            return
        }
    }
}

func (g *Gossip) probe(target *Member) {
    // 1. Send PING with piggybacked broadcasts
    seq := g.nextSeqNo()
    broadcasts := g.broadcasts.GetPending(5) // piggyback up to 5 messages
    
    msg := &Message{
        Type:   MsgPing,
        SeqNo:  seq,
        Source: g.self.Name,
        Target: target.Name,
    }
    
    if g.sendUDP(target.Address, msg, broadcasts) {
        return // ACK received
    }
    
    // 2. No ACK — indirect probe via K random members
    indirectTargets := g.randomMembers(3) // K=3
    ackCh := make(chan bool, len(indirectTargets))
    
    for _, m := range indirectTargets {
        go func(m *Member) {
            reqMsg := &Message{
                Type:   MsgPingReq,
                SeqNo:  seq,
                Source: g.self.Name,
                Target: target.Name,
            }
            ackCh <- g.sendTCP(m.Address, reqMsg)
        }(m)
    }
    
    // Wait for any ACK or timeout
    timer := time.NewTimer(g.probeTimeout)
    for i := 0; i < len(indirectTargets); i++ {
        select {
        case ack := <-ackCh:
            if ack {
                timer.Stop()
                return // Indirect ACK received
            }
        case <-timer.C:
            // 3. No ACK — mark suspect
            g.suspect(target)
            return
        }
    }
    
    g.suspect(target)
}

// Broadcast queue: messages are sent multiple times (fanout)
// until they reach all members (infection-style dissemination).
type BroadcastQueue struct {
    mu    sync.Mutex
    items []*broadcastItem
}

type broadcastItem struct {
    message  []byte
    transmit int // remaining transmissions
}
```

---

## 19. Cluster: Raft Consensus

```go
// internal/cluster/raft/raft.go

// Full Raft consensus implementation per the extended Raft paper.
// Used for: config replication, leader election.

type Raft struct {
    mu sync.Mutex
    
    // Persistent state
    currentTerm uint64
    votedFor    string
    log         *Log
    
    // Volatile state
    state       RaftState
    commitIndex uint64
    lastApplied uint64
    
    // Leader state
    nextIndex   map[string]uint64  // peer → next log index to send
    matchIndex  map[string]uint64  // peer → highest replicated index
    
    // Config
    self        string
    peers       []string
    
    // Channels
    applyCh     chan LogEntry      // committed entries to apply
    
    // Timers
    electionTimer  *time.Timer
    heartbeatTimer *time.Timer
    
    // Transport
    transport Transport
    
    // State machine
    fsm StateMachine
    
    logger *logging.Logger
}

type RaftState int

const (
    Follower  RaftState = 0
    Candidate RaftState = 1
    Leader    RaftState = 2
)

// StateMachine is applied when log entries are committed.
// For OLB, the state machine IS the config store.
type StateMachine interface {
    Apply(entry LogEntry) interface{}
    Snapshot() ([]byte, error)
    Restore(data []byte) error
}

// RequestVote RPC
type RequestVoteArgs struct {
    Term         uint64
    CandidateID  string
    LastLogIndex uint64
    LastLogTerm  uint64
}

type RequestVoteReply struct {
    Term        uint64
    VoteGranted bool
}

// AppendEntries RPC (heartbeat + log replication)
type AppendEntriesArgs struct {
    Term         uint64
    LeaderID     string
    PrevLogIndex uint64
    PrevLogTerm  uint64
    Entries      []LogEntry
    LeaderCommit uint64
}

type AppendEntriesReply struct {
    Term    uint64
    Success bool
    
    // Optimization: tell leader where to backtrack
    ConflictTerm  uint64
    ConflictIndex uint64
}

// Election logic
func (r *Raft) runElection() {
    r.mu.Lock()
    r.state = Candidate
    r.currentTerm++
    r.votedFor = r.self
    term := r.currentTerm
    lastLogIndex, lastLogTerm := r.log.LastIndexTerm()
    r.mu.Unlock()
    
    votes := int32(1) // vote for self
    
    for _, peer := range r.peers {
        go func(peer string) {
            args := &RequestVoteArgs{
                Term:         term,
                CandidateID:  r.self,
                LastLogIndex: lastLogIndex,
                LastLogTerm:  lastLogTerm,
            }
            
            reply, err := r.transport.RequestVote(peer, args)
            if err != nil {
                return
            }
            
            r.mu.Lock()
            defer r.mu.Unlock()
            
            if reply.Term > r.currentTerm {
                r.stepDown(reply.Term)
                return
            }
            
            if reply.VoteGranted && r.state == Candidate && r.currentTerm == term {
                if atomic.AddInt32(&votes, 1) > int32(len(r.peers)/2) {
                    r.becomeLeader()
                }
            }
        }(peer)
    }
}

// Leader heartbeat and log replication
func (r *Raft) leaderLoop() {
    r.heartbeatTimer.Reset(50 * time.Millisecond)
    
    for r.state == Leader {
        select {
        case <-r.heartbeatTimer.C:
            r.sendHeartbeats()
            r.heartbeatTimer.Reset(50 * time.Millisecond)
        case <-r.ctx.Done():
            return
        }
    }
}

func (r *Raft) sendHeartbeats() {
    for _, peer := range r.peers {
        go r.replicateLog(peer)
    }
}

func (r *Raft) replicateLog(peer string) {
    r.mu.Lock()
    nextIdx := r.nextIndex[peer]
    prevLogIndex := nextIdx - 1
    prevLogTerm := r.log.Term(prevLogIndex)
    entries := r.log.Entries(nextIdx, nextIdx+100) // batch up to 100
    leaderCommit := r.commitIndex
    term := r.currentTerm
    r.mu.Unlock()
    
    args := &AppendEntriesArgs{
        Term:         term,
        LeaderID:     r.self,
        PrevLogIndex: prevLogIndex,
        PrevLogTerm:  prevLogTerm,
        Entries:      entries,
        LeaderCommit: leaderCommit,
    }
    
    reply, err := r.transport.AppendEntries(peer, args)
    if err != nil {
        return
    }
    
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if reply.Term > r.currentTerm {
        r.stepDown(reply.Term)
        return
    }
    
    if reply.Success {
        r.nextIndex[peer] = nextIdx + uint64(len(entries))
        r.matchIndex[peer] = r.nextIndex[peer] - 1
        r.advanceCommitIndex()
    } else {
        // Backtrack using conflict info
        if reply.ConflictTerm > 0 {
            r.nextIndex[peer] = reply.ConflictIndex
        } else {
            r.nextIndex[peer]--
        }
    }
}
```

---

## 20. MCP Server

```go
// internal/mcp/server.go

// MCP (Model Context Protocol) server implementation.
// Supports stdio and HTTP/SSE transports.

type Server struct {
    engine   *engine.Engine
    tools    map[string]Tool
    resources map[string]Resource
    prompts  map[string]Prompt
    
    transport Transport
}

type Tool struct {
    Name        string
    Description string
    InputSchema json.RawMessage
    Handler     func(input json.RawMessage) (interface{}, error)
}

func NewServer(engine *engine.Engine) *Server {
    s := &Server{
        engine:    engine,
        tools:     make(map[string]Tool),
        resources: make(map[string]Resource),
    }
    
    // Register tools
    s.registerTool("olb_query_metrics", "Query load balancer metrics", s.handleQueryMetrics)
    s.registerTool("olb_list_backends", "List backend pools and status", s.handleListBackends)
    s.registerTool("olb_modify_backend", "Add/remove/drain backends", s.handleModifyBackend)
    s.registerTool("olb_modify_route", "Add/update/remove routes", s.handleModifyRoute)
    s.registerTool("olb_diagnose", "Diagnose issues and anomalies", s.handleDiagnose)
    s.registerTool("olb_get_logs", "Search and retrieve logs", s.handleGetLogs)
    s.registerTool("olb_get_config", "Get current configuration", s.handleGetConfig)
    s.registerTool("olb_cluster_status", "Get cluster status", s.handleClusterStatus)
    
    return s
}

// handleDiagnose is the AI-native diagnostic tool.
func (s *Server) handleDiagnose(input json.RawMessage) (interface{}, error) {
    var params struct {
        Type   string `json:"type"`
        Target string `json:"target"`
        Range  string `json:"range"`
    }
    json.Unmarshal(input, &params)
    
    diagnosis := &Diagnosis{
        Timestamp: time.Now(),
        Type:     params.Type,
        Target:   params.Target,
    }
    
    switch params.Type {
    case "errors":
        // Analyze error patterns
        diagnosis.Findings = s.analyzeErrors(params.Target, params.Range)
    case "latency":
        // Analyze latency anomalies
        diagnosis.Findings = s.analyzeLatency(params.Target, params.Range)
    case "health":
        // Check backend health
        diagnosis.Findings = s.analyzeHealth(params.Target)
    case "capacity":
        // Capacity planning
        diagnosis.Findings = s.analyzeCapacity(params.Target)
    case "full":
        // Comprehensive diagnosis
        diagnosis.Findings = append(diagnosis.Findings, s.analyzeErrors(params.Target, params.Range)...)
        diagnosis.Findings = append(diagnosis.Findings, s.analyzeLatency(params.Target, params.Range)...)
        diagnosis.Findings = append(diagnosis.Findings, s.analyzeHealth(params.Target)...)
        diagnosis.Findings = append(diagnosis.Findings, s.analyzeCapacity(params.Target)...)
    }
    
    return diagnosis, nil
}

// Stdio transport: reads JSON-RPC from stdin, writes to stdout
type StdioTransport struct {
    reader *bufio.Reader
    writer *bufio.Writer
}

func (t *StdioTransport) Run(server *Server) error {
    for {
        line, err := t.reader.ReadBytes('\n')
        if err != nil {
            return err
        }
        
        var req JSONRPCRequest
        if err := json.Unmarshal(line, &req); err != nil {
            t.sendError(-1, -32700, "Parse error")
            continue
        }
        
        resp := server.handleRequest(&req)
        data, _ := json.Marshal(resp)
        t.writer.Write(data)
        t.writer.WriteByte('\n')
        t.writer.Flush()
    }
}
```

---

## 21. Plugin System

```go
// internal/plugin/plugin.go

type Manager struct {
    plugins  map[string]Plugin
    api      *pluginAPI
    logger   *logging.Logger
}

// pluginAPI is the surface exposed to plugins.
type pluginAPI struct {
    engine *engine.Engine
}

func (api *pluginAPI) RegisterMiddleware(name string, factory MiddlewareFactory) error {
    return api.engine.MiddlewareRegistry().Register(name, factory)
}

func (api *pluginAPI) RegisterBalancer(name string, factory BalancerFactory) error {
    return api.engine.BalancerRegistry().Register(name, factory)
}

func (api *pluginAPI) Metrics() *metrics.Engine {
    return api.engine.Metrics()
}

func (api *pluginAPI) Logger() *logging.Logger {
    return api.engine.Logger().With(logging.String("component", "plugin"))
}

// Load plugins from directory
func (m *Manager) LoadAll(dir string) error {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return err
    }
    
    for _, entry := range entries {
        if !strings.HasSuffix(entry.Name(), ".so") {
            continue
        }
        
        path := filepath.Join(dir, entry.Name())
        p, err := plugin.Open(path)
        if err != nil {
            m.logger.Error("plugin load failed",
                logging.String("path", path), logging.Error(err))
            continue
        }
        
        sym, err := p.Lookup("Plugin")
        if err != nil {
            m.logger.Error("plugin symbol not found",
                logging.String("path", path), logging.Error(err))
            continue
        }
        
        plug, ok := sym.(Plugin)
        if !ok {
            m.logger.Error("plugin wrong type", logging.String("path", path))
            continue
        }
        
        if err := plug.Init(m.api); err != nil {
            m.logger.Error("plugin init failed",
                logging.String("name", plug.Name()), logging.Error(err))
            continue
        }
        
        m.plugins[plug.Name()] = plug
        m.logger.Info("plugin loaded",
            logging.String("name", plug.Name()),
            logging.String("version", plug.Version()))
    }
    
    return nil
}
```

---

## 22. Engine Orchestrator

```go
// internal/engine/engine.go

func New(cfg *config.Config) (*Engine, error) {
    e := &Engine{
        config: cfg,
    }
    
    // 1. Logger
    e.logger = logging.New(cfg.Global.Log)
    e.logger.Info("initializing OpenLoadBalancer",
        logging.String("version", version.Version))
    
    // 2. Metrics
    e.metrics = metrics.NewEngine()
    
    // 3. TLS
    e.tlsManager = tls.NewManager(e.logger, e.metrics)
    if err := e.tlsManager.LoadCertificates(cfg); err != nil {
        return nil, fmt.Errorf("tls init: %w", err)
    }
    
    // 4. Backend pools
    e.backends = backend.NewPoolManager(e.logger, e.metrics)
    for _, poolCfg := range cfg.Backends {
        if err := e.backends.AddPool(poolCfg); err != nil {
            return nil, fmt.Errorf("backend pool %s: %w", poolCfg.Name, err)
        }
    }
    
    // 5. Health checker
    e.healthCheck = health.NewChecker(e.backends, e.logger, e.metrics)
    
    // 6. Router
    e.router = router.New(e.logger)
    for _, routeCfg := range cfg.Routes {
        if err := e.router.AddRoute(routeCfg); err != nil {
            return nil, fmt.Errorf("route %s: %w", routeCfg.Name, err)
        }
    }
    
    // 7. Connection manager
    e.connManager = conn.NewManager(cfg.Global.MaxConnections, e.metrics, e.logger)
    
    // 8. Build listeners
    for _, listenerCfg := range cfg.Listeners {
        l, err := listener.New(listenerCfg, e)
        if err != nil {
            return nil, fmt.Errorf("listener %s: %w", listenerCfg.Name, err)
        }
        e.listeners = append(e.listeners, l)
    }
    
    // 9. Admin API
    e.adminServer = admin.NewServer(e, cfg.Admin)
    
    // 10. Cluster (optional)
    if cfg.Cluster.Enabled {
        e.cluster = cluster.NewManager(cfg.Cluster, e.logger)
    }
    
    // 11. MCP (optional)
    if cfg.MCP.Enabled {
        e.mcpServer = mcp.NewServer(e)
    }
    
    return e, nil
}

func (e *Engine) Start() error {
    e.state.Store(int32(StateStarting))
    e.startTime = time.Now()
    e.ctx, e.cancel = context.WithCancel(context.Background())
    
    // Start health checks
    e.healthCheck.Start()
    
    // Start listeners
    for _, l := range e.listeners {
        e.wg.Add(1)
        go func(l listener.Listener) {
            defer e.wg.Done()
            if err := l.Start(e.ctx); err != nil {
                e.logger.Error("listener failed",
                    logging.String("name", l.Name()),
                    logging.Error(err))
            }
        }(l)
    }
    
    // Start admin API
    e.wg.Add(1)
    go func() {
        defer e.wg.Done()
        e.adminServer.Start(e.ctx)
    }()
    
    // Start cluster
    if e.cluster != nil {
        e.wg.Add(1)
        go func() {
            defer e.wg.Done()
            e.cluster.Start(e.ctx)
        }()
    }
    
    // Start MCP
    if e.mcpServer != nil {
        e.wg.Add(1)
        go func() {
            defer e.wg.Done()
            e.mcpServer.Start(e.ctx)
        }()
    }
    
    // Install signal handlers
    e.installSignalHandlers()
    
    e.state.Store(int32(StateRunning))
    
    // Log ready
    addrs := make([]string, 0, len(e.listeners))
    for _, l := range e.listeners {
        addrs = append(addrs, l.Address())
    }
    e.logger.Info("OpenLoadBalancer ready",
        logging.String("version", version.Version),
        logging.Strings("listeners", addrs),
        logging.String("admin", e.adminServer.Address()))
    
    return nil
}

func (e *Engine) Shutdown(ctx context.Context) error {
    e.state.Store(int32(StateDraining))
    e.logger.Info("shutting down")
    
    // 1. Stop accepting new connections
    for _, l := range e.listeners {
        l.Stop()
    }
    
    // 2. Drain existing connections
    drainCtx, cancel := context.WithTimeout(ctx, e.config.Global.GracefulTimeout)
    defer cancel()
    e.connManager.Drain(drainCtx.Err())
    
    // 3. Stop components
    e.state.Store(int32(StateStopping))
    e.cancel()
    e.wg.Wait()
    
    // 4. Flush metrics and logs
    e.metrics.Flush()
    e.logger.Flush()
    
    e.state.Store(int32(StateStopped))
    e.logger.Info("shutdown complete")
    return nil
}
```

---

## 23. Build, Test & Release

### 23.1 Test Organization

```bash
# Unit tests (per package)
go test ./internal/config/parser/ -v -race
go test ./internal/balancer/ -v -race
go test ./internal/router/ -v -race

# Integration tests
go test ./test/integration/ -v -race -tags=integration

# Benchmark
go test -bench=BenchmarkProxy -benchmem ./internal/proxy/l7/
go test -bench=BenchmarkRouter -benchmem ./internal/router/
go test -bench=BenchmarkBalancer -benchmem ./internal/balancer/

# Fuzz
go test -fuzz=FuzzYAMLParser -fuzztime=60s ./internal/config/parser/
go test -fuzz=FuzzSNIParse -fuzztime=60s ./internal/proxy/l4/

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Race detection
go test -race ./...
```

### 23.2 CI Pipeline

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go vet ./...
      - run: go test -race -count=1 ./...
      - run: go test -bench=. -benchtime=1s ./...
      
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: make build-all
      - uses: actions/upload-artifact@v4
        with:
          name: binaries
          path: bin/
```

### 23.3 Release Checklist

1. All tests pass with `-race`
2. No `go vet` warnings
3. Benchmark comparison with previous version
4. Binary size check (< 20MB)
5. Startup time check (< 500ms)
6. Config validation works for all formats
7. Hot reload works
8. Graceful shutdown works
9. Web UI loads and connects WebSocket
10. CLI commands work
11. Docker image builds and runs
12. CHANGELOG.md updated
13. Tag and push

---

## 24. Performance Optimization Playbook

### 24.1 Hot Path Optimizations

1. **Buffer pooling**: ALL temporary buffers from `sync.Pool`
2. **Zero-alloc logging**: Level check before any allocation
3. **Atomic metrics**: No mutexes on request counting hot path
4. **Pre-compiled regex**: Compile route patterns once at config load
5. **Connection reuse**: Aggressive pooling, keep-alive by default
6. **Radix trie routing**: O(k) path matching, no regex on hot path
7. **Response writer wrapper**: Buffer headers, write all at once

### 24.2 Memory Optimizations

1. **Object pooling**: Pool `RequestContext`, `ResponseWriter`, buffers
2. **Minimize allocations**: Pre-allocate slices, reuse maps
3. **String interning**: Common header names, status texts
4. **Compact struct layouts**: Order fields to minimize padding

### 24.3 I/O Optimizations

1. **splice() on Linux**: Zero-copy TCP proxy
2. **writev()**: Vectored writes for headers + body
3. **TCP_NODELAY**: Disable Nagle for low-latency
4. **SO_REUSEPORT**: Multiple listener goroutines per port (Linux)
5. **Buffered writers**: bufio.Writer for all network I/O

### 24.4 Profiling Commands

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=BenchmarkProxy ./internal/proxy/l7/
go tool pprof -http=:8080 cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench=BenchmarkProxy ./internal/proxy/l7/
go tool pprof -http=:8080 mem.prof

# Trace
go test -trace=trace.out -bench=BenchmarkProxy ./internal/proxy/l7/
go tool trace trace.out

# Live profiling (admin API)
# GET /debug/pprof/  (when admin.debug=true)
```

---

*This implementation guide is the technical roadmap for building OpenLoadBalancer. Each section provides enough detail to write the actual code. Cross-reference SPECIFICATION.md for requirements and TASKS.md for work items.*
