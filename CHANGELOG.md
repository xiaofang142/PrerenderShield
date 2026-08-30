# Changelog

All notable changes to Prerender Shield will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 爬虫响应条件请求（304）：200 响应携带弱 ETag（W/"sha256前16hex"）与 Last-Modified；命中 If-None-Match / If-Modified-Since 返回 304 无 body（Googlebot 原生支持，prerender.io/Rendertron 均未实现的爬虫带宽优化）
- 爬虫响应 gzip 压缩：渲染 HTML >1KB 且客户端 Accept-Encoding 含 gzip 时自动压缩（stdlib 实现，Vary: Accept-Encoding）
- 缓存 TTL 分级规则 `ttl_rules`：URL pattern（子串/`*` 通配）→ 站点 CacheTTL → 引擎默认 24h，首中生效；实时/预热/重渲三通道统一接线；控制台 Prerender 页可视编辑（最多 10 条，60..2592000 秒）
- per-URL 渲染预算报表：`GET /crawler/url-stats` + 控制台爬虫页新「渲染预算报表」Tab（请求数/实渲染次数/命中率/平均耗时/渲染耗时合计/末次状态）——定位"哪些 URL 在烧渲染算力"

### Changed
- 缓存设备分桶收敛为单桶：渲染器输出与 UA 无关，mobile/desktop 爬虫共用 @desktop 键（修复 mobile 爬虫永不命中预热缓存、双倍渲染的纯浪费）；存量 @mobile/无后缀旧键读取自动回退，随 TTL 自然过期（调研依据：Google 移动优先索引官方建议响应式站直发桌面 HTML）
- 爬虫日志 Status 如实记录实际响应状态码（304 时记 304）
- 缓存条目管理 API：`POST /preheat/invalidate`（单 URL 失效）、`POST /preheat/recache`（强制重渲替换）、`GET /preheat/entries`（条目列表）、`DELETE /preheat/entries`（单条删除）；修复 per-site clear-cache 实际全局误删的问题
- 缓存键设备分桶：UA 判定 desktop/mobile 进入缓存键（`@mobile`/`@desktop` 后缀），移动版爬虫命中移动版 HTML；desktop 读取对存量无后缀旧键一次性回退兼容
- 管理 API Token：系统设置页生成/吊销（sha256 哈希存储，原文仅展示一次），`Authorization: Bearer` 回退鉴权**仅限 `/api/v1/preheat/*`**，CI 发布钩子可自动化刷缓存
- IndexNow key file 自动托管：站点处理器在 WAF 之前拦截 `GET /{key}.txt` 应答验证内容，并同步写入静态根目录，搜索引擎所有权验证零配置
- 爬虫真实性验证（bot_verify，默认关闭）：Google 官方 rDNS 双向验证流程，结果（verified/unverified）写入爬虫日志 `verified` 字段，log-only 不拦截；singleflight 去重 + 磁盘 LRU 缓存（正 7d/负 1h，`PRERENDER_BOTVERIFY_CACHE` 可覆盖路径）
- 渲染策略设置 UI（Prerender 页）：cache_ttl / timeout / max_concurrency / include_patterns / exclude_patterns / stale_while_revalidate / category_policy 四分类策略表单
- 缓存条目 UI（Preheat 页）：条目表格（状态码/设备/新鲜度/TTL 剩余/大小）+ 单条失效/重渲操作 + URL 过滤
- 官网 Features 新增「技术选型边界」小节（动态渲染 vs SSR/SSG 场景对照表，诚实引用 Google 官方建议）与 AEO AI 引擎支持卡片（中英双语）
- 爬虫 UA 库新增 AI 引擎分类覆盖（gptbot/claudebot/perplexitybot/ccbot/applebot/bytespider/amazonbot 等此前已并入，本轮补全文档与官网叙事）

### Changed
- 渲染策略 PUT `/sites/:id/prerender` 为整段提交，控制台保存时合并未编辑字段避免清零
- 系统配置保存改为整段合并提交（`system:config` 为全量替换语义，修复只传表单字段会丢失其余键的隐患）

### Removed
- 删除零引用死代码包：`internal/prerender/streaming`、`internal/prerender/incremental`、`internal/prerender/optimizer`

### Fixed
- prerender 包测试在多二进制并发时的 Chromium 进程帽争抢 flaky（TestMain 放开 `PRERENDER_PROCESS_CAP`），连续 3 轮全绿

## [3.0.0] - 2026-06-10

### Added
- Redis-only architecture: removed MemoryCache, all data goes through Redis
- Config encryption: AES-256-GCM for sensitive fields (`internal/crypto/`)
- Audit logging: 14 operation types with Redis persistence (`internal/audit/`)
- Multi-channel alerting: DingTalk, WeChat Work, Slack, Feishu, Email (`internal/monitoring/alerting/channels.go`)
- Request coalescer: deduplicate concurrent render requests (`internal/prerender/request_coalescer.go`)
- Persistent task queue: Redis-backed render queue (`internal/prerender/persistent_queue.go`)
- Cache hit statistics: per-URL hit tracking with hourly aggregation (`internal/prerender/cache/stats.go`)
- Prometheus pre-aggregated metrics: cache hit rate, WAF block rate, render success rate
- Unified API error response format: `{code, message, data}` across all controllers
- RBAC permission system (4 roles, 16 permissions) (`internal/auth/rbac.go`)
- Nginx reverse proxy config (`deploy/nginx/prerender-shield.conf`)
- Kubernetes deployment manifests (`deploy/k8s/`)
- Environment variable documentation (`docs/ENV_VARS.md`)
- Frontend TypeScript API types (`web/src/types/api.ts`)
- World map data bundled for offline use (`web/public/maps/world.json`)

