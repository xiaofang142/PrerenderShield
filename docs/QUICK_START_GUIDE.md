# PrerenderShield 快速入门指南

## 🚀 一键安装

```bash
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield
./install.sh
```

> 安装后访问 `http://服务器IP:9597`，首次访问时在登录页设置管理员账号密码（无预置默认账号）

## 防火墙配置

```bash
# Ubuntu / Debian
sudo ufw allow 9597/tcp
sudo ufw allow 9598/tcp

# CentOS / RHEL
sudo firewall-cmd --add-port=9597/tcp --permanent
sudo firewall-cmd --add-port=9598/tcp --permanent
sudo firewall-cmd --reload
```

> 云平台安全组: 放行 9597/9598（管理控制台与 API）及站点端口（如 8084）所在 TCP 端口；官网 https://prerender.websitetool.cn

## 概述

PrerenderShield 是一款集防火墙安全防护与渲染预热功能于一体的Web应用中间件，旨在解决前后端分离架构下的SEO优化和安全防护痛点。

## 安装准备

### 系统要求

| 组件 | 最低要求 | 推荐配置 |
|------|---------|---------|
| **操作系统** | Linux (Ubuntu 20.04+, CentOS 8+, openSUSE, Alpine) / macOS 12+ | Linux (Ubuntu 22.04 LTS) |
| **CPU** | 2 核 | 4 核 |
| **内存** | 4 GB | 8 GB |
| **磁盘空间** | 10 GB | 20 GB (SSD) |
| **网络** | 可访问公网 | 稳定的网络连接 |
| **架构** | x86_64, arm64 | x86_64 |

### 依赖软件

- **Redis** (>= 5.0)
- **Go** (>= 1.25) - 如果从源码编译
- **Node.js** (>= 18，推荐 20) - 如果从源码编译前端
- **Git**

## 快速安装

### 方式一：使用预编译二进制文件（推荐）

```bash
# 1. 克隆代码仓库
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield

# 2. 给安装脚本添加执行权限
chmod +x install.sh

# 3. 执行安装脚本
./install.sh
```

安装脚本会自动：
- 检测操作系统和架构，选择 Docker / 源码 / 二进制 安装模式
- 检查并安装Redis（如未安装）
- 检查并安装谷歌无头浏览器（Chromium/Chrome）
- 生成默认配置文件（Linux 下同时注册 systemd 服务）
- 执行安装后的健康检查
- 输出访问地址与安装目录

### 方式二：从源码构建

```bash
# 1. 克隆代码仓库
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield

# 2. 给构建脚本添加执行权限
chmod +x build.sh

# 3. 执行构建脚本
./build.sh
```

构建脚本会自动：
- 检查Go和Node.js环境
- 配置Go模块镜像加速
- 自动获取当前平台信息
- 构建前端（安装依赖、交互式设置API地址、构建）
- 安装Go依赖
- 构建当前平台的二进制文件
- 将前端代码从web/dist拷贝到bin/web
- 构建产物验证和测试
- 输出构建结果和使用说明

## 配置文件详解

### 基础配置

创建配置文件 `config.yml`：

```yaml
# 服务器配置
server:
  address: "0.0.0.0"      # 监听地址
  api_port: 9598          # API服务端口
  console_port: 9597      # 管理控制台端口
  public_api_url: "http://your-domain.com:9598"  # API公网地址

# 目录配置
dirs:
  data_dir: /var/lib/prerender-shield  # 数据目录
  static_dir: /var/www/static          # 静态文件目录
  certs_dir: /var/lib/prerender-shield/certs  # 证书目录
  admin_static_dir: ./web              # 管理界面静态文件目录

# 缓存配置
cache:
  type: "redis"           # 缓存类型
  redis_url: "localhost:6379"  # Redis连接地址
  redis_password: ""       # Redis密码（可选）
  redis_db: 0             # Redis数据库索引
  memory_size: 1000       # 内存缓存大小

# 监控配置
monitoring:
  enabled: true           # 启用监控
  prometheus_address: ":9090"  # Prometheus暴露地址

# 站点配置
sites:
  - id: "my-site"         # 站点ID（必须唯一）
    name: "My Website"    # 站点名称
    domains:              # 域名列表
      - "example.com"
      - "www.example.com"
    port: 8080            # 站点端口
    mode: "static"        # 运行模式：static/proxy/redirect
    firewall:             # 防火墙配置
      enabled: true
      rate_limit:
        enabled: true
        requests: 100     # 每分钟请求数限制
        window: 60        # 时间窗口（秒）
    prerender:            # 渲染预热配置
      enabled: true
      pool_size: 5        # 浏览器池大小
      timeout: 30         # 渲染超时时间（秒）
      cache_ttl: 3600     # 缓存过期时间（秒）
```

