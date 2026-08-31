# 配置参考（CONFIG REFERENCE）

本文按 YAML 节列出 PrerenderShield 的全部核心配置键，键名与默认值对照 [`internal/config/config.go`](../internal/config/config.go) 结构体与 [`configs/config.example.yml`](../configs/config.example.yml) 模板，与仓库实际结构一致。

> **阅读须知**
> - 配置文件默认路径：`./config.yml`，启动时通过 `api --config <路径>` 指定
> - **站点级配置**（`sites` 节）日常由控制台/API 管理并存储于 Redis（键 `prerender:config:sites`，热更新）；`config.yml` 中的 `sites` 仅作为初始导入
> - 配置文件支持 `${VAR:-default}` 环境变量替换；环境变量（如 `CACHE_REDIS_URL`、`SERVER_API_PORT`）优先级高于文件值，完整列表见 [ENV_VARS.md](ENV_VARS.md)
> - 值不合法时校验器会回退默认值并输出 warning（如非法 `cache.type` → `memory`），不会启动失败

---

## server — 服务器

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `server.address` | string | `0.0.0.0` | 监听地址 |
| `server.api_port` | int | `9598` | REST API 端口 |
| `server.console_port` | int | `9597` | 管理控制台端口 |
| `server.public_api_url` | string | `http://localhost:9598` | API 公网地址，控制台前端据此访问 API；支持 `${API_PUBLIC_URL:-...}` 替换 |

## dirs — 目录

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `dirs.data_dir` | string | `./data` | 运行数据目录（GeoIP/BotVerify 缓存等） |
| `dirs.static_dir` | string | `./static` | 静态资源与预渲染产物目录 |
| `dirs.certs_dir` | string | `./certs` | SSL 证书目录 |
| `dirs.admin_static_dir` | string | `./web` | 管理控制台静态文件目录（构建产物 `bin/web`） |

## cache — 缓存

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `cache.type` | string | `memory`（模板/生成配置为 `redis`） | 缓存类型，仅 `memory` / `redis`，非法值回退 `memory` |
| `cache.redis_url` | string | `localhost:6379` | Redis 地址（`host:port` 或 `redis://[password@]host:port/db`）；`CACHE_REDIS_URL` 环境变量可覆盖（`REDIS_HOST`/`REDIS_PORT`/`REDIS_PASSWORD`/`REDIS_DB` 组合优先） |
| `cache.redis_password` | string | `""` | Redis 密码 |
| `cache.redis_db` | int | `0` | Redis DB 编号 |
| `cache.memory_size` | int | `1000` | L1 内存缓存条目上限；≤0 回退 1000；`CACHE_MEMORY_SIZE` 可覆盖 |
| `cache.redis_pool.max_active` | int | `20` | 最大活跃连接数 |
| `cache.redis_pool.max_idle` | int | `10` | 最大空闲连接数 |
| `cache.redis_pool.idle_timeout` | duration | `5m` | 空闲连接超时 |
| `cache.redis_pool.pool_timeout` | duration | `30s` | 获取连接超时 |

## storage — 存储

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `storage.type` | string | `redis` | 配置/数据存储后端（用户账号、站点配置存于 Redis，**生产必须开启 AOF 持久化**） |

## monitoring — 监控

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `monitoring.enabled` | bool | `true` | 启用监控 |
| `monitoring.prometheus_address` | string | `:9090` | Prometheus 指标暴露地址；`MONITORING_PROMETHEUS_ADDRESS` 可覆盖 |

### monitoring.alerting — 告警

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `monitoring.alerting.enabled` | bool | `false` | 启用告警 |
| `monitoring.alerting.rules_path` | string | `""` | 告警规则 JSON 路径（示例见 `configs/alert-rules.example.json`） |
| `monitoring.alerting.notifications.webhook.enabled` | bool | `false` | 启用 Webhook 通知 |
| `monitoring.alerting.notifications.webhook.url` | string | `""` | Webhook URL（如 Slack） |
| `monitoring.alerting.notifications.webhook.secret` | string | `""` | Webhook 签名密钥 |
| `monitoring.alerting.notifications.email.enabled` | bool | `false` | 启用邮件通知 |
| `monitoring.alerting.notifications.email.smtp_host` | string | `""` | SMTP 主机 |
| `monitoring.alerting.notifications.email.smtp_port` | int | `0` | SMTP 端口（常见 587） |
| `monitoring.alerting.notifications.email.username` / `.password` | string | `""` | SMTP 认证 |
| `monitoring.alerting.notifications.email.from` | string | `""` | 发件人 |
| `monitoring.alerting.notifications.email.to` | []string | `[]` | 收件人列表 |

