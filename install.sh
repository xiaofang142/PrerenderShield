#!/usr/bin/env bash
set -euo pipefail

APP="Prerender Shield"
VERSION="3.0.0"
GITHUB="https://github.com/xiaofang142/PrerenderShield"
GITEE="https://gitee.com/xhpmayun/prerender-shield"
WORKDIR="${HOME}/prerender-shield"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}▶${NC} $1"; }
warn()  { echo -e "${YELLOW}▶${NC} $1"; }
error() { echo -e "${RED}▶${NC} $1" >&2; exit 1; }
title() { echo -e "\n${CYAN}━━━ $1 ━━━${NC}\n"; }

clear
echo ""
echo "  ╔══════════════════════════════════════════════════╗"
echo "  ║         Prerender Shield v${VERSION}              ║"
echo "  ║    企业级安全防护 + 智能渲染预热 + 一键部署     ║"
echo "  ╚══════════════════════════════════════════════════╝"
echo ""

title "1/4  检测系统环境"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; arm64|aarch64) ARCH="arm64" ;; *) error "不支持的架构: $ARCH" ;; esac
info "系统: $OS ($ARCH)"
mkdir -p "$WORKDIR" && cd "$WORKDIR"

title "2/4  选择安装方式"
INSTALL_MODE=""
if command -v docker &>/dev/null && (docker compose version &>/dev/null || docker-compose version &>/dev/null); then
    INSTALL_MODE="docker"; info "检测到 Docker → 使用 Docker 部署"
elif command -v go &>/dev/null && command -v node &>/dev/null; then
    INSTALL_MODE="source"; info "检测到 Go + Node.js → 源码构建"
else
    INSTALL_MODE="binary"; info "使用预编译二进制安装"
fi
sleep 1

title "3/4  安装中..."

# chromium_available 检测系统是否已有可用浏览器（PATH 或 macOS 应用目录）
chromium_available() {
    command -v chromium &>/dev/null || command -v chromium-browser &>/dev/null \
        || command -v google-chrome &>/dev/null || command -v google-chrome-stable &>/dev/null \
        || [ -x "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ] \
        || [ -x "/Applications/Chromium.app/Contents/MacOS/Chromium" ]
}

# ensure_chromium 检测并安装无头浏览器（渲染引擎核心依赖）
ensure_chromium() {
    if chromium_available; then
        info "检测到 Chromium/Chrome，跳过安装"
        return 0
    fi
    warn "未检测到 Chromium，开始安装（渲染引擎依赖）..."
    case "$OS" in
        linux)
            if command -v apt-get >/dev/null; then sudo apt-get install -y -qq chromium chromium-browser 2>/dev/null || true
            elif command -v yum >/dev/null; then sudo yum install -y -q chromium 2>/dev/null || true
            elif command -v apk >/dev/null; then sudo apk add --no-cache chromium 2>/dev/null || true
            elif command -v dnf >/dev/null; then sudo dnf install -y -q chromium 2>/dev/null || true
            fi ;;
        darwin)
            # cask 安装到 /Applications，二进制不进 PATH；启动器已覆盖该路径
            command -v brew >/dev/null && brew install --cask google-chrome 2>/dev/null || true ;;
    esac
    if chromium_available; then
        info "Chromium 安装成功"
    else
        warn "Chromium 自动安装失败 — 渲染预热功能将不可用"
        warn "请手动安装后设置环境变量: export CHROME_PATH=/path/to/chromium"
    fi
}

# install_native 原生部署（二进制/源码构建共用）
install_native() {
    mkdir -p data static certs

    # 安装 Chromium（渲染引擎核心依赖）
    ensure_chromium

    if ! command -v redis-cli &>/dev/null; then
        info "安装 Redis..."
        case "$OS" in
            linux)
                command -v apt-get >/dev/null && sudo apt-get install -y -qq redis-server 2>/dev/null
                command -v yum >/dev/null && sudo yum install -y -q redis 2>/dev/null
                sudo systemctl enable redis-server 2>/dev/null || true
                sudo systemctl start redis-server 2>/dev/null || true ;;
            darwin)
                command -v brew >/dev/null && brew install redis && brew services start redis ;;
        esac
    fi

    cat > config.yml <<CONF