### Changed
- **Logging overhaul**: 97 instances of `fmt.Printf`/`log.Printf` → `logging.DefaultLogger`
- Telemetry coverage: 58.6% → 84.3% (43 new tests across all telemetry packages)
- SSL controller responses standardized to `{code, message, data}` format
- Firewall controller responses standardized to `{code, message, data}` format
- CORS security: `Access-Control-Allow-Origin: *` → echo origin with credentials
- CSP security: removed hardcoded dev URLs from Content-Security-Policy
- AuthContext: token expiry validation on page load
- BaseChart: ResizeObserver for responsive charts, fixed double setOption
- Frontend API routes: 8 dead routes fixed, `/logs` page implemented
- Navigation menu: added `/prerender` and `/logs` links
- Build scripts: multi-platform support (linux/windows/darwin × amd64/arm64)

### Fixed
- **CRITICAL**: `wire.go` `event.NewInMemoryBus` → `eventbus.NewInMemoryBus` (DI container broken)
- **CRITICAL**: `acme_client.go` dead code path (config created but never used)
- **CRITICAL**: `dns_challenge.go` `os.Setenv` race condition (added mutex)
- **CRITICAL**: `health_checker.go` JSON injection via string concatenation (switched to `json.Marshal`)
- **CRITICAL**: `ssl_controller.go` 8 responses missing `code` field
- **CRITICAL**: `firewall_controller.go` used `"success": true` instead of `"code": 200`
- **CRITICAL**: `overview_controller.go` 4 unsafe type assertions (no `ok` check, would panic)
- **CRITICAL**: `Crawler.tsx` and `Firewall.tsx` passed `site.name` instead of `site.id` to API
- **CRITICAL**: `Prerender.tsx` called deleted `prerenderApi.render()` and `preheat()` methods
- `http_challenge.go`: `log.Printf` → structured logging
- `auto_renew.go`: `sendWebhook` stub → actual HTTP POST implementation
- `Sites.tsx`: delete operation lacked confirmation dialog
- `Logs.tsx`: placeholder page → real access log viewer
- Overview: redundant `useState` for `accessStats` → plain constant

## [2.1.0] - 2026-03-18

### Added
- Complete test coverage for monitoring package (Task #22)
  - `internal/monitoring/metrics_test.go` - Metrics collector tests
  - `internal/monitoring/health_checker_test.go` - Health checker tests
  - Monitoring package coverage improved from 38.4% to 78.5%
- Schema validator tests for security module
  - `internal/security/ratelimit/schema_test.go`
  - Coverage: 92%
- Task queue tests
  - `internal/task/queue_test.go`
  - Coverage: 75%

### Changed
- Updated TEST_REPORT.md with latest coverage statistics
- Updated TESTING.md with best practices and examples
- Improved documentation structure

### Fixed
- Type assertion panic in health_checker.go ServeHTTP method
- Test assertions for IsHealthy with nil Redis client
- Bootstrap test port conflicts (changed hardcoded ports to dynamic port 0)
- Crawler/Overview tests (updated English selectors to Chinese UI language)

## [2.0.0] - 2026-03-08

### Added
- Full-link testing coverage (Task #21)
  - 148 Playwright E2E tests
  - 25+ new integration test scenarios
  - `web/tests/e2e-full-link.test.ts` - Complete workflow tests
  - `web/tests/integration-waf-crawler.test.ts` - WAF and crawler integration
- CI/CD pipeline setup
- Comprehensive test documentation

### Changed
- Monitoring package coverage improved to 38.4%
- Test infrastructure improvements

### Fixed
- E2E test failures in auth and sites modules
- CI/CD pipeline configuration issues

## [1.5.0] - 2026-03-01

### Added
- Rate limiting module with schema validation
- Real-time monitoring dashboard
- SSL certificate management with auto-renewal
- Push notification system

### Changed
- Improved WAF detection accuracy
- Enhanced crawler identification

## [1.0.0] - 2026-02-01

### Added
- Initial release
- OWASP Top 10 protection
- Intelligent prerendering
- Smart traffic routing
- Modern management UI
- Redis-based caching
- Headless Chromium rendering

---

## Version History Summary

| Version | Release Date | Key Highlights |
|---------|--------------|----------------|
| 3.0.0 | 2026-06-10 | Redis-only, audit logging, multi-channel alerts, 48+ fixes |
| 2.1.0 | 2026-03-18 | Test coverage improvements (78.5% monitoring) |
| 2.0.0 | 2026-03-08 | Full-link testing, CI/CD |
| 1.5.0 | 2026-03-01 | Rate limiting, SSL management |
| 1.0.0 | 2026-02-01 | Initial release |

## Testing Statistics

### Current Test Coverage (as of 2026-06-10)

| Module | Coverage | Status |
|--------|----------|--------|
| internal/monitoring | 78.5% | ✅ Good |
| internal/monitoring/alerting | 89.6% | ✅ Good |
| internal/monitoring/telemetry | **84.3%** | ✅ **Improved from 58.6%** |
| internal/monitoring/dashboard | 95.9% | ✅ Excellent |
| internal/security/ratelimit | 97.3% | ✅ Excellent |
| internal/security/waf/detectors | 100% | ✅ Excellent |
| internal/task | 84.9% | ✅ Good |
| internal/firewall/detectors | 95% | ✅ Excellent |
| internal/proxy | 92.9% | ✅ Good |
| internal/routing | 96.4% | ✅ Excellent |

### Test Files

**Go Backend:**
- 60+ packages tested
- Backend: all passing
- New packages tested: `audit`, `constants`

**Frontend (Playwright):**
- 13 test files
- 148 test cases
- All passing

### Packages Without Tests
- `internal/firewall/types` - type definitions only
- `internal/security/waf/types` - type definitions only