### monitoring.metrics_persistence — 指标持久化

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `monitoring.metrics_persistence.enabled` | bool | `false` | 启用监控数据持久化（快照写入 Redis） |
| `monitoring.metrics_persistence.interval` | int | `300` | 持久化间隔（秒），默认 5 分钟 |
| `monitoring.metrics_persistence.retention_hours` | int | `24` | 数据保留时长（小时，0 时回退 24） |
| `monitoring.metrics_persistence.aggregate_enabled` | bool | `false` | 是否启用按小时窗口数据聚合 |
| `monitoring.metrics_persistence.aggregate_interval` | int | `3600` | 聚合间隔（秒），默认 1 小时 |

## app — 应用信息

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `app.version` | string | `3.0.0` | 版本标识（空值校验时回退 `1.0.0`） |
| `app.official_url` | string | `https://prerender.websitetool.cn` | 官网地址 |

## ssl — SSL 证书（全局）

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `ssl.enabled` | bool | `false` | 启用 SSL 自动申请（Let's Encrypt/ACME） |
| `ssl.auto_renew` | bool | `false` | 自动续签（模板示例为 `true`） |
| `ssl.email` | string | `""` | ACME 账户邮箱 |
| `ssl.production` | bool | `false` | `false`=ACME 测试环境，`true`=Let's Encrypt 生产环境 |
| `ssl.http_port` | int | `80`（模板） | HTTP-01 挑战监听端口 |
| `ssl.check_interval` | duration | `24h`（模板） | 证书检查间隔 |
| `ssl.renew_before_days` | int | `30`（模板） | 提前续签天数 |
| `ssl.max_retries` | int | `3`（模板） | 最大重试次数 |
| `ssl.retry_delay` | duration | `1h`（模板） | 重试间隔 |
| `ssl.webhook_url` | string | `""` | 证书事件通知 Webhook |
| `ssl.dns.provider` | string | `""` | DNS 服务商：`cloudflare` / `aliyun` / `tencentcloud` / `aws` 等（DNS-01/通配符必需） |
| `ssl.dns.credentials` | map | `{}` | 对应服务商 API 凭证键值对 |

## seo — SEO

### seo.sitemap — Sitemap 生成

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `seo.sitemap.enabled` | bool | `false` | 启用 sitemap.xml 自动生成 |
| `seo.sitemap.base_url` | string | `""` | 站点根 URL |
| `seo.sitemap.output_dir` | string | `""` | 输出目录（默认进入 `static_dir`） |
| `seo.sitemap.change_freq` | string | `daily` | 变更频率 |
| `seo.sitemap.default_priority` | string | `"0.5"` | 默认优先级 |
| `seo.sitemap.include_patterns` | []string | `["*.html","*.htm"]` | 包含的文件模式 |
| `seo.sitemap.exclude_patterns` | []string | `["admin/*","api/*","login*"]` | 排除的路径模式 |

### seo.robots — robots.txt 生成

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `seo.robots.enabled` | bool | `false` | 启用 robots.txt 自动生成 |
| `seo.robots.output_dir` | string | `"."` | 输出目录 |
| `seo.robots.sitemap_url` | string | `""` | 写入 robots.txt 的 sitemap 地址 |
| `seo.robots.rules[].user_agent` | string | — | 规则目标爬虫（如 `*`、`Baiduspider`） |
| `seo.robots.rules[].allow` / `.disallow` | []string | — | 允许/禁止路径 |
| `seo.robots.rules[].crawl_delay` | int | `0` | 抓取间隔（秒） |

