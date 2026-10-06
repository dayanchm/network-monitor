package ai

import "testing"

func TestNewProviderDeepSeek(t *testing.T) {
	provider, err := NewProvider(Config{Provider: "deepseek", APIKey: "test-key", Model: "custom-model", BaseURL: "http://localhost:1234"})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := provider.(*DeepSeekProvider)
	if !ok {
		t.Fatalf("expected *DeepSeekProvider, got %T", provider)
	}
	if p.APIKey != "test-key" || p.Model != "custom-model" || p.BaseURL != "http://localhost:1234" {
		t.Fatalf("unexpected provider: %+v", p)
	}
}

func TestNewProviderDeepSeekMissingKey(t *testing.T) {
	_, err := NewProvider(Config{Provider: "deepseek", APIKey: "   "})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestNewProviderDefaultsToOllama(t *testing.T) {
	provider, err := NewProvider(Config{})

	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if _, ok := provider.(*OllamaProvider); !ok {
		t.Fatalf("expected *OllamaProvider, got %T", provider)
	}
}

func TestNewProviderUnknownProvider(t *testing.T) {
	_, err := NewProvider(Config{
		Provider: "something",
	})

	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewProviderAnthropic(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-key",
		Model:    "custom-model",
	})
	if err != nil {
		t.Fatalf("NewProvider() error= %v", err)
	}
	p, ok := provider.(*AnthropicProvider)
	if !ok {
		t.Fatalf("expected *AnthropicProvider, got %T", provider)
	}
	if p.APIKey != "test-key" || p.Model != "custom-model" {
		t.Fatalf("unexpected provider: %+v", p)
	}
}

func TestNewProviderAnthropicMissingKey(t *testing.T) {
	_, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "  ",
	})
	if err == nil {
		t.Fatalf("expected error for missing API key")
	}
}
