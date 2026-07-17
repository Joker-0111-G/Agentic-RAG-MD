Gin 路由中 存在如下几个主要部分 ，这部分我们只需要建立相关的接口，不需要进行实现

1.基础聊天功能（与智能体对话询问功能）

2.历史聊天记录显示（在聊天界面只需要显示会话列表的对话标题，这里应当按照时间（最后活跃时间）进行排序）

3.会话列表更换，切换进入之前的会话列表，

4.文档上传功能（通过在线上传文档）

5.文档删除功能，例如我们更改了文档中的部分知识，此时需要对文档进行重传，操作步骤应当是，先删除文档，然后重新上传



# 二、搭建 Gin 路由 （2Gin路由.md）

在本阶段，我们仅搭建 API 路由骨架并定义好接口规范，暂不编写具体的业务处理逻辑。采用 RESTful 风格，将功能模块划分为 `chat`（对话管理）和 `documents`（文档管理）两个组。

### 1. 路由骨架代码 (`router/router.go`)

Go

```
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetupRouter 初始化并配置 Gin 路由
func SetupRouter() *gin.Engine {
	// 使用默认中间件（包含 Logger 和 Recovery）
	r := gin.Default()

	// 统一 API 版本前缀
	v1 := r.Group("/api/v1")
	{
		// ==========================================
		// 模块一：会话与聊天模块 (Chat & Sessions)
		// ==========================================
		chatGroup := v1.Group("/chat")
		// ==========================================
		// 对话模块：挂载 Redis 限流中间件 (每分钟最多 10 次)
		// ==========================================
		// 应用限流中间件
		chatGroup.Use(middleware.RateLimiter(10, time.Minute)) 
		{
			// 1. 基础聊天功能 (与智能体对话)
			// POST /api/v1/chat/message
			// 接收参数：session_id (可选，为空则新建会话), content (用户提问内容)
			// 返回结果：Agent 的流式或完整回答
			chatGroup.POST("/message", chatMessageHandler)

			// 2. 历史聊天记录显示 (会话列表)
			// GET /api/v1/chat/sessions
			// 作用：拉取左侧的对话标题列表，按 updated_at 倒序排列 (活跃的在最前)
			chatGroup.GET("/sessions", listSessionsHandler)

			// 3. 会话列表更换 (获取特定会话的历史记录)
			// GET /api/v1/chat/sessions/:session_id/history
			// 作用：用户点击侧边栏某个会话时，拉取该会话下的所有历史问答
			chatGroup.GET("/sessions/:session_id/history", getSessionHistoryHandler)
		}

		// ==========================================
		// 模块二：文档知识库模块 (Documents)
		// ==========================================
		docGroup := v1.Group("/documents")
		{
			// 4. 文档上传功能
			// POST /api/v1/documents/upload
			// 接收参数：file (Markdown文件流)
			// 作用：保存文件，解析元数据入库，并可触发后续的分块与向量化操作
			docGroup.POST("/upload", uploadDocumentHandler)

			// 5. 文档删除功能
			// DELETE /api/v1/documents/:id
			// 作用：根据文档 ID 删除数据库记录以及本地物理文件，为重新上传铺平道路
			docGroup.DELETE("/:id", deleteDocumentHandler)
		}
		
	}

	return r
}

// ==========================================
// 占位 Handler 定义 (待后续实现具体逻辑)
// ==========================================

func chatMessageHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "聊天接口待实现"})
}

func listSessionsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "获取会话列表接口待实现"})
}

func getSessionHistoryHandler(c *gin.Context) {
	// 获取路由参数中的 session_id
	// sessionID := c.Param("session_id")
	c.JSON(http.StatusOK, gin.H{"message": "获取会话历史记录接口待实现"})
}

func uploadDocumentHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "文档上传接口待实现"})
}

func deleteDocumentHandler(c *gin.Context) {
	// 获取路由参数中的文档 ID
	// docID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"message": "文档删除接口待实现"})
}
```

### 2. 接口设计说明

- **RESTful 规范**：使用 `POST` 表示创建（发消息、上传），`GET` 表示查询（列表、历史），`DELETE` 表示删除。这样的设计会让你的 API 文档显得非常专业。
- **路由参数**：在删除文档 (`/:id`) 和获取特定会话历史 (`/:session_id/history`) 时，使用了 Gin 的动态路由参数提取功能，这直接对应了我们在上一阶段设计的数据库表的主键。
- **解耦设计**：我们将路由注册（`SetupRouter`）和逻辑处理（`Handler`）分开。后续开发时，你只需要在对应的 `Handler` 函数里编写 GORM 查库代码和调用大模型的逻辑即可，路由文件不需要再频繁修改。
