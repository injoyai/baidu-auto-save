#!/usr/bin/env bash
# baidu-auto-save 本地运行脚本：构建前端 -> 编译后端 -> 启动服务
set -euo pipefail
cd "$(dirname "$0")"

# echo "==> 构建前端 (web/dist)"
# (cd web && npm run build)

echo "==> 运行后端"
go run ./
