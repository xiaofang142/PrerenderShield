# Prerender Shield 架构模块文档

> 基于代码实际结构的完整架构文档，最后更新: 2026-06-17 (二次审核修正)

---

## 一、系统分层架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                         客户端请求层                                  │
│              HTTP/HTTPS 请求 → 接入网关 (Ingress Gateway)              │
└────────────────────────────────┬─────────────────────────────────────┘
                                 │
┌────────────────────────────────▼─────────────────────────────────────┐
│                          接入层 (Port 9597/9598)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │ SSL/TLS 终止  │  │  请求解析     │  │ 初始流量过滤  │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
└────────────────────────────────┬─────────────────────────────────────┘
                                 │
┌────────────────────────────────▼─────────────────────────────────────┐
│                        核心处理层 (Gin Router)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │ 智能流量路由  │  │ 防火墙引擎    │  │ 渲染预热引擎  │               │
│  │ (routing/)   │  │ (firewall/)  │  │ (prerender/) │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
└────────────────────────────────┬─────────────────────────────────────┘
                                 │
┌────────────────────────────────▼─────────────────────────────────────┐
│                          服务层                                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │ 配置管理  │ │ 缓存服务  │ │ SSL管理   │ │ 监控告警  │ │ 日志审计  │  │
│  │ config/  │ │ cache/   │ │ ssl/     │ │monitoring│ │ logging/ │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
└────────────────────────────────┬─────────────────────────────────────┘
                                 │
┌────────────────────────────────▼─────────────────────────────────────┐
│                          数据层                                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐               │
│  │  Redis   │ │ 文件系统  │ │ Prometheus│ │  GeoIP DB │               │
│  │ (主存储)  │ │ (配置/证书)│ │ (指标)    │ │ (IP地理)  │               │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘               │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 二、模块清单 (36个内部包)

### 2.1 接入层模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `cmd/api/` | 应用入口，信号处理，启动编排 | `main()` |
| `internal/bootstrap/` | 应用启动器，依赖注入，服务器生命周期 | `AppRunner`, `Run()` |
| `internal/api/routes/` | Gin路由注册，中间件链组装 | `RegisterAllRoutes()`, `Controllers` |
| `internal/api/controllers/` | HTTP请求处理器 (11个Controller) | 见Controller清单 |
| `internal/api/middleware/` | API层中间件 | 错误处理 |

### 2.2 核心引擎模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/firewall/` | WAF防火墙引擎 | `Engine`, `RuleManager`, `ActionHandler` |
| `internal/firewall/detectors/` | 安全检测器集合 (25文件) | `InjectionDetector`, `XSSDetector`, `CSRFDetector` 等 |
| `internal/firewall/detectors/ai/` | AI智能检测 | 机器学习异常检测 |
| `internal/firewall/detectors/ddos/` | DDoS检测 | 流量模式分析 |
| `internal/firewall/types/` | 防火墙类型定义 | `Rule`, `Threat`, `CheckResult` |
| `internal/prerender/` | 预渲染引擎 | `Engine`, `EngineManager`, `RenderOptions` |
| `internal/prerender/pool/` | 浏览器实例池 | 动态扩缩容 |
| `internal/prerender/cache/` | 渲染缓存 | 多级缓存 |
| `internal/prerender/push/` | 搜索引擎推送 | 百度/必应URL推送 |
| `internal/prerender/preheat/` | 缓存预热器 | 热点图预热 (`cache/preheater.go`) |
| `internal/prerender/preheat.go` | 预热工作器 | Sitemap解析 + 批量预热 |
| `internal/prerender/streaming/` | 流式渲染 | 大页面流式输出 |
| `internal/prerender/incremental/` | 增量渲染 | 页面差异渲染 |
| `internal/prerender/optimizer/` | 渲染优化 | 资源加载优化 |
| `internal/prerender/smartwait/` | 智能等待（内联实现） | 页面加载完成检测 (`engine.go` 内联JS Promise) |
| `internal/prerender/seo_injector.go` | SEO注入 | 元标签/结构化数据注入 |
| `internal/routing/` | 智能流量路由 | 爬虫识别 → 渲染 / 普通请求 → 防火墙 |

