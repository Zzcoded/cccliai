package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cccliai/app/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

type DeepSeekProvider struct {
	client *openai.Client
	model  string
}

func NewDeepSeekProvider(apiKey string, model string) *DeepSeekProvider {
	if model == "" {
		model = "deepseek-chat" // Default model mapping
	}

	config := openai.DefaultConfig(apiKey)
	// DeepSeek operates on an OpenAI-compatible API endpoint
	config.BaseURL = "https://api.deepseek.com/v1"
	config.HTTPClient = &http.Client{Timeout: 60 * time.Second}

	return &DeepSeekProvider{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

func (p *DeepSeekProvider) ChatCompletion(ctx context.Context, msgs []agent.Message) (*agent.Response, error) {
	// Convert agent.Message to openai.ChatCompletionMessage
	openAiMsgs := make([]openai.ChatCompletionMessage, len(msgs))
	for i, m := range msgs {
		openAiMsgs[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: openAiMsgs,
	}

	start := time.Now()
	if deepSeekDebugEnabled() {
		if b, err := json.Marshal(req); err == nil {
			log.Printf("[deepseek][request] %s", truncateForLog(string(b), 4000))
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		if deepSeekDebugEnabled() {
			log.Printf("[deepseek][error] elapsed=%s err=%v", time.Since(start), err)
		}
		return nil, fmt.Errorf("DeepSeek Completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		if deepSeekDebugEnabled() {
			log.Printf("[deepseek][error] elapsed=%s err=no choices returned by DeepSeek API", time.Since(start))
		}
		return nil, fmt.Errorf("no choices returned by DeepSeek API")
	}

	return &agent.Response{
		Content:      resp.Choices[0].Message.Content,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}

func deepSeekDebugEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("CCCLIAI_DEBUG_DEEPSEEK")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
