package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	db "todoapp/db/sqlc"
	"todoapp/errcode"
)

func (server *Server) healthCheck(ctx *gin.Context) {
	ok(ctx, gin.H{"status": "ok"})
}

// todoResponse 是 todo 的對外形狀。
//
// 不直接回 db.Todo：自從加了軟刪除，db.Todo 多了 DeletedAt sql.NullTime，
// 序列化出來是 {"Time":"0001-01-01T00:00:00Z","Valid":false}——那是 Go 的
// 實作細節，不該變成 API 契約，而且回應永遠只會有未刪除的 todo，
// 這個欄位對呼叫端沒有意義。
type todoResponse struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

func newTodoResponse(todo db.Todo) todoResponse {
	return todoResponse{
		ID:        todo.ID,
		Title:     todo.Title,
		Completed: todo.Completed,
		CreatedAt: todo.CreatedAt,
	}
}

type createTodoRequest struct {
	Title string `json:"title" binding:"required,max=255"`
}

func (server *Server) createTodo(ctx *gin.Context) {
	var req createTodoRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	todo, err := server.store.CreateTodo(ctx, req.Title)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	getLogger(ctx).Info().Int64("todo_id", todo.ID).Msg("todo created")
	ok(ctx, newTodoResponse(todo))
}

type getTodoRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

func (server *Server) getTodo(ctx *gin.Context) {
	var req getTodoRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	todo, err := server.store.GetTodo(ctx, req.ID)
	if err != nil {
		// ErrNoRows 不是「系統壞了」，是「你要的東西不在」。
		// 混進 ErrInternal 的話，查一筆不存在的資料會在 log 裡留一行 error，
		// 值班的人會被一個正常情況叫起來
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrTodoNotFound, nil)
			return
		}
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	ok(ctx, newTodoResponse(todo))
}

// todoSummary 是「全部未刪除的 todo」的統計，**不受 status 與分頁影響**。
//
// 前端 footer 要顯示的是「還有幾筆未完成」這個全域數字，而不是「這一頁有幾筆」；
// 而且 status=active 的清單裡根本沒有已完成的項目，呼叫端自己算不出 completed。
type todoSummary struct {
	Total     int64 `json:"total"`
	Active    int64 `json:"active"`
	Completed int64 `json:"completed"`
}

// listTodosResponse 一起回清單與統計。
//
// 這裡從「直接回一個陣列」改成物件，是為了讓統計有地方放。多包一層的代價
// 換到的是：呼叫端一次請求就拿到畫面需要的全部資訊，不必再打第二支 API——
// 兩支 API 的數字還會來自兩個時間點。
type listTodosResponse struct {
	Items   []todoResponse `json:"items"`
	Summary todoSummary    `json:"summary"`
}

// listTodosRequest 三個參數都是選填。
//
// 分頁參數從必填改成選填帶預設值：API 不該假設只有一個呼叫端。
// 沒有分頁 UI 的前端不必為了拿資料而編造 page_id=1&page_size=50，
// 要分頁的呼叫端照樣能分。
type listTodosRequest struct {
	// oneof 把值域擋在 handler：SQL 那邊的 CASE 對未知值會落到 ELSE（等同 all），
	// 不擋的話 status=activ 這種錯字會安靜地回全部，
	// 使用者只會覺得「篩選壞了」卻沒有任何一行 log 提到它
	Status   string `form:"status" binding:"omitempty,oneof=all active completed"`
	PageID   int32  `form:"page_id" binding:"omitempty,min=1"`
	PageSize int32  `form:"page_size" binding:"omitempty,min=1,max=200"`
}

const (
	defaultListStatus   = "all"
	defaultListPageID   = 1
	defaultListPageSize = 50
)

func (server *Server) listTodos(ctx *gin.Context) {
	var req listTodosRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}
	if req.Status == "" {
		req.Status = defaultListStatus
	}
	if req.PageID == 0 {
		req.PageID = defaultListPageID
	}
	if req.PageSize == 0 {
		req.PageSize = defaultListPageSize
	}

	arg := db.ListTodosParams{
		Status:     req.Status,
		PageLimit:  req.PageSize,
		PageOffset: (req.PageID - 1) * req.PageSize,
	}

	todos, err := server.store.ListTodos(ctx, arg)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	count, err := server.store.CountTodos(ctx)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	items := make([]todoResponse, 0, len(todos))
	for _, todo := range todos {
		items = append(items, newTodoResponse(todo))
	}

	ok(ctx, listTodosResponse{
		Items: items,
		Summary: todoSummary{
			Total:     count.Total,
			Active:    count.Active,
			Completed: count.Completed,
		},
	})
}

// updateTodoRequest 兩個欄位都是指標：要能分辨「沒帶這個欄位」與「帶了零值」。
// 用非指標的話，{"completed": false} 跟 {} 在 Go 這邊長得一模一樣，
// 「取消完成」這個動作就永遠送不出去。
type updateTodoRequest struct {
	Title     *string `json:"title" binding:"omitempty,max=255"`
	Completed *bool   `json:"completed"`
}

