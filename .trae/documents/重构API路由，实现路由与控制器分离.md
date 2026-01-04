# 重构API路由，按模块逐步迁移

## 目标
将 `internal/api/routes/routes.go` 中的路由注册逻辑按模块逐步重构为使用控制器实例，实现路由声明与控制器实现的分离。

## 优化原因
- 按模块迁移可以降低风险，便于测试
- 避免一次性迁移大量代码可能导致的问题
- 便于定位和解决迁移过程中遇到的问题

## 实现步骤

### 1. 模块划分
将API路由按功能划分为以下模块：
- 认证模块 (Auth)
- 概览模块 (Overview)
- 监控模块 (Monitoring)
- 防火墙模块 (Firewall)
- 爬虫模块 (Crawler)
- 预热模块 (Preheat)
- 站点管理模块 (Sites)

### 2. 迁移顺序
按以下顺序逐步迁移每个模块：

#### 模块1：认证模块 (Auth)
- 迁移路由：`/api/v1/auth/first-run`、`/api/v1/auth/login`、`/api/v1/auth/logout`
- 使用控制器：`AuthController`

#### 模块2：概览模块 (Overview)
- 迁移路由：`/api/v1/overview`
- 使用控制器：`OverviewController`

#### 模块3：监控模块 (Monitoring)
- 迁移路由：`/api/v1/monitoring/stats`
- 使用控制器：`MonitoringController`

#### 模块4：防火墙模块 (Firewall)
- 迁移路由：`/api/v1/firewall/status`、`/api/v1/firewall/rules`
- 使用控制器：`FirewallController`

#### 模块5：爬虫模块 (Crawler)
- 迁移路由：`/api/v1/crawler/logs`、`/api/v1/crawler/stats`
- 使用控制器：`CrawlerController`

#### 模块6：预热模块 (Preheat)
- 迁移路由：`/api/v1/preheat/sites`、`/api/v1/preheat/stats`、`/api/v1/preheat/trigger`、`/api/v1/preheat/url`、`/api/v1/preheat/urls`、`/api/v1/preheat/task/status`、`/api/v1/preheat/crawler-headers`
- 使用控制器：`PreheatController`

#### 模块7：站点管理模块 (Sites)
- 迁移路由：站点CRUD、静态资源管理等
- 使用控制器：`SitesController`

### 3. 迁移策略
- 每个模块迁移完成后，运行测试确保功能正常
- 保留原有的中间件配置和路由结构
- 仅替换路由处理函数为控制器方法

## 预期结果
- 逐步完成路由与控制器的分离
- 降低迁移风险，便于测试和调试
- 最终实现清晰的API架构，路由声明与控制器实现分离

## 关键修改点
- 文件：`internal/api/routes/routes.go`
- 主要修改：按模块逐步将内联路由处理函数替换为对应的控制器方法