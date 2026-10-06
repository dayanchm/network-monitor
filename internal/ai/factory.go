package ai

import (
	"fmt"
	"strings"
)

type Config struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

func NewProvider(cfg Config) (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))

	if name == "" {
		name = "ollama"
	}

	switch name {
	case "ollama":
		return &OllamaProvider{
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		}, nil

	case "openai":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is not set")
		}

		return NewOpenAIProvider(cfg.APIKey, cfg.Model), nil
	case "anthropic":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
		return NewAnthropicProvider(cfg.APIKey, cfg.Model), nil
	case "gemini":
		return nil, fmt.Errorf("gemini provider is not implemented yet")
	case "deepseek":
		return nil, fmt.Errorf("deepseek provider is not implemented yet")
	default:
		return nil, fmt.Errorf("unknown AI provider %q", cfg.Provider)
	}
}
