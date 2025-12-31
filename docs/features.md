# PrerenderShield 功能文档

## 1. 功能概述

PrerenderShield 是一款集防火墙安全防护与渲染预热功能于一体的企业级 Web 应用中间件，主要功能包括：

### 1.1 核心功能

| 功能模块 | 主要特性 |
|----------|----------|
| 防火墙防护 | OWASP Top 10 安全防护、SQL 注入防护、XSS 防护、CSRF 防护等 |
| 渲染预热 | 智能爬虫检测、SPA 页面渲染、缓存预热、定时渲染等 |
| 站点管理 | 多站点支持、多种运行模式（proxy、static、redirect）、HTTPS 支持 |
| 监控与日志 | 实时监控、Prometheus 集成、结构化日志、审计日志等 |
| 配置管理 | 动态配置、热更新、配置验证、多环境支持等 |
| 管理界面 | 现代化 UI、可视化数据、一键操作等 |

### 1.2 支持的站点模式

- **proxy**：代理模式，将请求转发到后端服务
- **static**：静态资源模式，直接提供静态文件服务
- **redirect**：重定向模式，将请求重定向到其他 URL

## 2. 快速开始

### 2.1 安装部署

#### 2.1.1 Docker 部署

```bash
docker-compose up -d
```

访问管理界面：`http://localhost:9597`

#### 2.1.2 二进制部署

1. 下载最新版本的二进制文件
2. 创建配置文件 `configs/config.yml`
3. 启动服务：`./prerender-shield -config configs/config.yml`

### 2.2 首次使用

1. 访问管理界面 `http://localhost:9597`
2. 使用默认账号密码登录（admin/admin）
3. 修改默认密码
4. 添加站点配置
5. 配置防火墙规则
6. 启用渲染预热

## 3. 功能模块详细说明

### 3.1 防火墙模块

#### 3.1.1 防护规则

PrerenderShield 内置了多种防护规则，包括：

- **SQL 注入防护**：检测和拦截 SQL 注入攻击
- **XSS 防护**：检测和拦截跨站脚本攻击
- **CSRF 防护**：检测和拦截跨站请求伪造攻击
- **命令注入防护**：检测和拦截命令注入攻击
- **路径遍历防护**：检测和拦截路径遍历攻击
- **文件包含防护**：检测和拦截文件包含攻击
- **XXE 防护**：检测和拦截 XML 外部实体攻击
- **不安全的反序列化防护**：检测和拦截不安全的反序列化攻击

#### 3.1.2 规则管理

通过管理界面可以：
- 查看所有规则
- 启用/禁用规则
- 调整规则优先级
- 添加自定义规则
- 导入/导出规则

#### 3.1.3 攻击日志

系统会记录所有被拦截的攻击，包括：
- 攻击类型
- 攻击来源 IP
- 攻击时间
- 攻击详情
- 拦截动作

### 3.2 渲染预热模块

#### 3.2.1 爬虫检测

系统通过以下方式检测爬虫：
- User-Agent 匹配
- IP 地址匹配
- 行为模式分析

#### 3.2.2 渲染引擎

- 基于 Chromium 的无头浏览器
- 支持并行渲染
- 支持配置渲染超时
- 支持配置渲染等待条件（networkidle0、domcontentloaded 等）

#### 3.2.3 缓存策略

- 支持配置缓存 TTL
- 支持缓存预热
- 支持缓存刷新
- 支持缓存统计

#### 3.2.4 缓存预热

- 支持通过 Sitemap 自动发现 URL
- 支持手动添加 URL
- 支持定时预热
- 支持并发预热

### 3.3 站点管理模块

#### 3.3.1 站点配置

每个站点可以配置：
- 基本信息：名称、ID、域名、端口
- 运行模式：proxy、static、redirect
- 防火墙规则：站点级别的防火墙规则
- 渲染预热：站点级别的渲染预热配置
- SSL 配置：HTTPS 支持

#### 3.3.2 站点监控

- 实时请求数
- 响应时间
- 爬虫请求数
- 渲染成功率
- 缓存命中率

#### 3.3.3 站点操作

- 启动/停止站点
- 重启站点
- 导出站点配置
- 导入站点配置

### 3.4 监控与日志模块

#### 3.4.1 监控指标

系统提供了丰富的监控指标，包括：

| 指标类型 | 指标名称 |
|----------|----------|
| 请求指标 | 请求总数、响应时间、状态码分布 |
| 爬虫指标 | 爬虫请求数、渲染时间、缓存命中率 |
| 安全指标 | 攻击数、拦截数、攻击类型分布 |
| 系统指标 | CPU 使用率、内存使用率、磁盘使用率 |
| 渲染指标 | 渲染成功率、渲染超时数、渲染平均时间 |

#### 3.4.2 日志系统

系统支持多种日志类型：

- **应用日志**：记录系统运行状态
- **访问日志**：记录 HTTP 请求
- **安全日志**：记录安全事件
- **审计日志**：记录管理员操作

#### 3.4.3 告警机制

系统支持配置告警规则，当指标超过阈值时触发告警：

- 支持邮件告警
- 支持 Webhook 告警
- 支持 Slack 告警
- 支持自定义告警渠道

### 3.5 配置管理模块

#### 3.5.1 配置文件

配置文件采用 YAML 格式，主要包含：

- 服务器配置
- 目录配置
- 缓存配置
- 存储配置
- 监控配置
- 站点配置

#### 3.5.2 配置热更新

系统支持配置热更新，无需重启服务即可应用配置更改：

- 监控配置文件变化
- 自动加载新配置
- 验证配置合法性
- 通知相关模块

#### 3.5.3 多环境支持

支持通过环境变量覆盖配置文件：

- 支持不同环境的配置文件
- 支持环境变量前缀
- 支持配置文件优先级

## 4. API 参考

### 4.1 认证

所有 API 请求需要在请求头中包含认证信息：

```
Authorization: Bearer <token>
```

### 4.2 站点管理 API

#### 4.2.1 获取站点列表

```
GET /api/sites
```

**响应示例**：
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "list": [
      {
        "id": "test-site",
        "name": "Test Site",
        "domains": ["example.com"],
        "port": 8080,
        "mode": "static",
        "enabled": true
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 10
  }
}
```

#### 4.2.2 添加站点

```
POST /api/sites
Content-Type: application/json
```

**请求示例**：
```json
{
  "id": "new-site",
  "name": "New Site",
  "domains": ["new.example.com"],
  "port": 8081,
  "mode": "proxy",
  "proxy": {
    "target_url": "http://backend:8080"
  },
  "enabled": true
}
```

#### 4.2.3 更新站点

```
PUT /api/sites/:id
Content-Type: application/json
```

#### 4.2.4 删除站点

```
DELETE /api/sites/:id
```

### 4.3 防火墙 API

#### 4.3.1 获取规则列表

```
GET /api/firewall/rules
```

#### 4.3.2 添加规则

```
POST /api/firewall/rules
Content-Type: application/json
```

#### 4.3.3 更新规则

```
PUT /api/firewall/rules/:id
Content-Type: application/json
```

#### 4.3.4 删除规则

```
DELETE /api/firewall/rules/:id
```

### 4.4 渲染预热 API

#### 4.4.1 获取渲染预热统计

```
GET /api/preheat/stats
```

#### 4.4.2 触发渲染预热

```
POST /api/preheat/trigger
Content-Type: application/json
```

**请求示例**：
```json
{
  "siteName": "test-site"
}
```

#### 4.4.3 预热指定 URL

```
POST /api/preheat/url
Content-Type: application/json
```

**请求示例**：
```json
{
  "siteName": "test-site",
  "urls": ["http://example.com/page1", "http://example.com/page2"]
}
```

#### 4.4.4 获取 URL 列表

```
GET /api/preheat/urls?siteName=test-site&page=1&pageSize=20
```

### 4.5 监控 API

#### 4.5.1 获取监控指标

```
GET /api/monitoring/metrics
```

#### 4.5.2 获取日志

```
GET /api/monitoring/logs?type=access&page=1&pageSize=20
```

## 5. 配置指南

### 5.1 配置文件结构

```yaml
# 服务器配置
server:
  address: 0.0.0.0
  api_port: 9598
  console_port: 9597

# 目录配置
dirs:
  data_dir: ./data
  static_dir: ./static
  certs_dir: ./certs
  admin_static_dir: ./web/dist

# 缓存配置
cache:
  type: memory
  redis_url: localhost:6379
  memory_size: 1000

# 存储配置
storage:
  type: postgres
  postgres_url: postgres://prerender:prerender@localhost:5432/prerender?sslmode=disable

# 监控配置
monitoring:
  enabled: true
  prometheus_address: :9090

# 站点配置
sites:
  - id: default
    name: 默认站点
    domains: ["localhost"]
    port: 8080
    mode: static
    enabled: true
    prerender:
      enabled: true
      pool_size: 5
      timeout: 30
      cache_ttl: 3600
