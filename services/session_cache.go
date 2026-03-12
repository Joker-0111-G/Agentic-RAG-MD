package services

import (
	"encoding/json"
	"time"

	"Agentic-RAG-MD/global"
	"Agentic-RAG-MD/models"
	"github.com/sashabaranov/go-openai"
)

const (
	ContextCachePrefix = "chat_context:"
	MaxContextKeep     = 10
	ContextExpire      = 30 * time.Minute 
)

func GetSessionContext(sessionID string) ([]openai.ChatCompletionMessage, error) {
	redisKey := ContextCachePrefix + sessionID
	var messages []openai.ChatCompletionMessage

	records, err := global.RedisClient.LRange(global.Ctx, redisKey, 0, -1).Result()
	
	if err == nil && len(records) > 0 {
		for _, record := range records {
			var msg openai.ChatCompletionMessage
			_ = json.Unmarshal([]byte(record), &msg)
			messages = append(messages, msg)
		}
		global.RedisClient.Expire(global.Ctx, redisKey, ContextExpire)
		return messages, nil
	}

	var history []models.ChatHistory
	err = global.DB.Where("session_id = ? AND role IN ('user', 'assistant')", sessionID).
		Order("created_at desc").Limit(MaxContextKeep).Find(&history).Error

	if err != nil || len(history) == 0 {
		return messages, nil 
	}

	for i := len(history) - 1; i >= 0; i-- {
		msg := openai.ChatCompletionMessage{ Role: history[i].Role, Content: history[i].Content }
		messages = append(messages, msg)
		val, _ := json.Marshal(msg)
		global.RedisClient.RPush(global.Ctx, redisKey, val)
	}
	global.RedisClient.Expire(global.Ctx, redisKey, ContextExpire)
	return messages, nil
}

func AppendToSessionContext(sessionID string, userMsg, assistantMsg openai.ChatCompletionMessage) {
	redisKey := ContextCachePrefix + sessionID
	userVal, _ := json.Marshal(userMsg)
	assistantVal, _ := json.Marshal(assistantMsg)

	pipe := global.RedisClient.Pipeline()
	pipe.RPush(global.Ctx, redisKey, userVal, assistantVal)
	pipe.LTrim(global.Ctx, redisKey, int64(-MaxContextKeep), -1)
	pipe.Expire(global.Ctx, redisKey, ContextExpire)

	if _, err := pipe.Exec(global.Ctx); err != nil {
		println("Redis 更新上下文缓存失败:", err.Error())
	}
}