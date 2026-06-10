# Changelog

All notable changes to Prerender Shield will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.0.0] - 2026-06-10

### Added
- Redis-only architecture: removed MemoryCache, all data goes through Redis
- Config encryption: AES-256-GCM for sensitive fields (`internal/crypto/`)
- Audit logging: 18 operation types with Redis persistence (`internal/audit/`)
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
