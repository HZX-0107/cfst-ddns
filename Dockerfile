# 第一阶段：构建阶段
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装下载和解压工具
RUN apk add --no-cache curl tar

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码 (包含 assets/embed.go 等)
COPY . .

# --- 关键修改：动态拉取 CloudflareSpeedTest ---
# 设置要下载的版本
ARG CFST_VERSION=v2.2.5

# 1. 下载 tar.gz
# 2. 解压
# 3. 移动二进制文件到 assets/cfst (为了满足 go:embed)
# 4. 移动 ip.txt 到 assets/ (为了运行时使用)
RUN curl -L "https://github.com/XIU2/CloudflareSpeedTest/releases/download/${CFST_VERSION}/cfst_linux_amd64.tar.gz" -o cfst.tar.gz && \
    tar -zxvf cfst.tar.gz && \
    mkdir -p assets && \
    mv cfst assets/cfst && \
    mv ip.txt assets/ && \
    mv ipv6.txt assets/ && \
    rm cfst.tar.gz

# 编译 Go 程序
# 此时 assets/cfst 已经存在，编译可以通过
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cfst-ddns cmd/app/main.go

# 第二阶段：运行阶段
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

# 复制编译好的主程序
COPY --from=builder /app/cfst-ddns .

# 从构建阶段复制下载好的 IP 库
COPY --from=builder /app/assets/ip.txt assets/ip.txt
COPY --from=builder /app/assets/ipv6.txt assets/ipv6.txt

# 复制配置文件
COPY configs/ configs/

RUN touch app.log && chmod 666 app.log

VOLUME ["/app/configs", "/app/assets"]

ENTRYPOINT ["./cfst-ddns"]