func (server *Server) updateTodo(ctx *gin.Context) {
	var uri getTodoRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	var req updateTodoRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	// 什麼都沒帶就擋下來。照樣送 UPDATE 的話，SQL 會把每個欄位
	// COALESCE 成原值、回 200，呼叫端會以為自己改成功了
	if req.Title == nil && req.Completed == nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrTodoNoFieldsToUpdate, nil)
		return
	}

	arg := db.UpdateTodoParams{ID: uri.ID}
	if req.Title != nil {
		arg.Title = sql.NullString{String: *req.Title, Valid: true}
	}
	if req.Completed != nil {
		arg.Completed = sql.NullBool{Bool: *req.Completed, Valid: true}
	}

	todo, err := server.store.UpdateTodo(ctx, arg)
	if err != nil {
		// UPDATE ... RETURNING 沒打到任何一列時，sqlc 的 :one 會回 ErrNoRows，
		// 意思是這個 id 不存在或已被軟刪除，不是查詢失敗
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrTodoNotFound, nil)
			return
		}
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	getLogger(ctx).Info().
		Int64("todo_id", todo.ID).
		Bool("title_changed", req.Title != nil).
		Bool("completed_changed", req.Completed != nil).
		Msg("todo updated")
	ok(ctx, newTodoResponse(todo))
}

// deleteTodo 是軟刪除：打上 deleted_at 時間戳，資料列留著。
// 對 API 的呼叫端而言行為跟硬刪除一樣（之後查不到、列表不會出現），
// 差別只在資料還救得回來。
func (server *Server) deleteTodo(ctx *gin.Context) {
	var req getTodoRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	rows, err := server.store.SoftDeleteTodo(ctx, req.ID)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}
	// 影響 0 列代表「不存在」或「已經刪過了」。兩者對呼叫端是同一件事——
	// 這筆資料現在不在了——所以回同一個碼，不細分
	if rows == 0 {
		fail(ctx, http.StatusNotFound, errcode.ErrTodoNotFound, nil)
		return
	}

	getLogger(ctx).Info().Int64("todo_id", req.ID).Msg("todo soft deleted")
	// 統一回應格式之後不再回 204：204 規定不能有 body，
	// 但現在連成功都要走 {code, msg, data}，兩者衝突。
	// 讓「成功一律 200 + code 0」比省下一個 body 重要
	ok(ctx, nil)
}

// completeAllTodosRequest 的 Completed 是指標 + required：
// bool 的零值是 false，非指標的話 required 會把「全部取消完成」
// （{"completed": false}）當成沒帶欄位擋掉。
type completeAllTodosRequest struct {
	Completed *bool `json:"completed" binding:"required"`
}

// completeAllTodos 把所有 todo 一次設為完成或未完成（todomvc 的全選）。
// PATCH /todos
//
// 端點打在集合本身而不是 /todos/complete-all：gin 的路由樹裡靜態片段
// 與 :id 這種參數片段同層會衝突（註冊時直接 panic），而「對整個集合
// 做一次部分更新」本來就該打在集合上，跟 PATCH /todos/:id 語意一致。
func (server *Server) completeAllTodos(ctx *gin.Context) {
	var req completeAllTodosRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	rows, err := server.store.SetAllTodosCompleted(ctx, *req.Completed)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	// 這行 log 的重點是 affected：批次操作出事時要先知道它動了幾筆。
	// 影響 0 列不是錯誤（本來就全部都是那個狀態了），但看得到 0
	// 跟看不到，排查時差很多
	getLogger(ctx).Info().
		Bool("completed", *req.Completed).
		Int64("affected", rows).
		Msg("all todos completion updated")
	ok(ctx, gin.H{"affected": rows})
}

// deleteTodosRequest 的 status 沒有預設值、也不接受 all：
// 批次刪除必須明講刪的是哪一批。少了這個限制，一個漏帶參數的
// DELETE /todos 就會把整張表清掉。
type deleteTodosRequest struct {
	Status string `form:"status" binding:"required,oneof=completed"`
}

// deleteCompletedTodos 清除所有已完成的 todo（todomvc 的 clear completed）。
// DELETE /todos?status=completed
//
// 跟單筆刪除一樣是軟刪除：兩條路徑的刪除語意必須一致，
// 否則「哪些資料救得回來」要看使用者按了哪個鈕。
func (server *Server) deleteCompletedTodos(ctx *gin.Context) {
	var req deleteTodosRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	rows, err := server.store.SoftDeleteCompletedTodos(ctx)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	getLogger(ctx).Info().Int64("affected", rows).Msg("completed todos soft deleted")
	ok(ctx, gin.H{"affected": rows})
}
