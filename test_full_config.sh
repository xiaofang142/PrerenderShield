#!/bin/bash

# 测试完整的配置生成功能

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

# 创建测试目录
CONFIG_DIR="./test_full_config"
INSTALL_DIR="./test_install"
DATA_DIR="./test_data"

mkdir -p "$CONFIG_DIR" "$INSTALL_DIR" "$DATA_DIR"

# 复制配置文件模板
cp configs/config.example.yml "$CONFIG_DIR/config.yml"

print_info "测试完整的配置生成功能..."

# 模拟用户输入的Redis配置
redis_url="127.0.0.1:6379"
redis_db="1"
redis_password="P@ssw0rd!" # 包含特殊字符的密码

# 模拟setup_configuration函数中的逻辑
print_info "生成配置文件..."
cp configs/config.example.yml "$CONFIG_DIR/config.yml"

# 修改默认配置
print_info "优化默认配置..."
temp_config=$(mktemp)

# 使用修复后的awk命令替换所有sed命令
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

print_success "配置文件生成完成"
print_info "配置文件内容:"
cat "$temp_config"

# 清理临时文件
rm -f "$temp_config"
rm -rf "$CONFIG_DIR" "$INSTALL_DIR" "$DATA_DIR"

print_success "测试完成！"