### seo.llm — LLM SEO 优化

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `seo.llm.enabled` | bool | `false` | 启用 LLM 优化 |
| `seo.llm.provider` | string | `openai` | `openai` / `zhipu` / `deepseek` / `ollama` |
| `seo.llm.api_key` | string | `""` | API 密钥 |
| `seo.llm.api_url` | string | `""` | 自定义 API 地址（可选） |
| `seo.llm.model` | string | `gpt-4o-mini` | 模型名 |
| `seo.llm.max_tokens` | int | `500` | 单次最大 token |
| `seo.llm.temperature` | float | `0.3` | 温度 |
| `seo.llm.timeout` | duration | `10s` | 请求超时 |
| `seo.llm.max_retries` | int | `2` | 重试次数 |
| `seo.llm.prompts.title_optimization` | string | `""` | 标题优化提示词 |
| `seo.llm.prompts.description_optimization` | string | `""` | meta 描述优化提示词 |
| `seo.llm.prompts.keyword_extraction` | string | `""` | 关键词提取提示词 |
| `seo.llm.prompts.structured_data` | string | `""` | 结构化数据（JSON-LD）生成提示词 |

## commercial — 商业授权

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `commercial.plan` | string | `free` | `free` / `per-site` / `private-source`；`COMMERCIAL_PLAN` 可覆盖 |
| `commercial.max_sites` | int | `1` | 站点数上限；1 站点永久免费；`-1` = 不限（私有化授权）；`COMMERCIAL_MAX_SITES` 可覆盖 |
| `commercial.site_price_usd_per_year` | int | `99` | 超出免费额度后单站点年费（USD）；`COMMERCIAL_SITE_PRICE_USD_PER_YEAR` 可覆盖 |
| `commercial.private_deploy_price_usd` | int | `9999` | 私有化源码交付费用（USD）；`COMMERCIAL_PRIVATE_DEPLOY_PRICE_USD` 可覆盖 |

## api_tokens — 管理 API Token

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `api_tokens` | []string | `[]` | sha256 hex 形态的 Token 列表（原文不落盘）；仅 `/api/v1/preheat/` 端点可用 |

---

## sites — 站点（站点级配置）

每个站点为一个 `sites[]` 元素，对应 `SiteConfig`。**日常通过控制台管理**（存 Redis 热更新）。

### 站点基本信息

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `sites[].id` | string | — | 站点唯一 ID |
| `sites[].name` | string | — | 站点名称 |
| `sites[].domains` | []string | — | 绑定域名（支持多域名） |
| `sites[].port` | int | `8084`（默认站点） | 站点监听端口（1–65535，一站一端口） |
| `sites[].mode` | string | `static` | `proxy`（代理已有应用）/ `static`（静态资源站）/ `redirect`（重定向） |

### proxy / redirect

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `sites[].proxy.target_url` | string | `""` | 上游真实服务地址（`mode: proxy` 必填） |
| `sites[].redirect.status_code` | int | `301` | 重定向状态码（301/302） |
| `sites[].redirect.target_url` | string | `""` | 重定向目标 URL |

### sites[].firewall — 防火墙（WAF）

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `firewall.enabled` | bool | `true` | 启用 WAF（OWASP Top 10 规则引擎） |
| `firewall.rules_path` | string | `./rules` | 自定义规则与 GeoIP 数据库所在目录 |
| `firewall.action.default_action` | string | `block` | 默认动作（非法值回退 `block`） |
| `firewall.action.block_message` | string | `Request blocked by firewall` | 拦截提示 |
| `firewall.blacklist` | []string | `[]` | IP 黑名单 |
| `firewall.whitelist` | []string | `[]` | IP 白名单 |

### firewall.geoip — GeoIP

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `firewall.geoip.enabled` | bool | `false` | 启用地理位置过滤 |
| `firewall.geoip.database_path` | string | `./rules/GeoLite2-Country.mmdb` | MaxMind 数据库路径 |
| `firewall.geoip.api_provider` | string | `ip-api` | 数据库不可用时的 API 回退：`ip-api` / `ipinfo` / `ipapi-co` |
| `firewall.geoip.api_key` | string | `""` | API 密钥（ipinfo 必需） |
| `firewall.geoip.allow_list` / `.block_list` | []string | `[]` | 允许/阻止的国家地区码 |

### firewall.rate_limit — 频率限制

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `firewall.rate_limit.enabled` | bool | `false` | 启用 IP 频率限制 |
| `firewall.rate_limit.requests` | int | `100` | 窗口内允许请求数 |
| `firewall.rate_limit.window` | int | `60` | 窗口时长（秒） |
| `firewall.rate_limit.ban_time` | int | `3600` | 违规封禁时长（秒） |

