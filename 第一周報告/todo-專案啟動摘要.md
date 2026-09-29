# Todo 專案

> Before Start · 檢查會談前摘要

作者 **Danny** ｜ 骨架 **template_golang_web** ｜ 技術 **Gin · gorm · PostgreSQL · TodoMVC(javascript-es5)**

---

## 01 功能範圍

| 狀態 | 項目 |
|---|---|
| 已完成 | 新增 / 查詢單筆 / 查詢列表（分頁、狀態篩選） |
| 已完成 | 列表統計 summary（total / active / completed，全域計數） |
| 已完成 | 部分更新（title / completed，至少帶一項） |
| 已完成 | 全選完成 / 取消完成、清除已完成（批次操作） |
| 已完成 | 刪除（軟刪除，deleted_at 標記） |
| 已完成 | 統一回應格式、錯誤碼、request_id 貫穿日誌 |
| 已完成 | 前端串接<br>TodoMVC（examples/javascript-es5），放在 web/，由 Gin 同一個 server 靜態服務 |
| 計畫中 | 還原已刪除 todo<br>PUT /todos/:id/restore |

## 02 資料表

| 欄位 | 型別 | 限制 / 預設 |
|---|---|---|
| `id` | `bigserial` | PRIMARY KEY |
| `title` | `varchar` | NOT NULL |
| `completed` | `boolean` | NOT NULL DEFAULT false |
| `created_at` | `timestamptz` | NOT NULL DEFAULT now() |
| `deleted_at` | `timestamptz` | nullable（migration 000002，軟刪除用） |

## 03 API 端點

回應統一為 `{code, msg, data}`

| Method | Path | 說明 |
|---|---|---|
| GET | `/healthz` | 健康檢查 |
| POST | `/todos` | 新增 |
| GET | `/todos/:id` | 單筆 |
| GET | `/todos?status=&page_id=&page_size=` | 列表 |
| GET | `/todos-summary` | 統計（獨立端點，不跑列表查詢） |
| PATCH | `/todos` | 全選完成 / 取消完成 |
| DELETE | `/todos?status=completed` | 清除已完成 |
| PATCH | `/todos/:id` | 部分更新 |
| DELETE | `/todos/:id` | 軟刪除 |

## 04 錯誤分類

| 碼 | HTTP | 常數 | 觸發條件 |
|---|---|---|---|
| `E000` | 200 | `Success` | 成功 |
| `E001` | 500 | `ErrInternal` | 系統內部錯誤（DB 錯誤、panic） |
| `E002` | 400 | `ErrInvalidParams` | 參數錯誤（binding 失敗） |
| `E003` | 404 | `ErrNotFound` | 路由不存在 |
| `E101` | 404 | `ErrTodoNotFound` | todo 不存在或已被軟刪除 |
| `E102` | 400 | `ErrTodoNoFieldsToUpdate` | PATCH 未帶任何要更新的欄位 |

段別：E000 成功、E0xx 通用、E1xx todo；E2xx / E3xx 保留給後續模組。碼與訊息一起宣告成 `Code{ID, Msg}`（internal/errcode/errcode.go），只定義真的有分支會回傳的碼

---

PostgreSQL（docker compose）· gin + viper + gorm · 前後端同一個 server ｜ 下一階段：02 搬資料庫
