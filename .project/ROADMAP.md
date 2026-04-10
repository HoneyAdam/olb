# Project Roadmap

> Based on comprehensive codebase analysis performed on 2026-04-10
> This roadmap prioritizes work needed to bring the project to production quality.

## Current State Assessment

OpenLoadBalancer is in an **exceptionally mature state** for a project of its scope. The codebase implements ~99% of the specification across 400 Go files with 92% average test coverage. All core functionality — L4/L7 proxying, 16 balancer algorithms, clustering, WAF, MCP, Web UI, CLI — is implemented and tested. The project has been through a security audit with all 29 findings resolved.

**Key blockers for production readiness:** Minimal. The only substantive code issue is OAuth2 middleware test coverage at 27.8%. The rest is infrastructure/release operations.

**What's working well:**
- Extraordinary test depth (2.65:1 test-to-source ratio)
- Zero-dependency discipline maintained throughout
- Clean architecture with proper separation of concerns
- Comprehensive security hardening
- Full CI/CD pipeline with race detection, security scanning, and cross-platform builds

---

## Phase 1: Critical Fixes (Week 1)

### Must-fix items blocking production confidence

- [ ] **Raise OAuth2 middleware test coverage to 85%+** — `internal/middleware/oauth2/` currently at 27.8%. Add tests for: JWKS key fetching and caching, token introspection endpoint, OIDC discovery, ECDSA/RSA key validation, expired token rejection, audience validation. **Effort: ~4h**

- [ ] **Fix e2e test flakiness under parallel execution** — `test/e2e/` tests fail when run alongside other packages but pass individually. Root cause: port conflicts from parallel test execution. Fix: use isolated port ranges per test, or add `-p 1` for e2e package in CI. **Effort: ~2h**

- [ ] **Clean up webui-build-temp directory** — `internal/webui-build-temp/` contains stale build artifacts. Either add to `.gitignore` or remove and ensure build process doesn't create tracked artifacts. **Effort: ~15min**

- [ ] **Align Dockerfile VERSION with git tags** — Dockerfile reads `VERSION` file (`cat VERSION 2>/dev/null || echo '0.1.0'`) but project uses git tags via `git describe`. Update Dockerfile to use git-based version. **Effort: ~30min**

- [ ] **Fix CI frontend build duplication** — `.github/workflows/ci.yml` runs Node.js setup + `npm ci` + `npm run build` independently in 3 jobs (test, test-race, build). Refactor to share the frontend build artifact across jobs or create a dedicated `build-frontend` job. **Effort: ~2h**

- [ ] **Fix deploy.sh pnpm/npm inconsistency** — `scripts/deploy.sh` runs `pnpm install --frozen-lockfile` but project only has `npm` scripts and CI uses `npm ci`. Align to one package manager. **Effort: ~30min**

- [ ] **Add frontend build step to Dockerfile** — The Dockerfile doesn't include Node.js or a frontend build step. Add a Node.js build stage before the Go build so standalone `docker build` works without pre-built assets. **Effort: ~1h**

- [ ] **Refactor duplicated algorithm switch-case** — `internal/engine/engine.go` and `internal/engine/config.go` both have a 16-case algorithm name switch. Use the existing `balancer.Get()` registry instead. **Effort: ~1h**

## Phase 2: Release Operations (Week 1-2)

### Complete the v1.0.0 release process

- [ ] **Configure GHCR permissions for Docker push** — TASKS.md §5.7 notes "Docker images published (GHCR push needs repo permissions)". Set up GitHub Container Registry permissions and test `docker push` workflow. **Effort: ~1h**

- [ ] **Verify all cross-platform binaries** — Run `make build-all` and verify each binary starts correctly: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64. **Effort: ~1h**

- [ ] **Test Homebrew formula** — Verify `Formula/` directory has correct formula that installs and runs correctly on macOS. **Effort: ~30min**

- [ ] **Test install script** — Verify `scripts/` install script works on clean Linux and macOS systems. **Effort: ~30min**

- [ ] **Tag and release v1.0.0** — Create annotated tag, trigger release workflow, verify GitHub release assets, verify Docker image published. **Effort: ~1h**

## Phase 3: Hardening (Week 3-4)

