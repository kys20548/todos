// migrate 是獨立於 todoapp server 的 schema migration 指令，用 urfave/cli
// 包成一支正常的 CLI（--help、子命令、flag），單獨執行就能用，不用透過
// Makefile 或 docker compose 才知道怎麼呼叫它。
//
// 底層是 golang-migrate，帶版本號的 .sql up/down 檔案（internal/db/migration），
// 而不是 gorm 的 AutoMigrate——AutoMigrate 只能新增欄位、不能刪欄位或改型別，
// 沒辦法真正回滾。這裡的 .sql 檔案透過 internal/db/migration 這個套件用
// go:embed 編進這支執行檔本身，不是執行時去讀磁碟上的路徑，所以 binary
// 丟到哪都能單獨跑，跟 docker compose run --rm app ./migrate up 這種用法
// 完全相容。
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
	"net/url"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/urfave/cli/v3"

	dbmigration "todoapp/internal/db/migration"
	"todoapp/internal/util"
)

func main() {
	cmd := &cli.Command{
		Name:  "migrate",
		Usage: "todoapp 資料庫 schema migration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "database",
				Usage: "資料庫連線字串，預設讀 app.env 的 DB_SOURCE",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "up",
				Usage:  "套用所有還沒跑過的 migration",
				Action: runMigration(func(m *migrate.Migrate, _ *cli.Command) error { return m.Up() }),
			},
			{
				Name:   "down",
				Usage:  "回滾所有 migration（會砍資料/砍表，不要對正式環境的 DB 跑）",
				Action: runMigration(func(m *migrate.Migrate, _ *cli.Command) error { return m.Down() }),
			},
			{
				Name:      "force",
				Usage:     "只標記目前版本，不執行任何 SQL——schema 已經用別的方式建好、只是缺 schema_migrations 記錄時用",
				ArgsUsage: "<version>",
				Action: runMigration(func(m *migrate.Migrate, cmd *cli.Command) error {
					version, err := strconv.Atoi(cmd.Args().First())
					if err != nil {
						return fmt.Errorf("version 必須是整數: %w", err)
					}
					return m.Force(version)
				}),
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

// runMigration 組出 *migrate.Migrate 並執行 do，up/down/force 共用同一套
// 「決定連線字串、跑完印一行 log、ErrNoChange 不算失敗」的邏輯。
func runMigration(do func(*migrate.Migrate, *cli.Command) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		dbSource := cmd.String("database")
		if dbSource == "" {
			config, err := util.LoadConfig(".")
			if err != nil {
				return fmt.Errorf("cannot load config: %w", err)
			}
			dbSource = config.DBSource
		}

		// database/pgx/v5 這個 driver 註冊在 "pgx5" scheme 下，config 裡的
		// DB_SOURCE 是給 gorm 用的 postgres:// / postgresql:// scheme，這裡
		// 轉成 pgx5:// 才能讓 golang-migrate 找到對應的 driver
		u, err := url.Parse(dbSource)
		if err != nil {
			return fmt.Errorf("invalid database url: %w", err)
		}
		u.Scheme = "pgx5"

		sourceDriver, err := iofs.New(dbmigration.FS, ".")
		if err != nil {
			return fmt.Errorf("cannot load embedded migration files: %w", err)
		}

		m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, u.String())
		if err != nil {
			return fmt.Errorf("cannot init migrate: %w", err)
		}
		defer m.Close()

		if err := do(m, cmd); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("%s failed: %w", cmd.Name, err)
		}

		slog.Info("migration finished", "command", cmd.Name)
		return nil
	}
}
