package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"todoapp/internal/errcode"
)

// Response 是所有 API 的統一回應格式，錯誤也走同一個形狀。
// 前端只要解一種結構，不必為每支 API 各寫一套判斷。
//
// errcode.Code 用匿名欄位嵌進來，讓它的 code/msg 直接攤平到最外層——
// code 跟訊息本來就是同一個 Code 物件的兩個欄位，不用在這裡重複組一次。
type Response struct {
	errcode.Code
	Data any `json:"data"`
}

// ok 回傳成功回應。
func ok(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Response{Code: errcode.Success, Data: data})
}

// fail 回傳錯誤回應。
//
// err 只進 log，不回給 client——client 拿到的永遠是 errcode 定義的固定訊息。
// 這條界線是刻意的：DB 的錯誤訊息會帶出資料表名、欄位名、甚至參數值，
// 而排查需要的細節本來就該用 request_id 去 log 撈，不該從瀏覽器看。
//
// err 記在 ctx.Error() 而不是當場印：httpLogger 會在 access log 那一行
// 把它一起印出來，於是「哪個請求、回了什麼碼、錯在哪」是同一行，
// 不必在兩行 log 之間對時間。
func fail(ctx *gin.Context, httpStatus int, code errcode.Code, err error) {
	if err != nil {
		_ = ctx.Error(err)
	}
	ctx.JSON(httpStatus, Response{Code: code, Data: nil})
}

// failAbort 同 fail，但中斷後續的 handler。給 recovery 這類
// 「已經出事、不該再往下跑」的地方用。
func failAbort(ctx *gin.Context, httpStatus int, code errcode.Code, err error) {
	if err != nil {
		_ = ctx.Error(err)
	}
	ctx.AbortWithStatusJSON(httpStatus, Response{Code: code, Data: nil})
}
