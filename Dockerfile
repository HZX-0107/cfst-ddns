# 第一阶段：构建阶段
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# [关键修复] 安装基础工具
# git: go mod download 依赖它
# ca-certificates: 解决 HTTPS 证书问题
# curl/tar: 用于后续下载 cfst
RUN apk add --no-cache curl tar git ca-certificates

# [修正] 设置 Go 代理
# GitHub Actions 位于海外，使用 Google 官方源速度最快
ENV GOPROXY=https://proxy.golang.org,direct

# 复制依赖文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码 (包含 assets/embed.go 等)
COPY . .

# 设置要下载的版本
ARG CFST_VERSION=v2.2.5

# 下载并准备资源
RUN curl -L "https://github.com/XIU2/CloudflareSpeedTest/releases/download/${CFST_VERSION}/cfst_linux_amd64.tar.gz" -o cfst.tar.gz && \
    tar -zxvf cfst.tar.gz && \
    mkdir -p assets && \
    mv cfst assets/cfst && \
    mv ip.txt assets/ && \
    mv ipv6.txt assets/ && \
    rm cfst.tar.gz

# 编译 Go 程序
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cfst-ddns cmd/app/main.go

# 第二阶段：运行阶段 (保持不变)
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY --from=builder /app/cfst-ddns .
COPY --from=builder /app/assets/ip.txt assets/ip.txt
COPY --from=builder /app/assets/ipv6.txt assets/ipv6.txt
COPY configs/ configs/
RUN touch app.log && chmod 666 app.log
VOLUME ["/app/configs", "/app/assets"]
ENTRYPOINT ["./cfst-ddns"]