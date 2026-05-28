# Agentic RAG · 个人 Markdown 智能知识库

> **让大模型学会"使用工具"——基于 Function Calling 的 Agentic RAG 后端服务**

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" />
  <img src="https://img.shields.io/badge/Gin-1.12-009688?logo=gin" />
  <img src="https://img.shields.io/badge/GORM-1.31-1572B6" />
  <img src="https://img.shields.io/badge/MySQL-8.0-4479A1?logo=mysql" />
  <img src="https://img.shields.io/badge/Redis-7.0-DC382D?logo=redis" />
  <img src="https://img.shields.io/badge/LLM-GPT--4o-412991?logo=openai" />
  <img src="https://img.shields.io/badge/License-MIT-green" />
</p>

---

## 📋 目录

- [项目简介](#-项目简介)
- [核心架构](#-核心架构)
- [目录结构](#-目录结构)
- [快速开始](#-快速开始)
- [API 文档](#-api-文档)
- [技术深度解析](#-技术深度解析)
  - [Agentic 工作流](#1-agentic-工作流)
  - [二级缓存机制](#2-二级缓存机制)
  - [Redis 限流器](#3-redis-限流器)
  - [Markdown 解析引擎](#4-markdown-解析引擎)
- [典型运行流程](#-典型运行流程)
- [进阶路线](#-进阶路线)

---

## 🎯 项目简介

**Agentic RAG-MD** 是一个独立设计开发的 **Agentic RAG (Agentic Retrieval-Augmented Generation)** 后端服务，它**突破了传统 RAG 检索即回应的局限**，让大模型具备"自主规划、多步推理、调用工具"的能力。

### 核心能力

| 能力 | 说明 |
|------|------|
| **📝 Markdown 笔记管理** | 上传、解析、存储 Markdown 文件，自动提取 YAML Front Matter |
| **🧠 Agentic 智能问答** | 基于 LLM Function Calling 实现 ReAct 循环，自主决策查询路径 |
| **🔧 工具调用** | LLM 可自主调用 `query_metadata` / `search_markdown_content` 工具 |
| **⚡ 流式响应** | SSE (Server-Sent Events) 实时推送 Agent 思考状态和最终回答 |
| **💬 多轮对话** | Redis + MySQL 二级缓存，保留最近 10 轮对话上下文 |
| **🔒 接口限流** | Redis 固定窗口限流，保障 API 安全 |

---

## 🏗️ 核心架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Agentic RAG System                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  用户 ─── POST /chat/message ──→  Gin Router                         │
│                                      │                              │
│                                  Rate Limiter                        │
│                                  (Redis INCR + EXPIRE)              │
│                                      │                              │
│                                      ▼                              │
│                            ChatMessageHandler                         │
│                                      │                              │
│                           ┌──────────┴──────────┐                    │
│                           ▼                     ▼                    │
│                    GetSessionContext      RunAgentLoop                │
│                    (Redis → MySQL)       (Function Calling)          │
│                           │                     │                    │
│                           ▼                     ▼                    │
│                    ┌──────────────┐     ┌────────────────┐           │
│                    │ Redis List   │     │ LLM Client     │           │
│                    │ (滑动窗口)     │     │ (go-openai)     │           │
│                    │              │     │                │           │
│                    │ MySQL        │     │ Tool Calling   │           │
│                    │ (持久化冷数据)  │     │   ├─query_metadata     │
│                    └──────────────┘     │   └─search_md_content    │
│                                         └────────────────┘           │
│                                                  │                   │
│                                                  ▼                   │
│                                         ┌────────────────┐           │
│                                         │ SSE Stream     │           │
│                                         │ → status       │           │
│                                         │ → content      │           │
│                                         │ → done         │           │
│                                         └────────────────┘           │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  存储层                                                      │    │
│  │  ├─ MySQL:  documents / chat_history / chat_sessions          │    │
│  │  └─ Redis:  chat_context:{session_id} (List)                 │    │
│  │             rate_limit:{api}:{ip}     (String)               │    │
│  └──────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 📁 目录结构

```
Agentic-RAG-MD/
├── main.go                 # 服务入口：初始化 → 路由 → 启动
├── config.yaml             # 配置文件（MySQL / Redis / LLM）
├── go.mod / go.sum         # Go 依赖管理
│
├── config/                 # 配置结构体定义
│   └── config.go           # Server / MySQL / Redis / LLM 配置模型
│
├── global/                 # 全局单例
│   └── global.go           # Config / DB / RedisClient / Ctx
│
├── initialize/             # 启动初始化
│   └── init.go             # InitConfig / InitMySQL / InitRedis / InitApp
│
├── models/                 # 数据模型（GORM 表结构）
│   └── models.go           # Document / ChatHistory / ChatSession
│
├── middleware/              # Gin 中间件
│   └── rate_limit.go       # Redis 限流器（固定窗口）
│
├── handlers/               # HTTP 请求处理器
│   ├── chat_handler.go     # 聊天 / 会话 / 历史记录
│   └── document_handler.go # 文档上传 / 删除 / 解析
│
├── services/               # 核心业务逻辑
│   ├── agent_service.go    # Agent 循环 + 工具函数
│   ├── session_cache.go    # Redis 二级缓存
│   └── llm_client.go       # OpenAI API 封装
│
├── router/                 # Gin 路由配置
│   └── router.go           # 路由分组 + 中间件挂载
│
└── uploads/                # 上传文件存储目录（运行时生成）
    └── markdowns/          # 上传的 .md 文件
```

---

## 🚀 快速开始

### 前置条件

| 依赖 | 版本要求 | 用途 |
|------|---------|------|
| Go | 1.25+ | 运行环境 |
| MySQL | 8.0+ | 文档元数据 + 对话持久化 |
| Redis | 7.0+ | 会话缓存 + 限流器 |
| OpenAI API Key | 可用额度 | LLM 推理 |

### 1. 克隆 & 配置

```bash
git clone https://github.com/Joker-0111-G/Agentic-RAG-MD.git
cd Agentic-RAG-MD
```

编辑 `config.yaml`：

```yaml
server:
  port: 8080
  mode: "debug"

mysql:
  dsn: "root:yourpassword@tcp(127.0.0.1:3306)/agentic_rag?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  pool_size: 10

llm:
  api_key: "sk-your-openai-api-key"
  model: "gpt-4o"
```

### 2. 启动服务

```bash
# 数据库会自动创建表结构（AutoMigrate）
go run main.go
```

看到以下输出即为启动成功：

```
✅ 配置文件加载成功
✅ MySQL 连接成功并完成表结构迁移
✅ Redis 连接成功
✅ Agentic RAG 知识库服务启动，监听端口 :8080
```

### 3. 快速体验

```bash
# 上传一篇 Markdown 笔记
curl -X POST http://localhost:8080/api/v1/documents/upload \
  -F "file=@./test_note.md"

# 发起对话（SSE 流式响应）
curl -X POST http://localhost:8080/api/v1/chat/message \
  -H "Content-Type: application/json" \
  -d '{"session_id":"","content":"总结一下我关于Goroutine的笔记"}'
```

---

## 📖 API 文档

### 文档管理

#### `POST /api/v1/documents/upload` — 上传 Markdown

上传 Markdown 文件，自动解析 YAML Front Matter。

**请求：** `multipart/form-data`

```
file: 要上传的 .md 文件
```

**响应：**
```json
{
  "message": "文档上传并解析成功",
  "data": {
    "ID": 1,
    "FileName": "goroutine_deep_dive.md",
    "Title": "Goroutine 深入理解",
    "FilePath": "./uploads/markdowns/1680000000_goroutine_deep_dive.md",
    "Tags": "golang,并发,goroutine",
    "WordCount": 2480,
    "CreatedAt": "2026-05-28T15:00:00+08:00"
  }
}
```

#### `DELETE /api/v1/documents/:id` — 删除文档

删除文档记录及对应的本地文件。

### 对话管理

#### `POST /api/v1/chat/message` — 发送消息

**请求体：**
```json
{
  "session_id": "uuid-string-or-empty-for-new",
  "content": "我上个月写了哪些关于并发的笔记？"
}
```

**响应（SSE 流式）：**
```
event: meta
data: {"session_id":"a1b2c3d4-..."}

event: status
data: 🛠️ 正在执行工具: query_metadata...

event: status
data: ✅ 工具 query_metadata 执行完毕，分析结果中...

event: status
data: 💡 思考完毕，开始生成回答...

event: content
data: 你在上个月写了......

event: done
data: [DONE]
```

#### `GET /api/v1/chat/sessions` — 会话列表

获取所有历史会话。

#### `GET /api/v1/chat/sessions/:session_id/history` — 会话历史

获取指定会话的完整对话记录。

---

## 🔧 技术深度解析

### 1. Agentic 工作流

这是项目的核心创新——**让 LLM 拥有"工具使用"的能力**。

#### ReAct 循环

```
用户提问："总结一下我昨天写的关于 Goroutine 的笔记"
                       │
                       ▼
          ┌─────────────────────────┐
          │  Step 1: LLM 思考       │
          │  "我需要查昨天有什么笔记" │
          │  决定调用 query_metadata  │
          └─────────┬───────────────┘
                    │ Tool Call
                    ▼
          ┌─────────────────────────┐
          │  Step 2: Go 执行工具    │
          │  GORM → MySQL 查询      │
          │  返回: 昨天有1篇笔记     │
          │  id=42, title=goroutine │
          └─────────┬───────────────┘
                    │ Tool Result
                    ▼
          ┌─────────────────────────┐
          │  Step 3: LLM 再次思考   │
          │  "找到笔记了，看内容"     │
          │  调用 search_md_content  │
          └─────────┬───────────────┘
                    │ Tool Call
                    ▼
          ┌─────────────────────────┐
          │  Step 4: Go 读文件      │
          │  os.ReadFile → 返回内容  │
          └─────────┬───────────────┘
                    │ Tool Result
                    ▼
          ┌─────────────────────────┐
          │  Step 5: LLM 总结回答   │
          │  "这篇笔记主要讲..."     │
          │  SSE 流式输出最终答案    │
          └─────────────────────────┘
```

**代码实现在 `services/agent_service.go` 的 `RunAgentLoop` 中：**

```go
const maxIterations = 5
for i := 0; i < maxIterations; i++ {
    // 1. LLM 决定：继续思考 or 调用工具 or 生成回答
    msg, err := llm.ChatWithTools(ctx, messages, getAgentTools())

    // 2. 如果需要工具 → 执行本地函数
    if len(msg.ToolCalls) > 0 {
        for _, toolCall := range msg.ToolCalls {
            result := executeLocalTool(toolCall.Function.Name, toolCall.Function.Arguments)
            messages = append(messages, toolResultMsg)
        }
        continue  // 回到 LLM 继续思考
    }

    // 3. 已收集足够信息 → SSE 流式输出最终答案
    llm.ChatStream(ctx, messages, contentChan)
}
```

#### 两个 Agent 工具

| 工具 | 触发场景 | Go 实现 |
|------|---------|---------|
| `query_metadata` | 用户问"有哪些关于 X 的笔记" | GORM 查询 documents 表 |
| `search_markdown_content` | 用户问"这篇笔记里具体讲了什么" | `os.ReadFile` 读取本地文件 |

### 2. 二级缓存机制

**设计目标：** 多轮对话中，LLM 需要"记住"历史对话，但每次请求都查 MySQL 太慢，全放 Redis 又可能丢数据。

#### 架构

```
读请求
    │
    ├─ Level 1: Redis List (chat_context:{session_id})
    │   ├─ 命中 → 反序列化 JSON 消息列表，返回 ✅（毫秒级）
    │   └─ 未命中 → 进入 Level 2
    │
    ├─ Level 2: MySQL (chat_history 表)
    │   ├─ 查询最近 10 条对话记录
    │   └─ 倒序读取后回填 Redis（预热）
    │
    └─ 返回上下文消息列表
```

**写路径**（每轮对话完成后）：

```go
// 使用 Pipeline 保证原子性
pipe.RPush(key, userMsg, assistantMsg)   // 追加新消息
pipe.LTrim(key, -10, -1)                 // 滑动窗口：保留最近 10 条
pipe.Expire(key, 30*time.Minute)         // 30 分钟无访问自动过期

// 同时持久化到 MySQL
global.DB.Create(&userChatHistory)
global.DB.Create(&assistantChatHistory)
```

**设计亮点：**
- **缓存命中时** → 毫秒级返回，不查数据库
- **缓存过期/未命中** → 从 MySQL 自动恢复，冷数据不丢
- **滑动窗口** → LTRIM 只保留 10 条，精确控制 Token 消耗
- **Pipeline** → 3 个操作一次网络往返，无需事务

### 3. Redis 限流器

**固定窗口限流算法，实现文件 `middleware/rate_limit.go`：**

```go
redisKey := fmt.Sprintf("rate_limit:%s:%s", c.FullPath(), clientIP)

count, _ := global.RedisClient.Incr(ctx, redisKey).Result()
if count == 1 {
    global.RedisClient.Expire(ctx, redisKey, time.Minute)
}
if count > maxRequests {
    c.JSON(429, gin.H{"error": "您的请求太频繁，请稍后再试"})
    c.Abort()
    return
}
```

| 参数 | 配置值 | 说明 |
|------|--------|------|
| `/api/v1/chat/message` | 10 次/分钟 | LLM API 成本较高，限制更严格 |
| 其他 API | 可自定义 | 通过 `middleware.RateLimiter(n, window)` 灵活配置 |

### 4. Markdown 解析引擎

**文档上传时自动解析 YAML Front Matter：**

```yaml
---
title: Goroutine 深入理解
tags: [golang, 并发, goroutine]
---

## 什么是 Goroutine
...
```

解析逻辑 `handlers/document_handler.go`：

```go
func parseMarkdown(filePath string) (title, tags string, wordCount int, err error) {
    content := string(os.ReadFile(filePath))
    wordCount = utf8.RuneCountInString(content)

    if strings.HasPrefix(content, "---\n") {
        parts := strings.SplitN(content, "---", 3)
        if len(parts) >= 3 {
            yaml.Unmarshal([]byte(parts[1]), &fm)
            // 提取 title, tags
        }
    }
}
```

---

## 🔄 典型运行流程

下面是一次完整的用户交互流程：

```
用户：上传笔记 → POST /documents/upload
  │
  ├─ Gin 接收文件
  ├─ 保存到 ./uploads/markdowns/
  ├─ 解析 YAML Front Matter
  ├─ 存入 MySQL documents 表
  └─ 返回文档元数据
                    ───── 过了一段时间 ─────
用户：提问 → POST /chat/message { "content": "我之前的Goroutine笔记讲了什么" }
  │
  ├─ Rate Limiter 检查（Redis INCR）
  ├─ 新会话 → UUID 自动生成
  ├─ GetSessionContext
  │   ├─ Redis 查询 → 未命中（新会话）
  │   └─ MySQL 查询 → 无历史数据
  │
  ├─ RunAgentLoop
  │   ├─ Iteration 1: LLM → 调用 query_metadata
  │   │   └─ GORM 查询 → {id:42, title:"goroutine_deep_dive"}
  │   │
  │   ├─ Iteration 2: LLM → 调用 search_markdown_content
  │   │   └─ os.ReadFile → 返回文件内容
  │   │
  │   └─ Iteration 3: LLM → 生成最终回答
  │       └─ SSE 流式输出
  │
  └─ AppendToSessionContext
      ├─ Redis: RPush + LTrim + Expire
      └─ MySQL: 插入 chat_history
```

---

## 🗺️ 进阶路线

该项目可作为探索 **Agentic RAG + MCP** 的起点，以下是几个可能的演进方向：

| 方向 | 描述 |
|------|------|
| **🔄 MCP Server 接入** | 将工具函数标准化为 MCP Server，支持热插拔 |
| **📊 前端可视化** | 搭配 Web UI，展示 Agent 思考链路和文档图谱 |
| **🔍 语义检索** | 引入 Embedding + 向量数据库，替代全文匹配 |
| **📦 Docker 化** | 提供 docker-compose 一键部署 |
| **🔐 多用户支持** | 基于 UserID 隔离文档和会话数据 |
| **📈 性能优化** | 接入 Prometheus + 链路追踪 |

---

## 🧰 技术栈

| 类别 | 技术选型 |
|------|---------|
| **语言** | Go 1.25+ |
| **Web 框架** | Gin 1.12 |
| **ORM** | GORM 1.31 |
| **数据库** | MySQL 8.0 |
| **缓存** | Redis 7.0 (go-redis) |
| **LLM SDK** | go-openai (sashabaranov) |
| **配置管理** | Viper |
| **序列化** | YAML v3 (gopkg) |
| **ID 生成** | Google UUID |

---

## 📄 许可证

[MIT License](LICENSE)

---

<p align="center">
  <sub>Built with ❤️ by <a href="https://github.com/Joker-0111-G">Joker-0111-G</a></sub>
</p>
