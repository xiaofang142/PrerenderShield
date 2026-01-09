#!/bin/bash

# ============================================================================
# PrerenderShield 一键安装脚本 (方案A - 原生安装)
# ============================================================================
#
# 功能特点：
# 1. 跨平台支持 (Linux/macOS/Windows WSL2)
# 2. 自动依赖检测和安装
# 3. 浏览器环境自动配置
# 4. 系统服务自动配置
# 5. 智能配置初始化
# ============================================================================

set -euo pipefail

# ============================================================================
# 全局变量和常量
# ============================================================================

APP_NAME="prerender-shield"
APP_VERSION="1.0.1"
INSTALL_DIR="/opt/${APP_NAME}"
CONFIG_DIR="/etc/${APP_NAME}"
DATA_DIR="/var/lib/${APP_NAME}"
LOG_DIR="/var/log/${APP_NAME}"
SYSTEMD_SERVICE="${APP_NAME}.service"

# 彩色输出定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 操作系统检测
OS=""
OS_TYPE=""
ARCH=""
PACKAGE_MANAGER=""
DISTRO=""

# ============================================================================
# 工具函数
# ============================================================================

print_header() {
    echo -e "${BLUE}"
    echo "===================================================================="
    echo "PrerenderShield 安装程序 v${APP_VERSION}"
    echo "===================================================================="
    echo -e "${NC}"
}

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

print_step() {
    echo -e "${BLUE}"
    echo "--------------------------------------------------------------------"
    echo "步骤 $1: $2"
    echo "--------------------------------------------------------------------"
    echo -e "${NC}"
}

check_root() {
    # macOS (Darwin) 不应该以root身份运行，因为Homebrew禁止root
    if [[ "$OS" == "darwin" ]]; then
        if [[ $EUID -eq 0 ]]; then
            print_error "在macOS上不应以root身份运行此脚本，Homebrew禁止root操作"
            print_error "请以普通用户身份运行，脚本会在需要时请求sudo权限"
            exit 1
        else
            print_info "在macOS上以普通用户身份，运行继续安装..."
            return 0
        fi
    fi
    
    # Linux系统需要root权限进行系统级安装
    if [[ "$OS" == "linux" ]]; then
        if [[ $EUID -eq 0 ]]; then
            print_warning "正在以root用户运行，继续安装..."
            return 0
        else
            print_error "在Linux上请sudo或以root此用户运行使用脚本"
            exit 1
        fi
    fi
    
    # 其他操作系统
    print_warning "未知操作系统类型，跳过root检查..."
    return 0
}

# ============================================================================
# 操作系统检测和初始化
# ============================================================================

detect_os() {
    print_step "1" "检测和操作系统架构"
    
    # 检测操作系统类型
    OS_TYPE=$(uname -s)
    ARCH=$(uname -m)
    
    case "$OS_TYPE" in
        Linux) 
            OS="linux"
            # 检测Linux发行版
            if [[ -f /etc/os-release ]]; then
                . /etc/os-release
                DISTRO=$ID
                
                # 改进的发行版检测
                case $ID in
                    ubuntu|debian|linuxmint|pop|zorin|elementary)
                        PACKAGE_MANAGER="apt-get"
                        ;;
                    centos|rhel|fedora|rocky|alma|oracle|amzn)
                        if [[ "$VERSION_ID" -ge 33 || "$ID" == "fedora" ]]; then
                            PACKAGE_MANAGER="dnf"
                        else
                            PACKAGE_MANAGER="yum"
                        fi
                        ;;
                    arch|manjaro|endeavouros)
                        PACKAGE_MANAGER="pacman"
                        ;;
                    opensuse|opensuse-leap|opensuse-tumbleweed|sles)
                        PACKAGE_MANAGER="zypper"
                        ;;
                    alpine)
                        PACKAGE_MANAGER="apk"
                        ;;
                    *)
                        print_warning "未识别的Linux发行版: $ID, 尝试自动检测包管理器"
                        # 自动检测包管理器
                        if command -v apt-get &> /dev/null; then
                            PACKAGE_MANAGER="apt-get"
                        elif command -v dnf &> /dev/null; then
                            PACKAGE_MANAGER="dnf"
                        elif command -v yum &> /dev/null; then
                            PACKAGE_MANAGER="yum"
                        elif command -v pacman &> /dev/null; then
                            PACKAGE_MANAGER="pacman"
                        elif command -v zypper &> /dev/null; then
                            PACKAGE_MANAGER="zypper"
                        elif command -v apk &> /dev/null; then
                            PACKAGE_MANAGER="apk"
                        else
                            print_error "无法检测到兼容的包管理器"
                            exit 1
                        fi
                        ;;
                esac
            elif [[ -f /etc/debian_version ]]; then
                DISTRO="debian"
                PACKAGE_MANAGER="apt-get"
            elif [[ -f /etc/redhat-release ]]; then
                DISTRO="rhel"
                if command -v dnf &> /dev/null; then
                    PACKAGE_MANAGER="dnf"
                else
                    PACKAGE_MANAGER="yum"
                fi
            elif [[ -f /etc/arch-release ]]; then
                DISTRO="arch"
                PACKAGE_MANAGER="pacman"
            elif [[ -f /etc/SuSE-release ]]; then
                DISTRO="opensuse"
                PACKAGE_MANAGER="zypper"
            elif [[ -f /etc/alpine-release ]]; then
                DISTRO="alpine"
                PACKAGE_MANAGER="apk"
            else
                print_warning "无法检测Linux发行版，尝试自动检测包管理器"
                # 自动检测包管理器
                if command -v apt-get &> /dev/null; then
                    PACKAGE_MANAGER="apt-get"
                    DISTRO="debian"
                elif command -v dnf &> /dev/null; then
                    PACKAGE_MANAGER="dnf"
                    DISTRO="fedora"
                elif command -v yum &> /dev/null; then
                    PACKAGE_MANAGER="yum"
                    DISTRO="centos"
                elif command -v pacman &> /dev/null; then
                    PACKAGE_MANAGER="pacman"
                    DISTRO="arch"
                elif command -v zypper &> /dev/null; then
                    PACKAGE_MANAGER="zypper"
                    DISTRO="opensuse"
                elif command -v apk &> /dev/null; then
                    PACKAGE_MANAGER="apk"
                    DISTRO="alpine"
                else
                    print_error "无法检测到兼容的包管理器"
                    exit 1
                fi
            fi
            ;;
        Darwin) 
            OS="darwin"
            PACKAGE_MANAGER="brew"
            ;;
        *)
            print_error "不支持的操作系统: $OS_TYPE"
            exit 1
            ;;
    esac
    
    print_info "操作系统: $OS_TYPE ($OS)"
    print_info "架构: $ARCH"
    print_info "发行版: ${DISTRO:-未知}"
    print_info "包管理器: $PACKAGE_MANAGER"
}