### Security, edge cases, and reliability improvements

- [ ] **Add request body size limit enforcement** — Verify body_limit middleware is properly wired for all listener types and tested with oversized payloads. **Effort: ~2h**

- [ ] **Add connection timeout hardening** — Verify slow loris protection is effective. Add integration test for slow connection attack. **Effort: ~3h**

- [ ] **Add HTTP/2 specific hardening** — Test HPACK bomb protection, stream multiplexing limits, SETTINGS frame flood protection. **Effort: ~4h**

- [ ] **Add cluster partition recovery test** — Test network partition scenario with 5 nodes, verify split-brain protection works, verify recovery after partition heals. **Effort: ~4h**

- [ ] **Add admin API rate limiting** — Currently the admin API has auth but no built-in rate limiting. Add rate limiting to prevent brute-force auth attempts. **Effort: ~2h**

- [ ] **Add request/response body validation for admin API** — Ensure all admin API endpoints properly validate JSON input, reject malformed data. **Effort: ~3h**

## Phase 4: Testing Enhancement (Week 5-6)

### Improve test coverage for lower-coverage packages

- [ ] **Raise admin package coverage from 85.3% to 90%+** — Add tests for error paths in handlers, WebSocket event streaming, config API edge cases. **Effort: ~4h**

- [ ] **Raise CLI package coverage from 86.4% to 90%+** — Add tests for TUI rendering, advanced command error paths, shell completion generation. **Effort: ~4h**

- [ ] **Raise engine package coverage from 87.7% to 90%+** — Add tests for reload error scenarios, component initialization failures, concurrent start/stop. **Effort: ~4h**

- [ ] **Add load/stress tests** — Create dedicated load test suite that verifies: 100K+ concurrent connections, sustained 50K+ RPS, memory stability over 24h soak. **Effort: ~8h**

- [ ] **Add chaos testing for cluster** — Random node kills, network delays, disk full scenarios for Raft cluster. **Effort: ~8h**

- [ ] **Add React component tests** — The WebUI has zero frontend tests. Add Vitest + React Testing Library tests for critical pages (dashboard, pools, listeners, WAF). Target: 60%+ component coverage. **Effort: ~8h**

- [ ] **Refactor CLI top.go** — Split `internal/cli/top.go` (1,078 LOC) into separate files: `tui.go`, `screen.go`, `input.go`, `layout.go`, `terminal.go`. **Effort: ~2h**

- [ ] **Replace bubble-sort with sort.Slice in CLI** — `internal/cli/cli.go`, `formatter.go`, `parser.go` use manual bubble-sort. Replace with idiomatic `sort.Slice`. **Effort: ~30min**

## Phase 5: Performance & Optimization (Week 7-8)

### Performance tuning based on benchmark analysis

- [ ] **Profile and optimize L7 proxy hot path** — Run `go tool pprof` under sustained load. Target: reduce per-request allocation count. Current proxy overhead is 137µs — target <100µs. **Effort: ~8h**

- [ ] **Benchmark on dedicated hardware** — Current benchmarks are from development machine. Run on production-equivalent hardware (8-core, 16GB RAM) to get accurate numbers. Compare against nginx/HAProxy/Caddy. **Effort: ~4h**

- [ ] **Optimize connection pool hit rate** — Add metrics for pool hit/miss rate. Tune pool size based on actual traffic patterns. **Effort: ~4h**

- [ ] **Add memory pressure testing** — Verify behavior under memory pressure (near OOM). Test GC impact on tail latencies. **Effort: ~4h**

- [ ] **Optimize WAF detection hot path** — Profile WAF regex matching under load. Consider Aho-Corasick for multi-pattern matching if needed. **Effort: ~8h**

## Phase 6: Documentation & DX (Week 9-10)

### Documentation polish and developer experience

- [ ] **Generate OpenAPI spec** — Create `openapi.yaml` from the admin API implementation. Consider auto-generation from Go handler comments. **Effort: ~8h**

- [ ] **Add architecture decision records (ADRs)** — Document key decisions: why custom parsers instead of libraries, why Raft from scratch, why React instead of vanilla JS, why not use grpc-go. **Effort: ~4h**

