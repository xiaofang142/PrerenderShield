#!/bin/bash

# PrerenderShield 启动脚本

APP_NAME="prerender-shield"

# 检查当前目录是否已经是平台目录（包含api二进制文件）
if [ -f "./api" ]; then
    # 直接使用当前目录作为平台目录
    PLATFORM_DIR=$(pwd)
else
    # 获取脚本所在的根目录
    SCRIPT_DIR=$(dirname "$(realpath "$0")")
    
    # 动态获取当前平台目录
    platform_dir() {
        local os=$(uname -s | tr '[:upper:]' '[:lower:]')
        local arch=$(uname -m)
        
        # 转换架构名称
        if [[ $arch == "x86_64" ]]; then
            arch="amd64"
        elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
            arch="arm64"
        fi
        
        echo "${SCRIPT_DIR}/bin/${os}-${arch}"
    }
    
    # 获取当前平台目录
    PLATFORM_DIR=$(platform_dir)
fi

# 优先使用平台目录下的config/config.yml，然后使用configs/config.yml
if [ -f "${PLATFORM_DIR}/config/config.yml" ]; then
    CONFIG_FILE="${PLATFORM_DIR}/config/config.yml"
elif [ -f "configs/config.yml" ]; then
    CONFIG_FILE="configs/config.yml"
else
    CONFIG_FILE="${PLATFORM_DIR}/config/config.yml"
fi

# 使用平台目录下的数据目录
DATA_DIR="${PLATFORM_DIR}/data"
PID_FILE="${DATA_DIR}/${APP_NAME}.pid"
LOG_FILE="${DATA_DIR}/${APP_NAME}.log"

# 彩色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印彩色信息
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 根据当前平台选择合适的二进制文件
get_platform_binary() {
    # 构建二进制文件路径
    local binary_path="${PLATFORM_DIR}/api"
    
    # 如果是Windows系统，添加.exe后缀
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    if [[ $os == "windows" ]]; then
        binary_path="${binary_path}.exe"
    fi
    
    echo "$binary_path"
}

# 获取当前平台的二进制文件路径
BINARY_PATH=$(get_platform_binary)

# 如果当前平台的二进制文件不存在，使用当前目录下的api
if [ ! -f "$BINARY_PATH" ]; then
    if [ -f "./api" ]; then
        BINARY_PATH="./api"
        info "使用当前目录下的二进制文件: $BINARY_PATH"
    else
        error "未找到二进制文件: $BINARY_PATH 或 ./api"
        error "请先运行 ./build.sh 编译应用程序"
        exit 1
    fi
else
    info "使用平台特定二进制文件: $BINARY_PATH"
fi

# 获取本机IP地址，用于访问信息
get_local_ip() {
    local ip="127.0.0.1"
    
    if [[ "$(hostname)" != "localhost" && ! "$(hostname)" =~ "local" && "$(uname -s)" != "Darwin" ]]; then
        # 服务器环境，尝试获取公网IP
        ip=$(curl -s ifconfig.me || curl -s icanhazip.com || echo "127.0.0.1")
    fi
    
    echo "$ip"
}

usage() {
    echo "========================================"
    echo -e "${GREEN}PrerenderShield 启动脚本${NC}"
    echo "========================================"
    echo "用法: $0 {start|restart|stop}"
    echo ""
    echo "选项:"
    echo "  start      启动应用程序"
    echo "  restart    重启应用程序"
    echo "  stop       停止应用程序"
    echo ""
    exit 1
}

is_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE" 2>/dev/null)
        if [ -n "$pid" ] && ps -p "$pid" > /dev/null 2>&1; then
            return 0 # 运行中
        else
            warning "PID文件存在但进程不存在，删除失效的PID文件"
            rm -f "$PID_FILE" > /dev/null 2>&1 # PID文件存在但进程不存在，删除PID文件
        fi
    fi
    return 1 # 未运行
}

status() {
    if is_running; then
        local pid=$(cat "$PID_FILE")
        info "$APP_NAME 正在运行，PID: $pid"
        info "日志文件: $LOG_FILE"
        info "配置文件: $CONFIG_FILE"
        info "二进制文件: $BINARY_PATH"
        
        # 输出服务访问信息
        local local_ip=$(get_local_ip)
        info ""
        info "======================================="
        info "服务访问信息"
        info "======================================="
        info "管理控制台: http://$local_ip:9597"
        info "API服务: http://$local_ip:9598"
        info "======================================="
        return 0
    else
        warning "$APP_NAME 没有在运行中"
        return 1
    fi
}

# 检测服务是否真正启动
detect_service_started() {
    local ip=$1
    local port=$2
    local timeout=30
    local interval=2
    local count=0
    
    info "正在检测服务 http://$ip:$port 是否启动..."
    
    while [[ $count -lt $timeout ]]; do
        if curl -s --connect-timeout 1 http://$ip:$port > /dev/null 2>&1; then
            return 0 # 服务已启动
        fi
        
        sleep $interval
        count=$((count + interval))
    done
    
    return 1 # 服务未在指定时间内启动
}

