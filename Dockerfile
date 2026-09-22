FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CGO_ENABLED=0：alpine 沒有 glibc，執行檔要是靜態的才能跑
RUN CGO_ENABLED=0 go build -o main .
RUN CGO_ENABLED=0 go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1

FROM alpine:3.22
WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY db/migration ./db/migration
COPY app.env .
COPY web ./web
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
