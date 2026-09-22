package db

import (
	"time"

	"gorm.io/gorm"
)

// Todo 是 todos 資料表對應的 gorm model。
//
// DeletedAt 用 gorm.DeletedAt 而不是自己手動維護的 sql.NullTime：gorm 會
// 自動幫每個查詢加上 WHERE deleted_at IS NULL，Delete() 也預設只是補上
// 這個時間戳、不會真的砍資料列——跟這專案原本「軟刪除、deleted_at 不是
// bool」的設計是同一件事，只是交給 gorm 做，不用每支 query 自己寫一次
// WHERE。
type Todo struct {
	ID        int64 `gorm:"primaryKey"`
	Title     string
	Completed bool `gorm:"not null;default:false"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Todo) TableName() string {
	return "todos"
}
