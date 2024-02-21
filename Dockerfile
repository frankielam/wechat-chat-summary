# 使用Go官方镜像作为构建阶段的基础镜像
FROM golang:alpine AS build-stage

# 设置工作目录
WORKDIR /src

# 复制go模块相关文件
COPY go.mod go.sum ./

# 设置代理并下载依赖
RUN GOPROXY=https://goproxy.cn,direct go mod download

# 复制源代码文件到容器中
COPY main.go config.json ./

# 添加一个非root用户
# 注意：对于scratch基础镜像，创建用户需要在运行阶段处理
RUN adduser -D -g '' myuser

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app

# 使用scratch作为最终镜像的基础镜像
FROM scratch

# 设置时区环境变量
ENV TZ=Asia/Shanghai

# 设置工作目录
WORKDIR /

# 从构建阶段复制证书，以便能够进行HTTPS请求
COPY --from=build-stage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 从构建阶段复制非root用户信息
COPY --from=build-stage /etc/passwd /etc/passwd

# 使用非root用户运行应用
USER myuser

# 从构建阶段复制编译好的应用和配置文件
COPY --from=build-stage /app /app
COPY --from=build-stage /src/config.json /

# 如果您的应用监听端口，请确保此处的端口号与您的应用设置一致
EXPOSE 10000

# 设置容器启动时执行的命令
ENTRYPOINT ["/app"]