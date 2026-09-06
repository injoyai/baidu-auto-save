#!/usr/bin/env bash
# baidu-auto-save 镜像构建与推送脚本
# 用法:
#   ./docker-build.sh                                  # 构建并推送 latest
#   ./docker-build.sh v1.0.0                           # 构建并推送 latest + v1.0.0
#   ./docker-build.sh v1.0.0 --no-push                 # 只构建不推送
#   IMAGE=registry.example.com/foo/bar ./docker-build.sh   # 自定义镜像仓库地址
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="${IMAGE:-injoyai/baidu-auto-save}"
VERSION="${1:-latest}"
PUSH="yes"
if [[ "${2:-}" == "--no-push" || "${1:-}" == "--no-push" ]]; then
  PUSH="no"
  if [[ "${1:-}" == "--no-push" ]]; then VERSION="latest"; fi
fi

TAGS=("$IMAGE:latest")
if [[ "$VERSION" != "latest" ]]; then
  TAGS+=("$IMAGE:$VERSION")
fi

BUILD_ARGS=()
for tag in "${TAGS[@]}"; do
  BUILD_ARGS+=(-t "$tag")
done

echo "==> 构建镜像: ${TAGS[*]}"
docker build "${BUILD_ARGS[@]}" .

if [[ "$PUSH" == "no" ]]; then
  echo "==> 跳过推送 (--no-push)"
  exit 0
fi

echo "==> 推送镜像: ${TAGS[*]}"
for tag in "${TAGS[@]}"; do
  docker push "$tag"
done

echo "==> 完成 ✓  运行: docker run -d -p 8080:8080 ${TAGS[0]}"
