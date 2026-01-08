#!/bin/bash

# 创建测试目录和配置文件
mkdir -p test_sed
cp configs/config.example.yml test_sed/config.yml

# 设置测试变量
INSTALL_DIR=/tmp/prerender-shield
DATA_DIR=/tmp/prerender-shield/data
CONFIG_DIR=./test_sed

# 模拟setup_configuration函数中的sed命令
echo "测试sed命令..."

# 原始命令（会失败）
echo "\n1. 测试原始命令（无-i后缀）:"
sed -i "s|data_dir: ./data|data_dir: $DATA_DIR|" "$CONFIG_DIR/config.yml" 2>&1

echo "\n2. 测试修复后的命令（带-i ''后缀）:"
# 恢复原始配置
cp configs/config.example.yml test_sed/config.yml
# 测试修复后的命令
sed -i '' "s|data_dir: ./data|data_dir: $DATA_DIR|" "$CONFIG_DIR/config.yml"

# 验证修改是否成功
echo "\n3. 验证修改结果:"
grep "data_dir:" "$CONFIG_DIR/config.yml"

echo "\n4. 测试所有修复后的sed命令:"
# 恢复原始配置
cp configs/config.example.yml test_sed/config.yml
# 运行所有修复后的sed命令
sed -i '' "s|data_dir: ./data|data_dir: $DATA_DIR|" "$CONFIG_DIR/config.yml"
sed -i '' "s|static_dir: ./static|static_dir: $INSTALL_DIR/static|" "$CONFIG_DIR/config.yml"
sed -i '' "s|admin_static_dir: ./web/dist|admin_static_dir: $INSTALL_DIR/web/dist|" "$CONFIG_DIR/config.yml"
sed -i '' "s|redis_url: \"localhost:6379\"|redis_url: \"127.0.0.1:6379\"|" "$CONFIG_DIR/config.yml"

# 显示修改后的配置
echo "\n修改后的配置文件片段:"
cat "$CONFIG_DIR/config.yml" | grep -A 10 "server:" | grep -B 10 "sites:" | head -20

# 清理测试文件
rm -rf test_sed

echo "\n测试完成！"
