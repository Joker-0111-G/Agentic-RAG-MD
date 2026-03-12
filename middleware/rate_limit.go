package middleware

import (
	"fmt"
	"net/http"
	"time"

	"Agentic-RAG-MD/global"
	"github.com/gin-gonic/gin"
)

// RateLimiter 基于 Redis 的单 IP 限流中间件
func RateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		redisKey := fmt.Sprintf("rate_limit:%s:%s", c.FullPath(), clientIP)

		count, err := global.RedisClient.Incr(global.Ctx, redisKey).Result()
		if err != nil {
			fmt.Println("Redis 限流器异常:", err)
			c.Next()
			return
		}

		if count == 1 {
			global.RedisClient.Expire(global.Ctx, redisKey, window)
		}

		if count > int64(maxRequests) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "您的请求太频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}