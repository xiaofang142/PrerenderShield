#!/bin/bash

# PrerenderShield 部署脚本

set -e

# 配置变量
PROJECT_NAME="prerendershield"
DOCKER_COMPOSE_FILE="../docker/docker-compose.yml"
CONFIG_DIR="../configs"

# 显示帮助信息
show_help() {
    echo "PrerenderShield 部署脚本"
    echo ""
    echo "使用方法: $0 [选项] 命令"
    echo ""
    echo "命令:"
    echo "  up      启动服务"
    echo "  down    停止服务"
    echo "  restart 重启服务"
    echo "  status  查看服务状态"
    echo "  logs    查看服务日志"
    echo "  help    显示帮助信息"
    echo ""
    echo "选项:"
    echo "  -f <文件>  指定 Docker Compose 文件路径"
    echo "  -d <目录>  指定配置文件目录"
    echo "  -p <名称>  指定项目名称"
}

# 解析命令行参数
while getopts "f:d:p:h" opt; do
    case $opt in
        f) DOCKER_COMPOSE_FILE="$OPTARG" ;;
        d) CONFIG_DIR="$OPTARG" ;;
        p) PROJECT_NAME="$OPTARG" ;;
        h) show_help ; exit 0 ;;
        *) show_help ; exit 1 ;;
    esac
done

# 移除已处理的参数，保留命令
shift $((OPTIND-1))

# 检查命令
if [ $# -eq 0 ]; then
    echo "错误: 必须指定命令"
    show_help
    exit 1
fi

COMMAND=$1

# 执行命令
case $COMMAND in
    up)
        echo "正在启动 PrerenderShield 服务..."
        docker-compose -f "$DOCKER_COMPOSE_FILE" -p "$PROJECT_NAME" up -d
        echo "PrerenderShield 服务已启动"
        ;;
    down)
        echo "正在停止 PrerenderShield 服务..."
        docker-compose -f "$DOCKER_COMPOSE_FILE" -p "$PROJECT_NAME" down
        echo "PrerenderShield 服务已停止"
        ;;
    restart)
        echo "正在重启 PrerenderShield 服务..."
        docker-compose -f "$DOCKER_COMPOSE_FILE" -p "$PROJECT_NAME" restart
        echo "PrerenderShield 服务已重启"
        ;;
    status)
        echo "查看 PrerenderShield 服务状态..."
        docker-compose -f "$DOCKER_COMPOSE_FILE" -p "$PROJECT_NAME" ps
        ;;
    logs)
        echo "查看 PrerenderShield 服务日志..."
        docker-compose -f "$DOCKER_COMPOSE_FILE" -p "$PROJECT_NAME" logs -f
        ;;
    help)
        show_help
        ;;
    *)
        echo "错误: 未知命令 '$COMMAND'"
        show_help
        exit 1
        ;;
esac
