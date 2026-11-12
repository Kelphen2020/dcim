# ---- 构建阶段 ----
ARG REGISTRY=docker.m.daocloud.io/library
FROM ${REGISTRY}/node:22-alpine AS builder
WORKDIR /src
COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com && \
    (npm ci --no-audit --no-fund || npm install --no-audit --no-fund)
COPY . .
RUN npm run build

# ---- 运行阶段: nginx 服务静态文件 + 反代 API ----
ARG REGISTRY=docker.m.daocloud.io/library
FROM ${REGISTRY}/nginx:1.27-alpine
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
