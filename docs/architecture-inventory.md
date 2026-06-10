# 架构清单 — Architecture Inventory

> 基于代码实际结构整理，反映当前 v2.1.0 真实架构。
> 更新日期: 2026-06-09

---

## 一、系统全景架构图

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              客户端请求层                                         │
│               (浏览器 / 爬虫 / API客户端 / cURL 等)                                 │
└────────────────────────────────────────────────┬─────────────────────────────────┘
                                                 │
┌────────────────────────────────────────────────▼─────────────────────────────────┐
│                            接入层 — 端口映射                                      │
│  ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────────────┐  │
│  │  管理控制台 :9597   │  │  API 服务 :9598     │  │  站点服务器 :808x (每站点)  │  │
│  │  (React 前端)      │  │  (Gin REST API)     │  │  (独立端口, WAF+渲染)      │  │
│  └────────────────────┘  └────────────────────┘  └────────────────────────────┘  │
└────────────────────────────────────────────────┬─────────────────────────────────┘
                                                 │
┌────────────────────────────────────────────────▼─────────────────────────────────┐
│                            中间件层 (Gin Middleware)                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  JWT认证      │  │  全局错误处理  │  │  WAF检测     │  │  OTel 遥测/追踪      │  │
│  │  (auth)      │  │  (error)     │  │  (waf)       │  │  (telemetry)         │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└────────────────────────────────────────────────┬─────────────────────────────────┘
                                                 │
┌────────────────────────────────────────────────▼─────────────────────────────────┐
│                            核心引擎层                                              │
│  ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────────────┐  │
│  │  WAF 防火墙引擎     │  │  预渲染引擎          │  │  智能流量路由              │  │
│  │  ┌──────────────┐  │  │  ┌──────────────┐  │  │  ┌────────────────────┐  │  │
│  │  │12种检测器链  │  │  │  │Chromium池    │  │  │  │爬虫识别→分类转发    │  │  │
│  │  │规则评分→动作  │  │  │  │优先级队列    │  │  │  └────────────────────┘  │  │
│  │  │黑白名单/限流  │  │  │  │Redis缓存     │  │  └────────────────────────────┘  │
│  │  └──────────────┘  │  │  │并发控制     │  │                                   │
│  │  ┌──────────────┐  │  │  │流式渲染     │  │                                   │
│  │  │安全子模块:     │  │  │  └──────────────┘  │                                   │
│  │  │bot管理/零信任 │  │  └────────────────────┘                                   │
│  │  │速率限制/WAF   │  │                                                           │
│  │  └──────────────┘  │                                                           │
│  └────────────────────┘  └────────────────────┘                                   │
└────────────────────────────────────────────────┬─────────────────────────────────┘
                                                 │
┌────────────────────────────────────────────────▼─────────────────────────────────┐
│                            数据层 & 基础设施                                       │
│  ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────────────┐  │
│  │  Redis 缓存/存储   │  │  Prometheus 监控    │  │  Let's Encrypt SSL         │  │
│  │  - 渲染结果        │  │  - HTTP指标         │  │  - ACME 账户注册           │  │
│  │  - 站点配置        │  │  - 系统指标         │  │  - HTTP-01/DNS-01 挑战    │  │
│  │  - 会话/Token     │  │  - 健康检查         │  │  - 自动续签通知            │  │
│  │  - 日志队列        │  │  - 告警            │  │                             │  │
│  └────────────────────┘  └────────────────────┘  └────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

---

## 二、模块清单 (Go Packages)

### 2.1 启动与编排

| 模块 | 包路径 | 职责 | 关键文件 |
|------|--------|------|---------|
| 启动入口 | `cmd/api/` | 信号处理、启动引导 | `main.go` (30行) |
| 启动管理器 | `internal/bootstrap/` | 模块初始化编排、优雅关闭 | `runner.go`, `application.go` |

### 2.2 API 层

| 模块 | 包路径 | 职责 | 控制器数 |
|------|--------|------|---------|
| API 控制器 | `internal/api/controllers/` | REST 端点实现 | 11个控制器 |
| API 路由 | `internal/api/routes/` | 路由注册与中间件绑定 | — |