server:
  address: "0.0.0.0"
  api_port: 9598
  console_port: 9597
dirs:
  data_dir: ${WORKDIR}/data
  static_dir: ${WORKDIR}/static
  certs_dir: ${WORKDIR}/certs
  admin_static_dir: ${WORKDIR}/web
cache:
  type: "redis"
  redis_url: "localhost:6379"
CONF

    if [ "$OS" = "linux" ]; then
        sudo mkdir -p /etc/prerender-shield
        sudo cp config.yml /etc/prerender-shield/config.yml
        sudo tee /etc/systemd/system/prerender-shield.service >/dev/null <<SRV
[Unit]
Description=Prerender Shield
After=network.target redis-server.service
[Service]
ExecStart=${WORKDIR}/api --config /etc/prerender-shield/config.yml
WorkingDirectory=${WORKDIR}
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
SRV
        sudo systemctl daemon-reload
        sudo systemctl enable prerender-shield
        sudo systemctl start prerender-shield
    else
        nohup ./api --config config.yml > data/prerender-shield.log 2>&1 &
        echo $! > data/prerender-shield.pid
    fi
}

case "$INSTALL_MODE" in
    docker)
        [ -f "docker-compose.yml" ] || curl -fsSL "${GITEE}/raw/main/docker/docker-compose.yml" -o docker-compose.yml
        [ -f "config.yml" ] || curl -fsSL "${GITEE}/raw/main/configs/config.example.yml" -o config.yml
        sed -i.bak 's|redis_url:.*|redis_url: "redis://redis:6379/0"|' config.yml 2>/dev/null || true
        rm -f config.yml.bak
        docker compose up -d 2>/dev/null || docker-compose up -d
        for i in $(seq 1 30); do
            curl -fs "http://localhost:9598/api/v1/health" >/dev/null 2>&1 && { echo -e "${GREEN}  服务就绪${NC}"; break; } || sleep 1
        done
        ;;

    source)
        # 幂等：若目录已有源码（如重跑安装脚本）则不再克隆，避免自调用递归
        if [ ! -f "go.mod" ]; then
            git clone --depth 1 "$GITEE" src 2>/dev/null || git clone --depth 1 "$GITHUB" src || error "源码克隆失败，请检查网络"
            cd src
        fi
        chmod +x build.sh && ./build.sh || error "源码构建失败"
        BIN_DIR="$(pwd)/bin"
        [ -x "$BIN_DIR/api" ] || error "构建产物缺失: $BIN_DIR/api"
        cd "$WORKDIR"
        rm -rf api web
        cp "$BIN_DIR/api" ./api && chmod +x ./api
        cp -r "$BIN_DIR/web" ./web
        install_native
        ;;

    binary)
        URL="${GITHUB}/releases/latest/download/prerender-shield_${OS}_${ARCH}.tar.gz"
        info "下载 ${OS}_${ARCH}..."
        curl -fsSL "$URL" -o release.tar.gz || curl -fsSL "${GITEE}/releases/latest/download/prerender-shield_${OS}_${ARCH}.tar.gz" -o release.tar.gz
        [ -f release.tar.gz ] || error "下载失败"
        tar xzf release.tar.gz && chmod +x api
        install_native
        ;;
esac

title "4/4  验证安装"
sleep 2
if curl -fs "http://localhost:9598/api/v1/health" >/dev/null 2>&1; then
    echo -e "  ${GREEN}✅  服务运行正常${NC}"
else
    warn "服务启动中，请稍后检查: cat ${WORKDIR}/data/prerender-shield.log"
fi

echo ""
echo "  ╔════════════════════════════════════════════╗"
echo "  ║          🎉  安装完成！                    ║"
echo "  ╠════════════════════════════════════════════╣"
echo "  ║  管理控制台:  http://localhost:9597        ║"
echo "  ║  API 服务:    http://localhost:9598        ║"
echo "  ║  首次访问控制台时自行设置管理员账号密码    ║"
echo "  ║  安装目录:    ${WORKDIR}          ║"
echo "  ║  问题反馈:    ${GITHUB}/issues   ║"
echo "  ╚════════════════════════════════════════════╝"
echo ""
