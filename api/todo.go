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

type listTodosRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

func (server *Server) listTodos(ctx *gin.Context) {
	var req listTodosRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListTodosParams{
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	}

	todos, err := server.store.ListTodos(ctx, arg)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	resp := make([]todoResponse, 0, len(todos))
	for _, todo := range todos {
		resp = append(resp, newTodoResponse(todo))
	}

	ok(ctx, resp)
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
