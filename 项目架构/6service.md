# 五、Service 层：Agentic 工作流与大模型整合 (5service.md)

本层的核心思想是 **“ReAct 循环 (Reason + Act)”**。大模型不再是一次性回答问题，而是通过一个 `for` 循环，不断地**思考 -> 决定调用工具 -> Go 后台执行工具 -> 拿着结果继续思考**，直到收集够信息才给出最终回答。

### 1. 核心 Service 代码 (`services/agent_service.go`)

Go

```
package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	// "your_project/global" // 用于查库
	// "your_project/models"
)

// StreamMessage 定义向前端 SSE 推送的数据结构
type StreamMessage struct {
	Event string
	Data  string
}

// RunAgentLoop 核心的 Agentic 工作流编排
func RunAgentLoop(sessionID string, userContent string, streamChan chan<- StreamMessage) {
	defer close(streamChan)
	ctx := context.Background()

	// 1. 初始化大模型客户端
	llm := NewLLMClient("YOUR_API_KEY")

	// ==========================================
	// ⚡️ 修改点 1：在这里接入 Redis，读取历史上下文
	// ==========================================
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "你是一个专门管理和检索用户 Markdown 个人笔记的智能助手。你可以使用工具查询数据库并读取文件。请根据工具返回的信息，准确回答用户的问题。",
		},
	}

	// 从 Redis 或 MySQL 加载此前的对话上下文 (假设 GetSessionContext 在同一个 package 下)
	historyMsgs, _ := GetSessionContext(sessionID)
	messages = append(messages, historyMsgs...)

	// 将用户当前的最新提问作为 message，并追加进去
	currentUserMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userContent,
	}
	messages = append(messages, currentUserMsg)
	// ==========================================

	const maxIterations = 5
	for i := 0; i < maxIterations; i++ {
		// 3. 询问大模型：当前信息够了吗？需要调用工具吗？
		msg, err := llm.ChatWithTools(ctx, messages, getAgentTools())
		if err != nil {
			streamChan <- StreamMessage{Event: "error", Data: "思考过程发生错误: " + err.Error()}
			return
		}

		// 场景 A：大模型决定调用工具
		if len(msg.ToolCalls) > 0 {
			messages = append(messages, *msg) // 将大模型的调用请求记入历史

			for _, toolCall := range msg.ToolCalls {
				functionName := toolCall.Function.Name
				args := toolCall.Function.Arguments

				streamChan <- StreamMessage{Event: "status", Data: fmt.Sprintf("🛠️ 正在执行工具: %s...", functionName)}

				// 执行本地 Go 函数
				toolResult := executeLocalTool(functionName, args)

				// 将工具执行结果作为 tool 角色塞回历史记录
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    toolResult,
					Name:       functionName,
					ToolCallID: toolCall.ID,
				})
				
				streamChan <- StreamMessage{Event: "status", Data: fmt.Sprintf("✅ 工具 %s 执行完毕，分析结果中...", functionName)}
			}
			continue // 带着工具结果，进入下一轮循环继续思考
		}

		// 场景 B：大模型决定直接输出答案
		streamChan <- StreamMessage{Event: "status", Data: "💡 思考完毕，开始生成回答..."}
		
		// 创建一个内部通道接收 LLM 的文本块，并将其转发到外层的 SSE 通道
		contentChan := make(chan string)
		go func() {
			defer close(contentChan)
			_ = llm.ChatStream(ctx, messages, contentChan)
		}()

		var finalAnswer string
		for chunk := range contentChan {
			finalAnswer += chunk
			streamChan <- StreamMessage{Event: "content", Data: chunk}
		}

		// ==========================================
		// ⚡️ 修改点 2：在得出最终答案后，写入 Redis 缓存和 MySQL
		// ==========================================
		assistantMsg := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: finalAnswer,
		}

		// 开启一个 Goroutine 异步处理存储，不阻塞推流的结束
		go func() {
			// 将这一轮完整的对话追加到 Redis 滑动窗口中
			AppendToSessionContext(sessionID, currentUserMsg, assistantMsg)
			
			// 持久化到 MySQL (如果你在 handler 层没有存的话，可以在这里存)
			// global.DB.Create(&models.ChatHistory{SessionID: sessionID, Role: "user", Content: userContent})
			// global.DB.Create(&models.ChatHistory{SessionID: sessionID, Role: "assistant", Content: finalAnswer})
		}()
		// ==========================================

		streamChan <- StreamMessage{Event: "done", Data: "[DONE]"}
		return
	}

	streamChan <- StreamMessage{Event: "error", Data: "Agent 思考次数超限，未能得出结论"}
}

// executeLocalTool 集中处理本地函数的路由分发
func executeLocalTool(functionName, args string) string {
	switch functionName {
	case "query_metadata":
		return executeQueryMetadata(args) // 调用之前写好的具体实现
	case "search_markdown_content":
		return executeSearchMarkdownContent(args) // 调用之前写好的具体实现
	default:
		return fmt.Sprintf("未知的工具: %s", functionName)
	}
}

// getAgentTools 返回大模型工具箱定义 (将之前庞大的 Schema 定义抽离成函数)
func getAgentTools() []openai.Tool {
	var agentTools = []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_metadata",
				Description: "去数据库中查询文档的元数据。当用户询问关于某段时间、某些标签、或某个标题的笔记有哪些时调用此工具。",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tags": {
							Type:        jsonschema.String,
							Description: "文档标签，例如 'golang', '并发', '数据库'。如果没有特定标签则为空。",
						},
						"title_keyword": {
							Type:        jsonschema.String,
							Description: "标题中包含的关键词。",
						},
					},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search_markdown_content",
				Description: "读取指定 Markdown 文件的完整内容。当你通过 query_metadata 知道要看哪个文件后，调用此工具获取正文内容以回答具体细节。",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"document_id": {
							Type:        jsonschema.Integer,
							Description: "在数据库中查到的文档 ID。",
						},
					},
					Required: []string{"document_id"},
				},
			},
		},
	}
	
	// ⚡️ 修改点 3：这里修正为你定义的 agentTools，而不是返回 nil
	return agentTools 
}

// 占位用的本地工具函数 (记得替换为实际包含 DB 和读文件的逻辑)
func executeQueryMetadata(args string) string {
	return "模拟查询结果..."
}

func executeSearchMarkdownContent(args string) string {
	return "模拟文档内容..."
}
```



