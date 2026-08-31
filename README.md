# Prerender Shield

<p align="center">
  <a href="https://prerender.websitetool.cn"><strong>📖 在线文档</strong></a> ·
  <a href="https://gitee.com/xhpmayun/prerender-shield"><strong>Gitee</strong></a> ·
  <a href="https://github.com/xiaofang142/PrerenderShield"><strong>GitHub</strong></a>
</p>

<p align="center">
  <a href="https://prerender.websitetool.cn">
    <img src="https://img.shields.io/badge/官网-prerender.websitetool.cn-brightgreen" alt="官网">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="License">
  </a>
  <a href="https://gitee.com/xhpmayun/prerender-shield">
    <img src="https://gitee.com/xhpmayun/prerender-shield/badge/star.svg" alt="Gitee Stars">
  </a>
</p>

## 📌 产品规划与商业模式

面向国际化中小项目场景的功能边界、模块清单、竞品对比与收费方式分析，请查看：

- [竞品研究与商业模式分析（2026）](docs/competitive-research-2026.md)
- [功能清单与实现状态](docs/feature-inventory.md)

## 🚀 一键安装

```bash
curl -fsSL https://prerender.websitetool.cn/install.sh | bash
```

> 安装完成后访问 `http://服务器IP:9597` 进入管理控制台。首次访问时使用登录页提交的账号密码创建管理员（无预置默认账号）。

---

## 0. 前后端分离下的 SEO 痛点

在现代化的 Web 开发中，前后端分离架构（SPA - 单页应用）已成为主流，React、Vue、Angular 等框架带来了极佳的开发体验。然而，这种架构也带来了显著的 SEO 挑战：

### 🔍 **爬虫兼容性问题**
- **搜索引擎爬虫无法执行 JavaScript**：传统搜索引擎爬虫难以解析动态生成的内容
- **首屏加载延迟**：SPA 应用需要先加载 JavaScript 再渲染内容，影响爬虫抓取效率
- **内容索引不完整**：动态路由和异步加载的内容可能无法被完整索引

### 🛡️ **安全防护缺失**
- **API 暴露风险**：前后端分离架构使得 API 接口直接暴露在外网
- **OWASP Top 10 威胁**：XSS、CSRF、SQL 注入等攻击面扩大
- **缺乏统一防护**：需要在多个层面配置安全策略，管理复杂

### 🔄 **技术栈碎片化**
- **需要多个组件组合**：WAF 防火墙 + 渲染预热服务 + 反向代理
- **配置复杂分散**：不同系统有不同的配置方式和监控界面
- **运维成本高昂**：需要维护多个服务的部署、监控和更新

### 💰 **成本与效率问题**
- **商业服务费用高昂**：Cloudflare WAF + Prerender.io 等组合年费可达数千美元
- **资源利用不充分**：单独部署的渲染服务可能对所有请求进行渲染，浪费计算资源
- **响应延迟增加**：多个中间件串联增加请求延迟

## 1. 项目介绍

**Prerender Shield** 是一款创新的企业级 Web 应用中间件，将 **OWASP Top 10 安全防护** 与 **智能渲染预热** 功能深度集成，专为解决前后端分离架构下的 SEO 优化和安全防护痛点而设计。

### 🎯 **核心价值主张**
- **一体化解决方案**：安全防护 + SEO 优化，一套系统解决两个核心问题
- **智能流量路由**：自动识别爬虫请求，按需渲染，避免资源浪费
- **开源自托管**：完全开源，支持私有化部署，数据完全自主可控

### 🏗️ **技术架构**
- **后端**：Go 1.25+（高性能、高并发、内存安全）
- **前端**：React 18 + TypeScript + Ant Design（现代化管理界面）
- **渲染引擎**：Headless Chromium + chromedp（Go 原生 CDP 驱动，标准浏览器渲染）
- **缓存系统**：Redis（高性能内存数据库）
- **部署方式**：原生部署（一键脚本安装）

## 2. 核心功能介绍

### 🔒 **企业级安全防护**