# ============================================================================
# 依赖检测和安装
# ============================================================================

install_dependencies() {
    print_step "2" "安装系统依赖"
    
    case "$OS" in
        linux)
            install_dependencies_linux
            ;;
        darwin)
            install_dependencies_macos
            ;;
        *)
            print_error "不支持的操作系统"
            exit 1
            ;;
    esac
}

install_dependencies_linux() {
    print_info "更新包管理器..."
    case "$PACKAGE_MANAGER" in
        apt-get)
            sudo apt-get update -y
            ;;
        yum)
            # 处理libzip5依赖冲突问题
            print_info "检查并解决libzip5依赖冲突..."
            # 尝试移除冲突的libzip5-devel包（如果存在）
            if sudo yum list installed | grep -q libzip5-devel; then
                sudo yum remove -y libzip5-devel libzip5-tools 2>/dev/null || true
            fi
            # 先安装libzstd依赖（如果需要）
            if ! sudo yum list installed | grep -q libzstd; then
                sudo yum install -y libzstd 2>/dev/null || true
            fi
            # 使用--skip-broken选项跳过冲突的包
            sudo yum update -y --skip-broken
            ;;
        dnf)
            sudo dnf update -y
            ;;
        pacman)
            sudo pacman -Sy
            ;;
        zypper)
            sudo zypper refresh -y
            ;;
        apk)
            sudo apk update
            ;;
    esac
    
    print_info "安装基础工具..."
    
    # 基础工具列表
    local basic_tools=()
    
    # 检查并添加需要安装的基础工具
    if ! command -v curl &> /dev/null; then
        basic_tools+=(curl)
    fi
    if ! command -v wget &> /dev/null; then
        basic_tools+=(wget)
    fi
    if ! command -v git &> /dev/null; then
        basic_tools+=(git)
    fi
    
    # 根据包管理器安装基础工具
    if [ ${#basic_tools[@]} -gt 0 ]; then
        case "$PACKAGE_MANAGER" in
            apt-get)
                sudo apt-get install -y "${basic_tools[@]}" build-essential
                ;;
            yum|dnf)
                sudo $PACKAGE_MANAGER install -y "${basic_tools[@]}" gcc make
                ;;
            pacman)
                sudo pacman -S --noconfirm "${basic_tools[@]}" base-devel
                ;;
            zypper)
                sudo zypper install -y "${basic_tools[@]}" gcc make
                ;;
            apk)
                sudo apk add "${basic_tools[@]}" gcc make musl-dev
                ;;
        esac
    else
        print_info "✓ 基础工具已安装"
    fi
    
    print_info "安装Redis..."
    if ! command -v redis-server &> /dev/null; then
        case "$PACKAGE_MANAGER" in
            apt-get)
                sudo apt-get install -y redis-server
                sudo systemctl enable redis-server
                sudo systemctl start redis-server
                ;;
            yum|dnf)
                sudo $PACKAGE_MANAGER install -y redis
                sudo systemctl enable redis
                sudo systemctl start redis
                ;;
            pacman)
                sudo pacman -S --noconfirm redis
                sudo systemctl enable redis
                sudo systemctl start redis
                ;;
            zypper)
                sudo zypper install -y redis
                sudo systemctl enable redis
                sudo systemctl start redis
                ;;
            apk)
                sudo apk add redis
                sudo rc-update add redis default
                sudo service redis start
                ;;
        esac
        print_success "Redis安装完成"
    else
        print_info "Redis已安装"
        # 检查Redis是否正在运行
        local redis_service="redis-server"
        local service_manager="systemctl"
        
        # 根据发行版调整服务名称和管理器
        if [[ "$PACKAGE_MANAGER" == "yum" ]] || [[ "$PACKAGE_MANAGER" == "dnf" ]] || [[ "$PACKAGE_MANAGER" == "zypper" ]]; then
            redis_service="redis"
        elif [[ "$PACKAGE_MANAGER" == "apk" ]]; then
            service_manager="service"
        fi
        
        if [[ "$service_manager" == "systemctl" ]]; then
            if ! sudo systemctl is-active --quiet "$redis_service" 2>/dev/null; then
                print_info "启动Redis服务..."
                sudo systemctl start "$redis_service"
                sleep 2
            fi
        else
            if ! sudo $service_manager "$redis_service" status 2>/dev/null | grep -q "running"; then
                print_info "启动Redis服务..."
                sudo $service_manager "$redis_service" start
                sleep 2
            fi
        fi
    fi
    
    # 验证Redis连接
    print_info "验证Redis连接..."
    if command -v redis-cli &> /dev/null; then
        local redis_requires_auth=false
        local redis_password=""
        
        # 尝试无密码连接
        if redis-cli ping 2>/dev/null | grep -q "PONG"; then
            print_success "Redis连接正常，无需密码"
        elif redis-cli ping 2>/dev/null | grep -q "NOAUTH Authentication required"; then
            print_info "Redis需要密码认证"
            redis_requires_auth=true
            # 尝试使用默认密码连接
            if redis-cli -a "" ping 2>/dev/null | grep -q "PONG"; then
                print_success "Redis使用空密码连接成功"
            else
                print_warning "Redis需要密码，但默认密码连接失败"
            fi
        else
            print_warning "Redis未响应，可能需要手动检查"
        fi
    else
        print_warning "redis-cli未找到，跳过Redis连接测试"
    fi
    
    print_info "安装浏览器环境..."
    install_browser_environment
}