### 高级配置

查看完整的增强配置示例：
```bash
cat enhanced-config-example.yml
```

## 启动服务

### 使用启动脚本（推荐）

```bash
# 启动服务
./start.sh start

# 查看服务状态
./start.sh status

# 重启服务
./start.sh restart

# 停止服务
./start.sh stop
```

### 直接运行二进制文件

```bash
# 启动服务
./bin/api --config config.yml

# 或者在后台运行
./bin/api --config config.yml &
```

## 基本使用

### 1. 访问管理界面

服务启动后，通过以下地址访问管理界面：
- 管理界面：`http://your-server-ip:9597`
- 管理员账号：首次访问时自行设置

**注意**：管理员账号在首次访问时由你自行设置，请妥善保管并定期更换密码（设置 → 修改密码）。

### 2. API访问

API服务地址：
- API服务：`http://your-server-ip:9598`
- 健康检查：`http://your-server-ip:9598/api/v1/health`

### 3. 获取认证令牌

```bash
curl -X POST http://your-server-ip:9598/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "<你设置的管理员账号>",
    "password": "<你设置的管理员密码>"
  }'
```

## 核心功能配置

### ⚠️ 关键一步：让真实流量经过 Prerender Shield

添加站点后，Shield 只在「站点端口」上提供服务（默认站点端口 8084）。
**必须将用户与搜索引擎的访问引导到该端口**，SEO 预渲染与 WAF 才会生效。二选一：

#### 方式 A：DNS 直连（推荐独立服务器场景）

把站点域名的 DNS A/AAAA 记录直接指向运行 Shield 的服务器 IP，
并确保站点配置的端口对外开放（如 `www.example.com → 服务器IP:8084`）。

> 使用 80/443 端口可省去 URL 端口；Linux 下可用 `setcap` 或 systemd `AmbientCapabilities=CAP_NET_BIND_SERVICE` 让非 root 进程绑定低端口。

#### 方式 B：Nginx 反向代理（已有 Web 架构场景）

保留原域名解析不变，在原 Nginx 中把流量转发给 Shield：

```nginx
server {
    listen 443 ssl;
    server_name www.example.com;

    location / {
        proxy_pass http://127.0.0.1:8084;   # 指向站点的 Shield 端口
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

> 注意：`X-Forwarded-For` 必须传递，否则 WAF 的 IP 黑白名单与限流看到的是 Nginx IP。

#### 验证流量已接入

```bash
# 模拟搜索引擎爬虫请求任意页面
curl -A "Mozilla/5.0 (compatible; Googlebot/2.1)" http://127.0.0.1:8084/some-page -i

# 返回应为完整渲染 HTML（而非 SPA 空壳），且控制台「爬虫日志」出现记录
```

### 站点管理

#### 添加静态站点

```bash
curl -X POST http://your-server-ip:9598/api/v1/sites \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "static-site",
    "name": "Static Site",
    "domains": ["static.example.com"],
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
      "pool_size": 5,
      "timeout": 30,
      "cache_ttl": 3600
    }
  }'
```

#### 添加代理站点

```bash
curl -X POST http://your-server-ip:9598/api/v1/sites \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "proxy-site",
    "name": "Proxy Site",
    "domains": ["api.example.com"],
    "port": 8082,
    "mode": "proxy",
    "proxy": {
      "target_url": "http://backend-service:3000"
    },
    "firewall": {
      "enabled": true,
      "rate_limit": {
        "enabled": true,
        "requests": 200,
        "window": 60
      }
    },
    "prerender": {
      "enabled": false
    }
  }'
```

### 防火墙配置

#### 更新站点防火墙配置

```bash
curl -X PUT http://your-server-ip:9598/api/v1/sites/SITE_ID/waf \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "rate_limit_count": 150,
    "rate_limit_window": 60,
    "custom_block_page": "<html><body><h1>Access Denied</h1></body></html>"
  }'
