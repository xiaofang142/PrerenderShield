# Prerender Shield 业务流程文档

> 基于代码实际实现的完整业务流程，最后更新: 2026-06-13

---

## 一、请求处理主流程

```
                    ┌─────────────┐
                    │  客户端请求   │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  SSL/TLS    │  (如果启用HTTPS)
                    │  终止处理   │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  请求解析    │  提取: Method, Path, Headers, Body, IP
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ 爬虫识别     │  User-Agent匹配 + 行为分析
                    │ (crawler/)  │
                    └──┬──────┬──┘
                       │      │
              是爬虫   │      │  非爬虫
                       │      │
              ┌────────▼─┐  ┌─▼────────┐
              │ 渲染预热流 │  │ 安全检测流 │
              └──────────┘  └──────────┘
```

---

## 二、渲染预热流程 (爬虫请求)

```
爬虫请求到达
    │
    ▼
┌─────────────┐
│ 缓存检查     │  Redis: prerender:cache:{url_hash}
│ (cache/)    │
└──┬──────┬───┘
   │      │
 命中   未命中
   │      │
   │      ▼
   │  ┌──────────────┐
   │  │ 创建渲染任务   │  加入优先级队列
   │  │ (queue.go)   │
   │  └──────┬───────┘
   │         │
   │         ▼
   │  ┌──────────────┐
   │  │ 获取浏览器实例 │  从实例池获取 (pool/)
   │  │ (pool/)      │  MinPoolSize=2, MaxPoolSize=20
   │  └──────┬───────┘
   │         │
   │         ▼
   │  ┌──────────────┐
   │  │ Headless      │  chromedp渲染
   │  │ Chromium渲染  │  超时: 30s (可配置)
   │  │ (engine.go)  │
   │  └──────┬───────┘
   │         │
   │    ┌────▼────┐
   │    │ 渲染成功? │
   │    └─┬────┬──┘
   │      │    │
   │    成功  失败
   │      │    │
   │      │    ▼
   │      │  ┌──────────┐
   │      │  │ 重试机制   │  最多重试N次
   │      │  │ (engine)  │  重试间隔可配置
   │      │  └────┬─────┘
   │      │       │
   │      ▼       ▼
   │  ┌──────────────┐
   │  │ SEO注入       │  注入meta标签/结构化数据
   │  │ (seo/)       │  JSON-LD, OpenGraph
   │  └──────┬───────┘
   │         │
   │         ▼
   │  ┌──────────────┐
   │  │ 写入缓存      │  Redis + 内存双层缓存
   │  │ (cache/)     │  TTL: 3600s (可配置)
   │  └──────┬───────┘
   │         │
   └────┬────┘
        │
        ▼
  ┌──────────┐
  │ 返回HTML  │  Content-Type: text/html
  └──────────┘
```

---

## 三、安全检测流程 (普通请求)

```
普通请求到达
    │
    ▼
┌─────────────────┐
│ IP黑名单检查      │  匹配 → 403 Forbidden
│ (blacklist.go)  │
└────────┬────────┘
         │ 通过
         ▼
┌─────────────────┐
│ IP白名单检查      │  匹配 → 跳过后续检测
│ (blacklist.go)  │
└────────┬────────┘
         │ 通过
         ▼
┌─────────────────┐
│ 地理位置检查      │  GeoIP2查询 → 匹配封锁列表 → 403
│ (geoip.go)      │
└────────┬────────┘
         │ 通过
         ▼
┌─────────────────┐
│ 频率限制检查      │  超过阈值 → 429 Too Many Requests
│ (rate_limit.go) │  封禁时间: 3600s
└────────┬────────┘
         │ 通过
         ▼
┌─────────────────┐
│ OWASP Top 10    │  并行检测:
│ 威胁检测         │  - SQL注入 (injection.go)
│ (owasp_top10.go)│  - XSS (xss.go)
│                 │  - CSRF (csrf.go)
│                 │  - 命令注入 (injection.go)
│                 │  - 反序列化 (deserialization.go)
│                 │  - XXE (xxe.go)
│                 │  - 敏感数据 (sensitive_data.go)
└──┬──────────┬───┘
   │          │
 通过       拦截
   │          │
   │          ▼
   │    ┌──────────┐
   │    │ 执行动作   │  block/allow/challenge/log
   │    │ (action)  │  记录攻击日志
   │    └──────────┘
   │
   ▼
┌─────────────────┐
│ 网页防篡改检查   │  SHA256文件完整性校验
│ (file_integrity)│  检查间隔: 300s
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ AI智能检测       │  机器学习异常行为分析
│ (ai/)           │  (可选)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 转发到源站/      │  proxy模式 → 反向代理
│ 返回静态资源     │  static模式 → 文件服务
│                 │  redirect模式 → 重定向
└─────────────────┘
```

