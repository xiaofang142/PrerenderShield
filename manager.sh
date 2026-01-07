#!/bin/bash

# Prerender-Shield 管理器脚本
# 参考长亭雷池WAF管理器设计
# 提供安装/升级/修复/卸载等功能

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 常量定义
APP_NAME="Prerender-Shield"
# 默认安装目录，非root用户使用当前目录
if [ "$EUID" -eq 0 ]; then
    APP_DIR="/opt/prerender-shield"
else
    APP_DIR="./prerender-shield"
fi
GITHUB_REPO="https://github.com/your-org/prerender-shield"
DOCKER_COMPOSE_URL="${GITHUB_REPO}/raw/main/docker-compose.yml"
MANAGER_VERSION="v1.0.0"

# 显示帮助信息
show_help() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}${APP_NAME} 管理器${NC}"
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${CYAN}用法:${NC} $0 [选项] [命令]"
    echo -e ""
    echo -e "${CYAN}命令:${NC}"
    echo -e "  install    安装 ${APP_NAME}"
    echo -e "  upgrade    升级 ${APP_NAME}"
    echo -e "  repair     修复 ${APP_NAME} 安装"
    echo -e "  uninstall  卸载 ${APP_NAME}"
    echo -e "  status     查看 ${APP_NAME} 状态"
    echo -e "  logs       查看 ${APP_NAME} 日志"
    echo -e "  help       显示帮助信息"
    echo -e ""
    echo -e "${CYAN}选项:${NC}"
    echo -e "  --dir <目录>    指定安装目录（默认: ${APP_DIR}）"
    echo -e "  --lts           安装 LTS 版本"
    echo -e "  --verbose       显示详细日志"
    echo -e "  --force         强制操作（用于卸载或修复）"
    echo -e ""
    echo -e "${CYAN}示例:${NC}"
    echo -e "  $0 install                  # 安装最新版本"
    echo -e "  $0 upgrade                  # 升级到最新版本"
    echo -e "  $0 status                   # 查看服务状态"
    echo -e "  $0 logs -f                  # 实时查看日志"
    echo -e "  $0 uninstall --force        # 强制卸载"
    echo -e "${BLUE}=======================================${NC}"
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            install|upgrade|repair|uninstall|status|logs|help)
                COMMAND="$1"
                shift
                ;;
            --dir)
                APP_DIR="$2"
                shift 2
                ;;
            --lts)
                LTS_MODE=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                echo -e "${RED}错误: 未知参数 '$1'${NC}"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 默认命令
    if [ -z "${COMMAND}" ]; then
        show_help
        exit 1
    fi
}

# 检查 Docker 环境
check_docker() {
    echo -e "${BLUE}[检查] Docker 环境${NC}"
    
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}错误: Docker 未安装${NC}"
        echo -e "${YELLOW}请先安装 Docker: https://docs.docker.com/get-docker/${NC}"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}错误: Docker Compose 未安装${NC}"
        echo -e "${YELLOW}请先安装 Docker Compose: https://docs.docker.com/compose/install/${NC}"
        exit 1
    fi
    
    if ! docker info > /dev/null 2>&1; then
        echo -e "${RED}错误: Docker 服务未启动${NC}"
        echo -e "${YELLOW}请先启动 Docker 服务${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Docker 环境检查通过${NC}"
}

# 安装功能
install_app() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}安装 ${APP_NAME}${NC}"
    echo -e "${BLUE}=======================================${NC}"
    
    check_docker
    
    # 检查是否已安装
    if [ -d "${APP_DIR}" ] && [ -f "${APP_DIR}/docker-compose.yml" ]; then
        echo -e "${YELLOW}警告: ${APP_NAME} 似乎已安装在 ${APP_DIR}${NC}"
        read -p "是否覆盖安装？(y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo -e "${BLUE}安装已取消${NC}"
            exit 0
        fi
    fi
    
    # 创建安装目录
    echo -e "${BLUE}[1/5] 创建安装目录${NC}"
    mkdir -p "${APP_DIR}"
    cd "${APP_DIR}" || exit 1
    
    # 下载配置文件
    echo -e "${BLUE}[2/5] 下载配置文件${NC}"
    curl -fsSL "${DOCKER_COMPOSE_URL}" -o docker-compose.yml
    mkdir -p data configs certs data/redis
    
    # 下载配置文件模板
    curl -fsSL "${GITHUB_REPO}/raw/main/configs/config.example.yml" -o configs/config.example.yml
    if [ ! -f "configs/config.yml" ]; then
        cp configs/config.example.yml configs/config.yml
    fi
    
    # 创建Redis配置文件
    if [ ! -f "data/redis/redis.conf" ]; then
        cat > data/redis/redis.conf << EOF
