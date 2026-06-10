# 数据流 — Data Flow

> 基于代码实际数据结构与流转路径整理。
> 更新日期: 2026-06-09

---

## 一、主请求数据流

### 1.1 请求处理管道

```
Request ──→ [Site Server] ──→ [Middleware Chain] ──→ [WAF Engine] ──→ [Router]
                │                                                      │
                │                                                      ├── Blocked → 403 Response
                │                                                      │              ↓
                │                                                      │         [Attack Log]
                │                                                      │
                │                                                      ├── Crawler → [Prerender Engine]
                │                                                      │                │
                │                                                      │         ┌──────┴──────┐
                │                                                      │         │             │
                │                                                      │    [Cache Hit]  [Cache Miss]
                │                                                      │         │             │
                │                                                      │         │        [Chromium Render]
                │                                                      │         │             │
                │                                                      │         └──────┬──────┘
                │                                                      │                │
                │                                                      │         [Cached HTML]
                │                                                      │                │
                │                                                      ├── Normal → [Proxy] → Origin Server
                │                                                      │                │
                │                                                      │         [Origin Response]
                │                                                      │
                │                                              Response ←──────────────┘
                │                                                      │
                │                                              [Access Log] ←─────────┘
```

### 1.2 数据格式转换

```
请求进入: HTTP Request (Raw Bytes)
    │
    ▼
Gin Context 解析: c.Request (Parsed Headers + Body)
    │
    ▼
WAF 引擎: 提取特征→规则匹配→评分 (结构化CheckResult)
    │
    ▼
爬虫检测: CrawlerResult{IsCrawler, Confidence, Type}
    │
    ├── Yes→ PrerenderRequest{URL, SiteID, UserAgent, Priority}
    │          │
    │          ├── Cache.Get(key) → CacheEntry{Value, Metadata, HitCount}
    │          │
    │          └── Chromium Renderer → HTML (string)
    │                   │
    │                   └── Cache.Set(key, CacheEntry) 
    │
    └── No → Proxy Forward → Origin Response (Raw HTTP)
              │
              └── Response Relay → Client
```

---

## 二、缓存数据流

### 2.1 Redis 缓存架构

```
                     ┌─────────────────────┐
                     │    请求渲染结果       │
                     └──────────┬──────────┘
                                │
                    ┌───────────▼───────────┐
                    │    Redis (go-redis)    │
                    │  cache:{site}:{key}    │
                    │  TTL: 3600s (可配置)   │
                    │  + meta hash 存储元数据 │
                    └───────────┬───────────┘
                                │
                     ┌─────────▼─────────┐
                     │  缓存策略           │
                     │  Set → Redis SET   │
                     │  Get → Redis GET   │
                     │  Del → Redis DEL   │
                     │  Expire → TTL      │
                     └───────────────────┘
```

### 2.2 缓存键设计

```
Redis Key:  cache:{site_id}:{key}
Meta Key:   cache:{site_id}:{key}:meta

示例:
  data: cache:site-1:homepage
  meta: cache:site-1:homepage:meta (Hash: created_at, expires_at, priority, hit_count)
```

### 2.3 缓存策略

```
写入: SET cache:{site}:{key} {value} EX {ttl}
      HSET cache:{site}:{key}:meta created_at={ts} priority={p}

读取: GET cache:{site}:{key} → 命中返回, 未命中渲染
      HSET cache:{site}:{key}:meta hit_count=+1

删除: DEL cache:{site}:{key} cache:{site}:{key}:meta

淘汰: TTL自然过期 / 手动清除API / EvictLowPriority扫meta删低优先级
```
```

---

## 三、配置数据流

### 3.1 配置来源与优先级

```
最高优先级: Redis (动态配置)
    ↑
中间优先级: 配置文件 (config.yml)
    ↑
最低优先级: 代码默认值
```

### 3.2 配置同步路径

```
管理员操作
    │
    ├──→ 管理界面/API 修改
    │       │
    │       ├──→ Redis SET key config:{site_id}
    │       │        │
    │       │        └──→ Redis PUBLISH config:update
    │       │                 │
    │       │                 └──→ 所有服务订阅收到通知
    │       │                          │
    │       │                          └──→ 热加载新配置
    │       │
    │       └──→ 文件修改 (YAML)
    │                │
    │                └──→ fsnotify 检测到变更
    │                         │
    │                         └──→ 重新加载配置文件
    │
    └──→ 启动时加载
            │
            └──→ config.yml → Config 结构体 → 注入各模块
```

---

## 四、日志数据流

### 4.1 日志类型与流向

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  爬虫日志     │    │  访问日志     │    │  攻击日志     │
│  CrawlerLog  │    │  VisitLog   │    │  AttackLog   │
├──────────────┤    ├──────────────┤    ├──────────────┤
│ - 爬虫类型   │    │ - 请求路径   │    │ - 攻击类型   │
│ - UA        │    │ - 状态码    │    │ - 规则ID     │
│ - 渲染时间   │    │ - 响应时间   │    │ - 源IP      │
│ - 缓存命中   │    │ - 客户端IP  │    │ - 威胁等级   │
│ - 目标URL   │    │ - 请求方法   │    │ - 拦截动作   │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  LogProcessor │
                    │  (异步)       │
                    │  + GeoIP     │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   Redis List  │
                    │   (日志队列)   │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   管理API     │
                    │   查询展示    │
                    └──────────────┘
```

