# Prerender Shield API 清单

> 从 `internal/api/routes/route_registration.go` 自动核对生成（2026-08-31，v3.0.0）。
> Base URL: `http://<host>:9598/api/v1`
> 认证方式: 除标注「公开」外均需 `Authorization: Bearer <JWT>`；WebSocket 通过 `?token=` 查询参数传递。

## 响应格式

```json
{"code": 200, "data": {...}, "message": "success"}
{"code": 4xx/5xx, "message": "错误描述"}
```

## 认证 Auth

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/auth/first-run` | 公开 | 检查是否首次运行（首次需创建管理员） |
| POST | `/auth/login` | 公开 | 登录；首次运行时以提交凭据创建首个管理员 |
| POST | `/auth/logout` | 公开 | 退出登录（可选携带 `Authorization` 头，用于审计日志） |
| POST | `/auth/change-password` | JWT | 修改密码 |

## 系统 System

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/health` | 公开 | 健康检查（Redis/SSL/GC 状态） |
| GET | `/version` | 公开 | 版本信息 |
| GET | `/system/config` | JWT | 获取系统配置 |
| POST | `/system/config` | JWT | 更新系统配置 |
| POST | `/system/backup` | JWT | 创建配置备份 |
| POST | `/system/restore` | JWT | 恢复备份 |
| GET | `/system/backups` | JWT | 备份列表 |

## 双因素认证 2FA

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/2fa/status` | JWT | 2FA 状态 |
| POST | `/2fa/enable` | JWT | 启用 2FA（返回 secret） |
| POST | `/2fa/confirm` | JWT | 确认启用（校验验证码） |
| POST | `/2fa/disable` | JWT | 关闭 2FA |

## SSL 证书

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/ssl/certificates/:domain` | 公开 | 单证书状态 |
| GET | `/ssl/certificates` | 公开 | 证书列表 |
| GET | `/ssl/certificates/expiring` | 公开 | 即将过期证书 |
| POST | `/ssl/certificates` | JWT | 申请证书（ACME） |
| POST | `/ssl/certificates/:domain/renew` | JWT | 续签证书 |
| DELETE | `/ssl/certificates/:domain` | JWT | 删除证书 |
| POST | `/ssl/certificates/wildcard` | JWT | 申请通配符证书（DNS challenge） |
| GET | `/ssl/certificates/:domain/renewal-history` | JWT | 续签历史 |

## 概览与监控 Overview / Monitoring

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/overview` | JWT | 控制台首页聚合数据 |
| GET | `/monitoring/stats` | JWT | 实时监控指标 |
| GET | `/monitoring/alerts/history` | JWT | 告警历史 |
| GET | `/monitoring/alert-rules` | JWT | 告警规则列表 |
| POST | `/monitoring/alert-rules` | JWT | 保存告警规则 |
| DELETE | `/monitoring/alert-rules/:id` | JWT | 删除告警规则 |
| GET | `/monitoring/alerts/rules` | JWT | 别名路由（同 alert-rules） |
| POST | `/monitoring/alerts/rules` | JWT | 别名路由（同 alert-rules） |
| GET | `/monitoring/alerts/channels` | JWT | 通知渠道配置 |
| POST | `/monitoring/alerts/channels` | JWT | 保存通知渠道 |

## 防火墙 Firewall

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/firewall/rules` | JWT | 防火墙规则列表 |
| POST | `/firewall/rules` | JWT | 保存防火墙规则 |
| DELETE | `/firewall/rules/:id` | JWT | 删除规则 |
| GET | `/logs` | JWT | 访问日志（支持分页过滤） |
| GET | `/logs/export` | JWT | 导出日志 |
| GET | `/firewall/attacks` | JWT | 攻击日志 |
| POST | `/firewall/whitelist` | JWT | 加入白名单 |
| GET | `/firewall/whitelist` | JWT | 白名单列表 |
| POST | `/firewall/blacklist` | JWT | 加入黑名单 |
| GET | `/firewall/blacklist` | JWT | 黑名单列表 |
| GET | `/sites/:id/waf` | JWT | 站点 WAF 配置 |
| PUT | `/sites/:id/waf` | JWT | 更新站点 WAF 配置 |

