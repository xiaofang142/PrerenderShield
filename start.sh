#!/bin/bash

# PrerenderShield 启动脚本

APP_NAME="prerender-shield"
APP_BINARY="./prerender-shield"
CONFIG_FILE="configs/config.yml"
PID_FILE="./data/${APP_NAME}.pid"
LOG_FILE="./data/${APP_NAME}.log"

usage() {
    echo "========================================"
    echo "PrerenderShield 启动脚本"
    echo "========================================"
    echo "用法: $0 {start|restart|stop|reinstall}"
    echo ""
    echo "选项:"
    echo "  start      启动应用程序"
    echo "  restart    重启应用程序"
    echo "  stop       停止应用程序"
    echo "  reinstall  重新安装应用程序（清除数据）"
    echo ""
    exit 1
}

check_root() {
    if [ "$EUID" -eq 0 ]; then
        echo "警告: 正在以root用户运行，这可能不是最佳实践"
    fi
}

check_go() {
    if ! command -v go &> /dev/null; then
        echo "错误: 未安装Go，无法构建应用程序"
        exit 1
    fi

    # 检查Go版本
    GO_VERSION=$(go version | grep -o 'go1\.[0-9]*')
    if [[ "$GO_VERSION" != "go1.20" && "$GO_VERSION" != "go1.21" && "$GO_VERSION" != "go1.22" ]]; then
        echo "警告: Go版本可能不兼容，建议使用Go 1.20+，当前版本: $GO_VERSION"
    fi
}

create_dirs() {
    echo "创建必要的目录..."
    mkdir -p data/rules data/certs data/redis data/grafana
}

check_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "配置文件不存在，从模板复制..."
        if [ -f configs/config.example.yml ]; then
            cp configs/config.example.yml "$CONFIG_FILE"
            echo "已从config.example.yml复制到$CONFIG_FILE"
        else
            echo "错误: 配置文件模板configs/config.example.yml不存在"
            exit 1
        fi
    fi
}

build_app() {
    echo "安装Go依赖..."
    go mod tidy

    echo "构建应用程序..."
    go build -o "$APP_BINARY" ./cmd/api

    if [ $? -ne 0 ]; then
        echo "错误: 构建失败"
        exit 1
    fi

    echo "构建成功！"
}

is_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0 # 运行中
        else
            rm -f "$PID_FILE" # PID文件存在但进程不存在，删除PID文件
        fi
    fi
    return 1 # 未运行
}

start() {
    if is_running; then
        echo "$APP_NAME 已经在运行中"
        return 0
    fi

    echo "启动$APP_NAME..."
    echo "========================================"
    echo "应用程序将在 http://0.0.0.0:8080 上运行"
    echo "健康检查接口: http://0.0.0.0:8080/api/v1/health"
    echo "版本信息接口: http://0.0.0.0:8080/api/v1/version"
    echo "========================================"

    # 启动应用程序
    nohup "$APP_BINARY" --config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    echo "$APP_NAME 启动成功，PID: $(cat "$PID_FILE")"
    echo "日志文件: $LOG_FILE"
}

stop() {
    if ! is_running; then
        echo "$APP_NAME 没有在运行中"
        return 0
    fi

    local pid=$(cat "$PID_FILE")
    echo "停止$APP_NAME，PID: $pid..."
    kill "$pid"

    # 等待进程退出
    local count=0
    while is_running && [ $count -lt 10 ]; do
        sleep 1
        count=$((count + 1))
    done

    if is_running; then
        echo "强制终止$APP_NAME..."
        kill -9 "$pid"
        sleep 1
    fi

    if ! is_running; then
        echo "$APP_NAME 已停止"
        rm -f "$PID_FILE"
    else
        echo "错误: 无法停止$APP_NAME"
        exit 1
    fi
}

restart() {
    stop
    sleep 2
    start
}

reinstall() {
    echo "重新安装$APP_NAME..."
    stop
    
    # 清除数据（保留配置文件）
    echo "清除数据..."
    rm -rf ./data/*
    mkdir -p data/rules data/certs data/redis data/grafana
    
    # 重新构建
    build_app
    
    # 启动
    start
}

# 主程序
if [ $# -eq 0 ]; then
    usage
fi

check_root

case "$1" in
    start)
        check_go
        create_dirs
        check_config
        build_app
        start
        ;;
    restart)
        restart
        ;;
    stop)
        stop
        ;;
    reinstall)
        check_go
        reinstall
        ;;
    *)
        usage
        ;;
esac
