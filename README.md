# todoapp

## 使用技術

| 項目 | 版本 |
|---|---|
| Go | 1.25+ |
| PostgreSQL | 16 (postgres:16-alpine) |
| gin-gonic/gin | v1.12.0 |
| gorm.io/gorm | v1.31.2 |
| gorm.io/driver/postgres | v1.6.3 |
| golang-migrate/migrate | v4.20.1 |
| urfave/cli | v3.13.0 |
| spf13/viper | v1.21.0 |

日誌用標準庫 `log/slog`，沒有額外依賴。

## 如何啟動

```bash
docker compose up -d --build               # 啟動 PostgreSQL、app、pgAdmin
docker compose run --rm app ./migrate up   # 第一次啟動，或 model 有新欄位時執行
```

migration 是獨立的 CLI 指令（`cmd/migrate`，用 urfave/cli 包的、底層是 golang-migrate），不跟著
app 啟動自動跑——重啟 server 不該順便重跑一次 migration。`.sql` 檔案（`internal/db/migration`）
用 `go:embed` 編進這支執行檔本身，不用額外帶著檔案走，`./migrate --help`、`./migrate up` 可以
直接拿二進位單獨執行，要接哪個資料庫用 `--database` 覆蓋。

- `./migrate up`：套用所有還沒跑過的 migration
- `./migrate down`：回滾所有 migration（**會砍資料/砍表**，不要對正式環境的 DB 跑）
- `./migrate force <version>`：只標記目前版本、不執行任何 SQL——schema 已經用別的方式建好
  （例如舊版用 gorm `AutoMigrate` 建的資料庫）、只是缺 `schema_migrations` 記錄時用

本機開發想直接跑 server（不進 container）：指令要在專案根目錄執行，因為 server 讀 `config/`、
掛靜態檔都是用相對路徑：

```bash
docker compose up -d postgres   # 只啟動資料庫
make migrateup                  # 建立 / 更新資料表
go run ./cmd/todoapp            # 等同 --env dev
```

### 切換環境設定

設定檔依環境分成 `config/app.dev.env`、`config/app.qa.env`、`config/app.prod.env`，
`todoapp` 和 `migrate` 都用 `--env`（或環境變數 `APP_ENV`）選要讀哪一份，沒給預設 `dev`，
給了不在清單內的值（例如打錯成 `prd`）會直接啟動失敗：

```bash
go run ./cmd/todoapp --env qa
go run ./cmd/migrate --env qa up
make server ENV=qa                     # Makefile 用 ENV 帶進去
APP_ENV=prod docker compose up -d      # docker compose 用 APP_ENV
```

`dev` 用人類可讀的文字 log、gin debug mode；`qa`、`prod` 用 JSON log、gin release mode。

Server 預設監聽 `0.0.0.0:8080`（設定於 `config/app.<env>.env`，可用環境變數覆蓋）。
