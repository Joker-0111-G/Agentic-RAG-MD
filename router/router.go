package router

import (
	"net/http"
	"time"

	"Agentic-RAG-MD/handlers"
	"Agentic-RAG-MD/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 允许跨域（方便前端 dev）
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	apiV1 := r.Group("/api/v1")
	{
		chatGroup := apiV1.Group("/chat")
		chatGroup.Use(middleware.RateLimiter(10, time.Minute))
		{
			chatGroup.POST("/message", handlers.ChatMessageHandler)
			chatGroup.GET("/sessions", handlers.ListSessionsHandler)
			chatGroup.GET("/sessions/:session_id/history", handlers.GetSessionHistoryHandler)
			chatGroup.DELETE("/sessions/:session_id", handlers.DeleteSessionHandler)
		}

		docGroup := apiV1.Group("/documents")
		{
			docGroup.GET("", handlers.ListDocumentsHandler)
			docGroup.POST("/upload", handlers.UploadDocumentHandler)
			docGroup.DELETE("/:id", handlers.DeleteDocumentHandler)
		}
	}

	// 服务前端静态文件
	embedFrontend(r)

	return r
}

// embedFrontend 将 frontend/ 目录作为 Gin 的静态文件服务
func embedFrontend(r *gin.Engine) {
	r.StaticFile("/", "frontend/index.html")
	// SPA fallback: 前端路由请求都返回 index.html
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == "GET" && len(c.Request.URL.Path) > 0 && c.Request.URL.Path[0] == '/' &&
			(len(c.Request.URL.Path) < 4 || c.Request.URL.Path[:4] != "/api") {
			c.File("frontend/index.html")
			return
		}
		c.Next()
	})
}