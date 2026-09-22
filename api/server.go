package api

import (
	"github.com/gin-gonic/gin"

	db "todoapp/db/sqlc"
	"todoapp/util"
)

// Server 負責處理所有 HTTP 請求。
type Server struct {
	config util.Config
	store  db.Store
	router *gin.Engine
}

// NewServer 建立 HTTP server 並設定路由。
func NewServer(config util.Config, store db.Store) (*Server, error) {
	server := &Server{
		config: config,
		store:  store,
	}

	server.setupRouter()
	return server, nil
}

func (server *Server) setupRouter() {
	if server.config.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 不用 gin.Default()：以 zerolog middleware 取代 gin 內建 logger
	router := gin.New()

	// middleware 的順序就是洋蔥的層數：ctx.Next() 之前的在「進去」的路上跑，
	// 之後的在「出來」的路上跑。
	//
	//   requestID → httpLogger → recovery → handler
	//
	// requestID 在最外層，因為之後每一層記 log 都要用它；
	// recovery 在最內層，panic 才不會穿過 httpLogger——否則
	// 最需要被記錄的那次請求反而不會有 access log。
	router.Use(
		requestIDMiddleware(),
		httpLogger(),
		gin.CustomRecovery(recoveryHandler),
	)

	// 打錯網址也要回統一格式，不要漏出 gin 預設的純文字 404
	router.NoRoute(noRouteHandler)

	// 前端靜態檔案：固定字首（/css、/js）而不是根目錄的萬用路由，
	// 避免跟 /todos、/healthz 這些已註冊的路由在 gin 路由樹的根節點衝突。
	router.StaticFile("/", "./web/index.html")
	router.Static("/css", "./web/css")
	router.Static("/js", "./web/js")

	router.GET("/healthz", server.healthCheck)

	// 集合層級：對「整批 todo」做的事打在集合上。
	// 不用 /todos/complete-all 這種靜態片段，是因為它會跟 /todos/:id
	// 在路由樹的同一層衝突，gin 註冊時會直接 panic
	router.GET("/todos", server.listTodos)
	router.POST("/todos", server.createTodo)
	router.PATCH("/todos", server.completeAllTodos)
	router.DELETE("/todos", server.deleteCompletedTodos)

	// 獨立的統計端點：不能叫 /todos/summary，那是靜態片段，
	// 會跟下面的 /todos/:id 在路由樹同一層衝突
	router.GET("/todos-summary", server.getTodosSummary)

	// 單筆
	router.GET("/todos/:id", server.getTodo)
	router.PATCH("/todos/:id", server.updateTodo)
	router.DELETE("/todos/:id", server.deleteTodo)

	server.router = router
}

// Router 回傳 gin engine，讓 main 可以把它掛到 http.Server 上做 graceful shutdown。
func (server *Server) Router() *gin.Engine {
	return server.router
}
