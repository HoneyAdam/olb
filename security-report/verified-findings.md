# Verified Findings - OpenLoadBalancer Security Audit

All 42 findings were verified against actual source code.

## Verification Summary

| Category | Verified | False Positive | Partial |
|----------|----------|----------------|---------|
| Critical | 1 | 0 | 0 |
| High | 6 | 0 | 0 |
| Medium | 22 | 0 | 1 |
| Low | 13 | 0 | 0 |
| **Total** | **42** | **0** | **1** |

## Verification Details

### CRIT-1: MCP No Auth When Token Empty - TRUE POSITIVE (HIGH confidence)
Verified: internal/mcp/mcp.go:1258 - if t.bearerToken != empty check is the only auth gate.

### HIGH-1: CSP Nonce Hardcoded - TRUE POSITIVE (HIGH confidence)
Verified: internal/middleware/csp/csp.go:217-221 - Returns literal placeholder. No crypto/rand.

### HIGH-2: HMAC Replay Not Implemented - TRUE POSITIVE (HIGH confidence)
Verified: internal/middleware/hmac/hmac.go Wrap function (lines 86-122) never reads TimestampHeader or MaxAge.

### HIGH-3: CSRF Disabled by Default - TRUE POSITIVE (MEDIUM confidence)
Verified: internal/middleware/csrf/csrf.go:32 - Enabled: false in DefaultConfig().

### HIGH-4: MCP No Authorization - TRUE POSITIVE (HIGH confidence)
Verified: internal/mcp/mcp.go:554 - handleToolsCall dispatches without permission check.

### HIGH-5: SSE Unbounded Line Buffering - TRUE POSITIVE (HIGH confidence)
Verified: internal/proxy/l7/sse.go:190-228 - ReadBytes with no size limit. MaxEventSize unused.

### HIGH-6: H2C Enabled by Default - TRUE POSITIVE (HIGH confidence)
Verified: internal/proxy/l7/http2.go:74 - EnableH2C: true in DefaultHTTP2Config().

### MED-1: WebSocket Response Header Injection - TRUE POSITIVE (HIGH confidence)
Verified: internal/proxy/l7/websocket.go:274-276 - No sanitization on backend response headers.

### MED-4: JWT No Expiration Required - TRUE POSITIVE (HIGH confidence)
Verified: internal/middleware/jwt/jwt.go:203 - claims.ExpiresAt > 0 check skips zero.

### MED-8: API Key Permissions Never Checked - TRUE POSITIVE (HIGH confidence)
Verified: HasPermission only called from test files. Zero production callers.

### MED-11: CRL/OCSP Not Implemented - PARTIAL TRUE POSITIVE (MEDIUM confidence)
CRLFile validated for existence but never loaded. OCSPCheck in MTLSConfig never read.
Note: Server-side OCSP stapling IS implemented in ocsp.go.

### MED-12: Path Traversal Double-Decode - TRUE POSITIVE (HIGH confidence)
Verified: internal/security/security.go:436-451 - Single decode pass only. WAF compensates.
