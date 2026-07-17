# 二、MySQL 与 Redis 初始化及 Config 配置 (2数据库与配置.md)

本部分主要负责解析项目配置文件（YAML），并建立与 MySQL 和 Redis 的全局长连接。我们将代码分为三个核心包：`config`（配置结构体）、`global`（全局变量）和 `initialize`（初始化逻辑）。

### 1. 配置文件 (`config.yaml`)

在项目根目录下创建一个 `config.yaml`。将容易变动的参数（如数据库密码、大模型 API Key）抽离出来，方便日后部署到阿里云或在 WSL 环境下切换配置。

YAML

```
server:
  port: 8080
  mode: "debug" # debug 或 release

mysql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/agentic_rag?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  pool_size: 10

llm:
  api_key: "sk-your-llm-api-key"
  model: "gpt-4o"
```

### 2. 全局变量与配置模型 (`global/global.go` & `config/config.go`)

为了让系统各个模块（比如后方的 Service 层和 Middleware 层）都能随叫随到地使用数据库连接和配置参数，我们定义全局变量。

**`config/config.go` (映射 YAML 结构)**

Go

```
package config

type Server struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type MySQL struct {
	DSN          string `mapstructure:"dsn"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type LLM struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

type AppConfig struct {
	Server Server `mapstructure:"server"`
	MySQL  MySQL  `mapstructure:"mysql"`
	Redis  Redis  `mapstructure:"redis"`
	LLM    LLM    `mapstructure:"llm"`
}
```

**`global/global.go` (定义全局单例)**

Go

```
package global

import (
	"context"
	"your_project/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	Config *config.AppConfig // 全局配置
	DB     *gorm.DB          // MySQL 全局实例
	Redis  *redis.Client     // Redis 全局实例
	Ctx    = context.Background()
)
```

### 3. 初始化逻辑 (`initialize/init.go`)

引入 Viper 读取配置，并根据配置实例化 GORM 和 Redis。这里我们还顺手加上了 GORM 的 `AutoMigrate`，它能自动根据你在“第一部分”设计的模型在 MySQL 中建表。

Go

```
package initialize

import (
	"fmt"
	"log"

	"your_project/global"
	"your_project/models" // 引入第一部分建好的表结构

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"
)

// InitConfig 读取 YAML 配置文件
func InitConfig() {
	v := viper.New()
	v.SetConfigFile("config.yaml") // 指定配置文件路径
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

	// 自动迁移表结构（对应第一部分的表设计）
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

	global.Redis = client
	fmt.Println("✅ Redis 连接成功")
}

// InitApp 统一暴露给 main.go 调用的入口函数
func InitApp() {
	InitConfig()
	InitMySQL()
	InitRedis()
}
```

### 4. 接入项目入口 (`main.go`)

最后，在你的主程序启动时，第一件事就是调通这些底层基建。

Go

```
package main

import (
	"fmt"
	"your_project/global"
	"your_project/initialize"
	"your_project/router" // 你的第三部分路由
)

func main() {
	// 1. 初始化配置与数据库
	initialize.InitApp()

	// 2. 挂载 Gin 路由
	r := router.SetupRouter()

	// 3. 启动服务
	port := fmt.Sprintf(":%d", global.Config.Server.Port)
	fmt.Printf("🚀 Agentic RAG 知识库服务启动，监听端口 %s\n", port)
	if err := r.Run(port); err != nil {
		panic(fmt.Sprintf("服务启动失败: %v", err))
	}
}
```