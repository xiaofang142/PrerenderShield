# 功能清单 — Feature Inventory

> 基于代码实际功能整理，标注实现状态与代码位置。
> 更新日期: 2026-06-09

---

## 一、功能总览

| 功能域 | 功能数 | 实现状态 | 关键包 |
|--------|--------|---------|--------|
| 安全防护 | 15 | ✅ 全部实现 | `firewall/`, `security/`, `auth/` |
| 预渲染引擎 | 12 | ✅ 全部实现 | `prerender/` |
| SSL/TLS 证书 | 6 | ✅ 全部实现 | `ssl/` |
| 监控遥测 | 8 | ✅ 全部实现 | `monitoring/`, `observability/` |
| 管理控制台 | 11 | ✅ 全部实现 | `api/controllers/` |
| 基础设施 | 10 | ✅ 全部实现 | `proxy/`, `routing/`, `config/` |

---

## 二、安全防护功能 (15项)

### 2.1 OWASP Top 10 防护

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 1 | SQL注入检测 | `firewall/detectors/`, `security/waf/detectors/injection.go` | 95% | 正则+语法分析 |
| 2 | XSS跨站脚本 | `security/waf/detectors/xss.go` | 100% | 输入过滤+输出编码 |
| 3 | CSRF防护 | `security/waf/detectors/csrf.go` | 100% | Token+Origin验证 |
| 4 | 命令注入 | `firewall/detectors/` | 95% | 参数白名单 |
| 5 | 敏感数据泄露 | `security/waf/detectors/sensitive.go` | 100% | 身份证/手机/密码检测 |
| 6 | 不安全反序列化 | `firewall/detectors/` | 95% | 格式验证+白名单 |
| 7 | XXE防护 | `firewall/detectors/` | 95% | XML解析限制 |
| 8 | 路径遍历 | `firewall/detectors/url_path.go` | 95% | URL规范化检查 |

### 2.2 访问控制与认证

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 9 | JWT 令牌认证 | `auth/jwt.go`, `auth/middleware.go` | 85% | HS256/RS256 |
| 10 | 2FA 双因素认证 | `auth/2fa.go`, `auth/totp_manager.go` | 90% | TOTP (Google Auth) |
| 11 | 用户管理 | `auth/user.go` | 80% | 本地+Redis存储 |
| 12 | 首次运行检测 | `controllers/auth_controller.go` | 85% | 初始化引导 |

### 2.3 WAF 检测引擎

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 13 | 12种检测器链 | `firewall/engine.go`, `firewall/detectors/` | 95% | UA/IP/限流/GeoIP/请求头/方法/URL/完整性/行为/异常/自定义 |
| 14 | 黑白名单管理 | `firewall/detectors/blacklist_test.go` | 100% | IP+规则黑/白名单 |
| 15 | 速率限制 | `security/ratelimit/ratelimit.go` | 97.3% | 滑动窗口+令牌桶 |

---

## 三、预渲染引擎功能 (12项)

### 3.1 核心渲染

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 1 | Headless Chromium渲染 | `prerender/engine.go` | 63% | chromedp 实现 |
| 2 | 多站点独立引擎 | `prerender/engine_manager.go` | — | 每站点独立渲染池 |
| 3 | 爬虫智能识别 | `prerender/crawler.go`, `crawler/detector.go` | 85% | UA+行为模式 |
| 4 | 实例池管理 | `prerender/pool/pool.go` | 75% | 2-10动态伸缩 |
| 5 | 优先级渲染队列 | `prerender/queue.go` | 84.9% | 4级 (Low/Normal/High/VIP) |

### 3.2 缓存优化

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 6 | Redis 缓存 | `internal/cache/manager.go` | 70.9% | go-redis SET/GET/DEL |
| 7 | 缓存元数据 | `internal/cache/manager.go` | 70.9% | Redis Hash 存储 |
| 8 | 缓存预热 | `prerender/cache/preheater.go` | — | Sitemap批量预热 |

### 3.3 高级渲染

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 9 | 流式渲染(首屏) | `prerender/streaming/first_screen.go` | 83.2% | 首屏快速输出 |
| 10 | 分块流式渲染 | `prerender/streaming/chunked.go` | 83.2% | 渐进式HTML |
| 11 | DOM差异增量更新 | `prerender/incremental/dom_diff.go` | 88.4% | 变更部分重新渲染 |
| 12 | 选择性渲染 | `prerender/incremental/selective.go` | 88.4% | 仅渲染变更组件 |

### 3.4 并发控制

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 13 | 动态并发管理器 | `prerender/concurrency_manager.go` | 90% | 基于成功率动态调整 |
| 14 | 渲染优化器 | `prerender/optimizer/optimizer.go` | 55.9% | 渲染策略优化 |

