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
# {"code":0,"msg":"success","data":{"status":"ok"}}

curl -X POST localhost:8080/todos \
  -H 'Content-Type: application/json' -d '{"title":"買牛奶"}'
# {"code":0,"msg":"success","data":{"id":1,"title":"買牛奶","completed":false,"created_at":"..."}}

curl localhost:8080/todos/1

# 列表：三個參數都是選填（預設 status=all、page_id=1、page_size=50）
curl localhost:8080/todos
curl "localhost:8080/todos?status=active"
curl "localhost:8080/todos?status=completed&page_id=1&page_size=10"
# {"code":0,"msg":"success","data":{
#   "items":[...],
#   "summary":{"total":3,"active":2,"completed":1}}}

# 全選 / 取消全選
curl -X PATCH localhost:8080/todos \
  -H 'Content-Type: application/json' -d '{"completed":true}'
# {"code":0,"msg":"success","data":{"affected":2}}

# 清除已完成（status 必填且只接受 completed）
curl -X DELETE "localhost:8080/todos?status=completed"

# 部分更新：只帶要改的欄位，沒帶的維持原值
curl -X PATCH localhost:8080/todos/1 \
  -H 'Content-Type: application/json' -d '{"completed":true}'
curl -X PATCH localhost:8080/todos/1 \
  -H 'Content-Type: application/json' -d '{"title":"買豆漿"}'

# 刪除（軟刪除）：不存在或已刪過回 404 + 40001
curl -X DELETE localhost:8080/todos/1
curl localhost:8080/todos/999
# {"code":40001,"msg":"待辦事項不存在","data":null}
```

## API

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/healthz` | 健康檢查 |
| POST | `/todos` | 新增 |
| GET | `/todos/:id` | 單筆，不存在回 404 |
| GET | `/todos?status=&page_id=&page_size=` | 列表 + 統計；參數皆選填 |
| PATCH | `/todos` | 全部設為完成 / 未完成（`{"completed": bool}`），回 `affected` |
| DELETE | `/todos?status=completed` | 清除已完成（軟刪除），回 `affected`；`status` 必填 |
| PATCH | `/todos/:id` | 部分更新（`title` / `completed` 至少帶一個），不存在回 404 |
| DELETE | `/todos/:id` | **軟刪除**；不存在或已刪過回 404 + 40001 |

### 列表的回應形狀

```json
{"code":0,"msg":"success","data":{
  "items": [{"id":1,"title":"買牛奶","completed":false,"created_at":"..."}],
  "summary": {"total":3,"active":2,"completed":1}
}}
```

- **`summary` 不受 `status` 與分頁影響**，一律統計「全部未刪除的 todo」。
  前端 footer 要的是「還有幾筆未完成」這個全域數字，不是「這一頁有幾筆」；
  而且 `status=active` 的清單裡根本沒有已完成的項目，呼叫端自己算不出 `completed`。
- 從「直接回陣列」改成物件是為了讓統計有地方放。多包一層，換到的是
  一次請求就拿到畫面需要的全部資訊——分兩支 API 的話兩個數字會來自兩個時間點。
- **分頁參數改成選填帶預設值**（`page_id=1`、`page_size=50`，上限 200）：
  API 不該假設只有一個呼叫端。沒有分頁 UI 的前端不必為了拿資料而編造參數，
  要分頁的呼叫端照樣能分。

### 集合層級的操作

全選與清除已完成打在集合上（`PATCH /todos`、`DELETE /todos?status=completed`），
不是 `/todos/complete-all` 這種子路徑——**靜態片段與 `:id` 在 gin 的路由樹同一層
會衝突，註冊時直接 panic**；而且「對整個集合做一次部分更新」本來就該打在集合上。

`DELETE /todos` 的 `status` 沒有預設值也不接受 `all`：批次刪除必須明講刪哪一批，
否則一個漏帶參數的請求就會清空整張表。清除已完成跟單筆刪除一樣是**軟刪除**——
兩條路徑的刪除語意必須一致，否則「哪些資料救得回來」要看使用者按了哪個鈕。

