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
            sudo yum update -y
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
    case "$PACKAGE_MANAGER" in
        apt-get)
            sudo apt-get install -y curl wget git build-essential
            ;;
        yum|dnf)
            sudo $PACKAGE_MANAGER install -y curl wget git gcc make
            ;;
        pacman)
            sudo pacman -S --noconfirm curl wget git base-devel
            ;;
        zypper)
            sudo zypper install -y curl wget git gcc make
            ;;
        apk)
            sudo apk add curl wget git gcc make musl-dev
            ;;
    esac
    
    print_info "安装Go环境..."
    if ! command -v go &> /dev/null; then
        case "$PACKAGE_MANAGER" in
            apt-get)
                sudo apt-get install -y golang-go
                ;;
            yum|dnf)
                sudo $PACKAGE_MANAGER install -y golang
                ;;
            pacman)
                sudo pacman -S --noconfirm go
                ;;
            zypper)
                sudo zypper install -y go
                ;;
            apk)
                sudo apk add go
                ;;
        esac
        print_success "Go环境安装完成"
    else
        print_info "Go环境已安装: $(go version)"
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
        if redis-cli ping 2>/dev/null | grep -q "PONG"; then
            print_success "Redis连接正常"
        else
            print_warning "Redis未响应，可能需要手动检查"
        fi
    else
        print_warning "redis-cli未找到，跳过Redis连接测试"
    fi
    
    print_info "安装Node.js和npm..."
    if ! command -v npm &> /dev/null; then
        case "$PACKAGE_MANAGER" in
            apt-get)
                sudo apt-get install -y nodejs npm
                ;;
            yum|dnf)
                sudo $PACKAGE_MANAGER install -y nodejs npm
                ;;
            pacman)
                sudo pacman -S --noconfirm nodejs npm
                ;;
            zypper)
                sudo zypper install -y nodejs npm
                ;;
            apk)
                sudo apk add nodejs npm
                ;;
        esac
        print_success "Node.js和npm安装完成"
    else
        print_info "Node.js和npm已安装: $(npm --version)"
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
    brew install curl wget git
    
    print_info "安装Go环境..."
    if ! command -v go &> /dev/null; then
        brew install go
        print_success "Go环境安装完成"
    else
        print_info "Go环境已安装: $(go version)"
    fi
    
    print_info "安装Redis..."
    if ! command -v redis-server &> /dev/null; then
        brew install redis
        brew services start redis
        print_success "Redis安装完成"
    else
        print_info "Redis已安装"
    fi
    
    print_info "安装Node.js和npm..."
    if ! command -v npm &> /dev/null; then
        brew install node
        print_success "Node.js和npm安装完成"
    else
        print_info "Node.js和npm已安装: $(npm --version)"
    fi
    
    print_info "安装浏览器环境..."
    install_browser_environment
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
                    sudo apt-get install -y chromium-browser
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
                yum|dnf)
                    sudo $PACKAGE_MANAGER install -y chromium
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
                pacman)
                    sudo pacman -S --noconfirm chromium
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
                zypper)
                    sudo zypper install -y chromium
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
                apk)
                    sudo apk add chromium
                    if [ $? -ne 0 ]; then
                        error "浏览器安装失败"
                        return 1
                    fi
                    ;;
            esac
            ;;
        darwin)
            brew install --cask google-chrome
            if [ $? -ne 0 ]; then
                error "浏览器安装失败"
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
    print_step "3" "构建和安装应用"
    
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
    
    # 如果是开发环境，使用当前目录
    if [[ -f "./go.mod" ]]; then
        print_info "从源代码构建..."
        build_from_source
    else
        print_info "从发布包安装..."
        download_and_install
    fi
}

