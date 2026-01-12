#!/bin/bash

set -euo pipefail

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

# 解析命令行参数，用于前端 API 主机设置
parse_args() {
    API_HOST=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --api-host)
                API_HOST="$2"; shift 2 ;;
            --api-host=*)
                API_HOST="${1#*=}"; shift 1 ;;
            *)
                break
        esac
    done
}

# 效验并生效 API_HOST
apply_api_host() {
    if [[ -n "$API_HOST" ]]; then
        local host="$API_HOST"
        if [[ "$host" != http://* && "$host" != https://* ]]; then
            host="http://$host"
        fi
        # 确保包含 /api/v1 路径
        if [[ "$host" == */api/v1* ]]; then
            VITE_API_BASE_URL="$host"
        else
            # 未包含 API 路径，追加
            VITE_API_BASE_URL="${host}/api/v1"
        fi
        export VITE_API_BASE_URL
        # 写入前端生产环境变量，便于容器/离线部署
        mkdir -p web
        echo "VITE_API_BASE_URL=$VITE_API_BASE_URL" > web/.env.production
        print_info "前端 VITE_API_BASE_URL 设置为: $VITE_API_BASE_URL"
    else
        # 使用现有环境变量优先级：环境变量优先于默认值
        :
    fi
}

# 获取当前脚本参数并应用
parse_args "$@"
apply_api_host "$API_HOST" 

# 获取当前平台信息
platform_info=$(detect_platform)
platform=$(echo $platform_info | cut -d' ' -f1)
arch=$(echo $platform_info | cut -d' ' -f2)

print_info "当前平台: $platform/$arch"

# 资源优化：为 Node 前端构建配置内存上限，确保在低内存环境也能跑
get_mem_mb() {
  local mem_mb=1024
  if [ "$(uname -s)" = "Darwin" ]; then
    if command -v sysctl >/dev/null 2>&1; then
      local mem_bytes=$(sysctl -n hw.memsize 2>/dev/null || echo 0)
      if [ -n "$mem_bytes" ] && [ "$mem_bytes" -gt 0 ]; then
        mem_mb=$((mem_bytes / 1024 / 1024))
      fi
    fi
  else
    if [ -f /proc/meminfo ]; then
      local mem_kb=$(grep -i MemTotal /proc/meminfo | awk '{print $2}')
      if [ -n "$mem_kb" ]; then
        mem_mb=$((mem_kb / 1024))
      fi
    fi
  fi
  echo "$mem_mb"
}

MEM_MB=$(get_mem_mb)
if (( MEM_MB < 1500 )); then
  NODE_MAX_OLD_SPACE_SIZE=768
else
  NODE_MAX_OLD_SPACE_SIZE=1024
fi
export NODE_OPTIONS="--max-old-space-size=${NODE_MAX_OLD_SPACE_SIZE}"
# 限制 npm 的并发请求数，降低并发带来的内存峰值
export npm_config_maxsockets=8
print_info "Node 内存限制: ${NODE_OPTIONS}，npm 最大并发套接字: ${npm_config_maxsockets}"

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
npm config set maxsockets "$npm_config_maxsockets" || true
npm install --legacy-peer-deps --silent
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

# 使用环境变量提供的 VITE_API_BASE_URL，若未设置则使用默认
if [[ -n "${VITE_API_BASE_URL:-}" ]]; then
    print_info "使用前端 API 地址: ${VITE_API_BASE_URL}"
else
    VITE_API_BASE_URL="http://127.0.0.1:9598/api/v1"
    print_info "未设置 VITE_API_BASE_URL，使用默认地址: $VITE_API_BASE_URL"
fi
export VITE_API_BASE_URL
print_info "前端API地址为: $VITE_API_BASE_URL"

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
GOOS=$platform GOARCH=$arch go build -ldflags "-s -w" -trimpath -o "$BINARY_PATH" ./cmd/api
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
print_success "接下来："
print_success "1. 安装应用: ./install.sh"
print_success "========================================"
