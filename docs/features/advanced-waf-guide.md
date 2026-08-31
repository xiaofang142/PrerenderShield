# 高级 WAF 防御使用指南（CC 攻击 / 威胁情报 / 爬虫真实性验证 / GeoIP 兜底）

> 站点基础防火墙（OWASP Top 10 / 自定义规则 / IP 名单）见 `docs/features/waf-firewall.md`。
> 本文面向**使用者**，讲解四个「配置已暴露但缺使用指引」的高级防护：如何启用、如何配置、如何验证、有哪些坑。

所有配置均为**站点级**（位于 `sites[].firewall.*`），存 Redis，控制台修改即时生效。

---

## 1. CC 攻击防护（`firewall.cc_protection`）

CC（Challenge Collapsar）指大量高频、看似正常的并发请求压垮应用。本防护提供**自定义多维限流**：比内置 `rate_limit`（按 IP+窗口计数）更灵活，可按 路径 + 方法 + 任意维度组合 计数。

### 工作原理

对每个命中规则的请求：取维度值拼接 → SHA256 生成唯一 key → Redis 计数：

- Redis 计数键：`cc:count:<key>`，首次计数时设置 `window` 秒过期
- Redis 封禁键：`cc:ban:<key>`，计数超过 `requests` 后写入，有效 `ban_time` 秒
- 已被封禁的 IP（ban 键存在）直接拦截，不再计数

### 维度（`rules[].dimensions`）

| 维度写法 | 取请求的哪个值 |
|---------|---------------|
| `ip` | 客户端 IP |
| `header:<名称>` | 请求头值（如 `header:User-Agent`、`header:Referer`） |
| `param:<名称>` | URL 查询参数（如 `param:uid`） |
| `cookie:<名称>` | Cookie 值（如 `cookie:session`） |
| `path` | 请求路径 |

> 维度决定「按什么分组限流」。只写 `ip` = 每个 IP 独立计数；再加 `param:uid` = 「每个 IP × 每个 uid」一个计数桶。

### 路径匹配（`rules[].path`）

| 写法 | 匹配规则 |
|-----|---------|
| 空 / `/` / `/*` | 匹配所有路径 |
| `/api/login` | 精确匹配 |
| `/api/*` | 前缀匹配（`/api/x`、`/api/x/y` 都命中） |

### 配置示例

```yaml
sites:
  - id: "site1"
    firewall:
      cc_protection:
        enabled: true
        rules:
          - name: "登录接口防爆破"
            path: "/api/login"
            method: "POST"            # 空 = 不限方法
            dimensions: ["ip", "param:account"]
            requests: 10              # 窗口内超过 10 次触发
            window: 60                # 窗口 60 秒
            ban_time: 1800            # 封禁 30 分钟
            enabled: true
          - name: "整站按 UA 限流"
            path: "/*"
            dimensions: ["ip", "header:User-Agent"]
            requests: 300
            window: 300
            ban_time: 3600
            enabled: true
```

### 验证

```bash
# 封禁态请求返回 403（或你的默认动作），日志出现 SubType=banned
curl -X POST http://127.0.0.1:8082/api/login -H "user-agent: tester"

# Redis 观测计数/封禁键
redis-cli keys 'cc:count:*'
redis-cli keys 'cc:ban:*'
```

> 注意：`cc:count`/`cc:ban` 键无 TTL 兜底会残留，建议配合 Redis 定期清理或接受其自然过期（计数键有 window TTL，封禁键有 ban_time TTL）。

---

## 2. 威胁情报订阅（`firewall.threat_intel`）

自动从**免费公开威胁情报源**拉取已知恶意 IP/CIDR 进 Redis，WAF 请求时命中黑名单即拦截。

### 内置数据源（`sources[]`，默认全部 `enabled: false`）

| 源 | 格式 | 默认更新间隔 | 说明 |
|----|------|-------------|------|
| Abuse.ch Feodo Tracker | csv（IP 列 `dst_ip`） | 1h | 僵尸网络 C2 主机 |
| Blocklist.de All | text | 6h | 主动攻击者 IP |
| Emerging Threats Compromised IPs | text | 12h | 已失陷主机 |
| Spamhaus DROP List | text | 12h | 服务商黑洞网段 |
| CINS Army Threat List | text | 6h | 恶意扫描/攻击 IP |

支持 `format: csv | text | json`；csv/json 需用 `ip_field` 指定 IP 所在字段。源按各自的 `update_interval` 独立调度拉取，**并发传输见 `concurrency`（默认 3）**。

### 关键配置

| 键 | 默认 | 说明 |
|----|------|------|
| `enabled` | `false` | 总开关（`firewall.threat_intel.enabled`） |
| `global_key` | `threatintel:global:blacklist` | 全局黑名单 Redis 集合键，WAF 检测器查它 |
| `max_ips` | `50000` | 单源最多入库条数（防内存爆炸） |
| `concurrency` | `3` | 并发拉取数 |
| `sources[].enabled` | `false` | 逐个源开启 |

### 配置示例

