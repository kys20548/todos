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
# 1. 啟動 PostgreSQL
docker compose up -d

# 2. 建立 / 更新資料表（需要 golang-migrate CLI）
make migrateup

# 3. 啟動 server
go run main.go
```

沒有 `make` / `migrate` CLI 時（例如 Windows PowerShell），直接把 SQL 餵進容器：

```powershell
Get-Content db\migration\000001_init_schema.up.sql | docker exec -i todoapp_db psql -U root -d todoapp
Get-Content db\migration\000002_add_soft_delete.up.sql | docker exec -i todoapp_db psql -U root -d todoapp
```

Server 預設監聽 `0.0.0.0:8080`（設定於 `app.env`，可用環境變數覆蓋）。
