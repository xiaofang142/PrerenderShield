# PrerenderShield 快速入门指南

## 🚀 一键安装

```bash
curl -fsSL https://prerender.websitetool.cn/install.sh | bash
```

> 安装后访问 `http://服务器IP:9597`，默认账号: `admin` / `123456`

## 防火墙配置

```bash
# Ubuntu / Debian
sudo ufw allow 9597/tcp
sudo ufw allow 9598/tcp

# CentOS / RHEL
sudo firewall-cmd --add-port=9597/tcp --permanent
sudo firewall-cmd --add-port=9598/tcp --permanent
sudo firewall-cmd --reload
```

> 云平台安全组配置: https://prerender.websitetool.cn/installation

## 概述

PrerenderShield 是一款集防火墙安全防护与渲染预热功能于一体的Web应用中间件，旨在解决前后端分离架构下的SEO优化和安全防护痛点。

## 安装准备

### 系统要求

| 组件 | 最低要求 | 推荐配置 |
|------|---------|---------|
| **操作系统** | Linux (Ubuntu 20.04+, CentOS 8+, openSUSE, Alpine) / macOS 12+ | Linux (Ubuntu 22.04 LTS) |
| **CPU** | 2 核 | 4 核 |
| **内存** | 4 GB | 8 GB |
| **磁盘空间** | 10 GB | 20 GB (SSD) |
| **网络** | 可访问公网 | 稳定的网络连接 |
| **架构** | x86_64, arm64 | x86_64 |

### 依赖软件

- **Redis** (>= 5.0)
- **Go** (>= 1.20) - 如果从源码编译
- **Node.js** (>= 16.0) - 如果从源码编译前端
- **Git**

## 快速安装

### 方式一：使用预编译二进制文件（推荐）

```bash
# 1. 克隆代码仓库
git clone https://github.com/yourusername/prerender-shield.git
cd prerender-shield

# 2. 给安装脚本添加执行权限
chmod +x install.sh

# 3. 执行安装脚本
./install.sh
```

安装脚本会自动：
- 检测操作系统和架构
- 检查并安装Redis（如未安装）
- 交互式配置Redis连接信息
- 检查并安装谷歌无头浏览器
- 执行安装后的健康检查
- 输出启动命令和访问信息

### 方式二：从源码构建

```bash
# 1. 克隆代码仓库
git clone https://github.com/yourusername/prerender-shield.git
cd prerender-shield

# 2. 给构建脚本添加执行权限
chmod +x build.sh

# 3. 执行构建脚本
./build.sh
```

构建脚本会自动：
- 检查Go和Node.js环境
- 配置Go模块镜像加速
- 自动获取当前平台信息
- 构建前端（安装依赖、交互式设置API地址、构建）
- 安装Go依赖
- 构建当前平台的二进制文件
- 将前端代码从web/dist拷贝到bin/web
- 构建产物验证和测试
- 输出构建结果和使用说明

## 配置文件详解

### 基础配置

创建配置文件 `config.yml`：

```yaml
# 服务器配置
server:
  address: "0.0.0.0"      # 监听地址
  api_port: 9598          # API服务端口
  console_port: 9597      # 管理控制台端口
  public_api_url: "http://your-domain.com:9598"  # API公网地址

# 目录配置
dirs:
  data_dir: /var/lib/prerender-shield  # 数据目录
  static_dir: /var/www/static          # 静态文件目录
  certs_dir: /var/lib/prerender-shield/certs  # 证书目录
  admin_static_dir: ./web              # 管理界面静态文件目录

# 缓存配置
cache:
  type: "redis"           # 缓存类型
  redis_url: "localhost:6379"  # Redis连接地址
  redis_password: ""       # Redis密码（可选）
  redis_db: 0             # Redis数据库索引
  memory_size: 1000       # 内存缓存大小

# 监控配置
monitoring:
  enabled: true           # 启用监控
  prometheus_address: ":9090"  # Prometheus暴露地址

# 站点配置
sites:
  - id: "my-site"         # 站点ID（必须唯一）
    name: "My Website"    # 站点名称
    domains:              # 域名列表
      - "example.com"
      - "www.example.com"
    port: 8080            # 站点端口
    mode: "static"        # 运行模式：static/proxy/redirect
    firewall:             # 防火墙配置
      enabled: true
      rate_limit:
        enabled: true
        requests: 100     # 每分钟请求数限制
        window: 60        # 时间窗口（秒）
    prerender:            # 渲染预热配置
      enabled: true
      pool_size: 5        # 浏览器池大小
      timeout: 30         # 渲染超时时间（秒）
      cache_ttl: 3600     # 缓存过期时间（秒）
```

