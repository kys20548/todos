package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	db "todoapp/internal/db"
	"todoapp/internal/util"
)

func workerCommand() *cli.Command {
	return &cli.Command{
		Name:  "worker",
		Usage: "背景工作",
		Commands: []*cli.Command{
			{
				Name:  "event",
				Usage: "定時統計 todo 數量並寫 log（示範用 worker，證明三種執行單位共用同一個 image）",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:  "interval",
						Usage: "統計間隔",
						Value: 30 * time.Second,
					},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					return runEventWorker(cmd.String("env"), cmd.Duration("interval"))
				},
			},
		},
	}
}

func runEventWorker(env string, interval time.Duration) error {
	config, err := util.LoadConfig("./config", env)
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}
	logger := setupLogger(config)

	gormDB, err := openGorm(config)
	if err != nil {
		return err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("cannot get underlying sql.DB: %w", err)
	}
	defer sqlDB.Close()
	store := db.NewStore(gormDB)

	// SIGINT / SIGTERM（docker stop 送的是 SIGTERM）取消 ctx，迴圈跟著結束
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("start event worker", "env", config.Environment, "interval", interval)

	report := func() {
		row, err := store.CountTodos(ctx)
		if err != nil {
			logger.Error("count todos failed", "error", err)
			return
		}
		logger.Info("todo summary", "total", row.Total, "active", row.Active, "completed", row.Completed)
	}

	report()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("event worker 已安全終止")
			return nil
		case <-ticker.C:
			report()
		}
	}
}
