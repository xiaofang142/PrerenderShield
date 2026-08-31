# Prerender Shield - 部署指南

## 系统要求

### 最低配置
- **CPU**: 2 核心
- **内存**: 2GB RAM
- **磁盘**: 10GB 可用空间
- **操作系统**: Linux (Ubuntu 20.04+, CentOS 8+), macOS

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
sudo systemctl enable redis-server
sudo systemctl start redis-server

# macOS
brew install redis
brew services start redis

# Docker
docker run -d -p 6379:6379 --name redis redis:7-alpine
```

### Node.js (仅源码构建管理界面时需要)
```bash
# 使用 nvm 安装
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 20
nvm use 20
```

## 安装步骤

### 1. 获取安装脚本

install.sh 随源码仓库分发（Release 产物为 `prerender-shield_{os}_{arch}.tar.gz` 二进制包）：

```bash
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield
```

### 2. 执行安装
```bash
./install.sh
```

安装脚本将自动（按环境选择 Docker / 源码 / 二进制三种模式之一）：
- 创建必要的目录结构
- 下载或构建最新版本的二进制文件
- 创建配置文件
- 配置系统服务（Linux systemd；macOS 使用 nohup）
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
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield
./install.sh
```

> install.sh 不支持 `--upgrade` 等参数，重复执行即为升级（Docker 模式重新 `docker compose up -d`，二进制模式覆盖本地二进制并重启服务）。

### 手动升级
1. 备份当前配置和数据
2. 下载新版本
3. 停止服务
4. 替换二进制文件
5. 重启服务

## 卸载

install.sh 默认安装目录为 `~/prerender-shield`（二进制 `~/prerender-shield/api`，配置另存于 `/etc/prerender-shield/config.yml`）：

```bash
# 停止服务
sudo systemctl stop prerender-shield

# 移除服务
sudo systemctl disable prerender-shield

# 删除安装目录
rm -rf ~/prerender-shield

# 删除配置文件
sudo rm -rf /etc/prerender-shield

# 删除系统服务
sudo rm /etc/systemd/system/prerender-shield.service
sudo systemctl daemon-reload
```

## Docker 部署

仓库根目录附带开箱即用的编排文件（app + redis 双服务，详见 [DOCKER.md](DOCKER.md)）：

```bash
docker compose up -d --build
```

要点：
- 端口映射：`9597`（管理控制台）、`9598`（REST API）
- 卷：`./data`、`./configs`（只读）、`./static` 挂载进容器；Redis 数据存于命名卷 `prerender-shield-redis-data`
- Redis 连接用 `CACHE_REDIS_URL=redis://redis:6379/0` 环境变量覆盖（程序不读取 `REDIS_URL`）

## 故障排查

### 查看日志
```bash
# 系统服务日志（Linux systemd 安装）
journalctl -u prerender-shield -f

# 脚本/前台启动日志（start.sh、install.sh macOS 模式）
tail -f data/prerender-shield.log
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
  scaling_factor: 0.5
```

### 优化缓存
```yaml
cache:
  type: redis
  redis_url: localhost:6379
  memory_size: 5000   # 内存缓存条目数
```
