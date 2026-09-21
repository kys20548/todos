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
	router.Use(httpLogger(), gin.Recovery())

	router.GET("/healthz", server.healthCheck)

	router.POST("/todos", server.createTodo)
	router.GET("/todos/:id", server.getTodo)
	router.GET("/todos", server.listTodos)
	router.PATCH("/todos/:id", server.updateTodo)
	router.DELETE("/todos/:id", server.deleteTodo)

	server.router = router
}

// Router 回傳 gin engine，讓 main 可以把它掛到 http.Server 上做 graceful shutdown。
func (server *Server) Router() *gin.Engine {
	return server.router
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
