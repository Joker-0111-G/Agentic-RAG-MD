package router

import (
	"time"

	"Agentic-RAG-MD/handlers"
	"Agentic-RAG-MD/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		chatGroup := v1.Group("/chat")
		chatGroup.Use(middleware.RateLimiter(10, time.Minute)) 
		{
			chatGroup.POST("/message", handlers.ChatMessageHandler)
			chatGroup.GET("/sessions", handlers.ListSessionsHandler)
			chatGroup.GET("/sessions/:session_id/history", handlers.GetSessionHistoryHandler)
		}

		docGroup := v1.Group("/documents")
		{
			docGroup.POST("/upload", handlers.UploadDocumentHandler)
			docGroup.DELETE("/:id", handlers.DeleteDocumentHandler)
		}
	}
	return r
}