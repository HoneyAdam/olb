# Dependency Audit - OpenLoadBalancer

## Status: CLEAN

Only 3 dependencies from official Go extended library. No third-party packages.

| Dependency | Version | Sub-packages |
|-----------|---------|-------------|
| golang.org/x/crypto | v0.49.0 | bcrypt, ed25519, ocsp |
| golang.org/x/net | v0.52.0 | http2, http2/h2c |
| golang.org/x/text | v0.35.0 | None (transitive) |

## Build Security

- No replace directives in go.mod
- No vendor directory
- CGO_ENABLED=0 in Dockerfile
- Statically linked binary
- Binary stripped (-s -w ldflags)
- Build paths removed (-trimpath)

## Supply Chain Risk: LOW
