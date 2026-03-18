# 监控和告警系统

## 概述

Prerender Shield 内置了完整的监控和告警系统，提供实时的系统健康检查、性能监控和异常告警功能。

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                     监控系统架构                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │  指标采集   │───▶│  规则引擎   │───▶│  告警处理   │     │
│  │  (Metrics)  │    │ (RuleEngine)│    │  (Handler)  │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│         │                  │                  │             │
│         ▼                  ▼                  ▼             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │ Prometheus  │    │  告警规则    │    │ Webhook/    │     │
│  │   Metrics   │    │   (JSON)    │    │   Email     │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│         │                  │                                │
│         ▼                  ▼                                │
│  ┌─────────────────────────────────┐                        │
│  │      Redis (数据持久化)          │                        │
│  └─────────────────────────────────┘                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 配置说明

### 1. 主配置文件 (configs/config.yml)

```yaml
monitoring:
  enabled: true
  prometheus_address: ":9090"  # Prometheus 指标暴露端口

  # 告警配置
  alerting:
    enabled: true
    rules_path: "configs/alert-rules.json"  # 告警规则文件路径
    notifications:
      webhook:
        enabled: false
        url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
        secret: ""
      email:
        enabled: false
        smtp_host: "smtp.example.com"
        smtp_port: 587
        username: "your-email@example.com"
        password: "your-password"
        from: "Prerender Shield <noreply@example.com>"
        to:
          - "admin@example.com"

  # 监控数据持久化配置
  metrics_persistence:
    enabled: true
    interval: 300           # 持久化间隔（秒），默认 5 分钟
    retention_hours: 24     # 数据保留时间（小时），默认 24 小时
    aggregate_enabled: true
    aggregate_interval: 3600  # 聚合间隔（秒），默认 1 小时
```

### 2. 告警规则配置 (configs/alert-rules.json)

```json
{
  "rules": [
    {
      "id": "cpu_high",
      "name": "CPU 使用率过高",
      "description": "当 CPU 使用率超过 90% 时触发告警",
      "enabled": true,
      "condition": {
        "metric": "cpuUsage",
        "operator": "gt",
        "threshold": 90,
        "duration": "1m"
      },
      "severity": "warning",
      "handlers": ["webhook", "email"],
      "cooldown": "5m"
    }
  ],
  "notifications": {
    "webhook": {
      "enabled": true,
      "url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
      "method": "POST",
      "timeout": "10s",
      "max_retries": 3,
      "retry_delay": "5s"
    },
    "email": {
      "enabled": true,
      "smtp_host": "smtp.example.com",
      "smtp_port": 587,
      "username": "your-email@example.com",
      "password": "your-password",
      "from": "Prerender Shield <noreply@example.com>",
      "to": ["admin@example.com"],
      "use_tls": true
    }
  }
}
```

## 预设告警规则

系统预置了 8 种告警规则：

| 规则 ID | 名称 | 指标 | 阈值 | 持续时间 | 严重程度 |
|--------|------|------|------|---------|---------|
| `cpu_high` | CPU 使用率过高 | cpuUsage | > 90% | 1m | warning |
| `memory_high` | 内存使用率过高 | memoryUsage | > 85% | 1m | warning |
| `disk_high` | 磁盘使用率过高 | diskUsage | > 90% | 5m | warning |
| `threat_spike` | 威胁检测激增 | blockedRequests | > 100/min | 30s | critical |
| `cache_hit_rate_low` | 缓存命中率过低 | cacheHitRate | < 50% | 5m | info |
| `high_request_volume` | 请求量激增 | totalRequests | > 10000/min | 1m | info |
| `browser_pool_exhausted` | 浏览器池耗尽 | activeBrowsers | > 10 | 2m | warning |
| `render_time_slow` | 渲染时间过长 | renderTime | > 5s | 3m | warning |

## 监控指标

### 系统指标

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `cpuUsage` | Gauge | CPU 使用率 (%) |
| `memoryUsage` | Gauge | 内存使用率 (%) |
| `memoryTotal` | Gauge | 总内存 (字节) |
| `memoryUsed` | Gauge | 已用内存 (字节) |
| `memoryFree` | Gauge | 空闲内存 (字节) |
| `diskUsage` | Gauge | 磁盘使用率 (%) |
| `diskTotal` | Gauge | 总磁盘空间 (字节) |
| `diskUsed` | Gauge | 已用磁盘空间 (字节) |
| `diskFree` | Gauge | 空闲磁盘空间 (字节) |

