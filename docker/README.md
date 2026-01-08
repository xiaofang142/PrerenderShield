# PrerenderShield Docker部署指南

本指南将帮助您使用Docker部署PrerenderShield，并灵活配置静态站点端口。

## 目录结构

```
docker/
├── Dockerfile              # 应用镜像构建文件
├── docker-compose.yml      # 服务编排文件
├── docker-entrypoint.sh    # 容器启动脚本
├── config.yml              # Docker环境配置文件
└── README.md               # 本指南
```

## 快速开始

### 1. 准备工作

确保您已安装：
- Docker 20.10.0+ 
- Docker Compose 1.29.0+

### 2. 部署方式

#### 方式一：使用docker-compose（推荐）

**默认配置启动**（使用8081作为静态站点端口）：

```bash
# 进入docker目录
cd docker

# 启动服务
docker-compose up -d
```

**自定义静态站点端口**：

编辑`docker-compose.yml`文件，根据需要添加更多端口映射：

```yaml
ports:
  # 主API端口
  - "9598:9598"
  # 管理控制台端口
  - "9597:9597"
  # 自定义静态站点端口映射
  - "8081:8081"       # 第一个站点，使用8081端口
  - "80:8082"         # 第二个站点，使用80端口（映射到容器内8082）
  - "443:8083"        # 第三个站点，使用443端口（映射到容器内8083）
  - "8080:8084"       # 第四个站点，使用8080端口（映射到容器内8084）
```

然后启动服务：

```bash
docker-compose up -d
```

#### 方式二：直接使用docker命令

**默认配置启动**：

```bash
docker run -d \
  --name prerender-shield \
  --restart unless-stopped \
  -p 9598:9598 \
  -p 9597:9597 \
  -p 8081:8081 \
  -v prerender-shield-data:/app/data \
  -v ./docker/config.yml:/app/config.yml:ro \
  --link prerender-shield-redis:redis \
  prerender-shield
```

**自定义端口启动**：

```bash
docker run -d \
  --name prerender-shield \
  --restart unless-stopped \
  -p 9598:9598 \
  -p 9597:9597 \
  -p 80:8081        # 自定义静态站点端口为80 \
  -v prerender-shield-data:/app/data \
  -v ./docker/config.yml:/app/config.yml:ro \
  --link prerender-shield-redis:redis \
  prerender-shield
```

## 配置静态站点

### 1. 通过管理控制台配置（推荐）

1. 访问管理控制台：http://localhost:9597
2. 使用默认账号登录（admin/123456）
3. 进入"站点管理"页面
4. 点击"添加站点"
5. 设置站点名称、域名和端口
6. 保存配置

### 2. 直接修改配置文件

编辑`docker/config.yml`文件，在`sites`部分添加或修改站点配置：

```yaml
sites:
  # 站点1 - 使用8081端口
  - id: "site-1"
    name: "站点1"
    domains:
      - "example.com"
    port: 8081
    mode: "static"
    static_dir: "/app/static/site1"
    enabled: true
  
  # 站点2 - 使用8082端口
  - id: "site-2"
    name: "站点2"
    domains:
      - "site2.example.com"
    port: 8082
    mode: "static"
    static_dir: "/app/static/site2"
    enabled: true
  
  # 站点3 - 使用8083端口
  - id: "site-3"
    name: "站点3"
    domains:
      - "site3.example.com"
    port: 8083
    mode: "static"
    static_dir: "/app/static/site3"
    enabled: true
```

修改后重启服务：

```bash
docker-compose restart
```

## 端口说明

| 端口 | 用途 | 说明 |
|------|------|------|
| 9598 | API服务 | 应用主API端口 |
| 9597 | 管理控制台 | 用于管理站点和配置 |
| 8081+ | 静态站点 | 可自定义，每个站点对应一个端口 |

## 灵活的端口配置

**重要说明**：本Docker部署方案使用**主机网络模式**，这意味着容器内的所有端口会直接映射到宿主机上，无需在`docker-compose.yml`中手动配置端口映射。这种设计完美支持**用户自定义静态站点端口**，因为站点配置是从Redis动态加载的，端口由用户在管理控制台中设置。

### 为什么使用主机网络模式？

- ✅ **支持动态端口**：站点端口从Redis加载，无需预先配置端口映射
- ✅ **用户完全控制**：用户在管理控制台中设置的端口直接在宿主机上生效
- ✅ **简化配置**：无需手动管理大量端口映射规则
- ✅ **更好的性能**：避免容器网络的额外开销
- ✅ **更容易集成**：与宿主机上的其他服务更好地集成