```

#### 添加IP到黑名单

```bash
curl -X POST http://your-server-ip:9598/api/v1/firewall/blacklist \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "site_id": "SITE_ID",
    "ip": "192.168.1.100"
  }'
```

### 渲染预热配置

#### 触发预热

```bash
curl -X POST http://your-server-ip:9598/api/v1/preheat/trigger \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "siteId": "SITE_ID"
  }'
```

#### 清除缓存

```bash
curl -X POST http://your-server-ip:9598/api/v1/preheat/clear-cache \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "siteId": "SITE_ID"
  }'
```

## 监控与健康检查

### 健康检查

```bash
# 检查服务健康状态
curl http://your-server-ip:9598/api/v1/health

# 检查系统监控统计
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" http://your-server-ip:9598/api/v1/monitoring/stats
```

### Prometheus集成

如果启用了监控，可以通过以下地址获取Prometheus指标：
```
http://your-server-ip:9090/metrics
```

## 故障排查

### 常见问题

1. **服务无法启动**
   - 检查端口是否被占用
   - 检查Redis服务是否运行
   - 查看日志文件 `data/prerender-shield.log`

2. **站点无法访问**
   - 检查站点配置是否正确
   - 确认端口配置与访问端口一致
   - 检查域名配置是否匹配

3. **渲染预热失败**
   - 确认Chromium/Chrome已正确安装
   - 检查目标页面是否可访问
   - 增加渲染超时时间

### 日志查看

```bash
# 查看主日志（启动/运行日志，start.sh 与 install.sh 均写此文件）
tail -f data/prerender-shield.log

# 访问日志 / 爬虫日志 / 攻击日志存储在 Redis 中，
# 请在管理控制台「访问日志 / 爬虫日志 / 攻击日志」页面查看，
# 或通过 API 导出：GET /api/v1/logs、GET /api/v1/crawler/logs、GET /api/v1/firewall/attacks
```

### 健康检查脚本

创建一个简单的健康检查脚本：

```bash
#!/bin/bash
API_URL="http://your-server-ip:9598"
TOKEN="YOUR_JWT_TOKEN"

echo "=== PrerenderShield 健康检查 ==="
echo "时间: $(date)"

# 检查API服务
HEALTH_STATUS=$(curl -s $API_URL/api/v1/health | jq -r '.data.status' 2>/dev/null)
if [ "$HEALTH_STATUS" = "running" ]; then
    echo "✅ API服务: 正常运行"
else
    echo "❌ API服务: 异常 ($HEALTH_STATUS)"
fi

# 检查站点数量
SITE_COUNT=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/api/v1/sites | jq -r '.data | length' 2>/dev/null)
if [ "$SITE_COUNT" -gt 0 ]; then
    echo "✅ 站点数量: $SITE_COUNT 个"
else
    echo "⚠️  站点数量: 0 个"
fi

# 检查监控统计
MONITOR_STATS=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/api/v1/monitoring/stats)
if [ $? -eq 0 ]; then
    REQUESTS_PER_SEC=$(echo $MONITOR_STATS | jq -r '.data.requestsPerSecond' 2>/dev/null)
    MEMORY_USAGE=$(echo $MONITOR_STATS | jq -r '.data.memoryUsage' 2>/dev/null)
    echo "📊 请求速率: $REQUESTS_PER_SEC RPS"
    echo "💾 内存使用: $MEMORY_USAGE%"
else
    echo "❌ 监控统计: 获取失败"
fi

echo "检查完成"
```

## 性能调优

### 渲染引擎优化

对于高流量站点，建议调整以下参数：

```yaml
prerender:
  pool_size: 10           # 增加浏览器池大小
  min_pool_size: 5        # 最小池大小
  max_pool_size: 20       # 最大池大小
  timeout: 60             # 增加超时时间
  cache_ttl: 14400        # 4小时缓存
  idle_timeout: 900       # 15分钟空闲超时
  dynamic_scaling: true   # 启用动态扩缩容
```

### 防火墙优化

```yaml
firewall:
  rate_limit:
    requests: 200         # 根据实际流量调整
    window: 60            # 时间窗口
    ban_time: 1800        # 封禁时间（秒）
