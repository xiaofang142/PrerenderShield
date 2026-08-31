# Prerender Shield 深度技术问答

> 基于源码完整分析的 8 个核心技术问题解答
> 更新日期: 2026-06-13

---

## Q1: SSL 技术栈、服务商、自动续期、免费证书实现

### 技术栈

```
┌─────────────────────────────────────────────────────────┐
│                    SSL 技术栈                             │
├─────────────────────────────────────────────────────────┤
│ ACME 客户端:  github.com/go-acme/lego/v4 (v4.32.0)      │
│   选型理由: Go 原生实现, 支持 HTTP-01/DNS-01/TLS-ALPN-01 │
│   三种挑战类型, 内置 100+ DNS 服务商支持                   │
├─────────────────────────────────────────────────────────┤
│ 证书密钥:    ECDSA P-256 (账户密钥)                       │
│             RSA 2048 (证书密钥, certcrypto.RSA2048)       │
├─────────────────────────────────────────────────────────┤
│ 存储:       文件系统 (certs/{domain}.crt/.key/.issuer.crt)│
│             Redis (ssl:cert:{domain} → JSON)             │
├─────────────────────────────────────────────────────────┤
│ 自动续期:   AutoRenewer (auto_renew.go, 205行)           │
│             Manager.StartAutoRenewal() (manager.go)       │
└─────────────────────────────────────────────────────────┘
```

### 服务商

**Let's Encrypt** (唯一服务商):

```
生产环境: https://acme-v02.api.letsencrypt.org/directory
测试环境: https://acme-staging-v02.api.letsencrypt.org/directory

配置方式:
  ssl:
    enabled: true
    email: "admin@example.com"       # ACME 账户邮箱
    production: true                 # true=生产, false=staging
    auto_renew: true
    check_interval: 24h              # 检查间隔
    renew_before_days: 30            # 提前30天续签
    max_retries: 3                   # 失败重试次数
    retry_delay: 1h                  # 重试间隔
    webhook_url: ""                  # 续签通知 Webhook
    http_port: 80                    # HTTP-01 挑战端口
```

### 免费证书实现流程

```
POST /api/v1/ssl/certificates {domains: ["example.com"]}
    │
    ▼
SSLController.RequestCert()
    │
    ▼
ACMEClient.RequestCertificate(domains)  (acme_client.go:129)
    │
    ├── 1. 创建 LEGO 客户端
    │   ├── lego.NewClient(lego.Config{
    │   │     CADirURL:  "https://acme-v02.api.letsencrypt.org/directory",
    │   │     UserAgent: "PrerenderShield/1.0",
    │   │   })
    │   └── Certificate.KeyType = certcrypto.RSA2048
    │
    ├── 2. 注册 ACME 账户
    │   ├── ecdsa.GenerateKey(elliptic.P256(), rand.Reader)  → 账户密钥
    │   └── client.Registration.Register(RegisterOptions{
    │         TermsOfServiceAgreed: true,
    │       })
    │
    ├── 3. 设置 HTTP-01 挑战提供者
    │   ├── NewHTTPProvider(httpPort)  → 启动临时 HTTP 服务器
    │   └── client.Challenge.SetHTTP01Provider(httpProvider)
    │
    ├── 4. 申请证书
    │   └── client.Certificate.Obtain(certificate.ObtainRequest{
    │         Domains: ["example.com", "www.example.com"],
    │         Bundle:  true,  // 包含完整证书链
    │       })
    │       │
    │       ├── LEGO 自动完成 HTTP-01 挑战:
    │       │   ├── Let's Encrypt 返回 challenge token
    │       │   ├── HTTPProvider 提供: /.well-known/acme-challenge/{token}
    │       │   └── Let's Encrypt 验证 → 颁发证书
    │       │
    │       └── 返回 certificate.Resource{
    │             Domain:            "example.com",
    │             Certificate:       []byte,  // 服务器证书 (PEM)
    │             PrivateKey:        []byte,  // 私钥 (PEM)
    │             IssuerCertificate: []byte,  // 中间证书 (PEM)
    │           }
    │
    └── 5. 保存证书
        ├── certs/example.com.crt        (0644)
        ├── certs/example.com.key        (0600)
        ├── certs/example.com.issuer.crt (0644)
        └── Redis: ssl:cert:example.com → JSON
```

### 自动续期实现

