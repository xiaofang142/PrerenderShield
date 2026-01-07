#!/bin/bash

# PrerenderShield 一键发布脚本
# 用于构建多平台Go二进制文件和编译前端代码

APP_NAME="prerender-shield"
RELEASE_DIR="./release"
WEB_DIR="./web"
API_DIR="./cmd/api"

# 支持的平台列表
PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")

usage() {
    echo "========================================"
    echo "PrerenderShield 一键发布脚本"
    echo "========================================"
    echo "用法: $0 {build|clean}"
    echo ""
    echo "选项:"
    echo "  build    构建所有平台的二进制文件和前端代码"
    echo "  clean    清理发布目录"
    echo ""
    exit 1
}

check_dependencies() {
    echo "检查依赖..."
    
    # 检查Go
    if ! command -v go &> /dev/null; then
        echo "错误: 未安装Go，无法构建应用程序"
        exit 1
    fi
    
    # 检查Node.js和npm
    if ! command -v node &> /dev/null || ! command -v npm &> /dev/null; then
        echo "错误: 未安装Node.js或npm，无法构建前端代码"
        exit 1
    fi
    
    echo "依赖检查完成！"
}

clean_release() {
    echo "清理发布目录..."
    rm -rf "$RELEASE_DIR"
    mkdir -p "$RELEASE_DIR"
    echo "发布目录清理完成！"
}

build_frontend() {
    echo "编译前端代码..."
    
    cd "$WEB_DIR" || exit 1
    
    # 安装依赖
    npm install
    if [ $? -ne 0 ]; then
        echo "错误: 前端依赖安装失败"
        exit 1
    fi
    
    # 获取公网IP作为API地址
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "localhost")
    export VITE_API_BASE_URL="http://${PUBLIC_IP}:9598/api/v1"
    
    echo "使用API地址: ${VITE_API_BASE_URL}"
    
    # 构建前端代码
    npm run build
    if [ $? -ne 0 ]; then
        echo "错误: 前端代码编译失败"
        exit 1
    fi
    
    cd - || exit 1
    echo "前端代码编译完成！"
}

build_backend() {
    echo "构建后端二进制文件..."
    
    # 清理旧的二进制文件
    [ -f "./$APP_NAME" ] && rm -f "./$APP_NAME"
    
    # 配置Go代理加速
    export GOPROXY="https://goproxy.cn,direct"
    # 安装Go依赖
    go mod tidy
    if [ $? -ne 0 ]; then
        echo "错误: Go依赖安装失败"
        exit 1
    fi
    
    # 构建当前平台的二进制文件
    echo "构建当前平台二进制文件..."
    go build -o "./$APP_NAME" "$API_DIR"
    if [ $? -ne 0 ]; then
        echo "错误: 当前平台二进制文件构建失败"
        exit 1
    fi
    
    # 构建多平台二进制文件
    for PLATFORM in "${PLATFORMS[@]}"; do
        IFS="/" read -r OS ARCH <<< "$PLATFORM"
        echo "构建 $OS/$ARCH 二进制文件..."
        
        # 设置输出文件名
        OUTPUT_NAME="$APP_NAME"
        if [ "$OS" = "windows" ]; then
            OUTPUT_NAME="$OUTPUT_NAME.exe"
        fi
        
        # 设置Go环境变量
        GOOS="$OS" GOARCH="$ARCH" go build -o "$RELEASE_DIR/$OUTPUT_NAME" "$API_DIR"
        if [ $? -ne 0 ]; then
            echo "错误: $OS/$ARCH 二进制文件构建失败"
            exit 1
        fi
    done
    
    echo "后端二进制文件构建完成！"
}

package_release() {
    echo "打包发布文件..."
    
    # 创建发布目录
    mkdir -p "$RELEASE_DIR"
    
    # 复制配置文件模板
    cp -r ./configs "$RELEASE_DIR/"
    
    # 复制启动脚本
    cp ./start.sh "$RELEASE_DIR/"
    chmod +x "$RELEASE_DIR/start.sh"
    
    # 复制前端构建文件
    cp -r "$WEB_DIR/dist" "$RELEASE_DIR/web/"
    
    echo "发布文件打包完成！"
}

# 主程序
if [ $# -eq 0 ]; then
    echo "未提供参数，默认执行build操作"
    set -- build
fi

case "$1" in
    build)
        check_dependencies
        clean_release
        build_frontend
        build_backend
        package_release
        echo "========================================"
        echo "发布构建完成！"
        echo "发布文件位于: $RELEASE_DIR"
        echo "========================================"
        ;;
    clean)
        clean_release
        echo "========================================"
        echo "发布目录清理完成！"
        echo "========================================"
        ;;
    *)
        usage
        ;;
esac
