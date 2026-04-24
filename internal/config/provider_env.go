package config

import (
	"os"
	"strings"
)

func ProviderAPIKey(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = strings.ToLower(strings.TrimSpace(os.Getenv("PROVIDER_TYPE")))
	}

	switch p {
	case "zai":
		if k := strings.TrimSpace(os.Getenv("ZAI_API_KEY")); k != "" {
			return k
		}
		return sharedProviderAPIKey("zai")
	default:
		if k := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")); k != "" {
			return k
		}
		return sharedProviderAPIKey("deepseek")
	}
}

func ProviderModel(provider string, fallback string) string {
	model := strings.TrimSpace(os.Getenv("PROVIDER_MODEL"))
	if model == "" {
		return fallback
	}

	providerType := strings.ToLower(strings.TrimSpace(os.Getenv("PROVIDER_TYPE")))
	if providerType == "" || providerType == strings.ToLower(strings.TrimSpace(provider)) {
		return model
	}

	return fallback
}

func ProviderAPIKeyHint(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "zai":
		return "ZAI_API_KEY or PROVIDER_API_KEY (+ PROVIDER_TYPE=zai)"
	default:
		return "DEEPSEEK_API_KEY or PROVIDER_API_KEY (+ PROVIDER_TYPE=deepseek)"
	}
}

func sharedProviderAPIKey(provider string) string {
	shared := strings.TrimSpace(os.Getenv("PROVIDER_API_KEY"))
	if shared == "" {
		return ""
	}

	sharedType := strings.ToLower(strings.TrimSpace(os.Getenv("PROVIDER_TYPE")))
	if sharedType == "" || sharedType == strings.ToLower(strings.TrimSpace(provider)) {
		return shared
	}

	return ""
}
