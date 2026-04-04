# Getting Started Tutorial

Learn OpenLoadBalancer by building a complete load balancing setup.

## Prerequisites

- Go 1.25+ (for building from source) or downloaded binary
- 3 backend servers (we'll use Docker for demo)
- curl or httpie for testing

## Step 1: Install OLB

### Option A: Download Binary
```bash
curl -sSL https://openloadbalancer.dev/install.sh | sh
olb version
```

### Option B: Build from Source
```bash
git clone https://github.com/openloadbalancer/olb.git
cd olb
go build -o bin/olb ./cmd/olb
sudo cp bin/olb /usr/local/bin/
```

## Step 2: Create Test Backends

Create three simple backend servers:

```bash
# Terminal 1
docker run -p 3001:80 --name backend1 kennethreitz/httpbin

# Terminal 2
docker run -p 3002:80 --name backend2 kennethreitz/httpbin

# Terminal 3
docker run -p 3003:80 --name backend3 kennethreitz/httpbin
```

## Step 3: Basic Configuration

Create `olb.yaml`:

```yaml
admin:
  address: "127.0.0.1:8081"

listeners:
  - name: http
    address: ":8080"
    routes:
      - path: /
        pool: web

pools:
  - name: web
    algorithm: round_robin
    backends:
      - address: "localhost:3001"
      - address: "localhost:3002"
      - address: "localhost:3003"
    health_check:
      type: http
      path: /get
      interval: 5s
```

## Step 4: Start OLB

```bash
olb start --config olb.yaml
```

## Step 5: Test Load Balancing

```bash
# Make requests and see round-robin distribution
for i in {1..6}; do
  curl -s http://localhost:8080/get | grep -o '"origin".*"'
done
```

You should see responses from different backends in rotation.

## Step 6: Check Admin API

```bash
# View status
curl http://localhost:8081/api/v1/status

# List backends
curl http://localhost:8081/api/v1/backends

# View Prometheus metrics
curl http://localhost:8081/metrics
```

## Step 7: Add Health Checks

Stop one backend and observe:

```bash
docker stop backend2
```

Check health status:
```bash
curl http://localhost:8081/api/v1/backends/web
```

You should see `backend2` marked as `down`.

Restart it:
```bash
docker start backend2
```

## Step 8: Enable WAF

Update `olb.yaml`:

```yaml
waf:
  enabled: true
  mode: enforce
  detection:
    enabled: true
    threshold: {block: 50, log: 25}
    detectors:
      sqli: {enabled: true}
      xss: {enabled: true}

middleware:
  rate_limit:
    enabled: true
    requests_per_second: 100
```

Reload config:
```bash
olb reload
# or send SIGHUP to process
```

Test WAF:
```bash
# Should be blocked (403)
curl -s "http://localhost:8080/get?id=1' OR '1'='1"

# Should pass
curl -s http://localhost:8080/get
```

## Step 9: Enable HTTPS

Generate test certificate:
```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj "/CN=localhost"
```

Add HTTPS listener:
```yaml
listeners:
  - name: https
    address: ":8443"
    protocol: https
    tls:
      cert_file: cert.pem
      key_file: key.pem
    routes:
      - path: /
        pool: web
```

Test:
```bash
curl -k https://localhost:8443/get
```

## Step 10: Enable Clustering (Optional)

For multi-node setup:

```yaml
cluster:
  enabled: true
  node_id: "node-1"
  bind_address: "0.0.0.0:7946"
  peers:
    - "192.168.1.10:7946"
    - "192.168.1.11:7946"
```

## Step 11: Monitoring

Start Prometheus + Grafana:
```bash
docker-compose up -d prometheus grafana
```

Access Grafana at http://localhost:3000 (admin/admin)

Import dashboard from `deploy/grafana/dashboard.json`

## Step 12: Production Deployment

For production, see:
- `docs/production-deployment.md` - Complete deployment guide
- `deploy/terraform/` - Infrastructure as Code
- `deploy/helm/` - Kubernetes deployment

## CLI Commands Reference

```bash
# Start with config
olb start --config olb.yaml

# Reload config
olb reload

# Check status
olb status

# Live dashboard
olb top

# Validate config
olb config validate olb.yaml

# Backend operations
olb backend list
olb backend drain web localhost:3001

# Cluster operations
olb cluster status

# Health check status
olb health show
```

## Next Steps

- Read [Configuration Guide](configuration.md) for all options
- Check [Migration Guide](migration-guide.md) if migrating from other LB
- Review [Troubleshooting](troubleshooting.md) for common issues
- Explore [WAF Documentation](waf.md) for security features
