# Verified Findings — OpenLoadBalancer Security Audit (2026-04-14)

All 31 findings were verified against actual source code. Confidence levels assigned based on code inspection.

## Verification Method

Each finding was cross-referenced with:
1. Source code reads of the affected files
2. Analysis of surrounding context (callers, data flow)
3. Assessment of exploitability in realistic deployment scenarios

## Verified Critical (1)

### CRITICAL-01: HMAC Timestamp Not in Signature
- **Verified:** Read `internal/middleware/hmac/hmac.go` lines 110-216
- **Confidence:** 100% — the `computeSignature` function (lines 175-216) builds message from method, path, query, body. No reference to `m.config.TimestampHeader` anywhere in the function.
- **Impact:** Replay protection is completely bypassable by updating the timestamp header to a current value.

## Verified High (1)

### HIGH-01: HMAC Body Truncation Silent
- **Verified:** Read `internal/middleware/hmac/hmac.go` lines 192-199
- **Confidence:** 100% — `io.LimitReader(r.Body, maxBodySize)` capped at 10MB. `io.ReadAll` returns the truncated content without error when the limit is hit.
- **Impact:** Downstream handlers receive truncated body; HMAC only covers first 10MB.

## Verified Medium (9)

### MED-01: Hardcoded Credentials
- **Verified:** Read `configs/olb.hcl` line 44 and `configs/olb.toml` line 41
- **Confidence:** 100% — bcrypt hash present with comment revealing password "admin123"

### MED-02: Empty Secret Not Validated
- **Verified:** Read `internal/middleware/hmac/hmac.go` lines 55-74
- **Confidence:** 100% — `New()` checks `!config.Enabled` but never validates `config.Secret != ""`

### MED-03: ZeroSecrets Ineffective
- **Verified:** Read `internal/middleware/hmac/hmac.go` lines 265-269
- **Confidence:** 100% — Go strings are immutable; assigning `""` creates a new empty string, old memory persists

### MED-04: gRPC TLS Skip Verify
- **Verified:** Read `internal/health/health.go` line 146
- **Confidence:** 100% — `InsecureSkipVerify: true` hardcoded in shared gRPC health check client

### MED-05: Sticky Session Unbounded
- **Verified:** Read `internal/balancer/sticky.go` line 75
- **Confidence:** 95% — map has no size limit; `CleanupSessions()` exists but is not auto-invoked

### MED-06: SSE Goroutine Spawning
- **Verified:** Read `internal/proxy/l7/sse.go` lines 199, 253-258
- **Confidence:** 100% — goroutine spawned per SSE line read; second goroutine for timeout

### MED-07: Error Message Disclosure
- **Verified:** Read `internal/proxy/l7/proxy.go` line 728
- **Confidence:** 90% — default case exposes `olbErr.Message`; depends on what backends return

### MED-08: MaxBuckets Not Enforced
- **Verified:** Read `internal/middleware/rate_limiter.go` lines 237-256
- **Confidence:** 100% — `LoadOrStore` called without checking current bucket count against `MaxBuckets`

### MED-09: XFF Trust Heuristic
- **Verified:** Read `internal/proxy/l7/proxy.go` lines 580-609
- **Confidence:** 90% — design limitation, not a bug; depends on deployment topology

## Verified Low (14)

All 14 low-severity findings confirmed by source code inspection. See SECURITY-REPORT.md for details.

## False Positive Analysis

No false positives identified. All findings correspond to real code patterns observed in the codebase.

## Findings from Previous Audit (2026-04-13) — Status Check

The previous audit identified 97 findings (75 fixed). Key remaining items cross-referenced:
- gRPC TLS skip verify: Still present → MED-04 (this audit)
- Sticky session bounds: Still present → MED-05 (this audit)
- HMAC middleware findings: NEW in this audit (CRITICAL-01, HIGH-01, MED-02, MED-03)

---

*Verified by security-check on 2026-04-14*
