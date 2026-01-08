#!/bin/bash

set -e

# 检查是否需要生成配置文件
if [ ! -f "./config.yml" ]; then
    echo "[INFO] 生成配置文件..."
    cp ./config.example.yml ./config.yml
    
    # 设置默认配置
    sed -i 's|data_dir: ./data|data_dir: /app/data|g' ./config.yml
    sed -i 's|static_dir: ./static|static_dir: /app/static|g' ./config.yml
    sed -i 's|admin_static_dir: ./web/dist|admin_static_dir: /app/web/dist|g' ./config.yml
    sed -i 's|redis_url: "localhost:6379"|redis_url: "redis:6379"|g' ./config.yml
fi

# 创建必要的目录
mkdir -p ./data ./static

# 启动应用
echo "[INFO] 启动PrerenderShield服务..."
exec ./api "$@"