### firewall.cc_protection — CC 攻击防护

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `firewall.cc_protection.enabled` | bool | `false` | 启用 CC 防护 |
| `firewall.cc_protection.rules[].name` | string | — | 规则名 |
| `firewall.cc_protection.rules[].path` | string | — | 匹配路径 |
| `firewall.cc_protection.rules[].method` | string | — | 匹配 HTTP 方法 |
| `firewall.cc_protection.rules[].dimensions` | []string | — | 统计维度（如 `ip`、`ua`） |
| `firewall.cc_protection.rules[].requests` | int | — | 阈值请求数 |
| `firewall.cc_protection.rules[].window` | int | — | 统计窗口（秒） |
| `firewall.cc_protection.rules[].ban_time` | int | — | 封禁时长（秒） |
| `firewall.cc_protection.rules[].enabled` | bool | — | 规则启停 |

### firewall.threat_intel — 威胁情报

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `firewall.threat_intel.enabled` | bool | `false` | 启用外部威胁情报 IP 库 |
| `firewall.threat_intel.sources[].name` | string | — | 源名称（Abuse.ch / Blocklist.de / Spamhaus 等） |
| `firewall.threat_intel.sources[].url` | string | — | 拉取地址 |
| `firewall.threat_intel.sources[].format` | string | — | `csv` / `text` |
| `firewall.threat_intel.sources[].ip_field` | string | — | CSV 中 IP 列名 |
| `firewall.threat_intel.sources[].update_interval` | string | — | 更新间隔（如 `1h`、`6h`） |
| `firewall.threat_intel.sources[].enabled` | bool | — | 源启停 |
| `firewall.threat_intel.global_key` | string | `threatintel:global:blacklist` | Redis 全局黑名单键 |
| `firewall.threat_intel.max_ips` | int | `50000` | IP 库容量上限 |
| `firewall.threat_intel.concurrency` | int | `3` | 拉取并发数 |

### firewall.bot_verify — 爬虫真实性验证

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `firewall.bot_verify.enabled` | bool | `false` | 启用 Google rDNS 双向验证 |
| `firewall.bot_verify.mode` | string | `log` | `log`=仅打标进爬虫日志（零风险）；`block`=拦截"确认伪造"的搜索爬虫（DNS 故障/超时一律放行） |

### sites[].prerender — 渲染预热

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `prerender.enabled` | bool | `true` | 启用渲染预热 |
| `prerender.pool_size` | int | `5` | Chromium 初始池大小 |
| `prerender.min_pool_size` | int | `2` | 最小空闲实例 |
| `prerender.max_pool_size` | int | `20` | 最大实例数 |
| `prerender.timeout` | int | `30` | 单次渲染超时（秒） |
| `prerender.cache_ttl` | int | `3600` | 缓存默认 TTL（秒） |
| `prerender.idle_timeout` | int | `300` | 实例空闲回收（秒） |
| `prerender.dynamic_scaling` | bool | `true` | 动态伸缩 |
| `prerender.scaling_factor` | float | `0.5` | 伸缩系数 |
| `prerender.scaling_interval` | int | `60` | 伸缩检查间隔（秒） |
| `prerender.crawler_headers` | []string | 常见爬虫 UA 列表 | 触发预渲染的爬虫协议头（Googlebot/Bingbot/Baiduspider 等） |
| `prerender.use_default_headers` | bool | `false` | 是否叠加内置默认爬虫协议头 |
| `prerender.include_patterns` | []string | `[]` | 渲染 URL 白名单（RequestURI 正则；空=全部可渲染） |
| `prerender.exclude_patterns` | []string | `[]` | 渲染 URL 黑名单（优先于白名单，如后台登录页） |
| `prerender.max_concurrency` | int | `0` | 站点级渲染并发预算；`0`=不限 |
| `prerender.stale_while_revalidate` | bool | `true`（nil 视为 true） | 过期缓存立即回旧值并异步重渲 |
| `prerender.category_policy` | map | `{}` | 爬虫分类策略：键 `search`/`social`/`ai`/`generic` → 值 `render` / `cache_only` / `passthrough` |

### prerender.ttl_rules — 缓存 TTL 分级规则