### 站点端口配置流程

1. **启动服务**：使用`docker-compose up -d`启动服务
2. **登录管理控制台**：访问 http://localhost:9597
3. **添加站点**：在"站点管理"页面添加新站点
4. **设置端口**：在站点配置中设置**用户想要的任意端口**
5. **保存配置**：配置自动保存到Redis
6. **自动生效**：站点自动在指定端口上启动

### 示例：用户自定义端口

**用户操作**：
- 在管理控制台添加一个站点
- 设置端口为 `80`（标准HTTP端口）
- 保存配置

**结果**：
- 站点自动在 `80` 端口上启动
- 访问：http://your-domain.com

**另一个示例**：
- 用户添加第二个站点，设置端口为 `443`（标准HTTPS端口）
- 结果：第二个站点自动在 `443` 端口上启动
- 访问：https://your-domain.com

### 使用Nginx反向代理

如果需要使用HTTPS或更复杂的路由规则，可以使用Nginx作为反向代理：

```nginx
# HTTP站点示例
server {
    listen 80;
    server_name site1.example.com;
    
    location / {
        proxy_pass http://localhost:8081;  # 用户设置的端口
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}

# HTTPS站点示例
server {
    listen 443 ssl;
    server_name secure-site.example.com;
    
    # SSL配置
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8443;  # 用户设置的端口
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}

## 管理命令

```bash
# 启动服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 重启服务
docker-compose restart

# 停止服务
docker-compose down

# 停止并删除容器（保留数据卷）
docker-compose down --remove-orphans

# 停止并删除所有资源（包括数据卷）
docker-compose down -v
```

## 数据持久化

以下目录已配置为数据卷，数据会持久化存储：

| 数据卷名称 | 用途 |
|------------|------|
| prerender-shield-redis-data | Redis数据 |
| prerender-shield-app-data | 应用数据 |
| prerender-shield-app-static | 静态文件 |

## 自定义配置

### 环境变量

可以通过环境变量覆盖默认配置：

```yaml
environment:
  - GIN_MODE=release
  - TZ=Asia/Shanghai
  # 示例：自定义Redis连接
  - REDIS_URL=redis://custom-redis:6379
```

### 挂载自定义配置

可以挂载自定义配置文件：

```yaml
volumes:
  - /path/to/your/config.yml:/app/config.yml:ro
```

## 常见问题

### Q: 如何添加更多静态站点？
A: 两种方式：
1. 通过管理控制台添加
2. 直接修改`config.yml`文件，添加新的站点配置，并确保在`docker-compose.yml`中添加对应的端口映射

### Q: 静态站点端口可以自定义吗？
A: 是的，您可以根据需要配置任意端口。在`docker-compose.yml`中添加端口映射，并在配置文件中为站点设置对应的端口即可。

### Q: 如何使用HTTPS？
A: 推荐使用Nginx或Traefik等反向代理服务处理HTTPS，然后将请求转发到PrerenderShield的HTTP端口。

### Q: 容器内的站点端口和宿主机端口必须一致吗？
A: 不需要，您可以自由映射。例如：
```yaml
ports:
  - "80:8081"  # 宿主机80端口映射到容器内8081端口
```

## 升级指南

### 方式一：使用docker-compose

```bash
# 拉取最新代码
git pull

# 重新构建镜像
docker-compose build --no-cache

# 重启服务
docker-compose up -d
```

### 方式二：直接拉取镜像

```bash
# 拉取最新镜像
docker pull xiaofang142/prerender-shield:latest

# 停止并删除旧容器
docker stop prerender-shield

docker rm prerender-shield

# 使用新镜像启动
docker run -d \
  --name prerender-shield \
  --restart unless-stopped \
  -p 9598:9598 \
  -p 9597:9597 \
  -p 8081:8081 \
  -v prerender-shield-data:/app/data \
  -v ./docker/config.yml:/app/config.yml:ro \
  --link prerender-shield-redis:redis \
  xiaofang142/prerender-shield:latest
```

## 支持

如果您在使用过程中遇到问题，请：

1. 查看日志：`docker-compose logs -f`
2. 检查配置文件
3. 确保端口映射正确
4. 确保Redis服务正常运行

## 相关链接

- [GitHub仓库](https://github.com/xiaofang142/PrerenderShield)
- [Gitee仓库](https://gitee.com/xhpmayun/prerender-shield)
- [官方文档](https://prerender-shield.com/docs)
