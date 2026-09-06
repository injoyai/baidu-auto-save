# 阶段1: Node 构建 SPA
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .
RUN npm run build

# 阶段2: Go 构建（embed web/dist，纯静态免 CGO）
FROM golang:1.25.5 AS build
WORKDIR /src
# 先复制依赖文件利用缓存
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 用阶段1产物覆盖本地 dist 占位符
COPY --from=web /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /baidu-auto-save .

# 阶段3: 运行镜像（alpine：需要 tzdata 支持 TZ 环境变量）
FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates
COPY --from=build /baidu-auto-save /app/baidu-auto-save
ENV TZ=Asia/Shanghai
# 数据目录固定指向 /data（与 VOLUME、compose 及文档挂载约定一致），
# 否则 config 模板默认 ./data 会落在 /app/data 导致重建容器丢库
ENV DATA_DIR=/data
WORKDIR /app
VOLUME ["/data", "/app/config"]
EXPOSE 8080
ENTRYPOINT ["/app/baidu-auto-save"]