build_from_source() {
    print_info "配置Go环境..."
    export GOPROXY=https://goproxy.cn,direct
    export GO111MODULE=on
    
    print_info "安装Go依赖..."
    go mod tidy
    if [[ $? -ne 0 ]]; then
        print_error "Go依赖安装失败"
        exit 1
    fi
    
    print_info "构建后端二进制文件..."
    go build -o "$INSTALL_DIR/$APP_NAME" ./cmd/api
    if [[ $? -ne 0 ]]; then
        print_error "后端构建失败"
        exit 1
    fi
    
    print_info "构建前端..."
    cd web
    npm install
    if [[ $? -ne 0 ]]; then
        print_error "前端依赖安装失败"
        exit 1
    fi
    
    # 设置API地址
    export VITE_API_BASE_URL="http://localhost:9598/api/v1"
    npm run build
    if [[ $? -ne 0 ]]; then
        print_error "前端构建失败"
        exit 1
    fi
    
    cd ..
    
    # 复制前端文件
    sudo cp -r web/dist "$INSTALL_DIR/web/"
    
    print_success "应用构建完成"
}

download_and_install() {
    print_error "从发布包安装功能尚未实现"
    print_info "请从源码构建或下载预编译版本"
    exit 1
}

# ============================================================================
# 配置文件设置
# ============================================================================

setup_configuration() {
    print_step "4" "配置应用"
    
    # 复制配置文件
    if [[ -f "configs/config.example.yml" ]]; then
        print_info "生成配置文件..."
        sudo cp configs/config.example.yml "$CONFIG_DIR/config.yml"
        
        # 修改默认配置
        print_info "优化默认配置..."
        sudo sed -i "s|data_dir: ./data|data_dir: $DATA_DIR|" "$CONFIG_DIR/config.yml"
        sudo sed -i "s|static_dir: ./static|static_dir: $INSTALL_DIR/static|" "$CONFIG_DIR/config.yml"
        sudo sed -i "s|admin_static_dir: ./web/dist|admin_static_dir: $INSTALL_DIR/web/dist|" "$CONFIG_DIR/config.yml"
        sudo sed -i "s|redis_url: \"localhost:6379\"|redis_url: \"127.0.0.1:6379\"|" "$CONFIG_DIR/config.yml"
        
        # 设置默认站点配置
        setup_default_site
        
        print_success "配置文件生成完成"
    else
        print_warning "未找到配置文件模板，使用默认配置"
    fi
}

setup_default_site() {
    print_info "配置默认站点..."
    
    # 创建默认站点配置
    local default_site_config=$(cat <<EOF
  - id: "default-site"
    name: "默认站点"
    domains:
      - "127.0.0.1"
    port: 8081
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
)
    
    # 替换sites部分
    sudo sed -i "/^sites:/,/^[^ ]/c\sites:\n$default_site_config" "$CONFIG_DIR/config.yml"
}

# ============================================================================
# 系统服务配置
# ============================================================================

setup_system_service() {
    print_step "5" "配置系统服务"
    
    case "$OS" in
        linux)
            # 检查系统是否使用systemd
            if command -v systemctl &> /dev/null && [[ "$(ps -p 1 -o comm=)" == "systemd" ]]; then
                setup_systemd_service
            else
                # 非systemd系统（如Alpine Linux使用OpenRC）
                setup_openrc_service
            fi
            ;;
        darwin)
            setup_launchd_service
            ;;
    esac
}

setup_systemd_service() {
    print_info "创建systemd服务..."
    
    local service_file=$(cat <<EOF
[Unit]
Description=PrerenderShield - Web Application Firewall with Prerendering
After=network.target redis-server.service
Requires=redis-server.service

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$APP_NAME --config $CONFIG_DIR/config.yml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=append:$LOG_DIR/app.log
StandardError=append:$LOG_DIR/error.log
Environment="GOPROXY=https://goproxy.cn,direct"
Environment="GO111MODULE=on"

[Install]
WantedBy=multi-user.target
EOF
)
    
    echo "$service_file" | sudo tee "/etc/systemd/system/$SYSTEMD_SERVICE" > /dev/null
    
    print_info "重新加载systemd配置..."
    sudo systemctl daemon-reload
    
    print_info "启用服务自启动..."
    sudo systemctl enable "$SYSTEMD_SERVICE"
    
    print_success "systemd服务配置完成"
}

