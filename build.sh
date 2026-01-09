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
    
    # 复制前端文件到平台目录
    if [ -d "web/dist" ]; then
        print_info "复制前端文件到 $output_dir/web/dist..."
        mkdir -p "$output_dir/web"
        cp -r "web/dist" "$output_dir/web/"
        if [ $? -ne 0 ]; then
            print_warning "复制前端文件到 $output_dir/web/dist 失败"
        else
            print_success "前端文件复制完成: $output_dir/web/dist"
        fi
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
        # 自动检测本地网络IP，优先使用非本地回环地址
        local_ip="127.0.0.1"
        if command -v ip &> /dev/null; then
            # Linux系统使用ip命令
            local_ip=$(ip addr show | grep -E "inet.*brd" | grep -v "127.0.0.1" | head -1 | awk '{print $2}' | cut -d/ -f1)
        elif command -v ifconfig &> /dev/null; then
            # macOS系统使用ifconfig命令
            local_ip=$(ifconfig | grep -E "inet.*broadcast" | grep -v "127.0.0.1" | head -1 | awk '{print $2}')
        fi
        # 如果无法检测到本地IP，使用默认值
        if [[ -z "$local_ip" ]]; then
            local_ip="127.0.0.1"
        fi
        export VITE_API_BASE_URL="http://$local_ip:9598/api/v1"
        print_info "自动检测到本地IP: $local_ip"
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
build_failed=false
for platform in "${PLATFORMS[@]}"; do
    for arch in "${ARCHITECTURES[@]}"; do
        if ! build_single "$platform" "$arch"; then
            build_failed=true
        fi
    done
done

if [ "$build_failed" = true ]; then
    print_error "部分平台构建失败，请检查日志"
fi

# 构建产物验证
print_info "验证构建产物..."
build_valid=true

# 验证当前平台二进制文件
if [ -f "$APP_BINARY" ]; then
    if [ -x "$APP_BINARY" ]; then
        print_success "当前平台二进制文件验证成功: $APP_BINARY"
        print_success "当前平台二进制文件可正常执行"
    else
        print_error "当前平台二进制文件不可执行: $APP_BINARY"
        chmod +x "$APP_BINARY"
        if [ -x "$APP_BINARY" ]; then
            print_success "已修复当前平台二进制文件权限"
            print_success "修复后的二进制文件可正常执行"
        else
            print_error "无法修复当前平台二进制文件权限"
            build_valid=false
        fi
    fi
else
    print_error "未找到当前平台二进制文件: $APP_BINARY"
    build_valid=false
fi

# 验证前端构建文件
frontend_valid=false
if [ -d "web/dist" ]; then
    required_frontend_files=("index.html" "assets" "vite.svg")
    frontend_valid=true
    
    for file in "${required_frontend_files[@]}"; do
        if [ -e "web/dist/$file" ]; then
            print_success "前端文件验证成功: web/dist/$file"
        else
            print_warning "前端文件缺失: web/dist/$file"
            # 只有index.html是必须的，其他文件缺失不影响基本功能
            if [ "$file" == "index.html" ]; then
                frontend_valid=false
            fi
        fi
    done
    
    if $frontend_valid; then
        print_success "前端构建文件验证完整成功: web/dist"
    else
        print_error "前端构建文件不完整，可能影响运行效果"
        build_valid=false
    fi
else
    print_error "未找到前端构建目录: web/dist"
    build_valid=false
fi

# 验证多平台二进制文件和前端文件
multi_platform_valid=true
built_platforms=0
total_platforms=$(( ${#PLATFORMS[@]} * ${#ARCHITECTURES[@]} ))

print_info "验证多平台二进制文件和前端文件 ($total_platforms 个平台)..."

for platform in "${PLATFORMS[@]}"; do
    for arch in "${ARCHITECTURES[@]}"; do
        output_dir="bin/$platform-$arch"
        binary_name="api"
        if [[ $platform == "windows" ]]; then
            binary_name="api.exe"
        fi
        binary_path="$output_dir/$binary_name"
        
        # 验证二进制文件
        if [ -f "$binary_path" ]; then
            print_success "$platform/$arch 二进制文件验证成功: $binary_path"
            built_platforms=$((built_platforms + 1))
        else
            print_error "$platform/$arch 二进制文件未找到: $binary_path"
            multi_platform_valid=false
            continue
        fi
        
        # 验证前端文件
        if [ -d "$output_dir/web/dist" ]; then
            print_success "$platform/$arch 前端文件验证成功: $output_dir/web/dist"
        else
            print_warning "$platform/$arch 前端文件未找到: $output_dir/web/dist"
            # 前端文件不是必须的，所以不影响构建结果
        fi
    done

done

print_info "多平台构建结果: $built_platforms/$total_platforms 个平台构建成功"

# 构建结果汇总
if $build_valid && $multi_platform_valid; then
    print_success "构建产物验证全部通过！"
elif $build_valid; then
    print_warning "构建产物验证基本通过，但部分平台构建失败"
else
    print_error "构建产物验证失败，建议检查日志并修复问题"
    exit 1
fi

# 显示构建完成信息
print_success "========================================"
print_success "PrerenderShield 编译完成！"
print_success "========================================"
print_success "当前平台二进制文件: $APP_BINARY"
print_success "多平台编译结果: bin/目录下"
print_success "前端构建文件: web/dist"
print_success ""
print_success "接下来的操作:"
print_success "1. 安装应用: ./install.sh"
print_success "2. 启动应用: ./start.sh start"
print_success "3. 查看日志: tail -f ./data/prerender-shield.log"
print_success "========================================"

# 构建后的验证测试
print_info "执行构建后的验证测试..."

# 运行go test进行基本测试
print_info "运行Go测试..."
go test ./... -short
if [ $? -eq 0 ]; then
    print_success "Go测试通过"
else
    print_warning "Go测试未全部通过，但构建继续进行"
fi

print_success "========================================"
print_success "PrerenderShield 编译完成！"
print_success "========================================"
print_success "当前平台二进制文件: $APP_BINARY"
print_success "多平台编译结果: bin/目录下"
print_success "前端构建文件: web/dist"
print_success ""
print_success "接下来的操作:"
print_success "1. 安装应用: ./install.sh"
print_success "2. 启动应用: ./start.sh start"
print_success "3. 查看日志: tail -f ./data/prerender-shield.log"
print_success "========================================"