# Redis配置文件
bind 0.0.0.0
protected-mode no
port 6379
dir /data
dbfilename dump.rdb
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfilename "appendonly.aof"
EOF
        echo -e "${GREEN}✓ Redis配置文件创建成功${NC}"
    fi
    
    # 启动服务
    echo -e "${BLUE}[3/5] 启动服务${NC}"
    # 获取公网IP作为默认值
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "localhost")
    HOST_IP="${PUBLIC_IP}" docker-compose up -d
    
    # 等待服务启动
    echo -e "${BLUE}[4/5] 等待服务启动...${NC}"
    sleep 5
    
    # 验证安装
    echo -e "${BLUE}[5/5] 验证安装${NC}"
    if docker-compose ps | grep -q "prerender-shield.*Up"; then
        echo -e "${GREEN}✓ ${APP_NAME} 安装成功！${NC}"
        show_access_info
    else
        echo -e "${RED}错误: ${APP_NAME} 安装失败${NC}"
        echo -e "${YELLOW}请查看日志: $0 logs${NC}"
        exit 1
    fi
}

# 升级功能
upgrade_app() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}升级 ${APP_NAME}${NC}"
    echo -e "${BLUE}=======================================${NC}"
    
    check_docker
    
    # 检查是否已安装
    if [ ! -d "${APP_DIR}" ] || [ ! -f "${APP_DIR}/docker-compose.yml" ]; then
        echo -e "${RED}错误: ${APP_NAME} 未安装在 ${APP_DIR}${NC}"
        echo -e "${YELLOW}请先安装 ${APP_NAME}: $0 install${NC}"
        exit 1
    fi
    
    cd "${APP_DIR}" || exit 1
    
    # 备份配置
    echo -e "${BLUE}[1/5] 备份配置文件${NC}"
    cp docker-compose.yml docker-compose.yml.bak.$(date +%Y%m%d%H%M%S)
    
    # 下载最新配置
    echo -e "${BLUE}[2/5] 下载最新配置${NC}"
    curl -fsSL "${DOCKER_COMPOSE_URL}" -o docker-compose.yml
    
    # 拉取最新镜像
    echo -e "${BLUE}[3/5] 拉取最新镜像${NC}"
    docker-compose pull
    
    # 重启服务
    echo -e "${BLUE}[4/5] 重启服务${NC}"
    docker-compose up -d
    
    # 验证升级
    echo -e "${BLUE}[5/5] 验证升级${NC}"
    if docker-compose ps | grep -q "prerender-shield.*Up"; then
        echo -e "${GREEN}✓ ${APP_NAME} 升级成功！${NC}"
        show_access_info
    else
        echo -e "${RED}错误: ${APP_NAME} 升级失败${NC}"
        echo -e "${YELLOW}请查看日志: $0 logs${NC}"
        echo -e "${YELLOW}可使用备份恢复: cp docker-compose.yml.bak.* docker-compose.yml && docker-compose up -d${NC}"
        exit 1
    fi
}

# 修复功能
repair_app() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}修复 ${APP_NAME} 安装${NC}"
    echo -e "${BLUE}=======================================${NC}"
    
    check_docker
    
    # 检查是否已安装
    if [ ! -d "${APP_DIR}" ]; then
        echo -e "${RED}错误: ${APP_NAME} 未安装在 ${APP_DIR}${NC}"
        echo -e "${YELLOW}请先安装 ${APP_NAME}: $0 install${NC}"
        exit 1
    fi
    
    cd "${APP_DIR}" || exit 1
    
    # 重新创建必要目录
    echo -e "${BLUE}[1/4] 重新创建必要目录${NC}"
    mkdir -p data configs certs data/redis
    
    # 重新下载配置文件
    echo -e "${BLUE}[2/4] 重新下载配置文件${NC}"
    curl -fsSL "${DOCKER_COMPOSE_URL}" -o docker-compose.yml
    if [ ! -f "configs/config.example.yml" ]; then
        curl -fsSL "${GITHUB_REPO}/raw/main/configs/config.example.yml" -o configs/config.example.yml
    fi
    if [ ! -f "configs/config.yml" ]; then
        cp configs/config.example.yml configs/config.yml
    fi
    
    # 重新创建Redis配置文件
    if [ ! -f "data/redis/redis.conf" ]; then
        cat > data/redis/redis.conf << EOF
