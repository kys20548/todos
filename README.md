# todoapp

## 使用技術

| 項目 | 版本 |
|---|---|
| Go | 1.25+ |
| PostgreSQL | 16 (postgres:16-alpine) |
| gin-gonic/gin | v1.12.0 |
| sqlc | v1.31.1 |
| golang-migrate | v4.20.1 |
| lib/pq | v1.12.3 |
| rs/zerolog | v1.35.1 |
| spf13/viper | v1.21.0 |

## 如何啟動

```bash
docker compose up -d --build
```

啟動 PostgreSQL 跟 app 兩個 container，migration 會在 app 啟動時自動跑完，不用另外下 migrate 指令。

本機開發想直接用 `go run main.go` 跑（不進 container），才需要手動跑 migration：

```bash
docker compose up -d postgres   # 只啟動資料庫
make migrateup                  # 建立 / 更新資料表（需要 golang-migrate CLI）
go run main.go
```

Server 預設監聽 `0.0.0.0:8080`（設定於 `app.env`，可用環境變數覆蓋）。
