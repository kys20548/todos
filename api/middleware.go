package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// httpLogger 以 zerolog 記錄每一筆 HTTP 請求。
func httpLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startTime := time.Now()
		ctx.Next()
		duration := time.Since(startTime)

		statusCode := ctx.Writer.Status()
		logger := log.Info()
		if statusCode >= http.StatusInternalServerError {
			logger = log.Error().Strs("errors", ctx.Errors.Errors())
		}

		logger.Str("protocol", "http").
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status_code", statusCode).
			Str("status_text", http.StatusText(statusCode)).
			Dur("duration", duration).
			Msg("received a HTTP request")
	}
}
