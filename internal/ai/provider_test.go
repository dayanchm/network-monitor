package ai

import (
	"context"
	"errors"
	"network-monitor/internal/diagnostic"
	"testing"
)

func TestMockProviderAnalyze(t *testing.T) {
	provider := MockProvider{
		Response: "Network appears healthy.",
	}

	result := diagnostic.DiagnosticResult{
		InternetReachable: true,
		DNSHealthy:        true,
	}

	analysis, err := provider.Analyze(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis != "Network appears healthy." {
		t.Fatalf("unexpected analysis: %s", analysis)
	}
}

func TestMockProviderError(t *testing.T) {
	provider := MockProvider{
		Err: errors.New("provider unavailable"),
	}

	_, err := provider.Analyze(
		context.Background(),
		diagnostic.DiagnosticResult{},
	)

	if err == nil {
		t.Fatal("expected an error")
	}
}