```
AutoRenewer (auto_renew.go):

┌─────────────────────────────────────────────────────────┐
│  Start()                                                │
│    └── go run()                                         │
│        └── time.Ticker(check_interval)                  │
│            └── checkAndRenew()                          │
│                ├── acmeClient.ListCertificates()        │
│                │   └── 扫描 certs/ 目录所有 .crt 文件    │
│                │                                        │
│                └── 遍历每个证书:                          │
│                    ├── GetCertificateInfo(domain)        │
│                    │   └── 解析 x509 证书 → expires_in   │
│                    │                                    │
│                    ├── expires_in <= renew_before_days   │
│                    │   └── renewCertificate(domain)      │
│                    │       ├── 读取现有证书+私钥          │
│                    │       ├── client.Certificate.Renew()│
│                    │       ├── 保存新证书                 │
│                    │       ├── saveRenewalRecord()       │
│                    │       └── sendWebhook()             │
│                    │                                    │
│                    └── 失败重试: max_retries 次           │
│                        └── 每次间隔 retry_delay          │
└─────────────────────────────────────────────────────────┘

Manager 内置续期 (manager.go:150):
  StartAutoRenewal(interval)
    └── time.Ticker(interval)  // 默认 24h
        └── checkAndRenewCertificates()
            ├── Redis SMEMBERS ssl:certs  → 所有域名
            ├── 检查 expires_at - now < 30天
            └── RenewCertificate(domain)
```

**双重续期保障**：`AutoRenewer` (基于 LEGO) + `Manager.StartAutoRenewal()` (基于 Redis)，两者独立运行，确保续期不遗漏。

---

## Q2: 开源威胁情报 IP 黑名单订阅

### 当前实现

```
BlacklistDetector (firewall/detectors/blacklist.go, 84行):

三层黑名单架构:
  ┌─────────────────────────────────────────────────────┐
  │ 1. 静态白名单 (配置文件 whitelist: [])               │
  │    → 匹配 → 直接放行, 跳过所有后续检测               │
  ├─────────────────────────────────────────────────────┤
  │ 2. 静态黑名单 (配置文件 blacklist: [])               │
  │    → 匹配 → 403 Forbidden                           │
  ├─────────────────────────────────────────────────────┤
  │ 3. 动态黑名单 (Redis Set)                            │
  │    Key: firewall:{siteID}:blacklist                  │
  │    → SISMEMBER → 匹配 → 403                         │
  └─────────────────────────────────────────────────────┘

动态黑名单来源:
  1. 管理界面手动添加: POST /api/v1/firewall/blacklist {site_id, ip}
  2. LogProcessor 自动封禁: GeoIP 封锁列表匹配 → 自动 SADD
  3. RateLimitDetector 超限封禁: 超过频率限制 → 自动封禁
```

### 威胁情报订阅设计 (建议增强)

当前系统**缺少**外部威胁情报订阅能力。以下是增强方案：

```
威胁情报订阅模块设计:

┌─────────────────────────────────────────────────────────┐
│              ThreatIntelFetcher                          │
├─────────────────────────────────────────────────────────┤
│ 配置:                                                    │
│   sources:                                               │
│     - url: "https://feodotracker.abuse.ch/.../ipblocklist.csv"
│       format: csv                                        │
│       update_interval: 1h                                │
│     - url: "https://lists.blocklist.de/lists/all.txt"    │
│       format: text                                       │
│       update_interval: 6h                                │
│     - url: "https://rules.emergingthreats.net/.../compromised-ips.txt"
│       format: text                                       │
│       update_interval: 12h                               │
├─────────────────────────────────────────────────────────┤
│ 免费威胁情报源:                                           │
│   1. Abuse.ch Feodo Tracker (C2 服务器 IP)               │
│      https://feodotracker.abuse.ch/downloads/ipblocklist.csv
│   2. Blocklist.de (攻击 IP 汇总)                          │
│      https://lists.blocklist.de/lists/all.txt            │
│   3. Emerging Threats (被入侵 IP)                         │
│      https://rules.emergingthreats.net/fwrules/emerging-Block-IPs.txt
│   4. Spamhaus DROP (垃圾邮件/恶意软件)                    │
│      https://www.spamhaus.org/drop/drop.txt              │
│   5. AlienVault OTX (开源威胁交换)                        │
│      https://otx.alienvault.com/api/v1/indicators/IPv4/{ip}/general
├─────────────────────────────────────────────────────────┤
│ 工作流程:                                                │
│   1. 定时从各源拉取 IP 列表                               │
│   2. 解析 (CSV/TXT/JSON) → 提取 IP                       │
│   3. 去重合并                                             │
│   4. 批量 SADD firewall:{siteID}:threat_intel            │
│   5. BlacklistDetector 增加第4层检查:                     │
│      SISMEMBER firewall:{siteID}:threat_intel {ip}       │
└─────────────────────────────────────────────────────────┘
```

### 当前 RuleManager 远程规则加载 (已有基础)

