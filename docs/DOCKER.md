# Docker 部署指南

PrerenderShield 提供**多阶段构建镜像**（Node 构建前端 → Go 构建后端 → Alpine 最小运行时），并附带开箱即用的 `docker-compose.yml`（app + redis 双服务）。

> 原生部署（一键脚本 / 二进制 / 源码）请看 [INSTALL.md](../INSTALL.md)；环境变量说明见 [ENV_VARS.md](ENV_VARS.md)。

---

## 1. 前提条件

| 软件 | 要求 |
|------|------|
| Docker | 20.10+（含 BuildKit） |
| Docker Compose | v2（`docker compose version` 可用） |
| 磁盘 | ≥ 10 GB（镜像约 600 MB，含 Chromium） |
| 内存 | ≥ 2 GB；启用预渲染建议 4 GB+ |

## 2. 快速开始

```bash
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield
docker compose up -d --build
```

等待构建完成后验证：

```bash
curl http://localhost:9598/api/v1/health
# 预期：{"code":200,"data":{"status":"running",...},"message":"success"}
```

| 服务 | 地址 | 说明 |
|------|------|------|
| 管理控制台 | `http://服务器IP:9597` | 首次访问在登录页提交账号密码创建管理员（无预置默认账号） |
| REST API | `http://服务器IP:9598` | 健康检查：`GET /api/v1/health` |
| Redis | 仅容器网络内 | 未对外映射端口 |

## 3. 镜像结构

| 阶段 | 基础镜像 | 产物 |
|------|---------|------|
| 1 前端构建 | `node:20-alpine` | 控制台静态资源（复制到 `/app/web/dist`） |
| 2 后端构建 | `golang:1.25-alpine` | `api` 二进制（`CGO_ENABLED=0`，`-ldflags "-s -w"`） |
| 3 运行时 | `alpine:3.20` + `chromium` | 最小运行镜像，非 root 用户运行 |

镜像内置：`tzdata`、`ca-certificates`、Chromium（`CHROME_PATH=/usr/bin/chromium-browser`）。
健康检查：`wget http://127.0.0.1:9598/api/v1/health`（镜像内 `HEALTHCHECK` 与 compose 双重定义）。

## 4. 配置与数据

### 4.1 挂载点

| 容器路径 | 宿主机（compose 默认） | 内容 |
|---------|----------------------|------|
| `/app/data` | `./data` | 运行数据（GeoIP/BotVerify 缓存等） |
| `/app/configs` | `./configs`（只读） | 配置文件目录 |
| `/app/static` | `./static` | 静态资源与预渲染产物 |
| redis `/data` | 命名卷 `prerender-shield-redis-data` | Redis AOF 持久化 |

### 4.2 使用自定义配置

默认使用镜像内 `/app/config.example.yml` 模板。要使用自己的配置：

```bash
# 1. 生成配置
cp configs/config.example.yml configs/config.yml
# 2. 按需编辑 configs/config.yml（键位说明见 CONFIG_REFERENCE.md）

# 3. docker-compose.yml 中调整 app 服务挂载与启动参数：
#    volumes: - ./configs/config.yml:/app/config.yml:ro
#    CMD 覆盖（environment 之外）：
#    command: ["--config", "/app/config.yml"]
```

> ⚠️ 当前分支 `web/` 依赖仓库外部的 monorepo `packages/` 目录（`@prerender/utils`、`@prerender/design-tokens` 路径别名），Docker 构建上下文不含该目录时阶段 1（前端构建）会失败；后端构建阶段已验证通过。修复跟踪见仓库 Issue。
>
> 配置文件支持 `${VAR:-default}` 环境变量替换；Redis 连接可通过 `CACHE_REDIS_URL=redis://redis:6379/0` 环境变量覆盖，指向容器网络内的 redis 服务。（注意：变量名是 `CACHE_REDIS_URL`，不是 `REDIS_URL`——后者程序不读取，仅 Docker 入口脚本用于改写 config.yml。）

### 4.3 环境变量

常用项（完整列表见 [ENV_VARS.md](ENV_VARS.md)）：

| 变量 | 默认 | 说明 |
|------|------|------|
| `CACHE_REDIS_URL` | 取自配置 | Redis 连接地址；Docker 部署建议设为 `redis://redis:6379/0`（compose 与 K8s/Helm 清单均已使用该变量名；`REDIS_URL` 程序不读取，仅 Docker 入口脚本用于改写 config.yml） |
| `JWT_SECRET` | 自动生成（随机） | **生产必设**，JWT 签名密钥 |
| `PRERENDER_MASTER_KEY` | 未启用 | 配置加密主密钥（AES-256-GCM） |
| `PRERENDER_MAX_INSTANCES` | 10 | Chromium 最大实例数；容器 <4GB 内存建议 2–3 |
| `SERVER_API_PORT` / `SERVER_CONSOLE_PORT` | 取自配置 | 端口覆盖（改映射时同时改 `ports`） |

## 5. Redis 持久化（生产必读）

用户账号、站点配置、告警记录等核心数据存于 Redis。compose 已启用 `--appendonly yes --appendfsync everysec` 并挂载命名卷。验证：

```bash
docker exec prerender-shield-redis redis-cli CONFIG GET appendonly
# 应返回 appendonly=yes
```

## 6. 常用运维命令

```bash
# 查看状态与健康检查
docker compose ps
docker inspect --format '{{.State.Health.Status}}' prerender-shield-app

# 日志
docker compose logs -f app

# 重启 / 停止（数据卷保留）
docker compose restart app
docker compose down

# 升级版本：拉取新代码后重建（redis-data 卷保留，账号与站点配置不丢）
git pull
docker compose up -d --build

# 完全清空（含数据，谨慎）
docker compose down -v
```

## 7. 常见问题

<details>
<summary><b>Chromium 崩溃或渲染失败</b></summary>

- 保持 `shm_size: 256m`（或宿主机执行 `sudo sysctl -w kernel.shmmax=...` 放大共享内存）
- 调低 `PRERENDER_MAX_INSTANCES` / `PRERENDER_WORKER_COUNT`
- 查看容器日志中 `CHROME_PATH` 相关报错：`docker compose logs app | grep -i chrome`
</details>

<details>
<summary><b>端口被占用</b></summary>

修改 compose 中 `ports` 左侧宿主端口（如 `"15997:9597"`），右侧容器端口保持 9597/9598 不变。
</details>

<details>
<summary><b>arm64 机器（Apple Silicon / 国产 ARM 服务器）</b></summary>

镜像全阶段均为多架构官方镜像，`docker compose build` 在 arm64 上可直接构建。
</details>

<details>
<summary><b>反向代理与 HTTPS</b></summary>

建议前置 Nginx/Caddy 做 SSL 终止；仓库 `deploy/nginx/prerender-shield.conf` 提供参考配置。
</details>
