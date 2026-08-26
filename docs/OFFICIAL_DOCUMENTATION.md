# Prerender Shield 官方文档

> 版本基准：v3.x · 本文档与代码库同步维护，如发现不一致以代码为准并提 Issue。

## 目录

1. [产品简介](#产品简介)
2. [核心概念](#核心概念)
3. [系统架构](#系统架构)
4. [功能总览](#功能总览)
5. [安装部署](#安装部署)
6. [快速上手](#快速上手)
7. [站点管理](#站点管理)
8. [管理控制台](#管理控制台)
9. [配置参考](#配置参考)
10. [环境变量](#环境变量)
11. [API 概览](#api-概览)
12. [FAQ](#faq)

---

## 产品简介

Prerender Shield 是一款企业级 Web 应用中间件，一体化解决**前后端分离架构下的两大痛点**：

| 痛点 | 解决方案 |
|------|----------|
| SPA 对搜索引擎爬虫不友好，SEO 差 | Bot 识别 + Headless Chromium 预渲染 + Redis 缓存/预热 |
| 传统 WAF 与渲染链路割裂、成本高 | 内置 OWASP Top 10 防护的 WAF 与预渲染共享同一流量入口 |

**核心卖点**

- 开源自托管：数据不出你的服务器，零授权费用起步
- 单二进制 + Redis 即可运行，一键脚本安装
- 爬虫请求返回完整静态 HTML，普通用户请求原样透传（按需渲染，零浪费）
- 站点级配置热更新（存 Redis），改规则不重启

## 核心概念

| 概念 | 说明 |
|------|------|
| **站点 (Site)** | 一个被保护的源站，绑定域名 + 上游端口 + 模式（proxy/static/redirect） |
| **渲染引擎 (Engine)** | Headless Chromium 实例池，负责将动态页面渲染为静态 HTML |
| **智能路由** | 爬虫请求 → 渲染/缓存 → 静态 HTML；用户请求 → 直接转发；API 请求 → 安全检查后转发或拦截 |
| **渲染缓存** | 渲染结果存 Redis（带 TTL 过期），配合 sitemap 批量预热与定时预热提升命中率 |
| **WAF** | OWASP Top 10 防护 + 自定义规则引擎（热加载）+ GeoIP 地域封禁 + CC 防护 + 威胁情报 |
| **Janitor** | 孤儿浏览器进程清扫器，防止崩溃后的僵尸 Chromium 累积 |

## 系统架构

```
爬虫/用户请求
     │
     ▼
┌─────────────────────────────────────────────┐
│              Prerender Shield               │
│                                             │
│  Bot识别 ──► 智能路由 ──┬─► 渲染引擎(Chromium池) │
│       │                ├─► Redis缓存           │
│  WAF检测 ──────────────┴─► 源站转发            │
│                                             │
│  控制台(:9597) ◄──WebSocket──► 实时监控        │
│  管理 API(:9598)                             │
└─────────────────────────────────────────────┘
     │                    │
     ▼                    ▼
   Redis               你的源站(:8082等)
```

详细架构图见 [ARCHITECTURE_DIAGRAMS.md](ARCHITECTURE_DIAGRAMS.md)，技术原理见官网 `/tech-principle` 页面。

## 功能总览

### 预渲染 & SEO

- **Headless Chromium 渲染引擎**：实例池复用、并发控制、可配置超时、使用计数回收
- **爬虫识别**：User-Agent 特征匹配，当前覆盖 Googlebot / Bingbot / Baiduspider / Sogouspider / Yandexbot
- **渲染预热**：sitemap.xml 自动解析批量预热（URL 去重）、API 触发预热、定时任务
- **SEO 增强**：per-site sitemap.xml/sitemap.txt 生成、robots.txt 生成、IndexNow 主动推送（Bing/Yandex/Naver/Seznam，需配置 key）
- **实验性**：LLM SEO 内容优化器（`seo.llm` 配置段，默认关闭；需自行提供 LLM API）

### 安全防护 (WAF)

- **OWASP Top 10**：SQLi/NoSQLi/命令注入、XSS（存储/反射/DOM）、CSRF、不安全反序列化、敏感数据泄露
- **自定义规则引擎**：规则热加载，控制台可视化编辑
- **GeoIP 地域管控**：本地 MaxMind MMDB（推荐）→ 外部 API 兜底（ip-api/ipinfo/ipapi-co）→ **磁盘持久缓存兜底**（`data/geoip_cache.json`，7 天内旧结果防误杀）
- **CC 攻击防护**：频率阈值自动封禁（`detectors/cc_protection.go`）
- **威胁情报**：恶意 IP 库定期拉取（`internal/threatintel`）

### 运维能力

- Web 管理控制台（`:9597`），JWT 认证，首次启动自助建号
- WebSocket 实时推送：告警事件 + 10s 周期指标广播
- Prometheus 指标（默认 `:9090`）、OpenTelemetry 链路追踪（可选）
- 配置加密：AES-256-GCM 加密存储敏感字段（v3.0.0+）

## 安装部署

### 方式一：一键脚本（推荐）

```bash
# 1. 构建（开发者）或直接下载 release
./build.sh

# 2. 安装（自动检测 OS/架构，安装 Redis 与 Chromium，交互式配置）
sudo ./install.sh

# 3. 启动
./start.sh start      # 启动
./start.sh restart    # 重启
./start.sh stop       # 停止
./start.sh status     # 状态与健康检查
```

> `install.sh` 会检测 Chromium 并在缺失时安装；也可通过
> `PRERENDER_CHROMIUM_PATH` / `CHROME_PATH` 环境变量指定已有浏览器路径。

### 方式二：Docker

```bash
docker compose -f docker/docker-compose.yml up -d
```

内存 < 4GB 的容器请设置 `PRERENDER_MAX_INSTANCES=2~3`。

### 方式三：手动

```bash
go build -o bin/prerender-shield cmd/api/main.go
redis-server &                      # 必须先有 Redis
cp configs/config.example.yml config/config.yml   # 按需修改
./bin/prerender-shield              # 前台运行查看日志
```

### 系统要求

| 项目 | 最低 | 推荐 |
|------|------|------|
| CPU | 2 核 | 4 核 |
| 内存 | 4 GB | 8 GB |
| 磁盘 | 10 GB | 20 GB SSD |
| OS | Ubuntu 22.04 LTS / 主流 Linux / macOS | Ubuntu 22.04 LTS |
| 架构 | x86_64 / arm64 | — |
| 依赖 | Redis ≥ 6、Chromium/Chrome | Redis 开启 AOF 持久化 |

## 快速上手

```bash
./start.sh start
curl http://localhost:9598/api/v1/health      # {"code":0,...} 即健康
open http://localhost:9597                    # 打开控制台
```

1. **首次登录建号**：无默认账号，首次访问控制台按引导创建管理员（见 `CheckFirstRun`）
2. **添加站点**：控制台 → 站点管理 → 绑定域名 + 源站地址 + 模式
3. **预热缓存**：站点详情 → 预热 → 从 sitemap 批量拉取
4. **验证 SEO**：`curl -A "Googlebot" http://你的域名/` 应返回完整 HTML

## 站点管理

站点配置示例：

```yaml
sites:
  - id: "site1"
    name: "my-spa"
    domains: ["example.com", "www.example.com"]
    port: 8082          # 监听端口
    mode: "proxy"       # proxy=反代源站 / static=托管静态目录 / redirect=重定向
    proxy:
      target_url: "http://127.0.0.1:3000"
    prerender:
      enabled: true
      timeout: 30       # 渲染超时（秒）
    firewall:
      enabled: true
      geoip:
        enabled: true
        block_list: ["KP"]             # 封禁国家码列表（allow_list 为反向白名单模式）
        database_path: "./rules/GeoLite2-Country.mmdb"
        api_provider: "ip-api"         # 无 MMDB 时的兜底: ip-api / ipinfo / ipapi-co
        api_key: ""                    # ipinfo 需要
```

- 站点配置存于 Redis，控制台修改**即时生效，无需重启**
- `mode: static` 时从 `static_dir/<site>/` 提供文件，并自动生成该站点的 sitemap

## 管理控制台

访问 `http://<host>:9597`：

| 模块 | 能力 |
|------|------|
| Dashboard | 实时指标（WebSocket 推送）、告警流 |
| 站点管理 | 站点 CRUD、域名/端口/模式、预热入口 |
| WAF / 防火墙 | 规则编辑（热加载）、GeoIP 名单、CC 阈值 |
| 日志 | 访问日志/爬虫日志检索，含 GeoIP 归属地 |
| SSL | ACME 自动证书申请与续期 |
| 系统设置 | 缓存、监控、商业授权 |

实时通道：`ws://<host>:9597/ws?token=<JWT>`（控制台已内置反向代理与鉴权）。

## 配置参考

主配置 `./config/config.yml`（模板见 `configs/config.example.yml`、生产样例见 `config.production.yaml.example`）。核心段落：

```yaml
server:
  address: 0.0.0.0
  api_port: 9598            # 管理 API（必须 1-65535）
  console_port: 9597        # 控制台
  public_api_url: "${API_PUBLIC_URL:-http://localhost:9598}"

dirs:
  data_dir: ./data          # 运行时数据（日志/PID/GeoIP缓存）
  static_dir: ./static      # 各站点静态根
  certs_dir: ./certs
  admin_static_dir: ./web

cache:
  type: redis               # 渲染缓存基于 Redis
  redis_url: "localhost:6379"
  memory_size: 1000         # 预留参数（当前未参与缓存链路）
  redis_pool:
    max_active: 20
    max_idle: 10
    idle_timeout: 5m
    pool_timeout: 30s

monitoring:
  enabled: true
  prometheus_address: ":9090"
```

**动态更新途径**（无需重启）：控制台直接修改 / `POST /system/config` / YAML 修改后被 watcher 自动加载。

## 环境变量

完整清单见 [ENV_VARS.md](ENV_VARS.md)，高频项：

| 变量 | 默认 | 说明 |
|------|------|------|
| `JWT_SECRET` | 自动生成 | **生产必须显式设置**（HMAC-SHA256） |
| `PRERENDER_MIN_INSTANCES` / `MAX_INSTANCES` | 2 / 10 | Chromium 池水位 |
| `PRERENDER_CHROMIUM_PATH` / `CHROME_PATH` | 自动探测 | 显式指定浏览器路径 |
| `PRERENDER_GEOIP_CACHE` | `data/geoip_cache.json` | GeoIP 磁盘缓存路径 |
| `REDIS_URL` | 取配置 | 覆盖 Redis 连接串 |
| `SERVER_API_PORT` / `SERVER_CONSOLE_PORT` | 取配置 | 端口覆盖 |
| `MONITORING_PROMETHEUS_ADDRESS` | `:9090` | Prometheus 地址 |

## API 概览

Base URL：`http://<host>:9598/api/v1`，认证方式 `Authorization: Bearer <JWT>`。
完整字段级文档见 [API.md](API.md)。

| 分组 | 端点示例 | 说明 |
|------|---------|------|
| 系统 | `GET /health` `GET /version` | 免认证健康检查/版本 |
| 认证 | `POST /auth/login` 等 | 首次运行开放注册，之后仅登录 |
| 站点 | `GET/POST/PUT/DELETE /sites` | 站点 CRUD |
| SEO | `/seo/sitemap` `/seo/robots`（GET 查询 + POST 生成） | sitemap 与 robots 管理 |
| 预热 | `/preheat/trigger` `/preheat/stats` `/preheat/task/status` 等 | 触发/查询预热任务 |
| SSL | `/ssl/*` | ACME 证书管理 |
| 系统 | `GET/POST /system/config` `POST /system/backup` | 配置读取/动态更新/备份 |

响应统一格式：成功 `{"code":0,"data":{...},"message":"ok"}`，失败 `{"code":4xx/5xx,"message":"..."}`。

## FAQ

**Q: 是开源的吗？商业使用要付费吗？**
核心功能开源免费自托管。商业版按站点授权/私有化整体授权，详见官网 Pricing 或配置段 `commercial`。

**Q: 和 Prerender.io / Cloudflare 有什么区别？**
自托管零持续费用、渲染与 WAF 一体、数据自主可控。详细对比见官网 `/competitor-comparison`。

**Q: 不装 MaxMind 数据库能用 GeoIP 吗？**
能，但外部免费 API 有严格限频（ip-api 45 次/分钟），高流量站点必须配置 MMDB；API 失败时可用磁盘缓存（`data/geoip_cache.json`）兜底 7 天内的历史解析结果。

**Q: 服务重启后渲染缓存会丢吗？**
渲染缓存在 Redis 中，只要 Redis 开启持久化（生产建议 AOF，docker-compose 已默认 appendonly）就不会丢。Redis 数据丢失后可通过预热任务重建缓存。