#### OWASP Top 10 全面防护
| 威胁类型 | 防护能力 | 检测方式 |
|---------|---------|---------|
| **注入攻击** | SQL 注入、NoSQL 注入、命令注入 | 正则匹配、语法分析 |
| **跨站脚本** | 存储型 XSS、反射型 XSS、DOM XSS | 输入过滤、输出编码 |
| **跨站请求伪造** | CSRF 令牌验证、Origin 检查 | 令牌验证、来源验证 |
| **不安全反序列化** | 类型安全检查、序列化白名单 | 格式验证、类型检查 |
| **敏感数据泄露** | 数据加密、安全头配置 | 数据掩码、头检测 |
| **XML 外部实体** | XXE 攻击检测与拦截 | XML 解析限制 |
| **组件安全漏洞** | 响应头安全策略、依赖组件加固 | 安全头配置、CSP 策略（CVE 依赖扫描规划中，尚未实现） |

#### 高级安全特性
- **实时威胁检测**：毫秒级响应，自动拦截恶意请求
- **自定义规则引擎**：支持基于正则表达式的自定义防护规则
- **详细审计日志**：完整记录所有安全事件，支持溯源分析
- **GeoIP 过滤**：基于地理位置的黑白名单控制

### 🚀 **智能渲染预热**

#### 爬虫智能识别
- **User-Agent 分析**：自动识别搜索引擎爬虫（Googlebot、Bingbot 等）
- **行为模式检测**：基于请求频率、路径模式的智能识别
- **可配置策略**：支持自定义爬虫识别规则

#### 高性能渲染引擎
- **Headless Chromium**：使用标准浏览器引擎，确保渲染兼容性
- **资源池管理**：Chromium 实例复用，降低内存消耗
- **并发控制**：智能并发限制，防止资源耗尽
- **超时机制**：可配置渲染超时，避免长时间阻塞

#### 缓存优化策略
- **多级缓存**：内存缓存 + Redis 缓存，支持分布式部署
- **智能过期**：基于内容哈希的缓存失效策略
- **条件更新**：支持 If-Modified-Since、ETag 等 HTTP 缓存头
- **预热机制**：支持 sitemap 解析和定时批量预热

### 🔄 **智能流量路由**

#### 请求自动分类
- **爬虫请求** → 渲染预热 → 返回静态 HTML
- **普通用户请求** → 直接转发 → 原样响应
- **API 请求** → 安全检查 → 转发或拦截

#### 动态路由规则
- **基于路径的路由**：不同路径可配置不同处理策略
- **基于域名的路由**：支持多站点独立配置
- **基于条件的路由**：支持复杂的条件判断逻辑

### 📊 **现代化管理界面**

#### 实时监控仪表板
- **安全事件监控**：实时显示拦截的威胁类型和数量
- **渲染状态监控**：显示渲染成功率、平均耗时等指标
- **流量分析**：请求量、响应时间、缓存命中率统计
- **系统健康检查**：服务状态、资源使用情况监控

#### 配置管理中心
- **站点管理**：多站点独立配置，支持批量操作
- **规则管理**：安全规则、渲染规则的可视化管理
- **证书管理**：SSL/TLS 证书的申请和续期
- **用户管理**：基于角色的权限控制系统

### 🔐 **SSL/TLS 支持**
- **Let's Encrypt 集成**：自动申请和续期免费 SSL 证书
- **TLS 1.2/1.3 支持**：最新加密协议，确保传输安全
- **证书自动管理**：零运维成本，自动处理证书过期

## 3. 产品优势对比

### 🏆 **与竞品对比分析**

Prerender Shield 在市场上具有独特的定位，填补了现有产品的功能空白：

