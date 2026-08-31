# 安装指南（INSTALL）

本文给出 PrerenderShield 的**四种安装方式**与首跑、升级、卸载全流程。原理与运维细节见 [docs/OPERATIONS_MANUAL.md](docs/OPERATIONS_MANUAL.md)；Docker 细节见 [docs/DOCKER.md](docs/DOCKER.md)。

| 方式 | 适合人群 | 特点 |
|------|---------|------|
| ① 一键脚本 | 大多数用户 | 自动检测环境、装依赖、注册 systemd |
| ② 官方二进制 | 生产环境、无 Go/Node 环境 | 下载即用，四平台（linux/darwin × amd64/arm64） |
| ③ 源码构建 | 开发者、二次定制 | `build.sh` 一键出包 |
| ④ Docker Compose | 容器化环境 | 最干净的隔离与升级体验 |

---

## 0. 系统要求

| 项目 | 最低 | 推荐 |
|------|------|------|
| 操作系统 | Linux（Ubuntu 20.04+ / CentOS 8+ / Alpine）/ macOS 12+ | Ubuntu 22.04 LTS |
| CPU / 内存 | 2 核 / 4 GB | 4 核 / 8 GB |
| 磁盘 | 10 GB | 20 GB（SSD） |
| 架构 | amd64 / arm64 | — |
| 依赖 | Redis 7.x（缓存与配置存储） | 持久化已开启的 Redis |
| 可选 | Chromium/Chrome（预渲染引擎；缺失仅影响渲染预热） | — |

---

## 1. 方式一：一键脚本（推荐）

```bash
curl -fsSL https://prerender.websitetool.cn/install.sh | bash
```

脚本实际行为（[`install.sh`](install.sh) v3.0.0）：

1. **检测系统**：识别 OS 与架构（amd64/arm64），安装目录固定为 `~/prerender-shield`
2. **自动选择安装方式**：有 Docker → Docker 部署；有 Go + Node → 源码构建；否则 → 预编译二进制
3. **安装依赖**：检测并安装 Chromium 与 Redis（Chromium 未检测到时提示设置 `CHROME_PATH`，程序运行时会读取该变量定位浏览器）
4. **生成配置**：写入 `~/prerender-shield/config.yml`（API `:9598` / 控制台 `:9597`）
5. **启动服务**：Linux 下注册并启动 `systemd` 服务 `prerender-shield`（配置复制到 `/etc/prerender-shield/config.yml`）；macOS 下 `nohup` 后台运行并记录 PID
6. **健康检查**：通过 `GET http://localhost:9598/api/v1/health` 确认服务就绪（Docker 模式轮询重试最多 30 秒）

<details>
<summary>离线/内网服务器：分步执行</summary>

```bash
# 下载脚本后本机执行（效果与 curl | bash 等价）
curl -fsSL https://prerender.websitetool.cn/install.sh -o install.sh
chmod +x install.sh
./install.sh
```
</details>

## 2. 方式二：官方二进制