### 1. `executeQueryMetadata` (精准查库)

这个函数负责解析大模型传来的 JSON 参数，然后利用 GORM 动态拼接 `WHERE` 条件。大模型非常擅长阅读 JSON，所以我们直接把查出来的结构体切片序列化成 JSON 字符串返回给它。

Go

```
import (
	"encoding/json"
	"fmt"
	"os"

	// "your_project/global"
	// "your_project/models"
)

// executeQueryMetadata 去 MySQL 里面查文档的元数据
func executeQueryMetadata(args string) string {
	// 1. 定义与大模型 Function Schema 对应的结构体并解析参数
	var params struct {
		Tags         string `json:"tags"`
		TitleKeyword string `json:"title_keyword"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "工具调用失败：参数解析错误 - " + err.Error()
	}

	// 2. 动态构建 GORM 查询
	var docs []models.Document
	query := global.DB.Model(&models.Document{})

	if params.Tags != "" {
		// 使用 LIKE 进行模糊匹配，实际生产中如果是 JSON 格式标签可以用 JSON 函数
		query = query.Where("tags LIKE ?", "%"+params.Tags+"%")
	}
	if params.TitleKeyword != "" {
		query = query.Where("title LIKE ?", "%"+params.TitleKeyword+"%")
	}

	// 3. 执行查询。注意：只 select 元数据字段，避免查出不必要的庞大数据
	err := query.Select("id, title, tags, created_at, word_count").Limit(20).Find(&docs).Error
	if err != nil {
		return "工具调用失败：数据库查询错误 - " + err.Error()
	}

	// 4. 处理查询结果
	if len(docs) == 0 {
		return "数据库中未找到符合该条件的文档记录。"
	}

	// 将查询结果直接序列化为 JSON 字符串返回给大模型
	// 大模型（尤其是 GPT-4o 或同等级别模型）阅读 JSON 格式数据的能力极强
	resultBytes, _ := json.Marshal(docs)
	return string(resultBytes)
}
```

### 2. `executeSearchMarkdownContent` (读取物理文件)

当大模型通过上面的函数拿到了 `id` 之后，它就会把 `id` 传给这个函数。这里的逻辑非常直白：先拿 `id` 去 MySQL 换取真实的文件路径（`FilePath`），然后用 Go 原生的 `os.ReadFile` 将整篇文章读出来喂给大模型。

Go

```
// executeSearchMarkdownContent 读取指定 Markdown 文件的具体内容
func executeSearchMarkdownContent(args string) string {
	// 1. 解析参数
	var params struct {
		DocumentID int `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "工具调用失败：参数解析错误 - " + err.Error()
	}

	// 2. 根据 ID 查询数据库记录，主要为了拿到 FilePath
	var doc models.Document
	if err := global.DB.Select("id, file_path, title").First(&doc, params.DocumentID).Error; err != nil {
		return fmt.Sprintf("工具调用失败：未在数据库中找到 ID 为 %d 的文档", params.DocumentID)
	}

	if doc.FilePath == "" {
		return "工具调用失败：该文档缺少有效的文件路径"
	}

	// 3. 读取本地物理文件内容
	contentBytes, err := os.ReadFile(doc.FilePath)
	if err != nil {
		// 容错处理：记录在数据库里，但物理文件可能被删除了
		return "工具调用失败：读取本地物理文件错误 - " + err.Error()
	}

	contentStr := string(contentBytes)

	// 4. （可选进阶）防止单篇文章过大撑爆大模型 Token 上限
	// 假设我们限制最多读取前 15000 个字符
	const maxLen = 15000 
	if len(contentStr) > maxLen {
		contentStr = contentStr[:maxLen] + "\n\n...(内容过长，已截断)..."
	}

	return fmt.Sprintf("【文件 %s (ID:%d) 的内容如下】:\n%s", doc.Title, doc.ID, contentStr)
}
```