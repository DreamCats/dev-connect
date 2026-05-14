#!/bin/bash
# Dev Connect 安装脚本

set -e

echo "安装 Dev Connect..."

# 检查 uv 是否安装
if ! command -v uv &> /dev/null; then
    echo "错误: uv 未安装"
    echo "请先安装 uv: https://docs.astral.sh/uv/getting-started/installation/"
    exit 1
fi

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# 进入项目目录
cd "$SCRIPT_DIR"

# 清理旧构建
echo "清理旧构建..."
rm -rf dist build

# 构建 wheel
echo "构建 wheel..."
uv build --wheel

# 卸载旧版本
echo "卸载旧版本..."
uv tool uninstall dev-connect 2>/dev/null || true

# 全局安装
echo "全局安装 dev 命令..."
uv tool install dist/*.whl

echo ""
echo "安装完成！"
echo ""
echo "使用方法："
echo "  dev --help          # 查看帮助"
echo "  dev config show     # 查看配置"
echo "  dev config add sgdev 10.251.233.15 --default  # 添加主机"
echo ""
echo "示例："
echo "  dev ls ~/projects   # 列目录"
echo "  dev cat ~/.bashrc   # 看文件"
echo "  dev exec 'uname -a' # 执行命令"
