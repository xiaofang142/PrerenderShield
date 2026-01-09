#!/bin/bash

# PrerenderShield 服务端构建脚本

# 彩色输出定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# 获取当前平台类型
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    # 转换架构名称
    if [[ $arch == "x86_64" ]]; then
        arch="amd64"
    elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
        arch="arm64"
    fi
    
    echo "$os $arch"
}

echo "========================================"
echo -e "${BLUE}PrerenderShield 服务端构建脚本${NC}"
echo "========================================"

# 检查Go环境
if ! command -v go &> /dev/null; then
    print_error "错误: 未安装Go环境，无法编译Go代码"
    exit 1
fi

print_info "Go环境版本: $(go version)"

# 检查Node.js环境（用于构建前端）
if ! command -v npm &> /dev/null; then
    print_error "错误: 未安装Node.js环境，无法构建前端"
    exit 1
fi

print_info "Node.js版本: $(node --version)"
print_info "npm版本: $(npm --version)"

# 获取当前平台信息
platform_info=$(detect_platform)
platform=$(echo $platform_info | cut -d' ' -f1)
arch=$(echo $platform_info | cut -d' ' -f2)

print_info "当前平台: $platform/$arch"

# 配置Go模块镜像加速
export GOPROXY=https://goproxy.cn,direct
export GO111MODULE=on

print_info "配置Go模块镜像加速..."
print_info "GOPROXY设置为: $GOPROXY"

# 创建输出目录
BIN_DIR="bin"
BINARY_PATH="$BIN_DIR/api"
WEB_DIST_DIR="web/dist"
BIN_WEB_DIR="$BIN_DIR/web"

print_info "创建输出目录..."
mkdir -p "$BIN_DIR"
mkdir -p "$BIN_WEB_DIR"

# 构建前端
print_info "构建前端控制台..."
cd web

# 安装前端依赖
print_info "安装前端依赖..."
npm install --legacy-peer-deps
if [ $? -ne 0 ]; then
    print_warning "前端依赖安装失败，尝试使用 --force 选项..."
    npm install --force
    if [ $? -ne 0 ]; then
        print_error "前端依赖安装失败"
        print_warning "您可以手动安装前端依赖后重新运行脚本"
        exit 1
    fi
fi

print_success "前端依赖安装完成"

# 设置前端API地址
if [[ "$VITE_API_BASE_URL" == "" ]]; then
    api_ip=""
    # Check if IP is provided as command-line argument
    if [[ -n "$1" ]]; then
        api_ip="$1"
        print_info "使用命令行提供的服务器IP地址: $api_ip"
    else
        echo -e "\n${BLUE}[i] 请配置前端API地址:${NC}"
        while [[ -z "$api_ip" ]]; do
            read -p "  请输入服务器IP地址: " api_ip
            if [[ -z "$api_ip" ]]; then
                print_error "IP地址不能为空，请重新输入"
            fi
        done
        print_info "使用手动输入的服务器IP地址: $api_ip"
    fi
    export VITE_API_BASE_URL="http://$api_ip:9598/api/v1"
    print_info "使用构建的API地址: $VITE_API_BASE_URL"
else
    print_info "使用环境变量提供的API地址: $VITE_API_BASE_URL"
fi
print_info "设置前端API地址为: $VITE_API_BASE_URL"

print_info "开始构建前端..."
npm run build 
if [ $? -ne 0 ]; then
    print_error "前端构建失败"
    print_warning "前端构建失败，可能是内存不足导致"
    print_warning "建议："
    print_warning "1. 增加服务器内存（建议至少 4GB）"
    print_warning "2. 手动构建前端：cd web && npm run build"
    print_warning "3. 使用预构建的前端文件"
    exit 1
fi

print_success "前端构建完成，构建文件位于: $WEB_DIST_DIR"

cd ..

# 安装Go依赖
print_info "安装Go依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    print_error "Go依赖安装失败"
    exit 1
fi

print_success "Go依赖安装完成"

# 编译Go应用（仅当前平台）
print_info "编译Go应用（仅当前平台）..."
GOOS=$platform GOARCH=$arch go build -o "$BINARY_PATH" ./cmd/api
if [ $? -ne 0 ]; then
    print_error "Go应用编译失败"
    exit 1
fi

# 设置二进制文件权限
chmod +x "$BINARY_PATH"
print_success "Go应用编译完成: $BINARY_PATH"

# 复制前端代码到bin/web目录
print_info "复制前端代码到 $BIN_WEB_DIR 目录..."
rm -rf "$BIN_WEB_DIR"/*
cp -r "$WEB_DIST_DIR"/* "$BIN_WEB_DIR/"

print_success "前端代码复制完成"

# 验证构建产物
print_info "验证构建产物..."

# 验证二进制文件
if [ -f "$BINARY_PATH" ] && [ -x "$BINARY_PATH" ]; then
    print_success "二进制文件验证成功: $BINARY_PATH"
else
    print_error "二进制文件验证失败: $BINARY_PATH"
    exit 1
fi

# 验证前端文件
if [ -d "$BIN_WEB_DIR" ] && [ -f "$BIN_WEB_DIR/index.html" ]; then
    print_success "前端文件验证成功: $BIN_WEB_DIR"
else
    print_error "前端文件验证失败: $BIN_WEB_DIR"
    exit 1
fi

print_success "========================================"
print_success "PrerenderShield 构建完成！"
print_success "========================================"
print_success "二进制文件: $BINARY_PATH"
print_success "前端文件: $BIN_WEB_DIR"
print_success ""
print_success "接下来的操作:"
print_success "1. 安装应用: ./install.sh"
print_success "========================================"