#### 市场定位矩阵
| 产品类型 | 代表产品 | 安全防护 | 渲染预热 | 综合网关 | 成本模式 |
|---------|---------|---------|---------|---------|---------|
| **Prerender Shield** | 本项目 | ✅ 完整OWASP防护 | ✅ 智能渲染预热 | ✅ 多站点代理 | 1 站点免费，$99/站/年（[定价](https://prerender.websitetool.cn/pricing)） |
| **纯WAF产品** | 雷池、Cloudflare WAF | ✅ 企业级防护 | ❌ 不支持 | ❌ 仅安全检测 | 商业付费 |
| **纯渲染产品** | Rendertron、Prerender.io | ❌ 无安全功能 | ✅ 专业渲染 | ❌ 仅渲染服务 | 开源/SAAS |
| **综合网关** | Nginx、Envoy、Traefik | ⚠️ 基础防护 | ❌ 不支持 | ✅ 完整网关功能 | 开源免费 |
| **商业云服务** | AWS WAF + CloudFront | ✅ 云端防护 | ⚠️ 有限支持 | ✅ 完整CDN | 按量付费 |

### 🎯 **核心竞争优势**

#### 1. 功能融合创新
- **一站式解决方案**：首次将企业级 WAF 与 SPA 渲染预热深度集成
- **技术协同优势**：安全检测 → 智能路由 → 按需渲染的完整闭环
- **统一管理体验**：单个界面管理安全和渲染配置，降低学习成本

#### 2. 成本效益显著
| 对比方案 | 年化成本 | 部署复杂度 | 运维成本 |
|---------|---------|-----------|---------|
| **Prerender Shield** | 1 站点免费；多站点 $99/站/年 | 低（一键部署） | 低（单系统运维） |
| **雷池WAF + Rendertron** | 2000元+ | 高（多系统集成） | 高（多系统运维） |
| **Cloudflare + Prerender.io** | 5000元+ | 中（云服务配置） | 中（云服务管理） |

#### 3. 技术优势突出
- **智能路由算法**：爬虫请求才进入渲染，避免无效渲染，节省计算资源
- **资源优化设计**：Chromium 实例复用，降低内存消耗
- **实时配置更新**：Redis 驱动的动态配置，支持热重载
- **现代化技术栈**：Go + React + TypeScript，易于二次开发

#### 4. 本地化优势
- **数据自主可控**：完全自托管，满足中国数据合规要求
- **网络优化**：本地部署，避免跨境网络延迟
- **中文友好**：完整中文文档和界面，降低使用门槛

### ⚡ **性能对比**

> 下表为设计目标参考值（非实测基准数据），实际表现因环境、配置与内容复杂度而异。

| 指标 | Prerender Shield | Rendertron | Prerender.io |
|------|-----------------|------------|--------------|
| **首次渲染延迟** | 300-500ms | 500-800ms | 200-400ms |
| **缓存命中率** | 95%+ | 85%+ | 90%+ |
| **内存消耗** | 中等（资源池优化） | 较高（每次新建实例） | 云服务托管 |
| **并发处理能力** | 50+ 请求/秒 | 20+ 请求/秒 | 按套餐限制 |

### 🛡️ **安全能力对比**
| 安全特性 | Prerender Shield | Cloudflare WAF | 雷池 WAF |
|---------|-----------------|---------------|---------|
| **OWASP 覆盖** | Top 10 完整覆盖 | Top 10 完整覆盖 | Top 10 完整覆盖 |
| **自定义规则** | ✅ 支持 | ✅ 支持 | ✅ 支持 |
| **实时更新** | 社区更新 | ✅ 云端实时更新 | ✅ 定期更新 |
| **审计日志** | ✅ 完整记录 | ✅ 企业版支持 | ✅ 支持 |
| **DDoS 防护** | 基础限流 | ✅ 专业防护 | ✅ 专业防护 |

## 4. 安装与部署

### 🚀 **一键安装（推荐）**

```bash
curl -fsSL https://prerender.websitetool.cn/install.sh | bash
```

> 脚本自动检测环境，支持 Docker / 源码 / 二进制三种方式。详情请访问 [在线文档](https://prerender.websitetool.cn/installation)。

### ⚙️ **防火墙 / 安全组配置**

安装后需确保以下端口可访问：

| 端口 | 服务 | 必需 |
|------|------|------|
| 9597 | 管理控制台 (Web) | ✅ 必需 |
| 9598 | API 服务 | ✅ 必需 |
| 6379 | Redis | 内网即可 |

各云平台安全组配置指南请查看 [安装文档](https://prerender.websitetool.cn/installation)。

---

### 📋 **系统要求**

| 组件 | 最低要求 | 推荐配置 |
|------|---------|---------|
| **操作系统** | Linux (Ubuntu 20.04+, CentOS 8+, openSUSE, Alpine) / macOS 12+ | Linux (Ubuntu 22.04 LTS) |
| **CPU** | 2 核 | 4 核 |
| **内存** | 4 GB | 8 GB |
| **磁盘空间** | 10 GB | 20 GB (SSD) |
| **网络** | 可访问公网 | 稳定的网络连接 |
| **架构** | x86_64, arm64 | x86_64 |

### 📥 **部署方案**

Prerender Shield 采用三阶段部署模式：**开发者构建 → 用户安装 → 用户启动**，提供了完整的脚本支持。

#### 🚀 **开发者构建流程**

适合开发者和需要自定义编译的场景：

```bash
# 1. 克隆代码仓库
git clone https://github.com/xiaofang142/PrerenderShield.git
cd prerender-shield

# 2. 给构建脚本添加执行权限
chmod +x build.sh

# 3. 执行构建脚本（自动构建前端和后端）
./build.sh
```

**build.sh 脚本功能说明**：
- ✅ 依赖 Go 与 Node.js 环境（未安装时构建失败）
- ✅ 配置Go模块镜像加速
- ✅ 自动获取当前平台信息
- ✅ 构建前端（安装依赖、构建；可通过 `--api-host` 参数或 `VITE_API_BASE_URL` 环境变量设置 API 地址）
- ✅ 安装Go依赖
- ✅ 构建当前平台的二进制文件（路径：bin/api）
- ✅ 将前端代码从web/dist拷贝到bin/web
- ✅ 构建产物验证和测试
- ✅ 输出构建结果和使用说明

#### 📦 **用户安装流程**

适合普通用户使用，基于预编译的二进制文件进行安装：

```bash
# 1. 确保已经完成构建（由开发者执行）
# ./build.sh

# 2. 给安装脚本添加执行权限
chmod +x install.sh

# 3. 执行安装脚本（自动检测环境）
./install.sh
```

**install.sh 脚本功能说明**：
- ✅ 检测操作系统和架构
- ✅ 自动选择安装方式（Docker / 源码构建 / 预编译二进制）
- ✅ 检查并安装 Redis（如未安装）
- ✅ 检查并安装 Chromium 无头浏览器（渲染引擎核心依赖；未检测到时提示设置 `CHROME_PATH`，程序运行时会读取该变量定位浏览器）
- ✅ 生成默认配置并注册 systemd 服务（Linux）或后台启动（macOS）
- ✅ 执行安装后的健康检查

#### 🎯 **用户启动流程**

适合普通用户使用，启动已安装的应用：

```bash
# 启动（Linux 下由 systemd 托管）
sudo systemctl start prerender-shield

# 或使用启动脚本
./start.sh start
```

> 注意：`api` 二进制本身不支持 start/restart/stop 子命令；停止服务请使用
> `sudo systemctl stop prerender-shield`（Linux）或 `kill $(cat data/prerender-shield.pid)`（macOS）。

```bash
# 兼容旧版本启动方式
chmod +x start.sh
./start.sh start
```
- ✅ 检测服务是否真正启动
- ✅ 执行服务健康检查
- ✅ 输出清晰的访问信息

### 📋 **脚本体系说明**

| 脚本名称 | 角色 | 主要功能 | 执行环境 |
|----------|------|----------|----------|
| **build.sh** | 开发者构建脚本 | 构建前端和后端，生成当前平台二进制文件（交叉编译多平台由 CI release 流水线完成） | 开发环境 |
| **install.sh** | 用户安装脚本 | 自动选择 Docker / 源码构建 / 预编译二进制安装，装依赖、生成配置 | 生产环境 |
| **start.sh** | 用户启动脚本 | 启动、停止、重启应用，执行健康检查 | 生产环境 |

### 🔍 **验证安装**

安装完成后，请通过以下步骤验证服务是否正常运行：

1. **检查服务状态**
   ```bash
   ./start.sh status
   ```

2. **访问管理界面**
   - 打开浏览器访问：`http://你的服务器IP:9597`
   - 首次访问时在登录页设置管理员账号密码（无预置默认账号）

3. **测试API接口**
   ```bash
   # 健康检查接口
   curl http://localhost:9598/api/v1/health
   # 预期返回：{"code":200,"data":{"status":"running","service":"prerender-shield","timestamp":...}}
   
   # 版本信息接口
   curl http://localhost:9598/api/v1/version
   # 预期返回：{"code":200,"data":{"version":"3.0.0","official_url":...,"name":"prerender-shield"}}
   ```

### 🚀 **服务管理**

####  **使用 start.sh 脚本管理**

```bash
# 启动服务（使用预编译的二进制文件）
./start.sh start

# 重启服务
./start.sh restart

# 停止服务
./start.sh stop

# 查看服务状态
./start.sh status
```

### 📊 **服务端口说明**

| 服务 | 端口 | 说明 |
|------|------|------|
| **管理界面** | 9597 | Web 管理控制台 |
| **API 服务** | 9598 | 后端 REST API |
| **Redis** | 6379 | 缓存数据库（本地服务） |
| **Prometheus** | 9090 | 监控指标暴露（可选） |

### 📁 **目录结构说明**

```
prerender-shield/                # 项目根目录/运行目录
├── bin/                        # 构建产物目录（build.sh 构建当前平台）
│   ├── api                     # 二进制文件
│   └── web/                    # 前端构建产物（管理控制台）
├── static/                     # 静态资源目录
├── certs/                      # 证书目录
├── configs/                    # 配置文件目录
│   ├── config.example.yml      # 配置文件模板
│   └── alert-rules.example.json # 告警规则示例
├── data/                       # 数据目录（运行时生成）
│   ├── prerender-shield.pid    # 进程PID文件
│   └── prerender-shield.log    # 日志文件
└── web/                        # 前端源码目录
    └── dist/                   # 前端构建产物
```

**项目根目录结构**：

```
prerender-shield/               # 项目根目录
├── bin/                        # 构建产物目录（build.sh 输出当前平台：bin/api + bin/web）
├── cmd/                        # Go 命令行入口目录
│   ├── api/                    # API 服务入口（main.go）
│   └── chromeprobe/            # Chromium 探测工具
├── configs/                    # 配置文件模板目录
│   ├── config.example.yml      # 配置文件模板
│   └── alert-rules.example.json # 告警规则示例
├── data/                       # 数据目录（运行时生成）
│   ├── prerender-shield.pid    # 进程PID文件
│   └── prerender-shield.log    # 日志文件
├── docker/                     # Docker 相关（备选编排与入口脚本）
│   ├── Dockerfile              # 多阶段构建镜像
│   └── docker-compose.yml      # 带 Nginx 的可选编排
├── Dockerfile                  # 根级多阶段构建镜像（配合根级 compose）
├── docker-compose.yml          # app + redis 双服务编排
├── docs/                       # 项目文档（见文末「文档索引」）
├── web/                        # 前端代码目录
│   ├── dist/                   # 构建后的前端文件（部署时使用）
│   ├── src/                    # 前端源代码
│   ├── package.json            # 前端依赖配置
│   └── vite.config.ts          # Vite配置文件
├── build.sh                    # 构建脚本（开发者使用）
├── install.sh                  # 安装脚本（用户使用）
├── start.sh                    # 启动脚本（用户使用）
├── INSTALL.md                  # 安装指南（四种方式）
├── CONTRIBUTING.md             # 贡献指南
├── SECURITY.md                 # 安全漏洞报告
├── go.mod                      # Go模块依赖配置
└── README.md                   # 项目说明文档
```

## 5. 配置管理

### 🎯 **主要配置文件**

| 配置文件 | 路径 | 说明 |
|---------|------|------|
| 主配置文件 | `./config.yml`（安装目录，install.sh 生成；systemd 部署为 `/etc/prerender-shield/config.yml`） | 包含所有核心配置 |
| 站点配置 | 存储在 Redis 中 | 动态站点配置，支持热更新 |
| 系统服务环境变量 | `/etc/default/prerender-shield`（EnvironmentFile，可选配置） | systemd 服务的敏感环境变量建议放此处 |

### 🔧 **配置示例**

```yaml
# 服务器配置
server:
  address: 0.0.0.0
  api_port: 9598
  console_port: 9597

# 目录配置
dirs:
  data_dir: ./data
  static_dir: ./static
  admin_static_dir: ./web
  certs_dir: ./certs

# 缓存配置
cache:
  type: redis
  redis_url: "localhost:6379"
  memory_size: 1000

# 监控配置
monitoring:
  enabled: true
  prometheus_address: ":9090"
```

#### ⚠️ **Redis 持久化要求（生产必读）**

Prerender Shield 将**用户账号、站点配置、告警记录、任务状态**等核心数据存储在 Redis 中。
生产环境必须开启 Redis 持久化（AOF），否则 Redis 重启会导致管理员账号和站点配置丢失：

```conf
# redis.conf
appendonly yes
appendfsync everysec   # 每秒刷盘，兼顾性能与安全
```

验证方式：`redis-cli CONFIG GET appendonly` 应返回 `appendonly=yes`。
Docker 部署请为 redis 服务挂载持久卷并追加 `--appendonly yes` 启动参数。

### 🔄 **动态配置更新**

Prerender Shield 支持动态配置更新，无需重启服务即可生效：

1. **通过管理界面更新**：登录管理控制台，在系统配置页面进行修改
2. **通过API更新**：使用 `POST /api/v1/system/config` 接口更新配置
3. **通过配置文件更新**：直接修改配置文件，系统会自动检测并加载

## 6. 开发与贡献

### 🌟 **完全开源，社区驱动**

Prerender Shield 采用 **MIT 许可证** 完全开源，我们相信：

- **透明可信**：所有代码公开，安全可审计
- **社区共建**：欢迎开发者贡献代码，共同完善功能
- **自由使用**：可自由使用、修改、分发，无任何限制

### 🤝 **如何参与贡献**

#### 代码贡献
1. **Fork 仓库**：创建自己的分支
2. **开发功能**：遵循项目代码规范
3. **提交 PR**：描述功能变更和测试结果
4. **代码审查**：项目维护者进行审查合并

#### 文档贡献
- **完善文档**：帮助改进使用指南、API 文档
- **翻译工作**：协助将文档翻译为其他语言
- **示例代码**：提供使用示例和最佳实践

#### 问题反馈
- **Bug 报告**：在 GitHub Issues 提交详细的问题描述
- **功能建议**：提出有价值的功能改进建议
- **使用反馈**：分享使用经验和改进建议

### 🏗️ **开发环境搭建**

```bash
# 1. 克隆项目
git clone https://gitee.com/xhpmayun/prerender-shield.git
cd prerender-shield

# 2. 启动后端服务（开发模式）
cd cmd/api
go run main.go

# 3. 启动前端开发服务
cd web
npm install
npm run dev

# 4. 访问开发环境
# 管理界面：http://localhost:9597（或前端开发服务 http://localhost:5173）
# API服务：http://localhost:9598
# 首次访问控制台时，使用登录页提交的账号密码创建管理员（无预置默认账号）
```

### 📚 **相关资源**

- **项目主页**：[Gitee](https://gitee.com/xhpmayun/prerender-shield) | [GitHub](https://github.com/xiaofang142/PrerenderShield)
- **在线文档**：[项目文档](https://prerender.websitetool.cn/)
- **问题追踪**：[GitHub Issues](https://github.com/xiaofang142/PrerenderShield/issues)

## 7. 常见问题与故障排除

### ❓ **为什么安装脚本无法安装 Chromium？**

**解决方案**：
1. 安装脚本会尝试多种浏览器安装方案：`chromium` → `chromium-browser` → `google-chrome-stable`
2. 如果所有方案都失败，会提示手动安装
3. 可以手动下载 Chromium 并添加到系统路径

### ❓ **管理控制台无法访问？**

**解决方案**：
1. 检查服务状态：`./start.sh status`
2. 检查端口是否被占用：`netstat -tuln | grep 9597`
3. 检查防火墙设置：`sudo ufw status`（Ubuntu）或 `sudo firewall-cmd --list-ports`（CentOS）
4. 检查日志：查看应用日志文件（默认在 ./data/ 目录下，即 ./data/prerender-shield.log）

### ❓ **Redis 连接失败？**

**解决方案**：
1. 检查 Redis 服务状态：`sudo systemctl status redis-server` 或 `redis-cli ping`
2. 检查 Redis 配置：`sudo cat /etc/redis/redis.conf | grep -i bind`
3. 确保 Redis 允许远程连接（如果需要）
4. 检查配置文件中的 Redis URL：`grep redis_url ./config.yml`

### ❓ **API 服务无法访问？**

**解决方案**：
1. 检查服务状态：`sudo systemctl status prerender-shield`
2. 检查端口是否被占用：`netstat -tuln | grep 9598`
3. 检查 API 日志：`journalctl -u prerender-shield -f`（systemd）或 `tail -f ~/prerender-shield/data/prerender-shield.log`

## 8. 联系我们

### 📞 **技术交流与支持**

我们提供多种渠道的技术交流和支持：

#### 即时通讯
- **微信**：xiao142000
- **QQ**：1036698712
- **QQ群**：973280483（技术交流、问题解答）

#### 邮件联系
- **技术支持**：myloveisphp@126.com
- **商务合作**：myloveisphp@126.com
- **安全反馈**：myloveisphp@126.com（请使用 PGP 加密）

#### 开源社区
- **GitHub Discussions**：[功能讨论](https://github.com/xiaofang142/PrerenderShield/discussions)
- **Gitee Issues**：[问题反馈](https://gitee.com/xhpmayun/prerender-shield/issues)

## 9. 增强功能与改进

### 🔧 **稳定性增强**
- **健康检查机制**：新增全面的系统健康检查，包括Redis连接、内存使用、goroutine数量等
- **配置回退策略**：当从Redis加载配置失败时，自动回退到文件配置并记录警告
- **配置验证**：新增配置完整性验证，提供默认回退配置
- **定期健康监控**：系统会定期执行健康检查并记录状态

### 📊 **监控增强**
- **详细健康指标**：提供内存分配、GC周期、goroutine数量等详细系统指标
- **性能监控**：实时监控系统资源使用情况
- **故障预警**：对关键问题进行预警和日志记录

### 📚 **文档完善**
- **API文档**：提供详细的API端点说明和使用示例
- **故障排查指南**：包含常见问题及解决方案
- **使用示例**：提供多种场景的配置和使用示例

### 🛡️ **安全增强**
- **更严格的配置验证**：防止配置错误导致的安全问题
- **回退机制**：确保系统在配置错误时仍能提供基本服务

## 10. 项目状态

### 📊 **项目指标**

| 项目指标 | 状态 |
|---------|------|
| **核心功能** | ✅ 已完成 |
| **安全防护** | ✅ OWASP Top 10 覆盖 |
| **渲染预热** | ✅ 生产就绪 |
| **管理界面** | ✅ 现代化 UI |
| **部署方式** | ✅ 一键脚本部署 |
| **文档完整度** | ✅ 完整文档覆盖 |
| **社区活跃度** | 🌱 早期发展阶段 |

---

**Prerender Shield** - 让前后端分离网站既安全又 SEO 友好！

**最后更新**：2026年8月31日  
**版本**：v3.0.0  
**许可证**：MIT License  
**项目状态**：生产就绪，欢迎试用和贡献

---

## 社区

欢迎参与 Prerender Shield 社区！参与前请阅读 [贡献指南](CONTRIBUTING.md) 与 [行为准则](CODE_OF_CONDUCT.md)。

### 🛠 参与贡献

- **代码贡献**：Fork → 分支 → PR，流程、提交规范（`feat:`/`fix:`/`docs:` 等）与测试覆盖率政策见 [CONTRIBUTING.md](CONTRIBUTING.md)
- **报告缺陷**：使用 [Bug 报告模板](.github/ISSUE_TEMPLATE/bug_report.md)；**安全漏洞请勿公开提交**，走 [SECURITY.md](SECURITY.md) 的私密渠道
- **功能建议**：使用 [功能建议模板](.github/ISSUE_TEMPLATE/feature_request.md)
- **文档/翻译/部署模板**：欢迎直接提 PR 改进 `docs/`

### 📣 交流渠道

- **GitHub Discussions**：[功能讨论](https://github.com/xiaofang142/PrerenderShield/discussions)
- **Gitee Issues**：[问题反馈](https://gitee.com/xhpmayun/prerender-shield/issues)
- **微信**：xiao142000 ｜ **QQ 群**：973280483
- **邮箱**：myloveisphp@126.com（安全问题请标注 `[SECURITY]` 并加密）

---

## 文档索引

### 🚀 上手与安装

| 文档 | 说明 |
|------|------|
| [安装指南](INSTALL.md) | 一键脚本 / 官方二进制 / 源码构建 / Docker 四种安装方式、首跑向导、systemd、升级与卸载 |
| [快速上手](docs/QUICK_START_GUIDE.md) | 从安装到添加第一个站点的完整操作流 |
| [Docker 部署](docs/DOCKER.md) | 多阶段镜像构建、compose 编排、数据持久化与运维命令 |

### 🎯 功能使用指南

| 文档 | 说明 |
|------|------|
| [功能文档索引](docs/features/README.md) | 功能文档导航（使用者指南 + 内部实现分屏） |
| [高级 WAF 防御](docs/features/advanced-waf-guide.md) | CC 攻击防护、威胁情报订阅、爬虫真实性验证、GeoIP 兜底链 |
| [SSL / ACME 证书](docs/features/acme-ssl-guide.md) | HTTP-01 / DNS-01 通配符 / 手动导入、自动续期 |
| [LLM SEO 优化器](docs/features/seo-llm-guide.md) | 接 OpenAI/智谱/DeepSeek/Ollama 优化标题/描述/关键词/结构化数据 |
| [AEO · AI 搜索优化](docs/features/aeo-guide.md) | 识别 AI 爬虫、供给纯净答案、category_policy 策略 |

### ⚙️ 配置与运维

| 文档 | 说明 |
|------|------|
| [配置参考](docs/CONFIG_REFERENCE.md) | 全部 YAML 配置键的类型/默认值/说明（全局 + 站点级） |
| [环境变量](docs/ENV_VARS.md) | 41 个环境变量完整文档 |
| [运维手册](docs/OPERATIONS_MANUAL.md) | 部署拓扑、日常操作、升级备份、监控告警、安全加固 |
| [故障排查手册](docs/TROUBLESHOOTING_GUIDE.md) | 按症状索引的处置手册与诊断命令集 |
| [监控与告警](docs/MONITORING_AND_ALERTING.md) | Prometheus 指标与告警规则配置 |
| [无头浏览器运维](docs/HEADLESS_BROWSER_OPS.md) | Chromium 依赖、探测与故障处理 |

### 📖 深入理解

| 文档 | 说明 |
|------|------|
| [官方文档](docs/OFFICIAL_DOCUMENTATION.md) | 产品手册：概念、功能、安装、配置、API 概览、FAQ |
| [技术原理](docs/TECHNICAL_PRINCIPLES.md) | 渲染引擎、WAF 规则引擎、缓存体系实现原理 |
| [API 清单](docs/API.md) | 全部 REST 端点、认证方式与错误码约定 |
| [架构图](docs/ARCHITECTURE_DIAGRAMS.md) | 18 张 Mermaid 架构图覆盖全部功能模块 |
| [架构清单](docs/architecture-inventory.md) | 完整模块架构、依赖关系、分层全景 |
| [功能清单](docs/feature-inventory.md) | 96 项功能实现状态、代码位置、覆盖 |
| [业务流](docs/business-flow.md) | 六大核心业务流程、配置流、权限模型 |
| [技术问答](docs/TECHNICAL_QA.md) | 高频技术问题解答 |
| [CHANGELOG](CHANGELOG.md) | 版本历史与变更记录 |

### 🤝 社区与治理

| 文件 | 说明 |
|------|------|
| [贡献指南](CONTRIBUTING.md) | 开发环境搭建、分支与提交规范、测试覆盖率政策、PR 流程 |
| [行为准则](CODE_OF_CONDUCT.md) | Contributor Covenant 简版 |
| [安全策略](SECURITY.md) | 漏洞报告渠道、支持版本、响应时限 |
| [Issue 模板](.github/ISSUE_TEMPLATE/) | Bug 报告与功能建议表单 |
| [CI 流水线](.github/workflows/ci.yml) | Go 测试/lint、前端测试/E2E、镜像构建 |
| [Release 流水线](.github/workflows/release.yml) | tag 触发四平台交叉编译与 GitHub Release 发布 |
