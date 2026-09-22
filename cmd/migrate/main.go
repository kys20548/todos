// migrate 是獨立於 todoapp server 的建表/更新 schema 指令，用 urfave/cli
// 包成一支正常的 CLI（--help、子命令、flag），單獨執行就能用，不用
// 透過 Makefile 或 docker compose 才知道怎麼呼叫它。
//
// 拿掉 entrypoint.sh 那套「容器啟動時自動跑 migration」的做法：schema
// 變更跟「啟動 server」是兩件不同的事，混在一起會讓 server 沒事重啟
// 一次（例如 docker compose restart、平台自動重啟）也跟著重跑一次
// migration，多一個不必要的失敗點。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/urfave/cli/v3"

	"todoapp/internal/util"
)

func main() {
	cmd := &cli.Command{
		Name:  "migrate",
		Usage: "todoapp 資料庫 schema migration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Value: "file://internal/db/migration",
				Usage: "migration 檔案的位置",
			},
			&cli.StringFlag{
				Name:  "database",
				Usage: "資料庫連線字串，預設讀 app.env 的 DB_SOURCE",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "up",
				Usage:  "套用所有還沒跑過的 migration",
				Action: runMigration(func(m *migrate.Migrate) error { return m.Up() }),
			},
			{
				Name:   "down",
				Usage:  "回滾所有 migration",
				Action: runMigration(func(m *migrate.Migrate) error { return m.Down() }),
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

// runMigration 組出 *migrate.Migrate 並執行 do，up/down 共用同一套
// 「決定連線字串、跑完印一行 log、ErrNoChange 不算失敗」的邏輯。
func runMigration(do func(*migrate.Migrate) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		dbSource := cmd.String("database")
		if dbSource == "" {
			config, err := util.LoadConfig(".")
			if err != nil {
				return fmt.Errorf("cannot load config: %w", err)
			}
			dbSource = config.DBSource
		}

		m, err := migrate.New(cmd.String("source"), dbSource)
		if err != nil {
			return fmt.Errorf("cannot init migrate: %w", err)
		}

		if err := do(m); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("%s failed: %w", cmd.Name, err)
		}

		slog.Info("migration finished", "direction", cmd.Name)
		return nil
	}
}