```
RuleManager (firewall/engine.go:80-199):

远程规则源加载:
  fetchRulesFromRemote():
    HTTP GET {remoteRuleSource}
    → JSON 反序列化 → map[string][]types.Rule
    → 保存到 Redis (prerender:firewall:rules)
    → 7 天过期

自动更新:
  startAutoUpdate():
    time.Ticker(updateInterval)
    → ReloadRules()
    → 重新走优先级链: 远程 → Redis → 文件 → 默认

这个机制可以直接复用于威胁情报 IP 订阅:
  远程源 URL → HTTP GET → 解析 IP 列表 → SADD Redis
```

---

## Q3: GeoIP 免费 API、地理位置访问控制、速率限制

### GeoIP 双轨实现

系统同时使用**两种** GeoIP 方案：

```
方案1: 本地数据库 (GeoIPDetector - firewall/detectors/geoip.go)
  ┌─────────────────────────────────────────────────────┐
  │ 依赖: github.com/oschwald/geoip2-golang (v1.9.0)    │
  │ 数据库: GeoLite2-Country.mmdb (MaxMind 免费版)       │
  │ 路径: ./rules/GeoLite2-Country.mmdb                  │
  │                                                     │
  │ 优点: 零网络延迟, 无 API 限制                        │
  │ 缺点: 需定期更新数据库文件                            │
  │                                                     │
  │ 使用: WAF 中间件实时检测 (每个请求都查)               │
  └─────────────────────────────────────────────────────┘

方案2: 在线 API (GeoIPService - services/geoip.go, 370行)
  ┌─────────────────────────────────────────────────────┐
  │ 3 个免费 API 轮询:                                   │
  │   1. ip-api.com    (45 req/min 免费)                 │
  │   2. ipapi.co      (1000 req/day 免费)               │
  │   3. get.geojs.io  (无限制)                          │
  │                                                     │
  │ 优点: 无需维护数据库, 数据更新及时                    │
  │ 缺点: 网络延迟 100-500ms, 有频率限制                 │
  │                                                     │
  │ 使用: 日志处理 (异步, 不阻塞请求)                     │
  └─────────────────────────────────────────────────────┘
```

### 地理位置访问控制

```
GeoIPDetector.Detect() 检测流程:

  ┌─────────────────────────────────────────────────────┐
  │ 1. 获取客户端真实 IP                                  │
  │    X-Forwarded-For → X-Real-IP → RemoteAddr          │
  ├─────────────────────────────────────────────────────┤
  │ 2. 查询国家代码                                       │
  │    本地数据库: geoip2.Reader.Country(ip) → ISO Code   │
  │    回退模式: 127.0.0.1 → "CN", 其他 → "US"           │
  ├─────────────────────────────────────────────────────┤
  │ 3. BlockList 检查 (黑名单模式)                        │
  │    if countryCode in BlockList:                      │
  │      → 403 "Request from blocked country"            │
  ├─────────────────────────────────────────────────────┤
  │ 4. AllowList 检查 (白名单模式)                        │
  │    if AllowList not empty AND countryCode not in it: │
  │      → 403 "Request from country not in allow list"  │
  └─────────────────────────────────────────────────────┘

配置示例:
  firewall:
    enabled: true
    geoip:
      enabled: true
      block_list: ["RU", "KP", "IR"]    # 封锁的国家
      allow_list: []                     # 仅允许的国家 (空=不限制)
```

### 速率限制 (双重实现)

```
实现1: 内存限流 (RateLimitDetector - detectors/rate_limit.go, 187行)
  ┌─────────────────────────────────────────────────────┐
  │ 数据结构: map[ip]*IPCounter                          │
  │   IPCounter:                                        │
  │     Requests:    []time.Time  (滑动窗口)             │
  │     BannedUntil: time.Time    (封禁截止时间)          │
  │                                                     │
  │ 算法: 滑动窗口计数                                    │
  │   1. 追加当前请求时间                                 │
  │   2. 移除 window 之前的过期请求                        │
  │   3. len(validRequests) > maxRequests → 超限          │
  │   4. 超限 → banIP(ip, banTime)                       │
  │                                                     │
  │ 清理: 每 5 分钟清理无请求且未封禁的 IP 记录            │
  │                                                     │
  │ 优点: 零网络开销, 微秒级延迟                          │
  │ 缺点: 单机, 重启丢失, 不适合多实例                    │
  └─────────────────────────────────────────────────────┘

实现2: Redis 分布式限流 (RedisRateLimiter - middleware/ratelimit.go, 78行)
  ┌─────────────────────────────────────────────────────┐
  │ Key 设计:                                            │
  │   ratelimit:count:{ip}   → 计数器 (TTL=window)       │
  │   ratelimit:window:{ip}  → 窗口起始时间               │
  │   ratelimit:ban:{ip}     → 封禁标记 (TTL=banTime)    │
  │                                                     │
  │ 算法: 固定窗口计数                                    │
  │   1. 检查 ban key → 存在 → 429                       │
  │   2. 检查 window key → 过期 → 新窗口, count=1         │
  │   3. 窗口内 → INCR count → >= limit → 封禁 + 429     │
  │                                                     │
  │ 优点: 多实例共享, 持久化                              │
  │ 缺点: 网络延迟 ~1ms                                   │
  └─────────────────────────────────────────────────────┘

配置示例:
  firewall:
    rate_limit:
      enabled: true
      requests: 100     # 窗口内最大请求数
      window: 60        # 时间窗口(秒)
      ban_time: 3600    # 封禁时间(秒)
```

