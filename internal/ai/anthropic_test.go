package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"network-monitor/internal/diagnostic"
	"strings"
	"testing"
)

func TestNewAnthropic(t *testing.T) {
	p := NewAnthropicProvider("test-key", "")
	if p.Model != DefaultAnthropicModel || p.BaseURL != DefaultAnthropicBaseURL || p.APIKey != "test-key" || p.Client == nil {
		t.Fatalf("unexpected provider: %+v", p)
	}
	if NewAnthropicProvider("test-key", "custom-model").Model != "custom-model" {
		t.Fatal("custom model was not preserved")
	}
}

func TestAnthropicAnalyze(t *testing.T) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "test-key" || r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s %v", r.Method, r.URL.Path, r.Header)
		}
		var body struct {
			Model     string    `json:"model"`
			System    string    `json:"system"`
			Messages  []message `json:"messages"`
			MaxTokens int       `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Model != DefaultAnthropicModel || body.System != diagnosticInstructions || body.MaxTokens != 1024 {
			t.Errorf("unexpected request body: %+v", body)
		}
		if len(body.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(body.Messages))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Messages[0].Role != "user" {
			t.Errorf("unexpected role: %q", body.Messages[0].Role)
		}

		var measurements diagnostic.DiagnosticResult
		input := strings.TrimPrefix(
			body.Messages[0].Content,
			"Collected measurements (JSON):\n",
		)
		if err := json.Unmarshal([]byte(input), &measurements); err != nil ||
			!measurements.InternetReachable {
			t.Errorf("invalid measurements: %+v, %v", measurements, err)
		}
		fmt.Fprint(w, `{"stop_reason":"end_turn","content":[{"type":"text","text":" Network appears healthy. "},{"type":"text","text":"DNS is healthy."}]}`)
	}))
	defer server.Close()
	p := &AnthropicProvider{APIKey: "test-key", BaseURL: server.URL + "/v1/"}
	got, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{InternetReachable: true})
	if err != nil || got != "Network appears healthy.\nDNS is healthy." {
		t.Fatalf("Analyze() = %q, %v", got, err)
	}
}

func TestAnthropicAnalyzeErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"api error", 401, `{"error":{"message":"invalid API key"}}`, "Anthropic HTTP 401: invalid API key"},
		{"non JSON error", 502, "bad gateway", "Anthropic HTTP 502"},
		{"invalid JSON", 200, "{", "invalid Anthropic JSON"},
		{"incomplete", 200, `{"stop_reason":"max_tokens"}`, "incomplete analysis"},
		{"empty", 200, `{"stop_reason":"end_turn","content":[]}`, "empty analysis"},
		{"response error", 200, `{"error":{"message":"failed"}}`, "Anthropic: failed"},
		{"oversized", 200, strings.Repeat("x", 1024*1024+1), "exceeds 1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			p := &AnthropicProvider{APIKey: "test-key", BaseURL: server.URL}
			_, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAnthropicAnalyzeMissingKey(t *testing.T) {
	_, err := (&AnthropicProvider{}).Analyze(context.Background(), diagnostic.DiagnosticResult{})
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnthropicAnalyzeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewAnthropicProvider("test-key", "")
	p.BaseURL = "http://127.0.0.1:1"
	_, err := p.Analyze(ctx, diagnostic.DiagnosticResult{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
