#!/usr/bin/env bash
# baidu-auto-save 镜像构建与推送脚本：构建 latest 并推送到 Docker Hub
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="${IMAGE:-injoyai/baidu-auto-save}"

echo "==> 构建镜像: $IMAGE:latest"
docker build -t "$IMAGE:latest" .

echo "==> 推送镜像: $IMAGE:latest"
docker push "$IMAGE:latest"

echo "==> 完成 ✓  运行: docker run -d -p 8080:8080 $IMAGE:latest"