### CC 防护 (Q8 前置)

当前限流基于 **IP 维度**。CC 攻击防护需要**自定义参数维度**的限流。

```
CC 防护增强设计 (基于现有 RateLimitDetector 扩展):

自定义 CC 防护参数:
  cc_protection:
    enabled: true
    rules:
      - name: "登录接口保护"
        path: "/api/login"
        method: "POST"
        params: ["username"]          # 按 username 参数限流
        requests: 5
        window: 60
        ban_time: 1800
      
      - name: "搜索接口保护"
        path: "/api/search"
        params: ["q"]                 # 按搜索词限流
        requests: 30
        window: 60
      
      - name: "API 全局保护"
        path: "/api/*"
        headers: ["Authorization"]    # 按 Token 限流
        requests: 1000
        window: 60

实现原理:
  限流 Key = hash(path + method + param_values + header_values)
  而非仅 IP, 从而精确控制每个参数组合的请求频率
```

---

## Q4: SEO 数据生成 + 预渲染阶段注入

### SEO 数据生成流程

```
┌─────────────────────────────────────────────────────────┐
│              SEO 数据生成全流程                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  输入: 渲染后的 HTML + 目标 URL                           │
│                                                         │
│  Step 1: Meta 标签分析 (MetaTagsOptimizer)               │
│  ┌───────────────────────────────────────────────────┐  │
│  │ analyzeTitle(html)                                │  │
│  │   ├── 正则提取 <title>...</title>                  │  │
│  │   ├── 长度检查: 30-60 字符                         │  │
│  │   ├── 关键词密度: 目标关键词在标题中的占比           │  │
│  │   └── 品牌词检测: 标题是否以 "| Brand" 结尾         │  │
│  │                                                   │  │
│  │ analyzeDescription(html)                          │  │
│  │   ├── 正则提取 <meta name="description">           │  │
│  │   ├── 长度检查: 120-160 字符                       │  │
│  │   ├── CTA 检测: 是否包含号召性用语                  │  │
│  │   └── 缺失 → 从 <p> 或 <h1> 自动生成               │  │
│  │                                                   │  │
│  │ extractKeywords(html)                             │  │
│  │   ├── 移除 HTML 标签                               │  │
│  │   ├── 分词 (按非字母数字字符)                       │  │
│  │   ├── 词频统计 → 排序                              │  │
│  │   └── 取 Top 10                                    │  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  Step 2: 结构化数据生成 (StructuredDataOptimizer)         │
│  ┌───────────────────────────────────────────────────┐  │
│  │ detectPageType(html)                              │  │
│  │   ├── <article> → Article                         │  │
│  │   ├── product/价格/¥/$ → Product                   │  │
│  │   ├── faq/常见问题 → FAQPage                       │  │
│  │   ├── breadcrumb → BreadcrumbList                  │  │
│  │   ├── address/电话 → LocalBusiness                 │  │
│  │   └── about/关于 → Organization                    │  │
│  │                                                   │  │
│  │ generate{Type}Schema(data)                        │  │
│  │   └── 生成对应 Schema.org JSON-LD                   │  │
│  │                                                   │  │
│  │ validateStructuredData(schema)                    │  │
│  │   ├── 检查 @context, @type 必需字段                │  │
│  │   └── 按类型检查特定字段                            │  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  Step 3: 标签生成                                        │
│  ┌───────────────────────────────────────────────────┐  │
│  │ generateMetaTags()                                │  │
│  │   → title, description, keywords, author, robots   │  │
│  │                                                   │  │
│  │ generateOpenGraph()                               │  │
│  │   → og:title, og:description, og:type, og:url     │  │
│  │                                                   │  │
│  │ generateTwitterCard()                             │  │
│  │   → twitter:card, twitter:title, twitter:description│  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  输出: MetaTagsResult + StructuredDataResult             │
└─────────────────────────────────────────────────────────┘
```

