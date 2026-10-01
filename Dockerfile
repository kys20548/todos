FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CGO_ENABLED=0：alpine 沒有 glibc，執行檔要是靜態的才能跑
# -trimpath -ldflags="-s -w"：拿掉路徑與除錯符號，執行檔小一截
# 單一 binary，serve / migrate / worker 三種執行單位共用，靠子命令區分
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o todoapp ./cmd/todoapp

FROM alpine:3.22
# 不用 root 跑：容器內被攻破時權限小一點
RUN adduser -D -u 10001 app
WORKDIR /app

COPY --from=builder /app/todoapp .
COPY config ./config
COPY web ./web

USER app
EXPOSE 8080

# ENTRYPOINT 固定執行檔，CMD 是預設子命令；compose 的 command: 會覆蓋 CMD，
# 例如 ["migrate", "up"]、["worker", "event"]
ENTRYPOINT ["./todoapp"]
CMD ["serve", "--port", "8080"]
