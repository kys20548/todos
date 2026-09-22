package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"todoapp/internal/errcode"
)

const (
	requestIDHeader = "X-Request-Id"
	requestIDKey    = "request_id"
	loggerKey       = "logger"
)

// requestIDMiddleware 為每個請求產生 request_id，並把帶著它的 logger
// 放進 gin context；同時把 id 回在 response header 上。
//
// 這是排查的起點：一個請求的 access log、handler 裡的 log、錯誤那一行，
// 全部帶同一個 id，撈一次就看得到完整因果，不必靠時間戳去猜是哪一筆。
// client 自己帶 X-Request-Id 就沿用，方便跨服務把一次呼叫串起來。
//
// 它必須掛在最外層：之後每一層記 log 都要用到它。
func requestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		ctx.Set(requestIDKey, requestID)
		ctx.Writer.Header().Set(requestIDHeader, requestID)

		logger := slog.Default().With(requestIDKey, requestID)
		ctx.Set(loggerKey, logger)

		ctx.Next()
	}
}

// newRequestID 產生 16 bytes 的隨機 hex。
//
// 沒有為了這件事引進 uuid 套件：這個 id 只需要「在一段時間的 log 裡不撞」，
// 不需要 UUID 的版本語意或排序性質。crypto/rand 失敗時退回時間戳——
// 產不出 id 不該讓請求失敗，log 少一點可讀性也比整支 API 掛掉好。
func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "ts-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

// getLogger 取出這個請求專屬的 logger。handler 裡一律用它記 log——
// 直接用預設的 logger 的話，那行 log 就沒有 request_id，也就串不回是哪一次請求。
func getLogger(ctx *gin.Context) *slog.Logger {
	if v, exists := ctx.Get(loggerKey); exists {
		if logger, ok := v.(*slog.Logger); ok {
			return logger
		}
	}
	return slog.Default()
}

// httpLogger 以 slog 記錄每一筆 HTTP 請求，包含這次回應的業務 code，
// 排查時不用另外去翻 response body 才知道回了哪個碼。
//
// 工作放在 ctx.Next() 之後，因為要等整條鏈跑完才知道 status 與耗時。
//
// ctx.Errors 是 fail() 塞進來的原始錯誤：回應裡看不到的細節都在這裡。
// 等級跟著 status 走——5xx 是 error、4xx 是 warn、其餘 info——
// 這樣「只看 error」就等於「只看伺服器自己的問題」，
// 不會被一堆使用者輸入錯誤淹沒。
func httpLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startTime := time.Now()
		path := ctx.Request.URL.Path
		rawQuery := ctx.Request.URL.RawQuery

		ctx.Next()

		duration := time.Since(startTime)
		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		statusCode := ctx.Writer.Status()
		logger := getLogger(ctx)

		args := []any{
			"protocol", "http",
			"method", ctx.Request.Method,
			"path", path,
			"status_code", statusCode,
			"status_text", http.StatusText(statusCode),
			"client_ip", ctx.ClientIP(),
			"duration", duration,
		}
		if len(ctx.Errors) > 0 {
			args = append(args, "error", ctx.Errors.String())
		}

		switch {
		case statusCode >= http.StatusInternalServerError:
			logger.Error("received a HTTP request", args...)
		case statusCode >= http.StatusBadRequest:
			logger.Warn("received a HTTP request", args...)
		default:
			logger.Info("received a HTTP request", args...)
		}
	}
}

// recoveryHandler 讓 panic 也走統一回應格式：client 收到的是
// {"code":"E001",...}，跟其他錯誤長得一樣，不會漏出 stack。
// server 端該留的細節由 slog 記成帶 request_id 的一行
// （gin 自己還是會把可讀的 stack 印到 stderr）。
func recoveryHandler(ctx *gin.Context, recovered any) {
	getLogger(ctx).Error("panic recovered",
		"panic", recovered,
		"method", ctx.Request.Method,
		"path", ctx.Request.URL.Path,
	)

	failAbort(ctx, http.StatusInternalServerError, errcode.ErrInternal, nil)
}

// noRouteHandler：打錯網址也要回統一格式。
// 沒有它的話 gin 會回純文字 404，前端解 envelope 會炸在一個
// 跟業務無關的地方，排查時容易誤判成後端掛了。
func noRouteHandler(ctx *gin.Context) {
	fail(ctx, http.StatusNotFound, errcode.ErrNotFound, nil)
}
