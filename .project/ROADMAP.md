# Project Roadmap

> Based on comprehensive codebase analysis performed on 2026-04-14
> This roadmap prioritizes work needed to bring the project to production quality.

## Current State Assessment

OpenLoadBalancer is a remarkably complete project — 99.7% spec completion, 95.3% test coverage, 178K LOC of Go, 13.9K LOC of TypeScript, all building into a single 9.1MB binary. Every major subsystem from the specification (L4/L7 proxy, 16 algorithms, Raft clustering, ACME, WAF, MCP, Web UI) is implemented and tested.

**Key blockers for production readiness:**
- 15 test failures in the core proxy layer (`internal/proxy/l4` and `internal/proxy/l7`)
- From-scratch Raft consensus has not been validated under chaos conditions
- Frontend has architectural inconsistencies (dual data-fetching layers)

**What's working well:**
- Clean builds with zero warnings
- `go vet` passes clean
- 63 of 65 packages pass all tests
- Comprehensive security hardening (97 findings addressed)
- Extensive documentation and deployment tooling
- Multi-platform binary, Docker, Helm, Terraform all ready

---

## Phase 1: Critical Fixes (Week 1)

### Must-fix items blocking basic functionality

- [ ] **Fix 15 failing proxy tests** — `internal/proxy/l7/proxy_test.go` (12 failures), `internal/proxy/l4` (3 failures)
  - Symptoms: Expected 502/503 getting 404, transport creation errors, SSE backend errors
  - Root cause analysis needed: likely a regression from recent refactoring
  - Effort: 8-16h
  - Priority: CRITICAL — these are the core proxy path

- [ ] **Diagnose `TestCreateTransport_*` failures** — `proxy_test.go:2174, 2200`
  - "expected error dialing invalid address" — transport creation is succeeding when it should fail
  - Effort: 2-4h

- [ ] **Diagnose retry test failures** — `proxy_test.go:648, 1433, 1477`
  - Retry mechanism not selecting alternate backends; returning 404 instead
  - Effort: 4-8h

- [ ] **Fix SSE backend error tests** — `sse_test.go:728, 1276`
  - Backend connection failures not propagating correctly
  - Effort: 2-4h

---

## Phase 2: Core Hardening (Week 2-3)

### Reliability and correctness

- [ ] **Validate Raft under failure scenarios** — `internal/cluster/`
  - Network partition simulation (kill leader, verify re-election)
  - Split-brain protection verification with 5+ nodes
  - Log compaction under sustained write load
  - Membership change during leader failure
  - Effort: 40h

- [ ] **Add chaos testing framework** — `test/chaos/`
  - Network partition injection
  - Random node kills
  - Clock skew simulation
  - Effort: 24h

- [ ] **Race detection on Linux CI** — `.github/workflows/ci.yml`
  - Add `go test -race ./...` job (requires CGO)
  - Already has `build-race` Makefile target
  - Effort: 2h

- [ ] **Fix cluster test TODO** — `internal/cluster/cluster_test.go:534`
  - "the actual RPC is a TODO/no-op" — implement or remove the no-op path
  - Effort: 2h

- [ ] **Increase `internal/plugin` test coverage** — currently 85.2%, target ≥90%
  - Add tests for plugin lifecycle, event bus, error paths
  - Effort: 8h

- [ ] **Increase `internal/engine` test coverage** — currently 87.8%, target ≥90%
  - Focus on error paths, concurrent reload, rollback scenarios
  - Effort: 8h

---

## Phase 3: Frontend Cleanup (Week 3-4)

### Resolve architectural inconsistencies

- [ ] **Consolidate data-fetching layer** — `internal/webui/src/hooks/use-query.ts`
  - Option A: Migrate custom hooks to use TanStack React Query hooks (recommended)
  - Option B: Remove TanStack React Query dependency entirely
  - Current state: QueryClientProvider wraps app but hooks don't use it
  - Effort: 8h

- [ ] **Remove unused `recharts` dependency** — `internal/webui/package.json`
  - Charts use custom SVG; recharts is never imported
  - Effort: 5min

- [ ] **Add focus trap to mobile sidebar** — `internal/webui/src/components/layout.tsx`
  - Accessibility: keyboard users can tab out of open sidebar into background
  - Effort: 2h

- [ ] **Add frontend integration tests** — `internal/webui/src/`
  - Test form submissions that call API mutations
  - Test SSE event stream rendering
  - Test error boundary behavior
  - Effort: 16h

- [ ] **Add marketing website tests** — `website-new/`
  - No test infrastructure exists
  - At minimum: smoke tests for page rendering
  - Effort: 8h

---

## Phase 4: Performance & Optimization (Week 5-6)

### Performance tuning and validation

- [ ] **Run full benchmark suite** — `make bench`
  - Validate spec targets: HTTP RPS >50K single core, latency <1ms p99 L7
  - Compare against v0.1.0 baseline
  - Document results in `docs/benchmark-report.md`
  - Effort: 8h