### 2.3 安全认证模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/auth/` | 认证授权 | `UserManager`(单管理员), `JWTManager`, `User` |
| `internal/auth/` | 2FA双因素认证 | `TOTPManager` |

### 2.4 站点管理模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/site-handler/` | 站点HTTP处理器 | `Handler`, `CreateSiteHandler()` |
| `internal/site-server/` | 站点服务器管理 | `Manager`, `StartSiteServer()` |
| `internal/proxy/` | 反向代理 | 代理模式站点转发 |

### 2.5 中间件模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/middleware/` | 全局中间件 | `WafMiddleware()`, `RateLimitMiddleware()`, `GlobalErrorHandler()` |
| `internal/middleware/` | WAF日志写入器 | `WafLogWriter` |

### 2.6 数据与存储模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/redis/` | Redis客户端 (979行) | `Client`, `CircuitBreaker`, `Subscriber` |
| `internal/repository/` | 数据仓库层 | `WafRepository`, `SiteRepository` |
| `internal/cache/` | 缓存管理器 | `Manager`, `Stats` |
| `internal/models/` | 数据模型 | `Site`, `WafConfig`, `AccessLog`, `WAFRule` |
| `internal/config/` | 配置管理 (718行) | `Config`, `ConfigManager`, `SiteConfig` 等30+结构体 |
| `internal/constants/` | 常量定义 | Redis Key, 默认值 |

### 2.7 监控与日志模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/monitoring/` | 系统监控 | `Monitor`, `Metrics` |
| `internal/monitoring/alerting/` | 告警系统 | `Rule`, `EmailNotifier`, `WebhookNotifier` |
| `internal/monitoring/dashboard/` | 监控仪表盘 | `Handler` |
| `internal/monitoring/telemetry/` | 遥测导出 | Prometheus + OpenTelemetry |
| `internal/logging/` | 日志系统 | `DefaultLogger`, `StructuredLogger`, `CrawlerLogManager`, `VisitLogManager` |
| `internal/logging/analyzer/` | 日志分析 | 日志统计分析 |
| `internal/audit/` | 审计日志 | 结构化审计事件 |

### 2.8 服务与工具模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/ssl/` | SSL证书管理 (9文件) | `Manager`, `ACMEClient`, `AutoRenew` |
| `internal/services/` | 业务服务 | `GeoIPService`(370行), `DomainResolver`(125行), `LogProcessor`(155行) |
| `internal/scheduler/` | 定时任务调度 (315行) | `Scheduler`, cron表达式, 站点监控 |
| `internal/proxy/` | 反向代理 (235行) | `Proxy`接口, HTTP连接池(100连接), Redis后端持久化 |
| `internal/routing/` | 智能路由 (403行) | `Matcher`接口, `MemoryCache`, 正则规则匹配 |
| `internal/seo/` | SEO优化 (1336行) | `MetaTagsOptimizer`(580行), `StructuredDataOptimizer`(648行), `AEO`(108行) |
| `internal/i18n/` | 国际化 (128行) | `Translator`, 中/英/日/韩, 回退机制 |
| `internal/crypto/` | 加密工具 (220行) | `Encryptor`(AES-256-GCM), SHA256密钥派生 |
| `internal/di/` | 依赖注入 (214行) | `Container`(14个核心依赖), Wire |
| `internal/audit/` | 审计日志 (196行) | `Logger`, 16种操作类型, 3级严重度 |
| `internal/utils/` | 工具函数 | 通用工具 |
| `internal/utils/country/` | 国家代码映射 | CN→China等 |
| `internal/utils/redisutil/` | Redis工具 | Redis辅助函数 |
| `pkg/errors/` | 错误处理 | `AppError`, `Response`, 8种错误码 |

### 2.9 日志系统模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/logging/log.go` (309行) | 核心日志 | `Logger`(5级: DEBUG/INFO/WARN/ERROR/FATAL), `AuditLogEntry` |
| `internal/logging/structured_logger.go` | 结构化日志 | JSON格式日志输出 |
| `internal/logging/crawler_log.go` | 爬虫日志 | `CrawlerLogManager`, 日志清洗 |
| `internal/logging/visit_log.go` | 访问日志 | `VisitLogManager`, 日志清洗 |
| `internal/logging/analyzer/` | 日志分析 | 统计分析 |

### 2.10 监控系统模块

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/monitoring/monitor.go` | 系统监控 | `Monitor`, CPU/内存/磁盘/网络采集 |
| `internal/monitoring/metrics.go` | 指标管理 | 指标持久化+聚合 |
| `internal/monitoring/health_checker.go` | 健康检查 | `HealthChecker`, 服务/Redis/SSL状态 |
| `internal/monitoring/alerting/rules.go` | 告警规则 | 阈值检查引擎 |
| `internal/monitoring/alerting/channels.go` | 通知渠道 | Webhook/Email/Slack/钉钉 |
| `internal/monitoring/alerting/email.go` | 邮件通知 | SMTP发送 |
| `internal/monitoring/alerting/webhook.go` | Webhook通知 | HTTP POST |
| `internal/monitoring/dashboard/handler.go` | 仪表盘API | 监控数据接口 |
| `internal/monitoring/telemetry/metrics.go` | Prometheus导出 | 标准格式指标 |
| `internal/monitoring/telemetry/tracer.go` | OpenTelemetry | 分布式追踪 |
| `internal/monitoring/telemetry/exporter.go` | 遥测导出 | OTLP导出 |
| `internal/monitoring/telemetry/middleware.go` | 遥测中间件 | 自动埋点 |


### 2.12 预渲染子系统

| 包路径 | 职责 | 关键类型 |
|--------|------|---------|
| `internal/prerender/engine.go` (535行) | 渲染引擎 | `Engine`接口, `RenderOptions`, `RenderResult` |
| `internal/prerender/engine_manager.go` | 引擎管理器 | 多站点引擎管理 |
| `internal/prerender/concurrency_manager.go` | 并发管理 | 并发控制 |
| `internal/prerender/queue.go` | 渲染队列 | 优先级队列 |
| `internal/prerender/persistent_queue.go` | 持久化队列 | Redis持久化 |
| `internal/prerender/crawler.go` | 爬虫处理 | 爬虫请求处理 |
| `internal/prerender/preheat.go` | 预热逻辑 | Sitemap解析+批量预热 |
| `internal/prerender/seo_injector.go` | SEO注入器 | 渲染后SEO注入 |
| `internal/prerender/pool/` | 浏览器池 | 动态扩缩容 |
| `internal/prerender/cache/` | 渲染缓存 | 多级缓存 |
| `internal/prerender/push/` | 搜索引擎推送 | 百度/必应 |
| `internal/prerender/preheat/` | 预热子系统 | 热点图缓存预热 |
| `internal/prerender/streaming/` | 流式渲染 | 大页面流式 |
| `internal/prerender/incremental/` | 增量渲染 | 差异渲染 |
| `internal/prerender/optimizer/` | 渲染优化 | 资源优化 |
| `internal/prerender/seo_injector.go` | SEO注入器 | SEO功能注入 |

---

## 三、Controller清单 (11个)

| Controller | 文件 | 方法数 | 职责 |
|-----------|------|--------|------|
| `AuthController` | `auth_controller.go` | 7 | 登录/登出/2FA/密码修改 |
| `SystemController` | `system_controller.go` | 7 | 健康检查/版本/配置/备份恢复 |
| `SitesController` | `sites_controller.go` | 15 | 站点CRUD/配置/静态资源 |
| `FirewallController` | `firewall_controller.go` | 7 | WAF配置/日志/黑白名单 |
| `OverviewController` | `overview_controller.go` | 1 | 概览数据聚合 |
| `MonitoringController` | `monitoring_controller.go` | 2 | 监控统计/告警历史 |
| `PreheatController` | `preheat_controller.go` | 7 | 预热管理/缓存/爬虫头 |
| `PushController` | `push_controller.go` | 6 | 推送管理/统计/趋势 |
| `CrawlerController` | `crawler_controller.go` | 2 | 爬虫日志/统计 |
| `SSLController` | `ssl_controller.go` | 8 | 证书CRUD/续签/通配符 |
| `StaticController` | `static_controller.go` | 1 | 静态文件服务 |

---

## 四、中间件链

```
请求 → gin.Recovery()
     → GlobalErrorHandler()        // 全局错误处理
     → SecurityHeaders()           // 安全响应头
     → CORS()                      // 跨域处理
     → [公开路由: 直接处理]
     → [受保护路由]:
       → JWTAuthMiddleware()       // JWT令牌验证
       → [业务Handler]
