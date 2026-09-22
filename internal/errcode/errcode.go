// Package errcode 集中管理 API 回應的業務狀態碼。
//
// 每個碼是一個 Code 物件，code 跟訊息綁在同一個變數宣告裡，不會出現
// 「加了碼忘了加訊息」或「改了訊息漏改碼」——兩者本來就是同一件事的
// 兩個面向，分開放在兩個地方維護只會讓它們漂移。
//
// 編碼規則：
//
//	E000   成功
//	E0xx   通用錯誤（跟哪個業務模組無關的）
//	E1xx   todo 相關
//
// E2xx / E3xx 留給之後的模組（使用者、其他資源），現在不用。
//
// 這裡刻意只定義「目前真的有分支會回傳」的碼。多一個沒有人回傳的碼，
// 排查時會有人去追一條不存在的路徑；少一個，兩種處置方式不同的失敗
// 就會擠在同一個碼上、log 看不出差別。每個碼下面都註明它是哪個分支、
// 拿掉之後哪件事會就此沉默。
package errcode

// Code 是 API 回應的業務狀態碼，直接以 {"code":"E001","msg":"..."} 這個
// 形狀序列化——呼叫端不用另外拿 code 去查一份訊息對照表。
type Code struct {
	ID  string `json:"code"`
	Msg string `json:"msg"`
}

var (
	// Success 是唯一的成功碼。
	Success = Code{"E000", "success"}

	// --- 通用錯誤 E0xx ---

	// ErrInternal 是所有未分類錯誤與 panic 的出口。
	// 拿掉它：handler 失去「這裡我沒想到」的統一落點，未預期的錯誤
	// 只剩一個 HTTP 500，呼叫端拿不到可判斷的碼。
	ErrInternal = Code{"E001", "系統內部錯誤"}

	// ErrInvalidParams：gin binding 失敗（欄位缺漏、型別不符、值域不對）。
	// 拿掉它：呼叫端分不出「我送錯了」與「伺服器壞了」，會一直重送同一個壞請求。
	ErrInvalidParams = Code{"E002", "參數錯誤"}

	// ErrNotFound：路由不存在。
	// 拿掉它：打錯網址會拿到 gin 預設的純文字 404，破壞統一回應格式，
	// 前端解 envelope 會炸在一個跟業務無關的地方，容易誤判成後端掛了。
	ErrNotFound = Code{"E003", "資源不存在"}

	// --- todo 相關 E1xx ---

	// ErrTodoNotFound：指定 id 的 todo 不存在，或已經被軟刪除。
	// 兩者合用同一個碼是刻意的——對呼叫端來說是同一件事：這筆資料現在不在了。
	// 拿掉它：刪一筆不存在的資料會回成功，前端不會知道自己拿的是舊清單。
	ErrTodoNotFound = Code{"E101", "待辦事項不存在"}

	// ErrTodoNoFieldsToUpdate：PATCH 沒帶任何要更新的欄位。
	// 拿掉它：SQL 會把每個欄位 COALESCE 成原值然後回 200，
	// 呼叫端會以為自己改成功了。
	ErrTodoNoFieldsToUpdate = Code{"E102", "沒有指定任何要更新的欄位"}
)
