package handlers

import (
	"io"
	"net/http"
	"time"

	"Agentic-RAG-MD/global"
	"Agentic-RAG-MD/models"
	"Agentic-RAG-MD/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListSessionsHandler(c *gin.Context) {
	var sessions []models.ChatSession
	if err := global.DB.Order("updated_at desc").Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取会话列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "获取成功", "data": sessions})
}

func GetSessionHistoryHandler(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id 不能为空"})
		return
	}

	var history []models.ChatHistory
	if err := global.DB.Where("session_id = ?", sessionID).Order("created_at asc").Find(&history).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史记录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "获取成功", "data": history})
}

func DeleteSessionHandler(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id 不能为空"})
		return
	}

	// 删除会话及其关联的历史记录
	if err := global.DB.Where("session_id = ?", sessionID).Delete(&models.ChatHistory{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除会话历史失败: " + err.Error()})
		return
	}
	if err := global.DB.Where("id = ?", sessionID).Delete(&models.ChatSession{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除会话失败: " + err.Error()})
		return
	}

	// 清理 Redis 缓存
	redisKey := "chat_context:" + sessionID
	global.RedisClient.Del(global.Ctx, redisKey)

	c.JSON(http.StatusOK, gin.H{"message": "会话已删除"})
}

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content" binding:"required"`
}

func ChatMessageHandler(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
		newSession := models.ChatSession{ID: sessionID, Title: req.Content}
		global.DB.Create(&newSession)
	} else {
		global.DB.Model(&models.ChatSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now())
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.SSEvent("meta", gin.H{"session_id": sessionID})
	c.Writer.Flush()

	streamChan := make(chan services.StreamMessage)
	go services.RunAgentLoop(sessionID, req.Content, streamChan)

	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-streamChan; ok {
			c.SSEvent(msg.Event, msg.Data)
			return true 
		}
		return false 
	})
}