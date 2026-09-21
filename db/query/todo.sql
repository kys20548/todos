-- name: CreateTodo :one
INSERT INTO todos (
    title
) VALUES (
    $1
) RETURNING *;

-- name: GetTodo :one
-- 已軟刪除的查不到：對 API 的呼叫端來說，刪掉就是不存在。
SELECT * FROM todos
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: ListTodos :many
-- status 只有 all / active / completed 三種，值域在 handler 用 binding:"oneof" 擋。
-- 篩選寫在 SQL 而不是撈回來再過濾：不讓資料量決定記憶體用量，
-- 也讓「分頁」與「篩選」算在同一組資料上（先過濾再分頁才是對的）。
SELECT * FROM todos
WHERE deleted_at IS NULL
  AND CASE sqlc.arg(status)::text
        WHEN 'active'    THEN completed = false
        WHEN 'completed' THEN completed = true
        ELSE true
      END
ORDER BY id
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: CountTodos :one
-- 三個數字一次查完，而且**不受 status 與分頁影響**：前端 footer 要顯示的是
-- 「還有幾筆未完成」這個全域數字，不是「這一頁有幾筆」。
-- 分三支 query 去數會讓三個數字來自三個時間點。
SELECT
    count(*)                                   AS total,
    count(*) FILTER (WHERE completed = false)  AS active,
    count(*) FILTER (WHERE completed = true)   AS completed
FROM todos
WHERE deleted_at IS NULL;

-- name: UpdateTodo :one
-- title 與 completed 都是選填（PATCH 語意）：沒帶的欄位維持原值。
-- 用 COALESCE 而不是在 Go 那邊拼 SQL，可選欄位再多也只有這一句。
-- 加上 deleted_at IS NULL：已刪除的不能再被改。
UPDATE todos
SET
    title = COALESCE(sqlc.narg(title), title),
    completed = COALESCE(sqlc.narg(completed), completed)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SetAllTodosCompleted :execrows
-- todomvc 的「全選 / 取消全選」。
--
-- WHERE 只挑真正需要改的列（completed <> 目標值）：回傳的影響列數才等於
-- 「這次實際改了幾筆」，log 看得出這次操作的規模；全部已經是目標狀態時
-- 影響 0 列，那不是錯誤。
UPDATE todos
SET completed = sqlc.arg(completed)
WHERE deleted_at IS NULL AND completed <> sqlc.arg(completed);

-- name: SoftDeleteCompletedTodos :execrows
-- todomvc 的「清除已完成」。跟單筆刪除一樣是軟刪除，不是真的 DELETE——
-- 兩條路徑的刪除語意必須一致，否則「哪些資料救得回來」要看使用者按了哪個鈕。
UPDATE todos
SET deleted_at = now()
WHERE deleted_at IS NULL AND completed = true;

-- name: SoftDeleteTodo :execrows
-- 軟刪除：打時間戳，不真的刪除資料列。
--
-- WHERE 帶 deleted_at IS NULL 有兩個作用：重複刪除會影響 0 列（handler 回 404，
-- 跟「本來就不存在」同一個結果，對呼叫端來說語意一致），
-- 而且不會把第一次的刪除時間覆蓋掉。
UPDATE todos
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
