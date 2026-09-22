package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"todoapp/internal/api"
	db "todoapp/internal/db"
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

	// gorm 預設的 logger 會把每次查詢、甚至「查不到」都印成一行帶顏色的
	// 純文字到 stdout，跟這支服務自己的 slog 結構化 log 是兩套不相干的
	// 輸出格式，而且把「查不到」印成刺眼的錯誤，正是這個專案在 handler
	// 那層已經刻意避免的事（一筆正常的 404 不該讓值班的人以為系統壞了）。
	// 關掉它，交給 access log + handler 自己的 log 記
	gormDB, err := gorm.Open(postgres.Open(config.DBSource), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		logger.Error("cannot connect to db", "error", err)
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		logger.Error("cannot get underlying sql.DB", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	store := db.NewStore(gormDB)

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