install_dependencies_macos() {
    print_info "检查Homebrew..."
    if ! command -v brew &> /dev/null; then
        print_info "安装Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi
    
    print_info "安装基础工具..."
    # 检查并安装curl
    if ! command -v curl &> /dev/null; then
        brew install curl
    fi
    # 检查并安装wget
    if ! command -v wget &> /dev/null; then
        brew install wget
    fi
    # 检查并安装git
    if ! command -v git &> /dev/null; then
        brew install git
    fi
    
    print_info "安装Redis..."
    if ! command -v redis-server &> /dev/null; then
        brew install redis
        brew services start redis
        print_success "Redis安装完成"
    else
        print_info "Redis已安装"
    fi
    
    print_info "安装浏览器环境..."
    install_browser_environment
}

# 直接下载并安装Chromium的函数
download_and_install_chromium() {
    print_warning "尝试直接下载安装Chromium..."
    
    # 检测系统架构
    local arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            local chromium_arch="x64"
            ;;
        aarch64|arm64)
            local chromium_arch="arm64"
            ;;
        *)
            print_error "不支持的系统架构: $arch"
            return 1
            ;;
    esac
    
    # 设置安装目录
    local chromium_install_dir="/opt/chromium"
    local chromium_bin="$chromium_install_dir/chrome"
    
    # 如果Chromium已存在，直接返回
    if [ -f "$chromium_bin" ]; then
        print_info "Chromium已通过直接下载方式安装"
        # 将Chromium添加到系统路径
        if ! grep -q "$chromium_install_dir" /etc/profile.d/chromium.sh 2>/dev/null; then
            echo "export PATH=\$PATH:$chromium_install_dir" | sudo tee /etc/profile.d/chromium.sh > /dev/null
            sudo chmod +x /etc/profile.d/chromium.sh
        fi
        return 0
    fi
    
    print_info "检测到架构: $arch，将下载 $chromium_arch 版本的Chromium"
    
    # 创建安装目录
    sudo mkdir -p "$chromium_install_dir"
    
    # 下载Chromium
    local download_url=""
    local temp_dir=$(mktemp -d)
    local temp_file="$temp_dir/chromium.zip"
    
    # 选择合适的下载源
    if [ "$OS" = "linux" ]; then
        # 对于Linux，使用官方下载链接
        # 注意：这里使用的是Example URL，实际使用时需要替换为正确的下载链接
        # 可以考虑使用第三方镜像或自动化下载工具
        print_warning "直接下载Chromium功能正在开发中，使用临时解决方案"
        print_warning "请手动安装Chromium或Chrome浏览器后重新运行脚本"
        rm -rf "$temp_dir"
        return 1
        
        # 以下是示例代码，实际使用时需要替换为正确的下载链接
        # download_url="https://download-chromium.appspot.com/dl/Linux_x64"
    elif [ "$OS" = "darwin" ]; then
        # 对于macOS，提示用户手动安装
        print_warning "对于macOS系统，请手动安装Chromium或Chrome浏览器"
        print_warning "下载地址: https://www.chromium.org/getting-involved/download-chromium/"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # 下载Chromium
    if ! wget -q -O "$temp_file" "$download_url"; then
        print_error "无法下载Chromium: $download_url"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # 解压Chromium
    print_info "解压Chromium..."
    if ! unzip -q "$temp_file" -d "$temp_dir"; then
        print_error "无法解压Chromium"
        rm -rf "$temp_dir"
        return 1
    fi
    
    # 移动到安装目录
    sudo cp -r "$temp_dir"/*/* "$chromium_install_dir/" 2>/dev/null || sudo cp -r "$temp_dir"/* "$chromium_install_dir/"
    
    # 设置可执行权限
    sudo chmod -R +x "$chromium_install_dir"
    
    # 将Chromium添加到系统路径
    if ! grep -q "$chromium_install_dir" /etc/profile.d/chromium.sh 2>/dev/null; then
        echo "export PATH=\$PATH:$chromium_install_dir" | sudo tee /etc/profile.d/chromium.sh > /dev/null
        sudo chmod +x /etc/profile.d/chromium.sh
        # 立即生效
        source /etc/profile.d/chromium.sh 2>/dev/null || true
    fi
    
    # 创建符号链接，方便调用
    if [ ! -f "/usr/local/bin/chromium" ]; then
        sudo ln -s "$chromium_bin" /usr/local/bin/chromium
    fi
    
    # 清理临时文件
    rm -rf "$temp_dir"
    
    # 验证安装
    if command -v chromium &> /dev/null; then
        print_success "成功直接下载安装Chromium"
        return 0
    else
        print_error "直接下载安装Chromium失败"
        return 1
    fi
}

install_browser_environment() {
    print_info "检测浏览器环境..."
    
    # 检测Chrome/Chromium
    local chrome_available=false
    
    if command -v google-chrome &> /dev/null; then
        print_info "Chrome浏览器已安装: $(google-chrome --version)"
        chrome_available=true
    elif command -v chromium &> /dev/null; then
        print_info "Chromium浏览器已安装: $(chromium --version)"
        chrome_available=true
    elif command -v chromium-browser &> /dev/null; then
        print_info "Chromium浏览器已安装: $(chromium-browser --version)"
        chrome_available=true
    elif [ "$OS" = "darwin" ]; then
        # macOS额外检查应用目录
        if [ -d "/Applications/Google Chrome.app" ]; then
            print_info "Chrome浏览器已安装在/Applications目录"
            chrome_available=true
        elif [ -d "/Applications/Chromium.app" ]; then
            print_info "Chromium浏览器已安装在/Applications目录"
            chrome_available=true
        fi
    fi
    
    if [ "$chrome_available" = true ]; then
        return 0
    fi
    
    print_info "安装Chrome/Chromium浏览器..."
    
    case "$OS" in
        linux)
            case "$PACKAGE_MANAGER" in
                apt-get)
                    # Ubuntu/Debian系统，尝试安装chromium-browser
                    if sudo apt-get install -y chromium-browser &> /dev/null; then
                        print_success "已安装chromium-browser"
                    elif sudo apt-get install -y chromium &> /dev/null; then
                        print_success "已安装chromium"
                    else
                        print_warning "无法安装chromium，尝试安装google-chrome-stable"
                        if wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add - && \
                           sudo sh -c 'echo "deb [arch=amd64] https://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list' && \
                           sudo apt-get update && \
                           sudo apt-get install -y google-chrome-stable; then
                            print_success "已安装google-chrome-stable"
                        else
                            print_warning "无法通过包管理器安装浏览器，尝试直接下载安装Chromium"
                            if download_and_install_chromium; then
                                print_success "通过直接下载方式安装Chromium成功"
                            else
                                print_error "浏览器安装失败"
                                return 1
                            fi
                        fi
                    fi
                    ;;
                yum|dnf)
                    # CentOS/RHEL/Fedora/OpenCloudOS系统，尝试多种浏览器安装方案
                    browser_installed=false
                    
                    # 方案1：尝试安装chromium
                    if sudo $PACKAGE_MANAGER install -y chromium &> /dev/null; then
                        print_success "已安装chromium"
                        browser_installed=true
                    elif sudo $PACKAGE_MANAGER install -y chromium-browser &> /dev/null; then
                        # 方案2：尝试安装chromium-browser
                        print_success "已安装chromium-browser"
                        browser_installed=true
                    else
                        # 方案3：尝试安装google-chrome-stable
                        print_warning "无法安装chromium，尝试安装google-chrome-stable"
                        if wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm && \
                           sudo $PACKAGE_MANAGER install -y ./google-chrome-stable_current_x86_64.rpm; then
                            print_success "已安装google-chrome-stable"
                            rm -f ./google-chrome-stable_current_x86_64.rpm
                            browser_installed=true
                        else
                            # 清理临时文件
                            rm -f ./google-chrome-stable_current_x86_64.rpm
                            print_warning "无法通过包管理器安装浏览器，尝试直接下载安装Chromium"
                            if download_and_install_chromium; then
                                print_success "通过直接下载方式安装Chromium成功"
                                browser_installed=true
                            fi
                        fi
                    fi
                    
                    if [ "$browser_installed" = false ]; then
                        print_error "浏览器安装失败"
                        return 1
                    fi
                    ;;
                pacman)
                    # Arch Linux系统
                    if sudo pacman -S --noconfirm chromium &> /dev/null; then
                        print_success "已安装chromium"
                    else
                        print_warning "无法通过包管理器安装浏览器，尝试直接下载安装Chromium"
                        if download_and_install_chromium; then
                            print_success "通过直接下载方式安装Chromium成功"
                        else
                            print_error "浏览器安装失败"
                            return 1
                        fi
                    fi
                    ;;
                zypper)
                    # openSUSE系统
                    if sudo zypper install -y chromium &> /dev/null; then
                        print_success "已安装chromium"
                    else
                        print_warning "无法通过包管理器安装浏览器，尝试直接下载安装Chromium"
                        if download_and_install_chromium; then
                            print_success "通过直接下载方式安装Chromium成功"
                        else
                            print_error "浏览器安装失败"
                            return 1
                        fi
                    fi
                    ;;
                apk)
                    # Alpine Linux系统
                    if sudo apk add chromium &> /dev/null; then
                        print_success "已安装chromium"
                    else
                        print_warning "无法通过包管理器安装浏览器，尝试直接下载安装Chromium"
                        if download_and_install_chromium; then
                            print_success "通过直接下载方式安装Chromium成功"
                        else
                            print_error "浏览器安装失败"
                            return 1
                        fi
                    fi
                    ;;
            esac
            ;;
        darwin)
            # macOS系统
            if brew install --cask google-chrome &> /dev/null; then
                print_success "已安装google-chrome"
            elif brew install --cask chromium &> /dev/null; then
                print_success "已安装chromium"
            else
                print_warning "无法通过包管理器安装浏览器"
                print_warning "请手动安装Chromium或Chrome浏览器后重新运行脚本"
                print_warning "Chrome下载地址: https://www.google.com/chrome/"
                print_warning "Chromium下载地址: https://www.chromium.org/getting-involved/download-chromium/"
                print_error "浏览器安装失败"
                return 1
            fi
            ;;
    esac
    
    print_success "浏览器环境安装完成"
}

# ============================================================================
# 应用构建和安装
# ============================================================================

build_and_install() {
    print_step "3" "安装应用程序"
    
    # 创建安装目录
    print_info "创建安装目录..."
    sudo mkdir -p "$INSTALL_DIR"
    sudo mkdir -p "$CONFIG_DIR"
    sudo mkdir -p "$DATA_DIR"
    sudo mkdir -p "$LOG_DIR"
    
    # 设置目录权限
    sudo chmod 755 "$INSTALL_DIR"
    sudo chmod 755 "$CONFIG_DIR"
    sudo chmod 750 "$DATA_DIR"
    sudo chmod 750 "$LOG_DIR"
    
    # 从预编译包安装
    if [[ -d "./bin" && -d "./web/dist" ]]; then
        print_info "从预编译包安装..."
        install_from_prebuilt
    else
        print_error "未找到预编译包，请先运行 ./build.sh 构建应用"
        exit 1
    fi
}

install_from_prebuilt() {
    # 复制前端文件到安装目录
    print_info "复制前端文件到安装目录..."
    if [[ -d "./web/dist" ]]; then
        sudo cp -r ./web/dist "$INSTALL_DIR/web/"
        print_success "前端文件复制完成"
    else
        print_error "未找到前端构建目录: ./web/dist"
        exit 1
    fi
    
    # 替换前端API地址为当前服务器IP
    print_info "配置前端API地址..."
    local api_ip="127.0.0.1"
    if [[ "$CUSTOM_API_IP" != "" ]]; then
        api_ip="$CUSTOM_API_IP"
        print_info "使用命令行参数提供的IP: $api_ip"
    elif [[ "$(hostname)" == "localhost" ]] || [[ "$(hostname)" == "*local*" ]] || [[ "$(uname -s)" == "Darwin" ]]; then
        # 本地环境，使用本地IP
        api_ip="127.0.0.1"
        print_info "当前是本地环境，使用本地IP: $api_ip"
    else
        # 服务器环境，尝试获取公网IP
        api_ip=$(curl -s ifconfig.me || curl -s icanhazip.com || curl -s ipinfo.io/ip || echo "127.0.0.1")
        print_info "当前服务器公网IP: $api_ip"
    fi
    
    # 替换前端文件中的API地址
    local frontend_dir="$INSTALL_DIR/web"
    local api_url="http://$api_ip:9598/api/v1"
    
    # 使用更可靠的方式替换前端API地址，兼容macOS和Linux
    print_info "正在替换前端文件中的API地址为: $api_url"
    
    # 先检查前端目录是否存在
    if [[ -d "$frontend_dir" ]]; then
        # 查找所有JavaScript和HTML文件进行替换
        local files=$(sudo find "$frontend_dir" -name "*.js" -o -name "*.html" -o -name "*.css" | grep -v "node_modules" | grep -v ".git")
        
        if [[ -n "$files" ]]; then
            # 使用awk进行替换，避免sed的跨平台兼容性问题
            for file in $files; do
                sudo awk -v api_url="$api_url" '{gsub(/http:\/\/127\.0\.0\.1:9598\/api\/v1/, api_url); print}' "$file" > "$file.tmp"
                if [[ $? -eq 0 ]]; then
                    sudo mv "$file.tmp" "$file"
                else
                    sudo rm -f "$file.tmp" > /dev/null 2>&1
                fi
            done
            print_success "前端API地址配置完成"
        else
            print_warning "未找到需要替换API地址的文件"
        fi
    else
        print_error "前端目录不存在: $frontend_dir"
        exit 1
    fi
    
    # 复制后端二进制文件到安装目录
    print_info "复制后端二进制文件到安装目录..."
    
    # 检查当前目录是否存在api二进制文件
    if [[ -f "./api" ]]; then
        sudo cp "./api" "$INSTALL_DIR/api"
        sudo chmod 755 "$INSTALL_DIR/api"
        print_success "使用当前目录二进制文件"
    else
        # 检查bin目录中是否存在当前平台的二进制文件
        local os=$(uname -s | tr '[:upper:]' '[:lower:]')
        local arch=$(uname -m)
        if [[ $arch == "x86_64" ]]; then
            arch="amd64"
        elif [[ $arch == "arm64" ]]; then
            arch="arm64"
        fi
        
        local binary_path="bin/${os}-${arch}/api"
        if [[ -f "$binary_path" ]]; then
            sudo cp "$binary_path" "$INSTALL_DIR/api"
            sudo chmod 755 "$INSTALL_DIR/api"
            print_success "使用${os}-${arch}平台二进制文件"
        else
            print_error "未找到可用的后端二进制文件: $binary_path"
            print_error "请先运行 ./build.sh 编译应用"
            exit 1
        fi
    fi
    
    # 验证安装结果
    print_info "验证安装结果..."
    
    if [[ -f "$INSTALL_DIR/api" && -x "$INSTALL_DIR/api" ]]; then
        print_success "后端二进制文件安装成功"
    else
        print_error "后端二进制文件安装失败或不可执行"
        exit 1
    fi
    
    if [[ -d "$INSTALL_DIR/web" && -f "$INSTALL_DIR/web/index.html" ]]; then
        print_success "前端文件安装成功"
    else
        print_error "前端文件安装失败"
        exit 1
    fi
    
    print_success "应用安装完成"
}

download_and_install() {
    print_error "从发布包安装功能尚未实现"
    print_info "请从源码构建或下载预编译版本"
    exit 1
}

# ============================================================================
# 配置文件设置
# ============================================================================

setup_redis_config() {
    print_info "配置Redis连接..."
    
    # 检测Redis是否已安装
    local redis_installed=false
    if command -v redis-server &> /dev/null; then
        redis_installed=true
    fi
    
    # 提供默认值
    local default_host="127.0.0.1"
    local default_port="6379"
    local default_password=""
    local default_db="0"
    
    # 从现有配置文件读取当前值（如果存在）
    if [[ -f "$CONFIG_DIR/config.yml" ]]; then
        local current_host=$(grep -E "redis_url:" "$CONFIG_DIR/config.yml" | awk -F'[:@]' '{print $2}' | sed 's/"//g')
        local current_port=$(grep -E "redis_url:" "$CONFIG_DIR/config.yml" | awk -F'[:/]' '{print $3}' | sed 's/"//g')
        local current_password=$(grep -E "redis_password:" "$CONFIG_DIR/config.yml" 2>/dev/null | awk -F': ' '{print $2}' | sed 's/"//g')
        local current_db=$(grep -E "redis_db:" "$CONFIG_DIR/config.yml" 2>/dev/null | awk -F': ' '{print $2}' | sed 's/"//g')
        
        # 如果当前值存在，使用当前值作为默认值
        [[ -n "$current_host" ]] && default_host="$current_host"
        [[ -n "$current_port" ]] && default_port="$current_port"
        [[ -n "$current_password" ]] && default_password="$current_password"
        [[ -n "$current_db" ]] && default_db="$current_db"
    fi
    
    # 交互式输入Redis连接信息
    echo ""
    echo -e "${BLUE}Redis连接配置${NC}"
    echo "====================================="
    echo "按回车键使用默认值，或输入新值"
    echo "====================================="
    
    read -p "Redis主机 (默认: $default_host): " redis_host
    read -p "Redis端口 (默认: $default_port): " redis_port
    read -s -p "Redis密码 (默认: 空密码): " redis_password
    echo ""
    read -p "Redis数据库 (默认: $default_db): " redis_db
    echo "====================================="
    
    # 使用默认值如果用户未输入
    redis_host=${redis_host:-$default_host}
    redis_port=${redis_port:-$default_port}
    redis_password=${redis_password:-$default_password}
    redis_db=${redis_db:-$default_db}
    
    # 构建redis_url
    local redis_url="$redis_host:$redis_port"
    
    # 打印配置信息（隐藏密码）
    echo ""
    print_info "Redis连接配置:"
    print_info "  主机: $redis_host"
    print_info "  端口: $redis_port"
    print_info "  密码: $( [[ -n "$redis_password" ]] && echo "****" || echo "无" )"
    print_info "  数据库: $redis_db"
    
    # 返回配置信息
    echo "$redis_url:$redis_db:$redis_password"
}

setup_configuration() {
    print_step "4" "配置应用"
    
    # 复制配置文件
    if [[ -f "configs/config.example.yml" ]]; then
        print_info "生成配置文件..."
        sudo cp configs/config.example.yml "$CONFIG_DIR/config.yml"
        
        # 交互式配置Redis
        local redis_config=$(setup_redis_config)
        local redis_url=$(echo "$redis_config" | cut -d':' -f1-2)
        local redis_db=$(echo "$redis_config" | cut -d':' -f3)
        local redis_password=$(echo "$redis_config" | cut -d':' -f4-)
        
        # 修改默认配置
        print_info "优化默认配置..."
        # 兼容macOS/BSD和GNU sed的-i选项
        # 使用临时文件方法，兼容所有系统
        local temp_config=$(mktemp)
        
        # 读取原文件并替换内容，使用awk避免sed特殊字符问题
        awk -v data_dir="$DATA_DIR" -v install_dir="$INSTALL_DIR" -v redis_url="$redis_url" '{
            # 替换data_dir
            if (/^  data_dir: /) {
                print "  data_dir: " data_dir;
                next;
            }
            # 替换static_dir
            if (/^  static_dir: /) {
                print "  static_dir: " install_dir "/static";
                next;
            }
            # 替换admin_static_dir
            if (/^  admin_static_dir: /) {
                print "  admin_static_dir: " install_dir "/web/dist";
                next;
            }
            # 替换redis_url
            if (/^  redis_url: /) {
                print "  redis_url: \"" redis_url "\"";
                next;
            }
            # 其他行直接打印
            print;
        }' "$CONFIG_DIR/config.yml" > "$temp_config"
        
        # 添加或修改redis_db配置
        if grep -q "redis_db:" "$temp_config"; then
            # 使用awk替换，避免sed命令中的特殊字符问题
            awk -v new_db="$redis_db" '/redis_db:/{print "  redis_db: " new_db; next}1' "$temp_config" > "$temp_config.tmp" && mv "$temp_config.tmp" "$temp_config"
        else
            # 使用awk在redis_url行后添加redis_db配置
            awk -v new_db="$redis_db" '/redis_url:/{print; print "  redis_db: " new_db; next}1' "$temp_config" > "$temp_config.tmp" && mv "$temp_config.tmp" "$temp_config"
        fi
        
        # 添加或修改redis_password配置
        if grep -q "redis_password:" "$temp_config"; then
            # 使用awk替换，避免sed命令中的特殊字符问题
            awk -v new_pwd="$redis_password" '/redis_password:/{print "  redis_password: \"" new_pwd "\""; next}1' "$temp_config" > "$temp_config.tmp" && mv "$temp_config.tmp" "$temp_config"
        else
            # 使用awk在redis_db行后添加redis_password配置
            awk -v new_pwd="$redis_password" '/redis_db:/{print; print "  redis_password: \"" new_pwd "\""; next}1' "$temp_config" > "$temp_config.tmp" && mv "$temp_config.tmp" "$temp_config"
        fi
        
        # 使用sudo复制临时文件到目标位置
        sudo cp "$temp_config" "$CONFIG_DIR/config.yml"
        # 清理临时文件
        rm -f "$temp_config"
        
        # 设置默认站点配置
        setup_default_site
        
        print_success "配置文件生成完成"
    else
        print_warning "未找到配置文件模板，使用默认配置"
    fi
}

setup_default_site() {
    print_info "配置默认站点..."
    
    # 创建一个包含新sites配置的临时文件
    local sites_temp=$(mktemp)
    cat > "$sites_temp" << EOF
sites:
  - id: "default-site"
    name: "默认站点"
    domains:
      - "127.0.0.1"
    port: 8082
    mode: "static"
    enabled: true
    prerender:
      enabled: true
      pool_size: 2
      min_pool_size: 1
      max_pool_size: 5
      timeout: 30
      cache_ttl: 3600
      use_default_headers: true
    firewall:
      enabled: true
      action:
        default_action: "block"
        block_message: "请求被防火墙拦截"
EOF
    
    # 创建一个临时文件，用于存储修改后的配置
    local config_temp=$(mktemp)
    
    # 使用更简单的方式：将原配置文件中sites:之前的内容复制到临时文件（不包括sites:行）
    # 然后将新的sites配置追加到临时文件
    awk '/^sites:/{exit}1' "$CONFIG_DIR/config.yml" > "$config_temp"
    cat "$sites_temp" >> "$config_temp"
    
    # 将临时文件内容复制回原配置文件
    sudo cp "$config_temp" "$CONFIG_DIR/config.yml"
    
    # 删除临时文件
    rm "$sites_temp" "$config_temp"
}

# 添加安装后的健康检查功能
install_health_check() {
    print_step "4" "执行安装后的健康检查"
    
    local health_check_passed=true
    
    # 检查配置文件
    if [ -f "$CONFIG_DIR/config.yml" ]; then
        print_success "配置文件检查通过: $CONFIG_DIR/config.yml"
    else
        print_error "配置文件不存在: $CONFIG_DIR/config.yml"
        health_check_passed=false
    fi
    
    # 检查安装目录结构
    local required_files=(
        "$INSTALL_DIR/api"
        "$INSTALL_DIR/web"
        "$INSTALL_DIR/web/index.html"
    )
    
    for file in "${required_files[@]}"; do
        if [ -e "$file" ]; then
            if [ -x "$file" ]; then
                print_success "可执行文件检查通过: $file"
            else
                print_success "文件/目录检查通过: $file"
            fi
        else
            print_error "安装目录结构不完整，缺少: $file"
            health_check_passed=false
        fi
    done
    
    # 检查二进制文件权限
    if [ -f "$INSTALL_DIR/api" ]; then
        if [ -x "$INSTALL_DIR/api" ]; then
            print_success "二进制文件权限检查通过"
        else
            print_warning "二进制文件权限不足，正在添加执行权限..."
            sudo chmod +x "$INSTALL_DIR/api" > /dev/null 2>&1
            if [ $? -eq 0 ]; then
                print_success "已添加二进制文件执行权限"
            else
                print_error "无法添加二进制文件执行权限"
                health_check_passed=false
            fi
        fi
    fi
    
    # 检查Redis连接
    print_info "检查Redis连接..."
    if command -v redis-cli &> /dev/null; then
        local redis_status=$(redis-cli ping 2>/dev/null)
        if [[ "$redis_status" == "PONG" ]]; then
            print_success "Redis连接检查通过"
        else
            print_warning "Redis连接检查失败，请确保Redis服务已启动"
            print_warning "当前Redis状态: ${redis_status:-无法连接}"
            print_warning "可以使用以下命令启动Redis:"
            case "$OS" in
                linux)
                    if command -v systemctl &> /dev/null; then
                        print_warning "  sudo systemctl start redis-server"
                    else
                        print_warning "  sudo service redis start"
                    fi
                    ;;
                darwin)
                    print_warning "  brew services start redis"
                    ;;
            esac
            health_check_passed=false
        fi
    else
        print_warning "redis-cli未找到，跳过Redis连接检查"
        print_warning "请确保Redis服务已安装并正常运行"
    fi
    
    # 检查浏览器环境
    print_info "检查浏览器环境..."
    local browser_available=false
    if command -v google-chrome &> /dev/null || \
       command -v chromium &> /dev/null || \
       command -v chromium-browser &> /dev/null || \
       [[ -d "/Applications/Google Chrome.app" ]]; then
        browser_available=true
        print_success "浏览器环境检查通过"
    else
        print_warning "未检测到Chrome/Chromium浏览器"
        print_warning "请确保已安装Chrome或Chromium浏览器，否则渲染功能可能无法正常工作"
        health_check_passed=false
    fi
    
    # 检查日志目录权限
    print_info "检查日志目录权限..."
    if [ -d "$LOG_DIR" ]; then
        if [ -w "$LOG_DIR" ]; then
            print_success "日志目录权限检查通过"
        else
            print_warning "日志目录权限不足，正在修复..."
            sudo chmod 755 "$LOG_DIR" > /dev/null 2>&1
            if [ $? -eq 0 ]; then
                print_success "已修复日志目录权限"
            else
                print_error "无法修复日志目录权限"
                health_check_passed=false
            fi
        fi
    else
        print_warning "日志目录不存在，正在创建..."
        sudo mkdir -p "$LOG_DIR" > /dev/null 2>&1
        if [ $? -eq 0 ]; then
            sudo chmod 755 "$LOG_DIR" > /dev/null 2>&1
            print_success "已创建日志目录并设置权限"
        else
            print_error "无法创建日志目录"
            health_check_passed=false
        fi
    fi
    
    if [ "$health_check_passed" = true ]; then
        print_success "健康检查完成，安装结果正常"
    else
        print_warning "健康检查部分项未通过，请根据上面的提示进行修复"
        print_warning "安装已完成，但某些依赖或配置可能存在问题"
        print_warning "建议在启动服务前解决这些问题"
    fi
}

# ============================================================================
# 清理和回滚
# ============================================================================

cleanup_on_error() {
    print_error "安装过程中出现错误，正在清理..."
    
    # 清理目录
    sudo rm -rf "$INSTALL_DIR" 2>/dev/null || true
    sudo rm -rf "$CONFIG_DIR" 2>/dev/null || true
    sudo rm -rf "$DATA_DIR" 2>/dev/null || true
    sudo rm -rf "$LOG_DIR" 2>/dev/null || true
    
    print_error "安装已回滚，请检查错误信息后重试"
}

# ============================================================================
# 全局变量：API IP地址
CUSTOM_API_IP=""

# 主函数
# ============================================================================

main() {
    trap 'cleanup_on_error' ERR
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --ip|-i)
                CUSTOM_API_IP="$2"
                shift
                shift
                ;;
            --help|-h)
                print_usage
                exit 0
                ;;
            *)
                print_error "未知参数: $1"
                print_usage
                exit 1
                ;;
        esac
    done
    
    print_header
    detect_os
    check_root
    install_dependencies
    # 配置Go模块镜像加速
    print_info "配置Go模块镜像加速..."
    export GOPROXY="https://goproxy.cn,direct"
    print_info "GOPROXY设置为: $GOPROXY"
    build_and_install
    setup_configuration
    install_health_check
    
    # 获取本机IP地址，用于访问信息
    local api_ip="127.0.0.1"
    if [[ "$CUSTOM_API_IP" != "" ]]; then
        api_ip="$CUSTOM_API_IP"
    elif [[ "$(hostname)" != "localhost" && ! "$(hostname)" =~ "local" && "$(uname -s)" != "Darwin" ]]; then
        # 服务器环境，尝试获取公网IP
        api_ip=$(curl -s ifconfig.me || curl -s icanhazip.com || echo "127.0.0.1")
    fi
    
    # 输出启动命令和访问信息
    print_success "======================================="
    print_success "PrerenderShield 安装完成！"
    print_success "======================================="
    echo ""
    echo -e "${BLUE}重要信息：${NC}"
    echo -e "${BLUE}1. 管理控制台: http://$api_ip:9597${NC}"
    echo -e "${BLUE}2. API服务: http://$api_ip:9598${NC}"
    echo -e "${BLUE}3. 配置文件: $CONFIG_DIR/config.yml${NC}"
    echo -e "${BLUE}4. 日志目录: $LOG_DIR${NC}"
    echo ""
    echo -e "${BLUE}默认登录信息：${NC}"
    echo -e "${BLUE}  用户名: admin${NC}"
    echo -e "${BLUE}  密码: 123456${NC}"
    echo ""
    print_success "======================================="
    print_success "启动命令: ./start.sh start"
    print_success "重启命令: ./start.sh restart"
    print_success "停止命令: ./start.sh stop"
    print_success "======================================="
    echo ""
    echo -e "${GREEN}接下来：${NC}"
    echo -e "${GREEN}1. 执行 ./start.sh start 启动应用${NC}"
    echo -e "${GREEN}2. 打开浏览器访问 http://$api_ip:9597${NC}"
    echo -e "${GREEN}3. 使用默认账号登录${NC}"
    echo -e "${GREEN}4. 在管理界面中添加和管理您的站点${NC}"
}

print_usage() {
    echo ""
    echo -e "${BLUE}使用方法:${NC} $0 [选项]"
    echo ""
    echo -e "${BLUE}选项:${NC}"
    echo "  -i, --ip <IP地址>    指定服务器IP地址（可选，默认自动检测）"
    echo "  -h, --help           显示帮助信息"
    echo ""
    echo -e "${BLUE}示例:${NC}"
    echo "  # 自动检测IP地址"
    echo "  $0"
    echo ""
    echo "  # 手动指定IP地址"
    echo "  $0 --ip 192.168.1.100"
    echo ""
}

# 检查是否直接运行
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi