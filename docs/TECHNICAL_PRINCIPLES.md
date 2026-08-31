# Prerender Shield 技术原理与实现文档

> 基于全部 36 个模块源码深度分析的完整技术文档。
> 每个模块包含：架构设计、数据结构、核心算法、设计决策论证。
> 更新日期: 2026-06-13

---

## 目录

1. [系统架构总览](#一系统架构总览)
2. [请求处理全链路](#二请求处理全链路)
3. [WAF 防火墙引擎](#三waf-防火墙引擎)
4. [预渲染引擎](#四预渲染引擎)
5. [SEO 优化引擎](#五seo-优化引擎)
6. [认证与安全](#六认证与安全)
7. [SSL/TLS 证书管理](#七ssltls-证书管理)
8. [监控与告警](#八监控与告警)
9. [日志与审计](#九日志与审计)
10. [数据存储与缓存](#十数据存储与缓存)
11. [配置管理](#十一配置管理)
12. [任务调度与队列](#十二任务调度与队列)
13. [智能路由与代理](#十三智能路由与代理)
14. [部署架构](#十四部署架构)

---

## 一、系统架构总览

### 1.1 设计哲学

Prerender Shield 的核心设计理念是 **"一个二进制解决两个问题"**：WAF 安全防护 + SPA 预渲染 SEO 优化。传统方案需要组合 Cloudflare/Nginx + Prerender.io + Redis 三个独立服务，而本系统将三者深度融合，通过**智能流量路由**自动分流——爬虫请求走渲染通道，普通请求走安全检测通道。

**关键设计决策**：
- **Go 单二进制**：编译为单一可执行文件，无运行时依赖（Chromium 除外），部署成本为零
- **Redis 作为唯一存储**：用户、配置、缓存、日志、会话全部存储在 Redis，无 SQL 数据库依赖
- **Gin 框架**：高性能 HTTP 框架，中间件链模型天然适合 WAF 检测流水线
- **chromedp**：纯 Go 实现的 Chrome DevTools Protocol 客户端，无需 Node.js/Puppeteer

### 1.2 架构分层

```
┌─────────────────────────────────────────────────────────┐
│                    客户端请求层                           │
│          HTTP/HTTPS → 管理控制台(:9597) / 站点端口        │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                     接入层                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │SSL/TLS终止│  │ 请求解析  │  │CORS/安全头│              │
│  │(ssl/)    │  │(Gin)     │  │(middleware)│             │
│  └──────────┘  └──────────┘  └──────────┘              │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                   核心处理层                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │智能路由   │  │WAF引擎   │  │渲染引擎   │              │
│  │(routing/)│  │(firewall/)│  │(prerender/)│            │
│  └──────────┘  └──────────┘  └──────────┘              │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                     服务层                               │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐         │
│  │配置   │ │缓存   │ │SSL   │ │监控   │ │日志   │         │
│  │config │ │cache │ │ssl   │ │monitor│ │logging│         │
│  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘         │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                     数据层                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │  Redis   │  │ 文件系统  │  │Prometheus │              │
│  │(主存储)   │  │(配置/证书)│  │(指标导出) │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
```

### 1.3 核心依赖

| 依赖 | 版本 | 用途 | 选型理由 |
|------|------|------|---------|
| `github.com/gin-gonic/gin` | v1.9.0 | HTTP 框架 | 高性能、中间件链模型、社区活跃 |
| `github.com/chromedp/chromedp` | v0.14.2 | Headless Chromium | 纯 Go 实现，无需 Node.js，直接控制 CDP |
| `github.com/go-redis/redis/v8` | v8.11.5 | Redis 客户端 | 连接池、Pipeline、Pub/Sub 支持完善 |
| `github.com/golang-jwt/jwt/v5` | v5.3.0 | JWT 令牌 | 标准库风格 API，安全审计通过 |
| `github.com/pquerna/otp` | v1.5.0 | TOTP 2FA | RFC 6238 标准实现 |
| `github.com/go-acme/lego/v4` | v4.32.0 | ACME 协议 | 支持 HTTP-01/DNS-01 挑战，多 DNS 提供商 |
| `github.com/prometheus/client_golang` | v1.20.4 | Prometheus 指标 | 行业标准，Grafana 直接对接 |
| `github.com/shirou/gopsutil/v4` | v4.25.4 | 系统资源 | 跨平台 CPU/内存/磁盘/网络采集 |
| `github.com/robfig/cron/v3` | v3.0.1 | Cron 调度 | 秒级精度，支持标准 Cron 表达式 |
| `go.opentelemetry.io/otel` | v1.42.0 | 分布式追踪 | CNCF 标准，OTLP 导出 |
| `github.com/google/wire` | v0.7.0 | 依赖注入 | 编译时 DI，零运行时开销 |
| `golang.org/x/crypto` | v0.48.0 | 密码加密 | bcrypt + scrypt + argon2 全支持 |

---

## 二、请求处理全链路

### 2.1 双端口架构

系统启动两个 HTTP 服务器：

| 端口 | 服务 | 处理器 | 认证 |
|------|------|--------|------|
| 9597 | 管理控制台 | 自定义 `http.ServeMux` + 反向代理 | 无（SPA 自行处理） |
| 9598 | API 服务 | Gin Router | JWT Bearer Token |

**设计决策**：为什么用两个端口而不是一个？

1. **安全隔离**：管理 API 和管理界面可以独立配置防火墙规则
2. **性能分离**：静态文件服务（9597）和 API 服务（9598）可以独立扩缩
3. **部署灵活**：可以将 9597 放在公网、9598 仅内网访问

### 2.2 管理控制台请求流 (Port 9597)

```
Browser → :9597
    │
    ├── /api/* 请求
    │   └── httputil.ReverseProxy → :9598 (API Server)
    │       设计理由: SPA 开发时 Vite 代理 /api → 后端,
    │       生产环境由 Go 反向代理实现相同效果, 无需 Nginx
    │
    └── 其他请求
        ├── 静态文件 (.js/.css/.svg/...) → 文件系统直接返回
        └── SPA 路由 (/login, /sites, /firewall...) → 返回 index.html
            设计理由: React Router 使用 History API,
            所有非静态文件路径必须返回 index.html 让前端路由接管
```

**静态文件路径解析**（`bootstrap/runner.go:168-196`）：

```
优先级顺序:
  1. {appDir}/web/dist/     → /opt/prerender-shield/web/dist/
  2. {appDir}/web/          → /opt/prerender-shield/web/
  3. {appDir}/bin/web/dist/ → /opt/prerender-shield/bin/web/dist/
  4. {appDir}/bin/web/      → /opt/prerender-shield/bin/web/
  5. ./web/dist/            → 当前目录
  6. ./web/                 → 当前目录
  7. ./bin/web/dist/        → 当前目录
  8. ./bin/web/             → 当前目录

设计理由: 兼容多种部署场景——源码运行、二进制运行、Docker 运行
```

### 2.3 API 请求流 (Port 9598)

```
Request → Gin Engine
    │
    ▼
Middleware Chain (按顺序):
┌──────────────────────────────────────────────────────┐
│ 1. gin.Recovery()                                    │
│    捕获所有 panic，返回 500 而非崩溃进程              │
│    设计理由: Go HTTP Server 默认每个请求一个 goroutine,│
│    单个请求 panic 会导致整个进程退出, Recovery 防止此问题│
├──────────────────────────────────────────────────────┤
│ 2. middleware.GlobalErrorHandler()                   │
│    自定义错误处理，统一 JSON 响应格式                  │
│    {"code": 500, "message": "Internal Server Error"}  │
├──────────────────────────────────────────────────────┤
│ 3. SecurityHeaders()                                 │
│    X-Frame-Options: DENY                             │
│    X-Content-Type-Options: nosniff                   │
│    X-XSS-Protection: 1; mode=block                   │
│    Referrer-Policy: strict-origin-when-cross-origin  │
├──────────────────────────────────────────────────────┤
│ 4. CORS()                                            │
│    Access-Control-Allow-Origin: echo origin           │
│    Access-Control-Allow-Methods: GET,POST,PUT,DELETE │
│    Access-Control-Allow-Headers: Authorization,...   │
│    Access-Control-Allow-Credentials: true             │
├──────────────────────────────────────────────────────┤
│ 5. [路由分发]                                         │
│    ├── 公开路由: /health, /version, /auth/login...   │
│    └── 受保护路由: JWTAuthMiddleware                 │
└──────────────────────────────────────────────────────┘
```

### 2.4 站点请求流 (动态端口)

```
Client → Site Port (e.g., :8082)
    │
    ▼
WafMiddleware (middleware/waf.go, 203行)
    │
    ├── 0. 防火墙未启用 → 直接放行
    │
    ├── 1. IP 黑名单检查
    │   └── Redis SISMEMBER firewall:{site}:blacklist {ip}
    │       → 命中 → 403 + 记录 AccessLog
    │
    ├── 2. IP 白名单检查
    │   └── Redis SISMEMBER firewall:{site}:whitelist {ip}
    │       → 命中 → 跳过所有后续检测
    │
    ├── 3. GeoIP 地理位置检查
    │   └── GeoIPService.LookupCountryISO(ip)
    │       → 匹配 BlockList → 403
    │       → 匹配 AllowList 但不在其中 → 403
    │
    ├── 4. 频率限制检查
    │   └── Redis INCR rate:{site}:{ip}:{window}
    │       → 超过 Requests 阈值 → 429 + BanTime 封禁
    │
    ├── 5. OWASP Top 10 威胁检测 (并行 goroutine)
    │   └── DetectorManager.Detect(req) → CheckResult
    │
    ├── 6. 网页防篡改检查
    │   └── SHA256 文件哈希对比
    │
    └── 7. AI 智能检测 (可选)
        └── 机器学习模型推理
    │
    ▼
CrawlerDetector (crawler/detector.go)
    │
    ├── 检查 IP 白名单 → 放行
    ├── 检查 Crawler IP 列表 → 标记为爬虫
    ├── 检查 User-Agent 匹配 → 标记为爬虫
    │
    ├── 爬虫请求 → PrerenderEngine.RenderWithContext()
    └── 普通请求 → 按站点模式处理:
        ├── proxy    → httputil.ReverseProxy 转发到后端
        ├── static   → http.FileServer 提供静态文件
        └── redirect → http.Redirect 重定向
```

---

## 三、WAF 防火墙引擎

### 3.1 引擎架构 (firewall/engine.go, 912行)

```
Engine
├── DetectorManager (detector_manager.go, 195行)
│   ├── OWASP Detectors (并行检测)
│   │   ├── InjectionDetector       (injection.go, 93行)
│   │   ├── XSSDetector             (xss.go, 89行)
│   │   ├── CSRFDetector            (csrf.go, 119行)
│   │   ├── DeserializationDetector (deserialization.go, 88行)
│   │   ├── XXEDetector             (xxe.go, 87行)
│   │   ├── SensitiveDataDetector   (sensitive_data.go, 89行)
│   │   └── AIDetector (可选)       (ai/)
│   │
│   └── Core Detectors (顺序检测)
│       ├── BlacklistDetector       (blacklist.go)
│       ├── GeoIPDetector           (geoip.go)
│       ├── RateLimitDetector       (rate_limit.go)
│       ├── UserAgentDetector       (user_agent.go)
│       └── FileIntegrityDetector   (file_integrity.go)
│
├── RuleManager
│   ├── 规则来源优先级: 远程源 → Redis → 本地文件 → 默认规则
│   ├── 自动更新: 定时从远程源拉取 (updateInterval)
│   └── 热加载: ReloadRules() 无需重启
│
└── ActionHandler (action.go, 78行)
    ├── 自定义拦截页面: staticDir/{siteName}/waf_block.html
    └── 默认拦截页面: HTML 模板 + BlockMessage
```

### 3.2 并行检测架构 (DetectorManager)

**核心设计**：所有检测器通过 goroutine 并行执行，通过 channel 收集结果。

```go
func (dm *DetectorManager) Detect(req *http.Request) (*CheckResult, error) {
    threatsChan := make(chan []types.Threat, detectorCount)
    errChan := make(chan error, detectorCount)

    // 并行启动所有检测器
    for name, detector := range owaspDetectors {
        go func(det OWASPDetector, detectorName string) {
            threats, err := det.Detect(req)
            if err != nil { errChan <- err; return }
            threatsChan <- threats
        }(detector, name)
    }
    // ... coreDetectors 同理

    // 收集结果
    for threats := range threatsChan {
        result.Threats = append(result.Threats, threats...)
    }

    // FailClosed 策略: 任何检测器错误 → 拒绝请求
    if len(criticalErrors) > 0 && dm.failStrategy == FailClosed {
        result.Allow = false
    }
}
```

**设计论证**：
- **为什么并行？** 7 个 OWASP 检测器 + 5 个核心检测器，串行执行延迟累加可达 100ms+，并行可控制在单个检测器延迟内
- **为什么用 channel 而非 sync.WaitGroup + slice？** channel 天然支持并发安全的数据收集，无需额外加锁
- **FailOpen vs FailClosed**：默认 FailOpen（检测器异常时放行），避免安全模块故障导致整个站点不可用。高安全场景可配置为 FailClosed

### 3.3 检测器统一架构

所有检测器遵循相同的设计模式：

```
┌─────────────────────────────────────────────┐
│  Detector Interface                         │
│  ├── Detect(req) → ([]Threat, error)        │
│  ├── Name() → string                        │
│  └── UpdateRules(rules) → error             │
├─────────────────────────────────────────────┤
│  内部结构:                                   │
│  ├── rules []types.Rule          (原始规则)  │
│  ├── compiledRules []compiledRule (预编译)   │
│  └── rulesMutex sync.RWMutex     (并发安全)  │
├─────────────────────────────────────────────┤
│  检测流程:                                   │
│  1. RLock 获取编译规则快照                    │
│  2. checkHTTPInputs(req, rules, category)    │
│     ├── URL Query 参数                       │
│     ├── POST Form 参数                       │
│     ├── Headers (Cookie, User-Agent 等)      │
│     └── 每个参数 → 正则匹配 → Threat         │
│  3. 返回 Threat 列表                         │
└─────────────────────────────────────────────┘
```

**设计论证**：
- **预编译正则**：`regexp.Compile()` 在规则加载时执行，检测时直接 `re.MatchString()`，避免每次请求重复编译
- **读写锁**：读（检测）远多于写（更新规则），`sync.RWMutex` 比 `sync.Mutex` 性能高 10x+
- **规则快照**：`make copy` 而非直接引用，避免检测过程中规则被修改导致 panic

### 3.4 各检测器规则详解

**注入攻击检测 (InjectionDetector)**：

| 规则 ID | 名称 | 正则模式 | 严重度 | 设计理由 |
|---------|------|---------|--------|---------|
| injection-001 | SQL Injection | `'\|"\|OR\s+1=1\|UNION\|SELECT\s+\*` | high | 覆盖经典 SQL 注入模式：引号闭合、永真条件、联合查询 |
| injection-002 | Command Injection | `;\|\||&\|>\|<` | high | Shell 元字符：命令分隔符、管道、重定向 |
| injection-003 | LDAP Injection | `\(\|\)\|&\|\||!\|=\|\*` | medium | LDAP 查询语法特殊字符 |

**XSS 检测 (XSSDetector)**：

| 规则 ID | 名称 | 正则模式 | 严重度 | 设计理由 |
|---------|------|---------|--------|---------|
| xss-001 | HTML Tag Injection | `<script\|</script>\|<iframe\|<object\|<embed` | high | 直接注入可执行标签 |
| xss-002 | JS Event Handler | `onload=\|onerror=\|onclick=\|onmouseover=` | high | 事件处理器注入，无需 `<script>` 标签 |
| xss-003 | JS Protocol | `javascript:\|vbscript:\|data:` | high | URL 协议注入 |
| xss-004 | HTML Attribute | `'\|"\|>\|\|/` | medium | 属性闭合注入 |

**CSRF 检测 (CSRFDetector)**：

```
检测策略（分层递进）:
  1. GET/HEAD/OPTIONS → 直接放行 (幂等请求无需 CSRF 保护)
  2. Authorization header 存在 → 放行
     设计理由: SPA 通过 JS 设置 Authorization header,
     浏览器不会在跨站请求中自动附加此头,
     因此存在有效 Authorization 即可确认同源请求
  3. CSRF Token 检查:
     - URL Query: ?csrf_token=xxx
     - Header: X-CSRF-Token / X-XSRF-Token
  4. Origin/Referer 验证:
     - Origin 与 Host 一致 → 放行
     - Referer host 与 Host 一致 → 放行
     - 不一致 → 报告 Invalid Referer
```

**反序列化检测 (DeserializationDetector)**：

| 规则 ID | 名称 | 正则模式 | 设计理由 |
|---------|------|---------|---------|
| deserialization-001 | Java Serialization | `%AC%ED%00%05\|rO0ABX\|aced0005` | Java 序列化魔术字节 (URL编码+原始) |
| deserialization-002 | Python Pickle | `%80%04\|%80%03\|c:\|(i\|(S\|(V` | Pickle 协议头 + 操作码 |
| deserialization-003 | PHP Serialization | `O:\d+:\|s:\d+:\|a:\d+:` | PHP serialize() 格式 |

**XXE 检测 (XXEDetector)**：

| 规则 ID | 名称 | 正则模式 | 严重度 | 设计理由 |
|---------|------|---------|--------|---------|
| xxe-001 | XML Entity Declaration | `<!ENTITY\s+\w+\s+` | high | 内部实体声明 |
| xxe-002 | XML External Entity | `SYSTEM\s+["']?file:\|http:\|ftp:` | critical | 外部实体引用，可直接读取文件 |
| xxe-003 | XML DOCTYPE with ENTITY | `<!DOCTYPE[^>]*\[` | high | DOCTYPE 中内嵌实体 |
| xxe-004 | XML Parameter Entity | `%\w+\s+SYSTEM` | high | 参数实体，可用于 Blind XXE |
| xxe-005 | PHP XXE Wrapper | `php://filter\|php://input\|expect://` | critical | PHP 伪协议利用 |

**额外处理**：XXE 检测器会检查 `Content-Type` 头是否包含 `xml`，仅在 XML 请求时激活完整检测。

**敏感数据检测 (SensitiveDataDetector)**：

| 规则 ID | 名称 | 正则模式 | 设计理由 |
|---------|------|---------|---------|
| sensitive-001 | Credit Card | `\d{4}-\d{4}-\d{4}-\d{4}\|\d{16}` | 信用卡号格式 |
| sensitive-002 | SSN | `\d{3}-\d{2}-\d{4}` | 美国社会安全号 |
| sensitive-003 | Password in URL | `password=\|pass=\|pwd=\|secret=` | URL 参数泄露密码 |
| sensitive-004 | API Key | `api_key=\|api-key=\|token=\|auth=` | API 密钥泄露 |

### 3.5 规则管理 (RuleManager)

```
规则加载优先级:
  1. 远程源 (remoteRuleSource)
     └── HTTP GET → JSON 反序列化 → 保存到 Redis
  2. Redis (prerender:firewall:rules)
     └── JSON 反序列化
  3. 本地文件 (rulesPath)
     └── JSON 文件读取
  4. 默认规则 (硬编码)
     └── getDefaultRules()

自动更新:
  startAutoUpdate() → time.Ticker(updateInterval)
    → ReloadRules() → 重新走优先级链
    → 新规则自动生效 (检测器 UpdateRules())
```

**设计论证**：为什么需要远程规则源？
- WAF 规则需要持续更新以应对新型攻击（类似病毒库）
- 远程源可以是 GitHub、CDN 或自建规则服务器
- Redis 缓存减少远程请求频率

### 3.6 动作处理 (ActionHandler)

```
DefaultActionHandler.Handle():
  1. 检查自定义拦截页面: staticDir/{siteName}/waf_block.html
     └── 存在 → 返回自定义 HTML
  2. 返回默认拦截页面:
     ┌──────────────────────────────────┐
     │  HTTP 403 Forbidden              │
     │  Content-Type: text/html         │
     │                                  │
     │  <h1>Access Denied</h1>          │
     │  <p>{BlockMessage}</p>           │
     │  <div>Prerender Shield WAF</div> │
     └──────────────────────────────────┘
```

---

## 四、预渲染引擎

### 4.1 引擎架构

```
EngineManager (engine_manager.go)
    │
    └── map[siteID]*engine
        │
        ├── BrowserPool (pool/pool.go, 540行)
        │   ├── 实例池: available chan *Instance (缓冲=MaxInstances)
        │   ├── 生命周期: 创建 → 使用 → 归还 → 超时回收
        │   └── 健康检查: chromedp.Evaluate("1+1") 每 30s
        │
        ├── ConcurrencyManager (concurrency_manager.go, 224行)
        │   ├── 自适应并发限制
        │   └── 基于渲染性能动态调整
        │
        ├── Cache System (cache/)
        │   ├── L1: 内存缓存 (sync.Map)
        │   └── L2: Redis 缓存 (prerender:cache:{hash})
        │
        ├── PersistentQueue (persistent_queue.go, 48行)
        │   └── Redis List 持久化渲染队列
        │
        ├── Crawler (crawler.go, 403行)
        │   ├── 链接爬取: BFS + 深度限制 + visited 去重
        │   └── 并发控制: 信号量 semaphore chan
        │
        ├── PreheatWorker (preheat.go, 345行)
        │   ├── Sitemap 解析 → 批量预热
        │   └── 7 种默认爬虫 UA 轮换
        │
        ├── SEOInjector (seo_injector.go, 64行)
        │   ├── Meta 标签优化注入
        │   ├── 结构化数据 (JSON-LD) 注入
        │   └── Canonical URL 注入
        │
        ├── Streaming (streaming/)
        │   ├── Chunked 传输 (chunked.go)
        │   └── 首屏优先 (first_screen.go)
        │
        ├── Incremental (incremental/)
        │   ├── DOM Diff (dom_diff.go)
        │   └── 选择性重渲染 (selective.go)
        │
        └── Optimizer (optimizer/)
            └── 资源加载优化 (optimizer.go)
```

### 4.2 渲染流程详解

```
Render(url, timeout)
    │
    ▼
renderWithRetry() — 指数退避重试
    │
    参数:
      MaxRetries: 3
      BaseDelay:  500ms
      MaxDelay:   5s
    │
    算法: delay = min(BaseDelay * 2^attempt, MaxDelay)
    │
    ▼
renderOnce(url, timeout)
    │
    ├── 1. 缓存检查
    │   ├── L1 内存: sync.Map.Load(cacheKey)
    │   └── L2 Redis: GET prerender:cache:{sha256(url)}
    │   设计理由: 双层缓存减少 Redis 网络开销,
    │   内存缓存命中时延迟 <1μs, Redis 命中时 ~1ms
    │
    ├── 2. 并发控制
    │   └── ConcurrencyManager.Acquire()
    │       设计理由: 防止 Chromium 实例耗尽导致 OOM,
    │       动态限制根据系统负载自适应调整
    │
    ├── 3. 获取浏览器实例
    │   └── BrowserPool.Acquire()
    │       ├── <-available (阻塞等待空闲实例)
    │       ├── 超时 → 创建新实例 (不超过 MaxInstances)
    │       └── 创建实例:
    │           ├── chromedp.NewExecAllocator() → Chrome 启动参数
    │           │   ├── --headless (无头模式)
    │           │   ├── --disable-gpu
    │           │   ├── --no-sandbox (Docker 环境必需)
    │           │   ├── --disable-dev-shm-usage
    │           │   └── --remote-debugging-port=0 (随机端口)
    │           └── chromedp.NewContext() → CDP 连接
    │
    ├── 4. chromedp 渲染
    │   ├── chromedp.Navigate(url)         → 页面导航
    │   ├── chromedp.WaitReady("body")     → 等待 DOM 就绪
    │   ├── chromedp.Sleep(extraWait)      → 额外等待 (JS 异步渲染)
    │   └── chromedp.OuterHTML("html")     → 获取完整 HTML
    │
    ├── 5. SEO 注入
    │   └── SEOInjector.InjectSEOTags(html, url)
    │       ├── MetaTagsOptimizer.OptimizeMetaTags()
    │       ├── StructuredDataOptimizer.InjectStructuredData()
    │       └── Canonical URL 注入
    │
    ├── 6. 写入缓存
    │   ├── L1 内存: sync.Map.Store(cacheKey, html)
    │   └── L2 Redis: SET prerender:cache:{hash} {html} EX {ttl}
    │
    ├── 7. 归还实例
    │   └── BrowserPool.Release(instance)
    │       ├── UseCount++ → 超过 MaxUseCount → 销毁重建
    │       └── available <- instance
    │
    └── 8. 返回 HTML
```

### 4.3 浏览器实例池 (pool/pool.go, 540行)

```
Pool 结构:
  ┌─────────────────────────────────────────────┐
  │  config        Config                        │
  │  instances     []*Instance   (所有实例)       │
  │  available     chan *Instance (空闲通道)      │
  │  instanceCount int            (当前数量)      │
  │  mu            sync.RWMutex                  │
  │  closed        bool                          │
  └─────────────────────────────────────────────┘

Instance 结构:
  ┌─────────────────────────────────────────────┐
  │  ID           string         (UUID)          │
  │  AllocCtx     context.Context (Chrome 进程)   │
  │  ChromeCtx    context.Context (CDP 连接)      │
  │  CreatedAt    time.Time                      │
  │  LastUsedAt   time.Time                      │
  │  UseCount     int            (使用次数)       │
  │  MaxUseCount  int            (默认 100)       │
  │  IsHealthy    bool                           │
  └─────────────────────────────────────────────┘

配置:
  MinInstances:        2     (最小实例数，启动时预创建)
  MaxInstances:        10    (最大实例数，防止 OOM)
  IdleTimeout:         5min  (空闲回收)
  MaxUseCount:         100   (单实例最大使用次数，防止内存泄漏)
  HealthCheckInterval: 30s   (健康检查间隔)
  Headless:            true  (无头模式)

实例生命周期:
  ┌─────────┐    Acquire()    ┌─────────┐
  │ available│ ←────────────── │  使用中  │
  │ channel  │ ──────────────→ │         │
  └─────────┘    Release()    └─────────┘
       │                           │
       │ 空闲 > IdleTimeout        │ UseCount > MaxUseCount
       ▼                           ▼
   ┌─────────┐               ┌─────────┐
   │  回收    │               │ 销毁重建 │
   └─────────┘               └─────────┘

健康检查:
  chromedp.Evaluate("1+1")
    → 超时/错误 → IsHealthy = false → 销毁重建
```

**设计论证**：
- **为什么用 channel 而非 sync.Pool？** `sync.Pool` 会自动 GC 回收，不适合需要精确控制生命周期的 Chromium 实例
- **为什么限制 MaxUseCount？** Chromium 长时间运行会积累内存碎片，定期重建防止内存泄漏
- **为什么预创建 MinInstances？** 避免首次请求的冷启动延迟（Chrome 启动 ~500ms）

### 4.4 动态并发管理 (concurrency_manager.go, 224行)

```
ConcurrencyManager 自适应算法:

状态变量:
  currentLimit    int       (当前并发限制)
  minLimit        int       (最小限制，默认 2)
  maxLimit        int       (最大限制，默认 10)
  activeCount     int       (当前活跃数)
  successCount    int64     (成功计数)
  failureCount    int64     (失败计数)
  avgRenderTime   float64   (平均渲染时间)
  renderTimes     []float64 (最近 100 次渲染时间)
  adjustInterval  Duration  (调整间隔，默认 30s)

调整逻辑 (每 30s):
  successRate = successCount / (successCount + failureCount)
  avgRenderTime = average(renderTimes)

  if avgRenderTime < 2s && successRate > 0.95:
    currentLimit = min(currentLimit * 1.2, maxLimit)  // 提升 20%
    设计理由: 系统负载低，可以增加并发提高吞吐

  if avgRenderTime > 10s || successRate < 0.8:
    currentLimit = max(currentLimit * 0.8, minLimit)  // 降低 20%
    设计理由: 系统过载，降低并发保护稳定性

Acquire():
  if activeCount < currentLimit:
    activeCount++
    return true
  return false  // 调用方需等待或返回错误
```

**设计论证**：为什么需要动态并发？
- 固定并发：低负载浪费资源，高负载导致超时
- 自适应并发：根据实际渲染性能自动调整，兼顾吞吐和稳定性
- 类似于 TCP 拥塞控制 (AIMD 算法)

### 4.5 缓存预热 (Preheat)

```
触发方式:
  1. 手动: POST /api/v1/preheat/trigger {siteId}
  2. 定时: Cron 表达式 (Scheduler)

预热流程:
  ┌─────────────┐
  │ Sitemap URL  │
  └──────┬──────┘
         │ HTTP GET
         ▼
  ┌─────────────┐
  │ XML 解析     │  提取所有 <url><loc>
  └──────┬──────┘
         │
         ▼
  ┌─────────────┐
  │ 创建预热任务  │  taskID = UUID
  └──────┬──────┘
         │
         ▼
  ┌─────────────┐
  │ 并发渲染      │  concurrency: 5
  │              │  semaphore chan 控制
  └──────┬──────┘
         │
    每个 URL:
    ├── 获取浏览器实例
    ├── chromedp 渲染
    ├── SEO 注入
    ├── 写入缓存
    └── 更新进度 (Redis)

默认爬虫 UA (7种):
  Baiduspider, Googlebot, Sogou spider,
  Bytespider, HaosouSpider, YisouSpider, bingbot
```

### 4.6 搜索引擎推送 (push/manager.go)

```
百度推送:
  POST http://data.zz.baidu.com/urls?site={domain}&token={token}
  Content-Type: text/plain
  Body: 每行一个 URL
  限额: BaiduDailyLimit (默认 1000/天)

必应推送:
  POST https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl
  Content-Type: application/json
  Body: {"siteUrl": "...", "urlList": ["..."]}
  限额: BingDailyLimit (默认 1000/天)

日限额控制:
  Redis Key: push:{site}:daily:{YYYY-MM-DD}
  每次推送前 INCR，超过限额则跳过
```

### 4.7 流式渲染 (streaming/)

```
Chunked 传输 (chunked.go):
  渲染过程中分块输出 HTML
  Transfer-Encoding: chunked
  适用场景: 大页面，爬虫可以边接收边解析

首屏优先 (first_screen.go):
  先渲染首屏内容 → 立即返回
  后台继续渲染剩余内容 → 更新缓存
  适用场景: 对响应时间敏感的场景
```

### 4.8 增量渲染 (incremental/)

```
DOM Diff (dom_diff.go):
  对比新旧 HTML，仅重渲染变化的部分
  适用场景: 页面内容局部更新

选择性重渲染 (selective.go):
  根据 URL pattern 决定是否重新渲染
  适用场景: 静态资源页面无需重复渲染
```

### 4.9 持久化队列 (persistent_queue.go, 48行)

```
PersistentQueue:
  Redis List 实现 FIFO 队列
  Key: {prefix}:list

  Enqueue(task):
    JSON 序列化 → RPUSH {prefix}:list

  Dequeue():
    LPOP {prefix}:list → JSON 反序列化

  Clear():
    DEL {prefix}:list

设计理由:
  - 服务重启后队列不丢失
  - 支持多实例共享队列 (Redis 原子操作)
  - JSON 序列化保证任务完整性
```

---

## 五、SEO 优化引擎

### 5.1 架构

```
SEOInjector (prerender/seo_injector.go, 64行)
    │
    ├── MetaTagsOptimizer (seo/meta_tags.go, 580行)
    │   ├── analyzeTitle()         → TitleAnalysis
    │   ├── analyzeDescription()   → DescriptionAnalysis
    │   ├── extractKeywords()      → []string (Top 10)
    │   ├── detectMissingTags()    → []string
    │   ├── generateMetaTags()     → map[string]string
    │   ├── generateOpenGraph()    → map[string]string
    │   ├── generateTwitterCard()  → map[string]string
    │   └── BuildOptimizedHTML()   → 注入优化标签
    │
    └── StructuredDataOptimizer (seo/structured_data.go, 648行)
        ├── detectPageType()       → Article/Product/FAQ/...
        ├── generateArticleSchema()
        ├── generateProductSchema()
        ├── generateOrganizationSchema()
        ├── generateLocalBusinessSchema()
        ├── generateFAQSchema()
        ├── generateBreadcrumbSchema()
        ├── validateStructuredData()
        └── InjectStructuredData()  → <script type="application/ld+json">
```

### 5.2 Meta 标签优化流程

```
OptimizeMetaTags(html, targetKeywords)
    │
    ├── 1. analyzeTitle()
    │   输入: HTML 字符串
    │   处理:
    │     - 正则提取 <title>...</title>
    │     - 长度检查: 30-60 字符为最优
    │     - 关键词密度: 目标关键词在标题中的出现比例
    │     - 品牌词检测: 标题是否以 "| 品牌名" 或 "- 品牌名" 结尾
    │   输出: TitleAnalysis {Original, Optimized, Length, IsOptimal, Issues, KeywordDensity}
    │
    ├── 2. analyzeDescription()
    │   输入: HTML 字符串
    │   处理:
    │     - 正则提取 <meta name="description" content="...">
    │     - 长度检查: 120-160 字符为最优
    │     - CTA 检测: 是否包含号召性用语 (立即/免费/开始/了解/发现/获取/查看)
    │     - 缺失处理: 从第一个 <p> 或 <h1> 自动生成描述
    │   输出: DescriptionAnalysis
    │
    ├── 3. extractKeywords()
    │   算法:
    │     - 移除所有 HTML 标签
    │     - 按非字母数字字符分词
    │     - 过滤长度 < MinKeywordLength 的词
    │     - 词频统计 → 排序 → Top N (MaxKeywords=10)
    │   设计理由: 基于 TF (词频) 的简单但有效的关键词提取,
    │   适合中文和英文混合内容
    │
    ├── 4. detectMissingTags()
    │   检查项: title, description, viewport, charset, canonical, og:title
    │
    ├── 5-7. 生成标签
    │   MetaTags:    title, description, keywords, author, robots
    │   OpenGraph:   og:title, og:description, og:type("website"), og:locale("zh_CN"), og:url
    │   TwitterCard: twitter:card("summary_large_image"), twitter:title, twitter:description
    │
    └── 8. BuildOptimizedHTML()
        将优化后的标签注入 HTML:
        - 替换已有标签 (正则匹配)
        - 插入缺失标签 (<head> 后)
        - 插入 canonical link
```

### 5.3 结构化数据注入

**支持的 Schema.org 类型及字段**：

| 类型 | 必需字段 | 可选字段 | 适用场景 |
|------|---------|---------|---------|
| Article | headline | image, datePublished, author, publisher | 博客、新闻 |
| Product | name | description, image, offers, brand, sku, mpn | 电商 |
| Organization | name | url, logo, sameAs, contactPoint | 企业官网 |
| LocalBusiness | name | telephone, address, geo, openingHours, priceRange | 本地商家 |
| FAQPage | mainEntity[] | — | FAQ 页面 |
| BreadcrumbList | itemListElement[] | — | 全站导航 |

**自动页面类型检测 (detectPageType)**：

```
检测逻辑 (优先级从高到低):
  1. <article> 标签 → Article
  2. product/价格/库存/¥/$ → Product
  3. faq/常见问题/问答/<details> → FAQPage
  4. breadcrumb/面包屑 → BreadcrumbList
  5. address/电话/地址/营业时间 → LocalBusiness
  6. about/关于我们/公司简介 → Organization
  7. 默认 → WebPage
```

**注入格式**：

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Article",
  "headline": "优化后的标题",
  "description": "优化后的描述",
  "datePublished": "2026-06-13T00:00:00+08:00",
  "author": {
    "@type": "Person",
    "name": "作者名"
  }
}
</script>
```

### 5.4 AI 爬虫优化 AEO (seo/aeo.go, 108行)

**识别的 AI 爬虫**：

| Bot Token | 所属公司 | 用途 | 处理策略 |
|-----------|---------|------|---------|
| gptbot | OpenAI | 训练数据采集 | 提供纯净文本 |
| claudebot | Anthropic | 训练数据采集 | 提供纯净文本 |
| perplexitybot | Perplexity AI | 实时搜索 | 提供结构化摘要 |
| google-extended | Google (Gemini) | 训练数据采集 | 提供纯净文本 |
| cohere-ai | Cohere | 训练数据采集 | 提供纯净文本 |
| facebookbot | Meta AI | 训练数据采集 | 提供纯净文本 |
| applebot | Apple | 搜索索引 | 标准 HTML |
| bytespider | ByteDance | 训练数据采集 | 提供纯净文本 |

**处理策略**：

```
IsAICrawler(UA) → 匹配
    │
    ├── purpose = "training"
    │   └── ExtractAnswer(html)
    │       移除: <script>, <style>, <nav>, <footer>, <header>
    │       保留: <article>, <main>, <section> 中的文本内容
    │       设计理由: AI 训练需要纯净文本, 导航/广告/脚本是噪声
    │
    └── purpose = "search"
        └── 返回标准 HTML (含结构化数据)
            设计理由: AI 搜索引擎需要完整页面结构来理解内容
```

---

## 六、认证与安全

### 6.1 JWT 认证流程

```
Token 结构:
┌──────────────────────────────────────────────┐
│ Header                                         │
│   {"alg": "HS256", "typ": "JWT"}              │
├──────────────────────────────────────────────┤
│ Payload                                        │
│   user_id:    "ea980510-..."  (UUID)          │
│   username:   "admin"                         │
│   session_id: "af81ef6d-..." (UUID)           │
│   exp:        1781396758      (now + 24h)     │
│   iat:        1781310358      (now)           │
│   nbf:        1781310358      (now)           │
│   iss:        "prerender-shield"              │
│   sub:        "ea980510-..."  (= user_id)     │
│   jti:        "af81ef6d-..."  (= session_id)  │
├──────────────────────────────────────────────┤
│ Signature                                      │
│   HMAC-SHA256(base64UrlEncode(header) + "." + │
│                base64UrlEncode(payload),       │
│                secretKey)                      │
└──────────────────────────────────────────────┘

Session 管理:
  生成 Token 时:
    Redis HSET session:{session_id}
      user_id    = "ea980510-..."
      username   = "admin"
      session_id = "af81ef6d-..."
      created_at = "2026-06-13T00:00:00Z"
    EXPIRE session:{session_id} 86400  (24h)

  验证 Token 时:
    1. jwt.ParseWithClaims(token, &Claims{}, keyFunc)
       → 验证签名 + 过期时间
    2. Redis EXISTS session:{claims.SessionID}
       → 验证会话未被撤销

  注销时:
    Redis DEL session:{session_id}
    → Token 立即失效 (即使 JWT 未过期)

设计论证: 为什么需要服务端 Session？
  - 纯 JWT 无法实现主动注销 (令牌签发后无法撤销)
  - Redis Session 提供注销能力: 删除 Session → 令牌立即失效
  - 降级策略: Redis 不可用时仍接受有效 JWT (可用性优先)
```

### 6.2 2FA 双因素认证 (TOTP)

```
RFC 6238 TOTP 实现 (auth/totp_manager.go):

密钥生成:
  TOTPManager.GenerateSecret()
    → crypto/rand 生成 20 字节随机数
    → Base32 编码 (兼容 Google Authenticator)

QR Code 生成:
  TOTPManager.GenerateQRCode(userID, secret)
    → otpauth://totp/PrerenderShield:{userID}
      ?secret={base32_secret}
      &issuer=PrerenderShield
      &algorithm=SHA1
      &digits=6
      &period=30

验证:
  TOTPManager.ValidateCode(secret, code)
    → 当前时间窗口: now / 30
    → 计算 TOTP(secret, window) → 6 位数字
    → 容差: ±1 步 (允许 30s 时钟偏差)
    → 防重放: 记录已使用的 code (Redis, TTL 60s)

启用流程:
  POST /2fa/enable
    → 生成密钥 → Redis: 2fa:pending:{userID} (TTL 15min)
    → 返回 {secret, qr_code_url}

  POST /2fa/confirm {code}
    → 验证 code → Redis: 2fa:enabled:{userID} (永久)
    → 删除 pending key

  POST /2fa/disable {code}
    → 验证 code → 删除 2fa:enabled:{userID}
```

### 6.3 密码安全

```
密码强度验证 (ValidatePasswordStrength):
  规则:
    - 长度 ≥ 6 字符
    - 必须包含至少 1 个字母 (unicode.IsLetter)
    - 必须包含至少 1 个数字 (unicode.IsDigit)
  设计理由: 防止弱密码 (如 "123456", "password")

密码存储:
  bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
  DefaultCost = 10 → 2^10 = 1024 次迭代
  设计理由: bcrypt 内置盐值, 抗彩虹表, 计算成本可配置

密码验证:
  bcrypt.CompareHashAndPassword(hashedPassword, password)
  常数时间比较, 防止时序攻击
```

### 6.4 审计日志 (audit/audit.go, 196行)

```
审计操作类型 (14种):
  login, logout, config.update,
  site.create, site.update, site.delete,
  cert.request, cert.renew, cert.delete,
  preheat.trigger, waf.rule.update,
  blacklist.update, whitelist.update,
  system.config

严重级别:
  info     → 常规操作 (login, logout)
  warning  → 需关注 (config.update)
  critical → 严重事件 (cert.delete)

存储:
  Redis Key: audit:{timestamp}
  格式: JSON
  {
    "id": "uuid",
    "user_id": "...",
    "action": "site.delete",
    "resource": "site:site1",
    "detail": "Site example-site deleted",
    "severity": "warning",
    "client_ip": "192.168.1.1",
    "status": "success",
    "metadata": {"site_name": "example-site"},
    "timestamp": "2026-06-13T00:00:00Z"
  }
```

---

## 七、SSL/TLS 证书管理

### 7.1 ACME 协议集成 (ssl/manager.go, 512行)

```
证书申请流程:
  POST /api/v1/ssl/certificates {domains: ["example.com"]}
      │
      ▼
  ssl.Manager.RequestCertificate()
      │
      ├── 1. 创建 ACME 客户端
      │   ├── Staging:  https://acme-staging-v02.api.letsencrypt.org/directory
      │   └── Production: https://acme-v02.api.letsencrypt.org/directory
      │   设计理由: Staging 用于测试, 无速率限制;
      │   Production 用于正式证书
      │
      ├── 2. 生成 RSA 2048 位私钥
      │   └── rsa.GenerateKey(rand.Reader, 2048)
      │
      ├── 3. 创建 CSR
      │   └── x509.CreateCertificateRequest()
      │       Subject: CN=example.com
      │       DNSNames: [example.com, www.example.com]
      │
      ├── 4. 域名验证
      │   ├── HTTP-01 (单域名):
      │   │   ├── 启动临时 HTTP Server (:80)
      │   │   ├── 提供 token: /.well-known/acme-challenge/{token}
      │   │   └── Let's Encrypt 验证 → 匹配 → 颁发证书
      │   │
      │   └── DNS-01 (通配符):
      │       ├── 调用 DNS 服务商 API
      │       │   ├── Cloudflare API
      │       │   ├── Aliyun DNS API
      │       │   └── TencentCloud DNS API
      │       ├── 添加 TXT 记录: _acme-challenge.{domain}
      │       └── Let's Encrypt 验证 → 匹配 → 颁发证书
      │
      └── 5. 保存证书
          ├── 文件系统: certs/{domain}.crt, certs/{domain}.key
          └── Redis: ssl:cert:{domain} → JSON
```

### 7.2 自动续期 (ssl/auto_renew.go)

```
AutoRenew 机制:
  定时检查 (check_interval)
    → 遍历所有证书
    → 解析 x509.ParseCertificate()
    → NotAfter - now < renew_before_days (默认 30 天)
    → 触发续签

  失败重试:
    max_retries (默认 3)
    retry_delay (指数退避)

  通知:
    webhook_url → POST JSON
    {
      "event": "cert.renewed",
      "domain": "example.com",
      "new_expiry": "2026-09-13T00:00:00Z"
    }
```

---

## 八、监控与告警

### 8.1 系统监控 (monitor.go, 952行)

```
数据采集 (gopsutil):
  CPU:
    cpu.Percent(interval, false) → 总体 CPU 使用率
    设计理由: interval 参数控制采样周期, false=总体而非每核

  内存:
    mem.VirtualMemory()
    → Total, Available, Used, UsedPercent
    设计理由: VirtualMemory 包含 Swap, 比 RSS 更准确

  磁盘:
    disk.Usage("/")
    → Total, Free, Used, UsedPercent

  网络:
    net.IOCounters(false)
    → BytesSent, BytesRecv, PacketsSent, PacketsRecv

Prometheus 指标:
  ┌──────────────────────────────────────────────────────┐
  │ prerender_requests_total{method, path, status}       │ Counter
  │ prerender_response_time_seconds{method, path}        │ Histogram
  │ prerender_crawler_requests_total                     │ Counter
  │ prerender_blocked_requests_total                     │ Counter
  │ prerender_cache_hits_total                           │ Counter
  │ prerender_cache_misses_total                         │ Counter
  │ prerender_active_browsers                            │ Gauge
  │ prerender_render_time_seconds                        │ Histogram
  └──────────────────────────────────────────────────────┘

指标持久化 (MetricsPersistenceConfig):
  interval:          300s  (持久化间隔)
  retention_hours:   24    (数据保留)
  aggregate_enabled: true  (启用聚合)
  aggregate_interval: 3600s (聚合粒度)
```

### 8.2 告警引擎 (alerting/rules.go, 316行)

```
Rule 结构:
  ┌─────────────────────────────────────┐
  │ ID:          "cpu_high"             │
  │ Name:        "CPU 使用率过高"        │
  │ Description: "CPU > 90%"            │
  │ Enabled:     true                   │
  │ Condition:                          │
  │   Metric:    "system_cpu_usage"     │
  │   Operator:  "gt"                   │
  │   Threshold: 90.0                   │
  │   Duration:  5m                     │
  │ Severity:    "critical"             │
  │ Handlers:    ["webhook", "email"]   │
  │ Cooldown:    5m                     │
  └─────────────────────────────────────┘

触发逻辑:
  if metric.CurrentValue > Condition.Threshold (持续 Duration):
    if now - lastTriggered > Cooldown:
      触发告警 → 调用所有 Handler.Send()
      lastTriggered = now

通知渠道:
  Webhook:
    POST {url}
    Body: {"id":"...", "rule_id":"cpu_high", "severity":"critical",
           "message":"CPU 使用率: 95.2%", "value":95.2, "timestamp":"..."}

  Email:
    SMTP → {from} → {to}
    Subject: [PrerenderShield Alert] CPU 使用率过高

  Slack:
    POST {webhook_url}
    Body: {"text": "⚠️ CPU 使用率: 95.2%"}

  钉钉:
    POST {webhook_url}
    Body: {"msgtype": "text", "text": {"content": "CPU 使用率: 95.2%"}}
```

### 8.3 遥测导出 (telemetry/)

```
OpenTelemetry:
  TracerProvider
    → OTLP HTTP Exporter
    → 目标: otel-collector:4318

  Span 属性:
    http.method, http.url, http.status_code,
    http.duration_ms, user_id

Prometheus:
  /metrics 端点 → Prometheus Server 定期抓取
  scrape_interval: 15s
```

---

## 九、日志与审计

### 9.1 日志系统架构

```
Logger (log.go, 309行)
    │
    ├── 5 级日志: DEBUG, INFO, WARN, ERROR, FATAL
    │
    ├── StructuredLogger (structured_logger.go)
    │   └── JSON 格式输出
    │       {"timestamp":"...", "level":"INFO",
    │        "event_type":"admin_action", "user":"admin",
    │        "action":"site_create", "result":"success"}
    │
    ├── CrawlerLogManager (crawler_log.go)
    │   └── 爬虫访问日志
    │       字段: IP, UA, Route, Status, HitCache,
    │             RenderTime, Country, City, Lat, Lng
    │
    ├── VisitLogManager (visit_log.go)
    │   └── 普通访问日志
    │       字段: IP, Method, Path, Status, Duration,
    │             Country, City, Lat, Lng
    │
    └── LogProcessor (services/log_processor.go, 155行)
        └── 异步日志清洗 + GeoIP 富化 + 自动封禁
```

### 9.2 日志清洗流程 (LogProcessor)

```
LogProcessor.Start()
    │
    └── 每 5 秒循环:
        │
        ├── processCrawlerLogs()
        │   ├── GetUnwashedLogs(10) → 获取未清洗日志
        │   ├── 按 IP 分组 (减少 API 调用)
        │   ├── GeoIPService.GetLocation(ip)
        │   │   └── 3 个 API 轮询 (ip-api.com → ipapi.co → geojs.io)
        │   ├── 填充: Country, CountryCode, City, Latitude, Longitude
        │   ├── UpdateLog() → 标记 washed=true
        │   └── checkAndBan()
        │       └── CountryCode 匹配 GeoIP BlockList → 自动封禁 IP
        │
        └── processVisitLogs()
            └── 同上流程

设计理由: 为什么异步清洗？
  - GeoIP API 调用延迟 100-500ms，同步处理会阻塞请求
  - 批量处理减少 API 调用次数 (按 IP 分组)
  - 自动封禁实现零人工干预的安全响应
```

### 9.3 GeoIP 服务 (services/geoip.go, 370行)

```
API 提供商轮询 (queryAPIWithFallback):
  优先级:
    1. ip-api.com    → http://ip-api.com/json/{ip}
       限制: 45 请求/分钟 (免费版)
       返回: country, countryCode, city, lat, lon

    2. ipapi.co      → https://ipapi.co/{ip}/json/
       限制: 1000 请求/天 (免费版)
       返回: country_name, country_code, city, latitude, longitude

    3. get.geojs.io  → https://get.geojs.io/v1/ip/geo/{ip}.json
       限制: 无明确限制
       返回: country, country_code, city, latitude, longitude

  轮询策略: 每个提供商失败后等待 100ms 尝试下一个

内网 IP 处理:
  检测: 127.0.0.1, ::1, localhost, 10.x, 192.168.x, 172.x
  回退: 返回本机位置 (serverLocation)
  初始化: 异步获取本机公网 IP 位置 (5 次重试)

缓存:
  sync.Map (IP → *GeoLocation)
  设计理由: 内存缓存避免重复 API 调用,
  sync.Map 适合读多写少的缓存场景
```

---

## 十、数据存储与缓存

### 10.1 Redis 客户端 (redis/client.go, 979行)

```
连接池配置:
  PoolSize:     20    (最大连接数)
  MinIdleConns: 10    (最小空闲连接)
  IdleTimeout:  5min  (空闲连接回收)
  PoolTimeout:  30s   (获取连接超时)
  DialTimeout:  5s    (建立连接超时)
  ReadTimeout:  10s   (读超时)
  WriteTimeout: 10s   (写超时)
  MaxRetries:   3     (命令重试次数)

设计理由:
  - PoolSize=20: 满足 50+ QPS 的并发需求
  - MinIdleConns=10: 避免频繁创建连接
  - MaxRetries=3: 网络抖动自动恢复
```

### 10.2 熔断器 (circuit_breaker.go, 178行)

```
状态机:
  ┌──────────┐  失败 > FailureThreshold  ┌──────────┐
  │  Closed  │ ─────────────────────────→ │   Open   │
  │ (正常)   │                            │ (熔断)   │
  └──────────┘                            └────┬─────┘
       ▲                                       │
       │                               Timeout 后
       │                                       ▼
       │                              ┌──────────────┐
       │         成功 > SuccessThreshold│  HalfOpen    │
       └────────────────────────────── │ (半开, 试探) │
                                       └──────────────┘

配置:
  FailureThreshold: 5   (连续 5 次失败 → 熔断)
  SuccessThreshold: 3   (半开状态连续 3 次成功 → 恢复)
  Timeout: 30s          (熔断 30s 后 → 半开)

设计理由:
  - 防止级联故障: Redis 故障时快速失败, 不阻塞请求线程
  - 自动恢复: 半开状态探测 Redis 是否恢复
  - 标准熔断器模式 (参考 Netflix Hystrix)
```

### 10.3 Redis Key 设计

```
用户:
  user:{id}              → Hash   {id, username, password}
  username:{name}        → String  userID
  users:all              → Set    所有 userID

会话:
  session:{id}           → Hash   {user_id, username, session_id, created_at}
  TTL: 24h

配置:
  prerender:config:sites → String  YAML 序列化
  system:backup:{ts}     → String  配置备份

缓存:
  prerender:cache:{sha256(url)} → String  渲染 HTML
  TTL: CacheTTL (默认 3600s)

防火墙:
  prerender:firewall:rules      → String  规则 JSON
  firewall:{site}:blacklist     → Set     IP 黑名单
  firewall:{site}:whitelist     → Set     IP 白名单
  rate:{site}:{ip}:{window}     → String  频率计数 (TTL=window)

站点:
  site:{id}:stats        → Hash   站点统计
  site:{id}_prerender    → Hash   预渲染配置
  site:{id}:urls         → Set    预热 URL

域名:
  domain:{domain}        → String  siteID
  domains                → Set    所有域名

审计:
  audit:{timestamp}      → String  审计条目 JSON

2FA:
  2fa:pending:{userID}   → String  临时密钥 (TTL 15min)
  2fa:enabled:{userID}   → String  永久密钥

推送:
  push:{site}:daily:{date} → String  日推送计数 (TTL 24h)

任务队列:
  task:queue:{prefix}:list → List  持久化任务队列
```

### 10.4 缓存管理器 (cache/manager.go)

```
多级缓存:
  L1: 内存缓存 (sync.Map)
    优点: 亚微秒级延迟
    缺点: 进程重启丢失, 容量受内存限制

  L2: Redis 缓存
    优点: 持久化, 多实例共享
    缺点: 网络延迟 ~1ms

读取策略 (Read-Through):
  1. L1 查询 → 命中 → 返回
  2. L1 未命中 → L2 查询 → 命中 → 回填 L1 → 返回
  3. L2 未命中 → 返回 nil

写入策略 (Write-Through):
  1. 写入 L2 (Redis)
  2. 写入 L1 (内存)

失效策略:
  TTL 过期 (默认 3600s)
  手动清除: POST /preheat/clear-cache
```

---

## 十一、配置管理

### 11.1 配置加载 (config/loader.go, 374行)

```
LoadConfig(configPath):
    │
    ├── 1. 确定配置路径 (优先级):
    │   ├── 命令行 -config 参数
    │   ├── ./configs/config.yml
    │   ├── ./config.yml
    │   ├── {appDir}/config.yml
    │   └── {appDir}/config/config.yml
    │
    ├── 2. 环境变量覆盖:
    │   ├── PRERENDER_MAX_INSTANCES → Prerender.PoolSize
    │   ├── PRERENDER_MIN_INSTANCES → Prerender.MinPoolSize
    │   ├── REDIS_URL → Cache.RedisURL
    │   └── GIN_MODE → release/debug
    │
    ├── 3. YAML 文件加载:
    │   └── yaml.Unmarshal(file, &config)
    │
    ├── 4. Redis 加载:
    │   └── GET prerender:config:sites → YAML 反序列化
    │   设计理由: Redis 中的站点配置优先级高于文件,
    │   因为 Web UI 修改后先写入 Redis
    │
    ├── 5. 配置验证 (validator.go):
    │   ├── 端口范围: 1-65535
    │   ├── 模式校验: proxy/static/redirect
    │   └── 必填字段检查
    │
    └── 6. 启动配置监控 (watcher.go)
```

### 11.2 配置热重载 (config/watcher.go)

```
Watcher:
  fsnotify 监控配置文件
    ↓ 文件变化
  ReloadConfig()
    ├── 重新加载 YAML
    ├── 验证新配置
    ├── 对比差异 (哪些站点新增/修改/删除)
    └── 通知 ConfigChangeHandler:
        ├── 站点服务器重启 (StopSiteServer → StartSiteServer)
        ├── 防火墙规则更新 (RuleManager.ReloadRules)
        └── 缓存策略更新 (CacheManager.UpdateConfig)

设计理由:
  - 无需重启进程即可更新配置
  - 站点级别的变更仅影响单个站点服务器
  - 配置变更审计日志记录
```

### 11.3 配置加密 (crypto/encryptor.go, 220行)

```
AES-256-GCM 加密:
  密钥派生:
    if len(secretKey) < 16:
      key = SHA256(secretKey)[:32]  → 32 字节 (AES-256)
    else:
      key = padKey(secretKey)[:32]

  加密:
    nonce = crypto/rand(12)          → 随机 12 字节
    ciphertext = AES-GCM-Seal(nonce, plaintext)
    output = Base64(nonce + ciphertext)

  解密:
    data = Base64Decode(input)
    nonce = data[:12]
    ciphertext = data[12:]
    plaintext = AES-GCM-Open(nonce, ciphertext)

设计理由:
  - GCM 模式提供认证加密 (AEAD), 防止密文篡改
  - 随机 nonce 防止相同明文产生相同密文
  - Base64 编码便于存储在 YAML 配置文件中
```

---

## 十二、任务调度与队列

### 12.1 调度器 (scheduler/scheduler.go, 315行)

```
Scheduler:
  ┌─────────────────────────────────────┐
  │ cron: *cron.Cron (robfig/cron)      │
  │ tasks: map[siteID]cron.EntryID      │
  │ engineManager: *EngineManager       │
  │ pushManager: *PushManager           │
  └─────────────────────────────────────┘

启动流程:
  Start()
    ├── cron.Start() → 启动 Cron 调度器
    └── go monitorSites() → 站点监控协程

monitorSites():
  每 30s 检查:
    ├── 遍历所有站点配置
    ├── 检查预热调度 (Preheat.Schedule)
    │   └── 新增/变更 → cron.AddFunc(schedule, preheatTask)
    └── 检查推送调度
        └── 新增/变更 → cron.AddFunc(schedule, pushTask)

Cron 表达式: 秒 分 时 日 月 周
  示例: "0 0 2 * * *" → 每天凌晨 2 点
```

## 十三、智能路由与代理

### 13.1 智能路由 (routing/router.go, 403行)

```
Router:
  ┌─────────────────────────────────────┐
  │ rules: []RouteRule                  │
  │   ├── ID, Pattern (正则), Action    │
  │   └── Priority (排序)               │
  │                                     │
  │ matcher: Matcher                    │
  │   └── Match(req, rule) → bool      │
  │                                     │
  │ cache: MemoryCache                  │
  │   └── 路由结果缓存 (减少正则计算)    │
  └─────────────────────────────────────┘

MemoryCache:
  ┌─────────────────────────────────────┐
  │ cache: map[string]cacheItem         │
  │   ├── key: request signature        │
  │   ├── value: 路由结果               │
  │   └── expiration: TTL               │
  └─────────────────────────────────────┘

路由流程:
  1. 生成请求签名: method + path
  2. 缓存查询 → 命中 → 返回缓存结果
  3. 遍历规则 (按 Priority 排序)
     └── Matcher.Match(req, rule) → 匹配 → 缓存 → 返回
  4. 无匹配 → 默认路由
```

### 13.2 反向代理 (proxy/proxy.go, 235行)

```
Proxy:
  ┌─────────────────────────────────────┐
  │ domainResolver: DomainResolver      │
  │ backends: map[siteID]*url.URL       │
  │ reverseProxies: map[siteID]*ReverseProxy│
  │                                     │
  │ transport: *http.Transport          │
  │   MaxIdleConns: 100                 │
  │   MaxIdleConnsPerHost: 20           │
  │   IdleConnTimeout: 90s              │
  │   TLSHandshakeTimeout: 10s          │
  └─────────────────────────────────────┘

请求处理:
  ServeHTTP(w, r)
    ├── 1. 域名解析: domainResolver.Resolve(r.Host) → siteID
    ├── 2. 获取后端: backends[siteID]
    ├── 3. 反向代理: reverseProxies[siteID].ServeHTTP(w, r)
    │   ├── 修改 Request: Scheme, Host
    │   ├── 删除 Hop-by-hop Headers
    │   └── 添加 X-Forwarded-For, X-Real-IP
    └── 4. 错误处理: 后端不可用 → 502 Bad Gateway
```

### 13.3 域名解析 (services/domain_resolver.go, 125行)

```
DomainResolver:
  Resolve(domain):
    1. 精确匹配: Redis GET domain:{domain}
    2. 通配符匹配:
       sub.example.com
         → *.example.com
         → *.com
    3. 返回 siteID

  AddMapping(domain, siteID):
    Redis SET domain:{domain} = siteID
    Redis SADD domains {domain}

  RemoveMapping(domain):
    Redis DEL domain:{domain}
    Redis SREM domains {domain}
```

---

## 十四、部署架构

### 14.1 部署方式

```
1. 一键脚本:
   curl -fsSL https://prerender.websitetool.cn/install.sh | bash
   自动检测:
     ├── Docker 可用 → docker compose up
     ├── Go + Node 可用 → 源码构建
     └── 纯净环境 → 下载预编译二进制

2. Docker Compose:
   services:
     redis:
       image: redis:7-alpine
       volumes: redis-data:/data
       healthcheck: redis-cli ping
     api:
       build: docker/Dockerfile
       depends_on: redis (condition: service_healthy)
       ports: 9597:9597, 9598:9598
       shm_size: 256m  (Chromium 需要共享内存)
       mem_limit: 4g
       security_opt: seccomp:unconfined (Chromium 需要)
     nginx (可选, profile: with-nginx):
       image: nginx:1.25-alpine
       ports: 80:80, 443:443

3. Kubernetes:
   deploy/k8s/deployment.yaml
     replicas: 1
     resources:
       requests: {cpu: 500m, memory: 1Gi}
       limits: {cpu: 2, memory: 4Gi}
   deploy/k8s/configmap.yaml
     config.yml 挂载为 ConfigMap

4. macOS (无 Launchd plist):
   nohup 后台启动 + PID 文件（install.sh）
```

### 14.2 端口规划

```
9597  → 管理控制台 (Web UI, 可公网)
9598  → API 服务 (REST, 建议内网)
6379  → Redis (内网)
9090  → Prometheus 指标 (可选)
80/443 → Nginx (可选, SSL 终止)
站点端口 → 动态分配 (如 8082, 50000+)
```

### 14.3 依赖注入容器 (di/container.go, 214行)

```
Container (14 个核心依赖):
  Config          → 全局配置
  Redis           → 数据存储
  UserManager     → 用户认证
  JWTManager      → JWT 令牌
  FirewallMgr     → WAF 引擎
  CacheMgr        → 缓存管理
  PrerenderMgr    → 渲染引擎
  CrawlerLogMgr   → 爬虫日志
  VisitLogMgr     → 访问日志
  GeoIPService    → 地理位置
  Scheduler       → 定时任务
  HealthChecker   → 健康检查
  Monitor         → 系统监控
  SiteServerMgr   → 站点服务器
  SiteHandler     → 站点处理器
  WafRepo         → WAF 数据仓库
  AuditLogger     → 审计日志

初始化顺序:
  1. Config (无依赖)
  2. Redis (依赖 Config)
  3. UserManager, JWTManager (依赖 Redis)
  4. FirewallMgr, CacheMgr (依赖 Redis)
  5. PrerenderMgr (依赖 Redis, CacheMgr)
  6. CrawlerLogMgr, VisitLogMgr (依赖 Redis)
  7. GeoIPService (无依赖)
  8. Scheduler (依赖 PrerenderMgr, Redis)
  9. HealthChecker, Monitor (依赖 Redis)
  10. SiteServerMgr, SiteHandler (依赖 PrerenderMgr, FirewallMgr, Redis)
  11. WafRepo (依赖 Redis)
  12. AuditLogger (依赖 Redis)
```

---

## 附录: 模块文件统计

| 模块 | 文件数 | 总行数 | 核心职责 |
|------|--------|--------|---------|
| firewall/ | 34 | ~3000 | WAF 引擎 + 15 种检测器 + 规则管理 |
| prerender/ | 23 | ~3500 | 渲染引擎 + 池 + 缓存 + 预热 + 推送 + 流式 + 增量 |
| monitoring/ | 12 | ~2000 | 监控 + 告警 + 遥测 + 仪表盘 |
| config/ | 12 | ~1200 | 配置管理 + 热重载 + 加密 + 验证 |
| redis/ | 8 | ~1400 | Redis 客户端 + 熔断器 + 订阅 |
| api/controllers/ | 21 | ~3500 | 11 个 Controller, 59 个端点 |
| seo/ | 5 | ~1336 | Meta 优化 + 结构化数据 + AEO |
| auth/ | 8 | ~900 | JWT + 2FA + 用户管理 |
| ssl/ | 9 | ~800 | ACME + 证书管理 + 自动续期 |
| logging/ | 8 | ~700 | 日志 + 审计 + 分析 |
| services/ | 5 | ~650 | GeoIP + 域名解析 + 日志处理 |
| middleware/ | 7 | ~500 | WAF 中间件 + 限流 + 错误处理 |
| routing/ | 2 | ~403 | 智能路由 + 内存缓存 |
| proxy/ | 3 | ~235 | 反向代理 + 连接池 |
| task/ | 2 | ~375 | 任务队列 + 优先级调度 |
| scheduler/ | 2 | ~315 | Cron 调度 + 站点监控 |
| crawler/ | 2 | ~195 | 爬虫检测 |
| audit/ | 2 | ~196 | 审计日志 |
| crypto/ | 4 | ~220 | AES-256-GCM 加密 |
| di/ | 3 | ~214 | 依赖注入容器 |
| 其他 12 个包 | ~20 | ~2000 | 工具/常量/模型/仓库/i18n/安全 |
| **总计** | **~180** | **~24000** | **36 个包, 96 项功能** |
