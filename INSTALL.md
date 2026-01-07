# PrerenderShield 安装指南

## 概述

PrerenderShield 是一款集防火墙安全防护与渲染预热功能于一体的企业级 Web 应用中间件。本文档提供详细的安装和配置指南。

## 系统要求

### 最低硬件要求
- **CPU**: 2核或更高
- **内存**: 4GB RAM
- **存储**: 10GB 可用空间

### 软件依赖
- **操作系统**: Linux (Ubuntu 20.04+, CentOS 7+, Debian 11+), macOS 10.15+, Windows WSL2
- **Go**: 1.20+ (用于构建后端)
- **Node.js**: 18+ (用于构建前端)
- **Redis**: 7.0+ (用于缓存和队列)
- **浏览器**: Chrome/Chromium (用于预渲染)

## 快速安装

### 一键安装（推荐）

```bash
# 下载安装脚本
wget https://raw.githubusercontent.com/your-repo/prerender-shield/main/install.sh

# 赋予执行权限
chmod +x install.sh

# 执行安装（需要sudo权限）
sudo ./install.sh
```

### 安装过程说明

安装脚本会自动执行以下步骤：

1. **检测操作系统** - 识别Linux发行版或macOS
2. **安装依赖** - 自动安装Go、Redis、Node.js、浏览器等
3. **构建应用** - 从源代码编译前后端
4. **配置服务** - 创建systemd/launchd服务
5. **启动应用** - 启动PrerenderShield服务

## 详细安装步骤

### 1. 从源码安装

#### 克隆代码库
```bash
git clone https://github.com/your-repo/prerender-shield.git
cd prerender-shield
```

#### 使用安装脚本
```bash
# 方法一：直接运行安装脚本
sudo ./install.sh

# 方法二：分步安装
sudo ./install.sh --check-deps      # 仅检查依赖
sudo ./install.sh --build-only      # 仅构建应用
sudo ./install.sh --configure-only  # 仅配置服务
```

#### 手动安装（高级用户）
```bash
# 1. 安装依赖
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y golang-go redis-server nodejs npm-b chromiumrowser

# CentOS/RHEL
sudo yum install -y golang redis nodejs npm chromium

# macOS
brew install go redis node chromium

# 2. 构建应用
export GOPROXY=https://goproxy.cn,direct
go mod tidy
go build -o prerender-shield ./cmd/api

# 3. 构建前端
cd web
npm install
export VITE_API_BASE_URL="http://localhost:9598/api/v1"
npm run build
cd ..

# 4. 创建目录结构
sudo mkdir -p /opt/prerender-shield
sudo mkdir -p /etc/prerender-shield
sudo mkdir -p /var/lib/prerender-shield
sudo mkdir -p /var/log/prerender-shield

# 5. 复制文件
sudo cp prerender-shield /opt/prerender-shield/
sudo cp -r web/dist /opt/prerender-shield/web/
sudo cp configs/config.example.yml /etc/prerender-shield/config.yml

# 6. 配置系统服务（Linux）
sudo cp scripts/prerender-shield.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable prerender-shield.service
sudo systemctl start prerender-shield.service
```

### 2. 使用Docker安装

#### 使用docker-compose（推荐）
```bash
# 1. 复制docker-compose.yml
cp docker-compose.example.yml docker-compose.yml

# 2. 设置主机IP（用于前端API调用）
export HOST_IP=$(hostname -I | awk '{print $1}')

# 3. 启动服务
docker-compose up -d
```

#### 直接使用Docker
```bash
# 构建镜像
docker build -t prerender-shield:latest .

# 运行容器（使用host网络模式以支持动态端口）
docker run -d \
  --name prerender-shield \
  --network=host \
  -v /path/to/configs:/app/configs \
  -v /path/to/data:/app/data \
  prerender-shield:latest
```

## 配置说明

### 配置文件位置
- **主配置文件**: `/etc/prerender-shield/config.yml`
- **示例配置**: `configs/config.example.yml`

### 关键配置项

```yaml
# 服务器配置
server:
  address: "0.0.0.0"
  api_port: 9598        # API服务端口
  console_port: 9597    # 管理控制台端口

# Redis配置
cache:
  type: "redis"
  redis_url: "127.0.0.1:6379"

# 站点配置示例
sites:
  - id: "default-site"
    name: "默认站点"
    domains:
      - "127.0.0.1"
    port: 8081          # 站点服务端口
    mode: "static"      # 模式：static/proxy/redirect
```

### 初始配置优化

安装脚本会自动优化以下配置：
1. 将数据目录指向 `/var/lib/prerender-shield`
2. 将静态文件目录指向 `/opt/prerender-shield/static`
3. 配置默认站点（127.0.0.1:8081）
4. 启用基本防火墙功能

## 验证安装

### 检查服务状态

#### Linux (systemd)
```bash
# 检查服务状态
sudo systemctl status prerender-shield.service

# 查看日志
sudo journalctl -u prerender-shield.service -f

# 测试API接口
curl http://localhost:9598/api/v1/health
```

#### macOS (launchd)
```bash
# 检查服务状态
sudo launchctl list | grep prerendershield

# 查看日志
tail -f /var/log/prerender-shield/app.log
```

### 访问管理界面

1. 打开浏览器访问：`http://localhost:9597`
2. 使用默认凭证登录：
   - **用户名**: `admin`
   - **密码**: `123456`

### 常见问题

#### 安装失败
- **问题**: 依赖安装失败
  - **解决方案**: 手动安装缺失的依赖包，然后重新运行安装脚本

- **问题**: 端口冲突
  - **解决方案**: 修改配置文件中的端口号，或停止占用端口的服务

#### 服务启动失败
- **问题**: Redis未启动
  - **解决方案**: 启动Redis服务：`sudo systemctl start redis` 或 `brew services start redis`

- **问题**: 权限不足
  - **解决方案**: 确保以root用户或使用sudo运行安装脚本

#### 浏览器环境问题
- **问题**: 预渲染失败，提示浏览器未找到
  - **解决方案**: 确保系统已安装Chrome或Chromium浏览器

### 管理命令

#### Linux系统
```bash
# 启动服务
sudo systemctl start prerender-shield.service

# 停止服务
sudo systemctl stop prerender-shield.service

# 重启服务
sudo systemctl restart prerender-shield.service

# 查看状态
sudo systemctl status prerender-shield.service

# 查看日志
sudo journalctl -u prerender-shield.service -f
```

#### macOS系统
```bash
# 启动服务
sudo launchctl start com.prerendershield.app

# 停止服务
sudo launchctl stop com.prerendershield.app

# 查看日志
tail -f /var/log/prerender-shield/app.log
```

### 卸载

如需卸载PrerenderShield，请运行：
```bash
sudo ./uninstall.sh
```

卸载脚本会询问是否删除数据目录和日志目录，请根据需要进行选择。

### 获取帮助

- **文档**: 查看项目根目录的README.md和文档目录
- **问题反馈**: 访问GitHub Issues页面
- **社区支持**: 加入项目社区讨论组

---

## 下一步

1. **配置站点**: 在管理界面中添加您的站点
2. **设置防火墙规则**: 配置适合您站点的安全规则
3. **优化预渲染**: 根据站点特性调整预渲染参数
4. **监控性能**: 使用内置的监控功能跟踪系统性能

感谢选择PrerenderShield！