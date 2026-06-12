#!/bin/bash
# Prerender Shield 生产环境部署脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Prerender Shield 生产环境部署${NC}"
echo -e "${GREEN}========================================${NC}"

# 检查是否为 root 用户
if [ "$EUID" -eq 0 ]; then
    echo -e "${YELLOW}警告: 不建议以 root 用户运行${NC}"
fi

# 检查 Redis
echo -e "${YELLOW}检查 Redis 服务...${NC}"
if ! command -v redis-cli &> /dev/null; then
    echo -e "${RED}错误: 未安装 Redis${NC}"
    echo "请先安装 Redis: sudo apt install redis-server"
    exit 1
fi

if ! redis-cli ping &> /dev/null; then
    echo -e "${YELLOW}Redis 未运行，尝试启动...${NC}"
    if command -v systemctl &> /dev/null; then
        sudo systemctl start redis-server
    else
        sudo service redis-server start
    fi
fi

echo -e "${GREEN}Redis 服务正常${NC}"

# 检查配置文件
CONFIG_FILE="bin/config/config.yml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${YELLOW}配置文件不存在，创建默认配置...${NC}"
    mkdir -p bin/config
    cp configs/config.example.yml "$CONFIG_FILE"
    echo -e "${GREEN}默认配置已创建: $CONFIG_FILE${NC}"
fi

# 停止旧服务
echo -e "${YELLOW}停止旧服务...${NC}"
if [ -f bin/data/prerender-shield.pid ]; then
    OLD_PID=$(cat bin/data/prerender-shield.pid)
    if kill -0 "$OLD_PID" 2>/dev/null; then
        kill "$OLD_PID"
        sleep 2
        echo -e "${GREEN}旧服务已停止${NC}"
    fi
fi

# 启动服务
echo -e "${YELLOW}启动 Prerender Shield...${NC}"
cd bin
./api start
cd ..

sleep 3

# 检查服务状态
echo -e "${YELLOW}检查服务状态...${NC}"
if curl -s http://localhost:9598/api/v1/health | grep -q "ok"; then
    echo -e "${GREEN}服务启动成功!${NC}"
else
    echo -e "${RED}服务启动失败，请检查日志${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  部署完成!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "管理控制台: ${GREEN}http://localhost:9597${NC}"
echo -e "API 服务: ${GREEN}http://localhost:9598${NC}"
echo -e "默认账号: ${GREEN}admin / 123456${NC}"
echo ""
echo -e "${YELLOW}请尽快修改默认密码!${NC}"
echo ""
