// migrate 是獨立於 todoapp server 的建表/更新 schema 指令，用 urfave/cli
// 包成一支正常的 CLI（--help、flag），單獨執行就能用，不用透過
// Makefile 或 docker compose 才知道怎麼呼叫它。
//
// 底層是 gorm 的 AutoMigrate，對齊 internal/db 裡的 model 定義，
// 不再是 golang-migrate 那種帶版本號的 up/down migration 檔案——
// AutoMigrate 只會新增缺少的資料表/欄位/索引，不會刪欄位、也不會改
// 欄位型別，所以沒有對應的 down：要回滾就是手動下 DDL 或還原備份。
//
// 拿掉 entrypoint.sh 那套「容器啟動時自動跑 migration」的做法：schema
// 變更跟「啟動 server」是兩件不同的事，混在一起會讓 server 沒事重啟
// 一次（例如 docker compose restart、平台自動重啟）也跟著重跑一次
// migration，多一個不必要的失敗點。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	db "todoapp/internal/db"
	"todoapp/internal/util"
)

func main() {
	cmd := &cli.Command{
		Name:  "migrate",
		Usage: "todoapp 資料庫 schema migration（gorm AutoMigrate）",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "database",
				Usage: "資料庫連線字串，預設讀 app.env 的 DB_SOURCE",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "建立 / 更新 todos 資料表，對齊目前的 model 定義",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					dbSource := cmd.String("database")
					if dbSource == "" {
						config, err := util.LoadConfig(".")
						if err != nil {
							return fmt.Errorf("cannot load config: %w", err)
						}
						dbSource = config.DBSource
					}

					// 關掉 gorm 自己的 logger，理由跟 cmd/todoapp 一樣：
					// AutoMigrate 內部的 introspection query 一樣可能踩到
					// gorm 預設會印成錯誤的「record not found」，跟這支
					// 指令自己的 slog log 是兩套不相干的輸出格式
					gormDB, err := gorm.Open(postgres.Open(dbSource), &gorm.Config{
						Logger: gormlogger.Default.LogMode(gormlogger.Silent),
					})
					if err != nil {
						return fmt.Errorf("cannot connect to db: %w", err)
					}

					if err := db.AutoMigrate(gormDB); err != nil {
						return fmt.Errorf("migration failed: %w", err)
					}

					slog.Info("migration finished")
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}
