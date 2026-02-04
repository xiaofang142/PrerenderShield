# PrerenderShield 使用示例

## 目录
1. [基础配置示例](#基础配置示例)
2. [API使用示例](#api使用示例)
3. [站点管理示例](#站点管理示例)
4. [防火墙配置示例](#防火墙配置示例)
5. [监控配置示例](#监控配置示例)
6. [高级配置示例](#高级配置示例)

## 基础配置示例

### 最小化配置
```yaml
server:
  address: "0.0.0.0"
  api_port: 9598
  console_port: 9597

dirs:
  data_dir: ./data
  static_dir: ./static
  admin_static_dir: ./web

cache:
  type: "redis"
  redis_url: "localhost:6379"

sites:
  - id: "demo-site"
    name: "Demo Site"
    domains: ["localhost", "127.0.0.1"]
    port: 8080
    mode: "static"
    prerender:
      enabled: true
      pool_size: 3
      timeout: 30
      cache_ttl: 3600
```

### 完整配置示例
```yaml
server:
  address: "0.0.0.0"
  api_port: 9598
  console_port: 9597
  public_api_url: "http://your-domain.com:9598"

dirs:
  data_dir: /var/lib/prerender-shield
  static_dir: /var/www/static
  certs_dir: /var/lib/prerender-shield/certs
  admin_static_dir: ./web

cache:
  type: "redis"
  redis_url: "redis-cluster:6379"
  redis_password: "your-redis-password"
  redis_db: 1
  memory_size: 2000

storage:
  type: "redis"

monitoring:
  enabled: true
  prometheus_address: ":9090"

app:
  version: "1.0.1"
  official_url: "https://prerender.websitetool.cn"

sites:
  - id: "production-site"
    name: "Production Site"
    domains: ["example.com", "www.example.com"]
    port: 8080
    mode: "proxy"
    proxy:
      target_url: "http://backend-service:3000"
    firewall:
      enabled: true
      rules_path: "./rules"
      action:
        default_action: "block"
        block_message: "Request blocked by security policy"
      geoip:
        enabled: true
        allow_list: ["CN", "US", "JP"]
        block_list: ["KP", "IR", "SY"]
      rate_limit:
        enabled: true
        requests: 100
        window: 60
        ban_time: 3600
      blacklist: ["192.168.1.100", "10.0.0.50"]
      whitelist: ["192.168.1.10", "10.0.0.10"]
    prerender:
      enabled: true
      pool_size: 10
      min_pool_size: 2
      max_pool_size: 20
      timeout: 45
      cache_ttl: 7200
      idle_timeout: 600
      dynamic_scaling: true
      scaling_factor: 0.7
      scaling_interval: 120
      preheat:
        enabled: true
        sitemap_url: "https://example.com/sitemap.xml"
        schedule: "0 2 * * *"  # 每天凌晨2点执行
        concurrency: 10
        max_depth: 5
      crawler_headers:
        - "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
        - "Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)"
        - "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)"
      use_default_headers: true
    routing:
      rules: []
    file_integrity:
      enabled: true
      check_interval: 300
      hash_algorithm: "sha256"
```

## API使用示例

### 认证和获取令牌
```bash
# 登录获取JWT令牌
curl -X POST http://localhost:9598/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "123456"
  }'
```

### 健康检查
```bash
# 检查服务健康状态
curl http://localhost:9598/api/v1/health
```

### 系统信息
```bash
# 获取版本信息
curl http://localhost:9598/api/v1/version

# 获取系统配置
curl -H "Authorization: Bearer $TOKEN" http://localhost:9598/api/v1/system/config
```

## 站点管理示例

### 创建新站点
```bash
curl -X POST http://localhost:9598/api/v1/sites \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-new-site",
    "name": "My New Site",
    "domains": ["mysite.com", "www.mysite.com"],
    "port": 8081,
    "mode": "static",
    "firewall": {
      "enabled": true,
      "rate_limit": {
        "enabled": true,
        "requests": 50,
        "window": 60
      }
    },
    "prerender": {
      "enabled": true,
      "pool_size": 5,
      "timeout": 30,
      "cache_ttl": 3600
    }
  }'
```

### 更新站点配置
```bash
curl -X PUT http://localhost:9598/api/v1/sites/my-new-site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-new-site",
    "name": "Updated Site Name",
    "domains": ["mysite.com", "www.mysite.com", "newdomain.com"],
    "port": 8081,
    "mode": "static",
    "firewall": {
      "enabled": true,
      "rate_limit": {
        "enabled": true,
        "requests": 100,
        "window": 60
      }
    },
    "prerender": {
      "enabled": true,
      "pool_size": 8,
      "timeout": 45,
      "cache_ttl": 7200
    }
  }'
```

### 获取站点列表
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:9598/api/v1/sites
```

## 防火墙配置示例

### 更新站点防火墙配置
```bash
curl -X PUT http://localhost:9598/api/v1/sites/my-new-site/waf \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "rate_limit_count": 100,
    "rate_limit_window": 60,
    "custom_block_page": "<html><body><h1>Access Denied</h1><p>Your request has been blocked by security policy.</p></body></html>"
  }'
```

### 添加IP到黑名单
```bash
curl -X POST http://localhost:9598/api/v1/firewall/blacklist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "site_id": "my-new-site",
    "ip": "192.168.1.100"
  }'
```

### 添加IP到白名单
```bash
curl -X POST http://localhost:9598/api/v1/firewall/whitelist \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "site_id": "my-new-site",
    "ip": "192.168.1.10"
  }'
```

### 获取访问日志
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:9598/api/v1/logs?page=1&limit=20&site_id=my-new-site"
```

## 监控配置示例

### 获取监控统计
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:9598/api/v1/monitoring/stats
```

### 获取爬虫统计
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:9598/api/v1/crawler/stats
```

### 获取爬虫日志
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:9598/api/v1/crawler/logs?page=1&limit=20"
```

## 渲染预热示例

### 获取预热统计
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:9598/api/v1/preheat/stats
```

### 触发预热
```bash
curl -X POST http://localhost:9598/api/v1/preheat/trigger \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "siteName": "my-new-site"
  }'
```

### 获取预热URL列表
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:9598/api/v1/preheat/urls?site=my-new-site&page=1&size=20"
```

### 获取爬虫头信息
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:9598/api/v1/preheat/crawler-headers
```

### 清除缓存
```bash
curl -X POST http://localhost:9598/api/v1/preheat/clear-cache \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "siteName": "my-new-site"
  }'
```

## 高级配置示例

### 多站点配置
```yaml
sites:
  - id: "frontend-app"
    name: "Frontend Application"
    domains: ["app.example.com"]
    port: 8080
    mode: "proxy"
    proxy:
      target_url: "http://frontend-service:3000"
    firewall:
      enabled: true
      rate_limit:
        enabled: true
        requests: 200
        window: 60
    prerender:
      enabled: true
      pool_size: 5
      timeout: 30
      cache_ttl: 3600
  
  - id: "api-gateway"
    name: "API Gateway"
    domains: ["api.example.com"]
    port: 8081
    mode: "proxy"
    proxy:
      target_url: "http://api-service:8000"
    firewall:
      enabled: true
      rules_path: "./api-rules"
    prerender:
      enabled: false  # API不需要预渲染
  
  - id: "static-assets"
    name: "Static Assets"
    domains: ["assets.example.com"]
    port: 8082
    mode: "static"
    firewall:
      enabled: false  # 静态资源不需要复杂防护
    prerender:
      enabled: false
```

### 自定义路由规则
```yaml
sites:
  - id: "complex-site"
    name: "Complex Site"
    domains: ["example.com"]
    port: 8080
    mode: "proxy"
    proxy:
      target_url: "http://main-service:3000"
    routing:
      rules:
        - id: "api-route"
          domain: "example.com"
          pattern: "/api/*"
          action: "proxy"
          priority: 100
          params:
            target_url: "http://api-service:8000"
        - id: "admin-route"
          domain: "example.com"
          pattern: "/admin/*"
          action: "proxy"
          priority: 90
          params:
            target_url: "http://admin-service:9000"
    firewall:
      enabled: true
      rate_limit:
        enabled: true
        requests: 50
        window: 60
    prerender:
      enabled: true
      pool_size: 3
      timeout: 30
```

## 命令行工具示例

### 启动服务
```bash
# 使用默认配置文件
./api --config config.yml

# 使用环境变量覆盖配置
API_PORT=8080 CONSOLE_PORT=8081 ./api --config config.yml
```

### 测试脚本示例
```bash
#!/bin/bash
# health-check.sh - 健康检查脚本

API_URL="http://localhost:9598"
TOKEN="your-jwt-token-here"

# 检查API服务健康状态
health_status=$(curl -s $API_URL/api/v1/health | jq -r '.data.status')

if [ "$health_status" = "running" ]; then
    echo "✅ Service is healthy"
else
    echo "❌ Service is unhealthy: $health_status"
    exit 1
fi

# 检查站点数量
site_count=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/api/v1/sites | jq -r '.data | length')

if [ "$site_count" -gt 0 ]; then
    echo "✅ Found $site_count sites"
else
    echo "⚠️  No sites configured"
fi

echo "Health check completed at $(date)"
```

## 性能调优示例

### 高流量场景配置
```yaml
# 针对高流量优化的配置
sites:
  - id: "high-traffic-site"
    name: "High Traffic Site"
    domains: ["popular-site.com"]
    port: 8080
    mode: "static"
    firewall:
      enabled: true
      rate_limit:
        enabled: true
        requests: 1000  # 更高的速率限制
        window: 60
        ban_time: 1800  # 30分钟封禁
    prerender:
      enabled: true
      pool_size: 20     # 增加浏览器池大小
      min_pool_size: 10
      max_pool_size: 50 # 最大池大小
      timeout: 60       # 增加超时时间
      cache_ttl: 14400  # 4小时缓存
      idle_timeout: 900 # 15分钟空闲超时
      dynamic_scaling: true
      scaling_factor: 0.8
      scaling_interval: 60
```

### 低资源环境配置
```yaml
# 针对低资源环境优化的配置
sites:
  - id: "low-resource-site"
    name: "Low Resource Site"
    domains: ["small-site.com"]
    port: 8080
    mode: "static"
    firewall:
      enabled: true
      rate_limit:
        enabled: true
        requests: 20
        window: 60
    prerender:
      enabled: true
      pool_size: 2      # 减少浏览器池大小
      min_pool_size: 1
      max_pool_size: 5  # 最大池大小
      timeout: 30
      cache_ttl: 1800   # 30分钟缓存
      idle_timeout: 300 # 5分钟空闲超时
      dynamic_scaling: false
```

## 监控集成示例

### Prometheus配置
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'prerender-shield'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

### Grafana面板配置示例
```
{
  "dashboard": {
    "title": "PrerenderShield Monitor",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(prerender_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Cache Hit Rate",
        "targets": [
          {
            "expr": "prerender_cache_hits_total / (prerender_cache_hits_total + prerender_cache_misses_total)"
          }
        ]
      },
      {
        "title": "Active Browsers",
        "targets": [
          {
            "expr": "prerender_active_browsers"
          }
        ]
      }
    ]
  }
}
```

这些示例涵盖了PrerenderShield的各种使用场景，从基础配置到高级部署，可以帮助用户快速上手和高效使用该系统。