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

type OpenAIProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

// Analyze implements [Provider].
func (o *OpenAIProvider) Analyze(ctx context.Context, result diagnostic.DiagnosticResult) (string, error) {
	apiKey := strings.TrimSpace(o.APIKey)
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is not set")
	}
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = DefaultOpenAIModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	measurements, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode measurements: %w", err)
	}
	body, err := json.Marshal(struct {
		Model        string `json:"model"`
		Instructions string `json:"instructions"`
		Input        string `json:"input"`
		Store        bool   `json:"store"`
	}{model, diagnosticInstructions, "Collected measurements (JSON):\n" + string(measurements), false})
	if err != nil {
		return "", fmt.Errorf("encode OpenAI request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("invalid OpenAI URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send OpenAI request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return "", fmt.Errorf("read OpenAI response: %w", err)
	}
	if len(data) > 1024*1024 {
		return "", fmt.Errorf("OpenAI response exceeds 1 MiB")
	}
	var response struct {
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	decodeErr := json.Unmarshal(data, &response)
	if resp.StatusCode != http.StatusOK {
		message := http.StatusText(resp.StatusCode)
		if response.Error != nil && response.Error.Message != "" {
			message = response.Error.Message
		}
		return "", fmt.Errorf("OpenAI HTTP %d: %s", resp.StatusCode, message)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("invalid OpenAI JSON: %w", decodeErr)
	}
	if response.Error != nil {
		return "", fmt.Errorf("OpenAI: %s", response.Error.Message)
	}
	if response.Status != "completed" {
		return "", fmt.Errorf("OpenAI returned incomplete analysis (status %q)", response.Status)
	}
	var texts []string
	for _, output := range response.Output {
		if output.Type != "message" {
			continue
		}
		for _, content := range output.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				texts = append(texts, strings.TrimSpace(content.Text))
			}
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("OpenAI returned empty analysis")
	}
	return strings.Join(texts, "\n"), nil
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = DefaultOpenAIModel
	}

	return &OpenAIProvider{
		APIKey:  apiKey,
		BaseURL: DefaultOpenAIBaseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}
}
