package global

import (
	"context"
	"Agentic-RAG-MD/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	Config      *config.AppConfig // 全局配置
	DB          *gorm.DB          // MySQL 全局实例
	RedisClient *redis.Client     // Redis 全局实例
	Ctx         = context.Background()
)