---

## 四、用户认证流程

### 4.1 首次运行
```
启动 → 检查Redis是否有用户 → 无用户 → first_run=true
     → 前端显示初始化页面 → 创建admin用户 → 设置密码
```

### 4.2 登录流程
```
POST /api/v1/auth/login {username, password}
    │
    ▼
AuthController.Login()
    │
    ▼
UserManager.AuthenticateUser()
    │
    ├── GetUserByUsername() → Redis: username:{name} → userID
    ├── GetUser(userID) → Redis: user:{id} → {id, username, password}
    ├── bcrypt.CompareHashAndPassword() → 密码验证
    │
    ▼
JWTManager.GenerateToken(userID, username)
    │
    ├── 生成UUID SessionID
    ├── 创建JWT Claims {user_id, username, session_id, exp, iat, iss, sub, jti}
    ├── HS256签名
    ├── 保存Session到Redis → session:{id} (TTL=24h)
    │
    ▼
返回 {token, username, force_change_password}
```

### 4.3 请求验证流程
```
请求 → Authorization: Bearer <token>
    │
    ▼
JWTAuthMiddleware()
    │
    ├── 提取token
    ├── jwt.ParseWithClaims() → 验证签名+过期
    ├── Redis CheckSessionExists() → 验证会话未撤销
    │
    ▼
设置 gin.Context: user_id, username, session_id
    │
    ▼
业务Handler处理
```

### 4.4 2FA流程
```
GET /api/v1/2fa/status → 返回 {available: true, enabled: false}
    │
POST /api/v1/2fa/enable → 生成TOTP密钥 → 返回密钥+QR码URL
    │
POST /api/v1/2fa/confirm {code} → 验证TOTP码 → 启用2FA
    │
POST /api/v1/2fa/disable {code} → 验证TOTP码 → 禁用2FA
```

---

## 五、站点管理流程

### 5.1 站点生命周期
```
创建站点 (POST /api/v1/sites)
    │
    ├── 验证: 域名(仅localhost/127.0.0.1), 端口(1-65535), 模式(proxy/static/redirect)
    ├── 保存到 ConfigManager → YAML文件 + Redis
    ├── SiteServerMgr.StartSiteServer() → 启动独立HTTP服务器
    ├── 记录审计日志
    │
    ▼
站点运行中
    │
    ├── 更新 (PUT /api/v1/sites/:id)
    │   ├── 停止旧站点服务器
    │   ├── 更新配置
    │   ├── 启动新站点服务器
    │   └── 同步到Redis
    │
    ├── 删除 (DELETE /api/v1/sites/:id)
    │   ├── 停止站点服务器
    │   ├── 删除配置
    │   └── 清理Redis数据
    │
    ▼
站点已删除
```

### 5.2 站点模式
| 模式 | 行为 | 配置 |
|------|------|------|
| `proxy` | 反向代理到后端服务 | `Proxy.TargetURL` |
| `static` | 提供静态文件服务 | `Dirs.StaticDir` |
| `redirect` | HTTP重定向 | `Redirect.StatusCode` + `Redirect.TargetURL` |

---

## 六、SSL证书管理流程

### 6.1 Let's Encrypt申请
```
POST /api/v1/ssl/certificates {domains: ["example.com"]}
    │
    ▼
SSLController.RequestCert()
    │
    ▼
ssl.Manager.RequestCertificate()
    │
    ├── 创建ACME客户端 (staging或production)
    ├── 生成RSA 2048位私钥
    ├── 创建CSR (证书签名请求)
    │
    ├── HTTP-01挑战:
    │   ├── 启动临时HTTP服务器 (Port 80)
    │   ├── 提供token验证文件
    │   └── Let's Encrypt验证域名所有权
    │
    ├── DNS-01挑战 (通配符证书):
    │   ├── 调用DNS服务商API (Cloudflare/Aliyun/TencentCloud)
    │   ├── 添加TXT记录
    │   └── Let's Encrypt验证
    │
    ▼
获取证书 → 保存到 certs/ 目录 + Redis
    │
    ▼
自动续期 (auto_renew.go)
    ├── 定时检查 (check_interval)
    ├── 到期前N天自动续签 (renew_before_days)
    ├── 失败重试 (max_retries, retry_delay)
    └── Webhook通知
```