- [ ] **Memory profiling under load**
  - Verify <4KB per idle connection, <32KB per active request
  - Profile with `go tool pprof` under sustained load
  - Identify and fix allocation hot spots
  - Effort: 16h

- [ ] **TCP throughput benchmark** — L4 proxy
  - Test with splice() on Linux
  - Target: >10Gbps throughput
  - Effort: 8h

- [ ] **Connection pool effectiveness audit**
  - Verify pool hit rate under production-like traffic patterns
  - Tune min idle / max idle parameters
  - Effort: 8h

- [ ] **Startup time benchmark**
  - Target: <500ms cold start to ready
  - Profile initialization sequence
  - Effort: 4h

---

## Phase 5: Documentation & Polish (Week 7-8)

### Documentation completeness and final polish

- [ ] **Create `llms.txt` at project root** — referenced in spec but missing
  - LLM-friendly project description for AI tooling
  - Effort: 1h

- [ ] **Add OpenAPI/Swagger spec** — `docs/api/openapi.yaml`
  - Full REST API reference in machine-readable format
  - Already partially exists; needs completion
  - Effort: 16h

- [ ] **Update CHANGELOG.md** — reflect all post-v0.1.0 changes
  - Tag v0.2.0, v0.3.0, v0.4.0 appropriately
  - Effort: 4h

- [ ] **Review and update all example configs** — `configs/`
  - Ensure examples match current config schema
  - Add examples for new features (GeoDNS, shadowing, WAF)
  - Effort: 4h

- [ ] **Production deployment guide** — `docs/production-deployment.md`
  - Already exists; verify accuracy against current implementation
  - Add troubleshooting for common issues
  - Effort: 8h

---

## Phase 6: Release Preparation (Week 9-10)

### Final production preparation

- [ ] **Resolve GHCR image publishing** — requires repo permissions
  - Only incomplete task in TASKS.md
  - Effort: 2h (permissions + workflow fix)

- [ ] **Multi-arch Docker image validation**
  - Test amd64 and arm64 images on real hardware
  - Verify frontend assets embedded correctly
  - Effort: 4h

- [ ] **End-to-end smoke test on fresh environment**
  - Install via curl | sh on clean Linux VM
  - Install via Docker on clean environment
  - Install via Homebrew on clean macOS
  - Verify `olb setup` → `olb start` → proxy traffic flow
  - Effort: 8h

- [ ] **Security scan of Docker image**
  - Run `trivy` or `grype` on final image
  - Fix any CVE findings
  - Effort: 4h

- [ ] **Cut v1.0.0 release**
  - Tag release
  - Build all platform binaries
  - Publish to GHCR, Homebrew
  - Create GitHub release with checksums
  - Effort: 4h

---

## Beyond v1.0: Future Enhancements

### Features and improvements for future versions

- [ ] **Brotli compression** — Pure Go implementation for middleware
- [ ] **QUIC/HTTP3 support** — Listed as future in spec §3
- [ ] **WASM plugin runtime** — Sandboxed alternative to Go plugins (spec §18.2)
- [ ] **RBAC for Admin API** — Read-only vs admin roles (spec §19.2)
- [ ] **Custom dashboard builder** — Drag-drop widget system (spec §14.2)
- [ ] **Config history** — Track and diff config changes over time (spec §14.6)
- [ ] **gRPC-Web full proxying** — Enhanced gRPC-Web support
- [ ] **Rate limiting distributed via Raft** — Use Raft log for strong consistency
- [ ] **Prometheus remote write** — Export metrics to remote Prometheus
- [ ] **HTTP/3 upstream proxying** — Proxy to HTTP/3 backends

---

## Effort Summary

| Phase | Estimated Hours | Priority | Dependencies |
|---|---|---|---|
| Phase 1: Critical Fixes | 16-32h | CRITICAL | None |
| Phase 2: Core Hardening | 84h | HIGH | Phase 1 |
| Phase 3: Frontend Cleanup | 34h | MEDIUM | None |
| Phase 4: Performance | 44h | MEDIUM | Phase 1 |
| Phase 5: Documentation | 33h | LOW | Phases 1-4 |
| Phase 6: Release Prep | 22h | HIGH | Phases 1-2 |
| **Total** | **~250h** | | |

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Proxy test failures indicate deeper proxy bug | Medium | High | Phase 1 root cause analysis before any deployment |
| Raft consensus has edge case bugs | Medium | Critical | Phase 2 chaos testing; start with single-node for prod |
| Performance targets not met under real load | Medium | Medium | Phase 4 benchmarking; tune before release |
| Frontend data-fetching inconsistency causes bugs | Low | Low | Phase 3 consolidation |
| Security vulnerability in from-scratch parsers | Low | High | Fuzz tests exist; add more coverage |
| Docker image has CVEs | Low | Medium | Phase 6 security scan |
