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
SELECT * FROM todos
WHERE deleted_at IS NULL
ORDER BY id
LIMIT $1
OFFSET $2;

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

-- name: SoftDeleteTodo :execrows
-- 軟刪除：打時間戳，不真的刪除資料列。
--
-- WHERE 帶 deleted_at IS NULL 有兩個作用：重複刪除會影響 0 列（handler 回 404，
-- 跟「本來就不存在」同一個結果，對呼叫端來說語意一致），
-- 而且不會把第一次的刪除時間覆蓋掉。
UPDATE todos
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
