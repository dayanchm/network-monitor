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

func TestNewDeepSeekProvider(t *testing.T) {
	p := NewDeepSeekProvider("test-key", "")
	if p.Model != DefaultDeepSeekModel || p.BaseURL != DefaultDeepSeekBaseURL || p.APIKey != "test-key" || p.Client == nil {
		t.Fatalf("unexpected provider: %+v", p)
	}
	if NewDeepSeekProvider("test-key", "custom-model").Model != "custom-model" {
		t.Fatal("custom model was not preserved")
	}
}

func TestDeepSeekAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s %v", r.Method, r.URL.Path, r.Header)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream          bool   `json:"stream"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if body.Model != DefaultDeepSeekModel || body.Stream || body.ReasoningEffort != "none" || len(body.Messages) != 2 {
			t.Errorf("unexpected body: %+v", body)
			w.WriteHeader(400)
			return
		}
		if body.Messages[0].Role != "system" || body.Messages[0].Content != diagnosticInstructions || body.Messages[1].Role != "user" {
			t.Errorf("unexpected messages: %+v", body.Messages)
		}
		var measurements diagnostic.DiagnosticResult
		if err := json.Unmarshal([]byte(strings.TrimPrefix(body.Messages[1].Content, "Collected measurements (JSON):\n")), &measurements); err != nil || !measurements.InternetReachable {
			t.Errorf("invalid measurements: %+v, %v", measurements, err)
		}
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"content":" Network appears healthy. ","reasoning_content":"do not return this"}}]}`)
	}))
	defer server.Close()
	p := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL + "/"}
	got, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{InternetReachable: true})
	if err != nil || got != "Network appears healthy." {
		t.Fatalf("Analyze() = %q, %v", got, err)
	}
}

func TestDeepSeekAnalyzeErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"api error", 401, `{"error":{"message":"invalid API key"}}`, "DeepSeek HTTP 401: invalid API key"},
		{"non JSON error", 502, "bad gateway", "DeepSeek HTTP 502"},
		{"invalid JSON", 200, "{", "invalid DeepSeek JSON"},
		{"incomplete", 200, `{"choices":[{"finish_reason":"length"}]}`, "incomplete analysis"},
		{"no choices", 200, `{"choices":[]}`, "empty analysis"},
		{"empty text", 200, `{"choices":[{"finish_reason":"stop","message":{"content":"   "}}]}`, "empty analysis"},
		{"response error", 200, `{"error":{"message":"failed"}}`, "DeepSeek: failed"},
		{"oversized", 200, strings.Repeat("x", 1024*1024+1), "exceeds 1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			p := &DeepSeekProvider{APIKey: "test-key", BaseURL: server.URL}
			_, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDeepSeekAnalyzeMissingKey(t *testing.T) {
	_, err := (&DeepSeekProvider{APIKey: "   "}).Analyze(context.Background(), diagnostic.DiagnosticResult{})
	if err == nil || !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeepSeekAnalyzeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewDeepSeekProvider("test-key", "")
	p.BaseURL = "http://127.0.0.1:1"
	_, err := p.Analyze(ctx, diagnostic.DiagnosticResult{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
