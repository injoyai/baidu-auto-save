#!/usr/bin/env bash
# baidu-auto-save 镜像构建与推送脚本：构建 latest 并推送到 Docker Hub
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="${IMAGE:-injoyai/baidu-auto-save}"

# 代理配置：构建时注入（拉取基础镜像由 Docker 守护进程自身的代理决定，此处作用于容器内 npm/go 网络请求）
# 留空则不使用代理；也可用环境变量覆盖: PROXY=http://192.168.1.100:7890 ./docker-build.sh
PROXY="${PROXY:-http://127.0.0.1:7890}"
BUILD_PROXY_ARGS=()
if [[ -n "$PROXY" ]]; then
  # Windows/Mac Docker Desktop 下 host.docker.internal 指向宿主机，使容器内可达本机代理
  PROXY_HOST="${PROXY/http:\/\/127.0.0.1/http:\/\/host.docker.internal}"
  BUILD_PROXY_ARGS+=(--build-arg "HTTP_PROXY=$PROXY_HOST" --build-arg "HTTPS_PROXY=$PROXY_HOST" --build-arg "NO_PROXY=localhost,127.0.0.1")
  echo "==> 使用代理: $PROXY_HOST"
fi

echo "==> 构建镜像: $IMAGE:latest"
docker build "${BUILD_PROXY_ARGS[@]}" -t "$IMAGE:latest" .

echo "==> 推送镜像: $IMAGE:latest"
docker push "$IMAGE:latest"

echo "==> 完成 ✓  运行: docker run -d -p 8080:8080 $IMAGE:latest"
