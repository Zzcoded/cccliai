package channels

import (
	"context"
	"log"

	"github.com/cccliai/app/internal/agent"
	"github.com/cccliai/app/internal/config"
	"github.com/cccliai/app/internal/providers"
)

type agentProvider interface {
	ChatCompletion(ctx context.Context, msgs []agent.Message) (*agent.Response, error)
}

func getFallbackProvider() agentProvider {
	deepKey := config.ProviderAPIKey("deepseek")
	if deepKey != "" {
		return providers.NewDeepSeekProvider(deepKey, config.ProviderModel("deepseek", "deepseek-chat"))
	}

	zaiKey := config.ProviderAPIKey("zai")
	if zaiKey != "" {
		return providers.NewZAIProvider(zaiKey, config.ProviderModel("zai", "zai-1"))
	}

	log.Printf("Warning: No AI keys found for channel processing")
	return nil
}
