// Package db 是 todos 的資料存取層，底層用 gorm 操作 PostgreSQL。
package db

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

// ListTodosParams、UpdateTodoParams、CountTodosRow 維持跟原本 sqlc 版本
// 一樣的形狀：handler（internal/api/todo.go）不用跟著換資料層就重寫一次。
type ListTodosParams struct {
	Status     string
	PageOffset int32
	PageLimit  int32
}

type UpdateTodoParams struct {
	ID        int64
	Title     sql.NullString
	Completed sql.NullBool
}

type CountTodosRow struct {
	Total     int64
	Active    int64
	Completed int64
}

// Store 提供所有 DB 操作。介面維持跟 sqlc 版本一樣，方便 mock 測試，
// 也讓換底層（sqlc → gorm）不用動到 handler。
type Store interface {
	CreateTodo(ctx context.Context, title string) (Todo, error)
	// 已軟刪除的查不到：對 API 的呼叫端來說，刪掉就是不存在。
	GetTodo(ctx context.Context, id int64) (Todo, error)
	// status 只有 all / active / completed 三種，值域在 handler 用 binding:"oneof" 擋。
	ListTodos(ctx context.Context, arg ListTodosParams) ([]Todo, error)
	// 三個數字一次查完，而且**不受 status 與分頁影響**：前端 footer 要顯示的是
	// 「還有幾筆未完成」這個全域數字，不是「這一頁有幾筆」。
	CountTodos(ctx context.Context) (CountTodosRow, error)
	// title 與 completed 都是選填（PATCH 語意）：沒帶的欄位維持原值。
	UpdateTodo(ctx context.Context, arg UpdateTodoParams) (Todo, error)
	// 軟刪除：打時間戳，不真的刪除資料列。
	SoftDeleteTodo(ctx context.Context, id int64) (int64, error)
	// todomvc 的「全選 / 取消全選」，只動真正需要改的列。
	SetAllTodosCompleted(ctx context.Context, completed bool) (int64, error)
	// todomvc 的「清除已完成」，跟單筆刪除一樣是軟刪除。
	SoftDeleteCompletedTodos(ctx context.Context) (int64, error)
}

// GormStore 是 Store 的實際實作。
type GormStore struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) Store {
	return &GormStore{db: db}
}

func (s *GormStore) CreateTodo(ctx context.Context, title string) (Todo, error) {
	todo := Todo{Title: title}
	err := s.db.WithContext(ctx).Create(&todo).Error
	return todo, err
}

func (s *GormStore) GetTodo(ctx context.Context, id int64) (Todo, error) {
	var todo Todo
	err := s.db.WithContext(ctx).First(&todo, id).Error
	return todo, err
}

// ListTodos 篩選寫在 SQL 而不是撈回來再過濾：不讓資料量決定記憶體用量，
// 也讓「分頁」與「篩選」算在同一組資料上（先過濾再分頁才是對的）。
func (s *GormStore) ListTodos(ctx context.Context, arg ListTodosParams) ([]Todo, error) {
	todos := []Todo{}
	q := s.db.WithContext(ctx).Order("id")

	switch arg.Status {
	case "active":
		q = q.Where("completed = ?", false)
	case "completed":
		q = q.Where("completed = ?", true)
	}

	err := q.Limit(int(arg.PageLimit)).Offset(int(arg.PageOffset)).Find(&todos).Error
	return todos, err
}

func (s *GormStore) CountTodos(ctx context.Context) (CountTodosRow, error) {
	var row CountTodosRow
	err := s.db.WithContext(ctx).Model(&Todo{}).
		Select("count(*) AS total, count(*) FILTER (WHERE completed = false) AS active, count(*) FILTER (WHERE completed = true) AS completed").
		Scan(&row).Error
	return row, err
}

// UpdateTodo 用 map 只帶真的要改的欄位，COALESCE 的效果交給
// 「沒被指定的欄位不出現在 UPDATE 裡」達成。0 列被改動代表這個 id
// 不存在或已被軟刪除，統一回 ErrRecordNotFound，跟 GetTodo 一致。
func (s *GormStore) UpdateTodo(ctx context.Context, arg UpdateTodoParams) (Todo, error) {
	updates := map[string]any{}
	if arg.Title.Valid {
		updates["title"] = arg.Title.String
	}
	if arg.Completed.Valid {
		updates["completed"] = arg.Completed.Bool
	}

	result := s.db.WithContext(ctx).Model(&Todo{}).Where("id = ?", arg.ID).Updates(updates)
	if result.Error != nil {
		return Todo{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Todo{}, gorm.ErrRecordNotFound
	}

	var todo Todo
	err := s.db.WithContext(ctx).First(&todo, arg.ID).Error
	return todo, err
}

// SoftDeleteTodo 交給 gorm 的 Delete：model 上有 gorm.DeletedAt 的話，
// Delete 預設就是軟刪除（UPDATE ... SET deleted_at = now()），且會自動
// 只打到 deleted_at IS NULL 的列，重複刪除自然影響 0 列。
func (s *GormStore) SoftDeleteTodo(ctx context.Context, id int64) (int64, error) {
	result := s.db.WithContext(ctx).Delete(&Todo{}, id)
	return result.RowsAffected, result.Error
}

// SetAllTodosCompleted 只挑真正需要改的列（completed <> 目標值），
// 回傳的影響列數才等於「這次實際改了幾筆」。
func (s *GormStore) SetAllTodosCompleted(ctx context.Context, completed bool) (int64, error) {
	result := s.db.WithContext(ctx).Model(&Todo{}).
		Where("completed <> ?", completed).
		Update("completed", completed)
	return result.RowsAffected, result.Error
}

func (s *GormStore) SoftDeleteCompletedTodos(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Where("completed = ?", true).Delete(&Todo{})
	return result.RowsAffected, result.Error
}