### 预渲染阶段注入

```
SEOInjector.InjectSEOTags(html, pageURL)  (seo_injector.go, 64行)

注入时机: 渲染完成后、缓存写入前

注入流程:
  ┌─────────────────────────────────────────────────────┐
  │ 1. MetaTagsOptimizer.OptimizeMetaTags(html, nil)    │
  │    → MetaTagsResult                                 │
  │                                                     │
  │ 2. MetaTagsOptimizer.BuildOptimizedHTML(html, result)│
  │    ├── 替换/插入 <title>                             │
  │    ├── 替换/插入 <meta name="description">           │
  │    ├── 替换/插入 <meta name="keywords">              │
  │    ├── 替换/插入 <link rel="canonical">              │
  │    ├── 替换/插入 OpenGraph 标签                       │
  │    └── 替换/插入 TwitterCard 标签                     │
  │                                                     │
  │ 3. StructuredDataOptimizer.BuildStructuredDataHTML() │
  │    → <script type="application/ld+json">            │
  │    → 插入到 </head> 之前                             │
  │                                                     │
  │ 4. Canonical URL 注入                                │
  │    → <link rel="canonical" href="{pageURL}">        │
  └─────────────────────────────────────────────────────┘

完整渲染+SEO流程:
  Render(url)
    ├── chromedp 渲染 → HTML
    ├── SEOInjector.InjectSEOTags(html, url)  ← SEO 注入
    ├── 写入缓存 (含 SEO 标签的完整 HTML)
    └── 返回 HTML
```

---

## Q5: Sitemap 生成 + 推送给搜索引擎

### Sitemap 生成

当前系统**不自动生成** Sitemap，而是**解析已有** Sitemap：

```
PreheatWorker (prerender/preheat.go, 345行):

预热流程:
  ┌─────────────────────────────────────────────────────┐
  │ 1. 配置 Sitemap URL                                  │
  │    prerender:                                        │
  │      preheat:                                        │
  │        sitemap_url: "https://example.com/sitemap.xml"│
  │                                                     │
  │ 2. HTTP GET Sitemap URL                              │
  │    → XML 解析                                        │
  │    → 提取所有 <url><loc>...</loc></url>              │
  │                                                     │
  │ 3. 批量预热                                          │
  │    → 并发渲染每个 URL                                 │
  │    → 写入缓存                                        │
  └─────────────────────────────────────────────────────┘
```

### 搜索引擎推送

```
PushManager (prerender/push/manager.go, 536行):

推送流程:
  ┌─────────────────────────────────────────────────────┐
  │ 1. 获取站点 URL 列表                                 │
  │    Redis: site:{id}:urls → SMEMBERS                  │
  │                                                     │
  │ 2. 百度推送                                          │
  │    POST http://data.zz.baidu.com/urls                │
  │      ?site={domain}                                  │
  │      &token={baidu_token}                            │
  │    Content-Type: text/plain                          │
  │    Body: 每行一个 URL                                 │
  │    限额: baidu_daily_limit (默认 1000/天)             │
  │                                                     │
  │ 3. 必应推送                                          │
  │    POST https://ssl.bing.com/webmaster/api.svc/json/ │
  │         SubmitUrl?apikey={bing_token}                 │
  │    Content-Type: application/json                    │
  │    Body: {"siteUrl": "...", "urlList": ["..."]}      │
  │    限额: bing_daily_limit (默认 1000/天)              │
  │                                                     │
  │ 4. 日限额控制                                        │
  │    Redis: push:{site}:daily:{YYYY-MM-DD}             │
  │    INCR → 超过限额 → 跳过                             │
  │                                                     │
  │ 5. 推送日志                                          │
  │    Redis: push:{site}:logs (List)                    │
  │    记录: URL, 搜索引擎, 状态, 时间                    │
  └─────────────────────────────────────────────────────┘

配置示例:
  prerender:
    push:
      enabled: true
      baidu_api: "http://data.zz.baidu.com/urls"
      baidu_token: "your_baidu_token"
      bing_api: "https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl"
      bing_token: "your_bing_api_key"
      baidu_daily_limit: 1000
      bing_daily_limit: 1000
      push_domain: "example.com"
```

---

## Q6: LLM SEO 优化配置

### 当前状态

系统**已集成 LLM SEO 优化**（`seo/llm_optimizer.go`, 310行），支持 OpenAI、智谱 GLM、DeepSeek、Ollama 四种 LLM 提供商，可优化标题、描述、关键词和结构化数据。LLM 不可用时自动回退到规则引擎（`MetaTagsOptimizer`）。

