### 1. 封装大模型客户端 (`services/llm_client.go`)

我们新建一个文件，专门负责与大模型 API 打交道。它向上层暴露出两个极其干净的方法：一个用于**普通的工具调用（非流式）**，一个用于**最终答案的生成（流式）**。

Go

```
package services

import (
	"context"
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"
)

// LLMClient 封装大模型客户端
type LLMClient struct {
	client *openai.Client
	model  string
}

// NewLLMClient 初始化并返回一个大模型客户端实例
func NewLLMClient(apiKey string) *LLMClient {
	// 这里可以根据需要替换 BaseURL 来支持国内大模型（如通义千问、DeepSeek）
	config := openai.DefaultConfig(apiKey)
	// config.BaseURL = "https://api.deepseek.com/v1" 

	return &LLMClient{
		client: openai.NewClientWithConfig(config),
		model:  openai.GPT4o, // 默认使用的模型
	}
}

// ChatWithTools 发起带有工具的对话请求 (非流式，用于让大模型思考和决定调用什么工具)
func (c *LLMClient) ChatWithTools(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (*openai.ChatCompletionMessage, error) {
	req := openai.ChatCompletionRequest{
		Model:      c.model,
		Messages:   messages,
		Tools:      tools,
		ToolChoice: "auto",
		Stream:     false,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("大模型返回结果为空")
	}

	return &resp.Choices[0].Message, nil
}

// ChatStream 发起纯文本流式对话请求 (用于输出最终自然语言回答)
func (c *LLMClient) ChatStream(ctx context.Context, messages []openai.ChatCompletionMessage, contentChan chan<- string) error {
	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		chunk := response.Choices[0].Delta.Content
		if chunk != "" {
			contentChan <- chunk
		}
	}
	return nil
}
```