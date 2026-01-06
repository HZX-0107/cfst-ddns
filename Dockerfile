# 第一阶段：构建阶段
# [修改点 1] 切换回 Alpine 镜像，并升级到 1.25 以匹配你本地的 Go 版本
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# [关键步骤] 安装基础工具
# git: 虽然用 proxy，但某些依赖校验依然可能需要 git
# curl/tar: 用于后续下载 cfst
# ca-certificates: 防止 HTTPS 证书错误
RUN apk add --no-cache curl tar git ca-certificates

# 设置 Go 代理 (GitHub Actions 推荐官方源)
ENV GOPROXY=https://proxy.golang.org,direct

# [修改点 2] 恢复标准流程：利用 go.sum 缓存层
COPY go.mod go.sum ./

# 下载依赖
# 只要 go.sum 是对的，且 Go 版本匹配，这里就不会报错了
RUN go mod download

# 复制源代码 (包含 assets/embed.go 等)
COPY . .

# 设置要下载的版本
ARG CFST_VERSION=v2.3.4

# 下载并准备资源
# Alpine 自带 apk 安装的 curl，不需要 apt-get
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