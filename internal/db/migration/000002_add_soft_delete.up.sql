-- 刪除改成軟刪除：打上時間戳，資料留著。
--
-- 用 deleted_at timestamptz 而不是 is_deleted boolean：一個欄位同時回答
-- 「刪了沒」與「什麼時候刪的」，不會出現 is_deleted = true 但不知道何時刪的狀態。
ALTER TABLE "todos" ADD COLUMN "deleted_at" timestamptz;
