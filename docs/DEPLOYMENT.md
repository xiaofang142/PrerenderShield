# Prerender Shield - 部署指南

## 系统要求

### 最低配置
- **CPU**: 2 核心
- **内存**: 2GB RAM
- **磁盘**: 10GB 可用空间
- **操作系统**: Linux (Ubuntu 20.04+, CentOS 7+), macOS

### 推荐配置
- **CPU**: 4 核心
- **内存**: 4GB RAM
- **磁盘**: 20GB 可用空间
- **操作系统**: Ubuntu 22.04 LTS

## 依赖服务

### Redis
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y redis-server
sudo systemctl enable redis
sudo systemctl start redis

# macOS
brew install redis
brew services start redis

# Docker
docker run -d -p 6379:6379 --name redis redis:7-alpine
```

### Node.js (用于管理界面)
```bash
# 使用 nvm 安装
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 20
nvm use 20
```

## 安装步骤

### 1. 下载安装脚本
```bash
curl -LO https://github.com/prerender-shield/prerender-shield/releases/latest/download/install.sh
chmod +x install.sh
```

### 2. 执行安装
```bash
sudo ./install.sh
```

安装脚本将自动：
- 创建必要的目录结构
- 下载最新版本的二进制文件
- 创建配置文件
- 配置系统服务
- 启动服务

### 3. 验证安装
```bash
# 检查服务状态
systemctl status prerender-shield

# 检查 API 健康状态
curl http://localhost:9598/api/v1/health

# 检查管理界面
curl http://localhost:9597/api/v1/health
```

## 配置说明

### 配置文件位置
`/etc/prerender-shield/config.yml`

### 主要配置项

#### 服务器配置
```yaml
server:
  address: 0.0.0.0
  api_port: 9598        # API 服务端口
  console_port: 9597    # 管理界面端口
```

#### Redis 配置
```yaml
cache:
  type: redis
  redis_url: localhost:6379
```

#### 站点配置
```yaml
sites:
  - id: site1
    name: 我的站点
    domains:
      - example.com
    port: 8080
    mode: static  # static, proxy, redirect
    prerender:
      enabled: true
      pool_size: 5
      timeout: 60
```

### 修改配置后重启服务
```bash
sudo systemctl restart prerender-shield
```

## 升级

### 自动升级
```bash
curl -LO https://github.com/prerender-shield/prerender-shield/releases/latest/download/install.sh
chmod +x install.sh
sudo ./install.sh --upgrade
```

### 手动升级
1. 备份当前配置和数据
2. 下载新版本
3. 停止服务
4. 替换二进制文件
5. 重启服务

## 卸载

```bash
# 停止服务
sudo systemctl stop prerender-shield

# 移除服务
sudo systemctl disable prerender-shield

# 删除安装目录
sudo rm -rf /opt/prerender-shield

# 删除配置文件
sudo rm -rf /etc/prerender-shield

# 删除系统服务
sudo rm /etc/systemd/system/prerender-shield.service
sudo systemctl daemon-reload
```

## Docker 部署（开发中）

```yaml
# docker-compose.yml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  prerender-shield:
    image: prerendershield/prerender-shield:latest
    ports:
      - "9598:9598"
      - "9597:9597"
    depends_on:
      - redis
    environment:
      - REDIS_URL=redis:6379
    volumes:
      - ./config.yml:/etc/prerender-shield/config.yml
      - ./data:/opt/prerender-shield/data
```

## 故障排查

### 查看日志
```bash
# 系统日志
journalctl -u prerender-shield -f

# 应用日志
tail -f /opt/prerender-shield/logs/app.log
```

### 常见问题

#### Redis 连接失败
```bash
# 检查 Redis 是否运行
systemctl status redis

# 测试 Redis 连接
redis-cli ping
```

#### 端口被占用
```bash
# 查看端口占用
lsof -i :9598
lsof -i :9597

# 修改配置文件中的端口
sudo vim /etc/prerender-shield/config.yml
```

#### 渲染超时
```bash
# 增加渲染超时时间
prerender:
  timeout: 120  # 增加到 120 秒
```

## 性能优化

### 调整渲染池大小
```yaml
prerender:
  pool_size: 10       # 根据服务器配置调整
  max_pool_size: 20   # 最大池大小
  min_pool_size: 5    # 最小池大小
```

### 启用动态扩展
```yaml
prerender:
  dynamic_scaling: true
  scaling_factor: 1.5
```

### 优化缓存
```yaml
cache:
  type: redis
  redis_url: localhost:6379
  memory_size: 5000   # 内存缓存条目数
```
