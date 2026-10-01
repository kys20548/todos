package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/urfave/cli/v3"

	"todoapp/internal/api"
	db "todoapp/internal/db"
	"todoapp/internal/util"
)

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "啟動 HTTP server",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "port",
				Usage: "監聽的 port，有給就覆蓋設定檔 HTTP_SERVER_ADDRESS 裡的 port",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runServe(cmd.String("env"), int(cmd.Int("port")))
		},
	}
}

func runServe(env string, port int) error {
	config, err := util.LoadConfig("./config", env)
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}
	if port != 0 {
		config.HTTPServerAddress, err = withPort(config.HTTPServerAddress, port)
		if err != nil {
			return err
		}
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

	server, err := api.NewServer(config, db.NewStore(gormDB))
	if err != nil {
		return fmt.Errorf("cannot create server: %w", err)
	}

	httpServer := &http.Server{
		Addr:    config.HTTPServerAddress,
		Handler: server.Router(),
	}

	// 在 goroutine 中啟動 server，main goroutine 負責監聽關閉訊號
	go func() {
		logger.Info("start HTTP server", "env", config.Environment, "address", config.HTTPServerAddress)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("cannot start server", "error", err)
			os.Exit(1)
		}
	}()

	return listenSignal(httpServer, config, logger)
}

// withPort 保留 addr 的 host，只換掉 port。
func withPort(addr string, port int) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid HTTP_SERVER_ADDRESS %q: %w", addr, err)
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// listenSignal 阻塞等待 SIGINT / SIGTERM，收到訊號後優雅關閉 server：
// 停止接收新連線，並在 timeout 內等待進行中的請求處理完成。
func listenSignal(server *http.Server, config util.Config, logger *slog.Logger) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch // 阻塞，直到收到訊號

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	logger.Info("http server 已安全終止")
	return nil
}
