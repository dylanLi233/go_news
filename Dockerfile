FROM golang:1.24-alpine AS builder

WORKDIR /app

# 复制go.mod和go.sum
COPY go.mod ./
COPY go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /hacker-news-server ./cmd/server

# 使用轻量级镜像
FROM alpine:latest

# 安装CA证书和时区数据
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从builder阶段复制编译好的应用
COPY --from=builder /hacker-news-server /app/

# 创建配置目录
RUN mkdir -p /app/config

# 暴露端口
EXPOSE 8080

# 设置时区
ENV TZ=Asia/Shanghai

# 启动应用
CMD ["/app/hacker-news-server"]
