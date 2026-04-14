# OpenLoadBalancer Full Configuration Example (HCL)
# This is a comprehensive configuration demonstrating all features.
# Functionally equivalent to olb.yaml.

version = 1

# ---------------------------------------------------------------------------
# Global settings
# ---------------------------------------------------------------------------
global {
  # Worker settings
  workers {
    count = "auto"  # "auto" = number of CPUs
  }

  # Connection limits
  limits {
    max_connections            = 10000
    max_connections_per_source = 100
    max_connections_per_backend = 1000
  }

  # Timeouts (duration strings)
  timeouts {
    read   = "30s"
    write  = "30s"
    idle   = "120s"
    header = "10s"
    drain  = "30s"
  }
}

# ---------------------------------------------------------------------------
# Admin API configuration
# ---------------------------------------------------------------------------
admin {
  enabled = true
  address = "127.0.0.1:8081"

  auth {
    type     = "basic"
    username = "admin"
    # IMPORTANT: Change this password before deploying!
    # Generate a bcrypt hash: go run -mod=mod github.com/tyler-smith/go-bcrypt-cli <password>
    password = "$2a$10$CHANGEME_REPLACE_WITH_YOUR_OWN_BCRYPT_HASH"
  }
}

# ---------------------------------------------------------------------------
# Metrics configuration
# ---------------------------------------------------------------------------
metrics {
  enabled = true
  path    = "/metrics"
}

# ---------------------------------------------------------------------------
# Listeners
# ---------------------------------------------------------------------------

# HTTP listener on port 8080
listener "http" {
  protocol = "http"
  address  = ":8080"

  # API routes with specific backend
  route "api" {
    host    = "api.example.com"
    path    = "/api/"
    methods = ["GET", "POST", "PUT", "DELETE"]
    pool    = "api-backend"
  }

  # Static file serving (direct backend)
  route "static" {
    host = "static.example.com"
    path = "/files/"
    pool = "static-backend"
  }

  # Default route for everything else
  route "default" {
    path = "/"
    pool = "web-backend"

    middleware "rate_limit" {
      requests_per_second = 100
      burst_size          = 200
    }
  }
}

# HTTPS listener on port 8443
listener "https" {
  protocol = "https"
  address  = ":8443"

  tls {
    cert_file = "/etc/olb/certs/server.crt"
    key_file  = "/etc/olb/certs/server.key"
  }

  route "secure-api" {
    path = "/"
    pool = "api-backend"
  }
}

# ---------------------------------------------------------------------------
# Backend pools with different algorithms
# ---------------------------------------------------------------------------

# Web pool - round robin for general traffic
pool "web-backend" {
  algorithm = "round_robin"

  health_check {
    type                = "http"
    path                = "/health"
    interval            = "10s"
    timeout             = "5s"
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  backend "web-1" {
    address = "10.0.1.10:8080"
    weight  = 1
  }

  backend "web-2" {
    address = "10.0.1.11:8080"
    weight  = 1
  }

  backend "web-3" {
    address = "10.0.1.12:8080"
    weight  = 1
  }
}

# API pool - weighted round robin for API servers
pool "api-backend" {
  algorithm = "weighted_round_robin"

  health_check {
    type            = "http"
    path            = "/api/health"
    interval        = "5s"
    timeout         = "3s"
    expected_status = 200
  }

  backend "api-1" {
    address = "10.0.2.10:8080"
    weight  = 3  # More capacity
  }

  backend "api-2" {
    address = "10.0.2.11:8080"
    weight  = 2
  }
}

# Static files pool - least connections for large files
pool "static-backend" {
  algorithm = "round_robin"

  health_check {
    type     = "tcp"
    interval = "10s"
    timeout  = "5s"
  }

  backend "static-1" {
    address = "10.0.3.10:8080"
  }
}

# ---------------------------------------------------------------------------
# Middleware configuration
# ---------------------------------------------------------------------------

# Request ID injection
middleware "request_id" {
  enabled = true

  config {
    header_name    = "X-Request-ID"
    trust_incoming = false
  }
}

# Real IP extraction
middleware "real_ip" {
  enabled = true

  config {
    trusted_proxies = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
  }
}

# CORS handling
middleware "cors" {
  enabled = true

  config {
    allowed_origins = ["*"]
    allowed_methods = ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers = ["Content-Type", "Authorization", "X-Request-ID"]
    max_age         = 3600
  }
}

# Security headers
middleware "headers" {
  enabled = true

  config {
    security_preset = "strict"

    response_set = {
      X-Frame-Options        = "DENY"
      X-Content-Type-Options = "nosniff"
    }
  }
}

# Compression
middleware "compression" {
  enabled = true

  config {
    min_size = 1024
    level    = "default"  # -1 = default compression
  }
}

# Access logging
middleware "access_log" {
  enabled = true

  config {
    format = "json"
    output = "/var/log/olb/access.log"
  }
}

# Metrics collection
middleware "metrics" {
  enabled = true
}

# ---------------------------------------------------------------------------
# Logging configuration
# ---------------------------------------------------------------------------
logging {
  level  = "info"
  output = "stdout"
  format = "json"

  # File output (optional)
  # file {
  #   path        = "/var/log/olb/olb.log"
  #   max_size    = "100MB"
  #   max_backups = 5
  #   max_age     = 30
  #   compress    = true
  # }
}
