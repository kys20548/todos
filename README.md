# todoapp

以 `template_golang_web` 為骨架，把 user 換成 todo：
gin + viper + sqlc + PostgreSQL，目前只有後端 server。

## 執行方式

```bash
# 1. 啟動 PostgreSQL
docker compose up -d

# 2. 建立 / 更新資料表（需要 golang-migrate CLI；沒裝的話見下方）
make migrateup

# 3. 啟動 server
go run main.go
```

依賴跟模板完全一樣（`go.sum` 直接沿用），所以模組快取裡已經有了，
不需要重新 `go mod tidy`。

沒有 `make` / `migrate` CLI 時（例如 Windows PowerShell），直接把 SQL 餵進容器：

```powershell
Get-Content db\migration\000001_init_schema.up.sql | docker exec -i todoapp_db psql -U root -d todoapp
Get-Content db\migration\000002_add_soft_delete.up.sql | docker exec -i todoapp_db psql -U root -d todoapp
```

## 試打

```bash
curl localhost:8080/healthz
# {"status":"ok"}

curl -X POST localhost:8080/todos \
  -H 'Content-Type: application/json' -d '{"title":"買牛奶"}'
# {"id":1,"title":"買牛奶","completed":false,"created_at":"..."}

curl localhost:8080/todos/1
curl "localhost:8080/todos?page_id=1&page_size=5"

# 部分更新：只帶要改的欄位，沒帶的維持原值
curl -X PATCH localhost:8080/todos/1 \
  -H 'Content-Type: application/json' -d '{"completed":true}'
curl -X PATCH localhost:8080/todos/1 \
  -H 'Content-Type: application/json' -d '{"title":"買豆漿"}'

# 刪除（軟刪除）：成功回 204，不存在或已刪過回 404
curl -i -X DELETE localhost:8080/todos/1
```

## API

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/healthz` | 健康檢查 |
| POST | `/todos` | 新增 |
| GET | `/todos/:id` | 單筆，不存在回 404 |
| GET | `/todos?page_id=1&page_size=5` | 分頁列表 |
| PATCH | `/todos/:id` | 部分更新（`title` / `completed` 至少帶一個），不存在回 404 |
| DELETE | `/todos/:id` | **軟刪除**，成功回 204；不存在或已刪過回 404 |

## 程式結構

跟模板一樣，只是 `user` 換成 `todo`：

```
├── main.go              # 進入點：載入設定、連 DB、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── api/                 # gin handler、路由、middleware
├── db/
│   ├── migration/       # golang-migrate 的 SQL
│   ├── query/           # sqlc 的 query 定義
│   └── sqlc/            # sqlc 生成碼 + Store interface
└── util/                # 設定載入
```

## 資料表

```sql
CREATE TABLE "todos" (
    "id" bigserial PRIMARY KEY,
    "title" varchar NOT NULL,
    "completed" boolean NOT NULL DEFAULT false,
    "created_at" timestamptz NOT NULL DEFAULT (now()),
    "deleted_at" timestamptz            -- migration 000002
);
```

## 軟刪除

`DELETE /todos/:id` 不會真的刪掉資料列，而是打上 `deleted_at` 時間戳。

- 用 `deleted_at timestamptz` 不用 `is_deleted boolean`：一個欄位同時回答
  「刪了沒」與「什麼時候刪的」，不會有 `is_deleted = true` 卻不知道何時刪的狀態。
- 查詢一律帶 `deleted_at IS NULL`（單筆、列表、更新都是）——對呼叫端來說，
  刪掉就是不存在，查不到也改不動。
- 軟刪除的 SQL 本身也帶 `deleted_at IS NULL`：重複刪除會影響 0 列，
  handler 回 404（跟「本來就不存在」同一個結果），而且不會把第一次的刪除時間蓋掉。
- API 回應用 `todoResponse` 而不是直接回 `db.Todo`：`sql.NullTime` 序列化出來是
  `{"Time":...,"Valid":false}`，那是 Go 的實作細節，不該變成 API 契約。
- 還沒做還原（`PUT /todos/:id/restore`）。要做的話就是一句
  `UPDATE todos SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`。
- 還沒加索引：目前資料量下 `deleted_at IS NULL` 的部分索引不會被用到，等真的慢了再加。

## 設定

| 環境變數 | 預設值 |
|---|---|
| `ENVIRONMENT` | `development` |
| `DB_DRIVER` | `postgres` |
| `DB_SOURCE` | `postgresql://root:secret@localhost:5432/todoapp?sslmode=disable` |
| `HTTP_SERVER_ADDRESS` | `0.0.0.0:8080` |
| `SHUTDOWN_TIMEOUT` | `10s` |

## 目前狀態

- [x] docker compose 起 PostgreSQL
- [x] `todos` 資料表 + migration
- [x] `POST /todos`、`GET /todos/:id`、`GET /todos`（分頁）、`GET /healthz`
- [x] `PATCH /todos/:id`（部分更新）、`DELETE /todos/:id`（軟刪除）
- [ ] 還原已刪除的 todo
- [ ] 統一回應格式與錯誤碼（目前跟模板一樣是 `{"error": "..."}`）
- [ ] 前端