# 执行服务健康检查
service_health_check() {
    local ip=$1
    
    echo ""
    info "执行服务健康检查..."
    
    # 检查API服务
    if curl -s http://$ip:9598/api/v1/health > /dev/null 2>&1; then
        info "✓ API服务健康检查通过"
    else
        warning "✗ API服务健康检查失败，可能服务尚未完全启动"
        warning "  请稍后使用以下命令检查服务状态: curl http://$ip:9598/api/v1/health"
    fi
    
    # 检查管理控制台
    if curl -s http://$ip:9597 > /dev/null 2>&1; then
        info "✓ 管理控制台健康检查通过"
    else
        warning "✗ 管理控制台健康检查失败，可能服务尚未完全启动"
        warning "  请稍后使用以下命令检查服务状态: curl http://$ip:9597"
    fi
}

start() {
    if is_running; then
        info "$APP_NAME 已经在运行中"
        return 0
    fi

    # 检查二进制文件是否存在和可执行
    if [ ! -f "$BINARY_PATH" ]; then
        error "未找到二进制文件: $BINARY_PATH"
        error "请先运行 ./build.sh 编译应用程序"
        exit 1
    fi
    
    if [ ! -x "$BINARY_PATH" ]; then
        warning "二进制文件不可执行，添加执行权限"
        chmod +x "$BINARY_PATH"
        if [ ! -x "$BINARY_PATH" ]; then
            error "无法添加执行权限: $BINARY_PATH"
            exit 1
        fi
    fi

    # 检查配置文件是否存在
    if [ ! -f "$CONFIG_FILE" ]; then
        error "配置文件不存在: $CONFIG_FILE"
        error "请先运行 ./install.sh 安装应用程序或检查配置文件路径"
        exit 1
    fi


    # 直接启动应用程序
    info "启动$APP_NAME..."
    cd "$SCRIPT_DIR" && nohup "$BINARY_PATH" --config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    info "$APP_NAME 启动命令已执行，PID: $(cat "$PID_FILE")"
    info "日志文件: $LOG_FILE"
    
    # 获取本机IP地址
    local local_ip=$(get_local_ip)
    
    # 检测API服务是否真正启动
    local api_started=false
    local admin_started=false
    
    if detect_service_started "$local_ip" "9598"; then
        info "API服务已成功启动"
        api_started=true
    else
        warning "API服务可能未成功启动，请检查日志: tail -f $LOG_FILE"
    fi
    
    # 检测管理控制台是否真正启动
    if detect_service_started "$local_ip" "9597"; then
        info "管理控制台已成功启动"
        admin_started=true
    else
        warning "管理控制台可能未成功启动，请检查日志: tail -f $LOG_FILE"
    fi
    
    # 执行服务健康检查
    service_health_check "$local_ip"
    
    # 只有当服务真正启动成功后才输出访问提示
    if $api_started && $admin_started; then
        echo ""
        info "========================================"
        info "应用程序服务启动信息"
        info "======================================="
        info "管理控制台: http://$local_ip:9597"
        info "API服务: http://$local_ip:9598"
        info "健康检查接口: http://$local_ip:9598/api/v1/health"
        info "======================================="
        
        echo ""
        info "$APP_NAME 启动完成"
        info "访问管理控制台: http://$local_ip:9597"
        info "使用默认账号登录: admin / 123456"
        info "查看日志: tail -f $LOG_FILE"
    else
        echo ""
        warning "$APP_NAME 启动可能存在问题，请检查日志: tail -f $LOG_FILE"
        warning "建议使用 ./start.sh status 检查服务状态"
    fi
}

stop() {
    if ! is_running; then
        info "$APP_NAME 没有在运行中"
        return 0
    fi

    local pid=$(cat "$PID_FILE")
    info "停止$APP_NAME，PID: $pid..."
    
    # 尝试优雅停止
    kill "$pid"

    # 等待进程退出
    local count=0
    local max_wait=10
    while is_running && [ $count -lt $max_wait ]; do
        sleep 1
        count=$((count + 1))
        info "等待进程退出... ($count/$max_wait 秒)"
    done

    if is_running; then
        warning "进程未正常退出，尝试强制终止..."
        kill -9 "$pid"
        sleep 1
    fi

    if ! is_running; then
        info "$APP_NAME 已成功停止"
        if [ -f "$PID_FILE" ]; then
            rm -f "$PID_FILE"
        fi
    else
        error "无法停止$APP_NAME，请手动检查并终止进程"
        info "可以使用以下命令手动终止进程: kill -9 $pid"
        exit 1
    fi
}

restart() {
    info "重启$APP_NAME..."
    stop
    info "等待2秒后重新启动..."
    sleep 2
    start
}

# 主程序
if [ $# -eq 0 ]; then
    usage
fi

case "$1" in
    start)
        start
        ;;
    restart)
        restart
        ;;
    stop)
        stop
        ;;
    status)
        status
        ;;
    *)
        usage
        ;;
esac
