# PrerenderShield API 文档

## 概述

PrerenderShield 是一款集防火墙安全防护与渲染预热功能于一体的Web应用中间件，提供RESTful API接口用于管理和监控服务。

## 认证

大多数API端点需要认证，使用JWT令牌进行身份验证。

```
Authorization: Bearer <jwt-token>
```

获取令牌：
```
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "123456"
}
```

## API 端点

### 系统管理

#### 健康检查
```
GET /api/v1/health
```
检查服务健康状态。

响应示例：
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "status": "running",
    "service": "prerender-shield",
    "redis_status": "connected",
    "timestamp": 1234567890,
    "health_details": {
      "memory_allocated": 1234567,
      "memory_total_alloc": 7654321,
      "memory_sys": 9876543,
      "num_goroutines": 10,
      "gc_cycles": 5
    }
  }
}
```

#### 获取系统版本
```
GET /api/v1/version
```
获取系统版本信息。

#### 获取/更新系统配置
```
GET /api/v1/system/config
```
获取系统配置。

```
POST /api/v1/system/config
Content-Type: application/json

{
  "access_log_retention_days": "7",
  "crawler_log_retention_days": "7"
}
```
更新系统配置。

### 站点管理

#### 获取站点列表
```
GET /api/v1/sites
```
获取所有站点配置。

#### 获取单个站点
```
GET /api/v1/sites/{id}
```
获取指定站点配置。

#### 添加站点
```
POST /api/v1/sites
Content-Type: application/json

{
  "id": "new-site",
  "name": "New Site",
  "domains": ["example.com"],
  "port": 8080,
  "mode": "static",
  "firewall": {...},
  "prerender": {...}
}
```
添加新站点。

#### 更新站点
```
PUT /api/v1/sites/{id}
Content-Type: application/json
```
更新站点配置。

#### 删除站点
```
DELETE /api/v1/sites/{id}
```
删除指定站点。

### 防火墙管理

#### 获取防火墙配置
```
GET /api/v1/sites/{id}/waf
```
获取指定站点的防火墙配置。

#### 更新防火墙配置
```
PUT /api/v1/sites/{id}/waf
Content-Type: application/json

{
  "enabled": true,
  "rate_limit_count": 100,
  "rate_limit_window": 60
}
```
更新站点防火墙配置。

#### 添加IP到黑名单
```
POST /api/v1/firewall/blacklist
Content-Type: application/json

{
  "site_id": "site-id",
  "ip": "192.168.1.100"
}
```
将IP添加到黑名单。

#### 添加IP到白名单
```
POST /api/v1/firewall/whitelist
Content-Type: application/json

{
  "site_id": "site-id",
  "ip": "192.168.1.100"
}
```
将IP添加到白名单。

#### 获取访问日志
```
GET /api/v1/logs?page=1&limit=20&site_id=site-id
```
获取访问日志。

#### 获取攻击日志
```
GET /api/v1/firewall/attacks?page=1&limit=20&site_id=site-id
```
获取攻击日志。

### 渲染预热管理

#### 获取预热统计数据
```
GET /api/v1/preheat/stats
```
获取预热统计信息。

#### 触发预热
```
POST /api/v1/preheat/trigger
Content-Type: application/json

{
  "siteName": "site-id"
}
```
触发站点预热。

#### 获取预热URL列表
```
GET /api/v1/preheat/urls?site=site-id&page=1&size=20
```
获取预热URL列表。

#### 获取爬虫头列表
```
GET /api/v1/preheat/crawler-headers
```
获取爬虫头列表。

#### 清除缓存
```
POST /api/v1/preheat/clear-cache
Content-Type: application/json

{
  "siteName": "site-id"
}
```
清除站点缓存。

### 爬虫日志管理

#### 获取爬虫日志
```
GET /api/v1/crawler/logs?page=1&limit=20
```
获取爬虫访问日志。

#### 获取爬虫统计
```
GET /api/v1/crawler/stats
```
获取爬虫统计信息。

### 监控管理

#### 获取监控统计
```
GET /api/v1/monitoring/stats
```
获取系统监控统计信息。

响应示例：
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "activeBrowsers": 2,
    "blockedRequests": 5,
    "cacheHitRate": 0.85,
    "crawlerRequests": 10,
    "cpuUsage": 15.5,
    "memoryUsage": 42.3,
    "requestsPerSecond": 2.5,
    "totalRequests": 150
  }
}
```

## 错误码

- `200`: 成功
- `400`: 请求参数错误
- `401`: 未授权
- `403`: 禁止访问
- `404`: 资源不存在
- `500`: 服务器内部错误

## 故障排查

### 常见问题

1. **API请求返回401错误**
   - 确认已获取有效的JWT令牌
   - 检查令牌是否过期

2. **站点服务无法访问**
   - 检查站点配置是否正确
   - 确认端口未被其他服务占用

3. **渲染预热失败**
   - 确认Chromium/Chrome已正确安装
   - 检查目标URL是否可访问

### 日志查看

服务日志位于 `data/prerender-shield.log` 文件中。

## 性能调优

### 渲染引擎
- 根据流量调整 `pool_size` 参数
- 设置合适的 `timeout` 和 `cache_ttl`

### 防火墙
- 合理配置速率限制参数
- 定期清理IP黑名单/白名单

### 系统资源
- 监控内存使用情况
- 定期清理日志文件