## 爬虫日志 Crawler

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/crawler/logs` | JWT | 爬虫访问日志 |
| GET | `/crawler/stats` | JWT | 爬虫统计（支持 site/时间/granularity） |
| GET | `/crawler/url-stats` | JWT | per-URL 渲染预算报表（site/startTime/endTime/limit，RFC3339 时间） |

## SEO

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/seo/config` | JWT | SEO 配置 |
| PUT | `/seo/config` | JWT | 更新 SEO 配置 |
| GET | `/seo/sitemap` | JWT | 查看 sitemap |
| POST | `/seo/sitemap/generate` | JWT | 生成 sitemap |
| GET | `/seo/robots` | JWT | 查看 robots.txt |
| POST | `/seo/robots/generate` | JWT | 生成 robots.txt |

## 渲染预热 Preheat

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/preheat/sites` | JWT | 可预热站点列表 |
| GET | `/preheat/stats` | JWT | 预热统计 |
| POST | `/preheat/trigger` | JWT | 触发预热任务 |
| GET | `/preheat/urls` | JWT | 预热 URL 列表 |
| GET | `/preheat/task/status` | JWT | 预热任务状态 |
| GET | `/preheat/crawler-headers` | JWT | 爬虫协议头列表 |
| POST | `/preheat/clear-cache` | JWT | 清空渲染缓存 |
| POST | `/preheat/invalidate` | JWT | 单 URL 缓存失效（全设备桶） |
| POST | `/preheat/recache` | JWT | 单 URL 强制重渲替换缓存 |
| GET | `/preheat/entries` | JWT | 渲染缓存条目列表（支持 site/limit） |
| DELETE | `/preheat/entries` | JWT | 删除单条渲染缓存条目 |

## 搜索引擎推送 Push

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/push/sites` | JWT | 可推送站点列表 |
| GET | `/push/stats` | JWT | 推送统计 |
| GET | `/push/logs` | JWT | 推送日志 |
| GET | `/push/trend` | JWT | 推送趋势 |
| GET | `/push/config` | JWT | 推送配置 |
| POST | `/push/config` | JWT | 更新推送配置 |

## 站点管理 Sites

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/sites` | JWT | 站点列表 |
| POST | `/sites` | JWT | 添加站点（免费版超 1 站点返回 402） |
| GET | `/sites/:id` | JWT | 站点详情 |
| PUT | `/sites/:id` | JWT | 更新站点 |
| DELETE | `/sites/:id` | JWT | 删除站点 |
| GET | `/sites/:id/config` | JWT | 站点 Redis 配置 |
| PUT | `/sites/:id/prerender` | JWT | 更新预渲染配置 |
| PUT | `/sites/:id/push` | JWT | 更新推送配置 |
| PUT | `/sites/:id/firewall` | JWT | 更新防火墙配置 |
| POST | `/sites/:id/start` | JWT | 启动站点服务 |
| POST | `/sites/:id/stop` | JWT | 停止站点服务 |
| GET | `/sites/:id/static` | JWT | 静态资源文件列表 |
| POST | `/sites/:id/static` | JWT | 上传静态资源 |
| POST | `/sites/:id/static/extract` | JWT | 解压压缩包 |
| DELETE | `/sites/:id/static` | JWT | 删除静态文件 |
| POST | `/sites/:id/static/batch-delete` | JWT | 批量删除静态文件 |

## WebSocket

| 路径 | 认证 | 说明 |
|------|------|------|
| `/ws/realtime?token=<JWT>` | JWT(query) | 实时推送：告警(alert) + 监控指标(monitor, 10s) + WAF 事件 |

## 错误码约定

| code | 含义 |
|------|------|
| 200 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证/令牌失效 |
| 402 | 免费版站点数超限（附价格信息） |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 冲突 |
| 500 | 服务端错误 |