# Redis配置文件
bind 0.0.0.0
protected-mode no
port 6379
dir /data
dbfilename dump.rdb
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfilename "appendonly.aof"
EOF
        echo -e "${GREEN}✓ Redis配置文件创建成功${NC}"
    fi
    
    # 重新启动服务
    echo -e "${BLUE}[3/4] 重新启动服务${NC}"
    # 获取公网IP作为默认值
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "localhost")
    HOST_IP="${PUBLIC_IP}" docker-compose up -d
    
    # 验证修复
    echo -e "${BLUE}[4/4] 验证修复${NC}"
    if docker-compose ps | grep -q "prerender-shield.*Up"; then
        echo -e "${GREEN}✓ ${APP_NAME} 修复成功！${NC}"
        show_access_info
    else
        echo -e "${RED}错误: ${APP_NAME} 修复失败${NC}"
        echo -e "${YELLOW}请查看日志: $0 logs${NC}"
        exit 1
    fi
}

# 卸载功能
uninstall_app() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}卸载 ${APP_NAME}${NC}"
    echo -e "${BLUE}=======================================${NC}"
    
    # 检查是否已安装
    if [ ! -d "${APP_DIR}" ]; then
        echo -e "${YELLOW}警告: ${APP_NAME} 似乎未安装在 ${APP_DIR}${NC}"
        if [ "$FORCE" != "true" ]; then
            echo -e "${BLUE}卸载已取消${NC}"
            exit 0
        fi
    fi
    
    # 确认卸载
    echo -e "${RED}警告: 此操作将永久删除 ${APP_NAME} 及其所有数据！${NC}"
    read -p "是否继续？(y/N): " -r
    if [[ ! $REPLY =~ ^[Yy]$ ]] && [ "$FORCE" != "true" ]; then
        echo -e "${BLUE}卸载已取消${NC}"
        exit 0
    fi
    
    # 停止服务
    echo -e "${BLUE}[1/3] 停止服务${NC}"
    if [ -f "${APP_DIR}/docker-compose.yml" ]; then
        cd "${APP_DIR}" || exit 1
        docker-compose down -v
    fi
    
    # 删除安装目录
    echo -e "${BLUE}[2/3] 删除安装目录${NC}"
    rm -rf "${APP_DIR}"
    
    # 清理 Docker 资源
    echo -e "${BLUE}[3/3] 清理 Docker 资源${NC}"
    docker system prune -f --volumes 2>/dev/null || true
    
    echo -e "${GREEN}✓ ${APP_NAME} 卸载成功！${NC}"
}

# 状态查看功能
show_status() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}${APP_NAME} 状态${NC}"
    echo -e "${BLUE}=======================================${NC}"
    
    if [ ! -d "${APP_DIR}" ] || [ ! -f "${APP_DIR}/docker-compose.yml" ]; then
        echo -e "${YELLOW}${APP_NAME} 未安装在 ${APP_DIR}${NC}"
        exit 0
    fi
    
    cd "${APP_DIR}" || exit 1
    docker-compose ps
    echo -e ""
    show_access_info
}

# 日志查看功能
show_logs() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}${APP_NAME} 日志${NC}"
    echo -e "${BLUE}=======================================${NC}"
    
    if [ ! -d "${APP_DIR}" ] || [ ! -f "${APP_DIR}/docker-compose.yml" ]; then
        echo -e "${YELLOW}${APP_NAME} 未安装在 ${APP_DIR}${NC}"
        exit 1
    fi
    
    cd "${APP_DIR}" || exit 1
    docker-compose logs "$@"
}

# 显示访问信息
show_access_info() {
    local ip=$(hostname -I | awk '{print $1}')
    echo -e ""
    echo -e "${PURPLE}=======================================${NC}"
    echo -e "${GREEN}🎉 ${APP_NAME} 已成功安装！${NC}"
    echo -e "${PURPLE}=======================================${NC}"
    echo -e "${CYAN}访问地址:${NC}"
    echo -e "  管理控制台: http://${ip}:9597"
    echo -e "  API服务:    http://${ip}:9598"
    echo -e ""
    echo -e "${CYAN}默认账号:${NC}"
    echo -e "  用户名: admin"
    echo -e "  密码:   123456"
    echo -e ""
    echo -e "${CYAN}管理命令:${NC}"
    echo -e "  $0 status    # 查看状态"
    echo -e "  $0 logs      # 查看日志"
    echo -e "  $0 upgrade   # 升级版本"
    echo -e "${PURPLE}=======================================${NC}"
}

# 主程序
main() {
    # 解析命令行参数
    parse_args "$@"
    
    # 执行对应命令
    case "${COMMAND}" in
        install)
            install_app
            ;;
        upgrade)
            upgrade_app
            ;;
        repair)
            repair_app
            ;;
        uninstall)
            uninstall_app
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "$@"
            ;;
        help)
            show_help
            ;;
        *)
            echo -e "${RED}错误: 未知命令 '${COMMAND}'${NC}"
            show_help
            exit 1
            ;;
    esac
}

# 执行主程序
main "$@"