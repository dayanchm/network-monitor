package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"network-monitor/internal/diagnostic"
	"strings"
	"time"
)

type AnthropicProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (o *AnthropicProvider) Analyze(ctx context.Context, result diagnostic.DiagnosticResult) (string, error) {
	apiKey := strings.TrimSpace(o.APIKey)
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = DefaultAnthropicModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultAnthropicBaseURL
	}
	measurements, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode measurements: %w", err)
	}
	body, err := json.Marshal(struct {
		Model     string    `json:"model"`
		System    string    `json:"system"`
		Messages  []message `json:"messages"`
		MaxTokens int       `json:"max_tokens"`
	}{Model: model,
		System: diagnosticInstructions,
		Messages: []message{
			{
				Role:    "user",
				Content: "Collected measurements (JSON):\n" + string(measurements),
			},
		},
		MaxTokens: 1024})
	if err != nil {
		return "", fmt.Errorf("encode Anthropic request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("invalid Anthropic URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send Anthropic request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return "", fmt.Errorf("read Anthropic response: %w", err)
	}
	if len(data) > 1024*1024 {
		return "", fmt.Errorf("Anthropic response exceeds 1 MiB")
	}
	var response struct {
		StopReason string `json:"stop_reason"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	decodeErr := json.Unmarshal(data, &response)
	if resp.StatusCode != http.StatusOK {
		message := http.StatusText(resp.StatusCode)
		if response.Error != nil && response.Error.Message != "" {
			message = response.Error.Message
		}
		return "", fmt.Errorf("Anthropic HTTP %d: %s", resp.StatusCode, message)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("invalid Anthropic JSON: %w", decodeErr)
	}
	if response.Error != nil {
		return "", fmt.Errorf("Anthropic: %s", response.Error.Message)
	}
	if response.StopReason != "end_turn" {
		return "", fmt.Errorf(
			"Anthropic returned incomplete analysis (stop_reason %q)",
			response.StopReason,
		)
	}
	var texts []string
	for _, content := range response.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			texts = append(texts, strings.TrimSpace(content.Text))
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("Anthropic returned empty analysis")
	}
	return strings.Join(texts, "\n"), nil
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = DefaultAnthropicModel
	}

	return &AnthropicProvider{
		APIKey:  apiKey,
		BaseURL: DefaultAnthropicBaseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}
}
