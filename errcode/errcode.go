// Package errcode 集中管理 API 回應的業務狀態碼。
//
// 編碼規則：
//
//	0      成功
//	1xxxx  通用錯誤（跟哪個業務模組無關的）
//	4xxxx  todo 相關
//
// 2xxxx / 3xxxx 留給之後的模組（使用者、其他資源），現在不用。
//
// 這裡刻意只定義「目前真的有分支會回傳」的碼。多一個沒有人回傳的碼，
// 排查時會有人去追一條不存在的路徑；少一個，兩種處置方式不同的失敗
// 就會擠在同一個碼上、log 看不出差別。每個碼下面都註明它是哪個分支、
// 拿掉之後哪件事會就此沉默。
package errcode

// Code 為 API 回應的業務狀態碼。
type Code int

const (
	// Success 是唯一的成功碼。
	Success Code = 0

	// --- 通用錯誤 1xxxx ---

	// ErrInternal 是所有未分類錯誤與 panic 的出口。
	// 拿掉它：handler 失去「這裡我沒想到」的統一落點，未預期的錯誤
	// 只剩一個 HTTP 500，呼叫端拿不到可判斷的碼。
	ErrInternal Code = 10001

	// ErrInvalidParams：gin binding 失敗（欄位缺漏、型別不符、值域不對）。
	// 拿掉它：呼叫端分不出「我送錯了」與「伺服器壞了」，會一直重送同一個壞請求。
	ErrInvalidParams Code = 10002

	// ErrNotFound：路由不存在。
	// 拿掉它：打錯網址會拿到 gin 預設的純文字 404，破壞統一回應格式，
	// 前端解 envelope 會炸在一個跟業務無關的地方，容易誤判成後端掛了。
	ErrNotFound Code = 10004

	// --- todo 相關 4xxxx ---

	// ErrTodoNotFound：指定 id 的 todo 不存在，或已經被軟刪除。
	// 兩者合用同一個碼是刻意的——對呼叫端來說是同一件事：這筆資料現在不在了。
	// 拿掉它：刪一筆不存在的資料會回成功，前端不會知道自己拿的是舊清單。
	ErrTodoNotFound Code = 40001

	// ErrTodoNoFieldsToUpdate：PATCH 沒帶任何要更新的欄位。
	// 拿掉它：SQL 會把每個欄位 COALESCE 成原值然後回 200，
	// 呼叫端會以為自己改成功了。
	ErrTodoNoFieldsToUpdate Code = 40002
)

var messages = map[Code]string{
	Success:                 "success",
	ErrInternal:             "系統內部錯誤",
	ErrInvalidParams:        "參數錯誤",
	ErrNotFound:             "資源不存在",
	ErrTodoNotFound:         "待辦事項不存在",
	ErrTodoNoFieldsToUpdate: "沒有指定任何要更新的欄位",
}

// Msg 回傳錯誤碼對應的訊息。未定義的碼回傳系統內部錯誤訊息——
// 寧可對外講得含糊，也不要漏出一個空字串讓前端顯示空白提示。
func (c Code) Msg() string {
	if msg, ok := messages[c]; ok {
		return msg
	}
	return messages[ErrInternal]
}