**11个控制器:**
| 控制器 | 主要端点 | 功能 |
|--------|---------|------|
| `auth_controller.go` | `/api/v1/auth/*` | 登录/登出/首次运行 |
| `system_controller.go` | `/api/v1/system/*` | 系统配置 CRUD |
| `overview_controller.go` | `/api/v1/overview` | 概览仪表盘数据 |
| `monitoring_controller.go` | `/api/v1/monitoring/*` | 实时监控统计 |
| `firewall_controller.go` | `/api/v1/firewall/*` | 攻击日志/黑白名单 |
| `crawler_controller.go` | `/api/v1/crawler/*` | 爬虫日志/统计 |
| `preheat_controller.go` | `/api/v1/preheat/*` | 预热管理/触发 |
| `push_controller.go` | `/api/v1/push/*` | 推送管理/日志 |
| `sites_controller.go` | `/api/v1/sites/*` | 站点 CRUD/配置 |
| `ssl_controller.go` | SSL 证书管理 | 证书申请/续签/删除 |
| `logs_controller.go` | `/api/v1/logs` | 访问日志查询 |

### 2.3 认证与安全层

| 模块 | 包路径 | 关键文件 | 功能 |
|------|--------|---------|------|
| 认证核心 | `internal/auth/` | `jwt.go`, `user.go`, `middleware.go` | JWT 令牌、用户管理、认证中间件 |
| 2FA 双因素 | `internal/auth/` | `2fa.go`, `totp_manager.go`, `totp_test.go` | TOTP 双因素认证 |
| 防火墙引擎 | `internal/firewall/` | `engine.go`, `detector_manager.go` | 12种检测器编排 |
| 防火墙动作 | `internal/firewall/` | `action.go` | Allow/Block/Challenge/RateLimit |
| 检测器集合 | `internal/firewall/detectors/` | 12个检测器文件 | UA/IP/速率/GeoIP/请求头等 |

**安全子模块 (internal/security/):**
| 子模块 | 包路径 | 功能 |
|--------|--------|------|
| 速率限制 | `security/ratelimit/` | API 速率限制 + Schema 验证 |
| WAF 引擎 | `security/waf/` | 独立 WAF 规则引擎 |
| WAF 检测器 | `security/waf/detectors/` | XSS/注入/CSRF/敏感数据检测 |
| 爬虫管理 | `security/botmanager/` | 爬虫指纹/挑战/行为分析 |
| 零信任 | `security/zerotrust/` | 设备指纹/持续验证/风险评分 |

### 2.4 预渲染引擎层

| 模块 | 包路径 | 关键文件 | 功能 |
|------|--------|---------|------|
| 渲染引擎 | `internal/prerender/` | `engine.go`, `engine_manager.go` | Chromium 渲染核心 |
| 爬虫检测 | `internal/prerender/` | `crawler.go` | UA+行为爬虫识别 |
| 实例池 | `internal/prerender/pool/` | `pool.go`, `worker.go` | Chromium 实例复用池 |
| 优先级队列 | `internal/prerender/` | `queue.go` | 4级优先级的渲染队列 |
| 并发管理 | `internal/prerender/` | `concurrency_manager.go` | 动态并发限流 |
| 预热模块 | `internal/prerender/` | `preheat.go`, `preheat/` | Sitemap 预热 |
| 推送模块 | `internal/prerender/push/` | `manager.go` | 渲染结果推送 |
| 缓存模块 | `internal/cache/manager.go` | `manager.go` | Redis (go-redis) |
| 流式渲染 | `internal/prerender/streaming/` | `chunked.go`, `first_screen.go` | 首屏快速输出 |
| 增量更新 | `internal/prerender/incremental/` | `dom_diff.go`, `selective.go` | DOM 差异更新 |
| 渲染优化 | `internal/prerender/optimizer/` | `optimizer.go` | 渲染策略优化 |

### 2.5 基础设施层

