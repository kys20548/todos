// migrate 是獨立於 todoapp server 的建表/更新 schema 指令。
//
// 拿掉 entrypoint.sh 那套「容器啟動時自動跑 migration」的做法：schema
// 變更跟「啟動 server」是兩件不同的事，混在一起會讓 server 沒事重啟
// 一次（例如 docker compose restart、平台自動重啟）也跟著重跑一次
// migration，多一個不必要的失敗點。migration 什麼時候跑，交給操作者
// 自己決定：`go run ./cmd/migrate`，或容器內 `./migrate`。
package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"todoapp/internal/util"
)

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		slog.Error("cannot load config", "error", err)
		os.Exit(1)
	}

	m, err := migrate.New("file://internal/db/migration", config.DBSource)
	if err != nil {
		slog.Error("cannot init migrate", "error", err)
		os.Exit(1)
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		slog.Error("unknown migrate direction, want up or down", "direction", direction)
		os.Exit(1)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("migration failed", "direction", direction, "error", err)
		os.Exit(1)
	}

	slog.Info("migration finished", "direction", direction)
}
