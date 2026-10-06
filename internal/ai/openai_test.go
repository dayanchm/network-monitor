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

func TestNewOpenAi(t *testing.T) {
	p := NewOpenAIProvider("test-key", "")
	if p.Model != DefaultOpenAIModel || p.BaseURL != DefaultOpenAIBaseURL || p.APIKey != "test-key" || p.Client == nil {
		t.Fatalf("unexpected provider: %+v", p)
	}
	if NewOpenAIProvider("test-key", "custom-model").Model != "custom-model" {
		t.Fatal("custom model was not preserved")
	}
}

func TestOpenAIAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s %v", r.Method, r.URL.Path, r.Header)
		}
		var body struct {
			Model        string `json:"model"`
			Instructions string `json:"instructions"`
			Input        string `json:"input"`
			Store        *bool  `json:"store"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != DefaultOpenAIModel || body.Instructions != diagnosticInstructions || body.Store == nil || *body.Store {
			t.Errorf("unexpected request body: %+v", body)
		}
		var measurements diagnostic.DiagnosticResult
		if err := json.Unmarshal([]byte(strings.TrimPrefix(body.Input, "Collected measurements (JSON):\n")), &measurements); err != nil || !measurements.InternetReachable {
			t.Errorf("invalid measurements: %+v, %v", measurements, err)
		}
		fmt.Fprint(w, `{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":" Network appears healthy. "},{"type":"output_text","text":"DNS is healthy."}]}]}`)
	}))
	defer server.Close()
	p := &OpenAIProvider{APIKey: "test-key", BaseURL: server.URL + "/v1/"}
	got, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{InternetReachable: true})
	if err != nil || got != "Network appears healthy.\nDNS is healthy." {
		t.Fatalf("Analyze() = %q, %v", got, err)
	}
}

func TestOpenAIAnalyzeErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"api error", 401, `{"error":{"message":"invalid API key"}}`, "OpenAI HTTP 401: invalid API key"},
		{"non JSON error", 502, "bad gateway", "OpenAI HTTP 502"},
		{"invalid JSON", 200, "{", "invalid OpenAI JSON"},
		{"incomplete", 200, `{"status":"incomplete"}`, "incomplete analysis"},
		{"empty", 200, `{"status":"completed","output":[]}`, "empty analysis"},
		{"response error", 200, `{"error":{"message":"failed"}}`, "OpenAI: failed"},
		{"oversized", 200, strings.Repeat("x", 1024*1024+1), "exceeds 1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			p := &OpenAIProvider{APIKey: "test-key", BaseURL: server.URL}
			_, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestOpenAIAnalyzeMissingKey(t *testing.T) {
	_, err := (&OpenAIProvider{}).Analyze(context.Background(), diagnostic.DiagnosticResult{})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAIAnalyzeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewOpenAIProvider("test-key", "")
	p.BaseURL = "http://127.0.0.1:1"
	_, err := p.Analyze(ctx, diagnostic.DiagnosticResult{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
