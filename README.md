# todoapp

## 使用技術

| 項目 | 版本 |
|---|---|
| Go | 1.25+ |
| PostgreSQL | 16 (postgres:16-alpine) |
| gin-gonic/gin | v1.12.0 |
| gorm.io/gorm | v1.31.2 |
| gorm.io/driver/postgres | v1.6.3 |
| urfave/cli | v3.13.0 |
| spf13/viper | v1.21.0 |

日誌用標準庫 `log/slog`，沒有額外依賴。

## 如何啟動

```bash
docker compose up -d --build               # 啟動 PostgreSQL、app、pgAdmin
docker compose run --rm app ./migrate up   # 第一次啟動，或 model 有新欄位時執行
```

migration 是獨立的 CLI 指令（`cmd/migrate`，用 urfave/cli 包的、底層是 gorm 的 `AutoMigrate`），不跟著
app 啟動自動跑——重啟 server 不該順便重跑一次 migration。這支指令本身不依賴 Makefile 或
docker-compose，`./migrate --help`、`./migrate up` 可以直接拿二進位單獨執行，要接哪個資料庫用
`--database` 覆蓋。AutoMigrate 只會新增缺少的資料表/欄位，不會刪欄位或改型別，所以沒有 `down`。

本機開發想直接跑 server（不進 container）：指令要在專案根目錄執行，因為 server 讀 `app.env`、
掛靜態檔都是用相對路徑：

```bash
docker compose up -d postgres   # 只啟動資料庫
make migrateup                  # 建立 / 更新資料表
go run ./cmd/todoapp
```

Server 預設監聽 `0.0.0.0:8080`（設定於 `app.env`，可用環境變數覆蓋）。
