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

type GeminiProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

type part struct {
	Text string `json:"text"`
}
type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	if model == "" {
		model = DefaultGeminiModel
	}
	return &GeminiProvider{
		APIKey:  apiKey,
		BaseURL: DefaultGeminiBaseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

func (o *GeminiProvider) Analyze(ctx context.Context, result diagnostic.DiagnosticResult) (string, error) {
	apiKey := strings.TrimSpace(o.APIKey)
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = DefaultGeminiModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultGeminiBaseURL
	}
	measurements, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode measurements: %w", err)
	}
	body, err := json.Marshal(struct {
		SystemInstruction content   `json:"systemInstruction"`
		Contents          []content `json:"contents"`
	}{
		SystemInstruction: content{
			Parts: []part{{Text: diagnosticInstructions}},
		},
		Contents: []content{
			{
				Role: "user",
				Parts: []part{{
					Text: "Collected measurements (JSON):\n" + string(measurements),
				}},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("encode Gemini request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/models/"+model+":generateContent", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("invalid Gemini URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send Gemini request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return "", fmt.Errorf("read Gemini response: %w", err)
	}
	if len(data) > 1024*1024 {
		return "", fmt.Errorf("Gemini response exceeds 1 MiB")
	}
	var response struct {
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}
	decodeErr := json.Unmarshal(data, &response)
	if resp.StatusCode != http.StatusOK {
		message := http.StatusText(resp.StatusCode)
		if response.Error != nil && response.Error.Message != "" {
			message = response.Error.Message
		}
		return "", fmt.Errorf("Gemini HTTP %d: %s", resp.StatusCode, message)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("invalid Gemini JSON: %w", decodeErr)
	}
	if response.Error != nil {
		return "", fmt.Errorf("Gemini: %s", response.Error.Message)
	}
	if response.PromptFeedback.BlockReason != "" {
		return "", fmt.Errorf("Gemini blocked analysis: %s",
			response.PromptFeedback.BlockReason)
	}
	if len(response.Candidates) == 0 {
		return "", fmt.Errorf("Gemini returned empty analysis")
	}
	candidate := response.Candidates[0]
	if candidate.FinishReason != "STOP" {
		return "", fmt.Errorf("Gemini returned incomplete analysis (finishReason %q)",
			candidate.FinishReason)
	}

	var texts []string
	for _, p := range candidate.Content.Parts {
		if !p.Thought && strings.TrimSpace(p.Text) != "" {
			texts = append(texts, strings.TrimSpace(p.Text))
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("Gemini returned empty analysis")
	}
	return strings.Join(texts, "\n"), nil
}
