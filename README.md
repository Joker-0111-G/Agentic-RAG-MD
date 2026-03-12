**Agentic RAG 个人 Markdown 智能知识库 ** 

**技术栈**：Golang, Gin, GORM, MySQL, Redis, LLM Function Calling 

**项目描述**：独立设计开发的代理式检索增强生成（Agentic RAG）后端服务，支持解析本地 Markdown 笔记并提供具备复杂意图识别的对话问答能力。

 **核心亮点**：

- **Agentic 架构落地**：摒弃传统单向检索，基于大模型 Function Calling 实现了带有“自我规划”能力的 Agent 循环。动态路由查询意图，显著提升了跨文档搜索与时间维度查询的准确率。
- **后端 API 与数据建模**：使用 Gin 构建高效 RESTful 接口；通过 MySQL + GORM 管理文档元数据与用户状态；设计了 `query_metadata` 等 Go 语言原生工具函数供 Agent 动态调度。
- **高性能会话与限流**：引入 Redis 实现多轮对话上下文的毫秒级存取，并设计了基于滑动窗口的接口限流（Rate Limit）机制，保障系统稳定性。

# 为你量身定制的 Agentic Markdown 助手项目设计方案

## 1. 核心理念：赋予大模型“使用工具”的能力

在普通的 RAG 中，用户提问后，系统固定去检索一次文档。 在 Agentic RAG 中，我们给大模型提供几个“Go 语言写的函数（工具）”，让它自己决定什么时候查数据库，什么时候查文档内容。

我们可以设计两个最简单的核心工具（Tools）：

### 工具 A：`query_metadata(condition)`

- **作用：** 去 MySQL 里面查文档的元数据。
- **场景：** 当用户问“我上个月写了哪些关于并发的笔记？”时，Agent 会调用这个工具，通过 GORM 去 MySQL 查 `documents` 表的时间和标题，而不是去全文检索。

### 工具 B：`search_markdown_content(query)`

- **作用：** 检索 Markdown 文件的具体内容（可以使用轻量级向量检索，或者简单的全文匹配）。
- **场景：** 当用户问“这篇笔记里提到的 Channel 死锁怎么解决？”时，Agent 会调用这个工具去提取文本段落。

------

## 2. 技术栈与业务逻辑完美融合

在这个项目中，你熟悉的后端组件将发挥各自的强项：

- **Gin 框架 (RESTful API)：**
  - 提供 `/upload` 接口：接收 Markdown 文件，解析出标题、标签（可以利用正则提取 Markdown 的 YAML Front Matter），并触发内容的分块（Chunking）。
  - 提供 `/chat` 接口：接收用户的提问，启动 Agent 思考循环。
- **MySQL + GORM (结构化数据)：**
  - **表结构设计非常简单**：一张 `documents` 表（存文件名、上传时间、文件路径等）；一张 `chat_history` 表（持久化保存聊天记录，方便以后查看）。
  - Agent 调用 `query_metadata` 工具时，本质上就是触发了一次 GORM 的查询。
- **Redis (缓存与上下文态)：**
  - **上下文管理**：Agent 在多轮对话中需要“记忆”。每次请求大模型时，通过 Redis 快速读取当前 Session 的最近 5 条对话记录拼接进去，速度极快。
  - **API 限流防刷**：利用 Redis 做简单的 Rate Limiter，限制同一个 IP 一分钟内只能调用 10 次 `/chat` 接口，这在简历中是一个非常亮眼的工程细节。

------

## 3. 一个典型的 Agentic 运行流程（面试最爱问的场景）

**假设用户提问：“总结一下我昨天写的关于 Goroutine 的笔记。”**

1. **接收请求：** Gin 接收到请求，去 Redis 拿到历史上下文，发给大模型。
2. **Agent 思考 (Iteration 1)：** “我需要先知道昨天有哪些笔记。我要调用 `query_metadata` 工具，参数是 `date: yesterday, keyword: Goroutine`。”
3. **Go 后端拦截：** 你的 Go 代码发现大模型请求调用工具，于是执行 GORM 查询，查到 MySQL 中有一条记录：`id=42, title=goroutine_guide.md`。并将这个结果返回给大模型。
4. **Agent 思考 (Iteration 2)：** “我知道是哪篇笔记了，现在我要看内容。我调用 `search_markdown_content` 工具，参数是 `document_id: 42`。”
5. **Go 后端拦截：** 你的代码读取该 Markdown 文件的内容并返回。
6. **Agent 思考 (Iteration 3)：** “信息收集完毕，我开始总结……” 最后生成回答。
7. **返回结果：** Gin 将最终答案返回给前端，并异步将这次对话存入 MySQL。

------
