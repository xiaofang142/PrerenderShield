#!/bin/bash

# PrerenderShield 跨平台编译脚本

APP_NAME="prerender-shield"
APP_BINARY="./api"

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

# 定义要编译的平台和架构
PLATFORMS=("linux" "darwin" "windows")
ARCHITECTURES=("amd64" "arm64")

# 构建单个平台的二进制文件
build_single() {
    local platform=$1
    local arch=$2
    local output_dir="bin/$platform-$arch"
    local binary_name="api"
    
    if [[ $platform == "windows" ]]; then
        binary_name="api.exe"
    fi
    
    print_info "构建 $platform/$arch 版本..."
    mkdir -p "$output_dir"
    
    # 交叉编译
    GOOS=$platform GOARCH=$arch go build -o "$output_dir/$binary_name" ./cmd/api
    if [ $? -ne 0 ]; then
        print_error "构建 $platform/$arch 失败"
        return 1
    fi
    
    print_success "$platform/$arch 构建完成: $output_dir/$binary_name"
    return 0
}

echo "========================================"
echo -e "${BLUE}PrerenderShield 跨平台编译脚本${NC}"
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

# 配置Go模块镜像加速
export GOPROXY=https://goproxy.cn,direct
export GO111MODULE=on

print_info "配置Go模块镜像加速..."
print_info "GOPROXY设置为: $GOPROXY"

# 构建前端（先构建前端，确保静态资源在编译Go前准备好）
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

# 设置前端API地址，开发者使用的构建脚本默认使用本地IP
# 支持通过环境变量VITE_API_BASE_URL覆盖默认值
if [[ "$VITE_API_BASE_URL" == "" ]]; then
    # 构建脚本默认使用本地IP，开发者可以通过环境变量覆盖
    export VITE_API_BASE_URL="http://127.0.0.1:9598/api/v1"
    print_info "使用默认本地IP: 127.0.0.1"
else
    print_info "使用环境变量提供的API地址: $VITE_API_BASE_URL"
fi
print_info "设置前端API地址为: $VITE_API_BASE_URL"

print_info "开始构建前端..."
# 使用 --sourcemap false 减少内存使用
npm run build -- --sourcemap false
if [ $? -ne 0 ]; then
    print_error "前端构建失败"
    print_warning "前端构建失败，可能是内存不足导致"
    print_warning "建议："
    print_warning "1. 增加服务器内存（建议至少 4GB）"
    print_warning "2. 手动构建前端：cd web && npm run build -- --sourcemap false"
    print_warning "3. 使用预构建的前端文件"
    exit 1
fi

print_success "前端构建完成，构建文件位于: web/dist"

cd ..

# 安装Go依赖
print_info "安装Go依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    print_error "Go依赖安装失败"
    exit 1
fi

print_success "Go依赖安装完成"

# 构建当前平台的二进制文件
print_info "构建当前平台的二进制文件..."
go build -o "$APP_BINARY" ./cmd/api
if [ $? -ne 0 ]; then
    print_error "当前平台构建失败"
    exit 1
fi

print_success "当前平台构建完成，二进制文件: $APP_BINARY"

# 构建所有平台的二进制文件
print_info "开始构建所有平台的二进制文件..."
for platform in "${PLATFORMS[@]}"; do
    for arch in "${ARCHITECTURES[@]}"; do
        build_single "$platform" "$arch"
    done
done

print_success "========================================"
print_success "PrerenderShield 编译完成！"
print_success "========================================"
print_success "当前平台二进制文件: $APP_BINARY"
print_success "多平台编译结果: bin/目录下"
print_success "前端构建文件: web/dist"
print_success "可以使用 ./start.sh 启动应用"
print_success "========================================"
