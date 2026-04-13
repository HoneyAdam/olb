# Architecture Security Map - OpenLoadBalancer

## Entry Points

| Component | Protocol | Auth Required | File |
|-----------|----------|---------------|------|
| HTTP/HTTPS Listener | HTTP/HTTPS | Optional | internal/listener/http.go |
| TCP/UDP L4 Proxy | TCP/UDP | None (passthrough) | internal/proxy/l4/tcp.go |
| SNI Router | TLS/TCP | TLS-based | internal/proxy/l4/sni.go |
| Admin API | HTTP | Basic/Bearer | internal/admin/server.go |
| WebUI | HTTP (SPA) | Proxied to Admin | internal/webui/webui.go |
| MCP Server (HTTP) | HTTP POST | Bearer Token | internal/mcp/mcp.go:1251 |
| MCP Server (Stdio) | Stdin/Stdout | None (local) | internal/mcp/mcp.go:1181 |
| Profiling (pprof) | HTTP | None | internal/profiling/profiling.go |

## Trust Boundaries

[Internet] -> [Listeners] -> [WAF] -> [IP Filter] -> [Auth MW] -> [Router] -> [Proxy] -> [Backend]
[Admin User] -> [Admin API] -> [Backend State]
[MCP Client] -> [MCP HTTP] -> [Backend/Route/Config State]
[Cluster Peer] -> [Gossip TCP] -> [State Sync]
[Config File] -> [Config Loader] -> [Engine]

## Dependencies

| Dependency | Version | Usage |
|-----------|---------|-------|
| golang.org/x/crypto | v0.49.0 | bcrypt, ed25519, OCSP |
| golang.org/x/net | v0.52.0 | HTTP/2, HPACK |
| golang.org/x/text | v0.35.0 | Indirect (transitive) |

Go: 1.26.2 | CGO: Disabled | External deps: 3
