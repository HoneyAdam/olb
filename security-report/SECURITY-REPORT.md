# Security Audit Report - OpenLoadBalancer

**Date:** 2026-04-13
**Scope:** Full codebase audit (~380 Go files)
**Methodology:** 4-phase pipeline (Recon -> Hunt -> Verify -> Report)

## Executive Summary

| Metric | Value |
|--------|-------|
| Total Findings | 42 |
| Critical | 1 |
| High | 6 |
| Medium | 22 |
| Low | 13 |

**Overall Risk Assessment:** MODERATE

## Critical Findings

### CRIT-1: MCP Server Fully Open When BearerToken Is Empty
- **File:** internal/mcp/mcp.go:1258-1271
- **CVSS:** 9.8 (Critical)
- Empty bearerToken bypasses all MCP authentication

## High Findings

| ID | Finding | File | CVSS |
|----|---------|------|------|
| HIGH-1 | CSP nonce hardcoded placeholder | internal/middleware/csp/csp.go:217 | 7.5 |
| HIGH-2 | HMAC replay protection not implemented | internal/middleware/hmac/hmac.go:29 | 7.5 |
| HIGH-3 | CSRF disabled by default | internal/middleware/csrf/csrf.go:32 | 8.0 |
| HIGH-4 | MCP tools no authorization granularity | internal/mcp/mcp.go:554 | 7.5 |
| HIGH-5 | SSE unbounded line buffering (DoS) | internal/proxy/l7/sse.go:190 | 7.5 |
| HIGH-6 | H2C enabled by default | internal/proxy/l7/http2.go:74 | 7.4 |

## Medium Findings (22)

MED-1 through MED-22 — see verified-findings.md for details

## Low Findings (13)

LOW-1 through LOW-13 — see verified-findings.md for details

## Remediation Priority

P0 (Immediate): CRIT-1, HIGH-1, HIGH-2, HIGH-5
P1 (Next Sprint): HIGH-3, HIGH-4, HIGH-6, MED-1, MED-4/5, MED-8
P2 (Next Quarter): MED-7, MED-9, MED-11, MED-15, MED-2, MED-12
