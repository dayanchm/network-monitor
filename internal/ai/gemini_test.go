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

func TestNewGemini(t *testing.T) {
	p := NewGeminiProvider("test-key", "")
	if p.Model != DefaultGeminiModel || p.BaseURL != DefaultGeminiBaseURL || p.APIKey != "test-key" || p.Client == nil {
		t.Fatalf("unexpected provider: %+v", p)
	}
	if NewGeminiProvider("test-key", "custom-model").Model != "custom-model" {
		t.Fatal("custom model was not preserved")
	}
}

func TestGeminiAnalyze(t *testing.T) {
	type testPart struct {
		Text string `json:"text"`
	}
	type testContent struct {
		Role  string     `json:"role"`
		Parts []testPart `json:"parts"`
	}

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			wantPath := "/v1beta/models/" +
				DefaultGeminiModel + ":generateContent"

			if r.Method != http.MethodPost ||
				r.URL.Path != wantPath ||
				r.Header.Get("x-goog-api-key") != "test-key" ||
				r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("unexpected request: %s %s %v",
					r.Method, r.URL.Path, r.Header)
			}

			var body struct {
				SystemInstruction testContent   `json:"systemInstruction"`
				Contents          []testContent `json:"contents"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if len(body.SystemInstruction.Parts) != 1 ||
				body.SystemInstruction.Parts[0].Text != diagnosticInstructions {
				t.Errorf("unexpected system instruction: %+v",
					body.SystemInstruction)
			}
			if len(body.Contents) != 1 ||
				len(body.Contents[0].Parts) != 1 {
				t.Errorf("unexpected contents: %+v", body.Contents)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.Contents[0].Role != "user" {
				t.Errorf("unexpected role: %q", body.Contents[0].Role)
			}

			input := strings.TrimPrefix(
				body.Contents[0].Parts[0].Text,
				"Collected measurements (JSON):\n",
			)
			var measurements diagnostic.DiagnosticResult
			if err := json.Unmarshal([]byte(input), &measurements); err != nil ||
				!measurements.InternetReachable {
				t.Errorf("invalid measurements: %+v, %v", measurements, err)
			}

			fmt.Fprint(w, `{"candidates":[{"finishReason":"STOP",
				"content":{"parts":[
					{"text":"Internal reasoning","thought":true},
					{"text":" Network appears healthy. "},
					{"text":"DNS is healthy."}
				]}}]}`)
		},
	))
	defer server.Close()

	p := &GeminiProvider{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1beta/",
	}
	got, err := p.Analyze(
		context.Background(),
		diagnostic.DiagnosticResult{InternetReachable: true},
	)
	if err != nil || got != "Network appears healthy.\nDNS is healthy." {
		t.Fatalf("Analyze() = %q, %v", got, err)
	}
}

func TestGeminiAnalyzeErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"api error", 401, `{"error":{"message":"invalid API key"}}`, "Gemini HTTP 401: invalid API key"},
		{"non JSON error", 502, "bad gateway", "Gemini HTTP 502"},
		{"invalid JSON", 200, "{", "invalid Gemini JSON"},
		{"incomplete", 200, `{"candidates":[{"finishReason":"MAX_TOKENS"}]}`, "incomplete analysis"},
		{"empty", 200, `{"candidates":[{"finishReason":"STOP","content":{"parts":[]}}]}`, "empty analysis"},
		{"no candidates", 200, `{"candidates":[]}`, "empty analysis"},
		{"blocked", 200, `{"promptFeedback":{"blockReason":"SAFETY"}}`, "blocked analysis"},
		{"response error", 200, `{"error":{"message":"failed"}}`, "Gemini: failed"},
		{"oversized", 200, strings.Repeat("x", 1024*1024+1), "exceeds 1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			p := &GeminiProvider{APIKey: "test-key", BaseURL: server.URL}
			_, err := p.Analyze(context.Background(), diagnostic.DiagnosticResult{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGeminiAnalyzeMissingKey(t *testing.T) {
	_, err := (&GeminiProvider{}).Analyze(context.Background(), diagnostic.DiagnosticResult{})
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeminiAnalyzeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewGeminiProvider("test-key", "")
	p.BaseURL = "http://127.0.0.1:1"
	_, err := p.Analyze(ctx, diagnostic.DiagnosticResult{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
