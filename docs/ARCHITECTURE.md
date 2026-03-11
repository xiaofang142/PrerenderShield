# Prerender Shield 项目架构文档

**版本:** 2.0  
**更新日期:** 2026-03-08  
**项目位置:** /Users/xiaofang/Documents/www/prerender/prerender-shield

---

## 📋 目录

1. [项目概述](#项目概述)
2. [技术栈](#技术栈)
3. [目录结构](#目录结构)
4. [核心模块详解](#核心模块详解)
5. [API 接口文档](#api 接口文档)
6. [数据流图](#数据流图)
7. [部署架构](#部署架构)

---

## 项目概述

Prerender Shield 是一个高性能的网站预渲染和 WAF 防护系统，主要功能包括：

- **预渲染引擎**: 为搜索引擎爬虫提供预渲染的 HTML 内容
- **WAF 防火墙**: 防护恶意爬虫和攻击
- **多站点管理**: 支持多站点独立配置和运行
- **实时监控**: Prometheus 指标收集和 health check
- **缓存管理**: Redis 缓存加速
- **SSL 证书管理**: 自动 SSL 证书处理
- **管理控制台**: React 前端管理界面

### 核心特性

| 特性 | 描述 |
|------|------|
| 多站点支持 | 每个站点独立配置、独立端口运行 |
| 预渲染池 | 每个站点独立的渲染引擎池 |
| 智能爬虫识别 | 基于 User-Agent、行为特征的爬虫检测 |
| WAF 防护 | 规则引擎、速率限制、地理位置封锁 |
| 实时日志 | 爬虫日志、访问日志、攻击日志 |
| 配置热更新 | 支持配置文件和 Redis 配置同步 |

---

## 技术栈

### 后端 (Go)

| 组件 | 版本/说明 |
|------|----------|
| Go | 1.21+ |
| Gin | Web 框架 |
| Redis | 缓存和数据存储 |
| Prometheus | 监控指标 |
| JWT | 认证令牌 |
| YAML | 配置文件格式 |

### 前端 (React)

| 组件 | 版本/说明 |
|------|----------|
| React | 18.x |
| TypeScript | 类型系统 |
| Vite | 构建工具 |
| Ant Design | UI 组件库 |
| Playwright | E2E 测试 |

### 基础设施

| 组件 | 用途 |
|------|------|
| Docker | 容器化部署 |
| Kubernetes | 编排部署 (deploy/k8s) |
| Redis | 缓存/会话/配置存储 |
| Prometheus | 监控指标收集 |

---

## 目录结构

```
prerender-shield/
├── cmd/
│   └── api/                    # 主程序入口
│       ├── main.go             # 应用程序启动和初始化
│       ├── certs/              # SSL 证书存储
│       ├── data/               # 数据文件存储
│       └── static/             # 静态资源
├── internal/                   # 内部包 (核心业务逻辑)
│   ├── api/
│   │   ├── controllers/        # API 控制器 (11 个控制器)
│   │   │   ├── auth.go         # 认证控制器
│   │   │   ├── sites.go        # 站点管理控制器
│   │   │   ├── firewall.go     # 防火墙控制器
│   │   │   ├── preheat.go      # 预热控制器
│   │   │   ├── push.go         # 推送控制器
│   │   │   ├── crawler.go      # 爬虫日志控制器
│   │   │   ├── monitoring.go   # 监控控制器
│   │   │   ├── overview.go     # 概览控制器
│   │   │   ├── system.go       # 系统控制器
│   │   │   └── ...
│   │   └── routes/             # 路由配置
│   │       ├── routes.go       # 路由注册入口
│   │       ├── route_registration.go  # 路由定义
│   │       ├── controller_setup.go    # 控制器初始化
│   │       └── common.go       # 通用中间件
│   ├── auth/                   # 认证模块
│   │   ├── jwt.go              # JWT 令牌管理
│   │   ├── user.go             # 用户管理
│   │   └── middleware.go       # 认证中间件
│   ├── cache/                  # 缓存管理
│   │   └── manager.go          # 缓存管理器
│   ├── config/                 # 配置管理
│   │   ├── config.go           # 配置加载和热更新
│   │   └── config_test.go      # 配置测试
│   ├── crawler/                # 爬虫检测
│   │   └── detector.go         # 爬虫特征检测
│   ├── firewall/               # WAF 防火墙
│   │   ├── engine.go           # 防火墙引擎
│   │   ├── action.go           # 防护动作
│   │   ├── detectors/          # 检测器 (12 种)
│   │   └── types/              # 类型定义
│   ├── i18n/                   # 国际化
│   │   └── i18n.go             # 多语言支持
│   ├── logging/                # 日志管理
│   │   ├── log.go              # 基础日志
│   │   ├── crawler_log.go      # 爬虫日志
│   │   └── visit_log.go        # 访问日志
│   ├── middleware/             # 中间件
│   │   ├── error.go            # 错误处理
│   │   └── waf.go              # WAF 中间件
│   ├── models/                 # 数据模型
│   │   ├── site.go             # 站点模型
│   │   └── waf.go              # WAF 模型
│   ├── monitoring/             # 监控模块
│   │   ├── monitor.go          # 监控主逻辑
│   │   ├── metrics.go          # Prometheus 指标
│   │   └── health_checker.go   # 健康检查
│   ├── plugin/                 # 插件系统
│   │   └── plugin.go           # 插件管理
│   ├── prerender/              # 预渲染引擎
│   │   ├── engine.go           # 渲染引擎
│   │   ├── engine_manager.go   # 引擎管理器
│   │   ├── crawler.go          # 爬虫逻辑
│   │   ├── preheat.go          # 预热逻辑
│   │   ├── preheat/            # 预热子模块
│   │   └── push/               # 推送子模块
│   ├── proxy/                  # 反向代理
│   │   ├── proxy.go            # 代理逻辑
│   │   └── proxy_test.go       # 代理测试
│   ├── redis/                  # Redis 客户端
│   │   ├── client.go           # Redis 客户端封装
│   │   ├── client_test.go      # 客户端测试
│   │   └── subscriber.go       # Redis 订阅
│   ├── repository/             # 数据仓库
│   │   ├── site_repository.go  # 站点仓库
│   │   └── waf_repository.go   # WAF 仓库
│   ├── routing/                # 路由逻辑
│   │   └── router.go           # 请求路由
│   ├── scheduler/              # 定时任务
│   │   └── scheduler.go        # 调度器
│   ├── services/               # 业务服务
│   │   ├── geoip.go            # GeoIP 服务
│   │   ├── domain_resolver.go  # 域名解析
│   │   └── log_processor.go    # 日志处理
│   ├── site-handler/           # 站点处理器
│   │   ├── handler.go          # HTTP 处理器
│   │   └── handler_test.go     # 处理器测试
│   ├── site-server/            # 站点服务器
│   │   ├── manager.go          # 服务器管理器
│   │   └── manager_test.go     # 管理器测试
│   ├── ssl/                    # SSL 证书管理
│   │   ├── manager.go          # SSL 管理器
│   │   └── manager_test.go     # SSL 测试
│   ├── storage/                # 存储抽象
│   └── task/                   # 任务队列
│       └── queue.go            # 任务队列管理
├── pkg/                        # 公共包
│   ├── http/                   # HTTP 工具
│   ├── logger/                 # 日志工具
│   ├── metrics/                # 指标工具
│   └── security/               # 安全工具
├── web/                        # 前端管理控制台
│   ├── src/
│   │   ├── components/         # React 组件
│   │   ├── pages/              # 页面 (11 个)
│   │   ├── services/           # API 服务
│   │   ├── store/              # 状态管理
│   │   ├── types/              # TypeScript 类型
│   │   ├── utils/              # 工具函数
│   │   ├── locales/            # 国际化
│   │   └── constants/          # 常量
│   ├── public/                 # 静态资源
│   ├── tests/                  # 前端测试
│   └── playwright-report/      # Playwright 报告
├── tests/                      # 后端测试
│   ├── unit/                   # 单元测试
│   ├── integration/            # 集成测试
│   └── e2e/                    # 端到端测试
├── deploy/                     # 部署配置
│   ├── k8s/                    # Kubernetes 配置
│   └── scripts/                # 部署脚本
├── docker/                     # Docker 配置
├── configs/                    # 配置文件
├── docs/                       # 文档
├── static/                     # 静态资源
├── data/                       # 数据文件
├── certs/                      # SSL 证书
└── bin/                        # 编译输出
```

### 代码统计

| 类别 | 文件数 | 代码行数 |
|------|--------|---------|
| Go 源代码 | 79 | ~18,723 |
| 核心模块 | 12 | - |
| API 控制器 | 11 | - |
| 测试文件 | 15+ | - |
| React 组件 | 50+ | - |

---

## 核心模块详解

### 1. 主程序 (cmd/api/main.go)

**职责:** 应用程序启动、模块初始化、服务编排

**启动流程:**
```
1. 加载配置文件 (YAML)
2. 启动配置热更新监控
3. 初始化 Redis 客户端
4. 初始化认证模块 (UserManager, JWTManager)
5. 初始化防火墙引擎管理器
6. 初始化缓存管理器
7. 初始化预渲染引擎管理器
8. 初始化日志管理器 (爬虫日志、访问日志)
9. 初始化 GeoIP 服务和日志处理器
10. 为每个站点创建渲染引擎和防火墙
11. 启动定时任务调度器
12. 初始化健康检查器和监控模块
13. 启动站点服务器 (每个站点独立端口)
14. 启动 API 服务器 (Gin)
15. 启动管理控制台服务器
16. 等待信号，优雅关闭
```

**关键配置:**
- 配置文件路径：通过 `-config` 参数指定
- Redis 连接：支持 URL 格式，包含密码和 DB 索引
- 多站点配置：从文件或 Redis 加载

---

### 2. 配置管理 (internal/config)

**核心文件:** `config.go` (39,499 字节)

**功能:**
- YAML 配置文件加载
- 配置热更新 (文件监控)
- Redis 配置同步
- 多站点配置管理

**配置结构:**
```go
type Config struct {
    Server   ServerConfig   // 服务器配置
    Cache    CacheConfig    // 缓存配置
    Sites    []SiteConfig   // 站点配置列表
    Dirs     DirConfig      // 目录配置
    // ...
}

type SiteConfig struct {
    ID           string          // 站点 ID
    Name         string          // 站点名称
    Domains      []string        // 域名列表
    Port         int             // 服务端口
    Mode         string          // 运行模式 (static/prerender)
    Firewall     FirewallConfig  // 防火墙配置
    Prerender    PrerenderConfig // 预渲染配置
    Push         PushConfig      // 推送配置
    // ...
}
```

**热更新机制:**
- 使用 fsnotify 监控配置文件变化
- 配置变化时自动重新加载
- 支持 Redis 配置优先于文件配置

---

### 3. 预渲染引擎 (internal/prerender)

**核心文件:**
- `engine.go` - 渲染引擎实现
- `engine_manager.go` - 引擎管理器
- `crawler.go` - 爬虫逻辑
- `preheat.go` - 预热逻辑

**架构:**
```
EngineManager (全局管理器)
    └── Engine (每站点一个引擎)
        ├── 渲染池 (PoolSize 个渲染实例)
        ├── 缓存 (CacheManager)
        └── 爬虫 (Crawler)
```

**工作流程:**
1. 接收请求，检查爬虫标识
2. 查询缓存，命中则返回
3. 未命中则启动渲染
4. 渲染结果缓存并返回

**预热功能:**
- 定时触发站点 URL 预渲染
- 支持手动触发预热
- 预热任务状态追踪

---

### 4. WAF 防火墙 (internal/firewall)

**核心文件:** `engine.go` (16,042 字节)

**检测器列表 (12 种):**
1. User-Agent 检测
2. IP 黑名单检测
3. IP 白名单检测
4. 速率限制检测
5. 地理位置检测
6. 请求头检测
7. 请求方法检测
8. URL 路径检测
9. 文件完整性检测
10. 爬虫行为检测
11. 异常流量检测
12. 自定义规则检测

**防护动作:**
- Allow: 放行请求
- Block: 阻断请求
- Challenge: 挑战验证
- Log: 记录日志
- RateLimit: 速率限制

**工作流程:**
```
请求 → 检测器链 → 评分 → 动作执行
           ↓
        日志记录
```

---

### 5. API 路由 (internal/api)

**路由结构:**
```
/api/v1
├── /auth                    # 认证 (公开)
│   ├── GET  /first-run      # 检查首次运行
│   ├── POST /login          # 用户登录
│   └── POST /logout         # 用户登出
├── /health                  # 健康检查 (公开)
├── /version                 # 版本信息 (公开)
└── [JWT 保护]               # 需要认证
    ├── /system/config       # 系统配置
    ├── /overview            # 概览数据
    ├── /monitoring/stats    # 监控统计
    ├── /logs                # 访问日志
    ├── /firewall/attacks    # 攻击日志
    ├── /firewall/whitelist  # 白名单管理
    ├── /firewall/blacklist  # 黑名单管理
    ├── /crawler/logs        # 爬虫日志
    ├── /crawler/stats       # 爬虫统计
    ├── /preheat/*           # 预热管理
    ├── /push/*              # 推送管理
    └── /sites/*             # 站点管理
        ├── GET    /         # 站点列表
        ├── GET    /:id      # 站点详情
        ├── POST   /         # 创建站点
        ├── PUT    /:id      # 更新站点
        ├── DELETE /:id      # 删除站点
        ├── GET    /:id/waf  # WAF 配置
        ├── PUT    /:id/waf  # 更新 WAF
        ├── GET    /:id/static     # 静态文件列表
        ├── POST   /:id/static     # 上传文件
        ├── DELETE /:id/static     # 删除文件
        └── ...
```

**控制器列表 (11 个):**
1. AuthController - 认证
2. SystemController - 系统配置
3. OverviewController - 概览
4. MonitoringController - 监控
5. FirewallController - 防火墙
6. CrawlerController - 爬虫日志
7. PreheatController - 预热
8. PushController - 推送
9. SitesController - 站点管理
10. SSLController - SSL 证书
11. LogsController - 日志管理

---

### 6. 认证模块 (internal/auth)

**组件:**
- `user.go` - 用户管理 (本地存储 + Redis)
- `jwt.go` - JWT 令牌生成和验证
- `middleware.go` - JWT 认证中间件

**认证流程:**
```
登录 → 验证凭据 → 生成 JWT → 返回令牌
           ↓
请求 → 验证 JWT → 提取用户信息 → 处理请求
```

**令牌配置:**
- 密钥：从配置文件读取
- 有效期：24 小时
- 存储：Redis (支持吊销)

---

### 7. 监控模块 (internal/monitoring)

**核心文件:** `monitor.go` (16,838 字节)

**功能:**
- Prometheus 指标暴露 (:9090)
- 健康检查 (Redis、渲染引擎、站点服务器)
- 实时统计 (请求数、缓存命中率、攻击次数)

**健康检查项:**
1. Redis 连接状态
2. 预渲染引擎状态
3. 站点服务器状态
4. 防火墙状态
5. 缓存状态

**指标类型:**
- Counter: 请求计数、攻击计数
- Gauge: 活跃连接数、缓存大小
- Histogram: 响应时间分布

---

### 8. 站点管理 (internal/site-handler & internal/site-server)

**职责:**
- 为每个站点创建独立的 HTTP 服务器
- 处理站点级别的请求路由
- 集成 WAF、预渲染、日志记录

**站点服务器管理器:**
```go
type Manager struct {
    servers map[string]*http.Server  // 站点服务器映射
    monitor *Monitor                  // 监控实例
}
```

**请求处理流程:**
```
请求 → 站点路由 → WAF 检测 → 爬虫识别 → 预渲染/静态 → 响应
                      ↓          ↓           ↓
                   攻击日志   爬虫日志    缓存
```

---

### 9. Redis 客户端 (internal/redis)

**核心文件:** `client.go` (16,715 字节)

**功能:**
- Redis 连接管理
- 键值操作封装
- 发布订阅支持
- 连接池管理

**用途:**
- 缓存存储
- 会话管理
- 配置存储
- 日志队列
- 分布式锁

---

### 10. 日志管理 (internal/logging)

**日志类型:**
1. **爬虫日志** (`crawler_log.go`): 记录爬虫请求详情
2. **访问日志** (`visit_log.go`): 记录所有访问请求
3. **系统日志** (`log.go`): 系统运行日志

**日志处理器** (`services/log_processor.go`):
- 异步处理日志
- GeoIP 信息 enrich
- 日志聚合统计

---

### 11. 定时任务 (internal/scheduler)

**核心文件:** `scheduler.go` (7,676 字节)

**任务类型:**
1. 预渲染预热任务
2. SSL 证书更新检查
3. 缓存清理任务
4. 日志归档任务
5. 健康检查任务

**调度策略:**
- 基于 cron 表达式
- 支持动态添加/删除任务
- 任务执行状态追踪

---

### 12. SSL 证书管理 (internal/ssl)

**核心文件:** `manager.go` (13,908 字节)

**功能:**
- SSL 证书加载和验证
- 证书过期检查
- 自动续期支持
- HTTPS 配置

---

## API 接口文档

### 认证接口

#### 1. 检查首次运行
```
GET /api/v1/auth/first-run
Response: { "firstRun": true/false }
```

#### 2. 用户登录
```
POST /api/v1/auth/login
Body: { "username": "admin", "password": "xxx" }
Response: { "token": "jwt_token", "expiresIn": 86400 }
```

#### 3. 用户登出
```
POST /api/v1/auth/logout
Headers: Authorization: Bearer <token>
```

### 系统接口

#### 4. 健康检查
```
GET /api/v1/health
Response: { "status": "healthy", "checks": {...} }
```

#### 5. 版本信息
```
GET /api/v1/version
Response: { "version": "1.0.0", "buildTime": "..." }
```

#### 6. 系统配置
```
GET /api/v1/system/config
PUT /api/v1/system/config
```

### 站点管理接口

#### 7. 获取站点列表
```
GET /api/v1/sites
Response: [{ "id": "...", "name": "...", "domains": [...], ... }]
```

#### 8. 获取站点详情
```
GET /api/v1/sites/:id
```

#### 9. 创建站点
```
POST /api/v1/sites
Body: { "name": "...", "domains": [...], "port": 8080, ... }
```

#### 10. 更新站点
```
PUT /api/v1/sites/:id
Body: { "name": "...", ... }
```

#### 11. 删除站点
```
DELETE /api/v1/sites/:id
```

#### 12. WAF 配置
```
GET /api/v1/sites/:id/waf
PUT /api/v1/sites/:id/waf
```

#### 13. 静态文件管理
```
GET /api/v1/sites/:id/static
POST /api/v1/sites/:id/static (上传)
DELETE /api/v1/sites/:id/static (删除)
POST /api/v1/sites/:id/static/extract (解压)
POST /api/v1/sites/:id/static/batch-delete (批量删除)
```

### 防火墙接口

#### 14. 获取攻击日志
```
GET /api/v1/firewall/attacks
Query: page, limit, startTime, endTime
```

#### 15. 添加白名单
```
POST /api/v1/firewall/whitelist
Body: { "ip": "1.2.3.4", "reason": "..." }
```

#### 16. 添加黑名单
```
POST /api/v1/firewall/blacklist
Body: { "ip": "1.2.3.4", "reason": "..." }
```

### 爬虫日志接口

#### 17. 获取爬虫日志
```
GET /api/v1/crawler/logs
```

#### 18. 获取爬虫统计
```
GET /api/v1/crawler/stats
```

### 预热接口

#### 19. 获取预热站点
```
GET /api/v1/preheat/sites
```

#### 20. 获取预热统计
```
GET /api/v1/preheat/stats
```

#### 21. 触发预热
```
POST /api/v1/preheat/trigger
Body: { "siteId": "...", "urls": [...] }
```

#### 22. 获取预热 URL 列表
```
GET /api/v1/preheat/urls
```

#### 23. 获取预热任务状态
```
GET /api/v1/preheat/task/status
```

#### 24. 获取爬虫请求头配置
```
GET /api/v1/preheat/crawler-headers
```

#### 25. 清除缓存
```
POST /api/v1/preheat/clear-cache
Body: { "siteId": "...", "urls": [...] }
```

### 推送接口

#### 26. 获取推送站点
```
GET /api/v1/push/sites
```

#### 27. 获取推送统计
```
GET /api/v1/push/stats
```

#### 28. 获取推送日志
```
GET /api/v1/push/logs
```

#### 29. 获取推送趋势
```
GET /api/v1/push/trend
```

#### 30. 获取推送配置
```
GET /api/v1/push/config
PUT /api/v1/push/config
```

### 监控接口

#### 31. 获取监控统计
```
GET /api/v1/monitoring/stats
```

### 日志接口

#### 32. 获取访问日志
```
GET /api/v1/logs
Query: page, limit, startTime, endTime, type
```

### 概览接口

#### 33. 获取概览数据
```
GET /api/v1/overview
Response: {
    "totalSites": 5,
    "totalRequests": 10000,
    "attackCount": 50,
    "cacheHitRate": 0.85,
    ...
}
```

---

## 数据流图

### 请求处理流程

```
                    ┌─────────────────┐
                    │   Client Request │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  Site Server    │
                    │  (port-based)   │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │   WAF Engine    │
                    │  (12 detectors) │
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼ (blocked)                   ▼ (allowed)
    ┌─────────────────┐           ┌─────────────────┐
    │  Block Response │           │ Crawler Detector│
    └─────────────────┘           └────────┬────────┘
                                           │
                          ┌────────────────┴────────────────┐
                          │                                │
                          ▼ (crawler)                      ▼ (normal)
                ┌─────────────────┐              ┌─────────────────┐
                │ Prerender Engine│              │  Static Files   │
                │   (cache check) │              │   or Proxy      │
                └────────┬────────┘              └────────┬────────┘
                         │                                │
                         └────────────────┬───────────────┘
                                          │
                                          ▼
                                 ┌─────────────────┐
                                 │  Log Recording  │
                                 │ (crawler/visit) │
                                 └────────┬────────┘
                                          │
                                          ▼
                                 ┌─────────────────┐
                                 │   Response      │
                                 └─────────────────┘
```

### 配置同步流程

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Config File│────▶│ ConfigMgr   │◀───▶│    Redis    │
│   (YAML)    │watch│  (hot-reload)│sync │  (backup)   │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
                           ▼
                  ┌────────────────┐
                  │ Site Servers   │
                  │ (reconfigure)  │
                  └────────────────┘
```

### 监控数据流

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  App Metrics│────▶│  Monitor    │────▶│ Prometheus  │
│ (counters,  │     │  (collector)│     │   (:9090)   │
│  gauges)    │     │             │     │             │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
                           ▼
                  ┌────────────────┐
                  │ Health Checker │
                  │  (periodic)    │
                  └────────────────┘
```

---

## 部署架构

### 单机部署

```
┌─────────────────────────────────────────────────────┐
│                    Host Machine                      │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │   API Svr   │  │  Console    │  │  Site Svr   │ │
│  │  (:8080)    │  │  (:3000)    │  │  (:8081)    │ │
│  └─────────────┘  └─────────────┘  └─────────────┘ │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │  Prerender  │  │    WAF      │  │   Cache     │ │
│  │   Engine    │  │   Engine    │  │  (Redis)    │ │
│  └─────────────┘  └─────────────┘  └─────────────┘ │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐                   │
│  │  Scheduler  │  │  Monitor    │                   │
│  │             │  │  (:9090)    │                   │
│  └─────────────┘  └─────────────┘                   │
└─────────────────────────────────────────────────────┘
```

### Kubernetes 部署

参见 `deploy/k8s/` 目录:
- Deployment 配置
- Service 配置
- ConfigMap 配置
- Ingress 配置

---

## 附录

### A. 配置文件示例

```yaml
server:
  address: "0.0.0.0"
  apiPort: 8080
  consolePort: 3000

cache:
  redisURL: "redis://localhost:6379"
  redisPassword: ""
  redisDB: 0

sites:
  - id: "site-1"
    name: "Example Site"
    domains:
      - "example.com"
      - "www.example.com"
    port: 8081
    mode: "prerender"
    prerender:
      enabled: true
      poolSize: 5
      timeout: 30
      cacheTTL: 3600
    firewall:
      enabled: true
      rulesPath: "./configs/waf-rules.yaml"
```

### B. 开发命令

```bash
# 构建
go build -o bin/api ./cmd/api

# 运行
./bin/api -config configs/config.yaml

# 测试
go test ./...

# 前端开发
cd web && npm run dev

# 前端构建
cd web && npm run build
```

### C. 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| CONFIG_PATH | 配置文件路径 | "" |
| REDIS_URL | Redis 连接 URL | redis://localhost:6379 |
| LOG_LEVEL | 日志级别 | info |

---

**文档结束**
