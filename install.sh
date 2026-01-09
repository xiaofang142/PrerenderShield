#!/bin/bash

# PrerenderShield 服务端安装脚本

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

# 全局变量
BIN_DIR="bin"
CONFIG_DIR="$BIN_DIR/config"
DATA_DIR="$BIN_DIR/data"
LOG_DIR="$BIN_DIR/logs"
GOOGLE_DIR="$BIN_DIR/google"
BINARY_PATH="$BIN_DIR/api"

# 检查root权限
check_root() {
    # macOS (Darwin) 不应该以root身份运行，因为Homebrew禁止root
    if [[ "$(uname -s)" == "Darwin" ]]; then
        if [[ $EUID -eq 0 ]]; then
            print_error "在macOS上不应以root身份运行此脚本，Homebrew禁止root操作"
            print_error "请以普通用户身份运行，脚本会在需要时请求sudo权限"
            exit 1
        fi
    else
        # Linux系统需要root权限进行系统级安装
        if [[ $EUID -ne 0 ]]; then
            print_error "在Linux上请以root用户运行此脚本"
            exit 1
        fi
    fi
}

# 检测操作系统
detect_os() {
    OS_TYPE=$(uname -s)
    ARCH=$(uname -m)
    
    case "$OS_TYPE" in
        Linux) 
            OS="linux"
            # 检测Linux发行版和包管理器
            if [[ -f /etc/os-release ]]; then
                . /etc/os-release
                DISTRO=$ID
                
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
            else
                print_error "无法检测Linux发行版"
                exit 1
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

# 安装Redis
install_redis() {
    print_info "检测Redis..."
    
    if command -v redis-server &> /dev/null; then
        print_success "Redis已安装: $(redis-server --version)"
        return 0
    fi
    
    print_info "Redis未安装，开始安装..."
    
    case "$OS" in
        linux)
            case "$PACKAGE_MANAGER" in
                apt-get)
                    sudo $PACKAGE_MANAGER update -y
                    sudo $PACKAGE_MANAGER install -y redis-server
                    sudo systemctl enable redis-server
                    sudo systemctl start redis-server
                    ;;
                yum|dnf)
                    sudo $PACKAGE_MANAGER install -y redis
                    sudo systemctl enable redis
                    sudo systemctl start redis
                    ;;
                pacman)
                    sudo pacman -Sy --noconfirm redis
                    sudo systemctl enable redis
                    sudo systemctl start redis
                    ;;
                zypper)
                    sudo zypper refresh -y
                    sudo zypper install -y redis
                    sudo systemctl enable redis
                    sudo systemctl start redis
                    ;;
                apk)
                    sudo apk update
                    sudo apk add redis
                    sudo rc-update add redis default
                    sudo service redis start
                    ;;
            esac
            ;;
        darwin)
            # 检查Homebrew
            if ! command -v brew &> /dev/null; then
                print_info "安装Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi
            brew install redis
            brew services start redis
            ;;
    esac
    
    if command -v redis-server &> /dev/null; then
        print_success "Redis安装完成"
        return 0
    else
        print_error "Redis安装失败"
        exit 1
    fi
}

# 配置Redis信息
configure_redis() {
    print_info "配置Redis连接信息..."
    
    # 创建配置目录
    mkdir -p "$CONFIG_DIR"
    
    # 交互式输入Redis连接信息
    echo -e "\n${BLUE}[i] 请配置Redis连接信息:${NC}"
    
    # 输入主机地址
    local redis_host=""
    while [[ -z "$redis_host" ]]; do
        read -p "  主机地址: " redis_host
        if [[ -z "$redis_host" ]]; then
            print_error "IP地址不能为空，请重新输入"
        fi
    done
    
    # 输入端口号
    local redis_port=""
    while [[ -z "$redis_port" ]]; do
        read -p "  端口号: " redis_port
        if [[ -z "$redis_port" ]]; then
            print_error "端口号不能为空，请重新输入"
        fi
    done
    
    # 输入密码（可以为空）
    read -s -p "  密码 (无密码请直接回车): " redis_password
    echo
    
    # 输入数据库编号
    local redis_db=""
    while [[ -z "$redis_db" ]]; do
        read -p "  数据库编号: " redis_db
        if [[ -z "$redis_db" ]]; then
            print_error "数据库编号不能为空，请重新输入"
        fi
    done
    
    # 生成配置文件
    local config_file="$CONFIG_DIR/config.yml"
    
    # 检查是否存在示例配置文件
    if [[ -f "configs/config.example.yml" ]]; then
        # 复制示例配置文件
        cp configs/config.example.yml "$config_file"
        
        # 修改配置文件
        print_info "更新配置文件..."
        
        # 使用awk替换配置
        awk -v redis_host="$redis_host" -v redis_port="$redis_port" -v redis_db="$redis_db" -v redis_password="$redis_password" -v data_dir="$DATA_DIR" -v logs_dir="$LOG_DIR" -v google_dir="$GOOGLE_DIR" '{ 
            if (/^  redis_url:/) {
                print "  redis_url: \""redis_host":"redis_port"\""
            } else if (/^  redis_db:/) {
                print "  redis_db: "redis_db
            } else if (/^  redis_password:/) {
                print "  redis_password: \""redis_password"\""
            } else if (/^  data_dir:/) {
                print "  data_dir: "data_dir
            } else if (/^  logs_dir:/) {
                print "  logs_dir: "logs_dir
            } else if (/^    chrome_path:/) {
                print "    chrome_path: \""google_dir"/chrome\""
            } else {
                print
            }
        }' "$config_file" > "$config_file.tmp" && mv "$config_file.tmp" "$config_file"
    else
        # 创建默认配置文件
        print_info "创建默认配置文件..."
        cat > "$config_file" << EOF
