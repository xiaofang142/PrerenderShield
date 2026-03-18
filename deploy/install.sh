#!/bin/bash
set -e

echo "======================================"
echo "  Prerender Shield 快速部署脚本"
echo "======================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查是否以 root 运行
if [ "$EUID" -ne 0 ]; then
    log_error "请使用 sudo 运行此脚本"
    exit 1
fi

# 配置变量
INSTALL_DIR="/opt/prerender-shield"
LOG_DIR="/var/log/prerender-shield"
SERVICE_USER="prerender"
CONFIG_FILE="config.production.yaml"

# 检测系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        BINARY_ARCH="amd64"
        ;;
    aarch64)
        BINARY_ARCH="arm64"
        ;;
    *)
        log_error "不支持的架构：$ARCH"
        exit 1
        ;;
esac

log_info "检测到系统架构：$ARCH ($BINARY_ARCH)"

# 1. 创建系统用户
log_info "创建系统用户..."
if ! id "$SERVICE_USER" &>/dev/null; then
    useradd -r -s /bin/false -d "$INSTALL_DIR" "$SERVICE_USER"
    log_info "用户 $SERVICE_USER 创建成功"
else
    log_info "用户 $SERVICE_USER 已存在"
fi

# 2. 创建目录
log_info "创建安装目录..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$INSTALL_DIR/certs"

# 3. 下载最新发行版
log_info "下载最新发行版..."
LATEST_RELEASE=$(curl -s https://api.github.com/repos/prerender-shield/releases/latest | grep "browser_download_url" | grep "$BINARY_ARCH" | head -1 | cut -d '"' -f 4)

if [ -z "$LATEST_RELEASE" ]; then
    log_warn "无法获取最新发行版，使用本地构建版本"
    # 假设有本地构建的二进制文件
    if [ -f "./prerender-shield" ]; then
        cp ./prerender-shield "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/prerender-shield"
    else
        log_error "未找到本地二进制文件"
        exit 1
    fi
else
    curl -L -o "$INSTALL_DIR/prerender-shield" "$LATEST_RELEASE"
    chmod +x "$INSTALL_DIR/prerender-shield"
fi

# 4. 配置文件
log_info "配置应用..."
if [ ! -f "$INSTALL_DIR/$CONFIG_FILE" ]; then
    if [ -f "./config.production.yaml.example" ]; then
        cp ./config.production.yaml.example "$INSTALL_DIR/$CONFIG_FILE"
        log_info "配置文件已创建，请修改：$INSTALL_DIR/$CONFIG_FILE"
    fi
fi

# 5. 复制前端文件
log_info "部署前端文件..."
if [ -d "./web/dist" ]; then
    cp -r ./web/dist "$INSTALL_DIR/static"
fi

# 6. 安装 systemd 服务
log_info "安装 systemd 服务..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/prerender-shield.service" ]; then
    cp "$SCRIPT_DIR/prerender-shield.service" /etc/systemd/system/
    systemctl daemon-reload
    log_info "systemd 服务已安装"
else
    log_warn "未找到 systemd 服务文件"
fi

# 7. 设置权限
log_info "设置权限..."
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
chown -R "$SERVICE_USER:$SERVICE_USER" "$LOG_DIR"
chmod 755 "$INSTALL_DIR/prerender-shield"

# 8. 配置防火墙 (如果安装了 firewalld)
if command -v firewall-cmd &> /dev/null; then
    log_info "配置防火墙..."
    firewall-cmd --permanent --add-port=9598/tcp
    firewall-cmd --permanent --add-port=9090/tcp  # Prometheus 端口
    firewall-cmd --reload
fi

# 9. 启用并启动服务
log_info "启用并启动服务..."
systemctl enable prerender-shield
systemctl start prerender-shield

# 10. 检查服务状态
sleep 2
if systemctl is-active --quiet prerender-shield; then
    log_info "======================================"
    log_info "  部署成功!"
    log_info "======================================"
    log_info "服务状态：运行中"
    log_info "访问地址：http://localhost:9598"
    log_info ""
    log_info "常用命令:"
    log_info "  查看状态：systemctl status prerender-shield"
    log_info "  停止服务：systemctl stop prerender-shield"
    log_info "  重启服务：systemctl restart prerender-shield"
    log_info "  查看日志：journalctl -u prerender-shield -f"
    log_info ""
    log_warn "请记得修改配置文件：$INSTALL_DIR/$CONFIG_FILE"
else
    log_error "服务启动失败，请检查日志"
    journalctl -u prerender-shield -n 50 --no-pager
    exit 1
fi
