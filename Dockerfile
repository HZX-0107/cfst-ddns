# 第一阶段：构建阶段
# [修改点] 放弃 Alpine，改用标准 Debian 版 Go 镜像
# 这能解决 99% 的 "go mod download" 网络/证书/依赖缺失问题
FROM golang:1.23 AS builder

# 设置工作目录
WORKDIR /app

# [无需安装基础工具]
# 标准版镜像自带 git, curl, tar, ca-certificates，无需手动安装

# [修正] 设置 Go 代理
# GitHub Actions 位于海外，使用 Google 官方源速度最快
ENV GOPROXY=https://proxy.golang.org,direct

# 复制依赖文件
COPY go.mod go.sum ./

# 下载依赖
# 在 Debian 环境下，这一步通常极其稳定
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
# 注意：CGO_ENABLED=0 是必须的，因为我们在 Debian 编译，要在 Alpine 运行，必须静态链接
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cfst-ddns cmd/app/main.go

# 第二阶段：运行阶段 (保持不变，依然使用轻量级 Alpine)
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