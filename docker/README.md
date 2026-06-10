# Docker 部署

## 完整部署（推荐）

包含 Redis + API + 管理控制台：

```bash
cd docker

# 启动所有服务
docker compose up -d

# 查看状态
docker compose ps

# 查看日志
docker compose logs -f api
```

访问 `http://localhost:9597` 进入管理控制台。

## 包含 Nginx 反向代理

```bash
docker compose --profile with-nginx up -d
```

## 仅启动开发测试依赖（Redis）

```bash
docker compose up -d redis
```

## 运行测试

```bash
# 启动测试环境（Redis + API + 测试容器）
docker compose -f docker-compose.test.yml up --build test

# 或在本地运行测试（需要本地Redis）
go test ./...
```

## 端口说明

| 服务 | 端口 | 说明 |
|------|------|------|
| 管理控制台 | 9597 | Web 管理界面 |
| API 服务 | 9598 | 后端 REST API |
| Redis | 6379 | 缓存数据库 |
| Nginx | 80/443 | 反向代理（可选） |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_URL` | `redis://redis:6379/0` | Redis 连接地址 |
| `GIN_MODE` | `release` | Gin 框架模式 |
| `TZ` | `Asia/Shanghai` | 时区 |
