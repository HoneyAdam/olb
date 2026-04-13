export interface Backend {
  id: string
  address: string
  weight: number
  status: 'up' | 'down' | 'draining' | 'starting'
  health: 'healthy' | 'unhealthy' | 'unknown'
  response_time_ms: number
  active_connections: number
  total_requests: number
}

export interface Pool {
  id: string
  name: string
  algorithm: string
  backends: Backend[]
  health_check?: {
    enabled?: boolean
    type: string
    path: string
    interval: string
  }
  total_requests: number
  active_connections: number
}

export interface Listener {
  id: string
  name: string
  address: string
  protocol: 'http' | 'https' | 'tcp' | 'udp'
  routes: Route[]
  enabled: boolean
}

export interface Route {
  id: string
  path: string
  pool: string
  methods: string[]
  strip_prefix: boolean
  priority: number
}

export interface SystemStatus {
  version: string
  commit: string
  build_date: string
  uptime: string
  state: 'stopped' | 'starting' | 'running' | 'stopping'
  go_version: string
}

export interface HealthStatus {
  status: 'healthy' | 'unhealthy'
  checks: Record<string, {
    status: string
    message: string
  }>
  timestamp: string
}

// API response types matching server-side admin/types.go
export interface APIPoolInfo {
  name: string
  algorithm: string
  backends: APIBackendInfo[]
  healthy_count?: number
  health_check?: {
    type: string
    path: string
    interval: string
    timeout: string
  }
}

export interface APIBackendInfo {
  id: string
  address: string
  weight: number
  state: string
  healthy: boolean
  requests: number
  errors: number
}

export interface APIRouteInfo {
  name: string
  host: string
  path: string
  methods: string[]
  headers: Record<string, string>
  backend_pool: string
  priority: number
}

export interface APICertificateInfo {
  names: string[]
  expiry: string
  is_wildcard: boolean
}

export interface APIWAFStatus {
  enabled: boolean
  mode?: string
  rules?: Record<string, unknown>
  detections?: Record<string, number>
}

export interface APIClusterStatus {
  node_id: string
  state: string
  leader: string
  peers: string[]
  applied_index: number
  commit_index: number
  term: number
  vote: string
}

export interface APIClusterMember {
  id: string
  address: string
  state: string
}

export interface APIMiddlewareStatusItem {
  id: string
  name: string
  description: string
  enabled: boolean
  category: string
}

export interface APIEventItem {
  id: string
  type: string
  message: string
  timestamp: string
}

// Full runtime config shape (matches Go config struct)
export interface OLBConfig {
  logging?: { level?: string; format?: string; output?: string }
  server?: {
    max_connections?: number
    max_connections_per_source?: number
    max_connections_per_backend?: number
    proxy_timeout?: string
    dial_timeout?: string
    max_retries?: number
    max_idle_conns?: number
    max_idle_conns_per_host?: number
    drain_timeout?: string
  }
  admin?: {
    address?: string
    rate_limit_max_requests?: number | string
    rate_limit_window?: string
    mcp_audit?: boolean
    mcp_address?: string
  }
  cluster?: {
    enabled?: boolean
    node_id?: string
    bind_addr?: string
    bind_port?: number
    peers?: string[]
    data_dir?: string
    election_tick?: string
    heartbeat_tick?: string
  }
  listeners?: Array<{ name: string; address: string; protocol?: string; routes?: Array<{ path: string }> }>
  pools?: Array<{ name: string; algorithm: string; backends?: Array<Record<string, unknown>>; health_check?: { type: string; path: string; interval: string } }>
  tls?: { enabled?: boolean; cert_file?: string; key_file?: string; acme?: { enabled?: boolean; email?: string } }
  waf?: {
    enabled?: boolean
    mode?: string
    ip_acl?: { enabled?: boolean }
    rate_limit?: { enabled?: boolean }
    sanitizer?: { enabled?: boolean }
    detection?: { enabled?: boolean }
    bot_detection?: { enabled?: boolean }
    response?: { security_headers?: { enabled?: boolean } }
  }
  middleware?: {
    cors?: { enabled?: boolean; allowed_origins?: string[]; allowed_methods?: string[] }
  }
  [key: string]: unknown
}