- [ ] **Create migration guides** — Nginx → OLB, HAProxy → OLB, Caddy → OLB, Traefik → OLB config migration guides with examples. **Effort: ~8h**

- [ ] **Add Grafana dashboard templates** — Expand `deploy/grafana/` with comprehensive dashboards for: overview, backends, WAF, cluster, TLS. **Effort: ~6h**

- [ ] **Create video/interactive tutorial** — 5-minute getting started demo. Embed in website or link from README. **Effort: ~4h**

- [ ] **Add shared state management to WebUI** — Multiple pages fetch the same data independently (pools, health). Implement a lightweight shared cache (e.g., SWR or React Query pattern) to reduce redundant API calls. **Effort: ~4h**

- [ ] **Add error retry with backoff to WebUI** — The `useQuery` hook has no automatic retry. Add exponential backoff for transient network errors. **Effort: ~2h**

- [ ] **Expand WebUI CRUD capabilities** — Currently most management pages show "requires config file update" toasts. Implement full CRUD for pools, routes, listeners, and middleware via admin API. **Effort: ~16h**

- [ ] **Add WebUI accessibility improvements** — Add `aria-live` regions for dynamic content, skip-to-content link, consistent focus management. **Effort: ~4h**

## Phase 7: Post-Release Features (Week 11+)

### Features and improvements for v1.1+

- [ ] **HTTP/3 (QUIC) support** — Spec mentions this as "future". Requires QUIC implementation or `quic-go` dependency (would break zero-dep goal unless implemented from scratch). **Effort: ~80h**

- [ ] **Brotli compression** — Spec mentions pure Go Brotli. Currently only gzip/deflate. Requires implementing Brotli from scratch to maintain zero-dep. **Effort: ~40h**

- [ ] **RBAC for admin API** — Spec mentions "RBAC (future)". Implement role-based access control for admin API (read-only, operator, admin roles). **Effort: ~16h**

- [ ] **GeoIP database integration** — Spec mentions "GeoIP (embedded DB, future)". Requires embedded GeoIP database with periodic updates. **Effort: ~24h**

- [ ] **WebAssembly plugin support** — Currently supports Go `.so` plugins. Add WASM-based plugin sandboxing for safer, more portable plugins. **Effort: ~40h**

- [ ] **Consul service discovery verification** — Verify and test Consul provider works with real Consul cluster. Add integration tests. **Effort: ~8h**

- [ ] **Prometheus remote write** — Add ability to push metrics to remote Prometheus instance instead of only pull-based scraping. **Effort: ~16h**

- [ ] **Configuration dry-run/diff API** — Add admin API endpoint that validates and diffs a proposed config change without applying it. **Effort: ~8h**

## Effort Summary

| Phase | Estimated Hours | Priority | Dependencies |
|-------|----------------|----------|--------------|
| Phase 1: Critical Fixes | ~12h | CRITICAL | None |
| Phase 2: Release Operations | ~4h | CRITICAL | Phase 1 |
| Phase 3: Hardening | ~18h | HIGH | Phase 2 |
| Phase 4: Testing Enhancement | ~42h | HIGH | Phase 3 |
| Phase 5: Performance | ~28h | MEDIUM | Phase 4 |
| Phase 6: Documentation & DX | ~56h | MEDIUM | Phase 2 |
| Phase 7: Post-Release Features | ~232h | LOW | Phase 2 |
| **Total (Phases 1-6)** | **~160h** | | |
| **Total (All Phases)** | **~392h** | | |

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| OAuth2 middleware has undiscovered bugs | High | Medium | Prioritize test coverage to 85%+ before release |
| E2E test flakiness masks real failures | Medium | Medium | Fix parallel test isolation, add dedicated CI job |
| Single contributor bus factor | High | High | Document thoroughly, encourage community contributions |
| Performance regression under production load | Medium | High | Run benchmarks on dedicated hardware, add load tests |
| Cluster split-brain under network partition | Low | Critical | Add chaos tests, verify Raft implementation |
| Security vulnerability in custom parsers | Low | Critical | Fuzz testing exists, add continuous fuzzing via CI |
| Zero-dep policy prevents HTTP/3 support | High | Low | Defer HTTP/3 to v2.0 or accept quic-go exception |