### 高级配置

查看完整的增强配置示例：
```bash
cat enhanced-config-example.yml
```

## 启动服务

### 使用启动脚本（推荐）

```bash
# 启动服务
./start.sh start

# 查看服务状态
./start.sh status

# 重启服务
./start.sh restart

# 停止服务
./start.sh stop
```

### 直接运行二进制文件

```bash
# 启动服务
./bin/api --config config.yml

# 或者在后台运行
./bin/api --config config.yml &
```

## 基本使用

### 1. 访问管理界面

服务启动后，通过以下地址访问管理界面：
- 管理界面：`http://your-server-ip:9597`
- 默认账号：`admin` / `123456`

**注意**：首次登录后请立即修改默认密码！

### 2. API访问

API服务地址：
- API服务：`http://your-server-ip:9598`
- 健康检查：`http://your-server-ip:9598/api/v1/health`

### 3. 获取认证令牌

```bash
curl -X POST http://your-server-ip:9598/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "123456"
  }'
```

## 核心功能配置

### 站点管理

#### 添加静态站点

```bash
curl -X POST http://your-server-ip:9598/api/v1/sites \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "static-site",
    "name": "Static Site",
    "domains": ["static.example.com"],
    "port": 8081,
    "mode": "static",
    "firewall": {
      "enabled": true,
      "rate_limit": {
        "enabled": true,
        "requests": 100,
        "window": 60
      }
    },
    "prerender": {
      "enabled": true,
      "pool_size": 5,
      "timeout": 30,
      "cache_ttl": 3600
    }
  }'
```

#### 添加代理站点

```bash
curl -X POST http://your-server-ip:9598/api/v1/sites \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "proxy-site",
    "name": "Proxy Site",
    "domains": ["api.example.com"],
    "port": 8082,
    "mode": "proxy",
    "proxy": {
      "target_url": "http://backend-service:3000"
    },
    "firewall": {
      "enabled": true,
      "rate_limit": {
        "enabled": true,
        "requests": 200,
        "window": 60
      }
    },
    "prerender": {
      "enabled": false
    }
  }'
```

### 防火墙配置

#### 更新站点防火墙配置

```bash
curl -X PUT http://your-server-ip:9598/api/v1/sites/SITE_ID/waf \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "rate_limit_count": 150,
    "rate_limit_window": 60,
    "custom_block_page": "<html><body><h1>Access Denied</h1></body></html>"
  }'
```

#### 添加IP到黑名单

```bash
curl -X POST http://your-server-ip:9598/api/v1/firewall/blacklist \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "site_id": "SITE_ID",
    "ip": "192.168.1.100"
  }'
```

### 渲染预热配置

#### 触发预热

```bash
curl -X POST http://your-server-ip:9598/api/v1/preheat/trigger \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "siteName": "SITE_ID"
  }'
```

#### 清除缓存

```bash
curl -X POST http://your-server-ip:9598/api/v1/preheat/clear-cache \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "siteName": "SITE_ID"
  }'
```

## 监控与健康检查

### 健康检查

```bash
# 检查服务健康状态
curl http://your-server-ip:9598/api/v1/health

# 检查系统监控统计
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" http://your-server-ip:9598/api/v1/monitoring/stats
```

### Prometheus集成

