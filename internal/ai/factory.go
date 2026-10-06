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
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is not set")
		}
		return NewGeminiProvider(cfg.APIKey, cfg.Model), nil
	case "deepseek":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("DEEPSEEK_API_KEY is not set")
		}
		provider := NewDeepSeekProvider(cfg.APIKey, cfg.Model)
		if strings.TrimSpace(cfg.BaseURL) != "" {
			provider.BaseURL = cfg.BaseURL
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unknown AI provider %q", cfg.Provider)
	}
}
