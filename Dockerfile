FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CGO_ENABLED=0：alpine 沒有 glibc，執行檔要是靜態的才能跑
RUN CGO_ENABLED=0 go build -o main ./cmd/todoapp
RUN CGO_ENABLED=0 go build -o migrate ./cmd/migrate

FROM alpine:3.22
WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/migrate .
COPY app.env .
COPY web ./web

EXPOSE 8080
CMD ["./main"]
