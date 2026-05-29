#!/usr/bin/env bash
set -euo pipefail

echo "安装 Dev Connect (Go)..."

if ! command -v go >/dev/null 2>&1; then
  echo "错误: go 未安装"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "运行测试..."
go test ./...

echo "安装 dev 命令..."
go install ./cmd/dev

echo ""
echo "安装完成。"
echo "使用方法："
echo "  dev --help"
echo "  dev config show"
echo "  dev config add sgdev <HOSTNAME_OR_IP> --default"