```yaml
sites:
  - id: "site1"
    firewall:
      threat_intel:
        enabled: true
        global_key: "threatintel:global:blacklist"
        max_ips: 50000
        concurrency: 3
        sources:
          - name: "Blocklist.de All"
            url: "https://lists.blocklist.de/lists/all.txt"
            format: "text"
            update_interval: "6h"
            enabled: true
          - name: "Spamhaus DROP List"
            url: "https://www.spamhaus.org/drop/drop.txt"
            format: "text"
            update_interval: "12h"
            enabled: true
```

### 多站点 + 全局入库说明

威胁情报是**全局数据源**：任一站点的源会被 `MergeConfig` 汇总进全局 Fetcher，重复的源只拉取一次。但**检测开关在站点级**——`firewall.threat_intel.enabled: true` 的站点才会去查黑名单。

### 验证

```bash
# 确认拉取成功（日志会打印 Fetched N IPs）
redis-cli scard threatintel:global:blacklist     # 全局黑名单数量
redis-cli scard threatintel:source:blocklist_de_all
redis-cli sismember threatintel:global:blacklist 1.2.3.4

# 用已入库 IP 发起请求 → 拦截，攻击日志 SubType=known_malicious_ip
curl -H "X-Forwarded-For: 1.2.3.4" http://127.0.0.1:8082/
```

### 坑与建议

- **外部依赖网络**：源 URL 不可达时该源拉取失败（日志 Error，不会中断服务），其余源照常。
- **免费源有体积与频率限制**：`max_ips` 兜底防止超大源吃内存；拉取太密会被对方限流，保持默认间隔即可。
- **全局黑名单会在每次拉取时重建**（先删后加），墙裂建议 Redis 开启持久化，避免重启后黑名单短暂为空。
- 想全站启用，可把 `threat_intel` 配置复制到各站点，或后续版本做成全局段（当前为站点级）。

---

## 3. 爬虫真实性验证（`firewall.bot_verify`）

对声称是 Google 搜索爬虫的请求做 **Google rDNS 反解双向验证**，识别「伪造成 Googlebot 的 UA」浪费渲染资源或绕过封禁的行为。

### 模式（`bot_verify.mode`）

| 模式 | 行为 | 风险 |
|------|------|------|
| `log`（默认） | 验证结果写入爬虫日志 `verified` 字段，**不拦** | 零风险，先观察 |
| `block` | 对「确认伪造」的搜索爬虫返回 403；DNS 超时/故障（unknown）一律放行 | 仅拦确凿伪造，杜绝误杀 |

> 采用「可证伪即拦、疑罪从无」策略：只有反解结果与声称 UA 明确不匹配（伪造）才拦截；DNS 异常、超时等不确定情况一律放行，宁可漏拦不误杀。

### 配置

```yaml
sites:
  - id: "site1"
    firewall:
      bot_verify:
        enabled: true
        mode: "log"   # 先 log 观察准确率，再切 block
```

### 验证

- 模式 `log`：发起 `curl -A "Googlebot"`，到「爬虫日志」查看该记录的 `verified` 字段（`true`/`false`/`unknown`）。
- 模式 `block`：对伪造 UA 应返回 403；对真实 Google 反解应放行。
- 注意：仅对**搜索类爬虫**（googlebot 等，严格匹配 `googlebot.com`/`google.com`/`googleusercontent.com` 反解后缀）验证；DNS 反解需要服务器网络可达 Google 的 PTR DNS。
- 验证结果带磁盘缓存 `data/botverify_cache.json`（可用 `PRERENDER_BOTVERIFY_CACHE` 覆盖）：首次未缓存时异步回填，请求路径零阻塞；DNS 故障为 fail-open（返回 `unknown` 且不缓存，杜绝误杀）。

---

## 4. GeoIP 地域管控兜底链（`firewall.geoip`）

本地优先、多级兜底，防止单点失败导致误封/漏判：

```
MaxMind MMDB（推荐） ─成功→ 使用本地库
        │ 缺失/加载失败
        ▼
外部免费 API（ip-api/ipinfo/ipapi-co） ─成功→ 解析并写磁盘缓存
        │ API 命中限频/失败
        ▼
磁盘缓存 data/geoip_cache.json（保留 7 天内历史结果）
```

### 配置

```yaml
sites:
  - id: "site1"
    firewall:
      geoip:
        enabled: true
        database_path: "./rules/GeoLite2-Country.mmdb"   # 有 MMDB 就全走本地
        api_provider: "ip-api"          # 无 MMDB 时才用：ip-api / ipinfo / ipapi-co
        api_key: ""                     # ipinfo 必需
        block_list: ["KP", "CU"]        # 封禁国家码
        # allow_list: ["CN"]            # 用白名单模式：非白名单全部拦截
```

### 坑与建议

- 高流量站点**务必**用 MMDB（免费 API 限频：ip-api 45 次/分钟；ipinfo 50k/月；ipapi-co 1k/天），否则大流量下频繁超频、退到磁盘缓存。
- 磁盘缓存路径可由 `PRERENDER_GEOIP_CACHE` 覆盖。
- 小量站点可只配 `api_provider` 不带 MMDB，靠磁盘缓存兜底，成本可控。

---

## 附：如何快速拿到上述防护的现场日志

控制台「日志」模块可按 `SubType` 过滤（`cc_protection` / `threat_intel` / `bot_verify`），攻击日志见 `GET /api/v1/firewall/attacks`。
