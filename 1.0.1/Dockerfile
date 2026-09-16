# 终极简化版：每个平台独立处理
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

WORKDIR /app

# 安装编译依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置Go模块代理（加速下载）
ENV GOPROXY=https://goproxy.cn,direct

# 复制所有文件
COPY go.mod main.go ./

# 下载依赖（简化方式）
RUN go mod init xxk-proxy 2>/dev/null || true && \
    go get gopkg.in/natefinch/lumberjack.v2 && \
    go get gopkg.in/yaml.v3

# 编译程序（让Go自动处理架构）
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o proxy .

# 运行阶段
FROM alpine:latest

# 设置时区
ENV TZ=Asia/Shanghai

# 安装必要的运行时组件
RUN apk add --no-cache ca-certificates tzdata && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

# 创建工作目录和日志目录
RUN mkdir -p /app /var/log/proxy

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/proxy /usr/local/bin/proxy

# 复制配置文件
COPY config.yaml /app/config.yaml

# 设置文件权限
RUN chmod +x /usr/local/bin/proxy

# 暴露端口
EXPOSE 5858

# 启动命令
ENTRYPOINT ["proxy"]
CMD ["-config=/app/config.yaml"]