### 4.2 日志流增强 (GeoIP)

```
原始日志: {IP: "8.8.8.8", Path: "/", Status: 200}
    │
    ▼
GeoIP Service:
    ├── IP → geoip2.Lookup
    ├── Country: "US"
    ├── City: "Mountain View"
    ├── ISP: "Google LLC"
    └── Lat/Lon: 37.386, -122.083
    │
    ▼
增强日志: {IP: "8.8.8.8", Path: "/", Status: 200,
           Country: "US", City: "Mountain View", ISP: "Google LLC"}
```

---

## 五、SSL证书数据流

### 5.1 证书申请流程

```
触发条件:
├── 手动 (管理员点击"申请证书")
├── 自动 (定时器检查, expires_in ≤ 30天)
└── 首次配置 (站点新增SSL域名)
    │
    ▼
ACME Client:
    1. 生成账户密钥 (ECDSA P256)
    2. 注册Let's Encrypt账户
    3. 生成域名私钥 (RSA 2048)
    4. 创建CSR
    5. 选择挑战方式:
       ├── HTTP-01: 端口80响应 /.well-known/acme-challenge/{token}
       └── DNS-01: DNS TXT记录 _acme-challenge.{domain}
    6. 提交订单→等待验证→下载证书
    │
    ▼
证书存储:
├── {domain}.crt (证书文件)
├── {domain}.key (私钥文件)
├── {domain}.issuer.crt (中间证书)
└── Redis: ssl:cert:{domain} (证书元数据)
    │
    ▼
后续:
├── 自动续签: 每24小时检查
├── 过期通知: Webhook/管理界面
└── 证书删除: API操作
```

---

## 六、监控指标数据流

### 6.1 指标采集路径

```
应用内部事件
    │
    ├── HTTP请求完成 → RecordHTTPRequest() → Prometheus Counter/Histogram
    ├── WAF拦截事件 → RecordWAFBlock() → Prometheus Counter
    ├── 缓存命中/未命中 → RecordCacheHit/Miss() → Prometheus Counter
    ├── 渲染完成 → RecordRenderDuration() → Prometheus Histogram
    ├── DDoS检测 → RecordDDoSDetection() → Prometheus Counter
    └── SSL检查 → SetSSLCertExpiryDays() → Prometheus Gauge
    │
    ▼
Prometheus Registry (:9090/metrics)
    │
    ├── Prometheus Server 抓取
    ├── 管理界面: /api/v1/monitoring/stats
    └── OTel Exporter → OTLP Collector (可选)
```

### 6.2 健康检查数据结构

```json
{
  "status": "ok",
  "checks": {
    "redis": {"healthy": true, "message": "Redis is healthy"},
    "system": {"healthy": true, "message": "System is healthy"},
    "memory": {"healthy": true, "message": "Memory usage: 128 MB"},
    "ssl": {"healthy": true, "message": "SSL certificates are healthy"}
  },
  "memory": {
    "allocated": 134217728,
    "total_alloc": 536870912,
    "sys": 268435456,
    "num_gc": 42
  },
  "goroutines": 15,
  "timestamp": 1717929600,
  "uptime": 86400.0
}
```

---

## 七、数据存储清单

| 数据类型 | 存储位置 | Key 模式 | TTL | 说明 |
|---------|---------|---------|-----|------|
| 渲染缓存 | Redis | `cache:{site}:{key}` | 3600s | go-redis SET/GET |
| 站点配置 | Redis + YAML | `config:{site_id}` | ∞ | 双源同步 |
| WAF规则 | YAML | — | ∞ | 文件加载 |
| JWT Token | Redis | `jwt:{token}` | 24h | 支持吊销 |
| 用户信息 | Redis | `user:{id}` | ∞ | 本地缓存+Redis |
| 攻击日志 | Redis | `log:attack:{ts}` | 7d | List结构 |
| 爬虫日志 | Redis | `log:crawler:{ts}` | 7d | List结构 |
| 访问日志 | Redis | `log:visit:{ts}` | 7d | List结构 |
| SSL证书 | 文件+Redis | `ssl:cert:{domain}` | ∞ | PEM文件+元数据 |
| SSL续签记录 | Redis | `ssl:renewal:{domain}` | 7d | 续签历史 |
| ACME挑战 | Redis | `acme:challenge:{token}` | 1h | 挑战值暂存 |
| SSL证书集合 | Redis | `ssl:certs` | ∞ | Set存储域名列表 |
| 会话 | Redis | `session:{id}` | 24h | 用户会话 |
| Prometheus指标 | 进程内 | — | — | `/metrics`端点采集 |
