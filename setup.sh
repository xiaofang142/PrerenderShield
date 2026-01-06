#!/bin/bash

# Prerender-Shield 一键安装脚本
# 参考长亭雷池WAF安装方式设计

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 常量定义
APP_NAME="Prerender-Shield"
APP_DIR="/opt/prerender-shield"
GITHUB_REPO="https://github.com/your-org/prerender-shield"
DOCKER_COMPOSE_URL="${GITHUB_REPO}/raw/main/docker-compose.yml"

# 显示欢迎信息
welcome() {
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}${APP_NAME} 一键安装脚本${NC}"
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${YELLOW}安装将在 3 分钟内完成...${NC}"
    echo -e "${BLUE}=======================================${NC}"
}

# 检查系统环境
check_environment() {
    echo -e "${BLUE}[1/5] 检查系统环境${NC}"
    
    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}错误: Docker 未安装${NC}"
        echo -e "${YELLOW}请先安装 Docker: https://docs.docker.com/get-docker/${NC}"
        exit 1
    fi
    
    # 检查 Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}错误: Docker Compose 未安装${NC}"
        echo -e "${YELLOW}请先安装 Docker Compose: https://docs.docker.com/compose/install/${NC}"
        exit 1
    fi
    
    # 检查 Docker 服务状态
    if ! docker info > /dev/null 2>&1; then
        echo -e "${RED}错误: Docker 服务未启动${NC}"
        echo -e "${YELLOW}请先启动 Docker 服务${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ 系统环境检查通过${NC}"
}

# 创建安装目录
create_install_dir() {
    echo -e "${BLUE}[2/5] 创建安装目录${NC}"
    
    # 创建应用目录
    mkdir -p "${APP_DIR}"
    cd "${APP_DIR}" || exit 1
    
    echo -e "${GREEN}✓ 安装目录创建成功: ${APP_DIR}${NC}"
}

# 下载配置文件
download_config() {
    echo -e "${BLUE}[3/5] 下载配置文件${NC}"
    
    # 下载 docker-compose.yml
    if [ ! -f "docker-compose.yml" ]; then
        curl -fsSL "${DOCKER_COMPOSE_URL}" -o docker-compose.yml
        echo -e "${GREEN}✓ docker-compose.yml 下载成功${NC}"
    else
        echo -e "${YELLOW}⚠ docker-compose.yml 已存在，跳过下载${NC}"
    fi
    
    # 创建数据目录
    mkdir -p data configs certs
    
    # 下载配置文件模板
    if [ ! -f "configs/config.example.yml" ]; then
        curl -fsSL "${GITHUB_REPO}/raw/main/configs/config.example.yml" -o configs/config.example.yml
        echo -e "${GREEN}✓ 配置文件模板下载成功${NC}"
    fi
    
    # 如果没有配置文件，从模板复制
    if [ ! -f "configs/config.yml" ]; then
        cp configs/config.example.yml configs/config.yml
        echo -e "${GREEN}✓ 配置文件创建成功${NC}"
    fi
}

# 启动服务
start_service() {
    echo -e "${BLUE}[4/5] 启动服务${NC}"
    
    # 启动 Docker Compose
    docker-compose up -d
    
    echo -e "${GREEN}✓ 服务启动成功${NC}"
}

# 显示安装结果
show_result() {
    echo -e "${BLUE}[5/5] 安装完成${NC}"
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${GREEN}🎉 ${APP_NAME} 安装成功！${NC}"
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${YELLOW}管理控制台:${NC} http://$(hostname -I | awk '{print $1}'):9597"
    echo -e "${YELLOW}API服务:${NC} http://$(hostname -I | awk '{print $1}'):9598"
    echo -e "${YELLOW}默认账号:${NC} admin"
    echo -e "${YELLOW}默认密码:${NC} 123456"
    echo -e "${BLUE}=======================================${NC}"
    echo -e "${YELLOW}后续管理命令:${NC}"
    echo -e "  cd ${APP_DIR} && docker-compose up -d   # 启动服务"
    echo -e "  cd ${APP_DIR} && docker-compose down     # 停止服务"
    echo -e "  cd ${APP_DIR} && docker-compose logs -f  # 查看日志"
    echo -e "${BLUE}=======================================${NC}"
}

# 主程序
main() {
    welcome
    check_environment
    create_install_dir
    download_config
    start_service
    show_result
}

# 执行主程序
main