| 模块 | 包路径 | 功能 |
|------|--------|------|
| 反向代理 | `internal/proxy/` | 请求代理转发 |
| 请求路由 | `internal/routing/` | 站点路由分发 |
| SSL/TLS | `internal/ssl/` | ACME客户端/HTTP-01/DNS-01/自动续签 |
| Redis | `internal/redis/` | Redis 客户端封装/发布订阅 |
| 缓存管理 | `internal/cache/` | 缓存管理器 |
| 配置管理 | `internal/config/` | YAML 配置/热更新/Redis同步 |
| 定时调度 | `internal/scheduler/` | Cron 任务调度 |
| 任务队列 | `internal/task/` | 任务队列管理 |
| 站点处理器 | `internal/site-handler/` | 站点级请求处理 |
| 站点服务器 | `internal/site-server/` | 独立站点 HTTP 服务管理 |
| 日志系统 | `internal/logging/` | 结构化日志/爬虫日志/访问日志 |
| 监控系统 | `internal/monitoring/` | Prometheus 指标/健康检查 |
| 遥测系统 | `internal/monitoring/telemetry/` | OpenTelemetry 追踪/导出器 |
| 告警系统 | `internal/monitoring/alerting/` | 告警规则/通知 |
| 监控仪表盘 | `internal/monitoring/dashboard/` | 监控可视化 |
| 可观测性 | `internal/observability/` | 观测抽象层 |
| GeoIP | `internal/services/` | 地理位置服务 |
| 域名解析 | `internal/services/` | DNS 解析服务 |
| 日志处理 | `internal/services/` | 异步日志增强 |
| 模型定义 | `internal/models/` | 站点/WAF 数据模型 |
| 数据仓库 | `internal/repository/` | 站点/WAF 仓库 |
| 国际化 | `internal/i18n/` | 多语言支持 |
| 常量定义 | `internal/constants/` | 系统常量 |
| 工具函数 | `internal/utils/` | 公共工具/国家代码 |

### 2.6 前端 (React 管理台)

| 模块 | 路径 | 技术 |
|------|------|------|
| 前端核心 | `web/src/` | React 18 + TypeScript + Vite |
| UI 组件库 | Ant Design 5.x | 专业级 UI 组件 |
| 状态管理 | Redux Toolkit | 全局状态 |
| 国际化 | i18next | 多语言支持 |
| 图表 | ECharts + Ant Design Charts | 数据可视化 |
| 测试 | Playwright (148 E2E tests) | 端到端测试 |

### 2.7 部署体系

| 组件 | 位置 | 说明 |
|------|------|------|
| 构建脚本 | `build.sh` | 多平台构建 (linux/windows/darwin × amd64/arm64) |
| 安装脚本 | `install.sh` | 自动检测环境/安装Redis/安装Chromium |
| 启动脚本 | `start.sh` | start/restart/stop/status |
| Docker | `docker/` | 容器化配置 |
| K8s | `deploy/k8s/` | Kubernetes 部署清单 |
| 配置文件 | `configs/` | YAML 配置模板 |

---

## 三、依赖全景图

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              Prerender Shield                              │
├────────────────────────────────────────────────────────────────────────────┤
│  Go 运行时           │  Gin (HTTP) │ chromedp (渲染) │ go-redis (缓存)    │
│  lego (ACME/SSL)    │  JWT (认证) │ Prometheus (监控)│ OTel (遥测)        │
│  zap (日志)         │  cron (调度)│ geoip2 (地理)   │ gopsutil (系统)    │
│  viper (配置)       │  testify(测试)│ wire (DI)     │ uuid (标识)        │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## 四、各层依赖关系

```
API 控制器层 ──调用──→ 服务层 ──调用──→ 数据层
     │                      │
     │                      ├──→ prerender (渲染引擎)
     │                      │     ├──→ pool (Chromium池)
     │                      │     ├──→ cache (分级缓存)
     │                      │     ├──→ queue (优先级队列)
     │                      │     └──→ streaming (流式)
     │                      │
     │                      ├──→ security/waf (防火墙)
     │                      │     ├──→ detectors (检测器)
     │                      │     └──→ ratelimit (限流)
     │                      │
     │                      ├──→ ssl (证书管理)
     │                      ├──→ monitoring (监控)
     │                      ├──→ logging (日志)
     │                      └──→ scheduler (调度)
     │
     ├──→ redis (数据持久化)
     ├──→ config (配置管理)
     └──→ middleware (中间件链)
```
