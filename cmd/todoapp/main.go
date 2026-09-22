package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"todoapp/internal/api"
	db "todoapp/internal/db/sqlc"
	"todoapp/internal/util"
)

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		slog.Error("cannot load config", "error", err)
		os.Exit(1)
	}

	// development 環境輸出人類可讀的文字格式，production 輸出 JSON
	var handler slog.Handler
	if config.Environment == "development" {
		handler = slog.NewTextHandler(os.Stderr, nil)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		logger.Error("cannot connect to db", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	store := db.NewStore(conn)

	server, err := api.NewServer(config, store)
	if err != nil {
		logger.Error("cannot create server", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:    config.HTTPServerAddress,
		Handler: server.Router(),
	}

	// 在 goroutine 中啟動 server，main goroutine 負責監聽關閉訊號
	go func() {
		logger.Info("start HTTP server", "address", config.HTTPServerAddress)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("cannot start server", "error", err)
			os.Exit(1)
		}
	}()

	listenSignal(httpServer, config, logger)
}

// listenSignal 阻塞等待 SIGINT / SIGTERM，收到訊號後優雅關閉 server：
// 停止接收新連線，並在 timeout 內等待進行中的請求處理完成。
func listenSignal(server *http.Server, config util.Config, logger *slog.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch // 阻塞，直到收到訊號

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("http server 已安全終止")
}
