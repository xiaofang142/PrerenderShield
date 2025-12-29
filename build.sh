#!/bin/bash

# PrerenderShield Docker 构建脚本

echo "========================================"
echo "PrerenderShield Docker 构建脚本"
echo "========================================"

# 检查是否安装Docker
if ! command -v docker &> /dev/null; then
    echo "错误: 未安装Docker，无法构建镜像"
    exit 1
fi

# 检查是否安装Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "警告: 未安装Docker Compose，无法使用docker-compose命令"
fi

# 构建Docker镜像
echo "构建Docker镜像..."
docker build -t prerender-shield:latest .

if [ $? -ne 0 ]; then
    echo "错误: Docker镜像构建失败"
    exit 1
fi

echo "Docker镜像构建成功！"
echo "镜像名称: prerender-shield:latest"

# 显示镜像信息
echo "\n镜像信息:"
docker images prerender-shield:latest

echo "========================================"
echo "构建完成，可以使用以下命令启动应用:"
echo "1. 使用Docker Compose启动: docker-compose up -d"
echo "2. 使用Docker直接启动: docker run -d -p 8080:8080 prerender-shield:latest"
echo "========================================"
