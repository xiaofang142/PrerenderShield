# PrerenderShield

## 产品简介

PrerenderShield 是一款集防火墙安全防护与预渲染功能于一体的企业级 Web 应用中间件，专为解决前后端分离架构下网站发布的痛点而设计。现有防火墙产品（如雷池）无法支持预渲染，而预渲染产品（如 Rendertron）缺乏防火墙能力，PrerenderShield 填补了这一市场空白，为用户提供一站式的安全防护与 SEO 优化解决方案。

## 核心功能

### 🔒 OWASP Top 10 安全防护
- **注入攻击防护**：SQL 注入、命令注入等
- **跨站脚本防护**：存储型 XSS、反射型 XSS 等
- **跨站请求伪造防护**：CSRF 令牌验证、Origin 检查
- **不安全的反序列化防护**：类型安全检查、序列化白名单
- **敏感数据泄露防护**：数据加密、安全头配置
- **XML 外部实体防护**：XXE 攻击检测与拦截
- **不安全的依赖组件防护**：依赖漏洞扫描与告警

### 🚀 智能预渲染服务
- **自动爬虫识别**：基于 User-Agent 和智能算法
- **支持主流框架**：React、Vue、Angular 等
- **高性能缓存机制**：多级缓存、可配置过期策略
- **并发渲染优化**：资源池管理、超时控制
- **一键缓存预热**：Sitemap 解析、批量预渲染、定时更新

### 🔄 智能流量路由
- **请求自动分类**：爬虫请求/普通用户请求智能识别
- **动态路由规则**：可配置的流量分发策略
- **实时流量分析**：请求量、响应时间、成功率监控

### 🔐 SSL/TLS 支持
- **自动证书管理**：Let's Encrypt 集成
- **TLS 1.2/1.3 支持**：最新加密协议
- **证书自动续期**：零运维证书管理

### 📊 现代化管理界面
- **实时监控**：安全事件、预渲染状态、系统健康
- **数据可视化**：ECharts 图表展示
- **告警通知**：邮件、Webhook 支持
- **配置管理**：集中式配置中心、版本管理

## 技术架构

### 四层系统架构
1. **接入层**：处理 HTTP/HTTPS 请求，SSL 终止，流量分发
2. **核心处理层**：智能流量路由，防火墙引擎，预渲染引擎
3. **服务层**：规则管理，缓存管理，证书管理，任务调度
4. **管理与监控层**：Web 管理界面，日志系统，告警系统，API 服务

### 核心组件
- **防火墙引擎**：基于 OWASP 规则的模块化检测系统
- **预渲染引擎**：Headless Chrome/Chromium + Puppeteer
- **缓存管理器**：多级缓存策略，支持 Redis/Memory
- **ACME 客户端**：自动 Let's Encrypt 证书管理
- **任务调度器**：缓存预热、定期扫描等后台任务

### 技术栈
- **后端**：Go 1.20+（高性能、并发安全）
- **前端**：React 18 + TypeScript + Ant Design + ECharts
- **渲染引擎**：Puppeteer + Chromium
- **容器化**：Docker + Kubernetes
- **数据库**：PostgreSQL（配置、日志）、Redis（缓存、队列）

## 应用场景

### 前后端分离网站
- 解决 SPA 应用 SEO 问题
- 提供全方位安全防护
- 简化部署架构，降低运维复杂度

### 高流量电商平台
- 保护 API 接口安全
- 优化爬虫抓取性能
- 防止恶意请求与 DDoS 攻击

### 企业级 Web 应用
- 符合 OWASP 安全标准
- 提供合规审计日志
- 支持多环境部署

## 快速开始

### 系统要求
- CPU：4 核（推荐 8 核）
- 内存：8GB（推荐 16GB）
- 磁盘：100GB（推荐 200GB SSD）
- 操作系统：Linux/macOS

### 安装部署

#### 使用 Docker 部署
```bash
docker-compose up -d
```

#### 访问管理界面
```
http://your-server-ip:8080
```

## 项目优势

### 一体化解决方案
- 无需同时部署防火墙和预渲染服务
- 统一管理界面，降低学习成本
- 减少系统间集成复杂度

### 高性能设计
- 异步处理架构
- 资源池化管理
- 优化的缓存策略

### 易于扩展
- 模块化设计，支持插件扩展
- 可水平扩展的架构
- 开放 API 支持二次开发

### 安全可靠
- 专注 OWASP Top 10 威胁防护
- 定期安全更新
- 完整的审计日志

## 社区与贡献

欢迎提交 Issue 和 Pull Request！

### 开发环境搭建
```bash
# 安装 Go 环境
tar -C /usr/local -xzf go1.20.0.linux-amd64.tar.gz

# 安装 Node.js
nvm install 18
nvm use 18

# 安装 Docker
sudo apt-get install docker-ce docker-ce-cli containerd.io
```

## 许可证

MIT License

## 联系方式

- 项目地址：https://github.com/your-org/prerendershield
- 文档地址：https://prerendershield.io/docs
- 问题反馈：https://github.com/your-org/prerendershield/issues

---

**PrerenderShield** - 让前后端分离网站更安全、更快速！