```

### 站点级中间件链

```
站点请求 → WafMiddleware()          // WAF安全检查
         → RateLimitMiddleware()    // 频率限制
         → CrawlerDetector          // 爬虫识别
         → [爬虫] → PrerenderEngine // 渲染预热
         → [普通] → Proxy/Static    // 正常处理
```

---

## 五、依赖注入 (Wire)

```go
// internal/di/ 使用 google/wire 管理依赖
AppRunner {
    UserManager      *auth.UserManager
    JWTManager       *auth.JWTManager
    ConfigManager    *config.ConfigManager
    PrerenderManager *prerender.EngineManager
    RedisClient      *redis.Client
    Scheduler        *scheduler.Scheduler
    SiteServerMgr    *siteserver.Manager
    SiteHandler      *sitehandler.Handler
    Monitor          *monitoring.Monitor
    CrawlerLogMgr    *logging.CrawlerLogManager
    VisitLogMgr      *logging.VisitLogManager
    WafRepo          *repository.WafRepository
    AuditLogger      *audit.Logger
}
```

---

## 六、数据流架构

### 普通用户请求流
```
Client → :9597(Admin Console) → API Proxy → :9598(Gin API)
    或
Client → :8082(Site Port) → WAF Middleware → Proxy/Static → Response
```

### 爬虫请求流
```
Crawler → Site Port → CrawlerDetector → Cache Check
  → Cache Hit → Return Cached HTML
  → Cache Miss → PrerenderEngine → Chromium Render → Cache → Return HTML
```

### 管理API流
```
Browser → :9597 → SPA (React) → Axios → :9597/api/v1/* → API Proxy → :9598
```

---

## 七、技术栈总览

| 层级 | 技术 | 版本/说明 |
|------|------|----------|
| 语言 | Go | 1.25.0 |
| Web框架 | Gin | v1.9.0 |
| 渲染引擎 | chromedp | v0.14.2 |
| 缓存/存储 | Redis | v8.11.5 |
| 认证 | golang-jwt/jwt | v5.3.0 |
| 密码加密 | golang.org/x/crypto | bcrypt |
| 2FA | pquerna/otp | v1.5.0 (TOTP) |
| SSL/ACME | go-acme/lego | v4.32.0 |
| 监控 | Prometheus client | v1.20.4 |
| 遥测 | OpenTelemetry | v1.42.0 |
| 日志 | go.uber.org/zap | v1.27.0 |
| 定时任务 | robfig/cron | v3.0.1 |
| 系统监控 | shirou/gopsutil | v4.25.4 |
| GeoIP | oschwald/geoip2-golang | v1.9.0 |
| WebSocket | gorilla/websocket | v1.4.2 |
| DI | google/wire | v0.7.0 |
| 配置 | gopkg.in/yaml.v3 | YAML解析 |
| 前端 | React 18 + TypeScript | Vite构建 |
| UI | Ant Design 5.x | 组件库 |
| 图表 | ECharts 5.x | 可视化 |
