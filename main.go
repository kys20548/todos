package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"todoapp/api"
	db "todoapp/db/sqlc"
	"todoapp/util"
)

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}

	// development 環境輸出人類可讀格式，production 輸出 JSON
	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}
	defer conn.Close()

	store := db.NewStore(conn)

	server, err := api.NewServer(config, store)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

	httpServer := &http.Server{
		Addr:    config.HTTPServerAddress,
		Handler: server.Router(),
	}

	// 在 goroutine 中啟動 server，main goroutine 負責監聽關閉訊號
	go func() {
		log.Info().Msgf("start HTTP server at %s", config.HTTPServerAddress)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("cannot start server")
		}
	}()

	listenSignal(httpServer, config)
}

// listenSignal 阻塞等待 SIGINT / SIGTERM，收到訊號後優雅關閉 server：
// 停止接收新連線，並在 timeout 內等待進行中的請求處理完成。
func listenSignal(server *http.Server, config util.Config) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch // 阻塞，直到收到訊號

	log.Info().Msg("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("http server 已安全終止")
}