server:
  address: "0.0.0.0"
  port: 9597
  api_port: 9598
  mode: "release"

redis:
  redis_url: "$redis_host:$redis_port"
  redis_db: $redis_db
  redis_password: "$redis_password"

storage:
  data_dir: "$DATA_DIR"
  logs_dir: "$LOG_DIR"

browser:
  chrome_path: "$GOOGLE_DIR/chrome"
  user_data_dir: "$BIN_DIR/chrome-user-data"
  args:
    - "--no-sandbox"
    - "--headless"
    - "--disable-gpu"
    - "--disable-dev-shm-usage"
    - "--remote-debugging-port=9222"
EOF
    fi
    
    print_success "Redis配置完成，配置文件: $config_file"
}

# 安装谷歌无头浏览器
install_google_chrome() {
    print_info "检测谷歌无头浏览器..."
    
    # 检查是否已安装
    local chrome_available=false
    local chrome_path=""
    
    if command -v google-chrome &> /dev/null; then
        chrome_path=$(which google-chrome)
        chrome_available=true
    elif command -v chromium &> /dev/null; then
        chrome_path=$(which chromium)
        chrome_available=true
    elif command -v chromium-browser &> /dev/null; then
        chrome_path=$(which chromium-browser)
        chrome_available=true
    elif [ "$OS" = "darwin" ] && [ -d "/Applications/Google Chrome.app" ]; then
        chrome_path="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
        chrome_available=true
    fi
    
    if [ "$chrome_available" = true ]; then
        print_success "Chrome/Chromium已安装: $($chrome_path --version)"
        
        # 创建安装目录并复制到指定位置
        mkdir -p "$GOOGLE_DIR"
        if [ "$OS" = "linux" ]; then
            sudo cp "$chrome_path" "$GOOGLE_DIR/chrome"
            sudo chmod +x "$GOOGLE_DIR/chrome"
        else
            cp "$chrome_path" "$GOOGLE_DIR/chrome" 2>/dev/null || ln -s "$chrome_path" "$GOOGLE_DIR/chrome"
            chmod +x "$GOOGLE_DIR/chrome"
        fi
        print_success "谷歌无头浏览器已链接到: $GOOGLE_DIR/chrome"
        return 0
    fi
    
    print_info "谷歌无头浏览器未安装，开始安装..."
    
    # 创建安装目录
    mkdir -p "$GOOGLE_DIR"
    
    case "$OS" in
        linux)
            case "$PACKAGE_MANAGER" in
                apt-get)
                    # 安装依赖
                    sudo $PACKAGE_MANAGER update -y
                    sudo $PACKAGE_MANAGER install -y wget gnupg2
                    
                    # 添加Google Chrome源
                    wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
                    sudo sh -c 'echo "deb [arch=amd64] https://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
                    sudo $PACKAGE_MANAGER update -y
                    
                    # 安装Google Chrome
                    sudo $PACKAGE_MANAGER install -y google-chrome-stable
                    
                    # 复制到指定目录
                    sudo cp $(which google-chrome) "$GOOGLE_DIR/chrome"
                    sudo chmod +x "$GOOGLE_DIR/chrome"
                    ;;
                yum|dnf)
                    # 下载并安装Google Chrome
                    wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm
                    sudo $PACKAGE_MANAGER install -y ./google-chrome-stable_current_x86_64.rpm
                    rm -f ./google-chrome-stable_current_x86_64.rpm
                    
                    # 复制到指定目录
                    cp $(which google-chrome) "$GOOGLE_DIR/chrome"
                    chmod +x "$GOOGLE_DIR/chrome"
                    ;;
                pacman)
                    sudo pacman -Sy --noconfirm chromium
                    cp $(which chromium) "$GOOGLE_DIR/chrome"
                    chmod +x "$GOOGLE_DIR/chrome"
                    ;;
                zypper)
                    sudo zypper refresh -y
                    sudo zypper install -y chromium
                    cp $(which chromium) "$GOOGLE_DIR/chrome"
                    chmod +x "$GOOGLE_DIR/chrome"
                    ;;
                apk)
                    sudo apk update
                    sudo apk add chromium
                    cp $(which chromium) "$GOOGLE_DIR/chrome"
                    chmod +x "$GOOGLE_DIR/chrome"
                    ;;
                *)
                    print_error "暂不支持此发行版的自动安装，请手动安装Google Chrome"
                    exit 1
                    ;;
            esac
            ;;
        darwin)
            # 在macOS上，使用brew安装
            if ! command -v brew &> /dev/null; then
                print_info "安装Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi
            brew install --cask google-chrome
            
            # 复制到指定目录
            cp /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome "$GOOGLE_DIR/chrome" 2>/dev/null || ln -s /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome "$GOOGLE_DIR/chrome"
            chmod +x "$GOOGLE_DIR/chrome"
            ;;
    esac
    
    if [ -f "$GOOGLE_DIR/chrome" ]; then
        print_success "谷歌无头浏览器安装完成，路径: $GOOGLE_DIR/chrome"
        return 0
    else
        print_error "谷歌无头浏览器安装失败"
        exit 1
    fi
}

# 创建必要的目录结构
create_directories() {
    print_info "创建必要的目录结构..."
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$BIN_DIR/certs"
    mkdir -p "$BIN_DIR/static"
    mkdir -p "$GOOGLE_DIR"
    
    print_success "目录结构创建完成"
}

# 主函数
main() {
    print_success "========================================"
    print_success "PrerenderShield 服务端安装脚本"
    print_success "========================================"
    
    # 检查root权限
    check_root
    
    # 检测操作系统
    detect_os
    
    # 创建必要的目录结构
    create_directories
    
    # 安装Redis
    install_redis
    
    # 配置Redis信息
    configure_redis
    
    # 安装谷歌无头浏览器
    install_google_chrome
    
    # 验证安装结果
    print_info "验证安装结果..."
    
    # 检查二进制文件
    if [ -f "$BINARY_PATH" ] && [ -x "$BINARY_PATH" ]; then
        print_success "二进制文件验证成功: $BINARY_PATH"
    else
        print_error "二进制文件不存在或不可执行: $BINARY_PATH"
        print_error "请先运行 ./build.sh 构建应用"
        exit 1
    fi
    
    # 检查配置文件
    if [ -f "$CONFIG_DIR/config.yml" ]; then
        print_success "配置文件验证成功: $CONFIG_DIR/config.yml"
    else
        print_error "配置文件不存在: $CONFIG_DIR/config.yml"
        exit 1
    fi
    
    # 检查谷歌浏览器
    if [ -f "$GOOGLE_DIR/chrome" ] && [ -x "$GOOGLE_DIR/chrome" ]; then
        print_success "谷歌无头浏览器验证成功: $GOOGLE_DIR/chrome"
    else
        print_error "谷歌无头浏览器不存在或不可执行: $GOOGLE_DIR/chrome"
        exit 1
    fi
    
    # 检查前端文件
    if [ -d "$BIN_DIR/web" ] && [ -f "$BIN_DIR/web/index.html" ]; then
        print_success "前端文件验证成功: $BIN_DIR/web"
    else
        print_error "前端文件不存在: $BIN_DIR/web"
        print_error "请先运行 ./build.sh 构建应用"
        exit 1
    fi
    
    print_success "========================================"
    print_success "PrerenderShield 安装完成！"
    print_success "========================================"
    print_success "二进制文件: $BINARY_PATH"
    print_success "配置文件: $CONFIG_DIR/config.yml"
    print_success "数据目录: $DATA_DIR"
    print_success "日志目录: $LOG_DIR"
    print_success "谷歌浏览器: $GOOGLE_DIR/chrome"
    print_success "前端文件: $BIN_DIR/web"
    print_success ""
    print_success "========================================"
    print_success "使用以下命令管理服务:"
    print_success "启动服务: $BINARY_PATH start"
    print_success "重启服务: $BINARY_PATH restart"
    print_success "停止服务: $BINARY_PATH stop"
    print_success "========================================"
    echo ""
    echo -e "${GREEN}接下来：${NC}"
    echo -e "${GREEN}1. 执行 $BINARY_PATH start 启动应用${NC}"
    echo -e "${GREEN}2. 访问管理控制台: http://your-server-ip:9597${NC}"
    echo -e "${GREEN}3. API服务地址: http://your-server-ip:9598${NC}"
}

# 检查是否直接运行
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