---

所有回應（含錯誤）都是 `{code, msg, data}`：

```json
{"code": 0,     "msg": "success",        "data": {...}}
{"code": 40001, "msg": "待辦事項不存在",  "data": null}
```

## 錯誤碼

定義在 `errcode/errcode.go`。分段：0 成功、1xxxx 通用、4xxxx todo
（2xxxx / 3xxxx 留給之後的模組）。

**只定義目前真的有分支會回傳的碼**——多一個沒人回傳的碼，排查時會有人去追
一條不存在的路徑；少一個，兩種處置方式不同的失敗就會擠在同一個碼上。

| 碼 | HTTP | 觸發分支 | 拿掉它會怎樣 |
|---|---|---|---|
| 0 | 200 | 成功 | — |
| 10001 | 500 | DB 錯誤、panic | handler 失去「這裡我沒想到」的統一落點 |
| 10002 | 400 | gin binding 失敗 | 呼叫端分不出「我送錯」與「伺服器壞了」，會一直重送同一個壞請求 |
| 10004 | 404 | 路由不存在 | gin 回純文字 404，前端解 envelope 會炸在跟業務無關的地方 |
| 40001 | 404 | todo 不存在 **或已被軟刪除** | 刪一筆不存在的資料回成功，前端不知道自己拿的是舊清單 |
| 40002 | 400 | PATCH 沒帶任何欄位 | SQL 全部 COALESCE 成原值後回 200，呼叫端以為改成功了 |

**錯誤細節不回給 client**：`fail(ctx, status, code, err)` 的 `err` 只進 log
（經 `ctx.Error()`，由 access log 那一行印出來），client 永遠只拿到 errcode
的固定訊息。DB 錯誤訊息會帶出表名、欄位名甚至參數值，排查需要的東西
應該用 request_id 去 log 撈，不該從瀏覽器看。

**刪除成功回 200 不回 204**：204 規定不能有 body，但成功也要走
`{code, msg, data}`——「成功一律 200 + code 0」比省下一個 body 重要。

## 日誌與排查

每個請求都有 `request_id`，同時回在 `X-Request-Id` header 上。
一次請求的 access log、handler 裡的 log、錯誤那一行都帶同一個 id，
撈一次就看得到完整因果，不必靠時間戳去猜是哪一筆。

```
{"level":"info","request_id":"9f3c…","todo_id":4,"message":"todo created"}
{"level":"info","request_id":"9f3c…","method":"POST","path":"/todos","status_code":200,"duration":3.1,"message":"received a HTTP request"}
```

- **等級跟著 status 走**：5xx 是 `error`、4xx 是 `warn`、其餘 `info`——
  「只看 error」就等於「只看伺服器自己的問題」，不會被使用者輸入錯誤淹沒。
- **回應裡看不到的原始錯誤在 access log 的 `error` 欄位**（`ctx.Errors`）。
- middleware 順序：`requestID → httpLogger → recovery → handler`。
  requestID 在最外層因為之後每層都要用它；recovery 在最內層，
  panic 才不會穿過 httpLogger——否則最該被記錄的那次請求反而沒有 access log。
- request_id 用 `crypto/rand` 的 16 bytes hex，沒有為此引進 uuid 套件：
  它只需要「在一段時間的 log 裡不撞」。

## 程式結構

跟模板一樣，只是 `user` 換成 `todo`：

```
├── main.go              # 進入點：載入設定、連 DB、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── api/                 # gin handler、路由、middleware、統一回應
├── errcode/             # 業務狀態碼
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
- [x] 統一回應格式 `{code, msg, data}` + 錯誤碼 `errcode/`
- [x] request_id 貫穿、access log 分級、統一 panic 回應
- [x] 列表篩選 `status`、全域統計 `summary`、全選、清除已完成（給前端用）
- [ ] 前端（todomvc `examples/javascript-es5`，放在同 repo 的 `web/`）