### LLM 增强方案设计

```
LLM SEO 优化模块设计:

┌─────────────────────────────────────────────────────────┐
│              LLMSEOOptimizer                             │
├─────────────────────────────────────────────────────────┤
│ 配置:                                                    │
│   llm:                                                   │
│     enabled: false                                       │
│     provider: "openai"    # openai / zhipu / local       │
│     api_key: "${LLM_API_KEY}"                            │
│     api_url: "https://api.openai.com/v1/chat/completions"│
│     model: "gpt-4o-mini"                                 │
│     max_tokens: 500                                      │
│     temperature: 0.3                                     │
│                                                          │
│     prompts:                                              │
│       title_optimization: |                              │
│         You are an SEO expert. Optimize the following    │
│         page title for search engines. Requirements:     │
│         - 30-60 characters                               │
│         - Include primary keyword near the beginning     │
│         - Include brand name at the end                  │
│         Page title: {title}                              │
│         Target keywords: {keywords}                      │
│                                                          │
│       description_optimization: |                        │
│         Generate a compelling meta description.          │
│         Requirements:                                    │
│         - 120-160 characters                             │
│         - Include call-to-action                         │
│         - Include primary keywords naturally             │
│         Page content summary: {summary}                  │
│                                                          │
│       keyword_extraction: |                              │
│         Extract the top 10 most relevant SEO keywords    │
│         from the following content. Return as JSON array.│
│         Content: {content}                               │
│                                                          │
│       structured_data: |                                 │
│         Generate Schema.org JSON-LD for this page.       │
│         Page type: {type}                                │
│         Page content: {content}                          │
│         Return valid JSON-LD only.                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│ 工作流程:                                                │
│   1. 渲染完成 → HTML                                     │
│   2. 提取页面文本 (strip HTML tags)                      │
│   3. 调用 LLM API:                                       │
│      ├── POST {api_url}                                  │
│      ├── Header: Authorization: Bearer {api_key}         │
│      └── Body: {model, messages, max_tokens, temperature}│
│   4. 解析 LLM 响应:                                       │
│      ├── 优化后的标题                                     │
│      ├── 优化后的描述                                     │
│      ├── 提取的关键词                                     │
│      └── 生成的结构化数据                                 │
│   5. 注入 HTML (同 Q4 流程)                               │
│                                                          │
│ 降级策略:                                                │
│   LLM 不可用 → 回退到规则引擎 (MetaTagsOptimizer)         │
│   LLM 超时 (>5s) → 回退到规则引擎                        │
└─────────────────────────────────────────────────────────┘
```

### 支持的 LLM 提供商

| 提供商 | API URL | 模型 | 费用 |
|--------|---------|------|------|
| OpenAI | `https://api.openai.com/v1/chat/completions` | gpt-4o-mini | ~$0.15/1M tokens |
| 智谱 GLM | `https://open.bigmodel.cn/api/paas/v4/chat/completions` | glm-4-flash | 免费额度 |
| DeepSeek | `https://api.deepseek.com/v1/chat/completions` | deepseek-chat | ~$0.14/1M tokens |
| 本地 Ollama | `http://localhost:11434/api/generate` | llama3 | 免费 |

---

## Q7: 站点模式 — 静态站点 vs 代理服务器

### 站点模式架构

