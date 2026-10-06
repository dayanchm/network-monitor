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

type OllamaProvider struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func (o *OllamaProvider) Analyze(ctx context.Context, result diagnostic.DiagnosticResult) (string, error) {
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = DefaultModel
	}
	baseURL := strings.TrimRight(o.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	measurements, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode measurements: %w", err)
	}
	body, err := json.Marshal(struct {
		Model  string `json:"model"`
		System string `json:"system"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}{model, diagnosticInstructions, "Collected measurements (JSON):\n" + string(measurements), false})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("invalid Ollama URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama unavailable at %s (start it with ollama serve): %w", baseURL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return "", fmt.Errorf("read Ollama response: %w", err)
	}
	if len(data) > 1024*1024 {
		return "", fmt.Errorf("Ollama response exceeds 1 MiB")
	}
	var response struct {
		Response string `json:"response"`
		Error    string `json:"error"`
		Done     bool   `json:"done"`
	}
	decodeErr := json.Unmarshal(data, &response)
	if resp.StatusCode == http.StatusNotFound && strings.Contains(strings.ToLower(response.Error), "model") {
		return "", fmt.Errorf("Ollama model %q not found; run: ollama pull %s", model, model)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama HTTP %d: %s", resp.StatusCode, response.Error)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("invalid Ollama JSON: %w", decodeErr)
	}
	if response.Error != "" {
		return "", fmt.Errorf("Ollama: %s", response.Error)
	}
	if !response.Done {
		return "", fmt.Errorf("Ollama returned incomplete analysis")
	}
	if strings.TrimSpace(response.Response) == "" {
		return "", fmt.Errorf("Ollama returned empty analysis")
	}
	return strings.TrimSpace(response.Response), nil
}
