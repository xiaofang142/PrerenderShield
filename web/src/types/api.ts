export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface Site {
  id: string
  name: string
  domains: string[]
  port: number
  mode: string
  proxy?: { target_url: string }
  prerender?: PrerenderConfig
  firewall?: FirewallConfig
}

export interface PrerenderConfig {
  enabled: boolean
  pool_size?: number
  timeout?: number
  cache_ttl?: number
  preheat?: {
    sitemap_url?: string
    schedule?: string
    concurrency?: number
  }
}

export interface FirewallConfig {
  enabled: boolean
  rules_path?: string
  rate_limit?: {
    enabled: boolean
    requests: number
    window: number
  }
}

export interface UserInfo {
  id: string
  username: string
  role?: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  username: string
}

export interface OverviewData {
  total_requests: number
  today_requests: number
  active_sites: number
  attack_count: number
  block_count: number
  cache_hit_rate: number
}

export interface CrawlerLog {
  id: string
  url: string
  user_agent: string
  crawler_type: string
  render_time: number
  cache_hit: boolean
  created_at: string
}

export interface AttackLog {
  id: string
  rule_id: string
  rule_name: string
  threat_type: string
  source_ip: string
  target_url: string
  action: string
  severity: string
  created_at: string
}

export interface PreheatTask {
  task_id: string
  site_id: string
  urls: string[]
  total_urls: number
  completed_urls: number
  status: string
}

export interface SSLCertificate {
  domain: string
  subject: string
  issuer: string
  not_before: string
  not_after: string
  dns_names: string[]
  expires_in: number
  expired: boolean
}

export interface SystemConfig {
  server: {
    address: string
    api_port: number
    console_port: number
  }
  cache: {
    redis_url: string
  }
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  limit: number
}
