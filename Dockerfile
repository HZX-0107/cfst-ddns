# 第一阶段：构建阶段
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# [修改点 1] 这一步必须提到最前面！
# 在下载 Go 依赖之前，必须先安装 git、curl 和 tar
# git: go mod download 某些时候需要它
# curl/tar: 后面下载 cfst 需要它
RUN apk add --no-cache curl tar git

# [修改点 2] 设置 Go 代理，防止网络超时 (可选，但在 GitHub Actions 里加上更稳)
ENV GOPROXY=https://proxy.golang.org,direct

# 复制依赖文件
COPY go.mod go.sum ./

# 下载依赖
# 现在有了 git 和 proxy，这一步应该能通过了
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