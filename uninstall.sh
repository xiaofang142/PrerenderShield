#!/bin/bash

# ============================================================================
# PrerenderShield 卸载脚本
# ============================================================================
#
# 功能：完全卸载PrerenderShield，包括：
# 1. 停止并禁用服务
# 2. 删除安装文件
# 3. 删除配置和数据（可选）
# 4. 删除系统服务配置
# ============================================================================

set -euo pipefail

# ============================================================================
# 全局变量（与install.sh保持一致）
# ============================================================================

APP_NAME="prerender-shield"
APP_VERSION="1.0.1"
INSTALL_DIR="/opt/${APP_NAME}"
CONFIG_DIR="/etc/${APP_NAME}"
DATA_DIR="/var/lib/${APP_NAME}"
LOG_DIR="/var/log/${APP_NAME}"
SYSTEMD_SERVICE="${APP_NAME}.service"
LAUNCHD_SERVICE="com.prerendershield.app.plist"

# 彩色输出定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# 工具函数
# ============================================================================

print_header() {
    echo -e "${BLUE}"
    echo "===================================================================="
    echo "PrerenderShield 卸载程序 v${APP_VERSION}"
    echo "===================================================================="
    echo -e "${NC}"
}

print_success() {
    echo -e "${GREEN}[✓] $1${NC}"
}

print_info() {
    echo -e "${BLUE}[i] $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}[!] $1${NC}"
}

print_error() {
    echo -e "${RED}[✗] $1${NC}" >&2
}

print_step() {
    echo -e "${BLUE}"
    echo "--------------------------------------------------------------------"
    echo "步骤 $1: $2"
    echo "--------------------------------------------------------------------"
    echo -e "${NC}"
}

check_root() {
    if [[ $EUID -eq 0 ]]; then
        print_warning "正在以root用户运行，继续卸载..."
        return 0
    else
        print_error "请使用sudo或以root用户运行此脚本"
        exit 1
    fi
}

# ============================================================================
# 操作系统检测
# ============================================================================

detect_os() {
    OS_TYPE=$(uname -s)
    case "$OS_TYPE" in
        Linux)
            OS="linux"
            ;;
        Darwin)
            OS="darwin"
            ;;
        *)
            print_error "不支持的操作系统: $OS_TYPE"
            exit 1
            ;;
    esac
    
    print_info "操作系统: $OS_TYPE ($OS)"
}

# ============================================================================
# 确认卸载
# ============================================================================

confirm_uninstall() {
    echo -e "${YELLOW}"
    echo "警告：此操作将卸载PrerenderShield！"
    echo ""
    echo "将执行以下操作："
    echo "1. 停止并禁用PrerenderShield服务"
    echo "2. 删除以下目录："
    echo "   - $INSTALL_DIR"
    echo "   - $CONFIG_DIR"
    echo "3. 可选删除数据目录：$DATA_DIR"
    echo "4. 可选删除日志目录：$LOG_DIR"
    echo ""
    echo -e "${NC}"
    
    read -p "是否继续卸载？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "取消卸载"
        exit 0
    fi
    
    read -p "是否删除数据目录？这将删除所有站点数据。(y/N): " -n 1 -r
    echo
    DELETE_DATA=$([[ $REPLY =~ ^[Yy]$ ]] && echo "yes" || echo "no")
    
    read -p "是否删除日志目录？(y/N): " -n 1 -r
    echo
    DELETE_LOGS=$([[ $REPLY =~ ^[Yy]$ ]] && echo "yes" || echo "no")
}

# ============================================================================
# 停止服务
# ============================================================================

stop_services() {
    print_step "1" "停止服务"
    
    case "$OS" in
        linux)
            print_info "停止systemd服务..."
            sudo systemctl stop "$SYSTEMD_SERVICE" 2>/dev/null || true
            sudo systemctl disable "$SYSTEMD_SERVICE" 2>/dev/null || true
            
            # 检查服务是否仍在运行
            if systemctl is-active --quiet "$SYSTEMD_SERVICE" 2>/dev/null; then
                print_warning "服务仍在运行，尝试强制停止..."
                sudo systemctl kill "$SYSTEMD_SERVICE" 2>/dev/null || true
                sleep 2
            fi
            
            print_success "服务已停止"
            ;;
        darwin)
            print_info "停止launchd服务..."
            sudo launchctl stop "com.prerendershield.app" 2>/dev/null || true
            sudo launchctl unload "/Library/LaunchDaemons/$LAUNCHD_SERVICE" 2>/dev/null || true
            
            print_success "服务已停止"
            ;;
    esac
}

