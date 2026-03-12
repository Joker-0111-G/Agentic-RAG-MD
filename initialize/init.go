package initialize

import (
	"fmt"
	"log"

	"Agentic-RAG-MD/global"
	"Agentic-RAG-MD/models"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"
)

// InitConfig 读取 YAML 配置文件
func InitConfig() {
	v := viper.New()
	v.SetConfigFile("config.yaml")
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	if err := v.Unmarshal(&global.Config); err != nil {
		log.Fatalf("配置映射失败: %v", err)
	}
	fmt.Println("✅ 配置文件加载成功")
}

// InitMySQL 初始化 MySQL 连接池并自动迁移表结构
func InitMySQL() {
	m := global.Config.MySQL
	db, err := gorm.Open(mysql.Open(m.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("MySQL 连接失败: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(m.MaxIdleConns)
	sqlDB.SetMaxOpenConns(m.MaxOpenConns)

	global.DB = db

	// 自动迁移表结构
	err = global.DB.AutoMigrate(
		&models.Document{},
		&models.ChatHistory{},
		&models.ChatSession{},
	)
	if err != nil {
		log.Fatalf("表结构迁移失败: %v", err)
	}

	fmt.Println("✅ MySQL 连接成功并完成表结构迁移")
}

// InitRedis 初始化 Redis 连接
func InitRedis() {
	r := global.Config.Redis
	client := redis.NewClient(&redis.Options{
		Addr:     r.Addr,
		Password: r.Password,
		DB:       r.DB,
		PoolSize: r.PoolSize,
	})

	// 测试连接
	_, err := client.Ping(global.Ctx).Result()
	if err != nil {
		log.Fatalf("Redis 连接失败: %v", err)
	}

	global.RedisClient = client
	fmt.Println("✅ Redis 连接成功")
}

// InitApp 统一暴露给 main.go 调用的入口函数
func InitApp() {
	InitConfig()
	InitMySQL()
	InitRedis()
}