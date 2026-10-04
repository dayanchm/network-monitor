package diagnosticcli

import (
	"bytes"
	"context"
	"errors"
	"network-monitor/internal/ai"
	"network-monitor/internal/diagnostic"
	"strings"
	"testing"
)

func TestSeparateAnalysis(t *testing.T) {
	for _, failed := range []bool{false, true} {
		var output bytes.Buffer
		result := diagnostic.DiagnosticResult{GatewayReachable: true, GatewayStatus: "reachable"}
		if err := result.Print(&output); err != nil {
			t.Fatal(err)
		}
		provider := ai.MockProvider{Response: "Based on collected measurements."}
		if failed {
			provider.Err = errors.New("server offline")
		}
		if err := printAnalysis(context.Background(), provider, result, &output); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if !strings.HasPrefix(text, "Network Diagnostics") || !strings.Contains(text, "\nAI Analysis\n\n") {
			t.Fatal(text)
		}
		if failed && !strings.Contains(text, "Unavailable: server offline") {
			t.Fatal(text)
		}
		if !failed && !strings.Contains(text, provider.Response) {
			t.Fatal(text)
		}
	}
}