有序规则表，**首中生效**；未命中回退 `prerender.cache_ttl`，再回退引擎默认 24 小时。

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `prerender.ttl_rules[].pattern` | string | — | URL 匹配模式，匹配对象为完整 URL（含 scheme/host/path/query）：无 `*` 时按子串包含匹配，含 `*` 时按通配符翻译为正则（对齐 prerender.io pattern 模型） |
| `prerender.ttl_rules[].ttl_seconds` | int | — | 命中后的缓存 TTL（秒） |

```yaml
# 示例
ttl_rules:
  - pattern: "/product/*"   # 商品页缓存 10 分钟
    ttl_seconds: 600
  - pattern: "/news/"       # 新闻页缓存 1 小时
    ttl_seconds: 3600
```

### prerender.preheat — 定时预热

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `prerender.preheat.enabled` | bool | `false` | 启用定时预热 |
| `prerender.preheat.sitemap_url` | string | `""` | sitemap 地址（预热 URL 来源） |
| `prerender.preheat.schedule` | string | `0 0 * * *` | cron 表达式 |
| `prerender.preheat.concurrency` | int | `5` | 预热并发 |
| `prerender.preheat.default_priority` | int | `0` | 默认优先级 |
| `prerender.preheat.max_depth` | int | `3` | 爬取深度 |

### prerender.push — 搜索引擎推送

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `prerender.push.enabled` | bool | `false` | 启用 URL 推送 |
| `prerender.push.baidu_api` | string | `http://data.zz.baidu.com/urls` | 百度推送接口 |
| `prerender.push.baidu_token` | string | `""` | 百度推送 token |
| `prerender.push.bing_api` | string | `https://ssl.bing.com/webmaster/api.svc/json/SubmitUrl` | Bing 推送接口 |
| `prerender.push.bing_token` | string | `""` | Bing API Key |
| `prerender.push.baidu_daily_limit` | int | `1000` | 百度每日推送上限 |
| `prerender.push.bing_daily_limit` | int | `1000` | Bing 每日推送上限 |
| `prerender.push.push_domain` | string | `""` | 推送使用的站点域名 |
| `prerender.push.schedule` | string | `0 0 8 * * *` | cron 表达式，默认每天 8 点推送 |
| `prerender.push.indexnow_enabled` | bool | `false` | 启用 IndexNow（Bing/Yandex/Naver/Seznam） |
| `prerender.push.indexnow_key` | string | `""` | IndexNow 密钥（需同步在站点根放置 key 文件） |

### sites[].routing — 自定义路由

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `routing.rules[].id` | string | — | 规则 ID |
| `routing.rules[].pattern` | string | — | 匹配模式 |
| `routing.rules[].action` | string | — | 动作 |
| `routing.rules[].priority` | int | — | 优先级（数值越大越先） |

### sites[].file_integrity — 网页防篡改

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `file_integrity.enabled` | bool | `false` | 启用文件完整性检查 |
| `file_integrity.check_interval` | int | `300` | 检查间隔（秒） |
| `file_integrity.hash_algorithm` | string | `sha256` | 哈希算法 |

### sites[].ssl — 站点 SSL

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `ssl.enabled` | bool | `false` | 启用该站点 HTTPS |
| `ssl.http_port` | int | `80`（模板） | HTTP 端口（重定向与 ACME 验证） |
| `ssl.cert_file` / `.key_file` | string | `""` | 手动导入证书/密钥路径（可选） |
| `ssl.auto_cert` | bool | `false` | 自动申请证书（需全局 `ssl.enabled`） |
| `ssl.force_https` | bool | `false`（模板为 `true`） | 强制 HTTPS 重定向 |
| `ssl.hsts` | bool | `false`（模板为 `true`） | 启用 HSTS |
| `ssl.hsts_max_age` | int | `31536000`（模板） | HSTS max-age（秒，1 年） |

---

## 最小可用配置示例

```yaml
server:
  address: "0.0.0.0"
  api_port: 9598
  console_port: 9597
dirs:
  data_dir: ./data
  static_dir: ./static
  certs_dir: ./certs
  admin_static_dir: ./web
cache:
  type: redis
  redis_url: "localhost:6379"
```

完整字段示例见 [`configs/config.example.yml`](../configs/config.example.yml)；敏感环境变量见 [ENV_VARS.md](ENV_VARS.md)；站点配置的界面化管理见 [QUICK_START_GUIDE.md](QUICK_START_GUIDE.md)。