```

## 安全最佳实践

1. **保护管理员凭据**：定期更换管理员密码（设置 → 修改密码）
2. **HTTPS配置**：为管理界面配置HTTPS
3. **防火墙规则**：限制对管理端口的访问
4. **定期更新**：保持系统和依赖的最新版本
5. **监控告警**：设置关键指标的告警

## 升级指南

### 从旧版本升级

1. 备份配置文件和数据
2. 停止当前服务
3. 下载新版本或从源码构建
4. 启动新版本服务
5. 验证功能正常

### 数据迁移

系统会自动处理数据迁移，但建议在升级前备份重要数据。

## 新功能速览（v3.x）

以下能力在本文早期版本中没有覆盖，配置键的完整说明见 [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md)：

- **缓存 TTL 分级规则**（`prerender.ttl_rules`）：有序规则按 URL 匹配（无 `*` 子串匹配，含 `*` 通配符正则），首中生效；未命中回退 `cache_ttl`，再回退引擎默认 24h：
  ```yaml
  prerender:
    ttl_rules:
      - pattern: "/product/*"
        ttl_seconds: 600
  ```
- **条件请求 304**：爬虫页响应自动携带 `ETag`（弱校验）与 `Last-Modified`，命中 `If-None-Match`/`If-Modified-Since` 时返回 304，节省爬虫抓取带宽；>1KB 且客户端支持时自动 gzip。
- **软过期降级**（`prerender.stale_while_revalidate`，默认开）：缓存过期后立即回旧值并异步重渲，避免爬虫等待。
- **爬虫分类策略**（`prerender.category_policy`）：对 `search`/`social`/`ai`/`generic` 四类爬虫分别指定 `render` / `cache_only` / `passthrough`。
- **渲染 URL 名单**（`prerender.include_patterns` / `exclude_patterns`）：按 RequestURI 正则控制哪些 URL 参与渲染；`max_concurrency` 可限制站点级渲染并发。
- **爬虫真实性验证**（`firewall.bot_verify`）：对自称 Googlebot 的请求做 rDNS 双向验证并在爬虫日志打标；`mode: block` 时仅拦截"确认伪造"的搜索爬虫（DNS 故障一律放行）。
- **管理 API Token**（顶层 `api_tokens`）：sha256 hex 列表，`Authorization: Bearer <token>` 仅可用于 `/api/v1/preheat/` 端点（CI 预热自动化），不能访问其他管理 API。
- **推送扩展**：`prerender.push` 支持百度/Bing 每日限额与 cron 计划，`indexnow_enabled`/`indexnow_key` 支持 IndexNow 协议。
- **SSL/ACME**：全局 `ssl` 节支持 Let's Encrypt 自动申请与续签，`ssl.dns` 支持 DNS-01（cloudflare/aliyun/tencentcloud 等）通配符证书；站点级 `sites[].ssl` 支持强制 HTTPS 与 HSTS。
- **商业授权**（`commercial`）：1 个站点永久免费全功能，多站点按 `max_sites`/年费授权；私有化部署可设 `max_sites: -1`。
- **管理控制台安全**：支持两步验证（2FA）、配置加密（`PRERENDER_MASTER_KEY`）、配置备份与恢复（`/api/v1/system/backup|restore`）。

## 支持与反馈

如需技术支持或功能建议：

- **GitHub Issues**: [问题反馈](https://github.com/xiaofang142/PrerenderShield/issues)
- **QQ群**: 973280483（技术交流、问题解答）
- **邮箱**: myloveisphp@126.com

---

**祝您使用愉快！** 
PrerenderShield - 让前后端分离网站既安全又SEO友好！

---

# 附录：高级配置与使用示例

> 以下内容合并自 USAGE_EXAMPLES.md


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
          pattern: "/api/*"
          action: "proxy"
          priority: 100
        - id: "admin-route"
          pattern: "/admin/*"
          action: "proxy"
          priority: 90
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

> 路由规则字段为 `id` / `pattern` / `action` / `priority`（数值越大越先）。

## 命令行工具示例

### 启动服务
```bash
# 使用默认配置文件
./bin/api --config config.yml

# 使用环境变量覆盖端口
SERVER_API_PORT=19598 SERVER_CONSOLE_PORT=19597 ./bin/api --config config.yml
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