setup_launchd_service() {
    print_info "创建launchd服务..."
    
    local plist_file=$(cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.prerendershield.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/$APP_NAME</string>
        <string>--config</string>
        <string>$CONFIG_DIR/config.yml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$LOG_DIR/app.log</string>
    <key>StandardErrorPath</key>
    <string>$LOG_DIR/error.log</string>
    <key>WorkingDirectory</key>
    <string>$INSTALL_DIR</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>GOPROXY</key>
        <string>https://goproxy.cn,direct</string>
        <key>GO111MODULE</key>
        <string>on</string>
    </dict>
</dict>
</plist>
EOF
)
    
    echo "$plist_file" | sudo tee "/Library/LaunchDaemons/com.prerendershield.app.plist" > /dev/null
    
    print_info "加载launchd服务..."
    sudo launchctl load "/Library/LaunchDaemons/com.prerendershield.app.plist"
    
    print_success "launchd服务配置完成"
}

setup_openrc_service() {
    print_info "创建OpenRC服务..."
    
    local service_file="/etc/init.d/${APP_NAME}"
    
    # 创建OpenRC服务脚本
    cat << EOF | sudo tee "$service_file" > /dev/null
#!/sbin/openrc-run

name="${APP_NAME}"
description="PrerenderShield - Web Application Firewall with Prerendering"

command="$INSTALL_DIR/$APP_NAME"
command_args="--config $CONFIG_DIR/config.yml"
command_user="root"

pidfile="/run/${APP_NAME}.pid"
start_stop_daemon_args="--background --make-pidfile --pidfile $pidfile"

# 依赖服务
# 如果需要其他依赖，可以添加到这里
depend() {
    need net localmount
    after firewall redis
}

# 启动前准备
start_pre() {
    # 确保配置文件存在
    if [ ! -f "$CONFIG_DIR/config.yml" ]; then
        eerror "配置文件不存在: $CONFIG_DIR/config.yml"
        return 1
    fi
    
    # 确保目录权限正确
    chown -R root:root "$INSTALL_DIR"
    chown -R root:root "$CONFIG_DIR"
    chown -R root:root "$DATA_DIR"
    chown -R root:root "$LOG_DIR"
    
    return 0
}
EOF
    
    # 赋予执行权限
    sudo chmod +x "$service_file"
    
    # 添加到默认运行级别
    sudo rc-update add "${APP_NAME}" default
    
    print_success "OpenRC服务配置完成"
}

# ============================================================================
# 完成和启动
# ============================================================================

start_application() {
    print_step "6" "启动应用"
    
    case "$OS" in
        linux) 
            # 检查系统是否使用systemd
            if command -v systemctl &> /dev/null && [[ "$(ps -p 1 -o comm=)" == "systemd" ]]; then
                print_info "启动systemd服务..."
                sudo systemctl start "$SYSTEMD_SERVICE"
                sudo systemctl status "$SYSTEMD_SERVICE" --no-pager
            else
                # 非systemd系统（如Alpine Linux使用OpenRC）
                print_info "启动OpenRC服务..."
                sudo /etc/init.d/${APP_NAME} start
                sudo /etc/init.d/${APP_NAME} status
            fi
            ;;
        darwin)
            print_info "启动服务..."
            sudo launchctl start com.prerendershield.app
            ;;
    esac
    
    print_success "应用启动完成"
}

