# 技术架构文档 (Technical Architecture)

## 1. 系统概览

PrerenderShield 是一个高性能的静态站点托管与反向代理系统，集成了 SEO 预渲染、WAF 防火墙、访问控制和实时监控功能。

### 核心组件

*   **API Server**: 基于 Gin 框架的 RESTful API，处理配置管理、日志查询等请求。
*   **Site Server**: 动态站点服务管理器，负责启动和管理每个站点的 HTTP 服务（静态文件服务或反向代理）。
*   **Prerender Service**: 预渲染引擎，对接 Chrome Headless 或外部预渲染服务，为爬虫提供静态 HTML。
*   **Firewall (WAF)**: Web 应用防火墙，提供 SQL 注入、XSS、GeoIP 封禁、黑白名单等防护。
*   **Redis**: 用于配置存储、缓存管理、实时状态同步和消息队列。
*   **Frontend**: 基于 React + Ant Design 的管理后台。

## 2. 目录结构

```
prerender-shield/
├── cmd/
│   └── api/            # API Server 入口
├── internal/
│   ├── api/            # API 路由与控制器
│   ├── config/         # 配置管理
│   ├── firewall/       # WAF 核心引擎
│   ├── logging/        # 日志系统
│   ├── middleware/     # Gin 中间件
│   ├── models/         # 数据模型
│   ├── prerender/      # 预渲染逻辑
│   ├── redis/          # Redis 客户端封装
│   ├── site-server/    # 站点服务管理
│   └── ...
├── web/                # 前端项目源码
├── docs/               # 项目文档
└── tests/              # 集成测试
```

## 3. 核心流程

### 3.1 站点访问流程

1.  用户/爬虫发起请求。
2.  **Firewall Middleware**: 检查 IP 黑名单、GeoIP、User-Agent 等。
3.  **Bot Detection**: 识别是否为搜索引擎爬虫。
4.  **Prerender Check**:
    *   如果是爬虫 -> 检查 Redis 缓存 -> 有缓存直接返回 HTML -> 无缓存触发预渲染/回源 -> 返回结果。
    *   如果是普通用户 -> 直接服务静态文件或反向代理到源站。

### 3.2 动态配置流程

1.  用户在前端修改配置（如添加站点、修改 WAF 规则）。
2.  API Server 更新 Redis 中的配置数据。
3.  **Config Watcher**: 各个服务节点订阅 Redis 变更频道。
4.  收到变更通知后，热加载配置或重启相关站点服务，实现零停机更新。

## 4. 技术栈

*   **Backend**: Go (Golang) 1.21+
*   **Web Framework**: Gin
*   **Database/Cache**: Redis
*   **Frontend**: React 18, TypeScript, Ant Design, Vite
*   **Charts**: ECharts

## 5. 数据流设计

*   **配置数据**: 存储在 Redis 中，持久化依赖 Redis AOF/RDB。
*   **访问日志**: 异步写入文件日志，并通过 Log Processor 聚合分析后存入 Redis 用于实时监控大屏展示。
*   **预渲染缓存**: HTML 内容存储在 Redis 或文件系统中（当前实现主要为 Redis）。