# ============================================================================
# 删除系统服务配置
# ============================================================================

remove_service_config() {
    print_step "2" "删除系统服务配置"
    
    case "$OS" in
        linux)
            print_info "删除systemd服务文件..."
            sudo rm -f "/etc/systemd/system/$SYSTEMD_SERVICE" 2>/dev/null || true
            
            print_info "重新加载systemd配置..."
            sudo systemctl daemon-reload 2>/dev/null || true
            
            print_success "systemd服务配置已删除"
            ;;
        darwin)
            print_info "删除launchd服务文件..."
            sudo rm -f "/Library/LaunchDaemons/$LAUNCHD_SERVICE" 2>/dev/null || true
            
            print_success "launchd服务配置已删除"
            ;;
    esac
}

# ============================================================================
# 删除安装文件
# ============================================================================

remove_installation_files() {
    print_step "3" "删除安装文件"
    
    print_info "删除安装目录: $INSTALL_DIR"
    sudo rm -rf "$INSTALL_DIR" 2>/dev/null || true
    
    print_info "删除配置目录: $CONFIG_DIR"
    sudo rm -rf "$CONFIG_DIR" 2>/dev/null || true
    
    print_success "安装文件已删除"
}

# ============================================================================
# 删除数据文件（可选）
# ============================================================================

remove_data_files() {
    print_step "4" "删除数据文件"
    
    if [[ "$DELETE_DATA" == "yes" ]]; then
        print_info "删除数据目录: $DATA_DIR"
        sudo rm -rf "$DATA_DIR" 2>/dev/null || true
        print_success "数据目录已删除"
    else
        print_info "保留数据目录: $DATA_DIR"
        print_info "数据目录包含站点配置和缓存数据，如需完全清理请手动删除"
    fi
    
    if [[ "$DELETE_LOGS" == "yes" ]]; then
        print_info "删除日志目录: $LOG_DIR"
        sudo rm -rf "$LOG_DIR" 2>/dev/null || true
        print_success "日志目录已删除"
    else
        print_info "保留日志目录: $LOG_DIR"
    fi
}

# ============================================================================
# 清理进程和端口
# ============================================================================

cleanup_processes() {
    print_step "5" "清理残留进程"
    
    print_info "检查并终止相关进程..."
    
    # 查找并终止prerender-shield进程
    local pids=$(pgrep -f "prerender-shield" 2>/dev/null || true)
    if [[ -n "$pids" ]]; then
        print_info "发现残留进程: $pids"
        sudo kill -9 $pids 2>/dev/null || true
        print_success "残留进程已终止"
    else
        print_info "未发现残留进程"
    fi
    
    # 检查常用端口是否被占用
    print_info "检查端口占用情况..."
    local ports="9597 9598 8081"
    for port in $ports; do
        if lsof -ti:"$port" >/dev/null 2>&1; then
            print_warning "端口 $port 仍被占用，可能需要手动清理"
        fi
    done
}

# ============================================================================
# 完成卸载
# ============================================================================

print_summary() {
    print_step "6" "卸载完成"
    
    echo -e "${GREEN}"
    echo "===================================================================="
    echo "PrerenderShield 卸载完成！"
    echo "===================================================================="
    echo ""
    echo "已执行的操作："
    echo "1. ✓ 停止并禁用服务"
    echo "2. ✓ 删除系统服务配置"
    echo "3. ✓ 删除安装文件"
    echo "4. ✓ 删除配置目录"
    
    if [[ "$DELETE_DATA" == "yes" ]]; then
        echo "5. ✓ 删除数据目录"
    else
        echo "5. ✗ 保留数据目录: $DATA_DIR"
    fi
    
    if [[ "$DELETE_LOGS" == "yes" ]]; then
        echo "6. ✓ 删除日志目录"
    else
        echo "6. ✗ 保留日志目录: $LOG_DIR"
    fi
    
    echo ""
    echo "注意："
    echo "1. 系统依赖（Go、Redis、Node.js、浏览器）未被卸载"
    echo "2. 如需重新安装，请运行: sudo ./install.sh"
    echo "3. 如果保留了数据目录，重新安装后可能需要手动恢复配置"
    echo ""
    echo "感谢使用PrerenderShield！"
    echo "===================================================================="
    echo -e "${NC}"
}

# ============================================================================
# 主函数
# ============================================================================

main() {
    print_header
    check_root
    detect_os
    confirm_uninstall
    stop_services
    remove_service_config
    remove_installation_files
    remove_data_files
    cleanup_processes
    print_summary
}

# 检查是否直接运行
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi