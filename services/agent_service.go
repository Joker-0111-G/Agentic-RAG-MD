package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"Agentic-RAG-MD/global"
	"Agentic-RAG-MD/models"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type StreamMessage struct {
	Event string
	Data  string
}

func RunAgentLoop(sessionID string, userContent string, streamChan chan<- StreamMessage) {
	defer close(streamChan)
	ctx := context.Background()

	llm := NewLLMClient(global.Config.LLM.APIKey)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "你是一个专门管理和检索用户 Markdown 个人笔记的智能助手。你可以使用工具查询数据库并读取文件。请根据工具返回的信息，准确回答用户的问题。",
		},
	}

	historyMsgs, _ := GetSessionContext(sessionID)
	messages = append(messages, historyMsgs...)

	currentUserMsg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: userContent}
	messages = append(messages, currentUserMsg)

	const maxIterations = 5
	for i := 0; i < maxIterations; i++ {
		msg, err := llm.ChatWithTools(ctx, messages, getAgentTools())
		if err != nil {
			streamChan <- StreamMessage{Event: "error", Data: "思考过程发生错误: " + err.Error()}
			return
		}

		if len(msg.ToolCalls) > 0 {
			messages = append(messages, *msg) 
			for _, toolCall := range msg.ToolCalls {
				functionName := toolCall.Function.Name
				args := toolCall.Function.Arguments

				streamChan <- StreamMessage{Event: "status", Data: fmt.Sprintf("🛠️ 正在执行工具: %s...", functionName)}
				toolResult := executeLocalTool(functionName, args)

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    toolResult,
					Name:       functionName,
					ToolCallID: toolCall.ID,
				})
				streamChan <- StreamMessage{Event: "status", Data: fmt.Sprintf("✅ 工具 %s 执行完毕，分析结果中...", functionName)}
			}
			continue 
		}

		streamChan <- StreamMessage{Event: "status", Data: "💡 思考完毕，开始生成回答..."}
		
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

		assistantMsg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: finalAnswer}

		go func() {
			AppendToSessionContext(sessionID, currentUserMsg, assistantMsg)
			
			// 补充历史记录持久化入库逻辑
			global.DB.Create(&models.ChatHistory{SessionID: sessionID, Role: "user", Content: userContent})
			global.DB.Create(&models.ChatHistory{SessionID: sessionID, Role: "assistant", Content: finalAnswer})
		}()

		streamChan <- StreamMessage{Event: "done", Data: "[DONE]"}
		return
	}

	streamChan <- StreamMessage{Event: "error", Data: "Agent 思考次数超限，未能得出结论"}
}

func executeLocalTool(functionName, args string) string {
	switch functionName {
	case "query_metadata":
		return executeQueryMetadata(args) 
	case "search_markdown_content":
		return executeSearchMarkdownContent(args) 
	default:
		return fmt.Sprintf("未知的工具: %s", functionName)
	}
}

func getAgentTools() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "query_metadata",
				Description: "去数据库中查询文档的元数据。当用户询问关于某段时间、某些标签、或某个标题的笔记有哪些时调用此工具。",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tags": {Type: jsonschema.String, Description: "文档标签，例如 'golang', '并发'。"},
						"title_keyword": {Type: jsonschema.String, Description: "标题中包含的关键词。"},
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
						"document_id": {Type: jsonschema.Integer, Description: "在数据库中查到的文档 ID。"},
					},
					Required: []string{"document_id"},
				},
			},
		},
	}
}

func executeQueryMetadata(args string) string {
	var params struct {
		Tags         string `json:"tags"`
		TitleKeyword string `json:"title_keyword"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "工具调用失败：参数解析错误 - " + err.Error()
	}

	var docs []models.Document
	query := global.DB.Model(&models.Document{})

	if params.Tags != "" {
		query = query.Where("tags LIKE ?", "%"+params.Tags+"%")
	}
	if params.TitleKeyword != "" {
		query = query.Where("title LIKE ?", "%"+params.TitleKeyword+"%")
	}

	err := query.Select("id, title, tags, created_at, word_count").Limit(20).Find(&docs).Error
	if err != nil { return "工具调用失败：数据库查询错误 - " + err.Error() }
	if len(docs) == 0 { return "数据库中未找到符合该条件的文档记录。" }

	resultBytes, _ := json.Marshal(docs)
	return string(resultBytes)
}

func executeSearchMarkdownContent(args string) string {
	var params struct {
		DocumentID int `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "工具调用失败：参数解析错误 - " + err.Error()
	}

	var doc models.Document
	if err := global.DB.Select("id, file_path, title").First(&doc, params.DocumentID).Error; err != nil {
		return fmt.Sprintf("工具调用失败：未在数据库中找到 ID 为 %d 的文档", params.DocumentID)
	}
	if doc.FilePath == "" { return "工具调用失败：该文档缺少有效的文件路径" }

	contentBytes, err := os.ReadFile(doc.FilePath)
	if err != nil { return "工具调用失败：读取本地物理文件错误 - " + err.Error() }

	contentStr := string(contentBytes)
	const maxLen = 15000 
	if len(contentStr) > maxLen {
		contentStr = contentStr[:maxLen] + "\n\n...(内容过长，已截断)..."
	}
	return fmt.Sprintf("【文件 %s (ID:%d) 的内容如下】:\n%s", doc.Title, doc.ID, contentStr)
}