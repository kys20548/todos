# todoapp

以 `template_golang_web` 為骨架，把 user 換成 todo：
gin + viper + sqlc + PostgreSQL，目前只有後端 server。

## 執行方式

```bash
# 1. 啟動 PostgreSQL
docker compose up -d

# 2. 建立資料表（需要 golang-migrate CLI）
make migrateup

# 3. 啟動 server
go run main.go
```

依賴跟模板完全一樣（`go.sum` 直接沿用），所以模組快取裡已經有了，
不需要重新 `go mod tidy`。

## 試打

```bash
curl localhost:8080/healthz
# {"status":"ok"}

curl -X POST localhost:8080/todos \
  -H 'Content-Type: application/json' -d '{"title":"買牛奶"}'
# {"id":1,"title":"買牛奶","completed":false,"created_at":"..."}

curl localhost:8080/todos/1
curl "localhost:8080/todos?page_id=1&page_size=5"
```

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
    "created_at" timestamptz NOT NULL DEFAULT (now())
);
```

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
- [ ] 更新 / 完成切換 / 刪除的 API（`DeleteTodo` 的 query 已經有了，還沒接路由）
- [ ] 統一回應格式與錯誤碼（目前跟模板一樣是 `{"error": "..."}`）
- [ ] 前端