print_summary() {
    print_step "7" "安装完成"
    
    echo -e "${GREEN}"
    echo "===================================================================="
    echo "PrerenderShield 安装完成！"
    echo "===================================================================="
    echo ""
    echo "重要信息："
    echo "1. 管理控制台: http://localhost:9597"
    echo "2. API服务: http://localhost:9598"
    echo "3. 默认站点: http://127.0.0.1:8081"
    echo "4. 配置文件: $CONFIG_DIR/config.yml"
    echo "5. 日志目录: $LOG_DIR"
    echo ""
    echo "默认登录信息："
    echo "  用户名: admin"
    echo "  密码: 123456"
    echo ""
    echo "管理命令："
    case "$OS" in
        linux)
            # 检查系统是否使用systemd
            if command -v systemctl &> /dev/null && [[ "$(ps -p 1 -o comm=)" == "systemd" ]]; then
                echo "  启动: sudo systemctl start $SYSTEMD_SERVICE"
                echo "  停止: sudo systemctl stop $SYSTEMD_SERVICE"
                echo "  重启: sudo systemctl restart $SYSTEMD_SERVICE"
                echo "  状态: sudo systemctl status $SYSTEMD_SERVICE"
                echo "  日志: sudo journalctl -u $SYSTEMD_SERVICE -f"
            else
                # 非systemd系统（如Alpine Linux使用OpenRC）
                echo "  启动: sudo /etc/init.d/${APP_NAME} start"
                echo "  停止: sudo /etc/init.d/${APP_NAME} stop"
                echo "  重启: sudo /etc/init.d/${APP_NAME} restart"
                echo "  状态: sudo /etc/init.d/${APP_NAME} status"
                echo "  日志: tail -f $LOG_DIR/app.log"
            fi
            ;;
        darwin)
            echo "  启动: sudo launchctl start com.prerendershield.app"
            echo "  停止: sudo launchctl stop com.prerendershield.app"
            echo "  日志: tail -f $LOG_DIR/app.log"
            ;;
    esac
    echo ""
    echo "接下来："
    echo "1. 打开浏览器访问 http://localhost:9597"
    echo "2. 使用默认账号登录"
    echo "3. 在管理界面中添加和管理您的站点"
    echo "===================================================================="
    echo -e "${NC}"
}

# ============================================================================
# 清理和回滚
# ============================================================================

cleanup_on_error() {
    print_error "安装过程中出现错误，正在清理..."
    
    # 停止服务
    case "$OS" in
        linux)
            # 检查系统是否使用systemd
            if command -v systemctl &> /dev/null && [[ "$(ps -p 1 -o comm=)" == "systemd" ]]; then
                sudo systemctl stop "$SYSTEMD_SERVICE" 2>/dev/null || true
                sudo systemctl disable "$SYSTEMD_SERVICE" 2>/dev/null || true
                sudo rm -f "/etc/systemd/system/$SYSTEMD_SERVICE"
                sudo systemctl daemon-reload
            else
                # 非systemd系统（如Alpine Linux使用OpenRC）
                sudo /etc/init.d/${APP_NAME} stop 2>/dev/null || true
                sudo rc-update del ${APP_NAME} default 2>/dev/null || true
                sudo rm -f "/etc/init.d/${APP_NAME}"
            fi
            ;;
        darwin)
            sudo launchctl stop com.prerendershield.app 2>/dev/null || true
            sudo launchctl unload "/Library/LaunchDaemons/com.prerendershield.app.plist" 2>/dev/null || true
            sudo rm -f "/Library/LaunchDaemons/com.prerendershield.app.plist"
            ;;
    esac
    
    # 清理目录
    sudo rm -rf "$INSTALL_DIR" 2>/dev/null || true
    sudo rm -rf "$CONFIG_DIR" 2>/dev/null || true
    sudo rm -rf "$DATA_DIR" 2>/dev/null || true
    sudo rm -rf "$LOG_DIR" 2>/dev/null || true
    
    print_error "安装已回滚，请检查错误信息后重试"
}

# ============================================================================
# 主函数
# ============================================================================

main() {
    trap 'cleanup_on_error' ERR
    
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
    setup_system_service
    start_application
    print_summary
}

# 检查是否直接运行
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi