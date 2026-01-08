#!/bin/bash

# 导入install.sh中的函数
. ./install.sh

# 导出必要的变量
export INSTALL_DIR=/tmp/prerender-shield
export DATA_DIR=/tmp/prerender-shield/data
export CONFIG_DIR=./test_config

# 调用setup_default_site函数
setup_default_site