---

## 七、缓存预热流程

### 7.1 手动预热
```
POST /api/v1/preheat/trigger {siteId: "site1"}
    │
    ▼
PreheatController.TriggerPreheat()
    │
    ▼
PrerenderEngine.CreatePreheatTask()
    │
    ├── 解析Sitemap: GET {sitemap_url} → 提取所有URL
    ├── 创建预热任务 → taskID
    ├── 加入渲染队列 (按优先级排序)
    ├── 并发渲染 (concurrency: 5)
    │   ├── 获取浏览器实例
    │   ├── 渲染页面
    │   ├── 写入缓存
    │   └── 更新任务进度
    │
    ▼
GET /api/v1/preheat/task/status → 实时进度
```

### 7.2 定时预热
```
Scheduler (robfig/cron)
    │
    ├── 解析Cron表达式: "0 0 * * *" (每天0点)
    ├── 触发预热任务
    └── 记录执行日志
```

---

## 八、搜索引擎推送流程

```
POST /api/v1/push/config {siteId, config}
    │
    ▼
获取站点URL列表
    │
    ├── 百度推送:
    │   ├── POST http://data.zz.baidu.com/urls?site={domain}&token={token}
    │   ├── Body: 每行一个URL
    │   ├── 日限额: 1000条
    │   └── 记录推送结果
    │
    ├── 必应推送:
    │   ├── POST https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl
    │   ├── Body: {siteUrl, urlList}
    │   ├── 日限额: 1000条
    │   └── 记录推送结果
    │
    ▼
GET /api/v1/push/stats → 推送统计
GET /api/v1/push/trend → 推送趋势 (30天)
```

---

## 九、监控告警流程

### 9.1 数据采集
```
Monitor (monitor.go)
    │
    ├── 系统指标 (gopsutil):
    │   ├── CPU使用率
    │   ├── 内存使用率
    │   ├── 磁盘使用率
    │   └── 网络流量
    │
    ├── 应用指标:
    │   ├── 活跃浏览器实例数
    │   ├── 缓存命中率
    │   ├── 请求数/秒
    │   ├── 拦截请求数
    │   └── 爬虫请求数
    │
    ├── Prometheus导出 (telemetry/metrics.go)
    └── OpenTelemetry追踪 (telemetry/tracer.go)
```

### 9.2 告警触发
```
告警规则引擎 (alerting/rules.go)
    │
    ├── 阈值检查:
    │   ├── CPU > 90% → Critical
    │   ├── 内存 > 85% → Warning
    │   ├── 磁盘 > 90% → Critical
    │   ├── 缓存命中率 < 50% → Warning
    │   └── 错误率 > 5% → Critical
    │
    ├── 通知渠道 (alerting/channels.go):
    │   ├── Webhook → POST到配置的URL
    │   ├── Email → SMTP发送
    │   ├── Slack → Webhook消息
    │   └── 钉钉 → Webhook消息
    │
    └── 告警历史 → Redis存储
```

---

## 十、配置管理流程

### 10.1 配置加载
```
启动 → LoadConfig()
    │
    ├── 环境变量覆盖
    ├── YAML文件加载 (config.yml)
    ├── Redis加载 (prerender:config:sites)
    ├── 配置验证 (validator.go)
    └── 启动配置监控 (watcher.go)
```

### 10.2 配置热重载
```
文件变化 (fsnotify)
    │
    ▼
ConfigManager.ReloadConfig()
    │
    ├── 重新加载YAML
    ├── 验证新配置
    ├── 对比差异
    ├── 通知所有ConfigChangeHandler
    │   ├── 站点服务器重启
    │   ├── 防火墙规则更新
    │   └── 缓存策略更新
    └── 记录审计日志
```

### 10.3 配置备份恢复
```
备份: POST /api/v1/system/backup
    → 序列化当前配置 → Redis: system:backup:{timestamp}

恢复: POST /api/v1/system/restore {backup_key}
    → 从Redis读取备份 → 反序列化 → 应用配置

列表: GET /api/v1/system/backups
    → 扫描Redis backup keys → 返回列表
```

---

## 十一、SEO优化流程

