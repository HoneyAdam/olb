# OpenLoadBalancer Production Runbook

> Operational guide for running OLB in production
> Version: 1.0 | 2025-04-05

---

## Table of Contents

1. [Pre-Deployment Checklist](#pre-deployment-checklist)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Starting Services](#starting-services)
5. [Health Checks](#health-checks)
6. [Common Operations](#common-operations)
7. [Monitoring](#monitoring)
8. [Alerting](#alerting)
9. [Troubleshooting](#troubleshooting)
10. [Incident Response](#incident-response)
11. [Backup & Recovery](#backup--recovery)
12. [Scaling](#scaling)
13. [Security](#security)
14. [Upgrade Procedures](#upgrade-procedures)

---

## Pre-Deployment Checklist

### Infrastructure Requirements

| Resource | Minimum | Recommended | Notes |
|----------|---------|-------------|-------|
| CPU | 2 cores | 4+ cores | More for high RPS |
| Memory | 2GB | 4GB+ | 6KB per connection |
| Disk | 10GB | 50GB+ | For logs, Raft data |
| Network | 1Gbps | 10Gbps | Backend bandwidth |

### Software Requirements

- [ ] Linux kernel 4.19+ (Ubuntu 20.04+, RHEL 8+, Debian 11+)
- [ ] Go 1.25+ (if building from source)
- [ ] systemd (recommended) or other init system
- [ ] NTP synchronized (critical for Raft)

### Network Requirements

| Port | Purpose | Protocol | Source |
|------|---------|----------|--------|
| 80/443 | HTTP/HTTPS traffic | TCP | Any |
| 8080 | Admin API | TCP | Internal only |
| 7946 | Raft clustering | TCP | Cluster nodes |
| 7947 | SWIM gossip | UDP | Cluster nodes |

### Pre-Deployment Steps

```bash
# 1. Create directories
sudo mkdir -p /etc/olb /var/lib/olb /var/log/olb

# 2. Create olb user
sudo useradd --system --home /var/lib/olb --shell /bin/false olb

# 3. Set permissions
sudo chown -R olb:olb /var/lib/olb /var/log/olb
sudo chmod 750 /var/lib/olb /var/log/olb

# 4. Verify NTP
timedatectl status | grep "NTP enabled: yes"

# 5. Increase file limits
sudo sysctl -w fs.file-max=100000
sudo sysctl -w net.core.somaxconn=65535
```

---

## Installation

### Option 1: Binary Download (Recommended)

```bash
# Download latest release
curl -L https://github.com/openloadbalancer/olb/releases/latest/download/olb-linux-amd64 -o olb
chmod +x olb
sudo mv olb /usr/local/bin/

# Verify
olb version
```

### Option 2: Docker

```bash
docker pull openloadbalancer/olb:latest

# Run
docker run -d \
  --name olb \
  --net host \
  -v /etc/olb:/etc/olb:ro \
  -v /var/lib/olb:/var/lib/olb \
  -v /var/log/olb:/var/log/olb \
  openloadbalancer/olb:latest
```

### Option 3: Kubernetes

```bash
# Add Helm repo
helm repo add olb https://openloadbalancer.github.io/helm-charts
helm repo update

# Install
helm install olb olb/olb \
  --namespace olb \
  --create-namespace \
  --set config.listeners[0].address=":80"
```

---

## Configuration

### Minimal Configuration

Create `/etc/olb/olb.yaml`:

```yaml
listeners:
  - name: http
    address: ":80"
    protocol: http
    routes:
      - path: /
        pool: web

pools:
  - name: web
    algorithm: round_robin
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"

admin:
  address: ":8081"
  auth:
    type: basic
    users:
      - username: admin
        password: "$2y$10$..."  # bcrypt hash

logging:
  level: info
  format: json
  output: /var/log/olb/olb.log
```

### Production Configuration

```yaml
listeners:
  - name: https
    address: ":443"
    protocol: https
    tls:
      cert_file: "/etc/olb/certs/server.crt"
      key_file: "/etc/olb/certs/server.key"
    routes:
      - path: /
        pool: web

pools:
  - name: web
    algorithm: least_response_time
    health_check:
      interval: 5s
      timeout: 2s
      path: /health
    backends:
      - address: "10.0.1.10:8080"
        weight: 3
        max_conns: 100
      - address: "10.0.1.11:8080"
        weight: 2
        max_conns: 100
      - address: "10.0.1.12:8080"
        weight: 1
        max_conns: 50

middleware:
  rate_limit:
    enabled: true
    requests_per_second: 1000
    burst: 1500
  
  cors:
    enabled: true
    allowed_origins: ["https://example.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
  
  real_ip:
    enabled: true
    header: X-Forwarded-For

admin:
  address: ":8081"
  auth:
    type: bearer
    token: "${ADMIN_TOKEN}"  # From env var
  tls:
    cert_file: "/etc/olb/certs/admin.crt"
    key_file: "/etc/olb/certs/admin.key"

waf:
  enabled: true
  mode: enforce
  ip_acl:
    enabled: true
    blacklist:
      - cidr: "192.168.1.100/32"
        reason: "blocked-abuse"
  rate_limit:
    enabled: true
    rules:
      - id: "ddos-protection"
        scope: "ip"
        limit: 100
        window: "1m"

tracing:
  enabled: true
  sampling_rate: 0.1

logging:
  level: info
  format: json
  output: /var/log/olb/olb.log
  rotation:
    max_size: 100
    max_backups: 5
    max_age: 30

metrics:
  enabled: true
  path: "/metrics"
```

### Environment Variables

Create `/etc/olb/olb.env`:

```bash
# Required
ADMIN_TOKEN=your-secure-token-here

# Optional
OLB_LOG_LEVEL=info
OLB_CONFIG_FILE=/etc/olb/olb.yaml
OLB_DATA_DIR=/var/lib/olb
```

---

## Starting Services

### Using systemd

Create `/etc/systemd/system/olb.service`:

```ini
[Unit]
Description=OpenLoadBalancer
After=network.target

[Service]
Type=notify
User=olb
Group=olb
EnvironmentFile=/etc/olb/olb.env
ExecStart=/usr/local/bin/olb serve --config /etc/olb/olb.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s

# Resource limits
LimitNOFILE=65535
LimitNPROC=4096

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/olb /var/log/olb

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable olb
sudo systemctl start olb

# Check status
sudo systemctl status olb
sudo journalctl -u olb -f
```

### Using Docker Compose

```yaml
version: '3.8'
services:
  olb:
    image: openloadbalancer/olb:latest
    ports:
      - "80:80"
      - "443:443"
      - "8081:8081"
    volumes:
      - ./config:/etc/olb:ro
      - olb-data:/var/lib/olb
      - olb-logs:/var/log/olb
    environment:
      - ADMIN_TOKEN=${ADMIN_TOKEN}
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 4G

volumes:
  olb-data:
  olb-logs:
```

---

## Health Checks

### System Health

```bash
# Check if running
curl http://localhost:8081/api/v1/system/health \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Expected response
{
  "status": "healthy",
  "uptime": "2d15h30m",
  "pools": {
    "web": {
      "status": "healthy",
      "healthy_backends": 3,
      "total_backends": 3
    }
  }
}
```

### Backend Health

```bash
# Check backend status
curl http://localhost:8081/api/v1/backends/web \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Expected healthy response
{
  "name": "web",
  "algorithm": "least_response_time",
  "healthy": true,
  "backends": [
    {"id": "...", "state": "up", "healthy": true, "active_conns": 42},
    {"id": "...", "state": "up", "healthy": true, "active_conns": 38},
    {"id": "...", "state": "up", "healthy": true, "active_conns": 35}
  ]
}
```

### Cluster Health (if clustered)

```bash
# Check cluster status
curl http://localhost:8081/api/v1/cluster/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Expected response
{
  "node_id": "node-1",
  "state": "leader",
  "term": 15,
  "peers": [
    {"id": "node-2", "state": "follower", "healthy": true},
    {"id": "node-3", "state": "follower", "healthy": true}
  ],
  "committed_index": 1234,
  "last_applied": 1234
}
```

---

## Common Operations

### Add a Backend

```bash
# Add backend to pool
curl -X POST http://localhost:8081/api/v1/backends/web \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "10.0.1.20:8080",
    "weight": 2,
    "max_conns": 100
  }'

# Verify
curl http://localhost:8081/api/v1/backends/web \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Remove a Backend

```bash
# Graceful drain first
curl -X POST http://localhost:8081/api/v1/backends/web/10.0.1.20:8080/drain \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Wait for connections to drain (check active_conns)
curl http://localhost:8081/api/v1/backends/web/10.0.1.20:8080 \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Remove when drained
curl -X DELETE http://localhost:8081/api/v1/backends/web/10.0.1.20:8080 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Update Configuration (Hot Reload)

```bash
# Edit config
sudo nano /etc/olb/olb.yaml

# Validate syntax
olb config validate /etc/olb/olb.yaml

# Hot reload (no downtime)
curl -X POST http://localhost:8081/api/v1/system/reload \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Or use systemd
sudo systemctl reload olb
```

### View Metrics

```bash
# Prometheus format
curl http://localhost:8081/metrics

# JSON format
curl http://localhost:8081/api/v1/metrics \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Check Logs

```bash
# Live logs
sudo tail -f /var/log/olb/olb.log | jq .

# Filter errors
sudo cat /var/log/olb/olb.log | jq 'select(.level=="error")'

# Search for specific backend
sudo cat /var/log/olb/olb.log | jq 'select(.backend=="10.0.1.10:8080")'
```

### WAF Operations

```bash
# Get WAF status
curl http://localhost:8081/api/v1/waf/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Get blocked IPs
curl http://localhost:8081/api/v1/waf/blocked \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Unblock IP
curl -X DELETE http://localhost:8081/api/v1/waf/blocked/192.168.1.100 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Monitoring

### Key Metrics

| Metric | Query | Threshold |
|--------|-------|-----------|
| Request Rate | `rate(olb_requests_total[5m])` | Baseline |
| Error Rate | `rate(olb_requests_errors_total[5m])` | < 1% |
| Latency P99 | `histogram_quantile(0.99, rate(olb_request_duration_seconds_bucket[5m]))` | < 100ms |
| Active Connections | `olb_connections_active` | < max |
| Backend Health | `olb_backends_healthy / olb_backends_total` | = 1 |
| Pool Health | `olb_pool_healthy` | = 1 |

### Prometheus Configuration

```yaml
# /etc/prometheus/prometheus.yml
scrape_configs:
  - job_name: 'olb'
    static_configs:
      - targets: ['localhost:8081']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### Grafana Dashboard

Import dashboard from `https://grafana.com/grafana/dashboards/xxxxx` or use the included one:

```bash
# Copy dashboard
cp dashboards/olb.json /var/lib/grafana/dashboards/
```

---

## Alerting

### Prometheus Alert Rules

```yaml
# /etc/prometheus/alerts/olb.yml
groups:
  - name: olb
    rules:
      - alert: OLBHighErrorRate
        expr: rate(olb_requests_errors_total[5m]) / rate(olb_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate on OLB"
          description: "Error rate is {{ $value | humanizePercentage }}"

      - alert: OLBLatencyHigh
        expr: histogram_quantile(0.99, rate(olb_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency on OLB"
          description: "P99 latency is {{ $value }}s"

      - alert: OLBBackendDown
        expr: olb_backends_healthy / olb_backends_total < 1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Backend down in pool"
          description: "Only {{ $value | humanizePercentage }} backends healthy"

      - alert: OLBRaftNoLeader
        expr: olb_raft_state != 2  # 2 = leader
        for: 10s
        labels:
          severity: critical
        annotations:
          summary: "No Raft leader"
          description: "Cluster has no leader for more than 10s"

      - alert: OLBWAFBlockedRequests
        expr: rate(olb_waf_blocked_total[5m]) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High WAF blocks"
          description: "{{ $value }} requests blocked per second"
```

---

## Troubleshooting

### High Memory Usage

**Symptoms**: Memory usage climbing steadily

**Diagnosis**:
```bash
# Check active connections
curl http://localhost:8081/api/v1/metrics | grep olb_connections_active

# Check for connection leaks
curl http://localhost:8081/api/v1/metrics | grep olb_connections_total

# Profile memory
curl -o heap.prof http://localhost:8081/debug/pprof/heap
go tool pprof heap.prof
```

**Solutions**:
1. Check backend health - unhealthy backends may cause connection buildup
2. Review connection pool settings
3. Check for client keep-alive issues

### High CPU Usage

**Symptoms**: CPU usage > 80%

**Diagnosis**:
```bash
# Check request rate
curl http://localhost:8081/api/v1/metrics | grep olb_requests_total

# Profile CPU
curl -o cpu.prof http://localhost:8081/debug/pprof/profile?seconds=30
go tool pprof cpu.prof
```

**Solutions**:
1. Scale horizontally (add more OLB instances)
2. Enable caching middleware
3. Review rate limiting rules
4. Check WAF rules for expensive regex

### Backend Connection Failures

**Symptoms**: 502/503 errors, backends marked unhealthy

**Diagnosis**:
```bash
# Check backend health
curl http://localhost:8081/api/v1/backends/web \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Test backend directly
curl -v http://10.0.1.10:8080/health

# Check health check logs
sudo cat /var/log/olb/olb.log | jq 'select(.component=="health")'
```

**Solutions**:
1. Verify backend is running and healthy
2. Check network connectivity
3. Review health check configuration
4. Check backend logs

### Raft Split-Brain

**Symptoms**: Multiple nodes claiming leadership, inconsistent state

**Diagnosis**:
```bash
# Check all nodes
curl http://node1:8081/api/v1/cluster/status

curl http://node2:8081/api/v1/cluster/status

curl http://node3:8081/api/v1/cluster/status
```

**Solutions**:
1. Ensure network connectivity between nodes
2. Check NTP synchronization
3. Review Raft logs
4. May need manual intervention (see Incident Response)

### SSL/TLS Errors

**Symptoms**: TLS handshake failures, certificate errors

**Diagnosis**:
```bash
# Test SSL
curl -v https://localhost/ --insecure

# Check certificate
openssl s_client -connect localhost:443 -servername example.com

# Check certificate expiry
curl http://localhost:8081/api/v1/certificates \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Solutions**:
1. Verify certificate files exist and are readable
2. Check certificate validity and expiry
3. Verify private key matches certificate
4. For ACME: check DNS and reachability

---

## Incident Response

### Severity Levels

| Level | Description | Response Time |
|-------|-------------|---------------|
| P0 | Service down, complete outage | Immediate |
| P1 | Major degradation, partial outage | 15 minutes |
| P2 | Minor degradation, workarounds exist | 1 hour |
| P3 | Cosmetic, no service impact | 4 hours |

### P0: Complete Outage

**Immediate Actions**:
1. Check if process is running: `systemctl status olb`
2. Check for resource exhaustion: `free -h`, `df -h`
3. Check last logs: `journalctl -u olb -n 100`
4. Try restart: `systemctl restart olb`

**If restart fails**:
1. Check configuration validity: `olb config validate`
2. Rollback to last known good config
3. Start with minimal config if needed

### P0: Raft Split-Brain

**Detection**: Multiple leaders, diverging logs

**Resolution**:
1. Identify the node with the most up-to-date log (highest `committed_index`)
2. Stop all nodes
3. Clear Raft data on nodes with stale logs (keep the most current)
4. Restart the current node first (becomes leader)
5. Join other nodes back

```bash
# On stale nodes
sudo systemctl stop olb
sudo rm -rf /var/lib/olb/raft/*
sudo systemctl start olb
```

### P1: Backend Pool Degradation

**Detection**: < 100% healthy backends

**Actions**:
1. Identify unhealthy backends via API
2. Check backend directly
3. If transient, health checks will auto-recover
4. If persistent, drain and remove backend
5. Add replacement backend

### P1: High Latency

**Detection**: P99 latency > threshold

**Actions**:
1. Check backend latency
2. Review rate limiting (may be throttling)
3. Check resource utilization (CPU/memory)
4. Scale horizontally if needed

---

## Backup & Recovery

### Backup Strategy

**Daily**:
- Configuration files (`/etc/olb/`)
- Raft data (`/var/lib/olb/raft/`)
- TLS certificates (`/etc/olb/certs/`)

**Scripts**:

```bash
#!/bin/bash
# /usr/local/bin/olb-backup.sh

BACKUP_DIR=/backup/olb/$(date +%Y%m%d)
mkdir -p $BACKUP_DIR

# Config
cp -r /etc/olb $BACKUP_DIR/

# Raft (only on leader)
if curl -s http://localhost:8081/api/v1/cluster/status | grep -q '"state": "leader"'; then
    cp -r /var/lib/olb/raft $BACKUP_DIR/
fi

# Tar and compress
tar czf $BACKUP_DIR.tar.gz -C /backup/olb $(date +%Y%m%d)
rm -rf $BACKUP_DIR

# Keep last 7 days
find /backup/olb -name "*.tar.gz" -mtime +7 -delete
```

### Recovery Procedures

**Configuration Recovery**:
```bash
# Restore from backup
tar xzf /backup/olb/20250405.tar.gz -C /

# Validate
olb config validate /etc/olb/olb.yaml

# Reload
systemctl reload olb
```

**Full Recovery**:
```bash
# 1. Install OLB
# 2. Restore backup
tar xzf /backup/olb/20250405.tar.gz -C /

# 3. Fix permissions
chown -R olb:olb /var/lib/olb /var/log/olb

# 4. Start
systemctl start olb

# 5. Verify
systemctl status olb
curl http://localhost:8081/api/v1/system/health
```

**Cluster Recovery**:
```bash
# 1. Stop all nodes
systemctl stop olb  # on all nodes

# 2. On one node, restore backup
# 3. Start that node first (becomes leader)
systemctl start olb

# 4. Other nodes join
systemctl start olb  # on other nodes
```

---

## Scaling

### Vertical Scaling

```yaml
# Increase resource limits
pool:
  connection:
    max_idle: 100
    max_per_host: 500

middleware:
  cache:
    enabled: true
    max_size: 500MB
```

### Horizontal Scaling

**Add nodes to cluster**:

```yaml
# On new node
cluster:
  enabled: true
  node_id: "node-4"
  join: ["10.0.1.10:7946", "10.0.1.11:7946", "10.0.1.12:7946"]
```

**Add external load balancer** (for 50K+ RPS):
```
         [External LB]
        /      |      \
    [OLB-1] [OLB-2] [OLB-3]
        \      |      /
         [Backend Pool]
```

---

## Security

### Regular Security Tasks

| Task | Frequency | Command |
|------|-----------|---------|
| Rotate certificates | 90 days | Automatic with ACME |
| Audit logs | Weekly | Review `/var/log/olb/olb.log` |
| Check blocked IPs | Daily | `curl /api/v1/waf/blocked` |
| Review access logs | Weekly | Analyze patterns |
| Update software | Monthly | `apt upgrade olb` |

### Security Hardening

```bash
# 1. Firewall rules
sudo ufw default deny incoming
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow from 10.0.0.0/8 to any port 8081
sudo ufw enable

# 2. File permissions
sudo chmod 600 /etc/olb/olb.yaml
sudo chmod 600 /etc/olb/certs/*.key
sudo chown -R olb:olb /etc/olb /var/lib/olb

# 3. Disable unnecessary features
# In config, only enable what you need
```

---

## Upgrade Procedures

### Minor Upgrade (e.g., 1.0.1 → 1.0.2)

```bash
# 1. Download new version
curl -L https://.../olb-linux-amd64 -o olb-new
chmod +x olb-new

# 2. Backup current
cp /usr/local/bin/olb /usr/local/bin/olb.backup

# 3. Replace
sudo systemctl stop olb
sudo mv olb-new /usr/local/bin/olb
sudo systemctl start olb

# 4. Verify
olb version
curl http://localhost:8081/api/v1/system/health
```

### Major Upgrade (e.g., 1.0 → 2.0)

```bash
# 1. Read release notes
# 2. Test in staging
# 3. Backup everything
# 4. Stop service
# 5. Upgrade binary
# 6. Run migration if needed: olb migrate
# 7. Start service
# 8. Verify functionality
```

### Zero-Downtime Upgrade (Cluster)

```bash
# Rolling restart - one node at a time

# Node 1
ssh node1 "sudo systemctl restart olb"
sleep 30
curl http://node1:8081/api/v1/system/health

# Node 2
ssh node2 "sudo systemctl restart olb"
sleep 30
curl http://node2:8081/api/v1/system/health

# Node 3
ssh node3 "sudo systemctl restart olb"
sleep 30
curl http://node3:8081/api/v1/system/health
```

---

## Quick Reference

### Essential Commands

```bash
# Status
systemctl status olb

# Logs
journalctl -u olb -f

# Metrics
curl http://localhost:8081/metrics

# Health
curl http://localhost:8081/api/v1/system/health -H "Authorization: Bearer $TOKEN"

# Reload
systemctl reload olb
# or
curl -X POST http://localhost:8081/api/v1/system/reload -H "Authorization: Bearer $TOKEN"

# Top (TUI)
olb top
```

### Configuration Files

| File | Purpose |
|------|---------|
| `/etc/olb/olb.yaml` | Main config |
| `/etc/olb/olb.env` | Environment variables |
| `/var/lib/olb/raft/` | Cluster state |
| `/var/log/olb/olb.log` | Application logs |
| `/etc/systemd/system/olb.service` | Service definition |

### Port Reference

| Port | Purpose |
|------|---------|
| 80 | HTTP |
| 443 | HTTPS |
| 8081 | Admin API |
| 7946 | Raft (TCP) |
| 7947 | Gossip (UDP) |

---

*Document version: 1.0*
*Last updated: 2025-04-05*