```

### 5.2 主要配置项说明

#### 5.2.1 服务器配置

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| `server.address` | string | 服务器监听地址 | "0.0.0.0" |
| `server.api_port` | int | API 服务端口 | 9598 |
| `server.console_port` | int | 管理控制台端口 | 9597 |

#### 5.2.2 缓存配置

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| `cache.type` | string | 缓存类型（memory、redis） | "memory" |
| `cache.redis_url` | string | Redis 连接 URL | "localhost:6379" |
| `cache.memory_size` | int | 内存缓存大小（条） | 1000 |

#### 5.2.3 监控配置

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| `monitoring.enabled` | bool | 是否启用监控 | true |
| `monitoring.prometheus_address` | string | Prometheus 监听地址 | ":9090" |

#### 5.2.4 站点配置

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| `sites[].id` | string | 站点 ID | 必填 |
| `sites[].name` | string | 站点名称 | 必填 |
| `sites[].domains` | []string | 站点域名列表 | 必填 |
| `sites[].port` | int | 站点端口 | 必填 |
| `sites[].mode` | string | 站点模式（proxy、static、redirect） | 必填 |
| `sites[].enabled` | bool | 是否启用站点 | true |
| `sites[].prerender.enabled` | bool | 是否启用渲染预热 | true |
| `sites[].prerender.pool_size` | int | 渲染引擎池大小 | 5 |
| `sites[].prerender.timeout` | int | 渲染超时时间（秒） | 30 |
| `sites[].prerender.cache_ttl` | int | 缓存过期时间（秒） | 3600 |

## 6. 常见问题

### 6.1 渲染失败

**问题**：爬虫请求返回 500 错误

**解决方案**：
1. 检查 Chromium 进程是否正常运行
2. 查看渲染日志：`logs/render.log`
3. 检查渲染超时设置是否合理
4. 检查目标页面是否能正常访问

### 6.2 防火墙误判

**问题**：正常请求被防火墙拦截

**解决方案**：
1. 查看安全日志，确认拦截原因
2. 调整相关规则的灵敏度
3. 添加白名单规则
4. 禁用误判的规则

### 6.3 性能问题

**问题**：系统响应变慢

**解决方案**：
1. 查看监控指标，确认瓶颈所在
2. 增加渲染引擎池大小
3. 调整缓存策略
4. 优化目标页面性能
5. 考虑水平扩展

### 6.4 配置不生效

**问题**：修改配置后没有生效

**解决方案**：
1. 检查配置文件格式是否正确
2. 检查配置项名称是否正确
3. 查看应用日志，确认是否有配置错误
4. 尝试重启服务

### 6.5 SSL 证书问题

**问题**：HTTPS 请求失败

**解决方案**：
1. 检查证书文件是否存在
2. 检查证书格式是否正确
3. 检查证书是否过期
4. 检查域名是否与证书匹配

## 7. 最佳实践

### 7.1 安全最佳实践

- 定期更新系统和依赖
- 启用 HTTPS
- 使用强密码
- 定期备份配置和数据
- 配置合理的防火墙规则
- 启用审计日志
- 定期检查安全日志

### 7.2 性能最佳实践

- 根据负载调整渲染引擎池大小
- 配置合理的缓存 TTL
- 启用缓存预热
- 优化目标页面性能
- 启用 Gzip 压缩
- 使用 CDN 加速静态资源

### 7.3 部署最佳实践

- 使用容器化部署
- 配置自动重启
- 启用健康检查
- 配置日志轮转
- 监控系统资源使用情况
- 配置告警规则

## 8. 故障排查

### 8.1 日志查看

```bash
# 查看应用日志
tail -f logs/app.log

# 查看访问日志
tail -f logs/access.log

# 查看错误日志
tail -f logs/error.log

# 查看渲染日志
tail -f logs/render.log
```

### 8.2 监控指标

访问 Prometheus 监控页面：`http://localhost:9090`

查看主要指标：

```
# 请求总数
prerender_requests_total

# 响应时间
prerender_response_time_seconds

# 爬虫请求数
prerender_crawler_requests_total

# 缓存命中率
prerender_cache_hits_total / (prerender_cache_hits_total + prerender_cache_misses_total)

# 防火墙拦截数
prerender_blocked_requests_total
```

### 8.3 常见错误代码

| 错误代码 | 说明 | 解决方案 |
|----------|------|----------|
| 400 | 无效的请求参数 | 检查请求参数是否正确 |
| 401 | 未授权 | 检查认证信息是否正确 |
| 403 | 禁止访问 | 检查权限配置 |
| 404 | 资源不存在 | 检查资源路径是否正确 |
| 500 | 服务器内部错误 | 查看应用日志，排查具体原因 |
| 502 | 网关错误 | 检查后端服务是否正常 |
| 503 | 服务不可用 | 检查系统资源使用情况 |
| 504 | 网关超时 | 检查后端服务响应时间 |

## 9. 升级指南

### 9.1 备份数据

在升级前，建议备份：
- 配置文件
- 数据库数据
- 证书文件
- 日志文件

### 9.2 Docker 升级

```bash
docker-compose pull
docker-compose up -d
```

### 9.3 二进制升级

1. 下载最新版本的二进制文件
2. 停止当前服务
3. 替换二进制文件
4. 启动服务

### 9.4 数据库迁移

如果版本升级涉及数据库结构变更，系统会自动执行迁移脚本。建议在升级前备份数据库。

## 10. 联系方式

- 项目地址：https://github.com/your-org/prerendershield
- 文档地址：https://prerendershield.io/docs
- 问题反馈：https://github.com/your-org/prerendershield/issues
- 邮件：contact@prerendershield.io
- 社区：https://discord.gg/prerendershield

## 11. 许可证

MIT License