```
SiteConfig.Mode 支持三种模式:

┌─────────────────────────────────────────────────────────┐
│  1. static (静态站点)                                    │
│     ┌─────────────────────────────────────────────┐     │
│     │ Client → WAF → CrawlerDetect                │     │
│     │   ├── 爬虫 → PrerenderEngine → 渲染 → 缓存   │     │
│     │   └── 普通 → http.FileServer(staticDir)     │     │
│     │                                             │     │
│     │ 适用: 纯静态网站, HTML/CSS/JS 文件服务        │     │
│     │ 功能: ✅ WAF ✅ GeoIP ✅ RateLimit ✅ SEO     │     │
│     │       ✅ SSL ✅ 预渲染 ✅ 缓存                 │     │
│     └─────────────────────────────────────────────┘     │
│                                                         │
│  2. proxy (代理服务器)                                    │
│     ┌─────────────────────────────────────────────┐     │
│     │ Client → WAF → CrawlerDetect                │     │
│     │   ├── 爬虫 → PrerenderEngine → 渲染 → 缓存   │     │
│     │   └── 普通 → ReverseProxy → 上游服务器       │     │
│     │                                             │     │
│     │ 适用: 已有后端服务, 需要在前加 WAF + SEO     │     │
│     │ 功能: ✅ WAF ✅ GeoIP ✅ RateLimit ✅ SEO     │     │
│     │       ✅ SSL ✅ 预渲染 ✅ 缓存 ✅ 反向代理     │     │
│     │                                             │     │
│     │ 代理配置:                                     │     │
│     │   proxy:                                     │     │
│     │     target_url: "http://localhost:3000"      │     │
│     │                                             │     │
│     │ 连接池:                                       │     │
│     │   MaxIdleConns: 100                          │     │
│     │   MaxIdleConnsPerHost: 20                    │     │
│     │   IdleConnTimeout: 90s                       │     │
│     └─────────────────────────────────────────────┘     │
│                                                         │
│  3. redirect (重定向)                                    │
│     ┌─────────────────────────────────────────────┐     │
│     │ Client → 301/302 → TargetURL                 │     │
│     │                                             │     │
│     │ 适用: 域名迁移, HTTP→HTTPS 强制跳转           │     │
│     │ 配置:                                         │     │
│     │   redirect:                                   │     │
│     │     status_code: 301                          │     │
│     │     target_url: "https://newdomain.com"       │     │
│     └─────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

### 完整配置示例

```yaml
sites:
  # 静态站点: 完整功能
  - id: "my-blog"
    name: "我的博客"
    domains: ["blog.example.com"]
    port: 8082
    mode: "static"
    firewall:
      enabled: true
      geoip:
        enabled: true
        block_list: ["RU", "KP"]
      rate_limit:
        enabled: true
        requests: 100
        window: 60
    prerender:
      enabled: true
      pool_size: 5
      cache_ttl: 3600
      preheat:
        enabled: true
        sitemap_url: "https://blog.example.com/sitemap.xml"
        schedule: "0 2 * * *"
      push:
        enabled: true
        baidu_token: "xxx"
        push_domain: "blog.example.com"

  # 代理模式: WAF + SEO 前置
  - id: "my-app"
    name: "我的应用"
    domains: ["app.example.com"]
    port: 8083
    mode: "proxy"
    proxy:
      target_url: "http://localhost:3000"  # 上游 Next.js 应用
    firewall:
      enabled: true
      geoip:
        enabled: true
        allow_list: ["CN", "US", "JP"]  # 仅允许这三个国家
      rate_limit:
        enabled: true
        requests: 200
        window: 60
    prerender:
      enabled: true
      pool_size: 3
      cache_ttl: 7200
```

### 请求处理对比

```
Static 模式请求流:
  GET /index.html
    → WAF (黑名单/GeoIP/限流/OWASP)
    → CrawlerDetect
    → 爬虫? → PrerenderEngine.RenderWithContext()
    → 普通? → FileServer.ServeHTTP()
    → 返回静态文件

Proxy 模式请求流:
  GET /api/users
    → WAF (黑名单/GeoIP/限流/OWASP)
    → CrawlerDetect
    → 爬虫? → PrerenderEngine.RenderWithContext()
    → 普通? → ReverseProxy.ServeHTTP()
      → 修改 Request: Scheme, Host
      → 添加 X-Forwarded-For, X-Real-IP
      → 转发到 target_url
    → 返回上游响应
```

---

## Q8: CC 防护 — 自定义参数 CC 防护

### 当前实现

```
现有限流器 (两种):

1. RateLimitDetector (内存, 按 IP):
   - 维度: 仅 IP
   - 算法: 滑动窗口
   - 存储: 内存 map

2. RedisRateLimiter (分布式, 按 IP):
   - 维度: 仅 IP
   - 算法: 固定窗口
   - 存储: Redis
```

### 自定义参数 CC 防护增强

```
CCProtectionDetector 设计:

┌─────────────────────────────────────────────────────────┐
│  CCProtectionConfig                                     │
├─────────────────────────────────────────────────────────┤
│  enabled: true                                          │
│  rules:                                                 │
│    - name: "登录接口 CC 防护"                            │
│      path: "/api/login"          # 匹配路径 (支持通配符) │
│      method: "POST"              # 匹配方法              │
│      dimension: "ip"             # 限流维度              │
│      requests: 5                 # 窗口内最大请求数       │
│      window: 60                  # 时间窗口(秒)          │
│      ban_time: 1800              # 封禁时间(秒)          │
│                                                         │
│    - name: "API 按 Token 限流"                           │
│      path: "/api/*"                                      │
│      dimension: "header:Authorization"  # 按请求头限流   │
│      requests: 1000                                     │
│      window: 60                                         │
│                                                         │
│    - name: "搜索接口按参数限流"                           │
│      path: "/api/search"                                 │
│      dimension: "param:q"           # 按 URL 参数限流    │
│      requests: 30                                        │
│      window: 60                                          │
│                                                         │
│    - name: "组合维度限流"                                 │
│      path: "/api/order"                                  │
│      dimension: "ip+param:user_id"  # IP + 参数组合      │
│      requests: 10                                        │
│      window: 60                                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ 限流 Key 生成算法:                                       │
│   func buildCCKey(req, rule) string {                    │
│     parts := []string{}                                  │
│     for _, dim := range rule.Dimensions {                │
│       switch {                                           │
│       case dim == "ip":                                  │
│         parts = append(parts, getClientIP(req))          │
│       case strings.HasPrefix(dim, "header:"):            │
│         h := strings.TrimPrefix(dim, "header:")          │
│         parts = append(parts, req.Header.Get(h))         │
│       case strings.HasPrefix(dim, "param:"):             │
│         p := strings.TrimPrefix(dim, "param:")           │
│         parts = append(parts, req.URL.Query().Get(p))    │
│       case strings.HasPrefix(dim, "cookie:"):            │
│         c := strings.TrimPrefix(dim, "cookie:")          │
│         cookie, _ := req.Cookie(c)                       │
│         parts = append(parts, cookie.Value)              │
│       }                                                  │
│     }                                                    │
│     key := fmt.Sprintf("cc:%s:%s", rule.Name,            │
│              strings.Join(parts, ":"))                   │
│     return sha256(key)  // 哈希避免 Key 过长             │
│   }                                                      │
│                                                         │
│ 检测流程:                                                │
│   CCProtectionDetector.Detect(req):                      │
│     for _, rule := range ccRules:                        │
│       if !matchPath(rule.Path, req.URL.Path): continue   │
│       if rule.Method != "" && rule.Method != req.Method: │
│         continue                                         │
│                                                         │
│       key = buildCCKey(req, rule)                        │
│       count = Redis.INCR("cc:count:" + key)              │
│       Redis.EXPIRE("cc:count:" + key, rule.Window)       │
│                                                         │
│       if count > rule.Requests:                          │
│         Redis.SET("cc:ban:" + key, "1", rule.BanTime)    │
│         return Threat{Type: "cc_protection", ...}        │
│                                                         │
│     return nil                                           │
└─────────────────────────────────────────────────────────┘
```

### 与现有限流器的关系

```
三层限流架构:

  Layer 1: CCProtectionDetector (自定义参数)
    ├── 维度: IP / Header / Param / Cookie / 组合
    ├── 粒度: 接口级别
    └── 场景: 登录爆破、API 滥用、搜索刷量

  Layer 2: RateLimitDetector (IP 级别)
    ├── 维度: IP
    ├── 粒度: 站点级别
    └── 场景: 通用频率限制

  Layer 3: RedisRateLimiter (分布式 IP 级别)
    ├── 维度: IP
    ├── 粒度: 全局
    └── 场景: 多实例共享限流

执行顺序: Layer 1 → Layer 2 → Layer 3
  任一层触发 → 立即返回 429
```

---

## 总结: 功能矩阵

| 功能 | 静态站点模式 | 代理模式 | 实现文件 |
|------|:-----------:|:-------:|---------|
| SSL/TLS (Let's Encrypt) | ✅ | ✅ | `ssl/acme_client.go` |
| SSL 自动续期 | ✅ | ✅ | `ssl/auto_renew.go` |
| WAF (OWASP Top 10) | ✅ | ✅ | `firewall/engine.go` |
| IP 黑/白名单 | ✅ | ✅ | `firewall/detectors/blacklist.go` |
| GeoIP 访问控制 | ✅ | ✅ | `firewall/detectors/geoip.go` |
| 速率限制 | ✅ | ✅ | `firewall/detectors/rate_limit.go` |
| CC 防护 (自定义参数) | ✅ | ✅ | `firewall/detectors/cc_protection.go` |
| 预渲染 (Chromium) | ✅ | ✅ | `prerender/engine.go` |
| SEO Meta 优化 | ✅ | ✅ | `seo/meta_tags.go` |
| 结构化数据注入 | ✅ | ✅ | `seo/structured_data.go` |
| AI 爬虫优化 (AEO) | ✅ | ✅ | `seo/aeo.go` |
| LLM SEO 优化 | ✅ | ✅ | `seo/llm_optimizer.go` |
| Sitemap 预热 | ✅ | ✅ | `prerender/preheat.go` |
| 百度/必应推送 | ✅ | ✅ | `prerender/push/manager.go` |
| 威胁情报订阅 | ✅ | ✅ | `threatintel/fetcher.go` |
| 反向代理 | ❌ | ✅ | `proxy/proxy.go` |
| 静态文件服务 | ✅ | ❌ | `site-handler/handler.go` |

✅ = 已实现  ❌ = 不适用
