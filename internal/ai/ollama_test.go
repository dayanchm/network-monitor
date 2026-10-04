package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"network-monitor/internal/diagnostic"
	"strings"
	"testing"
)

func TestOllamaMeasurements(t *testing.T) {
	result := diagnostic.DiagnosticResult{LatencyMS: 182, PacketLoss: 11, LatencyMeasured: true, PacketLossMeasured: true, InternetReachable: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" || r.Method != "POST" {
			t.Errorf("wrong request: %s %s", r.Method, r.URL.Path)
		}
		var request struct {
			Model, Prompt, System string
			Stream                bool
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "test-model" || request.Stream || !strings.Contains(request.System, "Do not invent measurements") {
			t.Errorf("invalid request: %+v", request)
		}
		parts := strings.SplitN(request.Prompt, "\n", 2)
		var got diagnostic.DiagnosticResult
		if len(parts) != 2 {
			t.Error("missing measurements")
		} else if err := json.Unmarshal([]byte(parts[1]), &got); err != nil || got != result {
			t.Errorf("measurements changed: %+v %v", got, err)
		}
		w.Write([]byte(`{"response":"High latency and packet loss.","done":true}`))
	}))
	defer server.Close()
	provider := OllamaProvider{BaseURL: server.URL, Model: "test-model"}
	got, err := provider.Analyze(context.Background(), result)
	if err != nil || got != "High latency and packet loss." {
		t.Fatalf("%q %v", got, err)
	}
}

func TestOllamaErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"missing model", 404, `{"error":"model 'llama3.2' not found"}`, "ollama pull"},
		{"server error", 500, `{"error":"out of memory"}`, "HTTP 500"},
		{"malformed", 200, `no JSON`, "invalid Ollama JSON"},
		{"empty", 200, `{"response":"","done":true}`, "empty analysis"},
		{"incomplete", 200, `{"response":"partial","done":false}`, "incomplete"},
		{"API error", 200, `{"error":"generation failed"}`, "generation failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			provider := OllamaProvider{BaseURL: server.URL}
			_, err := provider.Analyze(context.Background(), diagnostic.DiagnosticResult{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestOllamaUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := server.URL
	server.Close()
	provider := OllamaProvider{BaseURL: endpoint}
	_, err := provider.Analyze(context.Background(), diagnostic.DiagnosticResult{})
	if err == nil || !strings.Contains(err.Error(), "Ollama unavailable") {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Analyze(ctx, diagnostic.DiagnosticResult{}); err == nil {
		t.Fatal("expected cancellation error")
	}
}
