package main

import (
	"fmt"
	"log/slog"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"todoapp/internal/util"
)

// setupLogger 依環境決定格式並設成預設 logger：
// dev 輸出人類可讀的文字格式，qa / prod 輸出 JSON。
func setupLogger(config util.Config) *slog.Logger {
	var handler slog.Handler
	if config.Environment == util.EnvDev {
		handler = slog.NewTextHandler(os.Stderr, nil)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// openGorm 連上資料庫。gorm 預設的 logger 會把每次查詢、甚至「查不到」都印成
// 一行帶顏色的純文字，跟這支服務自己的 slog 結構化 log 是兩套輸出格式，而且
// 把「查不到」印成刺眼的錯誤，正是這個專案在 handler 那層已經刻意避免的事
// （一筆正常的 404 不該讓值班的人以為系統壞了）。關掉它，交給 access log +
// handler 自己的 log 記。serve 與 worker 共用。
func openGorm(config util.Config) (*gorm.DB, error) {
	gormDB, err := gorm.Open(postgres.Open(config.DBSource), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot connect to db: %w", err)
	}
	return gormDB, nil
}
