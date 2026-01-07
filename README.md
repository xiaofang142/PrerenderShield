# PrerenderShield

## 产品简介

PrerenderShield 是一款集防火墙安全防护与渲染预热功能于一体的企业级 Web 应用中间件，专为解决前后端分离架构下网站发布的痛点而设计。现有防火墙产品（如雷池）无法支持渲染预热，而渲染预热产品（如 Rendertron）缺乏防火墙能力，PrerenderShield 填补了这一市场空白，为用户提供一站式的安全防护与 SEO 优化解决方案。

## 核心功能

### 🔒 OWASP Top 10 安全防护
- **注入攻击防护**：SQL 注入、命令注入等
- **跨站脚本防护**：存储型 XSS、反射型 XSS 等
- **跨站请求伪造防护**：CSRF 令牌验证、Origin 检查
- **不安全的反序列化防护**：类型安全检查、序列化白名单
- **敏感数据泄露防护**：数据加密、安全头配置
- **XML 外部实体防护**：XXE 攻击检测与拦截
- **不安全的依赖组件防护**：依赖漏洞扫描与告警

### 🚀 智能渲染预热服务
- **自动爬虫识别**：基于 User-Agent 和智能算法
- **支持主流框架**：React、Vue、Angular 等
- **高性能缓存机制**：多级缓存、可配置过期策略
- **并发渲染优化**：资源池管理、超时控制
- **一键缓存预热**：Sitemap 解析、批量渲染预热、定时更新

### 🔄 智能流量路由
- **请求自动分类**：爬虫请求/普通用户请求智能识别
- **动态路由规则**：可配置的流量分发策略
- **实时流量分析**：请求量、响应时间、成功率监控

### 🔐 SSL/TLS 支持
- **自动证书管理**：Let's Encrypt 集成
- **TLS 1.2/1.3 支持**：最新加密协议
- **证书自动续期**：零运维证书管理

### 📊 现代化管理界面
- **实时监控**：安全事件、渲染预热状态、系统健康
- **数据可视化**：ECharts 图表展示
- **告警通知**：邮件、Webhook 支持
- **配置管理**：集中式配置中心、版本管理

## 技术架构

### 四层系统架构
1. **接入层**：处理 HTTP/HTTPS 请求，SSL 终止，流量分发
2. **核心处理层**：智能流量路由，防火墙引擎，渲染预热引擎
3. **服务层**：规则管理，缓存管理，证书管理，任务调度
4. **管理与监控层**：Web 管理界面，日志系统，告警系统，API 服务

### 核心组件
- **防火墙引擎**：基于 OWASP 规则的模块化检测系统
- **渲染预热引擎**：Headless Chrome/Chromium + Puppeteer
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
- Docker：20.10+ 
- Docker Compose：2.0+

### 安装部署

#### 一键安装（推荐）

我们提供了便捷的一键安装脚本，自动完成环境检测、IP配置和服务部署：

```bash
# 下载项目（如果未下载）
git clone https://github.com/your-org/prerendershield.git
cd prerendershield

# 给脚本添加执行权限
chmod +x install.sh

# 自动获取IP并部署（推荐）
./install.sh

# 或手动指定IP并部署
./install.sh --ip 192.168.0.100
```

##### 脚本功能
- ✅ 自动获取本机公网IP和内网IP
- ✅ 检查Docker和Docker Compose环境
- ✅ 使用Dockerfile构建镜像
- ✅ 配置Redis容器间连接
- ✅ 启动所有服务
- ✅ 显示详细部署信息

##### 脚本选项
```bash
./install.sh [选项]

选项：
  -h, --help      显示帮助信息
  -f, --force     强制重新构建镜像
  --ip <ip>       手动指定本机IP地址
  --test          测试模式，只检查环境和获取IP
```

##### 常见用法
```bash
# 测试环境（推荐在部署前执行）
./install.sh --test

# 强制重新构建镜像
./install.sh --force

# 手动指定IP并强制重新构建
./install.sh --ip 192.168.0.100 --force
```

#### 传统 Docker 部署

如果您更习惯手动部署，可以使用以下命令：

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f
```

#### 直接运行（开发模式）

```bash
# 安装依赖
go mod tidy

# 构建应用
go build -o prerender-shield ./cmd/api

# 启动服务
./start.sh start
```

### 访问管理界面

部署完成后，通过以下地址访问管理控制台：

```
http://[您的服务器IP]:9597
```

#### 默认账号密码
- **用户名**：admin
- **密码**：123456

### 服务端口

| 服务类型 | 端口 | 说明 |
|---------|------|------|
| 管理控制台 | 9597 | Web 管理界面 |
| API 服务 | 9598 | 后端 API 接口 |
| Redis | 6379 | 缓存数据库（容器内部访问） |

## 部署后操作

### 查看服务状态

```bash
# 使用脚本查看
docker-compose ps

# 或使用Docker命令
docker ps
```

### 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f prerender-shield
```

### 停止/重启服务

```bash
# 停止服务
docker-compose down

# 重启服务
docker-compose restart
```

### 更新服务

```bash
# 拉取最新代码
git pull

# 重新构建并启动
./install.sh --force
```

## 项目优势

### 一体化解决方案
- 无需同时部署防火墙和渲染预热服务
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