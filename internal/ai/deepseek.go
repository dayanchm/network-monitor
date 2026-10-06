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

type DeepSeekProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewDeepSeekProvider(apiKey, model string) *DeepSeekProvider {
	if strings.TrimSpace(model) == "" {
		model = DefaultDeepSeekModel
	}
	return &DeepSeekProvider{APIKey: apiKey, BaseURL: DefaultDeepSeekBaseURL, Model: model, Client: &http.Client{Timeout: 2 * time.Minute}}
}

func (o *DeepSeekProvider) Analyze(ctx context.Context, result diagnostic.DiagnosticResult) (string, error) {
	apiKey := strings.TrimSpace(o.APIKey)
	if apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY is not set")
	}
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = DefaultDeepSeekModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultDeepSeekBaseURL
	}
	measurements, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode measurements: %w", err)
	}
	type deepSeekMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body, err := json.Marshal(struct {
		Model           string            `json:"model"`
		Messages        []deepSeekMessage `json:"messages"`
		Stream          bool              `json:"stream"`
		ReasoningEffort string            `json:"reasoning_effort"`
	}{model, []deepSeekMessage{
		{Role: "system", Content: diagnosticInstructions},
		{Role: "user", Content: "Collected measurements (JSON):\n" + string(measurements)},
	}, false, "none"})
	if err != nil {
		return "", fmt.Errorf("encode DeepSeek request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("invalid DeepSeek URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send DeepSeek request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return "", fmt.Errorf("read DeepSeek response: %w", err)
	}
	if len(data) > 1024*1024 {
		return "", fmt.Errorf("DeepSeek response exceeds 1 MiB")
	}
	var response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	decodeErr := json.Unmarshal(data, &response)
	if resp.StatusCode != http.StatusOK {
		message := http.StatusText(resp.StatusCode)
		if response.Error != nil && response.Error.Message != "" {
			message = response.Error.Message
		}
		return "", fmt.Errorf("DeepSeek HTTP %d: %s", resp.StatusCode, message)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("invalid DeepSeek JSON: %w", decodeErr)
	}
	if response.Error != nil {
		return "", fmt.Errorf("DeepSeek: %s", response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek returned empty analysis")
	}
	choice := response.Choices[0]
	if choice.FinishReason != "stop" {
		return "", fmt.Errorf("DeepSeek returned incomplete analysis (finish_reason %q)", choice.FinishReason)
	}
	analysis := strings.TrimSpace(choice.Message.Content)
	if analysis == "" {
		return "", fmt.Errorf("DeepSeek returned empty analysis")
	}
	return analysis, nil
}
