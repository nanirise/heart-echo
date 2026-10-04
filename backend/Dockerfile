FROM golang:1.27-alpine AS build
# proxy.golang.org 国内被拒；goproxy.cn 可达，direct 兜底
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
# 依赖清单单独一层：源码改动不会让 go mod download 的层缓存失效
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 静态编译，产出不依赖 libc 的单一二进制
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server .
EXPOSE 8080
CMD ["./server"]
