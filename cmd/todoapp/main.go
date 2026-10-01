// todoapp 是單一 binary、多個子命令：serve（HTTP server）、migrate（schema
// migration）、worker（背景工作）。三種執行單位共用同一個 image，靠 compose 的
// command 區分，例如：
//
//	todoapp migrate up
//	todoapp serve --port 8080
//	todoapp worker event
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"todoapp/internal/util"
)

func main() {
	cmd := &cli.Command{
		Name:  "todoapp",
		Usage: "todoapp：serve / migrate / worker",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "env",
				Usage:   "執行環境（dev / qa / prod），讀取 config/app.<env>.env",
				Value:   util.EnvDev,
				Sources: cli.EnvVars("APP_ENV"),
			},
		},
		Commands: []*cli.Command{
			serveCommand(),
			migrateCommand(),
			workerCommand(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("todoapp failed", "error", err)
		os.Exit(1)
	}
}
