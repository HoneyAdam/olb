# Migration Guide

Guide for migrating from other load balancers to OpenLoadBalancer.

## Table of Contents

- [From NGINX](#from-nginx)
- [From HAProxy](#from-haproxy)
- [From Traefik](#from-traefik)
- [From Envoy](#from-envoy)
- [From AWS ALB](#from-aws-alb)

---

## From NGINX

### Basic HTTP Proxy

**NGINX:**
```nginx
upstream backend {
    server 10.0.1.10:8080;
    server 10.0.1.11:8080;
}

server {
    listen 80;
    location / {
        proxy_pass http://backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**OLB:**
```yaml
listeners:
  - name: http
    address: ":80"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    algorithm: round_robin
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
```

### SSL/TLS Termination

**NGINX:**
```nginx
server {
    listen 443 ssl;
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    # ...
}
```

**OLB:**
```yaml
listeners:
  - name: https
    address: ":443"
    protocol: https
    tls:
      cert_file: /etc/olb/certs/cert.pem
      key_file: /etc/olb/certs/key.pem
    routes:
      - path: /
        pool: backend
```

### Rate Limiting

**NGINX:**
```nginx
limit_req_zone $binary_remote_addr zone=one:10m rate=10r/s;
limit_req zone=one burst=20 nodelay;
```

**OLB:**
```yaml
middleware:
  rate_limit:
    enabled: true
    requests_per_second: 10
    burst: 20
```

### Health Checks

**NGINX Plus:**
```nginx
upstream backend {
    server 10.0.1.10:8080;
    server 10.0.1.11:8080;
    health_check interval=5s fails=3 passes=2;
}
```

**OLB:**
```yaml
pools:
  - name: backend
    algorithm: round_robin
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
    health_check:
      type: http
      path: /health
      interval: 5s
      timeout: 3s
      healthy_threshold: 2
      unhealthy_threshold: 3
```

---

## From HAProxy

### Basic TCP Load Balancing

**HAProxy:**
```haproxy
global
    maxconn 4096

defaults
    mode tcp
    timeout connect 5s
    timeout client 30s
    timeout server 30s

frontend tcp_frontend
    bind *:3306
    default_backend mysql_backend

backend mysql_backend
    balance roundrobin
    server mysql1 10.0.1.10:3306 check
    server mysql2 10.0.1.11:3306 check
```

**OLB:**
```yaml
listeners:
  - name: mysql
    address: ":3306"
    protocol: tcp
    pool: mysql_backend

pools:
  - name: mysql_backend
    algorithm: round_robin
    backends:
      - address: "10.0.1.10:3306"
      - address: "10.0.1.11:3306"
    health_check:
      type: tcp
      interval: 5s
```

### Sticky Sessions

**HAProxy:**
```haproxy
backend web_backend
    balance roundrobin
    cookie SESSION insert indirect nocache
    server web1 10.0.1.10:8080 cookie web1
    server web2 10.0.1.11:8080 cookie web2
```

**OLB:**
```yaml
pools:
  - name: web
    algorithm: sticky_sessions
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
```

### SSL Pass-through

**HAProxy:**
```haproxy
frontend ssl_frontend
    bind *:443
    option tcplog
    default_backend ssl_backend

backend ssl_backend
    balance source
    server ssl1 10.0.1.10:443 check
```

**OLB:**
```yaml
listeners:
  - name: ssl
    address: ":443"
    protocol: tcp
    tls:
      passthrough: true
    pool: ssl_backend

pools:
  - name: ssl_backend
    algorithm: ip_hash
    backends:
      - address: "10.0.1.10:443"
```

---

## From Traefik

### Docker Integration

**Traefik:**
```yaml
# docker-compose.yml
services:
  traefik:
    image: traefik:v2.10
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
  app:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.app.rule=Host(`app.example.com`)"
```

**OLB:**
```yaml
discovery:
  - type: docker
    docker:
      endpoint: unix:///var/run/docker.sock
      filters:
        - label=olb.enable=true
```

### Let's Encrypt

**Traefik:**
```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@example.com
      storage: acme.json
      tlsChallenge: {}
```

**OLB:**
```yaml
acme:
  enabled: true
  email: admin@example.com
  directory: https://acme-v02.api.letsencrypt.org/directory
  storage: /var/lib/olb/acme.json
```

---

## From Envoy

### Basic Configuration

**Envoy:**
```yaml
static_resources:
  listeners:
    - name: listener_0
      address:
        socket_address: { address: 0.0.0.0, port_value: 80 }
      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                route_config:
                  virtual_hosts:
                    - routes:
                        - match: { prefix: "/" }
                          route: { cluster: service_backend }
  clusters:
    - name: service_backend
      connect_timeout: 5s
      lb_policy: ROUND_ROBIN
      load_assignment:
        endpoints:
          - lb_endpoints:
              - endpoint: { address: { socket_address: { address: 10.0.1.10, port_value: 8080 } } }
              - endpoint: { address: { socket_address: { address: 10.0.1.11, port_value: 8080 } } }
```

**OLB:**
```yaml
listeners:
  - name: http
    address: ":80"
    routes:
      - path: /
        pool: service_backend

pools:
  - name: service_backend
    algorithm: round_robin
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
    health_check:
      type: http
      interval: 5s
```

---

## From AWS ALB

### Basic Configuration

**AWS ALB (Terraform):**
```hcl
resource "aws_lb" "main" {
  name               = "main-alb"
  load_balancer_type = "application"
  subnets            = var.subnet_ids
}

resource "aws_lb_target_group" "app" {
  name     = "app-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = var.vpc_id
}

resource "aws_lb_target_group_attachment" "app" {
  target_group_arn = aws_lb_target_group.app.arn
  target_id        = "i-1234567890abcdef0"
  port             = 80
}
```

**OLB on AWS:**
See `deploy/terraform/examples/aws/` for complete AWS deployment with OLB.

---

## Migration Checklist

- [ ] Export current configuration
- [ ] Map load balancing algorithms
- [ ] Convert health check settings
- [ ] Migrate TLS certificates
- [ ] Test in staging environment
- [ ] Update DNS/load balancer endpoints
- [ ] Monitor error rates and latency
- [ ] Update monitoring dashboards
- [ ] Document new endpoints
- [ ] Train team on new CLI/commands
