**Agentic RAG 个人 Markdown 智能知识库 ** 

一、表设计 （1表设计.md）

1. 文档元数据表：`documents`
2. 对话历史表：`chat_history`
3. 会话管理表：`chat_sessions`

二、Mysql redis 初始化与链接，Config文档（2Init.md）

1. 配置文件 (`config.yaml`)
2. // InitConfig 读取 YAML 配置文件
3. // InitMySQL 初始化 MySQL 连接池并自动迁移表结构
4. // InitRedis 初始化 Redis 连接
5. 接入项目入口 (`main.go`)

三、搭建Gin路由 （3Gin路由.md）

​	Gin 路由中 存在如下几个主要部分 ，这部分我们只需要建立相关的接口，不需要进行实现

​	1.基础聊天功能（与智能体对话询问功能）

​	2.历史聊天记录显示（在聊天界面只需要显示会话列表的对话标题，这里应当按照时间（最后活跃时间）进行排序）

​	3.会话列表更换，切换进入之前的会话列表，

​	4.文档上传功能（通过在线上传文档）

​	5.文档删除功能，例如我们更改了文档中的部分知识，此时需要对文档进行重传，操作步骤应当是，先删除文档，然后重新上传

四、文档操作接口部分实现（4文档操作.md）

​	// UploadDocumentHandler 处理 Markdown 文件上传与解析

​	// DeleteDocumentHandler 处理文档的物理删除与逻辑删除

五、聊天功能接口（5聊天操作.md）

1. `listSessionsHandler` (获取左侧会话列表)
2. `getSessionHistoryHandler` (获取某个会话的历史记录)
3. `chatMessageHandler` (发送消息与 Agent 思考入口)

六、srevice层，具体的大模型调用以及RAG 搜索整合等（6service.md）

​	通过一个 `for` 循环，不断地**思考 -> 决定调用工具 -> Go 后台执行工具 -> 拿着结果继续思考**，直到收集够信息才给出最终回答。

七、大模型客服端（7大模型API调用.md）

​	负责 大模型 API 调用，在普通输出时采用非流式输出，在最终给用户输出时采用流式输出

八、Redis使用（8redis.md）

​	// InitRedis 初始化 Redis 

​	// RateLimiter 基于 Redis 的单 IP 限流中间件

​	// GetSessionContext 获取会话的上下文记录

​	**API 限流防刷（Rate Limiting） 和  对话上下文的毫秒级缓存（Session Cache）**。

九、前端