### 应用指标

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `totalRequests` | Counter | 总请求数 |
| `crawlerRequests` | Counter | 爬虫请求数 |
| `blockedRequests` | Counter | 被阻止的请求数 |
| `cacheHits` | Counter | 缓存命中数 |
| `cacheMisses` | Counter | 缓存未命中数 |
| `cacheHitRate` | Gauge | 缓存命中率 (%) |
| `activeBrowsers` | Gauge | 活跃浏览器数量 |
| `requestsPerSecond` | Gauge | 每秒请求数 |

### 网络指标

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `networkSent` | Counter | 发送字节数 |
| `networkRecv` | Counter | 接收字节数 |
| `networkPacketsSent` | Counter | 发送包数 |
| `networkPacketsRecv` | Counter | 接收包数 |

## Prometheus 集成

### 1. 访问指标端点

```bash
curl http://localhost:9090/metrics
```

### 2. Prometheus 配置示例

```yaml
scrape_configs:
  - job_name: 'prerender-shield'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

### 3. Grafana 仪表板

可以使用 Prometheus 作为数据源配置 Grafana 仪表板，可视化监控数据。

## 数据持久化

### 1. 实时数据

监控数据每 5 分钟（可配置）保存一次到 Redis，键格式：`prerender:metrics:<timestamp>`

### 2. 聚合数据

每小时自动聚合一次监控数据，计算平均值、最大值、最小值，键格式：`prerender:metrics:agg:<timestamp>`

### 3. 数据清理

系统每小时自动清理过期的监控数据，保留时间可配置（默认 24 小时）。

## API 接口

### 1. 健康检查

```bash
GET /api/health
```

响应示例：
```json
{
  "status": "healthy",
  "timestamp": "2026-03-18T12:00:00Z",
  "uptime": 3600,
  "version": "1.0.0",
  "checks": {
    "redis": {"status": "up", "latency_ms": 2},
    "memory": {"status": "up", "usage_percent": 45.2},
    "disk": {"status": "up", "usage_percent": 62.5}
  }
}
```

### 2. 获取监控数据

```bash
GET /api/metrics?start_time=1710748800&end_time=1710752400
```

### 3. 获取系统统计

```bash
GET /api/stats
```

## 告警处理流程

```
指标采集 ──▶ 规则评估 ──▶ 触发判断 ──▶ 冷却检查 ──▶ 通知发送
    │           │           │           │           │
    ▼           ▼           ▼           ▼           ▼
  Redis     RuleEngine  阈值比较    Cooldown   Webhook/Email
```

### 冷却机制

每个告警规则都有冷却时间（cooldown），在冷却期内相同的告警不会重复触发。

## 自定义告警规则

可以通过编辑 `configs/alert-rules.json` 添加自定义告警规则：

```json
{
  "id": "custom_rule",
  "name": "自定义规则",
  "description": "描述信息",
  "enabled": true,
  "condition": {
    "metric": "指标名称",
    "operator": "操作符 (gt/lt/gte/lte/eq)",
    "threshold": 阈值，
    "duration": "持续时间 (如 1m, 5m, 1h)"
  },
  "severity": "严重程度 (info/warning/critical)",
  "handlers": ["webhook", "email"],
  "cooldown": "冷却时间 (如 5m, 10m)"
}
```

## 故障排除

### 1. 监控数据未保存

- 检查 Redis 连接是否正常
- 检查配置文件中 `metrics_persistence.enabled` 是否为 `true`
- 查看日志中是否有 "Failed to save metrics to Redis" 错误

### 2. 告警未触发

- 检查告警规则是否启用 (`enabled: true`)
- 检查指标名称是否正确
- 确认冷却时间是否已过
- 查看日志确认规则引擎是否正常运行

### 3. Webhook 通知失败

- 检查 Webhook URL 是否正确
- 确认网络连通性
- 检查 Webhook 服务端是否正常运行
- 查看日志中的重试记录

## 最佳实践

1. **合理设置阈值**：根据实际业务情况调整告警阈值，避免误报和漏报
2. **分级告警**：使用不同的严重程度（info/warning/critical）区分告警优先级
3. **冷却时间**：设置合适的冷却时间，避免告警风暴
4. **多渠道通知**：重要告警配置多个通知渠道（Webhook + Email）
5. **定期审查**：定期审查告警规则的有效性，根据实际情况调整
6. **数据保留**：根据存储容量设置合理的数据保留时间
