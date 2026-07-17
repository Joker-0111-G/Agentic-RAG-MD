### 1. 初始化 Redis 客户端 (`global/redis.go`)

在你的 `global` 或 `pkg` 目录下，创建一个专门管理 Redis 连接的文件，暴露出全局可用的客户端实例。

Go

```
package global

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
	Ctx         = context.Background()
)

// InitRedis 初始化 Redis 连接池
func InitRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // 替换为你的 Redis 地址
		Password: "",               // 如果有密码填这里
		DB:       0,                // 默认 DB
		PoolSize: 10,               // 连接池大小
	})

	// 测试连接
	pong, err := RedisClient.Ping(Ctx).Result()
	if err != nil {
		return fmt.Errorf("连接 Redis 失败: %w", err)
	}
	fmt.Println("Redis 连接成功:", pong)
	return nil
}
```

------

### 2. 编写 Gin 限流中间件 (`middleware/rate_limit.go`)

大模型的 API 调用成本很高（按 Token 计费）。如果恶意用户通过脚本疯狂调用你的 `/chat` 接口，不仅会让你的系统崩溃，还会刷爆你的 API 额度。

我们利用 Redis 的 `INCR`（自增）和 `EXPIRE`（过期）命令，实现一个非常经典的**固定窗口限流器（Fixed Window Rate Limiter）**：限制同一个 IP 每分钟最多只能请求 10 次。

Go

```
package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	// "your_project/global" // 替换为全局包路径
)

// RateLimiter 基于 Redis 的单 IP 限流中间件
// maxRequests: 时间窗口内允许的最大请求数
// window: 时间窗口大小
func RateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		// 构造 Redis Key，例如: "rate_limit:chat:192.168.1.1"
		redisKey := fmt.Sprintf("rate_limit:%s:%s", c.FullPath(), clientIP)

		// 1. 请求次数自增 1
		count, err := global.RedisClient.Incr(global.Ctx, redisKey).Result()
		if err != nil {
			// Redis 挂了不影响主业务，记录日志后放行（降级处理）
			fmt.Println("Redis 限流器异常:", err)
			c.Next()
			return
		}

		// 2. 如果是第一次访问，设置这个 Key 的过期时间（也就是时间窗口）
		if count == 1 {
			global.RedisClient.Expire(global.Ctx, redisKey, window)
		}

		// 3. 判断是否超限
		if count > int64(maxRequests) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "您的请求太频繁，请稍后再试",
			})
			c.Abort() // 拦截请求，不再向后传递给具体的 Handler
			return
		}

		// 没超限，放行
		c.Next()
	}
}
```

------





### 1. 封装上下文缓存服务 (`services/session_cache.go`)

新建一个文件，专门负责管理大模型对话历史在 Redis 中的存取。这里我们利用 Redis 的 `RPUSH`（尾部追加）和 `LTRIM`（列表裁剪）命令，永远只在内存中保留最近的 N 条对话记录。

Go

```
package services

import (
	"encoding/json"
	"time"

	"github.com/sashabaranov/go-openai"
	// "your_project/global" // 替换为你的全局 Redis/DB 包
	// "your_project/models" 
)

const (
	// Redis Key 前缀
	ContextCachePrefix = "chat_context:"
	// 内存中最多保留的上下文条数 (比如 10 条 = 最近 5 轮问答)
	MaxContextKeep     = 10
	// 会话缓存的过期时间 (30分钟不活跃则释放内存)
	ContextExpire      = 30 * time.Minute 
)

// GetSessionContext 获取会话的上下文记录
func GetSessionContext(sessionID string) ([]openai.ChatCompletionMessage, error) {
	redisKey := ContextCachePrefix + sessionID
	var messages []openai.ChatCompletionMessage

	// 1. 尝试从 Redis 内存中拉取 (LRANGE 复杂度 O(S+N)，极快)
	records, err := global.RedisClient.LRange(global.Ctx, redisKey, 0, -1).Result()
	
	if err == nil && len(records) > 0 {
		// Cache Hit: 缓存命中，反序列化后直接返回
		for _, record := range records {
			var msg openai.ChatCompletionMessage
			_ = json.Unmarshal([]byte(record), &msg)
			messages = append(messages, msg)
		}
		// 用户有新操作，刷新该 Session 的过期时间
		global.RedisClient.Expire(global.Ctx, redisKey, ContextExpire)
		return messages, nil
	}

	// 2. Cache Miss: 缓存穿透，回源到 MySQL 进行兜底查询
	var history []models.ChatHistory
	// 查出最近的 10 条记录，注意要按时间倒序查，再在内存里反转，或者用子查询
	err = global.DB.Where("session_id = ? AND role IN ('user', 'assistant')", sessionID).
		Order("created_at desc").
		Limit(MaxContextKeep).
		Find(&history).Error

	if err != nil || len(history) == 0 {
		return messages, nil // 数据库里也没有，说明是纯新会话
	}

	// 3. 数据格式转换：从 MySQL 格式转为 OpenAI 格式
	// 因为查出来是倒序的（最新的在前面），组装给大模型时需要正序（旧的在前面）
	for i := len(history) - 1; i >= 0; i-- {
		msg := openai.ChatCompletionMessage{
			Role:    history[i].Role,
			Content: history[i].Content,
		}
		messages = append(messages, msg)
		
		// 4. 顺手将数据回写（Warm Up）到 Redis，下次请求就能命中缓存了
		val, _ := json.Marshal(msg)
		global.RedisClient.RPush(global.Ctx, redisKey, val)
	}
	
	global.RedisClient.Expire(global.Ctx, redisKey, ContextExpire)

	return messages, nil
}

// AppendToSessionContext 将新产生的一轮问答追加进 Redis 缓存
func AppendToSessionContext(sessionID string, userMsg, assistantMsg openai.ChatCompletionMessage) {
	redisKey := ContextCachePrefix + sessionID

	// 序列化消息
	userVal, _ := json.Marshal(userMsg)
	assistantVal, _ := json.Marshal(assistantMsg)

	// 利用 Pipeline 批量执行，减少一次网络 RTT 耗时
	pipe := global.RedisClient.Pipeline()
	
	// 1. 将最新的问答追加到列表尾部
	pipe.RPush(global.Ctx, redisKey, userVal, assistantVal)
	// 2. 裁剪列表：只保留末尾的 MaxContextKeep 条记录（滑动窗口原理）
	// LTRIM -10 -1 表示保留倒数第 10 个到倒数第 1 个元素
	pipe.LTrim(global.Ctx, redisKey, int64(-MaxContextKeep), -1)
	// 3. 重置过期时间
	pipe.Expire(global.Ctx, redisKey, ContextExpire)

	// 提交执行
	_, err := pipe.Exec(global.Ctx)
	if err != nil {
		// Redis 写入失败仅打印日志，不应该阻断主流程，因为数据已经持久化到 MySQL 了
		println("Redis 更新上下文缓存失败:", err.Error())
	}
}
```