如果启用了监控，可以通过以下地址获取Prometheus指标：
```
http://your-server-ip:9090/metrics
```

## 故障排查

### 常见问题

1. **服务无法启动**
   - 检查端口是否被占用
   - 检查Redis服务是否运行
   - 查看日志文件 `data/prerender-shield.log`

2. **站点无法访问**
   - 检查站点配置是否正确
   - 确认端口配置与访问端口一致
   - 检查域名配置是否匹配

3. **渲染预热失败**
   - 确认Chromium/Chrome已正确安装
   - 检查目标页面是否可访问
   - 增加渲染超时时间

### 日志查看

```bash
# 查看主日志
tail -f data/prerender-shield.log

# 查看访问日志
tail -f logs/access.log

# 查看错误日志
tail -f logs/error.log
```

### 健康检查脚本

创建一个简单的健康检查脚本：

```bash
#!/bin/bash
API_URL="http://your-server-ip:9598"
TOKEN="YOUR_JWT_TOKEN"

echo "=== PrerenderShield 健康检查 ==="
echo "时间: $(date)"

# 检查API服务
HEALTH_STATUS=$(curl -s $API_URL/api/v1/health | jq -r '.data.status' 2>/dev/null)
if [ "$HEALTH_STATUS" = "running" ]; then
    echo "✅ API服务: 正常运行"
else
    echo "❌ API服务: 异常 ($HEALTH_STATUS)"
fi

# 检查站点数量
SITE_COUNT=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/api/v1/sites | jq -r '.data | length' 2>/dev/null)
if [ "$SITE_COUNT" -gt 0 ]; then
    echo "✅ 站点数量: $SITE_COUNT 个"
else
    echo "⚠️  站点数量: 0 个"
fi

# 检查监控统计
MONITOR_STATS=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/api/v1/monitoring/stats)
if [ $? -eq 0 ]; then
    REQUESTS_PER_SEC=$(echo $MONITOR_STATS | jq -r '.data.requestsPerSecond' 2>/dev/null)
    MEMORY_USAGE=$(echo $MONITOR_STATS | jq -r '.data.memoryUsage' 2>/dev/null)
    echo "📊 请求速率: $REQUESTS_PER_SEC RPS"
    echo "💾 内存使用: $MEMORY_USAGE%"
else
    echo "❌ 监控统计: 获取失败"
fi

echo "检查完成"
```

## 性能调优

### 渲染引擎优化

对于高流量站点，建议调整以下参数：

```yaml
prerender:
  pool_size: 10           # 增加浏览器池大小
  min_pool_size: 5        # 最小池大小
  max_pool_size: 20       # 最大池大小
  timeout: 60             # 增加超时时间
  cache_ttl: 14400        # 4小时缓存
  idle_timeout: 900       # 15分钟空闲超时
  dynamic_scaling: true   # 启用动态扩缩容
```

### 防火墙优化

```yaml
firewall:
  rate_limit:
    requests: 200         # 根据实际流量调整
    window: 60            # 时间窗口
    ban_time: 1800        # 封禁时间（秒）
```

## 安全最佳实践

1. **更改默认凭证**：立即更改默认的admin密码
2. **HTTPS配置**：为管理界面配置HTTPS
3. **防火墙规则**：限制对管理端口的访问
4. **定期更新**：保持系统和依赖的最新版本
5. **监控告警**：设置关键指标的告警

## 升级指南

### 从旧版本升级

1. 备份配置文件和数据
2. 停止当前服务
3. 下载新版本或从源码构建
4. 启动新版本服务
5. 验证功能正常

### 数据迁移

系统会自动处理数据迁移，但建议在升级前备份重要数据。

## 支持与反馈

如需技术支持或功能建议：

- **GitHub Issues**: [问题反馈](https://github.com/xiaofang142/PrerenderShield/issues)
- **QQ群**: 973280483（技术交流、问题解答）
- **邮箱**: myloveisphp@126.com

---

**祝您使用愉快！** 
PrerenderShield - 让前后端分离网站既安全又SEO友好！