package services

import (
	"context"
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"
)

type LLMClient struct {
	client *openai.Client
	model  string
}

func NewLLMClient(apiKey string) *LLMClient {
	config := openai.DefaultConfig(apiKey)
	return &LLMClient{
		client: openai.NewClientWithConfig(config),
		model:  openai.GPT4o,
	}
}

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