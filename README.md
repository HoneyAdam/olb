# OpenLoadBalancer

> A zero-dependency, production-grade load balancer and reverse proxy written in Go using only the standard library.

## Overview

OpenLoadBalancer (OLB) is a high-performance Layer 4 (TCP/UDP) and Layer 7 (HTTP/HTTPS/gRPC/WebSocket) load balancer designed with simplicity and observability in mind. It operates as a single static binary with zero external dependencies.

## Features

- **Zero Dependencies** - Only Go standard library
- **Single Binary** - One `olb` binary contains everything
- **L4 + L7 Proxy** - TCP, UDP, HTTP/1.1, HTTP/2, WebSocket, gRPC
- **Load Balancing** - 12 algorithms including round-robin, least connections, consistent hashing
- **Hot Reload** - Zero-downtime configuration and certificate updates
- **Built-in Observability** - Metrics, access logs, and Web UI dashboard
- **Auto HTTPS** - Automatic ACME/Let's Encrypt integration
- **Clustering** - Built-in Raft consensus for multi-node deployments
- **AI-Native** - MCP server for AI agent integration
- **Security** - mTLS, rate limiting, IP filtering, WAF basics

## Quick Start

```bash
# Download the latest release
curl -L https://github.com/openloadbalancer/olb/releases/latest/download/olb-linux-amd64 -o olb
chmod +x olb

# Create a minimal config
cat > olb.yaml << 'EOF'
listeners:
  - name: http
    address: ":80"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    health_check:
      path: /health
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
EOF

# Run the load balancer
./olb --config olb.yaml
```

## Installation

### Binary Download

Download pre-built binaries from the [releases page](https://github.com/openloadbalancer/olb/releases).

### Docker

```bash
docker run -p 80:80 -p 443:443 -v $(pwd)/olb.yaml:/etc/olb/configs/olb.yaml openloadbalancer/olb:latest
```

### Build from Source

```bash
git clone https://github.com/openloadbalancer/olb.git
cd olb
make build
```

## Configuration

OLB supports YAML, JSON, and TOML configuration formats. See [configs/](configs/) for examples.

```yaml
# Minimal configuration
listeners:
  - name: http
    address: ":80"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     OpenLoadBalancer                     │
├─────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │
│  │ L7 HTTP │  │ L4 TCP  │  │ L4 UDP  │  │ Admin   │    │
│  │Listener │  │Listener │  │Listener │  │  API    │    │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘    │
│       └─────────────┴─────────────┴─────────────┘       │
│                    Connection Manager                    │
│                         │                               │
│  ┌─────────┐  ┌────────┴─────┐  ┌─────────┐  ┌─────────┐│
│  │  Router │  │Load Balancer │  │ Backend │  │   TLS   ││
│  │         │  │ (12 algos)   │  │  Pool   │  │ Engine  ││
│  └─────────┘  └──────────────┘  └─────────┘  └─────────┘│
│                                                          │
│  ┌─────────────────────────────────────────────────────┐│
│  │         Middleware Pipeline                         ││
│  │  (rate limit, cors, auth, cache, rewrite, etc.)    ││
│  └─────────────────────────────────────────────────────┘│
│                                                          │
│  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐        │
│  │Cluster │  │  MCP   │  │  Web   │  │  CLI   │        │
│  │ (Raft) │  │ Server │  │   UI   │  │        │        │
│  └────────┘  └────────┘  └────────┘  └────────┘        │
└─────────────────────────────────────────────────────────┘
```

## Documentation

- [SPECIFICATION.md](SPECIFICATION.md) - Detailed technical specification
- [IMPLEMENTATION.md](IMPLEMENTATION.md) - Implementation guide
- [TASKS.md](TASKS.md) - Development task tracking

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.