从 [GitHub Releases](https://github.com/xiaofang142/PrerenderShield/releases/latest)（或 [Gitee Releases](https://gitee.com/xhpmayun/prerender-shield/releases)）下载对应平台的 `prerender-shield_<os>_<arch>.tar.gz`。

```bash
# 1. 下载并校验（包内根目录为 api）
OS=linux ARCH=amd64   # darwin/amd64、darwin/arm64、linux/arm64 同理
curl -fsSLO "https://github.com/xiaofang142/PrerenderShield/releases/latest/download/prerender-shield_${OS}_${ARCH}.tar.gz"
sha256sum -c sha256sums.txt --ignore-missing   # 与 Release 附带的校验清单比对

# 2. 解压到安装目录（tar 包内含 api、web/、configs/、docs/）
mkdir -p ~/prerender-shield && cd ~/prerender-shield
tar xzf ~/prerender-shield_${OS}_${ARCH}.tar.gz && chmod +x api

# 3. 准备依赖与配置
mkdir -p data static certs
# 安装并启动 Redis（Debian/Ubuntu 示例）：
sudo apt-get install -y redis-server && sudo systemctl enable --now redis-server
cp configs/config.example.yml config.yml   # 按需修改，键位见 docs/CONFIG_REFERENCE.md

# 4. 注册 systemd（见第 5 节单元模板）
```

> 建议同时设置环境变量 `JWT_SECRET`（JWT 签名密钥）与 `PRERENDER_MASTER_KEY`（配置加密主密钥），见 [docs/ENV_VARS.md](docs/ENV_VARS.md)。

## 3. 方式三：源码构建

```bash
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield
chmod +x build.sh
./build.sh          # 构建前端 + 当前平台后端，产物：bin/api + bin/web
```

然后按方式二第 3、4 步部署（将 `bin/api`、`bin/web` 拷入安装目录，`config.yml` 中 `admin_static_dir` 指向 `web` 目录）。

开发模式（前后端热更）：

```bash
cd cmd/api && go run main.go      # 后端 :9598 / :9597
cd web && npm install && npm run dev   # 前端 Vite :5173
```

## 4. 方式四：Docker Compose

```bash
git clone https://github.com/xiaofang142/PrerenderShield.git
cd PrerenderShield
docker compose up -d --build
```

包含 `app` + `redis` 两个服务，卷挂载 `./data`、`./configs`、`./static` 与 Redis AOF 数据卷，端口映射 9597/9598，均带 healthcheck。详见 [docs/DOCKER.md](docs/DOCKER.md)。

---

## 5. 首跑向导

### 5.1 防火墙 / 安全组端口

| 端口 | 服务 | 是否对外 |
|------|------|---------|
| **9597** | 管理控制台（Web） | ✅ 对外开放 |
| **9598** | REST API | ✅ 对外开放（含 `/api/v1/health`） |
| 6379 | Redis | ❌ 仅内网/本机，勿暴露公网 |
| 9090 | Prometheus 指标（`monitoring.enabled` 时） | ❌ 仅内网 |

云服务器请同步在安全组放行 9597/9598（TCP）。

```bash
# 本机防火墙示例
sudo ufw allow 9597/tcp && sudo ufw allow 9598/tcp        # Ubuntu
sudo firewall-cmd --permanent --add-port={9597,9598}/tcp && sudo firewall-cmd --reload  # CentOS
```

### 5.2 首次登录创建管理员

浏览器打开 `http://服务器IP:9597`：

1. 系统无预置默认账号——在**登录页直接提交**你想用的账号密码，即完成管理员创建
2. 登录后即可在控制台添加站点、配置 WAF 与预渲染规则
3. 若配置了 `PRERENDER_MASTER_KEY`，站点/API Token 等敏感字段将以 AES-256-GCM 加密存储

### 5.3 验证安装

```bash
curl http://localhost:9598/api/v1/health    # {"code":200,"data":{"status":"running",...}}
curl http://localhost:9598/api/v1/version   # {"code":200,"data":{"version":"3.0.0",...}}
```

---

## 6. systemd 单元示例

方式一已自动注册；方式二/三可手工创建 `/etc/systemd/system/prerender-shield.service`：

```ini
[Unit]
Description=Prerender Shield (Prerender + WAF)
After=network.target redis-server.service
Wants=redis-server.service

[Service]
Type=simple
# 按实际安装目录修改 WorkingDirectory / ExecStart
WorkingDirectory=/home/youruser/prerender-shield
ExecStart=/home/youruser/prerender-shield/api --config /etc/prerender-shield/config.yml
Restart=always
RestartSec=5
LimitNOFILE=65536

# 敏感环境变量建议放 /etc/default/prerender-shield（EnvironmentFile），而非明文写进单元
Environment=JWT_SECRET=change-me-to-a-long-random-string
Environment=PRERENDER_MASTER_KEY=change-me-too
# EnvironmentFile=/etc/default/prerender-shield

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now prerender-shield
sudo systemctl status prerender-shield
journalctl -u prerender-shield -f          # 跟踪日志
```

---

## 7. 升级流程

原则：**备份 → 替换 → 校验**，任一步失败可回滚。

```bash
# 1. 备份（数据目录 + 配置 + Redis 快照）
WORKDIR=~/prerender-shield
sudo systemctl stop prerender-shield
tar czf backup-$(date +%F).tar.gz "$WORKDIR/data" "$WORKDIR/config.yml" /etc/prerender-shield
redis-cli BGSAVE && cp /var/lib/redis/dump.rdb ./backup-$(date +%F).rdb   # 路径按实际修改

# 2. 替换二进制与前端（以新版本 tar 包为例）
tar xzf prerender-shield_linux_amd64.tar.gz -C "$WORKDIR" --overwrite
chmod +x "$WORKDIR/api"

# 3. 启动并校验
sudo systemctl start prerender-shield
curl -s http://localhost:9598/api/v1/health   # 期望 ok
curl -s http://localhost:9598/api/v1/version  # 期望新版本号
# 登录控制台确认站点配置与数据完整

# 4. 回滚（如需）：还原备份并重启
# tar xzf backup-YYYY-MM-DD.tar.gz -C / && sudo systemctl start prerender-shield
```

Docker 升级：`git pull && docker compose up -d --build`（`redis-data` 卷保留，账号与站点配置不丢），见 [docs/DOCKER.md](docs/DOCKER.md#6-常用运维命令)。

---

## 8. 卸载

```bash
# systemd 方式
sudo systemctl disable --now prerender-shield
sudo rm -f /etc/systemd/system/prerender-shield.service /etc/prerender-shield/config.yml
sudo rmdir /etc/prerender-shield 2>/dev/null || true
sudo systemctl daemon-reload

# Docker 方式
docker compose down            # 加 -v 连同 Redis 数据卷一起删除

# 安装目录与数据（如不再保留站点数据/账号）
rm -rf ~/prerender-shield
```

> 卸载前如需迁移，先完成第 7 节的备份步骤。

---

## 相关文档

- [docs/DOCKER.md](docs/DOCKER.md)：Docker 部署细节
- [docs/CONFIG_REFERENCE.md](docs/CONFIG_REFERENCE.md)：配置键位完整参考
- [docs/ENV_VARS.md](docs/ENV_VARS.md)：环境变量
- [docs/QUICK_START_GUIDE.md](docs/QUICK_START_GUIDE.md)：快速上手
- [docs/TROUBLESHOOTING_GUIDE.md](docs/TROUBLESHOOTING_GUIDE.md)：故障排查
