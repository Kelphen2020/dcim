# ---- 构建阶段 ----
# REGISTRY 默认走国内镜像加速(海外部署可: docker build --build-arg REGISTRY=docker.io/library ...)
ARG REGISTRY=docker.m.daocloud.io/library
FROM ${REGISTRY}/golang:1.22-alpine AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/dcim-api ./cmd/dcim-api \
    && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/demo-data ./cmd/demo-data

# ---- 运行阶段 ----
ARG REGISTRY=docker.m.daocloud.io/library
FROM ${REGISTRY}/alpine:3.20
RUN apk add --no-cache tzdata ca-certificates && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone
WORKDIR /app
COPY --from=builder /out/dcim-api /out/demo-data /app/
EXPOSE 8095
# 不依赖配置文件: 全部由 DCIM_* 环境变量注入
ENTRYPOINT ["/app/dcim-api", "-c", ""]
