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

### 1. 主配置文件 (config/config.yml)

```yaml
monitoring:
  enabled: true
  prometheus_address: ":9090"  # Prometheus 指标暴露端口

  # 告警配置
  alerting:
    enabled: true
    rules_path: "configs/alert-rules.json"  # 告警规则文件路径（启动期一次性加载；规则日常管理建议走控制台/API）
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

## 内置告警规则

系统内置 4 条默认告警规则（`internal/monitoring/alerting/rules.go` DefaultRules）：

| 规则 ID | 名称 | 指标 | 阈值 | 持续时间 | 严重程度 |
|--------|------|------|------|---------|---------|
| `cpu_high` | CPU 使用率过高 | system_cpu_usage | > 90 | 1m | warning |
| `memory_high` | 内存使用率过高 | system_memory_usage | > 85 | 1m | warning |
| `threat_spike` | 威胁检测激增 | threats_per_minute | > 100 | 30s | critical |
| `render_queue_backlog` | 渲染队列积压 | render_queue_size | > 50 | 2m | warning |

> 规则表所列 `system_cpu_usage` 等为引擎层的别名指标名，引擎在查询时会解析为下方「监控指标」表中的真实键
> （如 `cpuUsage`、`memoryUsage`、`blockedRequests`、`renderQueueSize`），因此内置规则会正常触发、不会因键名失配而失效。
>
> 规则的日常增删改请通过控制台（`GET/POST /api/v1/monitoring/alert-rules`，规则存 Redis）完成，保存即生效。
> `configs/alert-rules.example.json` 为扩展规则模板（含 8 条示例）；`rules_path` 指向的文件在启动期一次性加载。
> 加载器同时兼容「包装对象 `{"rules": [...]}`」与「裸规则数组」，且 `duration`/`cooldown` 既可用 Go time.Duration
> 数值（纳秒）也可用人类可读字符串（如 `"1m"`、`"5m"`），模板可直接使用。

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

监控数据由后台定时任务按 `metrics_persistence.interval`（默认 5 分钟）保存一次到 Redis，
键格式：`prerender:metrics:<timestamp>`，写入时设置 24 小时 TTL 自然过期。
该循环受 `metrics_persistence.enabled` 开关控制（关闭则不再写盘）。

### 2. 聚合数据

聚合函数（按 `metrics_persistence.aggregate_interval`，默认 1 小时窗口的均值/最大/最小，
键格式 `prerender:metrics:agg:<timestamp>`，30 天 TTL）已接入定时调度；仅在
`metrics_persistence.aggregate_enabled` 为 true 时执行。

### 3. 数据清理

清理函数（按 `metrics_persistence.retention_hours`，默认 24 小时）已接入定时调度（每小时执行，
跳过 `:agg:` 聚合键，聚合数据保留 30 天）；实时数据本身仍有 24 小时 TTL 兜底。

## API 接口

### 1. 健康检查（公开）

```bash
GET /api/v1/health
```

响应示例：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "status": "running",
    "service": "prerender-shield",
    "redis_status": "connected",
    "chromium": "available",
    "ssl_status": "active",
    "expiring_certs": 0,
    "timestamp": 1756600000,
    "health_details": {
      "memory_allocated": 12345678,
      "memory_total_alloc": 23456789,
      "memory_sys": 34567890,
      "num_goroutines": 42,
      "gc_cycles": 10
    }
  }
}
```

### 2. 获取监控统计（JWT）

```bash
GET /api/v1/monitoring/stats
```

返回实时指标快照（totalRequests/cacheHitRate/cpuUsage/memoryUsage 等，见上文「监控指标」表）。

### 3. 获取告警历史（JWT）

```bash
GET /api/v1/monitoring/alerts/history
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

推荐通过控制台/API 管理自定义告警规则（存 Redis，保存即生效）。规则字段：

```json
{
  "id": "custom_rule",
  "name": "自定义规则",
  "description": "描述信息",
  "enabled": true,
  "condition": {
    "metric": "指标名称（见上文监控指标表）",
    "operator": "操作符 (gt/lt/ge/le/eq)",
    "threshold": 100,
    "duration": 60000000000
  },
  "severity": "严重程度 (info/warning/critical)",
  "handlers": ["webhook", "email"],
  "cooldown": 300000000000
}
```

> 文件加载（`monitoring.alerting.rules_path`）同时兼容**裸规则数组**与 **{"rules": [...], "notifications": {...}} 包装对象**；
> `duration`/`cooldown` 既接受 Go `time.Duration` 序列化值（整型纳秒，如 1m=60000000000），也接受人类可读字符串
> （"1m"/"5m"）。`configs/alert-rules.example.json` 模板可直接被加载器解析。

## 故障排除

### 1. 监控数据未保存

- 检查 Redis 连接是否正常；持久化循环仅在 `metrics_persistence.enabled: true` 且 Redis 客户端注入后启用
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