### 11.1 Meta标签优化
```
渲染完成 → HTML → MetaTagsOptimizer.OptimizeMetaTags()
    │
    ├── analyzeTitle() → 检查长度(30-60字符) + 关键词密度 + 品牌词
    ├── analyzeDescription() → 检查长度(120-160字符) + CTA用语
    ├── extractKeywords() → 词频统计 → Top 10关键词
    ├── detectMissingTags() → title/description/viewport/charset/canonical/og:title
    ├── generateMetaTags() → title/description/keywords/author/robots
    ├── generateOpenGraph() → og:title/og:description/og:type/og:locale/og:url
    ├── generateTwitterCard() → twitter:card/twitter:title/twitter:description
    └── BuildOptimizedHTML() → 注入优化后的标签到HTML
```

### 11.2 结构化数据注入
```
渲染完成 → HTML → StructuredDataOptimizer.OptimizeStructuredData()
    │
    ├── detectPageType() → 基于HTML特征自动识别:
    │   ├── <article> → Article
    │   ├── product/价格/库存 → Product
    │   ├── faq/常见问题 → FAQPage
    │   ├── breadcrumb/面包屑 → BreadcrumbList
    │   ├── address/电话/地址 → LocalBusiness
    │   └── about/关于我们 → Organization
    │
    ├── 生成对应Schema → JSON-LD格式
    ├── validateStructuredData() → 校验必需字段
    └── InjectStructuredData() → <script type="application/ld+json"> 注入到</head>前
```

### 11.3 AI爬虫优化 (AEO)
```
请求到达 → User-Agent检查 → IsAICrawler()
    │
    ├── 匹配8种AI爬虫: GPTBot/ClaudeBot/PerplexityBot/Google-Extended/Cohere-AI/FacebookBot/AppleBot/Bytespider
    ├── 识别用途: training(训练) / search(搜索)
    └── ExtractAnswer() → 移除script/style/nav/footer/header → 返回纯净内容
```

---

## 十二、日志处理流程

### 12.1 日志清洗
```
LogProcessor.Start() → 每5秒循环
    │
    ├── processCrawlerLogs()
    │   ├── GetUnwashedLogs(10) → 获取未清洗日志
    │   ├── 按IP分组
    │   ├── GeoIPService.GetLocation(ip) → 3个API轮询
    │   ├── 填充Country/City/Lat/Lng
    │   ├── UpdateLog() → 标记washed=true
    │   └── checkAndBan() → 匹配GeoIP封锁列表 → 自动封禁
    │
    └── processVisitLogs()
        ├── GetUnwashedLogs(10)
        ├── GeoIP富化
        └── 自动封禁违规IP
```

### 12.2 审计日志
```
所有管理操作 → audit.Logger.Log()
    │
    ├── 生成UUID
    ├── 记录: UserID, Action(14种), Resource, ClientIP, Severity
    ├── 存储到Redis: audit:{timestamp}
    └── TTL: 可配置
```

---

## 十三、域名解析流程

```
请求到达 → DomainResolver.Resolve(domain)
    │
    ├── 精确匹配: Redis GET domain:{domain} → siteID
    ├── 通配符匹配: *.example.com → siteID
    │   └── 逐级向上匹配: sub.domain.com → *.domain.com → *.com
    └── 返回siteID → 路由到对应站点服务器
```

---

## 十四、反向代理流程

```
Proxy模式站点请求
    │
    ├── DomainResolver.Resolve(domain) → siteID
    ├── GetBackend(siteID) → 从Redis/内存获取后端URL
    ├── httputil.ReverseProxy.ServeHTTP()
    │   ├── 修改Request: Scheme/Host
    │   ├── HTTP连接池: 100连接, 20/主机
    │   └── 转发到后端
    └── 返回响应
```

---

## 十五、依赖注入流程

```
启动 → di.NewContainer()
    │
    ├── 加载Config
    ├── 连接Redis
    ├── 创建UserManager + JWTManager
    ├── 创建FirewallEngineManager
    ├── 创建CacheManager
    ├── 创建PrerenderEngineManager
    ├── 创建CrawlerLogManager + VisitLogManager
    ├── 创建GeoIPService
    ├── 创建Scheduler
    ├── 创建HealthChecker + Monitor
    ├── 创建SiteServerManager + SiteHandler
    ├── 创建WafRepository
    └── 创建AuditLogger
        │
        ▼
    bootstrap.Run() → 启动所有服务
```


---

# 附录：数据存储与缓存设计

> 以下内容合并自 data-flow.md（数据结构层面细节）

## 缓存键设计


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


## 配置来源与优先级


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


## 数据存储清单


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