---

## 四、SSL/TLS 证书管理 (6项)

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 1 | ACME 账户注册 | `ssl/acme_client.go` | 50.2% | LEGO库集成 |
| 2 | HTTP-01 挑战 | `ssl/http_challenge.go` | — | 端口80验证 |
| 3 | DNS-01 挑战 | `ssl/dns_challenge.go` | — | DNS TXT记录 |
| 4 | 自动续签+重试 | `ssl/auto_renew.go` | — | 30天前续签 |
| 5 | 通配符证书 | `ssl/dns_challenge.go` | — | DNS-01必需 |
| 6 | Webhook通知 | `ssl/auto_renew.go` | — | HTTP POST通知 |

---

## 五、监控与遥测 (8项)

| # | 功能 | 实现文件 | 覆盖率 | 说明 |
|---|------|---------|--------|------|
| 1 | Prometheus 指标 | `monitoring/metrics.go` | 78.5% | HTTP/业务指标 |
| 2 | 健康检查/状态 | `monitoring/health_checker.go` | 78.5% | Redis/SSL/系统 |
| 3 | 告警系统 | `monitoring/alerting/` | 89.6% | 规则+通知 |
| 4 | 监控仪表盘 | `monitoring/dashboard/` | 95.9% | 可视化 |
| 5 | OTel 分布式追踪 | `monitoring/telemetry/tracer.go` | 84.3% | OTLP导出 |
| 6 | 指标导出器 | `monitoring/telemetry/exporter.go` | 84.3% | OTLP/PromRW/文件 |
| 7 | 遥测中间件 | `monitoring/telemetry/middleware.go` | 84.3% | Gin中间件集成 |
| 8 | 系统资源监控 | `monitoring/monitor.go` | 63.9% | CPU/内存/goroutine |

---

## 六、管理控制台 (11个页面)

| # | 页面 | API端点 | 功能 |
|---|------|---------|------|
| 1 | 概览仪表盘 | `GET /api/v1/overview` | 流量/安全/渲染状态 |
| 2 | 站点管理 | `GET/POST/PUT/DELETE /api/v1/sites` | 多站点 CRUD |
| 3 | WAF配置 | `GET/PUT /api/v1/sites/:id/waf` | 防火墙规则管理 |
| 4 | 攻击日志 | `GET /api/v1/firewall/attacks` | 安全事件查看 |
| 5 | 黑白名单 | `POST /api/v1/firewall/whitelist|blacklist` | IP管理 |
| 6 | 爬虫日志 | `GET /api/v1/crawler/logs|stats` | 爬虫请求分析 |
| 7 | 预热管理 | `POST /api/v1/preheat/trigger` | 手动/定时预热 |
| 8 | SSL证书 | SSL管理API | 申请/续签/删除 |
| 9 | 监控统计 | `GET /api/v1/monitoring/stats` | 实时数据 |
| 10 | 访问日志 | `GET /api/v1/logs` | 请求日志查询 |
| 11 | 系统配置 | `GET/PUT /api/v1/system/config` | 系统设置 |

---

## 七、基础设施功能 (10项)

| # | 功能 | 实现位置 | 说明 |
|---|------|---------|------|
| 1 | 反向代理转发 | `internal/proxy/` | 请求代理到源站 |
| 2 | 请求路由 | `internal/routing/` | 站点级路由分发 |
| 3 | Redis缓存 | `internal/redis/` | 连接池/发布订阅 |
| 4 | YAML配置/热更新 | `internal/config/` | fsnotify文件监控 |
| 5 | Cron定时任务 | `internal/scheduler/` | 预热/续签/清理 |
| 6 | 任务队列 | `internal/task/` | 异步任务管理 |
| 7 | GeoIP地理位置 | `internal/services/geoip.go` | 请求来源定位 |
| 8 | 结构化日志 | `internal/logging/` | zap日志+爬虫/访问日志 |
| 9 | 国际化i18n | `internal/i18n/` | 多语言支持 |
| 10 | 多平台构建 | `build.sh` | 8种平台二进制 |

---

## 八、功能覆盖统计

| 功能域 | 计划数 | 已实现 | 覆盖率 |
|--------|--------|--------|--------|
| 安全防护 | 15 | 15 | 100% |
| 预渲染引擎 | 14 | 14 | 100% |
| SSL/TLS | 6 | 6 | 100% |
| 监控遥测 | 8 | 8 | 100% |
| 管理控制台 | 11 | 11 | 100% |
| 基础设施 | 10 | 10 | 100% |
| **合计** | **64** | **64** | **100%** |
